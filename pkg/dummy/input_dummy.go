package dummy

import (
	"time"

	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/proto"
)

// DummyInput used for debugging. It generate 1 "GET /"" request per second.
type DummyInput struct {
	data chan []byte
	quit chan struct{}
}

// NewDummyInput constructor for DummyInput
func NewDummyInput(options string) (di *DummyInput) {
	di = new(DummyInput)
	di.data = make(chan []byte)
	di.quit = make(chan struct{})

	go di.emit()

	return
}

// PluginRead reads message from this plugin
func (i *DummyInput) PluginRead() (*plugin.Message, error) {
	var msg plugin.Message
	select {
	case <-i.quit:
		return nil, plugin.ErrorStopped
	case buf := <-i.data:
		msg.Meta, msg.Data = proto.PayloadMetaWithBody(buf)
		return &msg, nil
	}
}

func (i *DummyInput) emit() {
	ticker := time.NewTicker(time.Second)

	for range ticker.C {
		uuid := proto.UUID()
		reqh := proto.PayloadHeader(proto.RequestPayload, uuid, time.Now().UnixNano(), -1)
		i.data <- append(reqh, []byte("GET / HTTP/1.1\r\nHost: www.w3.org\r\nUser-Agent: Go 1.1 package http\r\nAccept-Encoding: gzip\r\n\r\n")...)

		resh := proto.PayloadHeader(proto.ResponsePayload, uuid, time.Now().UnixNano()+1, 1)
		i.data <- append(resh, []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")...)
	}
}

func (i *DummyInput) String() string {
	return "Dummy Input"
}

// Close closes this plugins
func (i *DummyInput) Close() error {
	close(i.quit)
	return nil
}
