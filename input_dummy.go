package gor

import (
	"time"
)

type DummyInput struct {
	data chan []byte
}

func NewDummyInput(options string) (di *DummyInput) {
	di = new(DummyInput)
	di.data = make(chan []byte)

	go di.emit()

	return
}

func (i *DummyInput) Read() ([]byte, bool) {
	buf, ok := <-i.data

	return buf, ok
}

func (i *DummyInput) emit() {
	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-ticker.C:
			i.data <- []byte("GET / HTTP/1.1\r\n\r\n")
		}
	}
}

func (i *DummyInput) String() string {
	return "Dummy Input"
}
