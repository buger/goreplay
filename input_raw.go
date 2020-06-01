package main

import (
	"log"
	"net"
	"time"

	"github.com/buger/goreplay/proto"
	raw "github.com/buger/goreplay/raw_socket_listener"
)

// RAWInput used for intercepting traffic for given address
type RAWInput struct {
	data          chan *raw.TCPMessage
	address       string
	expire        time.Duration
	quit          chan bool
	engine        int
	realIPHeader  []byte
	trackResponse bool
	listener      *raw.Listener
	bpfFilter     string
	timestampType string
	bufferSize    int64
}

// Available engines for intercepting traffic
const (
	EngineRawSocket = 1 << iota
	EnginePcap
	EnginePcapFile
)

// NewRAWInput constructor for RAWInput. Accepts address with port as argument.
func NewRAWInput(address string, engine int, trackResponse bool, expire time.Duration, realIPHeader string, bpfFilter string, timestampType string, bufferSize int64) (i *RAWInput) {
	i = new(RAWInput)
	i.data = make(chan *raw.TCPMessage)
	i.address = address
	i.expire = expire
	i.engine = engine
	i.bpfFilter = bpfFilter
	i.realIPHeader = []byte(realIPHeader)
	i.quit = make(chan bool)
	i.trackResponse = trackResponse
	i.timestampType = timestampType
	i.bufferSize = bufferSize

	i.listen(address)
	i.listener.IsReady()

	return
}

func (i *RAWInput) Read(data []byte) (int, error) {
	msg := <-i.data
	buf := msg.Bytes()

	var header []byte

	if msg.IsIncoming {
		header = payloadHeader(RequestPayload, msg.UUID(), msg.Start.UnixNano(), -1)
		if len(i.realIPHeader) > 0 {
			buf = proto.SetHeader(buf, i.realIPHeader, []byte(msg.IP().String()))
		}
	} else {
		header = payloadHeader(ResponsePayload, msg.UUID(), msg.Start.UnixNano(), msg.End.UnixNano()-msg.AssocMessage.End.UnixNano())
	}

	copy(data[0:len(header)], header)
	copy(data[len(header):], buf)

	return len(buf) + len(header), nil
}

func (i *RAWInput) listen(address string) {
	Debug("Listening for traffic on: " + address)

	host, port, err := net.SplitHostPort(address)

	if err != nil {
		log.Fatal("input-raw: error while parsing address", err)
	}

	i.listener = raw.NewListener(host, port, i.engine, i.trackResponse, i.expire, i.bpfFilter, i.timestampType, i.bufferSize, Settings.inputRAWOverrideSnapLen, Settings.inputRAWImmediateMode)

	ch := i.listener.Receiver()

	go func() {
		for {
			select {
			case <-i.quit:
				return
			default:
			}

			// Receiving TCPMessage object
			m := <-ch

			i.data <- m
		}
	}()
}

func (i *RAWInput) String() string {
	return "Intercepting traffic from: " + i.address
}

// Close closes the input raw listener
func (i *RAWInput) Close() error {
	i.listener.Close()
	close(i.quit)
	return nil
}
