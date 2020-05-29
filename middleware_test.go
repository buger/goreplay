package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buger/goreplay/proto"
)

type fakeServiceCb func(string, int, []byte)

// Simple service that generate token on request, and require this token for accesing to secure area
func NewFakeSecureService(wg *sync.WaitGroup, cb fakeServiceCb) *httptest.Server {
	active_tokens := make([]string, 0)
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		Debug("Received request: " + req.URL.String())

		switch req.URL.Path {
		case "/token":
			// Generate random token
			token_length := 10
			buf := make([]byte, token_length)
			rand.Read(buf)
			token := hex.EncodeToString(buf)
			active_tokens = append(active_tokens, token)

			w.Write([]byte(token))

			cb(req.URL.Path, 200, []byte(token))
		case "/secure":
			token := req.URL.Query().Get("token")
			token_found := false

			for _, t := range active_tokens {
				if t == token {
					token_found = true
					break
				}
			}

			if token_found {
				w.WriteHeader(http.StatusAccepted)
				cb(req.URL.Path, 202, []byte(nil))
			} else {
				w.WriteHeader(http.StatusForbidden)
				cb(req.URL.Path, 403, []byte(nil))
			}
		}

		wg.Done()
	}))

	return server
}

func TestFakeSecureService(t *testing.T) {
	var resp, token []byte

	wg := new(sync.WaitGroup)

	server := NewFakeSecureService(wg, func(path string, status int, resp []byte) {
	})
	defer server.Close()

	wg.Add(3)

	client := NewHTTPClient(server.URL, &HTTPClientConfig{Debug: true})
	resp, _ = client.Get("/token")
	token = proto.Body(resp)

	// Right token
	resp, _ = client.Get("/secure?token=" + string(token))
	if !bytes.Equal(proto.Status(resp), []byte("202")) {
		t.Error("Valid token should return status 202:", string(proto.Status(resp)))
	}

	// Wrong tokens forbidden
	resp, _ = client.Get("/secure?token=wrong")
	if !bytes.Equal(proto.Status(resp), []byte("403")) {
		t.Error("Wrong token should returns status 403:", string(proto.Status(resp)))
	}

	wg.Wait()
}

func TestEchoMiddleware(t *testing.T) {
	wg := new(sync.WaitGroup)

	from := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Env", "prod")
		w.Header().Set("RequestPath", r.URL.Path)
		wg.Done()
	}))
	defer from.Close()

	to := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Env", "test")
		w.Header().Set("RequestPath", r.URL.Path)
		wg.Done()
	}))
	defer to.Close()

	quit := make(chan int)

	Settings.middleware = "./examples/middleware/echo.sh"

	// Catch traffic from one service
	fromAddr := strings.Replace(from.Listener.Addr().String(), "[::]", "127.0.0.1", -1)
	input := NewRAWInput(fromAddr, EnginePcap, true, testRawExpire, "", "", "", 0)
	defer input.Close()

	// And redirect to another
	output := NewHTTPOutput(to.URL, &HTTPOutputConfig{Debug: false})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	// Start Gor
	go Start(plugins, quit)

	// Wait till middleware initialization
	time.Sleep(100 * time.Millisecond)

	// Should receive 2 requests from original + 2 from replayed
	client := NewHTTPClient(from.URL, &HTTPClientConfig{Debug: false})

	for i := 0; i < 10; i++ {
		wg.Add(4)
		// Request should be echoed
		client.Get("/a")
		time.Sleep(5 * time.Millisecond)
		client.Get("/b")
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()
	close(quit)
	time.Sleep(200 * time.Millisecond)

	Settings.middleware = ""
}

func TestTokenMiddleware(t *testing.T) {
	var resp, token []byte

	wg := new(sync.WaitGroup)

	from := NewFakeSecureService(wg, func(path string, status int, tok []byte) {
		time.Sleep(10 * time.Millisecond)
	})
	defer from.Close()

	to := NewFakeSecureService(wg, func(path string, status int, tok []byte) {
		switch path {
		case "/secure":
			if status != 202 {
				t.Error("Server should receive valid rewritten token")
			}
		}

		time.Sleep(10 * time.Millisecond)
	})
	defer to.Close()

	quit := make(chan int)

	Settings.middleware = "go run ./examples/middleware/token_modifier.go"

	fromAddr := strings.Replace(from.Listener.Addr().String(), "[::]", "127.0.0.1", -1)
	// Catch traffic from one service
	input := NewRAWInput(fromAddr, EnginePcap, true, testRawExpire, "", "", "", 0)
	defer input.Close()

	// And redirect to another
	output := NewHTTPOutput(to.URL, &HTTPOutputConfig{Debug: true})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	// Start Gor
	go Start(plugins, quit)

	// Wait for middleware to initialize
	// Give go compiller time to build programm
	time.Sleep(500 * time.Millisecond)

	// Should receive 2 requests from original + 2 from replayed
	wg.Add(4)

	client := NewHTTPClient(from.URL, &HTTPClientConfig{Debug: true})

	// Sending traffic to original service
	resp, _ = client.Get("/token")
	token = proto.Body(resp)

	// When delay is too smal, middleware does not always rewrite requests in time
	// Hopefuly client will have delay more then 100ms :)
	time.Sleep(100 * time.Millisecond)

	resp, _ = client.Get("/secure?token=" + string(token))
	if !bytes.Equal(proto.Status(resp), []byte("202")) {
		t.Error("Valid token should return 202:", proto.Status(resp))
	}

	wg.Wait()
	close(quit)
	time.Sleep(100 * time.Millisecond)
	Settings.middleware = ""
}
