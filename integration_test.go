package main

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buger/gor/listener"
	"github.com/buger/gor/replay"

	"math/rand"
)

func isEqual(t *testing.T, a interface{}, b interface{}) {
	if a != b {
		t.Error("Original and Replayed request not match\n", a, "!=", b)
	}
}

var envs int

type Env struct {
	Verbose bool

	ListenHandler http.HandlerFunc
	ReplayHandler http.HandlerFunc

	ReplayLimit   int
	ListenerLimit int
	ForwardPort   int

	AdditionalHeaders replay.Headers
}

func (e *Env) start() (p int) {
	p = 50000 + envs*10

	go e.startHTTP(p, http.HandlerFunc(e.ListenHandler))
	go e.startHTTP(p+2, http.HandlerFunc(e.ReplayHandler))

	go e.startHTTP(p+3, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "OK", http.StatusAccepted)
	}))

	go e.startListener(p, p+1)
	go e.startReplay(p+1, p+2)

	// Time to start http and gor instances
	time.Sleep(time.Millisecond * 100)

	envs++

	return
}

func (e *Env) startListener(port int, replayPort int) {
	listener.Settings.Verbose = e.Verbose
	listener.Settings.Address = "127.0.0.1"
	listener.Settings.ReplayAddress = "127.0.0.1:" + strconv.Itoa(replayPort)
	listener.Settings.Port = port

	if e.ListenerLimit != 0 {
		listener.Settings.ReplayLimit = e.ListenerLimit
	}

	listener.Run()
}

func (e *Env) startReplay(port int, forwardPort int) {
	replay.Settings.Verbose = e.Verbose
	replay.Settings.Host = "127.0.0.1"
	replay.Settings.Address = "127.0.0.1:" + strconv.Itoa(port)
	replay.Settings.ForwardAddress = "127.0.0.1:" + strconv.Itoa(forwardPort)
	replay.Settings.Port = port

	if e.ReplayLimit != 0 {
		replay.Settings.ForwardAddress += "|" + strconv.Itoa(e.ReplayLimit)
	}

	if len(e.AdditionalHeaders) > 0 {
		replay.Settings.AdditionalHeaders = e.AdditionalHeaders
	}

	replay.Settings.ForwardAddress += ",127.0.0.1:" + strconv.Itoa(forwardPort+1)

	replay.Run()
}

func (e *Env) startHTTP(port int, handler http.Handler) {
	err := http.ListenAndServe(":"+strconv.Itoa(port), handler)

	if err != nil {
		fmt.Println("Error while starting http server:", err)
	}
}

func getRequest(port int) *http.Request {
	var req *http.Request

	rand.Seed(time.Now().UTC().UnixNano())

	if rand.Int31n(2) == 0 {
		req, _ = http.NewRequest("GET", "http://localhost:"+strconv.Itoa(port)+"/test", nil)
	} else {
		buf := bytes.NewReader([]byte("a=b&c=d"))
		req, _ = http.NewRequest("POST", "http://localhost:"+strconv.Itoa(port)+"/test", buf)
	}

	req.Header.Add("Referer", "http://localhost/test")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Accept-Language", "en-GB,*")
	req.Header.Add("X-Forwarded-For", "1.1.1.1, 2.2.2.2, 3.3.3.3")
	req.Header.Add("X-Forwarded-Proto", "http")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Unknown; Linux x86_64) AppleWebKit/534.34 (KHTML, like Gecko) PhantomJS/1.9.1 Safari/534.34")

	ck1 := new(http.Cookie)
	ck1.Name = "test"
	ck1.Value = "value"

	req.AddCookie(ck1)

	return req
}

func TestReplay(t *testing.T) {
	var request *http.Request
	received := make(chan int)

	listenHandler := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "OK", http.StatusAccepted)
	}

	replayHandler := func(w http.ResponseWriter, r *http.Request) {
		isEqual(t, r.URL.Path, request.URL.Path)

		isEqual(t, r.Header.Get("New-Header"), "Inserted")
		isEqual(t, r.Header.Get("X-Forwarded-Proto"), "Overwritten")

		if len(r.Cookies()) > 0 {
			isEqual(t, r.Cookies()[0].Value, request.Cookies()[0].Value)
		} else {
			t.Error("Cookies should not be blank")
		}

		http.Error(w, "OK", http.StatusAccepted)

		if t.Failed() {
			fmt.Println("\nReplayed:", r)
		}

		received <- 1
	}

	env := &Env{
		Verbose:       true,
		ListenHandler: listenHandler,
		ReplayHandler: replayHandler,
		AdditionalHeaders: replay.Headers{
			{"New-Header", "Inserted"},
			{"X-Forwarded-Proto", "Overwritten"},
		},
	}
	p := env.start()

	request = getRequest(p)

	_, err := http.DefaultClient.Do(request)

	if err != nil {
		t.Error("Can't make request", err)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Error("Timeout error")
	}
}

