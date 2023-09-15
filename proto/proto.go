/*
Package proto provides byte-level interaction with HTTP request payload.

Example of HTTP payload for future references, new line symbols escaped:

	POST /upload HTTP/1.1\r\n
	User-Agent: Gor\r\n
	Content-Length: 11\r\n
	\r\n
	Hello world

	GET /index.html HTTP/1.1\r\n
	User-Agent: Gor\r\n
	\r\n
	\r\n
*/
package proto

import (
	"bufio"
	"bytes"
	"github.com/buger/goreplay/internal/byteutils"

	_ "fmt"
	"net/http"
	"net/textproto"
	"strings"
)

// CRLF In HTTP newline defined by 2 bytes (for both windows and *nix support)
var CRLF = []byte("\r\n")

// EmptyLine acts as separator: end of Headers or Body (in some cases)
var EmptyLine = []byte("\r\n\r\n")

// HeaderDelim Separator for Header line. Header looks like: `HeaderName: value`
var HeaderDelim = []byte(": ")

// MIMEHeadersEndPos finds end of the Headers section, which should end with empty line.
func MIMEHeadersEndPos(payload []byte) int {
	pos := bytes.Index(payload, EmptyLine)
	if pos < 0 {
		return -1
	}
	return pos + 4
}

// MIMEHeadersStartPos finds start of Headers section
// It just finds position of second line (first contains location and method).
func MIMEHeadersStartPos(payload []byte) int {
	pos := bytes.Index(payload, CRLF)
	if pos < 0 {
		return -1
	}
	return pos + 2 // Find first line end
}

// header return value and positions of header/value start/end.
// If not found, value will be blank, and headerStart will be -1
// Do not support multi-line headers.
func header(payload []byte, name []byte) (value []byte, headerStart, headerEnd, valueStart, valueEnd int) {
	if HasTitle(payload) {
		headerStart = MIMEHeadersStartPos(payload)
		if headerStart < 0 {
			return
		}
	} else {
		headerStart = 0
	}

	var colonIndex int
	for headerStart < len(payload) {
		headerEnd = bytes.IndexByte(payload[headerStart:], '\n')
		if headerEnd == -1 {
			break
		}
		headerEnd += headerStart
		colonIndex = bytes.IndexByte(payload[headerStart:headerEnd], ':')
		if colonIndex == -1 {
			// Malformed header, skip, most likely packet with partial headers
			headerStart = headerEnd + 1
			continue
		}
		colonIndex += headerStart

		if bytes.EqualFold(payload[headerStart:colonIndex], name) {
			valueStart = colonIndex + 1
			valueEnd = headerEnd - 2
			break
		}
		headerStart = headerEnd + 1 // move to the next header
	}
	if valueStart == 0 {
		headerStart = -1
		headerEnd = -1
		valueEnd = -1
		valueStart = -1
		return
	}

	// ignore empty space after ':'
	for valueStart < valueEnd {
		if payload[valueStart] < 0x21 {
			valueStart++
		} else {
			break
		}
	}

	// ignore empty space at end of header value
	for valueEnd > valueStart {
		if payload[valueEnd] < 0x21 {
			valueEnd--
		} else {
			break
		}
	}
	value = payload[valueStart : valueEnd+1]

	return
}

// ParseHeaders Parsing headers from the payload
func ParseHeaders(p []byte) textproto.MIMEHeader {
	// trimming off the title of the request
	if HasTitle(p) {
		headerStart := MIMEHeadersStartPos(p)
		if headerStart > len(p)-1 {
			return nil
		}
		p = p[headerStart:]
	}
	headerEnd := MIMEHeadersEndPos(p)
	if headerEnd > 1 {
		p = p[:headerEnd]
	}
	return GetHeaders(p)
}

// GetHeaders returns mime headers from the payload
func GetHeaders(p []byte) textproto.MIMEHeader {
	reader := textproto.NewReader(bufio.NewReader(bytes.NewReader(p)))
	mime, err := reader.ReadMIMEHeader()
	if err != nil {
		return nil
	}
	return mime
}

// Header returns header value, if header not found, value will be blank
func Header(payload, name []byte) []byte {
	val, _, _, _, _ := header(payload, name)

	return val
}

// SetHeader sets header value. If header not found it creates new one.
// Returns modified request payload
func SetHeader(payload, name, value []byte) []byte {
	_, hs, _, vs, ve := header(payload, name)

	if hs != -1 {
		// If header found we just replace its value
		return byteutils.Replace(payload, vs, ve+1, value)
	}

	return AddHeader(payload, name, value)
}

