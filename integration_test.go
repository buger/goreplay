package main

import (
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buger/gor/listener"
	"github.com/buger/gor/replay"
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
}

func (e *Env) start() (p int) {
	p = 50000 + envs*10

	go e.startHTTP(p, http.HandlerFunc(e.ListenHandler))
	go e.startHTTP(p+2, http.HandlerFunc(e.ReplayHandler))
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

	replay.Run()
}

func (e *Env) startHTTP(port int, handler http.Handler) {
	err := http.ListenAndServe(":"+strconv.Itoa(port), handler)

	if err != nil {
		fmt.Println("Error while starting http server:", err)
	}
}

func getRequest(port int) *http.Request {
	req, _ := http.NewRequest("GET", "http://localhost:"+strconv.Itoa(port)+"/test", nil)
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

		if len(r.Cookies()) > 0 {
			isEqual(t, r.Cookies()[0].Value, request.Cookies()[0].Value)
		} else {
			t.Error("Cookies should not be blank")
		}

		http.Error(w, "OK", http.StatusAccepted)

		if t.Failed() {
			fmt.Println("\nReplayed:", r, "\nOriginal:", request)
		}

		received <- 1
	}

	env := &Env{
		Verbose:       true,
		ListenHandler: listenHandler,
		ReplayHandler: replayHandler,
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

func rateLimitEnv(replayLimit int, listenerLimit int, connCount int) int32 {
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
	}

	p := env.start()
	req := getRequest(p)

	for i := 0; i < connCount; i++ {
		go func() {
			http.DefaultClient.Do(req)
		}()
	}

	time.Sleep(time.Millisecond * 500)

	return processed
}

func TestWithoutReplayRateLimit(t *testing.T) {
	processed := rateLimitEnv(0, 0, 10)

	if processed != 10 {
		t.Error("It should forward all requests without rate-limiting", processed)
	}
}

func TestReplayRateLimit(t *testing.T) {
	processed := rateLimitEnv(5, 0, 10)

	if processed != 5 {
		t.Error("It should forward only 5 requests with rate-limiting", processed)
	}
}

func TestListenerRateLimit(t *testing.T) {
	processed := rateLimitEnv(0, 3, 100)

	if processed != 3 {
		t.Error("It should forward only 3 requests with rate-limiting", processed)
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
	listener.Settings.FileToReplyPath = "integration_request.gor"
	listener.Settings.Port = port

	if e.ListenerLimit != 0 {
		listener.Settings.ReplayAddress += "|" + strconv.Itoa(e.ListenerLimit)
	}

	listener.Run()
}

func (e *Env) startFileUsingReplay() {
	replay.Settings.Verbose = e.Verbose
	replay.Settings.FileToReplyPath = "integration_request.gor"
	replay.Settings.ForwardAddress = "127.0.0.1:" + strconv.Itoa(e.ForwardPort)

	if e.ReplayLimit != 0 {
		replay.Settings.ForwardAddress += "|" + strconv.Itoa(e.ReplayLimit)
	}

	replay.Run()
}

func TestSavingRequestToFileAndReplyThem(t *testing.T) {
	var request *http.Request
	processed := make(chan int)

	listenHandler := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "OK", http.StatusNotFound)
	}

	requestsCount := 0
	var replayedRequests []*http.Request
	replayHandler := func(w http.ResponseWriter, r *http.Request) {
		requestsCount++

		isEqual(t, r.URL.Path, request.URL.Path)
		isEqual(t, r.Cookies()[0].Value, request.Cookies()[0].Value)

		http.Error(w, "404 page not found", http.StatusNotFound)

		replayedRequests = append(replayedRequests, r)
		if t.Failed() {
			fmt.Println("\nReplayed:", r, "\nOriginal:", request)
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

	request = getRequest(p)

	for i := 0; i < 2; i++ {
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
