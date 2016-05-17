package main

import (
	"bytes"
	"crypto/rand"
	"github.com/Clever/gor/proto"
	"io/ioutil"
	_ "log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	_ "reflect"
	"sync"
	"testing"
	"time"
	_ "time"
)

func TestHTTPClientURLPort(t *testing.T) {
	c1 := NewHTTPClient("http://example.com", &HTTPClientConfig{})
	if c1.baseURL != "http://example.com" {
		t.Error("Sould not add 80 port for http:", c1.baseURL)
	}

	c2 := NewHTTPClient("https://example.com", &HTTPClientConfig{})
	if c2.baseURL != "https://example.com:443" {
		t.Error("Sould add 443 port for https:", c2.baseURL)
	}

	c3 := NewHTTPClient("https://example.com:1", &HTTPClientConfig{})
	if c3.baseURL != "https://example.com:1" {
		t.Error("Sould use specified port:", c3.baseURL)
	}

	c4 := NewHTTPClient("example.com", &HTTPClientConfig{})
	if c4.baseURL != "http://example.com" {
		t.Error("Sould not add default protocol:", c4.baseURL)
	}
}

func TestHTTPClientSend(t *testing.T) {
	wg := new(sync.WaitGroup)

	payload := func(reqType string) []byte {
		switch reqType {
		case "GET":
			return []byte("GET / HTTP/1.1\r\n\r\n")
		case "POST":
			return []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
		case "POST_CHUNKED":
			return []byte("POST / HTTP/1.1\r\nHost: www.w3.org\r\nTransfer-Encoding: chunked\r\n\r\n4\r\nWiki\r\n5\r\npedia\r\ne\r\n in\r\n\r\nchunks.\r\n0\r\n\r\n")
		}

		return []byte("")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Method == "POST" {
			defer r.Body.Close()
			body, _ := ioutil.ReadAll(r.Body)

			if len(r.TransferEncoding) > 0 && r.TransferEncoding[0] == "chunked" {
				if string(body) != "Wikipedia in\r\n\r\nchunks." {
					t.Error("Wrong POST body:", body, string(body))
				}
			} else {
				if string(body) != "a=1&b=2" {
					buf, _ := httputil.DumpRequest(r, true)
					t.Error("Wrong POST body:", string(body), string(buf))
				}
			}
		}

		wg.Done()
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, &HTTPClientConfig{Debug: true})

	wg.Add(4)
	client.Send(payload("POST"))
	client.Send(payload("GET"))
	client.Send(payload("POST_CHUNKED"))
	client.Send(payload("POST"))

	wg.Wait()
}

// https://github.com/buger/gor/issues/184
func TestHTTPClientResponseBuffer(t *testing.T) {
	wg := new(sync.WaitGroup)

	payload := []byte("GET / HTTP/1.1\r\n\r\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		size := 10 * 1024 // 10kb
		rb := make([]byte, size)
		rand.Read(rb)

		w.Write(rb)

		wg.Done()
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, &HTTPClientConfig{Debug: false, ResponseBufferSize: 1024})

	wg.Add(2)
	client.Send(payload)
	resp, _ := client.Send(payload)

	if !bytes.Equal(resp[0:8], []byte("HTTP/1.1")) {
		t.Error("Response buffer contains data from previous request", string(resp[0:5]))
	}

	wg.Wait()
}

func TestHTTPClientHTTPSSend(t *testing.T) {
	wg := new(sync.WaitGroup)

	payload := func(reqType string) []byte {
		switch reqType {
		case "GET":
			return []byte("GET / HTTP/1.1\r\n\r\n")
		case "POST":
			return []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
		case "POST_CHUNKED":
			return []byte("POST / HTTP/1.1\r\nHost: www.w3.org\r\nTransfer-Encoding: chunked\r\n\r\n4\r\nWiki\r\n5\r\npedia\r\ne\r\n in\r\n\r\nchunks.\r\n0\r\n\r\n")
		}

		return []byte("")
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Method == "POST" {
			defer r.Body.Close()
			body, _ := ioutil.ReadAll(r.Body)

			if len(r.TransferEncoding) > 0 && r.TransferEncoding[0] == "chunked" {
				if string(body) != "Wikipedia in\r\n\r\nchunks." {
					t.Error("Wrong POST body:", body, string(body))
				}
			} else {
				if string(body) != "a=1&b=2" {
					buf, _ := httputil.DumpRequest(r, true)
					t.Error("Wrong POST body:", string(body), string(buf))
				}
			}
		}

		wg.Done()
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, &HTTPClientConfig{})

	wg.Add(4)
	client.Send(payload("POST"))
	client.Send(payload("GET"))
	client.Send(payload("POST_CHUNKED"))
	client.Send(payload("POST"))

	wg.Wait()
}

func TestHTTPClientServerInstantDisconnect(t *testing.T) {
	wg := new(sync.WaitGroup)

	GETPayload := []byte("GET / HTTP/1.1\r\n\r\n")

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()

			if err != nil {
				break
			}
			conn.Close()
			wg.Done()
		}
	}()

	client := NewHTTPClient(ln.Addr().String(), &HTTPClientConfig{})

	wg.Add(2)
	client.Send(GETPayload)
	client.Send(GETPayload)

	wg.Wait()
}