// AddHeader takes http payload and appends new header to the start of headers section
// Returns modified request payload
func AddHeader(payload, name, value []byte) []byte {
	mimeStart := MIMEHeadersStartPos(payload)
	if mimeStart < 1 {
		return payload
	}
	header := make([]byte, len(name)+2+len(value)+2)
	copy(header[0:], name)
	copy(header[len(name):], HeaderDelim)
	copy(header[len(name)+2:], value)
	copy(header[len(header)-2:], CRLF)

	return byteutils.Insert(payload, mimeStart, header)
}

// DeleteHeader takes http payload and removes header name from headers section
// Returns modified request payload
func DeleteHeader(payload, name []byte) []byte {
	_, hs, he, _, _ := header(payload, name)
	if hs != -1 {
		return byteutils.Cut(payload, hs, he+1)
	}
	return payload
}

// Body returns request/response body
func Body(payload []byte) []byte {
	pos := MIMEHeadersEndPos(payload)
	if pos == -1 || len(payload) <= pos {
		return nil
	}
	return payload[pos:]
}

// Path takes payload and returns request path: Split(firstLine, ' ')[1]
func Path(payload []byte) []byte {
	if !HasRequestTitle(payload) {
		return nil
	}
	start := bytes.IndexByte(payload, ' ') + 1
	end := bytes.IndexByte(payload[start:], ' ')

	return payload[start : start+end]
}

// SetPath takes payload, sets new path and returns modified payload
func SetPath(payload, path []byte) []byte {
	if !HasTitle(payload) {
		return nil
	}
	start := bytes.IndexByte(payload, ' ') + 1
	end := bytes.IndexByte(payload[start:], ' ')

	return byteutils.Replace(payload, start, start+end, path)
}

// PathParam returns URL query attribute by given name, if no found: valueStart will be -1
func PathParam(payload, name []byte) (value []byte, valueStart, valueEnd int) {
	path := Path(payload)

	paramStart := -1
	if paramStart = bytes.Index(path, append([]byte{'&'}, append(name, '=')...)); paramStart == -1 {
		if paramStart = bytes.Index(path, append([]byte{'?'}, append(name, '=')...)); paramStart == -1 {
			return []byte(""), -1, -1
		}
	}

	valueStart = paramStart + len(name) + 2
	paramEnd := bytes.IndexByte(path[valueStart:], '&')

	// Param can end with '&' (another param), or end of line
	if paramEnd == -1 { // It is final param
		paramEnd = len(path)
	} else {
		paramEnd += valueStart
	}
	return path[valueStart:paramEnd], valueStart, paramEnd
}

// SetPathParam takes payload and updates path Query attribute
// If query param not found, it will append new
// Returns modified payload
func SetPathParam(payload, name, value []byte) []byte {
	path := Path(payload)
	_, vs, ve := PathParam(payload, name)

	if vs != -1 { // If param found, replace its value and set new Path
		newPath := make([]byte, len(path))
		copy(newPath, path)
		newPath = byteutils.Replace(newPath, vs, ve, value)

		return SetPath(payload, newPath)
	}

	// if param not found append to end of url
	// Adding 2 because of '?' or '&' at start, and '=' in middle
	newParam := make([]byte, len(name)+len(value)+2)

	if bytes.IndexByte(path, '?') == -1 {
		newParam[0] = '?'
	} else {
		newParam[0] = '&'
	}

	// Copy "param=value" into buffer, after it looks like "?param=value"
	copy(newParam[1:], name)
	newParam[1+len(name)] = '='
	copy(newParam[2+len(name):], value)

	// Append param to the end of path
	newPath := make([]byte, len(path)+len(newParam))
	copy(newPath, path)
	copy(newPath[len(path):], newParam)

	return SetPath(payload, newPath)
}

// SetHost updates Host header for HTTP/1.1 or updates host in path for HTTP/1.0 or Proxy requests
// Returns modified payload
func SetHost(payload, url, host []byte) []byte {
	// If this is HTTP 1.0 traffic or proxy traffic it may include host right into path variable, so instead of setting Host header we rewrite Path
	// Fix for https://github.com/buger/gor/issues/156
	if path := Path(payload); bytes.HasPrefix(path, []byte("http")) {
		hostStart := bytes.IndexByte(path, ':') // : position "https?:"
		hostStart += 3                          // Skip 1 ':' and 2 '\'
		hostEnd := hostStart + bytes.IndexByte(path[hostStart:], '/')

		newPath := make([]byte, len(path))
		copy(newPath, path)
		newPath = byteutils.Replace(newPath, 0, hostEnd, url)

		return SetPath(payload, newPath)
	}

	return SetHeader(payload, []byte("Host"), host)
}

