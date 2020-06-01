package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

var _ = log.Println

func TestInputFileWithGET(t *testing.T) {

	input := NewTestInput()
	rg := NewRequestGenerator([]io.Reader{input}, func() { input.EmitGET() }, 1)
	readPayloads := [][]byte{}

	// Given a capture file with a GET request
	expectedCaptureFile := CreateCaptureFile(rg)
	defer expectedCaptureFile.TearDown()

	// When the request is read from the capture file
	err := ReadFromCaptureFile(expectedCaptureFile.file, 1, func(data []byte) {
		readPayloads = append(readPayloads, Duplicate(data))
	})

	// The read request should match the original request
	if err != nil {
		t.Error(err)
	} else {
		if !expectedCaptureFile.PayloadsEqual(readPayloads) {
			t.Error("Request read back from file should match")
		}
	}
}

func TestInputFileWithPayloadLargerThan64Kb(t *testing.T) {

	input := NewTestInput()
	rg := NewRequestGenerator([]io.Reader{input}, func() { input.EmitSizedPOST(64 * 1024) }, 1)
	readPayloads := [][]byte{}

	// Given a capture file with a request over 64Kb
	expectedCaptureFile := CreateCaptureFile(rg)
	defer expectedCaptureFile.TearDown()

	// When the request is read from the capture file
	err := ReadFromCaptureFile(expectedCaptureFile.file, 1, func(data []byte) {
		readPayloads = append(readPayloads, Duplicate(data))
	})

	// The read request should match the original request
	if err != nil {
		t.Error(err)
	} else {
		if !expectedCaptureFile.PayloadsEqual(readPayloads) {
			t.Error("Request read back from file should match")
		}
	}

}

func TestInputFileWithGETAndPOST(t *testing.T) {

	input := NewTestInput()
	rg := NewRequestGenerator([]io.Reader{input}, func() {
		input.EmitGET()
		input.EmitPOST()
	}, 2)
	readPayloads := [][]byte{}

	// Given a capture file with a GET request
	expectedCaptureFile := CreateCaptureFile(rg)
	defer expectedCaptureFile.TearDown()

	// When the requests are read from the capture file
	err := ReadFromCaptureFile(expectedCaptureFile.file, 2, func(data []byte) {
		readPayloads = append(readPayloads, Duplicate(data))
	})

	// The read requests should match the original request
	if err != nil {
		t.Error(err)
	} else {
		if !expectedCaptureFile.PayloadsEqual(readPayloads) {
			t.Error("Request read back from file should match")
		}
	}

}

