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
	closed         bool
}

// NewRAWInput constructor for RAWInput. Accepts raw input config as arguments.
func NewRAWInput(address string, config RAWInputConfig) (i *RAWInput) {
	i = new(RAWInput)
	i.RAWInputConfig = config
	i.message = make(chan *tcp.Message, 10000)
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

// PluginRead reads meassage from this plugin
func (i *RAWInput) PluginRead() (*Message, error) {
	var msgTCP *tcp.Message
	var msg Message
	select {
	case <-i.quit:
		return nil, ErrorStopped
	case msgTCP = <-i.message:
		msg.Data = msgTCP.Data()
	}
	var msgType byte = ResponsePayload
	if msgTCP.IsIncoming {
		msgType = RequestPayload
		if i.RealIPHeader != "" {
			msg.Data = proto.SetHeader(msg.Data, []byte(i.RealIPHeader), []byte(msgTCP.SrcAddr))
		}
	}
	msg.Meta = payloadHeader(msgType, msgTCP.UUID(), msgTCP.Start.UnixNano(), msgTCP.End.UnixNano()-msgTCP.Start.UnixNano())

	// to be removed....
	if msgTCP.Truncated {
		Debug(2, "[INPUT-RAW] message truncated, increase copy-buffer-size")
	}
	// to be removed...
	if msgTCP.TimedOut {
		Debug(2, "[INPUT-RAW] message timeout reached, increase input-raw-expire")
	}
	if i.Stats {
		stat := msgTCP.Stats
		go i.addStats(stat)
	}
	msgTCP = nil
	return &msg, nil
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
	pool.MatchUUID(i.TrackResponse)
	if i.Protocol == ProtocolHTTP {
		pool.Start = http1StartHint
		pool.End = http1EndHint
	}
	var ctx context.Context
	ctx, i.cancelListener = context.WithCancel(context.Background())
	errCh := i.listener.ListenBackground(ctx, pool.Handler)
	<-i.listener.Reading
	Debug(1, i)
	go func() {
		<-errCh // the listener closed voluntarily
		i.Close()
	}()
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
	i.Lock()
	defer i.Unlock()
	if i.closed {
		return nil
	}
	i.cancelListener()
	close(i.quit)
	i.closed = true
	return nil
}

func (i *RAWInput) addStats(mStats tcp.Stats) {
	i.Lock()
	if len(i.messageStats) >= 10000 {
		i.messageStats = []tcp.Stats{}
	}
	i.messageStats = append(i.messageStats, mStats)
	i.Unlock()
}

func http1StartHint(pckt *tcp.Packet) (isIncoming, isOutgoing bool) {
	isIncoming = proto.HasRequestTitle(pckt.Payload)
	if isIncoming {
		return
	}
	return false, proto.HasResponseTitle(pckt.Payload)
}

func http1EndHint(m *tcp.Message) bool {
	return proto.HasFullPayload(m.Data(), m)
}
