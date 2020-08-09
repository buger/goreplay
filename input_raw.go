package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/buger/goreplay/capture"
	"github.com/buger/goreplay/proto"
	"github.com/buger/goreplay/size"
	"github.com/buger/goreplay/tcp"
)

// TCPProtocol is a number to indicate type of protocol
type TCPProtocol uint8

const (
	// ProtocolHTTP ...
	ProtocolHTTP TCPProtocol = iota
	// ProtocolBinary ...
	ProtocolBinary
)

// Set is here so that TCPProtocol can implement flag.Var
func (protocol *TCPProtocol) Set(v string) error {
	switch v {
	case "", "http":
		*protocol = ProtocolHTTP
	case "binary":
		*protocol = ProtocolBinary
	default:
		return fmt.Errorf("unsupported protocol %s", v)
	}
	return nil
}

func (protocol *TCPProtocol) String() string {
	switch *protocol {
	case ProtocolBinary:
		return "binary"
	case ProtocolHTTP:
		return "http"
	default:
		return ""
	}
}

// RAWInputConfig represents configuration that can be applied on raw input
type RAWInputConfig struct {
	capture.PcapOptions
	Expire         time.Duration      `json:"input-raw-expire"`
	CopyBufferSize size.Size          `json:"copy-buffer-size"`
	Engine         capture.EngineType `json:"input-raw-engine"`
	TrackResponse  bool               `json:"input-raw-track-response"`
	Protocol       TCPProtocol        `json:"input-raw-protocol"`
	RealIPHeader   string             `json:"input-raw-realip-header"`
	Stats          bool               `json:"input-raw-stats"`
	quit           chan bool          // Channel used only to indicate goroutine should shutdown
	host           string
	port           uint16
}

// RAWInput used for intercepting traffic for given address
type RAWInput struct {
	sync.Mutex
	RAWInputConfig
	messageStats   []tcp.Stats
	listener       *capture.Listener
	message        chan *tcp.Message
	cancelListener context.CancelFunc
}

// NewRAWInput constructor for RAWInput. Accepts raw input config as arguments.
func NewRAWInput(address string, config RAWInputConfig) (i *RAWInput) {
	i = new(RAWInput)
	i.RAWInputConfig = config
	i.message = make(chan *tcp.Message, 1000)
	i.quit = make(chan bool)
	var host, _port string
	var err error
	var port int
	host, _port, err = net.SplitHostPort(address)
	if err != nil {
		log.Fatalf("input-raw: error while parsing address: %s", err)
	}
	if _port != "" {
		port, err = strconv.Atoi(_port)
	}

	if err != nil {
		log.Fatalf("parsing port error: %v", err)
	}
	i.host = host
	i.port = uint16(port)

	i.listen(address)

	return
}

func (i *RAWInput) Read(data []byte) (n int, err error) {
	var msg *tcp.Message
	var buf []byte
	select {
	case <-i.quit:
		return 0, ErrorStopped
	case msg = <-i.message:
		buf = msg.Data()
	}
	var header []byte

	var msgType byte = ResponsePayload
	if msg.IsIncoming {
		msgType = RequestPayload
		if i.RealIPHeader != "" {
			buf = proto.SetHeader(buf, []byte(i.RealIPHeader), []byte(msg.SrcAddr))
		}
	}
	header = payloadHeader(msgType, msg.UUID(), msg.Start.UnixNano(), msg.End.UnixNano()-msg.Start.UnixNano())

	n = copy(data, header)
	if len(data) > len(header) {
		n += copy(data[len(header):], buf)
	}
	dis := len(header) + len(buf) - n
	if dis > 0 {
		go Debug(2, "[INPUT-RAW] discarded", dis, "bytes increase copy buffer size")
	}
	if msg.Truncated {
		go Debug(2, "[INPUT-RAW] message truncated, copy-buffer-size")
	}
	go i.addStats(msg.Stats)
	return n, nil
}

func (i *RAWInput) listen(address string) {
	var err error
	i.listener, err = capture.NewListener(i.host, i.port, "", i.Engine, i.TrackResponse)
	if err != nil {
		log.Fatal(err)
	}
	i.listener.SetPcapOptions(i.PcapOptions)
	err = i.listener.Activate()
	if err != nil {
		log.Fatal(err)
	}
	pool := tcp.NewMessagePool(i.CopyBufferSize, i.Expire, Debug, i.handler)
	pool.End = endHint
	pool.Start = startHint
	var ctx context.Context
	ctx, i.cancelListener = context.WithCancel(context.Background())
	errCh := i.listener.ListenBackground(ctx, pool.Handler)
	select {
	case err := <-errCh:
		log.Fatal(err)
	case <-i.listener.Reading:
		Debug(1, i)
	}
}

func (i *RAWInput) handler(m *tcp.Message) {
	i.message <- m
}

func (i *RAWInput) String() string {
	return fmt.Sprintf("Intercepting traffic from: %s:%d", i.host, i.port)
}

// GetStats returns the stats so far and reset the stats
func (i *RAWInput) GetStats() []tcp.Stats {
	i.Lock()
	defer func() {
		i.messageStats = []tcp.Stats{}
		i.Unlock()
	}()
	return i.messageStats
}

// Close closes the input raw listener
func (i *RAWInput) Close() error {
	i.cancelListener()
	close(i.quit)
	return nil
}

func (i *RAWInput) addStats(mStats tcp.Stats) {
	if i.Stats {
		i.Lock()
		if len(i.messageStats) >= 10000 {
			i.messageStats = []tcp.Stats{}
		}
		i.messageStats = append(i.messageStats, mStats)

		i.Unlock()
	}
}

func startHint(pckt *tcp.Packet) (isIncoming, isOutgoing bool) {
	return proto.HasRequestTitle(pckt.Payload), proto.HasResponseTitle(pckt.Payload)
}

func endHint(m *tcp.Message) bool {
	return proto.HasFullPayload(m.Data())
}
