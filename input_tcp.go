package gor

import (
	"bufio"
	"bytes"
	"log"
	"net"
)

// Can be tested using nc tool:
//    echo "asdad" | nc 127.0.0.1 27017
//
type TCPInput struct {
	data     chan []byte
	address  string
	listener net.Listener
}

func NewTCPInput(address string) (i *TCPInput) {
	i = new(TCPInput)
	i.data = make(chan []byte)
	i.address = address

	i.listen(address)

	return
}

func (i *TCPInput) Read() ([]byte, bool) {
	buf, ok := <-i.data

	return buf, ok
}

func (i *TCPInput) listen(address string) {
	listener, err := net.Listen("tcp", address)
	i.listener = listener

	if err != nil {
		log.Fatal("Can't start:", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()

			if err != nil {
				log.Println("Error while Accept()", err)
				continue
			}

			go i.handleConnection(conn)
		}
	}()
}

func scanBytes(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Search for ¶ symbol
	if i := bytes.IndexByte(data, 194); i >= 0 {
		if data[i+1] == 182 {
			// We have a full newline-terminated line.
			return i + 2, data[0:i], nil
		}
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func (i *TCPInput) handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)

	scanner.Split(scanBytes)

	for scanner.Scan() {
		i.data <- scanner.Bytes()
	}
}

func (i *TCPInput) String() string {
	return "TCP input: " + i.address
}