// Method returns HTTP method
func Method(payload []byte) []byte {
	end := bytes.IndexByte(payload, ' ')
	if end == -1 {
		return nil
	}

	return payload[:end]
}

// Status returns response status.
// It happens to be in same position as request payload path
func Status(payload []byte) []byte {
	if !HasResponseTitle(payload) {
		return nil
	}
	start := bytes.IndexByte(payload, ' ') + 1
	// status code are in range 100-600
	return payload[start : start+3]
}

// Methods holds the http methods ordered in ascending order
var Methods = [...]string{
	http.MethodConnect, http.MethodDelete, http.MethodGet,
	http.MethodHead, http.MethodOptions, http.MethodPatch,
	http.MethodPost, http.MethodPut, http.MethodTrace,
}

const (
	//MinRequestCount GET / HTTP/1.1\r\n
	MinRequestCount = 16
	// MinResponseCount HTTP/1.1 200\r\n
	MinResponseCount = 14
	// VersionLen HTTP/1.1
	VersionLen = 8
)

// HasResponseTitle reports whether this payload has an HTTP/1 response title
func HasResponseTitle(payload []byte) bool {
	s := byteutils.SliceToString(payload)
	if len(s) < MinResponseCount {
		return false
	}
	titleLen := bytes.Index(payload, CRLF)
	if titleLen == -1 {
		return false
	}
	major, minor, ok := http.ParseHTTPVersion(s[0:VersionLen])
	if !(ok && major == 1 && (minor == 0 || minor == 1)) {
		return false
	}
	if s[VersionLen] != ' ' {
		return false
	}
	status, ok := atoI(payload[VersionLen+1:VersionLen+4], 10)
	if !ok {
		return false
	}
	// only validate status codes mentioned in rfc2616.
	if http.StatusText(status) == "" {
		return false
	}
	// handle cases from #875
	return payload[VersionLen+4] == ' ' || payload[VersionLen+4] == '\r'
}

// HasRequestTitle reports whether this payload has an HTTP/1 request title
func HasRequestTitle(payload []byte) bool {
	s := byteutils.SliceToString(payload)
	if len(s) < MinRequestCount {
		return false
	}
	titleLen := bytes.Index(payload, CRLF)
	if titleLen == -1 {
		return false
	}
	if strings.Count(s[:titleLen], " ") != 2 {
		return false
	}
	method := string(Method(payload))
	var methodFound bool
	for _, m := range Methods {
		if methodFound = method == m; methodFound {
			break
		}
	}
	if !methodFound {
		return false
	}
	path := strings.Index(s[len(method)+1:], " ")
	if path == -1 {
		return false
	}
	major, minor, ok := http.ParseHTTPVersion(s[path+len(method)+2 : titleLen])
	return ok && major == 1 && (minor == 0 || minor == 1)
}

// HasTitle reports if this payload has an http/1 title
func HasTitle(payload []byte) bool {
	return HasRequestTitle(payload) || HasResponseTitle(payload)
}

// CheckChunked checks HTTP/1 chunked data integrity(https://tools.ietf.org/html/rfc7230#section-4.1)
// and returns the length of total valid scanned chunks(including chunk size, extensions and CRLFs) and
// full is true if all chunks was scanned.
func CheckChunked(bufs ...[]byte) (chunkEnd int, full bool) {
	var buf []byte
	if len(bufs) > 0 {
		buf = bufs[0]
	}
	for chunkEnd < len(buf) {
		sz := bytes.IndexByte(buf[chunkEnd:], '\r')
		if sz < 1 {
			break
		}
		// don't parse chunk extensions https://github.com/golang/go/issues/13135.
		// chunks extensions are no longer a thing, but we do check if the byte
		// following the parsed hex number is ';'
		sz += chunkEnd
		chkLen, ok := atoI(buf[chunkEnd:sz], 16)
		if !ok && bytes.IndexByte(buf[chunkEnd:sz], ';') < 1 {
			break
		}
		sz++ // + '\n'
		// total length = SIZE + CRLF + OCTETS + CRLF
		allChunk := sz + chkLen + 2
		if allChunk >= len(buf) ||
			buf[sz]&buf[allChunk] != '\n' ||
			buf[allChunk-1] != '\r' {
			break
		}
		chunkEnd = allChunk + 1
		if chkLen == 0 {
			full = true
			break
		}
	}
	return
}

// ProtocolStateSetter is an interface used to provide protocol state for future use
type ProtocolStateSetter interface {
	SetProtocolState(interface{})
	ProtocolState() interface{}
}

