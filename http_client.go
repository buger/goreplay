package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"io"
	"log"
	"net"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/buger/goreplay/proto"
)

var httpMu sync.Mutex

const (
	readChunkSize   = 64 * 1024
	maxResponseSize = 1073741824
)

var chunkedSuffix = []byte("0\r\n\r\n")

var defaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
}

type HTTPClientConfig struct {
	FollowRedirects    int
	Debug              bool
	OriginalHost       bool
	ConnectionTimeout  time.Duration
	Timeout            time.Duration
	ResponseBufferSize int
}

type HTTPClient struct {
	baseURL        string
	scheme         string
	host           string
	auth           string
	conn           net.Conn
	respBuf        []byte
	config         *HTTPClientConfig
	redirectsCount int
}

func NewHTTPClient(baseURL string, config *HTTPClientConfig) *HTTPClient {
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "http://" + baseURL
	}

	u, _ := url.Parse(baseURL)

	if config.Timeout == 0 {
		config.Timeout = time.Second
	}

	config.ConnectionTimeout = config.Timeout

	if config.ResponseBufferSize == 0 {
		config.ResponseBufferSize = 100 * 1024 // 100kb
	}

	client := new(HTTPClient)
	client.baseURL = u.String()
	client.host = u.Host
	client.scheme = u.Scheme
	client.respBuf = make([]byte, config.ResponseBufferSize)
	client.config = config

	if u.User != nil {
		client.auth = "Basic " + base64.StdEncoding.EncodeToString([]byte(u.User.String()))
	}

	return client
}

func (c *HTTPClient) Connect() (err error) {
	c.Disconnect()

	if !strings.Contains(c.host, ":") {
		c.conn, err = net.DialTimeout("tcp", c.host+":"+defaultPorts[c.scheme], c.config.ConnectionTimeout)
	} else {
		c.conn, err = net.DialTimeout("tcp", c.host, c.config.ConnectionTimeout)
	}

	if c.scheme == "https" {
		tlsConn := tls.Client(c.conn, &tls.Config{InsecureSkipVerify: true, ServerName: c.host})

		if err = tlsConn.Handshake(); err != nil {
			return
		}

		c.conn = tlsConn
	}

	return
}

func (c *HTTPClient) Disconnect() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		Debug("[HTTP] Disconnected: ", c.baseURL)
	}
}

func (c *HTTPClient) isAlive() bool {
	one := make([]byte, 1)

	// Ready 1 byte from socket without timeout to check if it not closed
	c.conn.SetReadDeadline(time.Now().Add(time.Millisecond))
	_, err := c.conn.Read(one)

	if err == nil {
		return true
	} else if err == io.EOF {
		Debug("[HTTPClient] connection closed, reconnecting")
		return false
	} else if err == syscall.EPIPE {
		Debug("Detected broken pipe.", err)
		return false
	}

	return true
}