func rateLimitEnv(replayLimit int, listenerLimit int, connCount int, t *testing.T) int32 {
	var processed int32

	listenHandler := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "OK", http.StatusAccepted)
	}

	replayHandler := func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&processed, 1)
		http.Error(w, "OK", http.StatusAccepted)
	}

	env := &Env{
		ListenHandler: listenHandler,
		ReplayHandler: replayHandler,
		ReplayLimit:   replayLimit,
		ListenerLimit: listenerLimit,
		Verbose:       true,
	}

	p := env.start()

	for i := 0; i < connCount; i++ {
		req := getRequest(p)

		go func() {
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
			} else {
				t.Errorf("", err)
			}
		}()
	}

	time.Sleep(time.Second + 200*time.Millisecond)

	return processed
}

func TestWithoutReplayRateLimit(t *testing.T) {
	processed := rateLimitEnv(0, 0, 10, t)

	if processed != 10 {
		t.Error("It should forward all requests without rate-limiting, got:", processed)
	}
}

func TestReplayRateOverLimit(t *testing.T) {
	processed := rateLimitEnv(10, 0, 5, t)

	if processed != 10 {
		t.Error("It should forward 2x more requests with rate over actual request count", processed)
	}
}

func TestReplayRateLimit(t *testing.T) {
	processed := rateLimitEnv(5, 0, 10, t)

	if processed != 5 {
		t.Error("It should forward only 5 requests with rate-limiting, got:", processed)
	}
}

func TestListenerRateLimit(t *testing.T) {
	processed := rateLimitEnv(0, 3, 100, t)

	if processed != 3 {
		t.Error("It should forward only 3 requests with rate-limiting, got:", processed)
	}
}

func (e *Env) startFileListener() (p int) {
	p = 50000 + envs*10

	e.ForwardPort = p + 2
	go e.startHTTP(p, http.HandlerFunc(e.ListenHandler))
	go e.startHTTP(p+2, http.HandlerFunc(e.ReplayHandler))
	go e.startFileUsingListener(p, p+1)

	// Time to start http and gor instances
	time.Sleep(time.Millisecond * 100)

	envs++

	return
}

func (e *Env) startFileUsingListener(port int, replayPort int) {
	listener.Settings.Verbose = e.Verbose
	listener.Settings.Address = "127.0.0.1"
	listener.Settings.FileToReplayPath = "integration_request.gor"
	listener.Settings.Port = port

	if e.ListenerLimit != 0 {
		listener.Settings.ReplayAddress += "|" + strconv.Itoa(e.ListenerLimit)
	}

	listener.Run()
}

func (e *Env) startFileUsingReplay() {
	replay.Settings.Verbose = e.Verbose
	replay.Settings.FileToReplayPath = "integration_request.gor"
	replay.Settings.ForwardAddress = "127.0.0.1:" + strconv.Itoa(e.ForwardPort)

	if e.ReplayLimit != 0 {
		replay.Settings.ForwardAddress += "|" + strconv.Itoa(e.ReplayLimit)
	}

	replay.Run()
}

func TestSavingRequestToFileAndReplayThem(t *testing.T) {
	processed := make(chan int)

	listenHandler := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "OK", http.StatusNotFound)
	}

	requestsCount := 0
	var replayedRequests []*http.Request
	replayHandler := func(w http.ResponseWriter, r *http.Request) {
		requestsCount++

		isEqual(t, r.URL.Path, "/test")
		isEqual(t, r.Cookies()[0].Value, "value")

		http.Error(w, "404 page not found", http.StatusNotFound)

		replayedRequests = append(replayedRequests, r)
		if t.Failed() {
			fmt.Println("\nReplayed:", r)
		}

		if requestsCount > 1 {
			processed <- 1
		}
	}

	env := &Env{
		Verbose:       true,
		ListenHandler: listenHandler,
		ReplayHandler: replayHandler,
	}

	p := env.startFileListener()

	for i := 0; i < 30; i++ {
		request := getRequest(p)

		go func() {
			_, err := http.DefaultClient.Do(request)

			if err != nil {
				t.Error("Can't make request", err)
			}
		}()
	}

	// TODO: wait until gor will process response, should be kind of flag/semaphore
	time.Sleep(time.Millisecond * 700)
	go env.startFileUsingReplay()

	select {
	case <-processed:
	case <-time.After(2 * time.Second):
		for _, value := range replayedRequests {
			fmt.Println(value)
		}
		t.Error("Timeout error")
	}
}
