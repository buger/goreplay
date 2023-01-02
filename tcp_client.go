package goreplay

import (
	"crypto/tls"
	"io"
	"net"
	"runtime/debug"
	"syscall"
	"time"
)

// TCPClientConfig client configuration
type TCPClientConfig struct {
	Debug              bool
	ConnectionTimeout  time.Duration
	Timeout            time.Duration
	ResponseBufferSize int
	Secure             bool
}

// TCPClient client connection properties
type TCPClient struct {
	baseURL        string
	addr           string
	conn           net.Conn
	respBuf        []byte
	config         *TCPClientConfig
	redirectsCount int
}

// NewTCPClient returns new TCPClient
func NewTCPClient(addr string, config *TCPClientConfig) *TCPClient {
	if config.Timeout.Nanoseconds() == 0 {
		config.Timeout = 5 * time.Second
	}

	config.ConnectionTimeout = config.Timeout

	if config.ResponseBufferSize == 0 {
		config.ResponseBufferSize = 100 * 1024 // 100kb
	}

	client := &TCPClient{config: config, addr: addr}
	client.respBuf = make([]byte, config.ResponseBufferSize)

	return client
}

// Connect creates a tcp connection of the client
func (c *TCPClient) Connect() (err error) {
	c.Disconnect()

	c.conn, err = net.DialTimeout("tcp", c.addr, c.config.ConnectionTimeout)

	if c.config.Secure {
		tlsConn := tls.Client(c.conn, &tls.Config{InsecureSkipVerify: true})

		if err = tlsConn.Handshake(); err != nil {
			return
		}

		c.conn = tlsConn
	}

	return
}

// Disconnect closes the client connection
func (c *TCPClient) Disconnect() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		Debug(1, "[TCPClient] Disconnected: ", c.baseURL)
	}
}

func (c *TCPClient) isAlive() bool {
	one := make([]byte, 1)

	// Ready 1 byte from socket without timeout to check if it not closed
	c.conn.SetReadDeadline(time.Now().Add(time.Millisecond))
	_, err := c.conn.Read(one)

	if err == nil {
		return true
	} else if err == io.EOF {
		Debug(1, "[TCPClient] connection closed, reconnecting")
		return false
	} else if err == syscall.EPIPE {
		Debug(1, "Detected broken pipe.", err)
		return false
	}

	return true
}

// Send sends data over created tcp connection
func (c *TCPClient) Send(data []byte) (response []byte, err error) {
	// Don't exit on panic
	defer func() {
		if r := recover(); r != nil {
			Debug(1, "[TCPClient]", r, string(data))

			if _, ok := r.(error); !ok {
				Debug(1, "[TCPClient] Failed to send request: ", string(data))
				Debug(1, "PANIC: pkg:", r, debug.Stack())
			}
		}
	}()

	if c.conn == nil || !c.isAlive() {
		Debug(1, "[TCPClient] Connecting:", c.baseURL)
		if err = c.Connect(); err != nil {
			Debug(1, "[TCPClient] Connection error:", err)
			return
		}
	}

	timeout := time.Now().Add(c.config.Timeout)

	c.conn.SetWriteDeadline(timeout)

	if c.config.Debug {
		Debug(1, "[TCPClient] Sending:", string(data))
	}

	if _, err = c.conn.Write(data); err != nil {
		Debug(1, "[TCPClient] Write error:", err, c.baseURL)
		return
	}

	var readBytes, n int
	var currentChunk []byte
	timeout = time.Now().Add(c.config.Timeout)

	for {
		c.conn.SetReadDeadline(timeout)

		if readBytes < len(c.respBuf) {
			n, err = c.conn.Read(c.respBuf[readBytes:])
			readBytes += n

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

			if err == io.EOF {
				break
			} else if err != nil {
				Debug(1, "[TCPClient] Read the whole body error:", err, c.baseURL)
				break
			}

			readBytes += int(n)
		}

		if readBytes >= maxResponseSize {
			Debug(1, "[TCPClient] Body is more than the max size", maxResponseSize,
				c.baseURL)
			break
		}

		// For following chunks expect less timeout
		timeout = time.Now().Add(c.config.Timeout / 5)
	}

	if err != nil {
		Debug(1, "[TCPClient] Response read error", err, c.conn, readBytes)
		return
	}

	if readBytes > len(c.respBuf) {
		readBytes = len(c.respBuf)
	}

	payload := make([]byte, readBytes)
	copy(payload, c.respBuf[:readBytes])

	if c.config.Debug {
		Debug(1, "[TCPClient] Received:", string(payload))
	}

	return payload, err
}
