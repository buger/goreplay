package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

// TCPOutput used for sending raw tcp payloads
// Currently used for internal communication between listener and replay server
// Can be used for transfering binary payloads like protocol buffers
type TCPOutput struct {
	address  string
	limit    int
	buf      chan []byte
	bufStats *GorStat
	config   *TCPOutputConfig
}

type TCPOutputConfig struct {
	secure bool
}

// NewTCPOutput constructor for TCPOutput
// Initialize 10 workers which hold keep-alive connection
func NewTCPOutput(address string, config *TCPOutputConfig) io.Writer {
	o := new(TCPOutput)

	o.address = address
	o.config = config

	o.buf = make(chan []byte, 100)
	if Settings.outputTCPStats {
		o.bufStats = NewGorStat("output_tcp")
	}

	for i := 0; i < 10; i++ {
		go o.worker()
	}

	return o
}

func (o *TCPOutput) worker() {
	retries := 1
	conn, err := o.connect(o.address)
	for {
		if err == nil {
			break
		}

		log.Println("Can't connect to aggregator instance, reconnecting in 1 second. Retries:", retries)
		time.Sleep(1 * time.Second)

		conn, err = o.connect(o.address)
		retries++
	}

	if retries > 0 {
		log.Println("Connected to aggregator instance after ", retries, " retries")
	}

	defer conn.Close()

	for {
		conn.Write(<-o.buf)
		_, err := conn.Write([]byte(payloadSeparator))

		if err != nil {
			log.Println("Lost connection with aggregator instance, reconnecting")
			go o.worker()
			break
		}
	}
}

func (o *TCPOutput) Write(data []byte) (n int, err error) {
	if !isOriginPayload(data) {
		return len(data), nil
	}

	// We have to copy, because sending data in multiple threads
	newBuf := make([]byte, len(data))
	copy(newBuf, data)

	o.buf <- newBuf

	if Settings.outputTCPStats {
		o.bufStats.Write(len(o.buf))
	}

	return len(data), nil
}

func (o *TCPOutput) connect(address string) (conn net.Conn, err error) {
	if o.config.secure {
		conn, err = tls.Dial("tcp", address, &tls.Config{})
	} else {
		conn, err = net.Dial("tcp", address)
	}

	return
}

func (o *TCPOutput) String() string {
	return fmt.Sprintf("TCP output %s, limit: %d", o.address, o.limit)
}
