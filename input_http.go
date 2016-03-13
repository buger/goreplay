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
}

// NewHTTPInput constructor for HTTPInput. Accepts address with port which he will listen on.
func NewHTTPInput(address string) (i *HTTPInput) {
	i = new(HTTPInput)
	i.data = make(chan []byte, 10000)
	i.address = address

	i.listen(address)

	return
}

func (i *HTTPInput) Read(data []byte) (int, error) {
	buf := <-i.data

	header := payloadHeader(RequestPayload, uuid(), time.Now().UnixNano())

	copy(data[0:len(header)], header)
	copy(data[len(header):], buf)

	return len(buf) + len(header), nil
}

func (i *HTTPInput) handler(w http.ResponseWriter, r *http.Request) {
	r.URL.Scheme = "http"
	r.URL.Host = i.listener.Addr().String()

	buf, _ := httputil.DumpRequestOut(r, true)
	http.Error(w, http.StatusText(200), 200)

	select {
	case i.data <- buf:
	default:
		Debug("[INPUT-HTTP] Dropping requests because output can't process them fast enough")
	}
}

func (i *HTTPInput) listen(address string) {
	var err error

	mux := http.NewServeMux()

	mux.HandleFunc("/", i.handler)

	i.listener, err = net.Listen("tcp", address)
	if err != nil {
		log.Fatal("HTTP input listener failure:", err)
	}

	go func() {
		err = http.Serve(i.listener, mux)
		if err != nil {
			log.Fatal("HTTP input serve failure:", err)
		}
	}()
}

func (i *HTTPInput) String() string {
	return "HTTP input: " + i.address
}
