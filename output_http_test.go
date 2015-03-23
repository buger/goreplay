package main

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	_ "strings"
	"sync"
	"testing"
	"time"
)

func startHTTP(cb func(*http.Request)) net.Listener {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cb(r)
	})

	listener, _ := net.Listen("tcp", ":0")

	go http.Serve(listener, handler)

	return listener
}

func TestSetHeader(t *testing.T) {

	req := &http.Request{
		Header: make(map[string][]string),
	}
	req.Host = "test.com"

	SetHeader(req, "Host", "test2.com")

	if req.Host != "test2.com" {
		t.Error("Expected test2.com - got ", req.Host)
	}

	SetHeader(req, "test_header", "test_value")

	if req.Header.Get("test_header") != "test_value" {
		t.Error("Wrong header value found")
	}

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

		if req.Method == "POST" {
			defer req.Body.Close()
			body, _ := ioutil.ReadAll(req.Body)

			if string(body) != "xyzxyzxyz" {
				buf, _ := httputil.DumpRequest(req, true)
				t.Error("Wrong POST body:", string(buf))
			}
		}

		wg.Done()
	})

	output := NewHTTPOutput(listener.Addr().String(), headers, methods, HTTPUrlRegexp{}, HTTPHeaderFilters{}, HTTPHeaderHashFilters{}, "", UrlRewriteMap{}, 0)

	Plugins.Inputs = []io.Reader{input}
	Plugins.Outputs = []io.Writer{output}

	go Start(quit)

	for i := 0; i < 100; i++ {
		wg.Add(3)
		input.EmitPOST()
		input.EmitChunkedPOST()
		input.EmitOPTIONS() // filtered out
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

	output := NewHTTPOutput(listener.Addr().String(), headers, methods, HTTPUrlRegexp{}, HTTPHeaderFilters{}, HTTPHeaderHashFilters{}, "", UrlRewriteMap{}, 0)

	Plugins.Inputs = []io.Reader{input}
	Plugins.Outputs = []io.Writer{output}

	go Start(quit)

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		input.EmitPOST()
	}

	wg.Wait()

	close(quit)
}