func TestHTTPClientServerNoKeepAlive(t *testing.T) {
	wg := new(sync.WaitGroup)

	GETPayload := []byte("GET / HTTP/1.1\r\n\r\n")

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				break
			}

			buf := make([]byte, 4096)
			reqLen, err := conn.Read(buf)
			if err != nil {
				t.Error("Error reading:", err.Error())
			}
			Debug("Received: ", string(buf[0:reqLen]))
			conn.Write([]byte("OK"))

			// No keep-alive connections
			conn.Close()

			wg.Done()
		}
	}()

	client := NewHTTPClient(ln.Addr().String(), &HTTPClientConfig{})

	wg.Add(2)
	client.Send(GETPayload)
	client.Send(GETPayload)

	wg.Wait()
}

func TestHTTPClientRedirect(t *testing.T) {
	wg := new(sync.WaitGroup)

	GETPayload := []byte("GET / HTTP/1.1\r\n\r\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path == "/" {
			http.Redirect(w, r, "/new", 301)
		}

		wg.Done()
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, &HTTPClientConfig{FollowRedirects: 1, Debug: false})

	// Should do 2 queries
	wg.Add(2)
	client.Send(GETPayload)

	wg.Wait()
}

func TestHTTPClientRedirectLimit(t *testing.T) {
	wg := new(sync.WaitGroup)

	GETPayload := []byte("GET / HTTP/1.1\r\n\r\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path == "/" {
			http.Redirect(w, r, "/r1", 301)
		}

		if r.URL.Path == "/r1" {
			http.Redirect(w, r, "/r2", 301)
		}

		if r.URL.Path == "/r2" {
			http.Redirect(w, r, "/new", 301)
		}

		wg.Done()
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, &HTTPClientConfig{FollowRedirects: 2, Debug: false})

	// Have 3 redirects + 1 GET, but should do only 2 redirects + GET
	wg.Add(3)
	client.Send(GETPayload)

	wg.Wait()
}

func TestHTTPClientHandleHTTP10(t *testing.T) {
	wg := new(sync.WaitGroup)

	GETPayload := []byte("GET http://foobar.com/path HTTP/1.0\r\n\r\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path != "/path" {
			t.Error("Path not match:", r.URL.Path)
		}

		wg.Done()
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, &HTTPClientConfig{Debug: true})

	wg.Add(1)
	client.Send(GETPayload)

	wg.Wait()
}

func TestHTTPClientErrors(t *testing.T) {
	req := []byte("GET http://foobar.com/path HTTP/1.0\r\n\r\n")

	// Port not exists
	client := NewHTTPClient("http://127.0.0.1:1", &HTTPClientConfig{Debug: true})
	if resp, err := client.Send(req); err != nil {
		if s := proto.Status(resp); !bytes.Equal(s, []byte("521")) {
			t.Error("Should return status 521 for connection refused, instead:", string(s))
		}
	} else {
		t.Error("Should throw error")
	}

	client = NewHTTPClient("http://not.existing", &HTTPClientConfig{Debug: true})
	if resp, err := client.Send(req); err != nil {
		if s := proto.Status(resp); !bytes.Equal(s, []byte("521")) {
			t.Error("Should return status 521 for no such host, instead:", string(s))
		}
	} else {
		t.Error("Should throw error")
	}

	// Non routable IP address to simulate connection timeout
	client = NewHTTPClient("http://10.255.255.1", &HTTPClientConfig{Debug: true, ConnectionTimeout: 100 * time.Millisecond})

	if resp, err := client.Send(req); err != nil {
		if s := proto.Status(resp); !bytes.Equal(s, []byte("521")) {
			t.Error("Should return status 521 for io/timeout:", string(s))
		}
	} else {
		t.Error("Should throw error")
	}

	// Connecting but io timeout on read
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	client = NewHTTPClient("http://"+ln.Addr().String(), &HTTPClientConfig{Debug: true, Timeout: 10 * time.Millisecond})
	defer ln.Close()

	if resp, err := client.Send(req); err != nil {
		if s := proto.Status(resp); !bytes.Equal(s, []byte("524")) {
			t.Error("Should return status 524 for io read, instead:", string(s))
		}
	} else {
		t.Error("Should throw error")
	}

	// Response read error read tcp [::1]:51128: connection reset by peer &{{0xc20802a000}}
	ln1, _ := net.Listen("tcp", "127.0.0.1:0")
	go func() {
		ln1.Accept()
	}()
	defer ln1.Close()

	client = NewHTTPClient("http://"+ln1.Addr().String(), &HTTPClientConfig{Debug: true, Timeout: 10 * time.Millisecond})

	if resp, err := client.Send(req); err != nil {
		if s := proto.Status(resp); !bytes.Equal(s, []byte("524")) {
			t.Error("Should return status 524 for connection reset by peer, instead:", string(s))
		}
	} else {
		t.Error("Should throw error")
	}

	ln2, _ := net.Listen("tcp", "127.0.0.1:0")
	go func() {
		buf := make([]byte, 64*1024)
		conn, _ := ln2.Accept()

		conn.Read(buf)
		defer conn.Close()
	}()
	defer ln2.Close()

	client = NewHTTPClient("http://"+ln2.Addr().String(), &HTTPClientConfig{Debug: true, Timeout: 10 * time.Millisecond})

	if resp, err := client.Send(req); err != nil {
		if s := proto.Status(resp); !bytes.Equal(s, []byte("524")) {
			t.Error("Should return status 524 for connection reset by peer, instead:", string(s))
		}
	} else {
		t.Error("Should throw error")
	}
}
