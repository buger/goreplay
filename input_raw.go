package gor

import (
	raw "github.com/buger/gor/raw_socket_listener"
	"log"
	"net"
	"strings"
)

type RAWInput struct {
	data    chan []byte
	address string
}

func NewRAWInput(address string) (i *RAWInput) {
	i = new(RAWInput)
	i.data = make(chan []byte)
	i.address = address

	go i.listen(address)

	return
}

func (i *RAWInput) Read() ([]byte, bool) {
	buf, ok := <-i.data

	return buf, ok
}

func (i *RAWInput) listen(address string) {
	address = strings.Replace(address, "[::]", "127.0.0.1", -1)

	host, port, err := net.SplitHostPort(address)

	if err != nil {
		log.Fatal("input-raw: error while parsing address", err)
	}

	listener := raw.NewListener(host, port)

	for {
		// Receiving TCPMessage object
		m := listener.Receive()

		i.data <- m.Bytes()
	}
}

func (i *RAWInput) String() string {
	return "RAW Socket input: " + i.address
}
