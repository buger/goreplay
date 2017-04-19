package main

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// TestInput used for testing purpose, it allows emitting requests on demand
type TestInput struct {
	data       chan []byte
	skipHeader bool
}

// NewTestInput constructor for TestInput
func NewTestInput() (i *TestInput) {
	i = new(TestInput)
	i.data = make(chan []byte, 100)

	return
}

func (i *TestInput) Read(data []byte) (int, error) {
	buf := <-i.data

	var header []byte

	if !i.skipHeader {
		header = payloadHeader(RequestPayload, uuid(), time.Now().UnixNano(), -1)
		copy(data[0:len(header)], header)
		copy(data[len(header):], buf)
	} else {
		copy(data, buf)
	}

	return len(buf) + len(header), nil
}

func (i *TestInput) EmitBytes(data []byte) {
	i.data <- data
}

// EmitGET emits GET request without headers
func (i *TestInput) EmitGET() {
	i.data <- []byte("GET / HTTP/1.1\r\n\r\n")
}

// EmitPOST emits POST request with Content-Length
func (i *TestInput) EmitPOST() {
	i.data <- []byte("POST /pub/WWW/ HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
}

// EmitChunkedPOST emits POST request with `Transfer-Encoding: chunked` and chunked body
func (i *TestInput) EmitChunkedPOST() {
	i.data <- []byte("POST /pub/WWW/ HTTP/1.1\r\nHost: www.w3.org\r\nTransfer-Encoding: chunked\r\n\r\n4\r\nWiki\r\n5\r\npedia\r\ne\r\n in\r\n\r\nchunks.\r\n0\r\n\r\n")
}

// EmitLargePOST emits POST request with large payload (5mb)
func (i *TestInput) EmitLargePOST() {
	size := 5 * 1024 * 1024 // 5 MB
	rb := make([]byte, size)
	rand.Read(rb)

	rs := base64.URLEncoding.EncodeToString(rb)

	i.data <- []byte("POST / HTTP/1.1\nHost: www.w3.org\nContent-Length:5242880\r\n\r\n" + rs)
	Debug("Sent large POST")
}

// EmitSizedPOST emit a POST with a payload set to a supplied size
func (i *TestInput) EmitSizedPOST(payloadSize int) {
	rb := make([]byte, payloadSize)
	rand.Read(rb)

	rs := base64.URLEncoding.EncodeToString(rb)

	i.data <- []byte("POST / HTTP/1.1\nHost: www.w3.org\nContent-Length:5242880\r\n\r\n" + rs)
	Debug("Sent large POST")
}

// EmitOPTIONS emits OPTIONS request, similar to GET
func (i *TestInput) EmitOPTIONS() {
	i.data <- []byte("OPTIONS / HTTP/1.1\nHost: www.w3.org\r\n\r\n")
}

func (i *TestInput) String() string {
	return "Test Input"
}
