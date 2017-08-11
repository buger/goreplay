package main

import (
	"bytes"
	"compress/gzip"
	"github.com/buger/goreplay/proto"
	"io/ioutil"
	"net/http/httputil"
	"strconv"
)

func prettifyHTTP(p []byte) []byte {
	headSize := bytes.IndexByte(p, '\n') + 1
	head := p[:headSize]
	body := p[headSize:]

	headersPos := proto.MIMEHeadersEndPos(body)
	headers := body[:headersPos]
	content := body[headersPos:]

	var tEnc, cEnc []byte
	proto.ParseHeaders([][]byte{headers}, func(header, value []byte) bool {
		if proto.HeadersEqual(header, []byte("Transfer-Encoding")) {
			tEnc = value
		}

		if proto.HeadersEqual(header, []byte("Content-Encoding")) {
			cEnc = value
		}

		return true
	})

	if len(tEnc) == 0 && len(cEnc) == 0 {
		return p
	}

	if bytes.Equal(tEnc, []byte("chunked")) {
		buf := bytes.NewBuffer(content)
		r := httputil.NewChunkedReader(buf)
		content, _ = ioutil.ReadAll(r)

		headers = proto.DeleteHeader(headers, []byte("Transfer-Encoding"))

		newLen := strconv.Itoa(len(content))
		headers = proto.SetHeader(headers, []byte("Content-Length"), []byte(newLen))
	}

	if bytes.Equal(cEnc, []byte("gzip")) {
		buf := bytes.NewBuffer(content)
		g, err := gzip.NewReader(buf)

		if err != nil {
			Debug("[Prettifier] GZIP encoding error:", err)
			return []byte{}
		}

		content, _ = ioutil.ReadAll(g)

		headers = proto.DeleteHeader(headers, []byte("Content-Encoding"))

		newLen := strconv.Itoa(len(content))
		headers = proto.SetHeader(headers, []byte("Content-Length"), []byte(newLen))
	}

	newPayload := append(append(head, headers...), content...)

	return newPayload
}
