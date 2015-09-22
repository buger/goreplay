package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testRawExpire = time.Millisecond * 200

func TestRAWInput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer origin.Close()
	originAddr := strings.Replace(origin.Listener.Addr().String(), "[::]", "127.0.0.1", -1)

	var respCounter, reqCounter int64

	input := NewRAWInput(originAddr, testRawExpire)
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

	testPlugins(input, output)

	client := NewHTTPClient(origin.URL, &HTTPClientConfig{})

	time.Sleep(time.Millisecond)

	go Start(quit)

	for i := 0; i < 100; i++ {
		// request + response
		wg.Add(2)
		client.Get("/")
	}

	wg.Wait()
	close(quit)
}

func TestInputRAW100Expect(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	fileContent, _ := ioutil.ReadFile("README.md")

	// Origing and Replay server initialization
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ioutil.ReadAll(r.Body)

		wg.Done()
	}))
	defer origin.Close()

	originAddr := strings.Replace(origin.Listener.Addr().String(), "[::]", "127.0.0.1", -1)

	input := NewRAWInput(originAddr, time.Second)
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

	testPlugins(input, testOutput, httpOutput)

	go Start(quit)

	// Origin + Response/Request Test Output + Request Http Output
	wg.Add(4)
	curl := exec.Command("curl", "http://"+originAddr, "--data-binary", "@README.md")
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
	input := NewRAWInput(originAddr, time.Second)
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

	testPlugins(input, httpOutput)

	go Start(quit)

	wg.Add(2)

	curl := exec.Command("curl", "http://"+originAddr, "--header", "Transfer-Encoding: chunked", "--data-binary", "@README.md")
	err := curl.Run()
	if err != nil {
		log.Fatal(err)
	}

	wg.Wait()

	close(quit)
}

func TestInputRAWLargePayload(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	// Generate 200kb file
	dd := exec.Command("dd", "if=/dev/urandom", "of=/tmp/large", "bs=1KB", "count=100")
	err := dd.Run()
	if err != nil {
		log.Fatal("dd error:", err)
	}

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		body, _ := ioutil.ReadAll(req.Body)

		if len(body) != 100*1000 {
			t.Error("File size should be 1mb:", len(body))
		}

		wg.Done()
	}))
	originAddr := strings.Replace(origin.Listener.Addr().String(), "[::]", "127.0.0.1", -1)

	input := NewRAWInput(originAddr, time.Second)
	defer input.Close()

	replay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, 1*1024*1024)
		buf := make([]byte, 1*1024*1024)
		n, _ := req.Body.Read(buf)
		body := buf[0:n]

		if len(body) != 100*1000 {
			t.Error("File size should be 100000 bytes:", len(body))
		}

		wg.Done()
	}))
	defer replay.Close()

	httpOutput := NewHTTPOutput(replay.URL, &HTTPOutputConfig{Debug: false})

	testPlugins(input, httpOutput)

	go Start(quit)

	wg.Add(2)
	curl := exec.Command("curl", "http://"+originAddr, "--header", "Transfer-Encoding: chunked", "--data-binary", "@/tmp/large")
	err = curl.Run()
	if err != nil {
		log.Fatal("curl error:", err)
	}

	wg.Wait()
	close(quit)
}
