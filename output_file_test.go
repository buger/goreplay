package main

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileOutput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()
	output := NewFileOutput("/tmp/test_requests.gor", &FileOutputConfig{flushInterval: time.Minute, append: true})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	go Start(plugins, quit)

	for i := 0; i < 100; i++ {
		wg.Add(2)
		input.EmitGET()
		input.EmitPOST()
	}
	time.Sleep(100 * time.Millisecond)
	output.flush()

	close(quit)

	quit = make(chan int)

	var counter int64
	input2 := NewFileInput("/tmp/test_requests.gor", false)
	output2 := NewTestOutput(func(data []byte) {
		atomic.AddInt64(&counter, 1)
		wg.Done()
	})

	plugins2 := &InOutPlugins{
		Inputs:  []io.Reader{input2},
		Outputs: []io.Writer{output2},
	}

	go Start(plugins2, quit)

	wg.Wait()
	close(quit)
}

func TestFileOutputWithNameCleaning(t *testing.T) {
	output := &FileOutput{pathTemplate: "./test_requests.gor", config: &FileOutputConfig{flushInterval: time.Minute, append: false}}
	expectedFileName := "test_requests_0.gor"
	output.updateName()

	if expectedFileName != output.currentName {
		t.Errorf("Expected path %s but got %s", expectedFileName, output.currentName)
	}

}

func TestFileOutputPathTemplate(t *testing.T) {
	output := &FileOutput{pathTemplate: "/tmp/log-%Y-%m-%d-%S-%t", config: &FileOutputConfig{flushInterval: time.Minute, append: true}}
	now := time.Now()
	output.payloadType = []byte("3")
	expectedPath := fmt.Sprintf("/tmp/log-%s-%s-%s-%s-3", now.Format("2006"), now.Format("01"), now.Format("02"), now.Format("05"))
	path := output.filename()

	if expectedPath != path {
		t.Errorf("Expected path %s but got %s", expectedPath, path)
	}
}

func TestFileOutputMultipleFiles(t *testing.T) {
	output := NewFileOutput("/tmp/log-%Y-%m-%d-%S", &FileOutputConfig{append: true, flushInterval: time.Minute})

	if output.file != nil {
		t.Error("Should not initialize file if no writes")
	}

	output.Write([]byte("1 1 1\r\ntest"))
	name1 := output.file.Name()

	output.Write([]byte("1 1 1\r\ntest"))
	name2 := output.file.Name()

	time.Sleep(time.Second)
	output.updateName()

	output.Write([]byte("1 1 1\r\ntest"))
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
	output := NewFileOutput("/tmp/log-%Y-%m-%d-%S-%r", &FileOutputConfig{append: true})

	if output.file != nil {
		t.Error("Should not initialize file if no writes")
	}

	output.Write([]byte("1 1 1\ntest"))
	name1 := output.file.Name()

	output.Write([]byte("1 2 1\ntest"))
	name2 := output.file.Name()

	time.Sleep(time.Second)
	output.updateName()

	output.Write([]byte("1 3 1\ntest"))
	name3 := output.file.Name()

	if name3 == name2 || name2 == name1 || name3 == name1 {
		t.Error("File name should change:", name1, name2, name3)
	}

	os.Remove(name1)
	os.Remove(name2)
	os.Remove(name3)
}

func TestFileOutputCompression(t *testing.T) {
	output := NewFileOutput("/tmp/log-%Y-%m-%d-%S.gz", &FileOutputConfig{append: true, flushInterval: time.Minute})

	if output.file != nil {
		t.Error("Should not initialize file if no writes")
	}

	for i := 0; i < 1000; i++ {
		output.Write([]byte("1 1 1\r\ntest"))
	}

	name := output.file.Name()
	output.Close()

	s, _ := os.Stat(name)
	if s.Size() == 12*1000 {
		t.Error("Should be compressed file:", s.Size())
	}

	os.Remove(name)
}

func TestParseDataUnit(t *testing.T) {
	var d = map[string]int64{
		"42mb":                 42 << 20,
		"4_2":                  42,
		"00":                   0,
		"\n\n 0.0\r\t\f":       0,
		"0_600tb":              384 << 40,
		"0600Tb":               384 << 40,
		"0o12Mb":               10 << 20,
		"0b_10010001111_1kb":   2335 << 10,
		"1024":                 1 << 10,
		"0b111":                7,
		"0x12gB":               18 << 30,
		"0x_67_7a_2f_cc_40_c6": 113774485586118,
		"121562380192901":      121562380192901,
	}
	for k, v := range d {
		n, err := bufferParser(k, "0")
		if err != nil || n != v {
			t.Errorf("Error parsing %s: %v", k, err)
		}
	}
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
		if getFileIndex(c.path) != c.index {
			t.Error(c.path, "should be", c.index, "instead", getFileIndex(c.path))
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

	output := NewFileOutput(name, &FileOutputConfig{append: false, flushInterval: time.Minute, queueLimit: 2})

	output.Write([]byte("1 1 1\r\ntest"))
	name1 := output.file.Name()

	output.Write([]byte("1 1 1\r\ntest"))
	name2 := output.file.Name()

	output.updateName()

	output.Write([]byte("1 1 1\r\ntest"))
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

	output := NewFileOutput(name, &FileOutputConfig{append: false, flushInterval: time.Minute, queueLimit: 3})

	output.Write([]byte("1 1 1\r\ntest"))
	name1 := output.file.Name()

	output.Write([]byte("1 1 1\r\ntest"))
	name2 := output.file.Name()

	output.updateName()

	output.Write([]byte("1 1 1\r\ntest"))
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

	output := NewFileOutput(name, &FileOutputConfig{append: false, flushInterval: time.Minute, queueLimit: 2})

	output.Write([]byte("1 1 1\r\ntest"))
	name1 := output.file.Name()

	output.Write([]byte("1 1 1\r\ntest"))
	name2 := output.file.Name()

	output.updateName()

	output.Write([]byte("1 1 1\r\ntest"))
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

	messageSize := len(message) + len(payloadSeparator)

	output := NewFileOutput(name, &FileOutputConfig{append: false, flushInterval: time.Minute, sizeLimit: 2 * int64(messageSize)})

	output.Write([]byte("1 1 1\r\ntest"))
	name1 := output.file.Name()

	output.Write([]byte("1 1 1\r\ntest"))
	name2 := output.file.Name()

	output.flush()
	output.updateName()

	output.Write([]byte("1 1 1\r\ntest"))
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
