package goreplay

import (
	"context"
	"crypto/tls"
	"fmt"
	"hash/fnv"
	"net"
	"time"
)

// TCPOutput used for sending raw tcp payloads
// Currently used for internal communication between listener and replay server
// Can be used for transferring binary payloads like protocol buffers
type TCPOutput struct {
	address     string
	limit       int
	buf         []chan *Message
	bufStats    *GorStat
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

	GetInitMessage     func() *Message                         `json:"-"`
	WriteBeforeMessage func(conn net.Conn, msg *Message) error `json:"-"`
}

// NewTCPOutput constructor for TCPOutput
// Initialize X workers which hold keep-alive connection
func NewTCPOutput(address string, config *TCPOutputConfig) PluginWriter {
	o := new(TCPOutput)

	o.address = address
	o.config = config

	if Settings.OutputTCPStats {
		o.bufStats = NewGorStat("output_tcp", 5000)
	}

	// create X buffers and send the buffer index to the worker
	o.buf = make([]chan *Message, o.config.Workers)
	for i := 0; i < o.config.Workers; i++ {
		o.buf[i] = make(chan *Message, 100)
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

		Debug(1, fmt.Sprintf("Can't connect to aggregator instance, reconnecting in 1 second. Retries:%d", retries))
		time.Sleep(1 * time.Second)

		conn, err = o.connect(o.address)
		retries++
	}

	if retries > 0 {
		Debug(2, fmt.Sprintf("Connected to aggregator instance after %d retries", retries))
	}

	defer conn.Close()

	if o.config.GetInitMessage != nil {
		msg := o.config.GetInitMessage()
		_ = o.writeToConnection(conn, msg)
	}

	for {
		msg := <-o.buf[bufferIndex]
		err = o.writeToConnection(conn, msg)
		if err != nil {
			Debug(2, "INFO: TCP output connection closed, reconnecting")
			go o.worker(bufferIndex)
			o.buf[bufferIndex] <- msg
			break
		}
	}
}

func (o *TCPOutput) writeToConnection(conn net.Conn, msg *Message) (err error) {
	if o.config.WriteBeforeMessage != nil {
		err = o.config.WriteBeforeMessage(conn, msg)
	}

	if err == nil {
		if _, err = conn.Write(msg.Meta); err == nil {
			if _, err = conn.Write(msg.Data); err == nil {
				_, err = conn.Write(payloadSeparatorAsBytes)
			}
		}
	}

	return err
}

func (o *TCPOutput) getBufferIndex(msg *Message) int {
	if !o.config.Sticky {
		o.workerIndex++
		return int(o.workerIndex) % o.config.Workers
	}

	hasher := fnv.New32a()
	hasher.Write(payloadID(msg.Meta))
	return int(hasher.Sum32()) % o.config.Workers
}

// PluginWrite writes message to this plugin
func (o *TCPOutput) PluginWrite(msg *Message) (n int, err error) {
	if !isOriginPayload(msg.Meta) {
		return len(msg.Data), nil
	}

	bufferIndex := o.getBufferIndex(msg)
	o.buf[bufferIndex] <- msg

	if Settings.OutputTCPStats {
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
