package listener

import (
	"encoding/binary"
	"log"
	"net"
)

// Capture traffic from socket using RAW_SOCKET's
// http://en.wikipedia.org/wiki/Raw_socket
//
// RAW_SOCKET allow you listen for traffic on any port (e.g. sniffing) because they operate on IP level.
// Ports is TCP feature, same as flow control, reliable transmission and etc.
// Since we can't use default TCP libraries RAWTCPLitener implements own TCP layer
// TCP packets is parsed using tcp_packet.go, and flow control is managed by tcp_message.go
type RAWTCPListener struct {
	messages map[uint32]*TCPMessage // buffer of TCPMessages waiting to be send

	c_packets  chan *TCPPacket
	c_messages chan *TCPMessage // Messages ready to be send to client

	c_del_message chan *TCPMessage // Used for notifications about completed or expired messages

	addr string // IP to listen
	port int    // Port to listen
}

// RAWTCPListen creates a listener to capture traffic from RAW_SOCKET
func RAWTCPListen(addr string, port int) (rawListener *RAWTCPListener) {
	rawListener = &RAWTCPListener{}

	rawListener.c_packets = make(chan *TCPPacket, 100)
	rawListener.c_messages = make(chan *TCPMessage, 100)
	rawListener.c_del_message = make(chan *TCPMessage, 100)
	rawListener.messages = make(map[uint32]*TCPMessage)

	rawListener.addr = addr
	rawListener.port = port

	go rawListener.listen()
	go rawListener.readRAWSocket()

	return
}

func (t *RAWTCPListener) listen() {
	for {
		select {
		// If message ready for deletion it means that its also complete or expired by timeout
		case message := <-t.c_del_message:
			t.c_messages <- message
			delete(t.messages, message.Ack)

		// We need to use channels to process each packet to avoid data races
		case packet := <-t.c_packets:
			t.processTCPPacket(packet)
		}
	}
}

func (t *RAWTCPListener) readRAWSocket() {
	conn, e := net.ListenPacket("ip4:tcp", t.addr)
	defer conn.Close()

	if e != nil {
		log.Fatal(e)
	}

	buf := make([]byte, 4096*2)

	for {
		// Note: ReadFrom receive messages without IP header
		n, _, err := conn.ReadFrom(buf)

		if err != nil {
			Debug("Error:", err)
			continue
		}

		if n > 0 {
			t.parsePacket(buf[:n])
		}
	}
}

func (t *RAWTCPListener) parsePacket(buf []byte) {
	if t.isIncomingDataPacket(buf) {
		new_buf := make([]byte, len(buf))
		copy(new_buf, buf)

		t.c_packets <- ParseTCPPacket(new_buf)
	}
}

func (t *RAWTCPListener) isIncomingDataPacket(buf []byte) bool {
	// To avoid full packet parsing every time, we manually parsing values needed for packet filtering
	// http://en.wikipedia.org/wiki/Transmission_Control_Protocol
	dest_port := binary.BigEndian.Uint16(buf[2:4])

	// Because RAW_SOCKET can't be bound to port, we have to control it by ourself
	if int(dest_port) == t.port {
		// Check TCPPacket code for more description
		flags := binary.BigEndian.Uint16(buf[12:14]) & 0x1FF

		// We need only packets with data inside
		// TCP PSH flag indicate that packet have data inside
		if (flags & TCP_PSH) != 0 {
			// We should create new buffer because go slices is pointers. So buffer data shoud be immutable.
			return true
		}
	}

	return false
}

// Trying to add packet to existing message or creating new message
//
// For TCP message unique id is Acknowledgment number (see tcp_packet.go)
func (t *RAWTCPListener) processTCPPacket(packet *TCPPacket) {
	var message *TCPMessage

	message, ok := t.messages[packet.Ack]

	if !ok {
		// We sending c_del_message channel, so message object can communicate with Listener and notify it if message completed
		message = NewTCPMessage(packet.Ack, t.c_del_message)
		t.messages[packet.Ack] = message
	}

	// Adding packet to message
	message.c_packets <- packet
}

// Receive TCP messages from the listener channel
func (t *RAWTCPListener) Receive() *TCPMessage {
	return <-t.c_messages
}