func TestInputFileMultipleFilesWithRequestsOnly(t *testing.T) {
	rnd := rand.Int63()

	file1, _ := os.OpenFile(fmt.Sprintf("/tmp/%d_0", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	file1.Write([]byte("1 1 1\ntest1"))
	file1.Write([]byte(payloadSeparator))
	file1.Write([]byte("1 1 3\ntest2"))
	file1.Write([]byte(payloadSeparator))
	file1.Close()

	file2, _ := os.OpenFile(fmt.Sprintf("/tmp/%d_1", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	file2.Write([]byte("1 1 2\ntest3"))
	file2.Write([]byte(payloadSeparator))
	file2.Write([]byte("1 1 4\ntest4"))
	file2.Write([]byte(payloadSeparator))
	file2.Close()

	input := NewFileInput(fmt.Sprintf("/tmp/%d*", rnd), false)
	buf := make([]byte, 1000)

	for i := '1'; i <= '4'; i++ {
		n, _ := input.Read(buf)
		if buf[4] != byte(i) {
			t.Error("Should emit requests in right order", string(buf[:n]))
		}
	}

	os.Remove(file1.Name())
	os.Remove(file2.Name())
}

func TestInputFileRequestsWithLatency(t *testing.T) {
	rnd := rand.Int63()

	file, _ := os.OpenFile(fmt.Sprintf("/tmp/%d", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	defer file.Close()

	file.Write([]byte("1 1 100000000\nrequest1"))
	file.Write([]byte(payloadSeparator))
	file.Write([]byte("1 2 150000000\nrequest2"))
	file.Write([]byte(payloadSeparator))
	file.Write([]byte("1 3 250000000\nrequest3"))
	file.Write([]byte(payloadSeparator))

	input := NewFileInput(fmt.Sprintf("/tmp/%d", rnd), false)
	buf := make([]byte, 1000)

	start := time.Now().UnixNano()
	for i := 0; i < 3; i++ {
		input.Read(buf)
	}
	end := time.Now().UnixNano()

	var expectedLatency int64 = 250000000 - 100000000
	realLatency := end - start
	if realLatency < expectedLatency {
		t.Errorf("Should emit requests respecting latency. Expected: %v, real: %v", expectedLatency, realLatency)
	}

	if realLatency > expectedLatency+10000000 {
		t.Errorf("Should emit requests respecting latency. Expected: %v, real: %v", expectedLatency, realLatency)

	}
}

func TestInputFileMultipleFilesWithRequestsAndResponses(t *testing.T) {
	rnd := rand.Int63()

	file1, _ := os.OpenFile(fmt.Sprintf("/tmp/%d_0", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	file1.Write([]byte("1 1 1\nrequest1"))
	file1.Write([]byte(payloadSeparator))
	file1.Write([]byte("2 1 1\nresponse1"))
	file1.Write([]byte(payloadSeparator))
	file1.Write([]byte("1 2 3\nrequest2"))
	file1.Write([]byte(payloadSeparator))
	file1.Write([]byte("2 2 3\nresponse2"))
	file1.Write([]byte(payloadSeparator))
	file1.Close()

	file2, _ := os.OpenFile(fmt.Sprintf("/tmp/%d_1", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	file2.Write([]byte("1 3 2\nrequest3"))
	file2.Write([]byte(payloadSeparator))
	file2.Write([]byte("2 3 2\nresponse3"))
	file2.Write([]byte(payloadSeparator))
	file2.Write([]byte("1 4 4\nrequest4"))
	file2.Write([]byte(payloadSeparator))
	file2.Write([]byte("2 4 4\nresponse4"))
	file2.Write([]byte(payloadSeparator))
	file2.Close()

	input := NewFileInput(fmt.Sprintf("/tmp/%d*", rnd), false)
	buf := make([]byte, 1000)

	for i := '1'; i <= '4'; i++ {
		n, _ := input.Read(buf)
		if buf[0] != '1' && buf[4] != byte(i) {
			t.Error("Shound emit requests in right order", string(buf[:n]))
		}

		n, _ = input.Read(buf)
		if buf[0] != '2' && buf[4] != byte(i) {
			t.Error("Shound emit responses in right order", string(buf[:n]))
		}
	}

	os.Remove(file1.Name())
	os.Remove(file2.Name())
}

func TestInputFileLoop(t *testing.T) {
	rnd := rand.Int63()

	file, _ := os.OpenFile(fmt.Sprintf("/tmp/%d", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	file.Write([]byte("1 1 1\ntest1"))
	file.Write([]byte(payloadSeparator))
	file.Write([]byte("1 1 2\ntest2"))
	file.Write([]byte(payloadSeparator))
	file.Close()

	input := NewFileInput(fmt.Sprintf("/tmp/%d", rnd), true)
	buf := make([]byte, 1000)

	// Even if we have just 2 requests in file, it should indifinitly loop
	for i := 0; i < 1000; i++ {
		input.Read(buf)
	}

	input.Close()
	os.Remove(file.Name())
}

func TestInputFileCompressed(t *testing.T) {
	rnd := rand.Int63()

	output := NewFileOutput(fmt.Sprintf("/tmp/%d_0.gz", rnd), &FileOutputConfig{flushInterval: time.Minute, append: true})
	for i := 0; i < 1000; i++ {
		output.Write([]byte("1 1 1\r\ntest"))
	}
	name1 := output.file.Name()
	output.Close()

	output2 := NewFileOutput(fmt.Sprintf("/tmp/%d_1.gz", rnd), &FileOutputConfig{flushInterval: time.Minute, append: true})
	for i := 0; i < 1000; i++ {
		output2.Write([]byte("1 1 1\r\ntest"))
	}
	name2 := output2.file.Name()
	output2.Close()

	input := NewFileInput(fmt.Sprintf("/tmp/%d*", rnd), false)
	buf := make([]byte, 1000)
	for i := 0; i < 2000; i++ {
		input.Read(buf)
	}

	os.Remove(name1)
	os.Remove(name2)
}

type CaptureFile struct {
	data [][]byte
	file *os.File
}

func NewExpectedCaptureFile(data [][]byte, file *os.File) *CaptureFile {
	ecf := new(CaptureFile)
	ecf.file = file
	ecf.data = data
	return ecf
}

func (expectedCaptureFile *CaptureFile) TearDown() {
	if expectedCaptureFile.file != nil {
		syscall.Unlink(expectedCaptureFile.file.Name())
	}
}

type RequestGenerator struct {
	inputs []io.Reader
	emit   func()
	wg     *sync.WaitGroup
}

func NewRequestGenerator(inputs []io.Reader, emit func(), count int) (rg *RequestGenerator) {
	rg = new(RequestGenerator)
	rg.inputs = inputs
	rg.emit = emit
	rg.wg = new(sync.WaitGroup)
	rg.wg.Add(count)
	return
}

func (expectedCaptureFile *CaptureFile) PayloadsEqual(other [][]byte) bool {

	if len(expectedCaptureFile.data) != len(other) {
		return false
	}

	for i, payload := range other {
		if !bytes.Equal(expectedCaptureFile.data[i], payload) {
			return false
		}
	}

	return true

}

func CreateCaptureFile(requestGenerator *RequestGenerator) *CaptureFile {

	f, err := ioutil.TempFile("", "testmainconf")
	if err != nil {
		panic(err)
	}

	quit := make(chan int)

	readPayloads := [][]byte{}
	output := NewTestOutput(func(data []byte) {
		readPayloads = append(readPayloads, Duplicate(data))

		requestGenerator.wg.Done()
	})

	outputFile := NewFileOutput(f.Name(), &FileOutputConfig{flushInterval: time.Minute, append: true})

	plugins := &InOutPlugins{
		Inputs:  requestGenerator.inputs,
		Outputs: []io.Writer{output, outputFile},
	}

	go Start(plugins, quit)

	requestGenerator.emit()
	requestGenerator.wg.Wait()

	time.Sleep(100 * time.Millisecond)
	outputFile.Close()

	close(quit)

	return NewExpectedCaptureFile(readPayloads, f)

}

func ReadFromCaptureFile(captureFile *os.File, count int, callback writeCallback) (err error) {

	quit := make(chan int)
	wg := new(sync.WaitGroup)

	input := NewFileInput(captureFile.Name(), false)
	output := NewTestOutput(func(data []byte) {
		callback(data)
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	wg.Add(count)
	go Start(plugins, quit)

	done := make(chan int, 1)
	go func() {
		wg.Wait()
		done <- 1
	}()

	select {
	case <-done:
		break
	case <-time.After(2 * time.Second):
		err = errors.New("Timed out")
	}
	close(quit)

	return

}

func Duplicate(data []byte) (duplicate []byte) {
	duplicate = make([]byte, len(data))
	copy(duplicate, data)

	return
}
