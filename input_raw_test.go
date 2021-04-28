package main

import (
	"bytes"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buger/goreplay/capture"
	"github.com/buger/goreplay/proto"
)

const testRawExpire = time.Millisecond * 200

func TestRAWInputIPv4(t *testing.T) {
	wg := new(sync.WaitGroup)

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
		return
	}
	origin := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ab"))
		}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go origin.Serve(listener)
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())

	var respCounter, reqCounter int64
	conf := RAWInputConfig{
		Engine:        capture.EnginePcap,
		Expire:        0,
		Protocol:      ProtocolHTTP,
		TrackResponse: true,
		RealIPHeader:  "X-Real-IP",
	}
	input := NewRAWInput(listener.Addr().String(), conf)

	output := NewTestOutput(func(msg *Message) {
		if msg.Meta[0] == '1' {
			if len(proto.Header(msg.Data, []byte("X-Real-IP"))) == 0 {
				t.Error("Should have X-Real-IP header")
			}
			reqCounter++
		} else {
			respCounter++
		}
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	addr := "http://127.0.0.1:" + port
	emitter := NewEmitter()
	defer emitter.Close()
	go emitter.Start(plugins, Settings.Middleware)
	for i := 0; i < 1; i++ {
		wg.Add(2)
		_, err = http.Get(addr)
		if err != nil {
			t.Error(err)
			return
		}
	}
	wg.Wait()
	const want = 10
	if reqCounter != respCounter && reqCounter != want {
		t.Errorf("want %d requests and %d responses, got %d requests and %d responses", want, want, reqCounter, respCounter)
	}
}

func TestRAWInputNoKeepAlive(t *testing.T) {
	wg := new(sync.WaitGroup)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	origin := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ab"))
		}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	origin.SetKeepAlivesEnabled(false)
	go origin.Serve(listener)
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())

	conf := RAWInputConfig{
		Engine:        capture.EnginePcap,
		Expire:        testRawExpire,
		Protocol:      ProtocolHTTP,
		TrackResponse: true,
	}
	input := NewRAWInput(":"+port, conf)
	var respCounter, reqCounter int64
	output := NewTestOutput(func(msg *Message) {
		if msg.Meta[0] == '1' {
			atomic.AddInt64(&reqCounter, 1)
		} else {
			atomic.AddInt64(&respCounter, 1)
		}
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	addr := "http://127.0.0.1:" + port

	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)

	for i := 0; i < 10; i++ {
		// request + response
		wg.Add(2)
		_, err = http.Get(addr)
		if err != nil {
			t.Error(err)
			return
		}
	}

	wg.Wait()
	const want = 10
	if reqCounter != respCounter && reqCounter != want {
		t.Errorf("want %d requests and %d responses, got %d requests and %d responses", want, want, reqCounter, respCounter)
	}
	emitter.Close()
}

func TestRAWInputIPv6(t *testing.T) {
	wg := new(sync.WaitGroup)

	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		return
	}
	origin := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ab"))
		}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go origin.Serve(listener)
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	originAddr := "[::1]:" + port

	var respCounter, reqCounter int64
	conf := RAWInputConfig{
		Engine:        capture.EnginePcap,
		Protocol:      ProtocolHTTP,
		TrackResponse: true,
	}
	input := NewRAWInput(originAddr, conf)

	output := NewTestOutput(func(msg *Message) {
		if msg.Meta[0] == '1' {
			atomic.AddInt64(&reqCounter, 1)
		} else {
			atomic.AddInt64(&respCounter, 1)
		}
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}

	emitter := NewEmitter()
	addr := "http://" + originAddr
	go emitter.Start(plugins, Settings.Middleware)
	for i := 0; i < 10; i++ {
		// request + response
		wg.Add(2)
		_, err = http.Get(addr)
		if err != nil {
			t.Error(err)
			return
		}
	}

	wg.Wait()
	const want = 10
	if reqCounter != respCounter && reqCounter != want {
		t.Errorf("want %d requests and %d responses, got %d requests and %d responses", want, want, reqCounter, respCounter)
	}
	emitter.Close()
}

func TestInputRAWChunkedEncoding(t *testing.T) {
	wg := new(sync.WaitGroup)

	fileContent, _ := ioutil.ReadFile("README.md")

	// Origing and Replay server initialization
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ioutil.ReadAll(r.Body)

		wg.Done()
	}))

	originAddr := strings.Replace(origin.Listener.Addr().String(), "[::]", "127.0.0.1", -1)
	conf := RAWInputConfig{
		Engine:        capture.EnginePcap,
		Expire:        time.Second,
		Protocol:      ProtocolHTTP,
		TrackResponse: true,
	}
	input := NewRAWInput(originAddr, conf)

	replay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := ioutil.ReadAll(r.Body)

		if !bytes.Equal(body, fileContent) {
			buf, _ := httputil.DumpRequest(r, true)
			t.Error("Wrong POST body:", string(buf))
		}

		wg.Done()
	}))
	defer replay.Close()

	httpOutput := NewHTTPOutput(replay.URL, &HTTPOutputConfig{})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{httpOutput},
	}
	plugins.All = append(plugins.All, input, httpOutput)

	emitter := NewEmitter()
	defer emitter.Close()
	go emitter.Start(plugins, Settings.Middleware)
	wg.Add(2)

	curl := exec.Command("curl", "http://"+originAddr, "--header", "Transfer-Encoding: chunked", "--header", "Expect:", "--data-binary", "@README.md")
	err := curl.Run()
	if err != nil {
		t.Error(err)
		return
	}

	wg.Wait()
}

func BenchmarkRAWInputWithReplay(b *testing.B) {
	var respCounter, reqCounter, replayCounter uint32
	wg := &sync.WaitGroup{}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		b.Error(err)
		return
	}
	listener0, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		b.Error(err)
		return
	}

	origin := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ab"))
		}),
	}
	go origin.Serve(listener)
	defer origin.Close()
	originAddr := listener.Addr().String()

	replay := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddUint32(&replayCounter, 1)
			w.Write(nil)
			wg.Done()
		}),
	}
	go replay.Serve(listener0)
	defer replay.Close()
	replayAddr := listener0.Addr().String()

	conf := RAWInputConfig{
		Engine:        capture.EnginePcap,
		Expire:        testRawExpire,
		Protocol:      ProtocolHTTP,
		TrackResponse: true,
	}
	input := NewRAWInput(originAddr, conf)

	testOutput := NewTestOutput(func(msg *Message) {
		if msg.Meta[0] == '1' {
			reqCounter++
		} else {
			respCounter++
		}
		wg.Done()
	})
	httpOutput := NewHTTPOutput("http://"+replayAddr, &HTTPOutputConfig{})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{testOutput, httpOutput},
	}

	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)
	addr := "http://" + originAddr
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(3) // reqCounter + replayCounter + respCounter
		resp, err := http.Get(addr)
		if err != nil {
			wg.Add(-3)
		}
		resp.Body.Close()
	}

	wg.Wait()
	b.ReportMetric(float64(reqCounter), "requests")
	b.ReportMetric(float64(respCounter), "responses")
	b.ReportMetric(float64(replayCounter), "replayed")
	emitter.Close()
}
