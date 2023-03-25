package file

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buger/goreplay/internal/size"
	"github.com/buger/goreplay/pkg/emitter"
	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/proto"
	"github.com/buger/goreplay/pkg/test"
)

func TestFileOutput(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := test.NewTestInput()
	output := NewFileOutput("/tmp/test_requests.gor", &FileOutputConfig{FlushInterval: time.Minute, Append: true})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	em := emitter.New()
	go em.Start(plugins)

	for i := 0; i < 100; i++ {
		wg.Add(2)
		input.EmitGET()
		input.EmitPOST()
	}
	time.Sleep(100 * time.Millisecond)
	output.flush()
	em.Close()

	var counter int64
	input2 := NewFileInput("/tmp/test_requests.gor", false, 100, 0, false)
	output2 := test.NewTestOutput(func(*plugin.Message) {
		atomic.AddInt64(&counter, 1)
		wg.Done()
	})

	plugins2 := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input2},
		Outputs: []plugin.Writer{output2},
	}
	plugins2.All = append(plugins2.All, input2, output2)

	em2 := emitter.New()
	go em2.Start(plugins2)

	wg.Wait()
	em2.Close()
}

func TestFileOutputWithNameCleaning(t *testing.T) {
	output := &FileOutput{pathTemplate: "./test_requests.gor", config: &FileOutputConfig{FlushInterval: time.Minute, Append: false}}
	expectedFileName := "test_requests_0.gor"
	output.updateName()

	if expectedFileName != output.currentName {
		t.Errorf("Expected path %s but got %s", expectedFileName, output.currentName)
	}

}

func TestFileOutputPathTemplate(t *testing.T) {
	output := &FileOutput{pathTemplate: "/tmp/log-%Y-%m-%d-%S-%t", config: &FileOutputConfig{FlushInterval: time.Minute, Append: true}}
	now := time.Now()
	output.payloadType = []byte("3")
	expectedPath := fmt.Sprintf("/tmp/log-%s-%s-%s-%s-3", now.Format("2006"), now.Format("01"), now.Format("02"), now.Format("05"))
	path := output.filename()

	if expectedPath != path {
		t.Errorf("Expected path %s but got %s", expectedPath, path)
	}
}

func TestFileOutputMultipleFiles(t *testing.T) {
	output := NewFileOutput("/tmp/log-%Y-%m-%d-%S", &FileOutputConfig{Append: true, FlushInterval: time.Minute})

	if output.file != nil {
		t.Error("Should not initialize file if no writes")
	}

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name1 := output.file.Name()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name2 := output.file.Name()

	time.Sleep(time.Second)
	output.updateName()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name3 := output.file.Name()

	if name2 != name1 {
		t.Error("Fast changes should happen in same file:", name1, name2, name3)
	}

	if name3 == name1 {
		t.Error("File name should change:", name1, name2, name3)
	}

	os.Remove(name1)
	os.Remove(name3)
}

func TestFileOutputFilePerRequest(t *testing.T) {
	output := NewFileOutput("/tmp/log-%Y-%m-%d-%S-%r", &FileOutputConfig{Append: true})

	if output.file != nil {
		t.Error("Should not initialize file if no writes")
	}

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name1 := output.file.Name()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 2 1\r\n"), Data: []byte("test")})
	name2 := output.file.Name()

	time.Sleep(time.Second)
	output.updateName()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 3 1\r\n"), Data: []byte("test")})
	name3 := output.file.Name()

	if name3 == name2 || name2 == name1 || name3 == name1 {
		t.Error("File name should change:", name1, name2, name3)
	}

	os.Remove(name1)
	os.Remove(name2)
	os.Remove(name3)
}

func TestFileOutputCompression(t *testing.T) {
	output := NewFileOutput("/tmp/log-%Y-%m-%d-%S.gz", &FileOutputConfig{Append: true, FlushInterval: time.Minute})

	if output.file != nil {
		t.Error("Should not initialize file if no writes")
	}

	for i := 0; i < 1000; i++ {
		output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	}

	name := output.file.Name()
	output.Close()

	s, _ := os.Stat(name)
	if s.Size() == 12*1000 {
		t.Error("Should be compressed file:", s.Size())
	}

	os.Remove(name)
}

func TestGetFileIndex(t *testing.T) {
	var tests = []struct {
		path  string
		index int
	}{
		{"/tmp/logs", -1},
		{"/tmp/logs_1", 1},
		{"/tmp/logs_2.gz", 2},
		{"/tmp/logs_0.gz", 0},
	}

	for _, c := range tests {
		if GetFileIndex(c.path) != c.index {
			t.Error(c.path, "should be", c.index, "instead", GetFileIndex(c.path))
		}
	}
}

