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
	"bytes"
	"github.com/buger/gor/byteutils"
)

// In HTTP newline defined by 2 bytes (for both windows and *nix support)
var CLRF = []byte("\r\n")

// New line acts as separator: end of Headers or Body (in some cases)
var EmptyLine = []byte("\r\n\r\n")

// Separator for Header line. Header looks like: `HeaderName: value`
var HeaderDelim = []byte(": ")

// MIMEHeadersEndPos finds end of the Headers section, which should end with empty line.
func MIMEHeadersEndPos(payload []byte) int {
	return bytes.Index(payload, EmptyLine) + 4
}

// MIMEHeadersStartPos finds start of Headers section
// It just finds position of second line (first contains location and method).
func MIMEHeadersStartPos(payload []byte) int {
	return bytes.Index(payload, CLRF) + 2 // Find first line end
}

func isLower(b byte) bool {
	if 'a' <= b && b <= 'z' {
		return true
	}

	return false
}

func toUpper(b byte) byte {
	if 'a' <= b && b <= 'z' {
		b -= 'a' - 'A'
	}
	return b
}

func toLower(b byte) byte {
	if 'A' <= b && b <= 'Z' {
		b += 'a' - 'A'
	}
	return b
}

func headerIndex(payload []byte, name []byte) int {
	isLower := isLower(name[0])
	i := 0

	for {
		if i >= len(payload) {
			return -1
		}

		if payload[i] == '\n' {
			i++

			// We are at the end
			if i == len(payload) {
				return -1
			}

			if payload[i] == name[0] ||
				(!isLower && payload[i] == toLower(name[0])) ||
				(isLower && payload[i] == toUpper(name[0])) {

				i++
				j := 1
				for {
					if j == len(name) {
						// Matched, and return start of the header
						return i - len(name)
					}

					if payload[i] != name[j] {
						break
					}

					// If compound header name do one more case check: Content-Length or Transfer-Encoding
					if name[j] == '-' {
						i++
						j++

						if !(payload[i] == name[j] ||
							(!isLower && payload[i] == toLower(name[j])) ||
							(isLower && payload[i] == toUpper(name[j]))) {
							break
						}
					}

					j++
					i++
				}
			}
		}

		i++
	}

	return -1
}

// header return value and positions of header/value start/end.
// If not found, value will be blank, and headerStart will be -1
// Do not support multi-line headers.
func header(payload []byte, name []byte) (value []byte, headerStart, headerEnd, valueStart, valueEnd int) {
	headerStart = headerIndex(payload, name)

	if headerStart == -1 {
		return
	}

	valueStart = headerStart + len(name) + 1 // Skip ":" after header name
	headerEnd = valueStart + bytes.IndexByte(payload[valueStart:], '\n')

	for valueStart < headerEnd { // Ignore empty space after ':'
		if payload[valueStart] == ' ' {
			valueStart++
		} else {
			break
		}
	}

	valueEnd = valueStart + bytes.IndexByte(payload[valueStart:], '\n')

	if payload[headerEnd-1] == '\r' {
		valueEnd--
	}

	// ignore empty space at end of header value
	for valueStart < valueEnd {
		if payload[valueEnd-1] == ' ' {
			valueEnd--
		} else {
			break
		}
	}
	value = payload[valueStart:valueEnd]

	return
}

// Works only with ASCII
func HeadersEqual(h1 []byte, h2 []byte) bool {
	if len(h1) != len(h2) {
		return false
	}

	for i, c1 := range h1 {
		c2 := h2[i]

		switch int(c1) - int(c2) {
		case 0, 32, -32:
		default:
			return false
		}
	}

	return true
}

