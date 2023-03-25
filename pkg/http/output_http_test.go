package http

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	_ "net/http/httputil"
	"sync"
	"testing"

	"github.com/buger/goreplay/pkg/emitter"
	"github.com/buger/goreplay/pkg/http_modifier"
	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/pro"
	"github.com/buger/goreplay/pkg/proto"
	"github.com/buger/goreplay/pkg/test"
)

func TestHTTPOutput(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := test.NewTestInput()

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

	headers := http_modifier.HTTPHeaders{http_modifier.HTTPHeader{"User-Agent", "Gor"}}
	methods := http_modifier.HTTPMethods{[]byte("GET"), []byte("PUT"), []byte("POST")}
	modifierConfig := http_modifier.HTTPModifierConfig{Headers: headers, Methods: methods}

	httpOutput := NewHTTPOutput(server.URL, &HTTPOutputConfig{TrackResponses: false})
	output := test.NewTestOutput(func(*plugin.Message) {
		wg.Done()
	})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{httpOutput, output},
	}
	plugins.All = append(plugins.All, input, output, httpOutput)

	emitter := emitter.New(&emitter.Config{
		ModifierConfig: modifierConfig,
	})
	go emitter.Start(plugins)

	for i := 0; i < 10; i++ {
		// 2 http-output, 2 - test output request
		wg.Add(4) // OPTIONS should be ignored
		input.EmitPOST()
		input.EmitOPTIONS()
		input.EmitGET()
	}

	wg.Wait()
	emitter.Close()
}

func TestHTTPOutputKeepOriginalHost(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := test.NewTestInput()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Host != "custom-host.com" {
			t.Error("Wrong header", req.Host)
		}

		wg.Done()
	}))
	defer server.Close()

	headers := http_modifier.HTTPHeaders{http_modifier.HTTPHeader{"Host", "custom-host.com"}}
	modifierConfig := http_modifier.HTTPModifierConfig{Headers: headers}

	output := NewHTTPOutput(server.URL, &HTTPOutputConfig{OriginalHost: true, SkipVerify: true})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := emitter.New(&emitter.Config{
		ModifierConfig: modifierConfig,
	})
	go emitter.Start(plugins)

	wg.Add(1)
	input.EmitGET()

	wg.Wait()
	emitter.Close()
}

func TestHTTPOutputSSL(t *testing.T) {
	wg := new(sync.WaitGroup)

	// Origing and Replay server initialization
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Done()
	}))

	input := test.NewTestInput()
	output := NewHTTPOutput(server.URL, &HTTPOutputConfig{SkipVerify: true})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := emitter.New()
	go emitter.Start(plugins)

	wg.Add(2)

	input.EmitPOST()
	input.EmitGET()

	wg.Wait()
	emitter.Close()
}

func TestHTTPOutputSessions(t *testing.T) {
	pro.Enable()

	wg := new(sync.WaitGroup)

	input := test.NewTestInput()
	input.SkipHeader = true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wg.Done()
	}))
	defer server.Close()

	output := NewHTTPOutput(server.URL, &HTTPOutputConfig{})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)
	emitter := emitter.New(&emitter.Config{
		RecognizeTCPSessions: true,
		SplitOutput:          true,
	})
	go emitter.Start(plugins)

	uuid1 := []byte("1234567890123456789a0000")
	uuid2 := []byte("1234567890123456789d0000")

	for i := 0; i < 10; i++ {
		wg.Add(1) // OPTIONS should be ignored
		copy(uuid1[20:], proto.RandByte(4))
		input.EmitBytes([]byte("1 " + string(uuid1) + " 1\n" + "GET / HTTP/1.1\r\n\r\n"))
	}

	for i := 0; i < 10; i++ {
		wg.Add(1) // OPTIONS should be ignored
		copy(uuid2[20:], proto.RandByte(4))
		input.EmitBytes([]byte("1 " + string(uuid2) + " 1\n" + "GET / HTTP/1.1\r\n\r\n"))
	}

	wg.Wait()

	emitter.Close()
}

func BenchmarkHTTPOutput(b *testing.B) {
	wg := new(sync.WaitGroup)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Done()
	}))
	defer server.Close()

	input := test.NewTestInput()
	output := NewHTTPOutput(server.URL, &HTTPOutputConfig{WorkersMax: 1})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := emitter.New()
	go emitter.Start(plugins)

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		input.EmitPOST()
	}

	wg.Wait()
	emitter.Close()
}

func BenchmarkHTTPOutputTLS(b *testing.B) {
	wg := new(sync.WaitGroup)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Done()
	}))
	defer server.Close()

	input := test.NewTestInput()
	output := NewHTTPOutput(server.URL, &HTTPOutputConfig{SkipVerify: true, WorkersMax: 1})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := emitter.New()
	go emitter.Start(plugins)

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		input.EmitPOST()
	}

	wg.Wait()
	emitter.Close()
}
