package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buger/goreplay/proto"
)

const testRawExpire = time.Millisecond * 200

func TestRAWInputIPv4(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	origin := &http.Server{
		Handler:      http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go origin.Serve(listener)
	defer listener.Close()

	originAddr := listener.Addr().String()

	var respCounter, reqCounter int64

	input := NewRAWInput(originAddr, EnginePcap, true, testRawExpire, "X-Real-IP", "", "", 0)
	defer input.Close()

	output := NewTestOutput(func(data []byte) {
		if data[0] == '1' {
			body := payloadBody(data)
			if len(proto.Header(body, []byte("X-Real-IP"))) == 0 {
				t.Error("Should have X-Real-IP header", string(body))
			}
			atomic.AddInt64(&reqCounter, 1)
		} else {
			atomic.AddInt64(&respCounter, 1)
		}

		if Settings.debug {
			log.Println(reqCounter, respCounter)
		}

		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	client := NewHTTPClient("http://"+listener.Addr().String(), &HTTPClientConfig{})

	go Start(plugins, quit)

	for i := 0; i < 100; i++ {
		// request + response
		wg.Add(2)
		client.Get("/")
		time.Sleep(2 * time.Millisecond)
	}

	wg.Wait()

	close(quit)
}

func TestRAWInputNoKeepAlive(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	origin := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("a"))
			w.Write([]byte("b"))
		}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	origin.SetKeepAlivesEnabled(false)
	go origin.Serve(listener)
	defer listener.Close()

	originAddr := listener.Addr().String()

	input := NewRAWInput(originAddr, EnginePcap, true, testRawExpire, "", "", "", 0)
	defer input.Close()

	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	client := NewHTTPClient("http://"+listener.Addr().String(), &HTTPClientConfig{})

	go Start(plugins, quit)

	for i := 0; i < 100; i++ {
		// request + response
		wg.Add(2)
		client.Get("/")
		time.Sleep(2 * time.Millisecond)
	}

	wg.Wait()

	close(quit)
}

func TestRAWInputIPv6(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Fatal(err)
	}
	origin := &http.Server{
		Handler:      http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go origin.Serve(listener)
	defer listener.Close()

	originAddr := listener.Addr().String()

	var respCounter, reqCounter int64

	input := NewRAWInput(originAddr, EnginePcap, true, testRawExpire, "", "", "", 0)
	defer input.Close()

	output := NewTestOutput(func(data []byte) {
		if data[0] == '1' {
			atomic.AddInt64(&reqCounter, 1)
		} else {
			atomic.AddInt64(&respCounter, 1)
		}

		if Settings.debug {
			log.Println(reqCounter, respCounter)
		}

		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	client := NewHTTPClient("http://"+listener.Addr().String(), &HTTPClientConfig{})

	go Start(plugins, quit)

	for i := 0; i < 100; i++ {
		// request + response
		wg.Add(2)
		client.Get("/")
		time.Sleep(2 * time.Millisecond)
	}

	wg.Wait()
	close(quit)
}

func TestInputRAW100Expect(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	fileContent, _ := ioutil.ReadFile("COMM-LICENSE")

	// Origing and Replay server initialization
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ioutil.ReadAll(r.Body)
		wg.Done()
	}))
	defer origin.Close()

	originAddr := strings.Replace(origin.Listener.Addr().String(), "[::]", "127.0.0.1", -1)

	input := NewRAWInput(originAddr, EnginePcap, true, time.Second, "", "", "", 0)
	defer input.Close()

	// We will use it to get content of raw HTTP request
	testOutput := NewTestOutput(func(data []byte) {
		switch data[0] {
		case RequestPayload:
			if strings.Contains(string(data), "Expect: 100-continue") {
				t.Error("Should not contain 100-continue header")
			}
			wg.Done()
		case ResponsePayload:
			wg.Done()
		}
	})

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
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{testOutput, httpOutput},
	}

	go Start(plugins, quit)

	// Origin + Response/Request Test Output + Request Http Output
	wg.Add(4)
	curl := exec.Command("curl", "http://"+originAddr, "--data-binary", "@COMM-LICENSE")
	err := curl.Run()
	if err != nil {
		log.Fatal(err)
	}

	wg.Wait()
	close(quit)
}