func TestSetFileIndex(t *testing.T) {
	var tests = []struct {
		path    string
		index   int
		newPath string
	}{
		{"/tmp/logs", 0, "/tmp/logs_0"},
		{"/tmp/logs.gz", 1, "/tmp/logs_1.gz"},
		{"/tmp/logs_1", 0, "/tmp/logs_0"},
		{"/tmp/logs_0", 10, "/tmp/logs_10"},
		{"/tmp/logs_0.gz", 10, "/tmp/logs_10.gz"},
		{"/tmp/logs_underscores.gz", 10, "/tmp/logs_underscores_10.gz"},
	}

	for _, c := range tests {
		if setFileIndex(c.path, c.index) != c.newPath {
			t.Error(c.path, "should be", c.newPath, "instead", setFileIndex(c.path, c.index))
		}
	}
}

func TestFileOutputAppendQueueLimitOverflow(t *testing.T) {
	rnd := rand.Int63()
	name := fmt.Sprintf("/tmp/%d", rnd)

	output := NewFileOutput(name, &FileOutputConfig{Append: false, FlushInterval: time.Minute, QueueLimit: 2})

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name1 := output.file.Name()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name2 := output.file.Name()

	output.updateName()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name3 := output.file.Name()

	if name2 != name1 || name1 != fmt.Sprintf("/tmp/%d_0", rnd) {
		t.Error("Fast changes should happen in same file:", name1, name2, name3)
	}

	if name3 == name1 || name3 != fmt.Sprintf("/tmp/%d_1", rnd) {
		t.Error("File name should change:", name1, name2, name3)
	}

	os.Remove(name1)
	os.Remove(name3)
}

func TestFileOutputAppendQueueLimitNoOverflow(t *testing.T) {
	rnd := rand.Int63()
	name := fmt.Sprintf("/tmp/%d", rnd)

	output := NewFileOutput(name, &FileOutputConfig{Append: false, FlushInterval: time.Minute, QueueLimit: 3})

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name1 := output.file.Name()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name2 := output.file.Name()

	output.updateName()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name3 := output.file.Name()

	if name2 != name1 || name1 != fmt.Sprintf("/tmp/%d_0", rnd) {
		t.Error("Fast changes should happen in same file:", name1, name2, name3)
	}

	if name3 != name1 || name3 != fmt.Sprintf("/tmp/%d_0", rnd) {
		t.Error("File name should not change:", name1, name2, name3)
	}

	os.Remove(name1)
	os.Remove(name3)
}

func TestFileOutputAppendQueueLimitGzips(t *testing.T) {
	rnd := rand.Int63()
	name := fmt.Sprintf("/tmp/%d.gz", rnd)

	output := NewFileOutput(name, &FileOutputConfig{Append: false, FlushInterval: time.Minute, QueueLimit: 2})

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name1 := output.file.Name()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name2 := output.file.Name()

	output.updateName()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name3 := output.file.Name()

	if name2 != name1 || name1 != fmt.Sprintf("/tmp/%d_0.gz", rnd) {
		t.Error("Fast changes should happen in same file:", name1, name2, name3)
	}

	if name3 == name1 || name3 != fmt.Sprintf("/tmp/%d_1.gz", rnd) {
		t.Error("File name should change:", name1, name2, name3)
	}

	os.Remove(name1)
	os.Remove(name3)
}

func TestFileOutputSort(t *testing.T) {
	var files = []string{"2016_0", "2014_10", "2015_0", "2015_10", "2015_2"}
	var expected = []string{"2014_10", "2015_0", "2015_2", "2015_10", "2016_0"}
	sort.Sort(sortByFileIndex(files))

	if !reflect.DeepEqual(files, expected) {
		t.Error("Should properly sort file names using indexes", files, expected)
	}
}

func TestFileOutputAppendSizeLimitOverflow(t *testing.T) {
	rnd := rand.Int63()
	name := fmt.Sprintf("/tmp/%d", rnd)

	message := []byte("1 1 1\r\ntest")

	messageSize := len(message) + len(proto.PayloadSeparator)

	output := NewFileOutput(name, &FileOutputConfig{Append: false, FlushInterval: time.Minute, SizeLimit: size.Size(2 * messageSize)})

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name1 := output.file.Name()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name2 := output.file.Name()

	output.flush()

	output.PluginWrite(&plugin.Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	name3 := output.file.Name()

	if name2 != name1 || name1 != fmt.Sprintf("/tmp/%d_0", rnd) {
		t.Error("Fast changes should happen in same file:", name1, name2, name3)
	}

	if name3 == name1 || name3 != fmt.Sprintf("/tmp/%d_1", rnd) {
		t.Error("File name should change:", name1, name2, name3)
	}

	os.Remove(name1)
	os.Remove(name3)
}
