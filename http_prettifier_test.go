package main

import (
	"bytes"
	"compress/gzip"
	"strconv"
	"testing"

	"github.com/buger/goreplay/proto"
)

func TestHTTPPrettifierGzip(t *testing.T) {
	b := bytes.NewBufferString("")
	w := gzip.NewWriter(b)
	w.Write([]byte("test"))
	w.Close()

	size := strconv.Itoa(len(b.Bytes()))

	payload := []byte("HTTP/1.1 200 OK\r\nContent-Length: " + size + "\r\nContent-Encoding: gzip\r\n\r\n")
	payload = append(payload, b.Bytes()...)

	newPayload := prettifyHTTP(payload)

	if string(newPayload) != "HTTP/1.1 200 OK\r\nContent-Length: 4\r\n\r\ntest" {
		t.Errorf("Payload not match %q", string(newPayload))
	}
}

func TestHTTPPrettifierChunked(t *testing.T) {
	payload := []byte("POST / HTTP/1.1\r\nHost: www.w3.org\r\nTransfer-Encoding: chunked\r\n\r\n4\r\nWiki\r\n5\r\npedia\r\ne\r\n in\r\n\r\nchunks.\r\n0\r\n\r\n")

	payload = prettifyHTTP(payload)
	if string(proto.Header(payload, []byte("Content-Length"))) != "23" {
		t.Errorf("payload should have content length of 23")
	}
}