func (c *HTTPClient) Send(data []byte) (response []byte, err error) {
	var payload []byte

	// Don't exit on panic
	defer func() {
		if r := recover(); r != nil {
			Debug("[HTTPClient]", r, string(data))

			if _, ok := r.(error); ok {
				log.Println("[HTTPClient] Failed to send request: ", string(data))
				log.Println("[HTTPClient] Response: ", string(response))
				log.Println("PANIC: pkg:", r, string(debug.Stack()))
			}
		}
	}()

	if c.conn == nil || !c.isAlive() {
		Debug("[HTTPClient] Connecting:", c.baseURL)
		if err = c.Connect(); err != nil {
			log.Println("[HTTPClient] Connection error:", err)
			response = errorPayload(HTTP_CONNECTION_ERROR)
			return
		}
	}

	timeout := time.Now().Add(c.config.Timeout)

	c.conn.SetWriteDeadline(timeout)

	if !c.config.OriginalHost {
		data = proto.SetHost(data, []byte(c.baseURL), []byte(c.host))
	}

	if c.auth != "" {
		data = proto.SetHeader(data, []byte("Authorization"), []byte(c.auth))
	}

	if c.config.Debug {
		Debug("[HTTPClient] Sending:", string(data))
	}

	if _, err = c.conn.Write(data); err != nil {
		Debug("[HTTPClient] Write error:", err, c.baseURL)
		response = errorPayload(HTTP_TIMEOUT)
		return
	}

	var readBytes, n int
	var currentChunk []byte
	timeout = time.Now().Add(c.config.Timeout)
	chunked := false
	contentLength := -1
	currentContentLength := 0
	chunks := 0

	for {
		c.conn.SetReadDeadline(timeout)

		if readBytes < len(c.respBuf) {
			n, err = c.conn.Read(c.respBuf[readBytes:])
			readBytes += n
			chunks++

			// First chunk
			if chunked || contentLength != -1 {
				currentContentLength += n
			} else {
				// If headers are finished

				if bytes.Contains(c.respBuf[:readBytes], proto.EmptyLine) {
					if bytes.Equal(proto.Header(c.respBuf[:readBytes], []byte("Transfer-Encoding")), []byte("chunked")) {
						chunked = true
					} else {
						status, _ := strconv.Atoi(string(proto.Status(c.respBuf[:readBytes])))
						if (status >= 100 && status < 200) || status == 204 || status == 304 {
							contentLength = 0
							break
						} else {
							l := proto.Header(c.respBuf[:readBytes], []byte("Content-Length"))
							if len(l) > 0 {
								contentLength, _ = strconv.Atoi(string(l))
							}
						}
					}

					currentContentLength += len(proto.Body(c.respBuf[:readBytes]))
				}
			}

			if chunked {
				// Check if chunked message finished
				if bytes.HasSuffix(c.respBuf[:readBytes], chunkedSuffix) {
					break
				}
			} else if contentLength != -1 {
				if currentContentLength > contentLength {
					Debug("[HTTPClient] disconnected, wrong length", currentContentLength, contentLength)
					c.Disconnect()
					break
				} else if currentContentLength == contentLength {
					break
				}
			}

			if err != nil {
				if err == io.EOF {
					err = nil
				}
				break
			}
		} else {
			if currentChunk == nil {
				currentChunk = make([]byte, readChunkSize)
			}

			n, err = c.conn.Read(currentChunk)

			readBytes += int(n)
			chunks++
			currentContentLength += n

			if chunked {
				// Check if chunked message finished
				if bytes.HasSuffix(currentChunk[:n], chunkedSuffix) {
					break
				}
			} else if contentLength != -1 {
				if currentContentLength > contentLength {
					Debug("[HTTPClient] disconnected, wrong length", currentContentLength, contentLength)
					c.Disconnect()
					break
				} else if currentContentLength == contentLength {
					break
				}
			} else {
				Debug("[HTTPClient] disconnected, can't find Content-Length or Chunked")
				c.Disconnect()
				break
			}

			if err == io.EOF {
				break
			} else if err != nil {
				Debug("[HTTPClient] Read the whole body error:", err, c.baseURL)
				break
			}

		}

		if readBytes >= maxResponseSize {
			Debug("[HTTPClient] Body is more than the max size", maxResponseSize,
				c.baseURL)
			break
		}

		// For following chunks expect less timeout
		timeout = time.Now().Add(c.config.Timeout / 5)
	}

	if err != nil && readBytes == 0 {
		Debug("[HTTPClient] Response read timeout error", err, c.conn, readBytes, string(c.respBuf[:readBytes]))
		response = errorPayload(HTTP_TIMEOUT)
		c.Disconnect()
		return
	}

	if readBytes < 4 || string(c.respBuf[:4]) != "HTTP" {
		Debug("[HTTPClient] Response read unknown error", err, c.conn, readBytes, string(c.respBuf[:readBytes]))
		response = errorPayload(HTTP_UNKNOWN_ERROR)
		c.Disconnect()
		return
	}

	if readBytes > len(c.respBuf) {
		readBytes = len(c.respBuf)
	}
	payload = make([]byte, readBytes)
	copy(payload, c.respBuf[:readBytes])

	if c.config.Debug {
		Debug("[HTTPClient] Received:", string(payload))
	}

	if c.config.FollowRedirects > 0 && c.redirectsCount < c.config.FollowRedirects {
		status := payload[9:12]

		// 3xx requests
		if status[0] == '3' {
			c.redirectsCount++

			location := proto.Header(payload, []byte("Location"))
			redirectPayload := []byte("GET " + string(location) + " HTTP/1.1\r\n\r\n")

			if c.config.Debug {
				Debug("[HTTPClient] Redirecting to: " + string(location))
			}

			return c.Send(redirectPayload)
		}
	}

	if bytes.Equal(proto.Status(payload), []byte("400")) {
		Debug("[HTTPClient] Closed connection on 400 response")
		c.Disconnect()
	}

	c.redirectsCount = 0

	return payload, err
}

func (c *HTTPClient) Get(path string) (response []byte, err error) {
	payload := "GET " + path + " HTTP/1.1\r\n\r\n"

	return c.Send([]byte(payload))
}

func (c *HTTPClient) Post(path string, body []byte) (response []byte, err error) {
	payload := "POST " + path + " HTTP/1.1\r\n"
	payload += "Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n"
	payload += string(body)

	return c.Send([]byte(payload))
}

const (
	// https://support.cloudflare.com/hc/en-us/articles/200171936-Error-520-Web-server-is-returning-an-unknown-error
	HTTP_UNKNOWN_ERROR = "520"
	// https://support.cloudflare.com/hc/en-us/articles/200171916-Error-521-Web-server-is-down
	HTTP_CONNECTION_ERROR = "521"
	// https://support.cloudflare.com/hc/en-us/articles/200171906-Error-522-Connection-timed-out
	HTTP_CONNECTION_TIMEOUT = "522"
	// https://support.cloudflare.com/hc/en-us/articles/200171946-Error-523-Origin-is-unreachable
	HTTP_UNREACHABLE = "523"
	// https://support.cloudflare.com/hc/en-us/articles/200171926-Error-524-A-timeout-occurred
	HTTP_TIMEOUT = "524"
)

var errorPayloadTemplate = "HTTP/1.1 202 Accepted\r\nDate: Mon, 17 Aug 2015 14:10:11 GMT\r\nContent-Length: 0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n"

func errorPayload(errorCode string) []byte {
	payload := make([]byte, len(errorPayloadTemplate))
	copy(payload, errorPayloadTemplate)

	copy(payload[29:58], []byte(time.Now().Format(time.RFC1123)))
	copy(payload[9:12], errorCode)

	return payload
}
