package main

import (
	"crypto/tls"
	"github.com/buger/gor/proto"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

var defaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
}

type HTTPClientConfig struct {
	FollowRedirects int
	Debug           bool
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
		u.Host += ":" + defaultPorts[u.Scheme]
	}

	client := new(HTTPClient)
	client.baseURL = u.String()
	client.host = u.Host
	client.scheme = u.Scheme
	client.respBuf = make([]byte, 4096*10)
	client.config = config

	return client
}

func (c *HTTPClient) Connect() (err error) {
	c.Disconnect()

	c.conn, err = net.Dial("tcp", c.host)

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
		Debug("Disconnected: ", c.baseURL)
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
	if c.conn == nil || !c.isAlive() {
		Debug("Connecting:", c.baseURL)
		if err = c.Connect(); err != nil {
			return
		}
	}

	timeout := time.Now().Add(5 * time.Second)

	c.conn.SetWriteDeadline(timeout)

	data = proto.SetHost(data, []byte(c.baseURL), []byte(c.host))

	if c.config.Debug {
		Debug("Sending:", string(data))
	}

	if _, err = c.conn.Write(data); err != nil {
		Debug("Write error:", err, c.baseURL)
		return
	}

	c.conn.SetReadDeadline(timeout)
	n, err := c.conn.Read(c.respBuf)

	if err != nil {
		Debug("READ ERRORR!", err, c.conn)
		return
	}

	payload := c.respBuf[:n]

	if c.config.Debug {
		Debug("Received:", string(payload))
	}

	if c.config.FollowRedirects > 0 && c.redirectsCount < c.config.FollowRedirects {
		status := payload[9:12]

		// 3xx requests
		if status[0] == '3' {
			c.redirectsCount += 1

			location, _, _, _ := proto.Header(payload, []byte("Location"))
			redirectPayload := []byte("GET " + string(location) + " HTTP/1.1\r\n\r\n")

			if c.config.Debug {
				Debug("Redirecting to: " + string(location))
			}

			return c.Send(redirectPayload)
		}
	}

	c.redirectsCount = 0

	return payload, err
}
