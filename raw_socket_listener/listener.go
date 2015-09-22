/*
Package rawSocket provides traffic sniffier using RAW sockets.

Capture traffic from socket using RAW_SOCKET's
http://en.wikipedia.org/wiki/Raw_socket

RAW_SOCKET allow you listen for traffic on any port (e.g. sniffing) because they operate on IP level.

Ports is TCP feature, same as flow control, reliable transmission and etc.

This package implements own TCP layer: TCP packets is parsed using tcp_packet.go, and flow control is managed by tcp_message.go
*/
package rawSocket

import (
	"bytes"
	"encoding/binary"
	"log"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// Listener handle traffic capture
type Listener struct {
	// buffer of TCPMessages waiting to be send
	messages map[string]*TCPMessage

	// Expect: 100-continue request is send in 2 tcp messages
	// We store ACK aliases to merge this packets together
	ackAliases map[uint32]uint32
	// To get ACK of second message we need to compute its Seq and wait for them message
	seqWithData map[uint32]uint32

	respAliases map[uint32]*request

	// Messages ready to be send to client
	packetsChan chan *TCPPacket

	// Messages ready to be send to client
	messagesChan chan *TCPMessage

	addr string // IP to listen
	port uint16 // Port to listen

	messageExpire time.Duration

	captureResponse bool

	conn net.PacketConn
	quit chan bool
}

type request struct {
	start time.Time
	ack   uint32
}

// NewListener creates and initializes new Listener object
func NewListener(addr string, port string, expire time.Duration, captureResponse bool) (l *Listener) {
	l = &Listener{captureResponse: captureResponse}

	l.packetsChan = make(chan *TCPPacket, 10000)
	l.messagesChan = make(chan *TCPMessage, 10000)
	l.quit = make(chan bool)

	l.messages = make(map[string]*TCPMessage)
	l.ackAliases = make(map[uint32]uint32)
	l.seqWithData = make(map[uint32]uint32)
	l.respAliases = make(map[uint32]*request)

	l.addr = addr
	_port, _ := strconv.Atoi(port)
	l.port = uint16(_port)

	if expire.Nanoseconds() == 0 {
		expire = 2000 * time.Millisecond
	}

	l.messageExpire = expire

	go l.listen()
	go l.readRAWSocket()

	return
}

func (t *Listener) listen() {
	gcTicker := time.Tick(t.messageExpire / 2)

	for {
		select {
		case <-t.quit:
			t.conn.Close()
			return
		// We need to use channels to process each packet to avoid data races
		case packet := <-t.packetsChan:
			t.processTCPPacket(packet)

		case <-gcTicker:
			now := time.Now()

			for _, message := range t.messages {
				if now.Sub(message.Start) >= t.messageExpire {
					t.dispatchMessage(message)
				}
			}
		}
	}
}

func (t *Listener) dispatchMessage(message *TCPMessage) {
	delete(t.ackAliases, message.Ack)
	delete(t.messages, message.ID)

	if !message.IsIncoming {
		delete(t.respAliases, message.Ack)

		// Do not track responses which have no associated requests
		if message.RequestAck == 0 {
			return
		}
	}
	t.messagesChan <- message
}

func (t *Listener) readRAWSocket() {
	conn, e := net.ListenPacket("ip4:tcp", t.addr)
	t.conn = conn

	if e != nil {
		log.Fatal(e)
	}

	defer t.conn.Close()

	buf := make([]byte, 64*1024) // 64kb

	for {
		// Note: ReadFrom receive messages without IP header
		n, addr, err := t.conn.ReadFrom(buf)

		if err != nil {
			if strings.HasSuffix(err.Error(), "closed network connection") {
				return
			} else {
				log.Println("Raw listener error:", err)
				continue
			}
		}

		if n > 0 {
			if t.isValidPacket(buf[:n]) {
				newBuf := make([]byte, n)
				copy(newBuf, buf[:n])

				go func(newBuf []byte) {
					t.packetsChan <- ParseTCPPacket(addr, newBuf)
				}(newBuf)
			}
		}
	}
}

func (t *Listener) isValidPacket(buf []byte) bool {
	// To avoid full packet parsing every time, we manually parsing values needed for packet filtering
	// http://en.wikipedia.org/wiki/Transmission_Control_Protocol
	destPort := binary.BigEndian.Uint16(buf[2:4])
	srcPort := binary.BigEndian.Uint16(buf[0:2])

	// Because RAW_SOCKET can't be bound to port, we have to control it by ourself
	if destPort == t.port || (t.captureResponse && srcPort == t.port) {
		// Get the 'data offset' (size of the TCP header in 32-bit words)
		dataOffset := (buf[12] & 0xF0) >> 4

		// We need only packets with data inside
		// Check that the buffer is larger than the size of the TCP header
		if len(buf) > int(dataOffset*4) {
			// We should create new buffer because go slices is pointers. So buffer data shoud be immutable.
			return true
		}
	}

	return false
}

var bExpect100ContinueCheck = []byte("Expect: 100-continue")
var bPOST = []byte("POST")

// Trying to add packet to existing message or creating new message
//
// For TCP message unique id is Acknowledgment number (see tcp_packet.go)
func (t *Listener) processTCPPacket(packet *TCPPacket) {
	// Don't exit on panic
	defer func() {
		if r := recover(); r != nil {
			log.Println("PANIC: pkg:", r, packet, string(debug.Stack()))
		}
	}()

	var message *TCPMessage

	isIncoming := packet.DestPort == t.port

	if parentAck, ok := t.seqWithData[packet.Seq]; ok {
		t.ackAliases[packet.Ack] = parentAck
		delete(t.seqWithData, packet.Seq)
	}

	if alias, ok := t.ackAliases[packet.Ack]; ok {
		packet.Ack = alias
	}

	var responseRequest *request

	if t.captureResponse && !isIncoming {
		responseRequest, _ = t.respAliases[packet.Ack]
	}

	mID := packet.Addr.String() + strconv.Itoa(int(packet.DestPort)) + strconv.Itoa(int(packet.Ack))

	message, ok := t.messages[mID]

	if !ok {
		message = NewTCPMessage(mID, packet.Ack, isIncoming)
		t.messages[mID] = message

		if !isIncoming && responseRequest != nil {
			message.RequestStart = responseRequest.start
			message.RequestAck = responseRequest.ack
		}
	}

	// Handling Expect: 100-continue requests
	if len(packet.Data) > 4 && bytes.Equal(packet.Data[0:4], bPOST) {
		// reading last 20 bytes (not counting CRLF): last header value (if no body presented)
		if bytes.Equal(packet.Data[len(packet.Data)-24:len(packet.Data)-4], bExpect100ContinueCheck) {
			t.seqWithData[packet.Seq+uint32(len(packet.Data))] = packet.Ack

			// Removing `Expect: 100-continue` header
			packet.Data = append(packet.Data[:len(packet.Data)-24], packet.Data[len(packet.Data)-2:]...)
		}
	}

	if t.captureResponse && isIncoming {
		// If message have multiple packets, delete previous alias
		if len(message.packets) > 0 {
			delete(t.respAliases, message.ResponseAck)
		}

		responseAck := packet.Seq + uint32(len(packet.Data))
		t.respAliases[responseAck] = &request{message.Start, message.Ack}
		message.ResponseAck = responseAck
	}

	// Adding packet to message
	message.AddPacket(packet)

	// If message contains only single packet immediately dispatch it
	if !message.IsMultipart() {
		t.dispatchMessage(message)
	}
}

// Receive TCP messages from the listener channel
func (t *Listener) Receive() *TCPMessage {
	return <-t.messagesChan
}

func (t *Listener) Close() {
	close(t.quit)
	t.conn.Close()
	return
}
