package main

import (
	"crypto/tls"
	"github.com/buger/gor/proto"
	"io"
	"log"
	"net"
	"net/url"
	"runtime/debug"
	"strings"
	"time"
)

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
	if !strings.Contains(u.Host, ":") {
		if u.Scheme != "http" {
			u.Host += ":" + defaultPorts[u.Scheme]
		}
	}

	if config.Timeout.Nanoseconds() == 0 {
		config.Timeout = 5 * time.Second
	}

	if config.ResponseBufferSize == 0 {
		config.ResponseBufferSize = 100 * 1024 // 100kb
	}

	client := new(HTTPClient)
	client.baseURL = u.String()
	client.host = u.Host
	client.scheme = u.Scheme
	client.respBuf = make([]byte, config.ResponseBufferSize)
	client.config = config

	return client
}

func (c *HTTPClient) Connect() (err error) {
	c.Disconnect()

	if !strings.Contains(c.host, ":") {
		c.conn, err = net.DialTimeout("tcp", c.host+":80", c.config.ConnectionTimeout)
	} else {
		c.conn, err = net.DialTimeout("tcp", c.host, c.config.ConnectionTimeout)
	}

	if c.scheme == "https" {
		tlsConn := tls.Client(c.conn, &tls.Config{InsecureSkipVerify: true})

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
	if _, err := c.conn.Read(one); err == io.EOF {
		return false
	}

	return true
}

func (c *HTTPClient) Send(data []byte) (response []byte, err error) {
	// Don't exit on panic
	defer func() {
		if r := recover(); r != nil {
			Debug("[HTTPClient]", r, string(data))

			if _, ok := r.(error); !ok {
				log.Println("[HTTPClient] Failed to send request: ", string(data))
				log.Println("PANIC: pkg:", r, debug.Stack())
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

	if c.config.Debug {
		Debug("[HTTPClient] Sending:", string(data))
	}

	if _, err = c.conn.Write(data); err != nil {
		Debug("[HTTPClient] Write error:", err, c.baseURL)
		response = errorPayload(HTTP_TIMEOUT)
		return
	}

	c.conn.SetReadDeadline(timeout)
	n, err := c.conn.Read(c.respBuf)

	// If response large then our buffer, we need to read all response buffer
	// Otherwise it will corrupt response of next request
	// Parsing response body is non trivial thing, especially with keep-alive
	// Simples case is to to close connection if response too large
	//
	// See https://github.com/buger/gor/issues/184
	if n == len(c.respBuf) {
		c.Disconnect()
	}

	if err != nil {
		Debug("[HTTPClient] Response read error", err, c.conn)
		response = errorPayload(HTTP_TIMEOUT)
		return
	}

	payload := c.respBuf[:n]

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

	c.redirectsCount = 0

	return payload, err
}

func (c *HTTPClient) Get(path string) (response []byte, err error) {
	payload := "GET " + path + " HTTP/1.1\r\n\r\n"

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
