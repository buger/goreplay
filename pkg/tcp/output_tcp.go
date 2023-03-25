package tcp

import (
	"context"
	"crypto/tls"
	"fmt"
	"hash/fnv"
	"net"
	"time"

	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/proto"
	"github.com/buger/goreplay/pkg/stats"

	"github.com/rs/zerolog/log"
)

// TCPOutput used for sending raw tcp payloads
// Currently used for internal communication between listener and replay server
// Can be used for transferring binary payloads like protocol buffers
type TCPOutput struct {
	address string
	limit   int

	buf         []chan *plugin.Message
	bufStats    *stats.Stats
	config      *TCPOutputConfig
	workerIndex uint32

	close bool
}

// TCPOutputConfig tcp output configuration
type TCPOutputConfig struct {
	Secure     bool `json:"output-tcp-secure"`
	Sticky     bool `json:"output-tcp-sticky"`
	SkipVerify bool `json:"output-tcp-skip-verify"`
	Workers    int  `json:"output-tcp-workers"`
	Stats      bool `json:"output-tcp-stats"`
}

// NewTCPOutput constructor for TCPOutput
// Initialize X workers which hold keep-alive connection
func NewTCPOutput(address string, config *TCPOutputConfig) plugin.Writer {
	o := new(TCPOutput)

	o.address = address
	o.config = config

	if config.Stats {
		o.bufStats = stats.New("output_tcp", 5000)
	}

	// create X buffers and send the buffer index to the worker
	o.buf = make([]chan *plugin.Message, o.config.Workers)
	for i := 0; i < o.config.Workers; i++ {
		o.buf[i] = make(chan *plugin.Message, 100)
		go o.worker(i)
	}

	return o
}

func (o *TCPOutput) worker(bufferIndex int) {
	retries := 0
	conn, err := o.connect(o.address)
	for {
		if o.close {
			return
		}

		if err == nil {
			break
		}

		log.Error().Msgf("Can't connect to aggregator instance, reconnecting in 1 second. Retries:%d", retries)
		time.Sleep(1 * time.Second)

		conn, err = o.connect(o.address)
		retries++
	}

	if retries > 0 {
		log.Info().Msgf("Connected to aggregator instance after %d retries", retries)
	}

	defer conn.Close()

	for {
		msg := <-o.buf[bufferIndex]
		if _, err = conn.Write(msg.Meta); err == nil {
			if _, err = conn.Write(msg.Data); err == nil {
				_, err = conn.Write(proto.PayloadSeparatorAsBytes)
			}
		}

		if err != nil {
			log.Info().Err(err).Msg("INFO: TCP output connection closed, reconnecting")
			o.buf[bufferIndex] <- msg
			go o.worker(bufferIndex)
			break
		}
	}
}

func (o *TCPOutput) getBufferIndex(msg *plugin.Message) int {
	if !o.config.Sticky {
		o.workerIndex++
		return int(o.workerIndex) % o.config.Workers
	}

	hasher := fnv.New32a()
	hasher.Write(proto.PayloadID(msg.Meta))
	return int(hasher.Sum32()) % o.config.Workers
}

// PluginWrite writes message to this plugin
func (o *TCPOutput) PluginWrite(msg *plugin.Message) (n int, err error) {
	if !proto.IsOriginPayload(msg.Meta) {
		return len(msg.Data), nil
	}

	bufferIndex := o.getBufferIndex(msg)
	o.buf[bufferIndex] <- msg

	if o.config.Stats {
		o.bufStats.Write(len(o.buf[bufferIndex]))
	}

	return len(msg.Data) + len(msg.Meta), nil
}

func (o *TCPOutput) connect(address string) (conn net.Conn, err error) {
	if o.config.Secure {
		var d tls.Dialer
		d.Config = &tls.Config{InsecureSkipVerify: o.config.SkipVerify}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err = d.DialContext(ctx, "tcp", address)
	} else {
		var d net.Dialer
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err = d.DialContext(ctx, "tcp", address)
	}

	return
}

func (o *TCPOutput) String() string {
	return fmt.Sprintf("TCP output %s, limit: %d", o.address, o.limit)
}

func (o *TCPOutput) Close() {
	o.close = true
}
