package prettify

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"net/http/httputil"
	"strconv"

	"github.com/buger/goreplay/pkg/http_proto"

	"github.com/rs/zerolog/log"
)

// PrettifyHTTP prettifies HTTP payload
func PrettifyHTTP(p []byte) []byte {

	tEnc := bytes.Equal(http_proto.Header(p, []byte("Transfer-Encoding")), []byte("chunked"))
	cEnc := bytes.Equal(http_proto.Header(p, []byte("Content-Encoding")), []byte("gzip"))

	if !(tEnc || cEnc) {
		return p
	}

	headersPos := http_proto.MIMEHeadersEndPos(p)

	if headersPos < 5 || headersPos > len(p) {
		return p
	}

	headers := p[:headersPos]
	content := p[headersPos:]

	if tEnc {
		buf := bytes.NewReader(content)
		r := httputil.NewChunkedReader(buf)
		content, _ = ioutil.ReadAll(r)

		headers = http_proto.DeleteHeader(headers, []byte("Transfer-Encoding"))

		newLen := strconv.Itoa(len(content))
		headers = http_proto.SetHeader(headers, []byte("Content-Length"), []byte(newLen))
	}

	if cEnc {
		buf := bytes.NewReader(content)
		g, err := gzip.NewReader(buf)

		if err != nil {
			log.Error().Err(err).Msg("GZIP encoding error")
			return []byte{}
		}

		content, err = ioutil.ReadAll(g)
		if err != nil {
			log.Error().Err(err).Msg("read error")
			return p
		}

		headers = http_proto.DeleteHeader(headers, []byte("Content-Encoding"))

		newLen := strconv.Itoa(len(content))
		headers = http_proto.SetHeader(headers, []byte("Content-Length"), []byte(newLen))
	}

	newPayload := append(headers, content...)

	return newPayload
}
