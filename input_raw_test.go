package gor

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestRAWInput(t *testing.T) {

	wg := new(sync.WaitGroup)
	quit := make(chan int)

	listener := startHTTP(func(req *http.Request) {})

	input := NewRAWInput(listener.Addr().String())
	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})

	Plugins.Inputs = []Input{input}
	Plugins.Outputs = []io.Writer{output}

	address := strings.Replace(listener.Addr().String(), "[::]", "127.0.0.1", -1)

	go Start(quit)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		res, _ := http.Get("http://" + address)
		res.Body.Close()
	}

	wg.Wait()

	close(quit)
}
