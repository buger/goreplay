package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileOutput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()
	output := NewFileOutput("/tmp/test_requests.gor", time.Minute)

	Plugins.Inputs = []io.Reader{input}
	Plugins.Outputs = []io.Writer{output}

	go Start(quit)

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

	Plugins.Inputs = []io.Reader{input2}
	Plugins.Outputs = []io.Writer{output2}

	go Start(quit)

	wg.Wait()
	close(quit)
}

func TestFileOutputPathTemplate(t *testing.T) {
	output := &FileOutput{pathTemplate: "/tmp/log-%Y-%m-%d-%S"}
	now := time.Now()
	expectedPath := fmt.Sprintf("/tmp/log-%s-%s-%s-%s", now.Format("2006"), now.Format("01"), now.Format("02"), now.Format("05"))
	path := output.filename()

	if expectedPath != path {
		t.Errorf("Expected path %s but got %s", expectedPath, path)
	}
}

func TestFileOutputMultipleFiles(t *testing.T) {
	output := NewFileOutput("/tmp/log-%Y-%m-%d-%S", time.Minute)

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

func TestFileOutputCompression(t *testing.T) {
	output := NewFileOutput("/tmp/log-%Y-%m-%d-%S.gz", time.Minute)

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
