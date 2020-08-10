package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

// TCPInput used for internal communication
type TCPInput struct {
	data     chan []byte
	listener net.Listener
	address  string
	config   *TCPInputConfig
	stop     chan bool // Channel used only to indicate goroutine should shutdown
}

type TCPInputConfig struct {
	Secure          bool   `json:"input-tcp-secure" mapstructure:"input-tcp-secure"`
	CertificatePath string `json:"input-tcp-certificate" mapstructure:"input-tcp-certificate"`
	KeyPath         string `json:"input-tcp-certificate-key" mapstructure:"input-tcp-certificate-key"`
}

// NewTCPInput constructor for TCPInput, accepts address with port
func NewTCPInput(address string, config *TCPInputConfig) (i *TCPInput) {
	i = new(TCPInput)
	i.data = make(chan []byte, 1000)
	i.address = address
	i.config = config
	i.stop = make(chan bool)

	i.listen(address)

	return
}

func (i *TCPInput) Read(data []byte) (int, error) {
	var buf []byte
	select {
	case <-i.stop:
		return 0, ErrorStopped
	case buf = <-i.data:
	}
	copy(data, buf)

	return len(buf), nil
}

func (i *TCPInput) Close() error {
	close(i.stop)
	return nil
}

func (i *TCPInput) listen(address string) {
	if i.config.Secure {
		cer, err := tls.LoadX509KeyPair(i.config.CertificatePath, i.config.KeyPath)
		if err != nil {
			log.Fatal("Error while loading --input-file certificate:", err)
		}

		config := &tls.Config{Certificates: []tls.Certificate{cer}}
		listener, err := tls.Listen("tcp", address, config)
		if err != nil {
			log.Fatal("Can't start --input-tcp with secure connection:", err)
		}
		i.listener = listener
	} else {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			log.Fatal("Can't start:", err)
		}

		i.listener = listener
	}

	go func() {
		for {
			conn, err := i.listener.Accept()

			if err != nil {
				log.Println("Error while Accept()", err)
				continue
			}

			go i.handleConnection(conn)
		}
	}()
}

func (i *TCPInput) handleConnection(conn net.Conn) {
	defer conn.Close()

	payloadSeparatorAsBytes := []byte(payloadSeparator)
	reader := bufio.NewReader(conn)
	var buffer bytes.Buffer

	for {
		line, err := reader.ReadBytes('\n')

		if err != nil {
			if err != io.EOF {
				fmt.Fprintln(os.Stderr, "Unexpected error in input tcp connection:", err)
			}
			break

		}

		if bytes.Equal(payloadSeparatorAsBytes[1:], line) {
			asBytes := buffer.Bytes()
			buffer.Reset()

			newBuf := make([]byte, len(asBytes)-1)
			copy(newBuf, asBytes)

			i.data <- newBuf
		} else {
			buffer.Write(line)
		}
	}
}

func (i *TCPInput) String() string {
	return "TCP input: " + i.address
}
