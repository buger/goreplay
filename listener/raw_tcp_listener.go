package listener

import (
	"github.com/akrennmair/gopcap"
	"log"
	"time"
)

type RAWTCPListener struct {
	messages map[uint32]*TCPMessage // buffer of TCPMessages waiting to be send

	c_packets  chan *pcap.Packet
	c_messages chan *TCPMessage

	addr string
	port int

	sniffer *pcap.Pcap
}

func RAWTCPListen(addr string, port int) (listener *RAWTCPListener) {
	listener = &RAWTCPListener{}
	listener.messages = make(map[uint32]*TCPMessage)

	listener.c_packets = make(chan *pcap.Packet)
	listener.c_messages = make(chan *TCPMessage)

	listener.addr = addr
	listener.port = port

	listener.startSniffer()

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
				break
			}
		}

		select {
		case messages <- message:
			delete(t.messages, message.ack)
		case packet := <-t.c_packets:
			t.processTCPPacket(packet)

		// Ensure that this will be run at least each 200 ms, to ensure that all messages will be send
		// Without it last message may not be send (it will be send only on next TCP packets)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (t *RAWTCPListener) startSniffer() {
	devices, err := pcap.Findalldevs()

	if err != nil {
		log.Fatal("Error while getting device list", err)
	}

	availableIPs := []string{}
	addrValid := false
	networkInterface := ""

	for _, device := range devices {
		if device.Name != "any" {

			for _, addr := range device.Addresses {
				availableIPs = append(availableIPs, addr.IP.String())
				if addr.IP.String() == Settings.IP {
					addrValid = true
					networkInterface = device.Name
				}
			}
		}
	}

	if addrValid == false {
		log.Println("Can't listen on given IP:", Settings.IP, ". Available addresses:", availableIPs)

		log.Fatal("`ip` attribute required: gor listen -p 80 -ip 127.0.0.1")
	}

	h, err := pcap.Openlive(networkInterface, int32(4026), true, 0)
	h.Setfilter("tcp dst port " + string(t.port))

	if err != nil {
		log.Fatal("Error while trying to listen", err)
	}

	t.sniffer = h
}

func (t *RAWTCPListener) readTCPPackets() {

	for {
		pkt := t.sniffer.Next()

		if pkt == nil {
			continue
		}

		pkt.Decode()

		switch pkt.Headers[1].(type) {
		case *pcap.Tcphdr:
			header := pkt.Headers[1].(*pcap.Tcphdr)

			port := int(header.DestPort)

			if port == t.port && (header.Flags&pcap.TCP_PSH) != 0 {
				t.c_packets <- pkt
			}
		}
	}
}

//
func (t *RAWTCPListener) processTCPPacket(packet *pcap.Packet) {
	ack := packet.Headers[1].(*pcap.Tcphdr).Ack

	if _, ok := t.messages[ack]; !ok {
		t.messages[ack] = NewTCPMessage(ack)
	}

	t.messages[ack].AddPacket(packet)
}

func (t *RAWTCPListener) Receive() *TCPMessage {
	return <-t.c_messages
}
