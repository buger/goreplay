package listener

import (
	"log"
	"net"
	"time"
)

type RAWTCPListener struct {
	messages map[uint32]*TCPMessage // buffer of TCPMessages waiting to be send

	c_packets  chan *TCPPacket
	c_messages chan *TCPMessage

	addr string
	port int
}

func RAWTCPListen(addr string, port int) (listener *RAWTCPListener) {
	listener = &RAWTCPListener{}
	listener.messages = make(map[uint32]*TCPMessage)

	listener.c_packets = make(chan *TCPPacket)
	listener.c_messages = make(chan *TCPMessage)

	listener.addr = addr
	listener.port = port

	go listener.listen()
	go listener.readTCPPackets()

	return
}

func (t *RAWTCPListener) listen() {

	for {
		var messages chan *TCPMessage
		var message *TCPMessage

		for _, msg := range t.messages {
			if msg.Complete() {
				messages = t.c_messages
				message = msg
			}
		}

		select {
		case messages <- message:
			delete(t.messages, message.ask)
		case packet := <-t.c_packets:
			t.processTCPPacket(packet)

		// Ensure that this will be run at least each 200 ms, to ensure that all messages will be send
		// Without it last message may not be send (it will be send only on next TCP packets)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (t *RAWTCPListener) readTCPPackets() {
	conn, e := net.ListenPacket("ip4:tcp", t.addr)
	defer conn.Close()

	if e != nil {
		log.Fatal(e)
	}

	for {
		buf := make([]byte, 65565)
		n, _, err := conn.ReadFrom(buf)

		if err != nil {
			Debug("Error:", err)
		}

		if n > 0 {
			packet := NewTCPPacket(buf[:n])

			if int(packet.dest_port) == t.port {
				t.c_packets <- packet
			}
		}
	}
}

//
func (t *RAWTCPListener) processTCPPacket(packet *TCPPacket) {
	// We interested only in packets that contain some data
	if !(packet.f_ask && packet.f_psh) {
		return
	}

	ask := packet.asknowledgement

	if _, ok := t.messages[ask]; !ok {
		t.messages[ask] = NewTCPMessage(ask)
	}

	t.messages[ask].AddPacket(packet)
}

func (t *RAWTCPListener) Receive() *TCPMessage {
	return <-t.c_messages
}
