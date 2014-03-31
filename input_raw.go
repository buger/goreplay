package main

import (
	raw "github.com/joekiller/gor/raw_socket_listener"
	"log"
	"net"
	"strings"
)

type RAWInput struct {
	data    chan []byte
	address string
}

func NewRAWInput(address string, buffer_size int) (i *RAWInput) {
	i = new(RAWInput)
	i.data = make(chan []byte)
	i.address = address

	go i.listen(address, buffer_size)

	return
}

func (i *RAWInput) Read(data []byte) (int, error) {
	buf := <-i.data
	copy(data, buf)

	return len(buf), nil
}

func (i *RAWInput) listen(address string, buffer_size int) {
	address = strings.Replace(address, "[::]", "127.0.0.1", -1)

	host, port, err := net.SplitHostPort(address)

	if err != nil {
		log.Fatal("input-raw: error while parsing address", err)
	}

	listener := raw.NewListener(host, port, buffer_size)

	for {
		// Receiving TCPMessage object
		m := listener.Receive()

		i.data <- m.Bytes()
	}
}

func (i *RAWInput) String() string {
	return "RAW Socket input: " + i.address
}
