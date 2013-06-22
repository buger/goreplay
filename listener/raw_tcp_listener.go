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
	messages []*TCPMessage // buffer of TCPMessages waiting to be send

	c_packets  chan *TCPPacket
	c_messages chan *TCPMessage // Messages ready to be send to client

	c_del_message chan *TCPMessage // Used for notifications about completed or expired messages

	addr string // IP to listen
	port int    // Port to listen
}

func RAWTCPListen(addr string, port int) (listener *RAWTCPListener) {
	listener = &RAWTCPListener{}

	listener.c_packets = make(chan *TCPPacket)
	listener.c_messages = make(chan *TCPMessage)
	listener.c_del_message = make(chan *TCPMessage)

	listener.addr = addr
	listener.port = port

	go listener.listen()
	go listener.readRAWSocket()

	return
}

func (t *RAWTCPListener) listen() {
	for {
		select {
		// If message ready for deletion it means that its also complete or expired by timeout
		case message := <-t.c_del_message:
			t.c_messages <- message
			t.deleteMessage(message)

		// We need to use channgels to process each packet to avoid data races
		case packet := <-t.c_packets:
			t.processTCPPacket(packet)
		}
	}
}

// Deleting messages that came from t.c_del_message channel
func (t *RAWTCPListener) deleteMessage(message *TCPMessage) bool {
	var idx int = -1

	// Searching for given message in messages buffer
	for i, m := range t.messages {
		if m.Ack == message.Ack {
			idx = i
			break
		}
	}

	if idx == -1 {
		return false
	}

	// Delete element from array
	// Note: that this version for arrays that consist of pointers
	// https://code.google.com/p/go-wiki/wiki/SliceTricks
	copy(t.messages[idx:], t.messages[idx+1:])
	t.messages[len(t.messages)-1] = nil // Ensure that value will be garbage-collected.
	t.messages = t.messages[:len(t.messages)-1]

	return true
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
		}

		if n > 0 {
			// To avoid full packet parsing every time, we manually parsing values needed for packet filtering
			// http://en.wikipedia.org/wiki/Transmission_Control_Protocol
			dest_port := binary.BigEndian.Uint16(buf[2:4])

			// Because RAW_SOCKET can't be bound to port, we have to control it by ourself
			if int(dest_port) == t.port {
				// Check TCPPacket code for more description
				flags := binary.BigEndian.Uint16(buf[12:14]) & 0x1FF
				f_psh := (flags & TCP_PSH) != 0

				// We need only packets with data inside
				// TCP PSH flag indicate that packet have data inside
				if f_psh {
					// We should create new buffer because go slices is pointers. So buffer data shoud be immutable.
					new_buf := make([]byte, n)
					copy(new_buf, buf[:n])

					// To avoid socket locking processing packet in new goroutine
					go func(buf []byte) {
						packet := NewTCPPacket(new_buf)
						t.c_packets <- packet
					}(new_buf)
				}
			}
		}
	}
}

// Trying to add packet to existing message or creating new message
//
// For TCP message unique id is Acknowledgment number (see tcp_packet.go)
func (t *RAWTCPListener) processTCPPacket(packet *TCPPacket) {
	var message *TCPMessage

	// Searching for message with same Ack
	for _, msg := range t.messages {
		if msg.Ack == packet.Ack {
			message = msg
			break
		}
	}

	if message == nil {
		// We sending c_del_message channel, so message object can communicate with Listener and notify it if message completed
		message = NewTCPMessage(packet.Ack, t.c_del_message)

		t.messages = append(t.messages, message)
	}

	// Adding packet to message
	message.c_packets <- packet
}

func (t *RAWTCPListener) Receive() *TCPMessage {
	return <-t.c_messages
}
