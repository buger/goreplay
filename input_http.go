package main

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"time"
)

// HTTPInput used for sending requests to Gor via http
type HTTPInput struct {
	data     chan []byte
	address  string
	listener net.Listener
	stop     chan bool // Channel used only to indicate goroutine should shutdown
}

// NewHTTPInput constructor for HTTPInput. Accepts address with port which it will listen on.
func NewHTTPInput(address string) (i *HTTPInput) {
	i = new(HTTPInput)
	i.data = make(chan []byte, 1000)
	i.stop = make(chan bool)

	i.listen(address)

	return
}

// PluginRead reads message from this plugin
func (i *HTTPInput) PluginRead() (*Message, error) {
	var msg Message
	select {
	case <-i.stop:
		return nil, ErrorStopped
	case buf := <-i.data:
		msg.Data = buf
		msg.Meta = payloadHeader(RequestPayload, uuid(), time.Now().UnixNano(), -1)
		return &msg, nil
	}
}

// Close closes this plugin
func (i *HTTPInput) Close() error {
	close(i.stop)
	return nil
}

func (i *HTTPInput) handler(w http.ResponseWriter, r *http.Request) {
	r.URL.Scheme = "http"
	r.URL.Host = i.address

	buf, _ := httputil.DumpRequestOut(r, true)
	http.Error(w, http.StatusText(200), 200)
	i.data <- buf
}

func (i *HTTPInput) listen(address string) {
	var err error

	mux := http.NewServeMux()

	mux.HandleFunc("/", i.handler)

	i.listener, err = net.Listen("tcp", address)
	if err != nil {
		log.Fatal("HTTP input listener failure:", err)
	}
	i.address = i.listener.Addr().String()

	go func() {
		err = http.Serve(i.listener, mux)
		if err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP input serve failure ", err)
		}
	}()
}

func (i *HTTPInput) String() string {
	return "HTTP input: " + i.address
}
