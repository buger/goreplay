package goreplay

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketOutput used for sending raw tcp payloads
// Can be used for transferring binary payloads like protocol buffers
type WebSocketOutput struct {
	address     string
	limit       int
	buf         []chan *Message
	bufStats    *GorStat
	config      *WebSocketOutputConfig
	workerIndex uint32
	headers     http.Header

	close bool
}

// WebSocketOutputConfig WebSocket output configuration
type WebSocketOutputConfig struct {
	Sticky     bool `json:"output-ws-sticky"`
	SkipVerify bool `json:"output-ws-skip-verify"`
	Workers    int  `json:"output-ws-workers"`

	Headers map[string][]string `json:"output-ws-headers"`
}

// NewWebSocketOutput constructor for WebSocketOutput
// Initialize X workers which hold keep-alive connection
func NewWebSocketOutput(address string, config *WebSocketOutputConfig) PluginWriter {
	o := new(WebSocketOutput)

	u, err := url.Parse(address)
	if err != nil {
		log.Fatal(fmt.Sprintf("[OUTPUT-WS] parse WS output URL error[%q]", err))
	}

	o.config = config
	o.headers = http.Header{
		"Authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte(u.User.String()))},
	}
	for k, values := range config.Headers {
		for _, v := range values {
			o.headers.Add(k, v)
		}
	}

	u.User = nil // must be after creating the headers
	o.address = u.String()

	if Settings.OutputWebSocketStats {
		o.bufStats = NewGorStat("output_ws", 5000)
	}

	// create X buffers and send the buffer index to the worker
	o.buf = make([]chan *Message, o.config.Workers)
	for i := 0; i < o.config.Workers; i++ {
		o.buf[i] = make(chan *Message, 100)
		go o.worker(i)
	}

	return o
}

func (o *WebSocketOutput) worker(bufferIndex int) {
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

	for {
		msg := <-o.buf[bufferIndex]
		err = conn.WriteMessage(websocket.BinaryMessage, append(msg.Meta, msg.Data...))
		if err != nil {
			Debug(2, "INFO: WebSocket output connection closed, reconnecting "+err.Error())
			go o.worker(bufferIndex)
			o.buf[bufferIndex] <- msg
			break
		}
	}
}

func (o *WebSocketOutput) getBufferIndex(msg *Message) int {
	if !o.config.Sticky {
		o.workerIndex++
		return int(o.workerIndex) % o.config.Workers
	}

	hasher := fnv.New32a()
	hasher.Write(payloadID(msg.Meta))
	return int(hasher.Sum32()) % o.config.Workers
}

// PluginWrite writes message to this plugin
func (o *WebSocketOutput) PluginWrite(msg *Message) (n int, err error) {
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

func (o *WebSocketOutput) connect(address string) (conn *websocket.Conn, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := websocket.DefaultDialer
	if strings.HasPrefix(address, "wss://") {
		d.TLSClientConfig = &tls.Config{InsecureSkipVerify: o.config.SkipVerify}
	}

	conn, _, err = d.DialContext(ctx, address, o.headers)
	return
}

func (o *WebSocketOutput) String() string {
	return fmt.Sprintf("WebSocket output %s, limit: %d", o.address, o.limit)
}

// Close closes the output
func (o *WebSocketOutput) Close() {
	o.close = true
}