// Parsing headers from multiple payloads
func ParseHeaders(payloads [][]byte, cb func(header []byte, value []byte) bool) {
	hS := [2]int{0, 0}   // header start
	hE := [2]int{-1, -1} // header end
	vS := [2]int{-1, -1} // value start
	vE := [2]int{-1, -1} // value end

	i := 0
	pIdx := 0
	lineBreaks := 0
	newLineBreak := true

	for {
		if len(payloads)-1 < pIdx {
			break
		}

		p := payloads[pIdx]

		if len(p)-1 < i {
			pIdx++
			i = 0
			continue
		}

		switch p[i] {
		case '\r', '\n':
			newLineBreak = true
			lineBreaks++

			// End of headers
			if lineBreaks == 4 {
				return
			}

			if lineBreaks > 1 {
				break
			}

			vE = [2]int{pIdx, i}

			if vS[1] != -1 && vE[1] != -1 &&
				hS[1] != -1 && hE[1] != -1 {

				var header, value []byte

				phS, phE, pvS, pvE := payloads[hS[0]], payloads[hE[0]], payloads[vS[0]], payloads[vE[0]]

				// If in same payload
				if hS[0] == hE[0] {
					header = phS[hS[1]:hE[1]]
				} else {
					header = make([]byte, len(phS)-hS[1]+hE[1])
					copy(header, phS[hS[1]:])
					copy(header[len(phS)-hS[1]:], phE[:hE[1]])
				}

				if vS[0] == vE[0] {
					value = pvS[vS[1]:vE[1]]
				} else {
					value = make([]byte, len(pvS)-vS[1]+vE[1])
					copy(value, pvS[vS[1]:])
					copy(value[len(pvS)-vS[1]:], pvE[:vE[1]])
				}

				if !cb(header, value) {
					return
				}
			}

			// Header found, reset values
			vS = [2]int{-1, -1}
			vE = [2]int{-1, -1}
			hS = [2]int{-1, -1}
			hE = [2]int{-1, -1}
		case ':':
			if newLineBreak {
				hE = [2]int{pIdx, i}
				newLineBreak = false
			}
			lineBreaks = 0
		default:
			lineBreaks = 0

			if hS[1] == -1 {
				hS = [2]int{pIdx, i}
				hE = [2]int{-1, -1}
			} else {
				if hE[1] == -1 {
					break
				}

				if vS[1] == -1 {
					if p[i] == ' ' {
						break
					}

					vS = [2]int{pIdx, i}
				}
			}
		}

		i++
	}

	return
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
		return byteutils.Replace(payload, vs, ve, value)
	}

	return AddHeader(payload, name, value)
}

// AddHeader takes http payload and appends new header to the start of headers section
// Returns modified request payload
func AddHeader(payload, name, value []byte) []byte {
	header := make([]byte, len(name)+2+len(value)+2)
	copy(header[0:], name)
	copy(header[len(name):], HeaderDelim)
	copy(header[len(name)+2:], value)
	copy(header[len(header)-2:], CLRF)

	mimeStart := MIMEHeadersStartPos(payload)

	return byteutils.Insert(payload, mimeStart, header)
}

// DelHeader takes http payload and removes header name from headers section
// Returns modified request payload
func DeleteHeader(payload, name []byte) []byte {
	_, hs, he, _, _ := header(payload, name)
	if hs != -1 {
		newHeader := make([]byte, len(payload)-(he-hs)-1)
		copy(newHeader[:hs], payload[:hs])
		copy(newHeader[hs:], payload[he+1:])
		return newHeader
	}
	return payload
}

// Body returns request/response body
func Body(payload []byte) []byte {
	// 4 -> len(EMPTY_LINE)
	return payload[MIMEHeadersEndPos(payload):]
}

// Path takes payload and retuns request path: Split(firstLine, ' ')[1]
func Path(payload []byte) []byte {
	start := bytes.IndexByte(payload, ' ') + 1
	end := bytes.IndexByte(payload[start:], ' ')

	return payload[start : start+end]
}

// SetPath takes payload, sets new path and returns modified payload
func SetPath(payload, path []byte) []byte {
	start := bytes.IndexByte(payload, ' ') + 1
	end := bytes.IndexByte(payload[start:], ' ')

	return byteutils.Replace(payload, start, start+end, path)
}

// PathParam returns URL query attribute by given name, if no found: valueStart will be -1
func PathParam(payload, name []byte) (value []byte, valueStart, valueEnd int) {
	path := Path(payload)

	if paramStart := bytes.Index(path, append(name, '=')); paramStart != -1 {
		valueStart := paramStart + len(name) + 1
		paramEnd := bytes.IndexByte(path[valueStart:], '&')

		// Param can end with '&' (another param), or end of line
		if paramEnd == -1 { // It is final param
			paramEnd = len(path)
		} else {
			paramEnd += valueStart
		}

		return path[valueStart:paramEnd], valueStart, paramEnd
	}

	return []byte(""), -1, -1
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

	return payload[:end]
}

// Status returns response status.
// It happend to be in same position as request payload path
func Status(payload []byte) []byte {
	return Path(payload)
}

var httpMethods []string = []string{
	"GET ", "OPTI", "HEAD", "POST", "PUT ", "DELE", "TRAC", "CONN", "PATC" /* custom methods */, "BAN", "PURG",
}

func IsHTTPPayload(payload []byte) bool {
	if len(payload) < 4 {
		return false
	}

	method := string(payload[0:4])

	for _, m := range httpMethods {
		if method == m {
			return true
		}
	}
	return false
}
