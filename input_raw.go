package goreplay

import (
	"context"
	"fmt"
	"github.com/buger/goreplay/internal/capture"
	"github.com/buger/goreplay/internal/tcp"
	"github.com/buger/goreplay/proto"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
)

// RAWInputConfig represents configuration that can be applied on raw input
type RAWInputConfig = capture.PcapOptions

// RAWInput used for intercepting traffic for given address
type RAWInput struct {
	sync.Mutex
	config         RAWInputConfig
	messageStats   []tcp.Stats
	listener       *capture.Listener
	messageParser  *tcp.MessageParser
	cancelListener context.CancelFunc
	closed         bool

	quit  chan bool // Channel used only to indicate goroutine should shutdown
	host  string
	ports []uint16
}

// NewRAWInput constructor for RAWInput. Accepts raw input config as arguments.
func NewRAWInput(address string, config RAWInputConfig) (i *RAWInput) {
	i = new(RAWInput)
	i.config = config
	i.quit = make(chan bool)

	host, _ports, err := net.SplitHostPort(address)
	if err != nil {
		// If we are reading pcap file, no port needed
		if strings.HasSuffix(address, "pcap") {
			host = address
			_ports = "0"
			err = nil
		} else if strings.HasPrefix(address, "k8s://") {
			portIndex := strings.LastIndex(address, ":")
			host = address[:portIndex]
			_ports = address[portIndex+1:]
		} else {
			log.Fatalf("input-raw: error while parsing address: %s", err)
		}
	}

	if strings.HasSuffix(host, "pcap") {
		i.config.Engine = capture.EnginePcapFile
	}

	var ports []uint16
	if _ports != "" {
		portsStr := strings.Split(_ports, ",")

		for _, portStr := range portsStr {
			port, err := strconv.Atoi(strings.TrimSpace(portStr))
			if err != nil {
				log.Fatalf("parsing port error: %v", err)
			}
			ports = append(ports, uint16(port))

		}
	}

	i.host = host
	i.ports = ports

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
	case msgTCP = <-i.listener.Messages():
		msg.Data = msgTCP.Data()
	}

	var msgType byte = ResponsePayload
	if msgTCP.Direction == tcp.DirIncoming {
		msgType = RequestPayload
		if i.config.RealIPHeader != "" {
			msg.Data = proto.SetHeader(msg.Data, []byte(i.config.RealIPHeader), []byte(msgTCP.SrcAddr))
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
	if i.config.Stats {
		stat := msgTCP.Stats
		go i.addStats(stat)
	}
	msgTCP = nil
	return &msg, nil
}

func (i *RAWInput) listen(address string) {
	var err error
	i.listener, err = capture.NewListener(i.host, i.ports, i.config)
	if err != nil {
		log.Fatal(err)
	}

	err = i.listener.Activate()
	if err != nil {
		log.Fatal(err)
	}

	var ctx context.Context
	ctx, i.cancelListener = context.WithCancel(context.Background())
	errCh := i.listener.ListenBackground(ctx)
	<-i.listener.Reading
	Debug(1, i)
	go func() {
		<-errCh // the listener closed voluntarily
		i.Close()
	}()
}

func (i *RAWInput) String() string {
	return fmt.Sprintf("Intercepting traffic from: %s:%s", i.host, strings.Join(strings.Fields(fmt.Sprint(i.ports)), ","))
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
