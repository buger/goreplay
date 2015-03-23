package main

import (
	"crypto/rand"
	"encoding/base64"
)

type TestInput struct {
	data chan []byte
}

func NewTestInput() (i *TestInput) {
	i = new(TestInput)
	i.data = make(chan []byte, 100)

	return
}

func (i *TestInput) Read(data []byte) (int, error) {
	buf := <-i.data
	copy(data, buf)

	return len(buf), nil
}

func (i *TestInput) EmitGET() {
	i.data <- []byte("GET / HTTP/1.1\r\n\r\n")
}

func (i *TestInput) EmitPOST() {
	i.data <- []byte("POST /pub/WWW/ HTTP/1.1\nHost: www.w3.org\r\n\r\nxyzxyzxyz")
}

func (i *TestInput) EmitChunkedPOST() {
	i.data <- []byte("POST /pub/WWW/ HTTP/1.1\r\nTransfer-Encoding: chunked\r\n\r\n9\r\nxyzxyzxyz\r\n0\r\n\r\n")
}

func (i *TestInput) EmitFile() {
	size := 1024 * 1024 // 1 MB
	rb := make([]byte, size)
	rand.Read(rb)

	rs := base64.URLEncoding.EncodeToString(rb)

	i.data <- []byte("POST /pub/WWW/ HTTP/1.1\nHost: www.w3.org\r\n\r\n------WebKitFormBoundaryePkpFF7tjBAqx29L\nContent-Disposition: form-data; name=\"uploadedfile\"; filename=\"hello.o\"\nContent-Type: application/x-object\n" + rs + "\r\n\r\n")
}

func (i *TestInput) EmitOPTIONS() {
	i.data <- []byte("OPTIONS / HTTP/1.1\nHost: www.w3.org\r\n\r\n")
}

func (i *TestInput) String() string {
	return "Test Input"
}
