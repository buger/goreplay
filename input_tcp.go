package main

import (
	"bufio"
	"io"
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
	buffer_size int // maximum size buffer in KB for listener
}

func NewTCPInput(address string, buffer_size int) (i *TCPInput) {
	i = new(TCPInput)
	i.data = make(chan []byte)
	i.address = address
	i.buffer_size = buffer_size

	i.listen(address)

	return
}

func (i *TCPInput) Read(data []byte) (int, error) {
	buf := <-i.data
	copy(data, buf)

	return len(buf), nil
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

func (i *TCPInput) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReaderSize(conn,i.buffer_size * 1024 + 2)

	for {
		buf,err := reader.ReadBytes('¶')
		buf_len := len(buf)
		if buf_len > 0 {
			new_buf_len := len(buf) - 2
			if new_buf_len > 2 {
				new_buf := make([]byte, new_buf_len)
				copy(new_buf, buf[:new_buf_len])
				i.data <- new_buf
				reader.Reset(conn)
				if err != nil {
					if err != io.EOF {
						log.Printf("error: %s\n", err)
					}
				}
			}
		}
	}
}

func (i *TCPInput) String() string {
	return "TCP input: " + i.address
}
