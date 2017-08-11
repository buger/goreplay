package main

import (
	"bytes"
	"compress/gzip"
	"strconv"
	"testing"
)

func TestHTTPPrettifierGzip(t *testing.T) {
	b := bytes.NewBufferString("")
	w := gzip.NewWriter(b)
	w.Write([]byte("test"))
	w.Close()

	size := strconv.Itoa(len(b.Bytes()))

	payload := []byte("2 1 1\nHTTP/1.1 200 OK\r\nContent-Length: " + size + "\r\nContent-Encoding: gzip\r\n\r\n")
	payload = append(payload, b.Bytes()...)

	newPayload := prettifyHTTP(payload)

	if string(newPayload) != "2 1 1\nHTTP/1.1 200 OK\r\nContent-Length: 4\r\n\r\ntest" {
		t.Error("Payload not match:", string(newPayload))
	}
}

func TestHTTPPrettifierChunked(t *testing.T) {
	payload := []byte("POST / HTTP/1.1\r\nHost: www.w3.org\r\nTransfer-Encoding: chunked\r\n\r\n4\r\nWiki\r\n5\r\npedia\r\ne\r\n in\r\n\r\nchunks.\r\n0\r\n\r\n")

	newPayload := prettifyHTTP(payload)

	if string(newPayload) != "POST / HTTP/1.1\r\nHost: www.w3.org\r\nContent-Length: 23\r\n\r\nWikipedia in\r\n\r\nchunks." {
		t.Error("Payload not match:", string(newPayload))
	}
}