func TestInputRAWChunkedEncoding(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	fileContent, _ := ioutil.ReadFile("README.md")

	// Origing and Replay server initialization
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ioutil.ReadAll(r.Body)

		wg.Done()
	}))

	originAddr := strings.Replace(origin.Listener.Addr().String(), "[::]", "127.0.0.1", -1)
	input := NewRAWInput(originAddr, EnginePcap, true, time.Second, "", "", "", 0)
	defer input.Close()

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

	httpOutput := NewHTTPOutput(replay.URL, &HTTPOutputConfig{Debug: true})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{httpOutput},
	}

	go Start(plugins, quit)

	wg.Add(2)

	curl := exec.Command("curl", "http://"+originAddr, "--header", "Transfer-Encoding: chunked", "--header", "Expect:", "--data-binary", "@README.md")
	err := curl.Run()
	if err != nil {
		log.Fatal(err)
	}

	wg.Wait()

	close(quit)
}

func TestInputRAWLargePayload(t *testing.T) {
	// FIXME: Large payloads does not work for travis for some reason...
	if os.Getenv("TRAVIS_BUILD_DIR") != "" {
		return
	}
	wg := new(sync.WaitGroup)
	quit := make(chan int)
	sizeB := 100 * 1000

	// Generate 100kb file
	dd := exec.Command("dd", "if=/dev/urandom", "of=/tmp/large", "bs=1", "count="+strconv.Itoa(sizeB))
	err := dd.Run()
	if err != nil {
		log.Fatal("dd error:", err)
	}

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		body, _ := ioutil.ReadAll(req.Body)

		if len(body) != sizeB {
			t.Error("File size should be 1mb:", len(body))
		}

		wg.Done()
	}))
	originAddr := strings.Replace(origin.Listener.Addr().String(), "[::]", "127.0.0.1", -1)

	input := NewRAWInput(originAddr, EnginePcap, true, testRawExpire, "", "", "", 0)
	defer input.Close()

	replay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := ioutil.ReadAll(req.Body)
		// // req.Body = http.MaxBytesReader(w, req.Body, 1*1024*1024)
		// // buf := make([]byte, 1*1024*1024)
		// n, _ := req.Body.Read(buf)
		// body := buf[0:n]

		if len(body) != sizeB {
			t.Errorf("File size should be %d bytes: %d", sizeB, len(body))
		}

		wg.Done()
	}))
	defer replay.Close()

	httpOutput := NewHTTPOutput(replay.URL, &HTTPOutputConfig{Debug: false})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{httpOutput},
	}

	go Start(plugins, quit)

	wg.Add(2)
	curl := exec.Command("curl", "http://"+originAddr, "--header", "Transfer-Encoding: chunked", "--header", "Expect:", "--data-binary", "@/tmp/large")
	err = curl.Run()
	if err != nil {
		log.Fatal("curl error:", err)
	}

	wg.Wait()
	close(quit)
}

func BenchmarkRAWInput(b *testing.B) {
	var respCounter, reqCounter, replayCounter int64

	quit := make(chan int)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer origin.Close()
	originAddr := strings.Replace(origin.Listener.Addr().String(), "[::]", "127.0.0.1", -1)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&replayCounter, 1)
	}))
	defer origin.Close()
	upstreamAddr := strings.Replace(upstream.Listener.Addr().String(), "[::]", "127.0.0.1", -1)

	input := NewRAWInput(originAddr, EnginePcap, true, testRawExpire, "", "", "", 0)
	defer input.Close()

	output := NewTestOutput(func(data []byte) {
		if data[0] == '1' {
			atomic.AddInt64(&reqCounter, 1)
		} else {
			atomic.AddInt64(&respCounter, 1)
		}
	})

	httpOutput := NewLimiter(NewHTTPOutput(upstreamAddr, &HTTPOutputConfig{}), "10%")

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output, httpOutput},
	}

	go Start(plugins, quit)

	emitted := 0
	fileContent, _ := ioutil.ReadFile("LICENSE.txt")

	time.Sleep(400 * time.Millisecond)

	for i := 0; i < b.N; i++ {
		wg := new(sync.WaitGroup)
		wg.Add(100 * 1000)
		emitted += 100 * 1000
		for w := 0; w < 100; w++ {
			go func() {
				client := NewHTTPClient(origin.URL, &HTTPClientConfig{})
				for i := 0; i < 1000; i++ {
					if i%2 == 0 {
						client.Post("/", fileContent)
					} else {
						client.Get("/")
					}
					time.Sleep(time.Duration(rand.Int63n(50)) * time.Millisecond)
					wg.Done()
				}
			}()
		}
		wg.Wait()
	}

	time.Sleep(400 * time.Millisecond)
	log.Println("Emitted ", emitted, ", Captured ", reqCounter, "requests and ", respCounter, " responses", "and replayed", replayCounter)

	close(quit)
}
