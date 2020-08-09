package main

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buger/goreplay/proto"
)

func TestHTTPInput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewHTTPInput("127.0.0.1:0")
	time.Sleep(time.Millisecond)
	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := NewEmitter(quit)
	go emitter.Start(plugins, Settings.Middleware)

	address := strings.Replace(input.address, "[::]", "127.0.0.1", -1)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		http.Get("http://" + address + "/")
	}

	wg.Wait()
	emitter.Close()
}

func TestInputHTTPLargePayload(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int, 1)
	const n = 10 << 20 // 10MB
	var large [n]byte
	large[n-1] = '0'

	input := NewHTTPInput("127.0.0.1:0")
	time.Sleep(time.Millisecond)
	output := NewTestOutput(func(data []byte) {
		_len := len(proto.Body(payloadBody(data)))
		if _len >= n { // considering http body CRLF
			t.Errorf("expected body to be >= %d", n)
		}
		wg.Done()
	})
	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := NewEmitter(quit)
	defer emitter.Close()
	go emitter.Start(plugins, Settings.Middleware)

	address := strings.Replace(input.address, "[::]", "127.0.0.1", -1)
	var req *http.Request
	var err error
	req, err = http.NewRequest("POST", "http://"+address, bytes.NewBuffer(large[:]))
	if err != nil {
		t.Error(err)
		return
	}
	wg.Add(1)
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
		return
	}
	wg.Wait()
}