type HTTPState struct {
	Body           int // body index
	HeaderStart    int
	HeaderEnd      int
	HeaderParsed   bool // we checked necessary headers
	HasFullPayload bool // all chunks has been parsed
	IsChunked      bool // Transfer-Encoding: chunked
	BodyLen        int  // Content-Length's value
	HasTrailer     bool // Trailer header?
	Continue100    bool
}

// HasFullPayload checks if this message has full or valid payloads and returns true.
// Message param is optional but recommended on cases where 'data' is storing
// partial-to-full stream of bytes(packets).
func HasFullPayload(m ProtocolStateSetter, payloads ...[]byte) bool {
	var state *HTTPState
	if m != nil {
		state, _ = m.ProtocolState().(*HTTPState)
	}
	if state == nil {
		state = new(HTTPState)
		if m != nil {
			m.SetProtocolState(state)
		}
	}

	// Http Packets can only start with a few things, check if this is one of them
	if len(payloads) == 0 {
		return false
	}
	if !HasRequestTitle(payloads[0]) && !HasResponseTitle(payloads[0]) {
		return false
	}

	if state.HeaderStart < 1 {
		for _, data := range payloads {
			state.HeaderStart = MIMEHeadersStartPos(data)
			if state.HeaderStart < 0 {
				return false
			} else {
				break
			}
		}
	}

	if state.Body < 1 || state.HeaderEnd < 1 {
		var pos int
		for _, data := range payloads {
			endPos := MIMEHeadersEndPos(data)
			if endPos < 0 {
				pos += len(data)
			} else {
				pos += endPos
				state.HeaderEnd = pos
			}

			if endPos > 0 {
				state.Body = pos
				break
			}
		}
	}

	if state.HeaderEnd < 1 {
		return false
	}

	if !state.HeaderParsed {
		var pos int
		for _, data := range payloads {
			chunked := Header(data, []byte("Transfer-Encoding"))

			if len(chunked) > 0 && bytes.Index(data, []byte("chunked")) > 0 {
				state.IsChunked = true
				// trailers are generally not allowed in non-chunks body
				state.HasTrailer = len(Header(data, []byte("Trailer"))) > 0
			} else {
				contentLen := Header(data, []byte("Content-Length"))
				state.BodyLen, _ = atoI(contentLen, 10)
			}

			pos += len(data)

			if string(Header(data, []byte("Expect"))) == "100-continue" {
				state.Continue100 = true
			}

			if state.BodyLen > 0 || pos >= state.Body {
				state.HeaderParsed = true
				break
			}
		}
	}

	bodyLen := 0
	for _, data := range payloads {
		bodyLen += len(data)
	}
	bodyLen -= state.Body

	if state.IsChunked {
		// check chunks
		if bodyLen < 1 {
			return false
		}

		// check trailer headers
		toCheck := getCheckBytes()
		if state.HasTrailer {
			if bytes.HasSuffix(toCheck, []byte("\r\n\r\n")) {
				return true
			}
		} else {
			if bytes.HasSuffix(toCheck, []byte("0\r\n\r\n")) {
				state.HasFullPayload = true
				return true
			}
		}

		return false
	}

	// check for content-length header
	return state.BodyLen == bodyLen
}

func getCheckBytes(payloads ...[]byte) []byte {
	out := make([]byte, 5)
	if len(payloads) >= 2 {
		i, j := len(payloads[len(payloads)-2])-1, len(payloads[len(payloads)-1])-1
		k := 4
		for k >= 0 {
			if j >= 0 {
				out[k] = payloads[len(payloads)-1][j]
				j--
			} else if i >= 0 {
				out[k] = payloads[len(payloads)-2][i]
				i--
			}
			k--
		}
	} else {
		out = payloads[len(payloads)-1]
	}
	return out
}

// this works with positive integers
func atoI(s []byte, base int) (num int, ok bool) {
	var v int
	ok = true
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			ok = false
			break
		}
		v = int(hexTable[s[i]])
		if v >= base || (v == 0 && s[i] != '0') {
			ok = false
			break
		}
		num = (num * base) + v
	}
	return
}

var hexTable = [128]byte{
	'0': 0,
	'1': 1,
	'2': 2,
	'3': 3,
	'4': 4,
	'5': 5,
	'6': 6,
	'7': 7,
	'8': 8,
	'9': 9,
	'A': 10,
	'a': 10,
	'B': 11,
	'b': 11,
	'C': 12,
	'c': 12,
	'D': 13,
	'd': 13,
	'E': 14,
	'e': 14,
	'F': 15,
	'f': 15,
}
