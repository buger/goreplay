package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buger/goreplay/capture"
	"github.com/buger/goreplay/proto"
)

type fakeServiceCb func(string, int, []byte)

// Simple service that generate token on request, and require this token for accesing to secure area
func NewFakeSecureService(wg *sync.WaitGroup, cb fakeServiceCb) *httptest.Server {
	activeTokens := make([]string, 0)
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch req.URL.Path {
		case "/token":
			// Generate random token
			tokenLength := 10
			buf := make([]byte, tokenLength)
			rand.Read(buf)
			token := hex.EncodeToString(buf)
			activeTokens = append(activeTokens, token)

			w.Write([]byte(token))

			cb(req.URL.Path, 200, []byte(token))
		case "/secure":
			token := req.URL.Query().Get("token")
			tokenFound := false

			for _, t := range activeTokens {
				if t == token {
					tokenFound = true
					break
				}
			}

			if tokenFound {
				w.WriteHeader(http.StatusAccepted)
				cb(req.URL.Path, 202, nil)
			} else {
				w.WriteHeader(http.StatusForbidden)
				cb(req.URL.Path, 403, nil)
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

	client := NewHTTPClient(&HTTPOutputConfig{}).Client
	rep, _ := client.Get(server.URL + "/token")
	resp, _ = httputil.DumpResponse(rep, true)
	token = proto.Body(resp)

	// Right token
	rep, _ = client.Get(server.URL + "/secure?token=" + string(token))
	resp, _ = httputil.DumpResponse(rep, true)
	if !bytes.Equal(proto.Status(resp), []byte("202")) {
		t.Error("Valid token should return status 202:", string(proto.Status(resp)))
	}

	// Wrong tokens forbidden
	rep, _ = client.Get(server.URL + "/secure?token=wrong")
	resp, _ = httputil.DumpResponse(rep, true)
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

	// Catch traffic from one service
	fromAddr := strings.Replace(from.Listener.Addr().String(), "[::]", "127.0.0.1", -1)
	conf := RAWInputConfig{
		Engine:        capture.EnginePcap,
		Expire:        testRawExpire,
		Protocol:      ProtocolHTTP,
		TrackResponse: true,
	}
	input := NewRAWInput(fromAddr, conf)

	// And redirect to another
	output := NewHTTPOutput(to.URL, &HTTPOutputConfig{})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	// Start Gor
	emitter := NewEmitter()
	emitter.Start(plugins, "echo -n && GOR_TEST=true && ./examples/middleware/echo.sh")

	// Wait till middleware initialization
	time.Sleep(100 * time.Millisecond)

	// Should receive 2 requests from original + 2 from replayed
	client := NewHTTPClient(output.(*HTTPOutput).config).Client

	for i := 0; i < 10; i++ {
		wg.Add(2)
		// Request should be echoed
		client.Get(to.URL + "/a")
		time.Sleep(5 * time.Millisecond)
		client.Get(to.URL + "/b")
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()
	emitter.Close()
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

	Settings.Middleware = "echo -n && GOR_TEST=true && go run ./examples/middleware/token_modifier.go"

	fromAddr := strings.Replace(from.Listener.Addr().String(), "[::]", "127.0.0.1", -1)
	conf := RAWInputConfig{
		Engine:        capture.EnginePcap,
		Expire:        testRawExpire,
		Protocol:      ProtocolHTTP,
		TrackResponse: true,
	}
	// Catch traffic from one service
	input := NewRAWInput(fromAddr, conf)

	// And redirect to another
	output := NewHTTPOutput(to.URL, &HTTPOutputConfig{})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	// Start Gor
	emitter := NewEmitter()
	emitter.Start(plugins, Settings.Middleware)

	// Should receive 2 requests from original + 2 from replayed
	wg.Add(2)

	client := NewHTTPClient(&HTTPOutputConfig{}).Client

	// Sending traffic to original service
	rep, _ := client.Get(to.URL + "/token")
	resp, _ = httputil.DumpResponse(rep, true)
	token = proto.Body(resp)

	rep, _ = client.Get(to.URL + "/secure?token=" + string(token))
	resp, _ = httputil.DumpResponse(rep, true)
	if !bytes.Equal(proto.Status(resp), []byte("202")) {
		t.Error("Valid token should return 202:", proto.Status(resp))
	}

	wg.Wait()
	emitter.Close()
	Settings.Middleware = ""
}
