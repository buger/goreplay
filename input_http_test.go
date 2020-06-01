package main

import (
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/buger/goreplay/proto"
)

func TestHTTPInput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewHTTPInput("127.0.0.1:0")
	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	go Start(plugins, quit)

	address := strings.Replace(input.listener.Addr().String(), "[::]", "127.0.0.1", -1)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		http.Get("http://" + address + "/")
	}

	wg.Wait()

	close(quit)
}

func TestInputHTTPLargePayload(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	dd := exec.Command("dd", "if=/dev/urandom", "of=/tmp/large", "bs=1", "count=4000000")
	err := dd.Run()
	if err != nil {
		log.Fatal("dd error:", err)
	}

	input := NewHTTPInput("127.0.0.1:0")
	output := NewTestOutput(func(data []byte) {
		if len(proto.Body(payloadBody(data))) != 4000000 {
			t.Error("Should receive full file")
		}
		wg.Done()
	})
	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	go Start(plugins, quit)

	wg.Add(1)
	address := strings.Replace(input.listener.Addr().String(), "[::]", "127.0.0.1", -1)
	curl := exec.Command("curl", "http://"+address, "--data-binary", "@/tmp/large")
	err = curl.Run()
	if err != nil {
		log.Fatal("curl error:", err)
	}

	wg.Wait()
	close(quit)
}
