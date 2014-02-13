package gor

import (
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func startHTTP(cb func(*http.Request)) net.Listener {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		go cb(r)
	})

	listener, _ := net.Listen("tcp", ":0")

	go http.Serve(listener, handler)

	return listener
}

func TestHTTPOutput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()

	headers := HTTPHeaders{HTTPHeader{"User-Agent", "Gor"}}
	methods := HTTPMethods{"GET", "PUT", "POST"}

	listener := startHTTP(func(req *http.Request) {
		if req.Header.Get("User-Agent") != "Gor" {
			t.Error("Wrong header")
		}

		if req.Method == "OPTIONS" {
			t.Error("Wrong method")
		}

		wg.Done()
	})

	output := NewHTTPOutput(listener.Addr().String(), headers, methods, "")

	Plugins.Inputs = []Input{input}
	Plugins.Outputs = []io.Writer{output}

	go Start(quit)

	for i := 0; i < 100; i++ {
		wg.Add(2)
		input.EmitPOST()
		input.EmitOPTIONS()
		input.EmitGET()
	}

	wg.Wait()

	close(quit)
}

func BenchmarkHTTPOutput(b *testing.B) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()

	headers := HTTPHeaders{HTTPHeader{"User-Agent", "Gor"}}
	methods := HTTPMethods{"GET", "PUT", "POST"}

	listener := startHTTP(func(req *http.Request) {
		time.Sleep(50 * time.Millisecond)
		wg.Done()
	})

	output := NewHTTPOutput(listener.Addr().String(), headers, methods, "")

	Plugins.Inputs = []Input{input}
	Plugins.Outputs = []io.Writer{output}

	go Start(quit)

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		input.EmitPOST()
	}

	wg.Wait()

	close(quit)
}
