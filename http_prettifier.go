package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"net/http/httputil"
	"strconv"

	"github.com/buger/goreplay/proto"
)

func prettifyHTTP(p []byte) []byte {
	headSize := bytes.IndexByte(p, '\n') + 1
	head := p[:headSize]
	body := p[headSize:]

	tEnc := bytes.Equal(proto.Header(body, []byte("Transfer-Encoding")), []byte("chunked"))
	cEnc := bytes.Equal(proto.Header(body, []byte("Content-Encoding")), []byte("gzip"))

	if !(tEnc || cEnc) {
		return p
	}

	headersPos := proto.MIMEHeadersEndPos(body)

	if headersPos < 5 || headersPos > len(body) {
		return p
	}

	headers := body[:headersPos]
	content := body[headersPos:]

	if tEnc {
		buf := bytes.NewReader(content)
		r := httputil.NewChunkedReader(buf)
		content, _ = ioutil.ReadAll(r)

		headers = proto.DeleteHeader(headers, []byte("Transfer-Encoding"))

		newLen := strconv.Itoa(len(content))
		headers = proto.SetHeader(headers, []byte("Content-Length"), []byte(newLen))
	}

	if cEnc {
		buf := bytes.NewReader(content)
		g, err := gzip.NewReader(buf)

		if err != nil {
			Debug(1, "[Prettifier] GZIP encoding error:", err)
			return []byte{}
		}

		content, err = ioutil.ReadAll(g)
		if err != nil {
			Debug(1, fmt.Sprintf("[HTTP-PRETTIFIER] %q", err))
			return p
		}

		headers = proto.DeleteHeader(headers, []byte("Content-Encoding"))

		newLen := strconv.Itoa(len(content))
		headers = proto.SetHeader(headers, []byte("Content-Length"), []byte(newLen))
	}

	newPayload := append(append(head, headers...), content...)

	return newPayload
}
