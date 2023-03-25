package binary

import (
	"crypto/tls"
	"io"
	"net"
	"syscall"
	"time"
)

const (
	maxResponseSize = 1073741824
	readChunkSize   = 64 * 1024
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

		outputLogger.Warn().Msgf("Disconnected: %s", c.baseURL)
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
		outputLogger.Warn().Msg("connection closed, reconnecting")
		return false
	} else if err == syscall.EPIPE {
		outputLogger.Warn().Msg("broken pipe, reconnecting")
		return false
	}

	return true
}

// Send sends data over created tcp connection
func (c *TCPClient) Send(data []byte) (response []byte, err error) {
	// Don't exit on panic
	defer func() {
		if r := recover(); r != nil {
			outputLogger.Error().Msgf("PANIC: pkg: %v", r)

			if _, ok := r.(error); !ok {
				outputLogger.Error().Stack().Msgf("faile to send request: %s", string(data))
			}
		}
	}()

	if c.conn == nil || !c.isAlive() {
		outputLogger.Info().Msgf("Connecting: %s", c.baseURL)
		if err = c.Connect(); err != nil {
			outputLogger.Error().Err(err).Msgf("Connection error: %s", c.baseURL)
			return
		}
	}

	timeout := time.Now().Add(c.config.Timeout)

	c.conn.SetWriteDeadline(timeout)

	if c.config.Debug {
		outputLogger.Debug().Msgf("Sending: %s", string(data))
	}

	if _, err = c.conn.Write(data); err != nil {
		outputLogger.Error().Err(err).Msgf("Write error: %s", c.baseURL)
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
				outputLogger.Error().Err(err).Msgf("Read error: %s", c.baseURL)
				break
			}

			readBytes += int(n)
		}

		if readBytes >= maxResponseSize {
			outputLogger.Error().Msgf("Body is more than the max size: %d", maxResponseSize)
			break
		}

		// For following chunks expect less timeout
		timeout = time.Now().Add(c.config.Timeout / 5)
	}

	if err != nil {
		outputLogger.Error().Err(err).Msgf("Response read error")
		return
	}

	if readBytes > len(c.respBuf) {
		readBytes = len(c.respBuf)
	}

	payload := make([]byte, readBytes)
	copy(payload, c.respBuf[:readBytes])

	if c.config.Debug {
		outputLogger.Debug().Msgf("Received: %s", string(payload))
	}

	return payload, err
}
