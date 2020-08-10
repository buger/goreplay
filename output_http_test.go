package main

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	_ "net/http/httputil"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPOutput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("User-Agent") != "Gor" {
			t.Error("Wrong header")
		}

		if req.Method == "OPTIONS" {
			t.Error("Wrong method")
		}

		if req.Method == "POST" {
			defer req.Body.Close()
			body, _ := ioutil.ReadAll(req.Body)

			if string(body) != "a=1&b=2" {
				t.Error("Wrong POST body:", string(body))
			}
		}

		wg.Done()
	}))
	defer server.Close()

	headers := HTTPHeaders{HTTPHeader{"User-Agent", "Gor"}}
	methods := HTTPMethods{[]byte("GET"), []byte("PUT"), []byte("POST")}
	Settings.setModifierConfig(HTTPModifierConfig{Headers: headers, Methods: methods})

	http_output := NewHTTPOutput(server.URL, &HTTPOutputConfig{Debug: true, TrackResponses: true})
	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{http_output, output},
	}
	plugins.All = append(plugins.All, input, output, http_output)

	emitter := NewEmitter(quit)
	go emitter.Start(plugins, Settings.middleware())

	for i := 0; i < 10; i++ {
		// 2 http-output, 2 - test output request, 2 - test output http response
		wg.Add(6) // OPTIONS should be ignored
		input.EmitPOST()
		input.EmitOPTIONS()
		input.EmitGET()
	}

	wg.Wait()
	close(quit)

	activeWorkers := atomic.LoadInt64(&http_output.(*HTTPOutput).activeWorkers)

	if activeWorkers < 50 {
		t.Error("Should create workers for each request", activeWorkers)
	}

	Settings.setModifierConfig(HTTPModifierConfig{})
}

func TestHTTPOutputKeepOriginalHost(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Host != "custom-host.com" {
			t.Error("Wrong header", req.Host)
		}

		wg.Done()
	}))
	defer server.Close()

	headers := HTTPHeaders{HTTPHeader{"Host", "custom-host.com"}}
	Settings.setModifierConfig(HTTPModifierConfig{Headers: headers})

	output := NewHTTPOutput(server.URL, &HTTPOutputConfig{Debug: false, OriginalHost: true})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := NewEmitter(quit)
	go emitter.Start(plugins, Settings.middleware())

	wg.Add(1)
	input.EmitGET()

	wg.Wait()
	emitter.Close()
	Settings.setModifierConfig(HTTPModifierConfig{})
}

func TestHTTPOutputSSL(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	// Origing and Replay server initialization
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Done()
	}))

	input := NewTestInput()
	output := NewHTTPOutput(server.URL, &HTTPOutputConfig{})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := NewEmitter(quit)
	go emitter.Start(plugins, Settings.middleware())

	wg.Add(2)

	input.EmitPOST()
	input.EmitGET()

	wg.Wait()
	emitter.Close()
}

func TestHTTPOutputSessions(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()
	input.skipHeader = true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wg.Done()
	}))
	defer server.Close()

	Settings.setRecognizeTCPSessions(true)
	output := NewHTTPOutput(server.URL, &HTTPOutputConfig{Debug: true})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}
	emitter := NewEmitter(quit)
	go emitter.Start(plugins, Settings.middleware())

	uuid1 := []byte("1234567890123456789a0000")
	uuid2 := []byte("1234567890123456789d0000")

	for i := 0; i < 100; i++ {
		wg.Add(1) // OPTIONS should be ignored
		copy(uuid1[20:], randByte(4))
		input.EmitBytes([]byte("1 " + string(uuid1) + " 1\n" + "GET / HTTP/1.1\r\n\r\n"))
	}

	for i := 0; i < 100; i++ {
		wg.Add(1) // OPTIONS should be ignored
		copy(uuid2[20:], randByte(4))
		input.EmitBytes([]byte("1 " + string(uuid2) + " 1\n" + "GET / HTTP/1.1\r\n\r\n"))
	}

	wg.Wait()

	if output.(*HTTPOutput).activeWorkers != 2 {
		t.Error("Should have only 2 workers", output.(*HTTPOutput).activeWorkers)
	}

	emitter.Close()

	Settings.setRecognizeTCPSessions(false)
}

func BenchmarkHTTPOutput(b *testing.B) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		wg.Done()
	}))
	defer server.Close()

	input := NewTestInput()
	output := NewHTTPOutput(server.URL, &HTTPOutputConfig{})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := NewEmitter(quit)
	go emitter.Start(plugins, Settings.middleware())

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		input.EmitPOST()
	}

	wg.Wait()
	emitter.Close()
}
