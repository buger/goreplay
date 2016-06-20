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
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"io"
	"log"
	"net"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"github.com/buger/gor/proto"
)

var _ = fmt.Println

// Listener handle traffic capture
type Listener struct {
	mu sync.Mutex
	// buffer of TCPMessages waiting to be send
	// ID -> TCPMessage
	messages map[tcpID]*TCPMessage

	// Expect: 100-continue request is send in 2 tcp messages
	// We store ACK aliases to merge this packets together
	ackAliases map[uint32]uint32
	// To get ACK of second message we need to compute its Seq and wait for them message
	seqWithData map[uint32]uint32

	// Ack -> Req
	respAliases map[uint32]*TCPMessage

	// Ack -> ID
	respWithoutReq map[uint32]tcpID

	// Messages ready to be send to client
	packetsChan chan []byte

	// Messages ready to be send to client
	messagesChan chan *TCPMessage

	addr string // IP to listen
	port uint16 // Port to listen

	trackResponse bool
	messageExpire time.Duration

	conn        net.PacketConn
	pcapHandles []*pcap.Handle

	quit    chan bool
	readyCh chan bool
}

type request struct {
	id    tcpID
	start time.Time
	ack   uint32
}

// Available engines for intercepting traffic
const (
	EngineRawSocket = 1 << iota
	EnginePcap
)

// NewListener creates and initializes new Listener object
func NewListener(addr string, port string, engine int, trackResponse bool, expire time.Duration) (l *Listener) {
	l = &Listener{}

	l.packetsChan = make(chan []byte, 10000)
	l.messagesChan = make(chan *TCPMessage, 10000)
	l.quit = make(chan bool)
	l.readyCh = make(chan bool, 1)

	l.messages = make(map[tcpID]*TCPMessage)
	l.ackAliases = make(map[uint32]uint32)
	l.seqWithData = make(map[uint32]uint32)
	l.respAliases = make(map[uint32]*TCPMessage)
	l.respWithoutReq = make(map[uint32]tcpID)
	l.trackResponse = trackResponse

	l.addr = addr
	_port, _ := strconv.Atoi(port)
	l.port = uint16(_port)

	if expire.Nanoseconds() == 0 {
		expire = 2000 * time.Millisecond
	}

	l.messageExpire = expire

	go l.listen()

	// Special case for testing
	if l.port != 0 {
		switch engine {
		case EngineRawSocket:
			go l.readRAWSocket()
		case EnginePcap:
			go l.readPcap()
		default:
			log.Fatal("Unknown traffic interception engine:", engine)
		}
	}

	return
}

func (t *Listener) listen() {
	gcTicker := time.Tick(t.messageExpire / 2)

	for {
		select {
		case <-t.quit:
			if t.conn != nil {
				t.conn.Close()
			}
			return
		case data := <-t.packetsChan:
			packet := ParseTCPPacket(data[:16], data[16:])
			t.processTCPPacket(packet)
		case <-gcTicker:
			now := time.Now()

			// Dispatch requests before responses
			for _, message := range t.messages {
				if now.Sub(message.End) >= t.messageExpire {
					t.dispatchMessage(message)
				}
			}
		}
	}
}

func (t *Listener) deleteMessage(message *TCPMessage) {
	delete(t.messages, message.ID())
	delete(t.ackAliases, message.Ack)
	if message.DataAck != 0 {
		delete(t.ackAliases, message.DataAck)
	}
	if message.DataSeq != 0 {
		delete(t.seqWithData, message.DataSeq)
	}

	delete(t.respAliases, message.ResponseAck)
}

func (t *Listener) dispatchMessage(message *TCPMessage) {
	// If already dispatched
	if _, ok := t.messages[message.ID()]; !ok {
		return
	}

	t.deleteMessage(message)

	// log.Println("Dispatching, message", message.Start.UnixNano(), message.Seq, message.Ack, string(message.Bytes()))

	if message.IsIncoming {
		// If there were response before request
		// log.Println("Looking for Response: ", t.respWithoutReq, message.ResponseAck)
		if t.trackResponse {
			if respID, ok := t.respWithoutReq[message.ResponseAck]; ok {
				if resp, rok := t.messages[respID]; rok {
					// if resp.AssocMessage == nil {
					// log.Println("FOUND RESPONSE")
					resp.AssocMessage = message
					message.AssocMessage = resp

					if resp.IsFinished() {
						defer t.dispatchMessage(resp)
					}
					// }
				}
			}

			if resp, ok := t.messages[message.ResponseID]; ok {
				resp.AssocMessage = message
			}
		}
	} else {
		if message.AssocMessage == nil {
			if responseRequest, ok := t.respAliases[message.Ack]; ok {
				message.AssocMessage = responseRequest
				responseRequest.AssocMessage = message
			}
		}

		delete(t.respAliases, message.Ack)
		delete(t.respWithoutReq, message.Ack)

		// Do not track responses which have no associated requests
		if message.AssocMessage == nil {
			// log.Println("Can't dispatch resp", message.Seq, message.Ack, string(message.Bytes()))
			return
		}
	}

	t.messagesChan <- message
}

// DeviceNotFoundError raised if user specified wrong ip
type DeviceNotFoundError struct {
	addr string
}

func (e *DeviceNotFoundError) Error() string {
	devices, _ := pcap.FindAllDevs()

	if len(devices) == 0 {
		return "Can't get list of network interfaces, ensure that you running Gor as root user or sudo.\nTo run as non-root users see this docs https://github.com/buger/gor/wiki/Running-as-non-root-user"
	}

	var msg string
	msg += "Can't find interfaces with addr: " + e.addr + ". Provide available IP for intercepting traffic: \n"
	for _, device := range devices {
		msg += "Name: " + device.Name + "\n"
		if device.Description != "" {
			msg += "Description: " + device.Description + "\n"
		}
		for _, address := range device.Addresses {
			msg += "- IP address: " + address.IP.String() + "\n"
		}
	}

	return msg
}

func findPcapDevices(addr string) (interfaces []pcap.Interface, err error) {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		log.Fatal(err)
	}

	for _, device := range devices {
		if (addr == "" || addr == "0.0.0.0" || addr == "[::]" || addr == "::") && len(device.Addresses) > 0 {
			interfaces = append(interfaces, device)
			continue
		}

		for _, address := range device.Addresses {
			if device.Name == addr || address.IP.String() == addr {
				interfaces = append(interfaces, device)
				return interfaces, nil
			}
		}
	}

	if len(interfaces) == 0 {
		return nil, &DeviceNotFoundError{addr}
	} else {
		return interfaces, nil
	}
}

func (t *Listener) readPcap() {
	devices, err := findPcapDevices(t.addr)
	if err != nil {
		log.Fatal(err)
	}

	bpfSupported := true
	if runtime.GOOS == "darwin" {
		bpfSupported = false
	}

	var wg sync.WaitGroup
	wg.Add(len(devices))

	for _, d := range devices {
		go func(device pcap.Interface) {
			handle, err := pcap.OpenLive(device.Name, 65536, true, t.messageExpire)
			if err != nil {
				log.Println("Pcap Error while opening device", device.Name, err)
				wg.Done()
				return
			}
			defer handle.Close()

			t.mu.Lock()
			t.pcapHandles = append(t.pcapHandles, handle)

			var bpfDstHost, bpfSrcHost string
			for i, addr := range device.Addresses {
				bpfDstHost += "dst host " + addr.IP.String()
				bpfSrcHost += "src host " + addr.IP.String()
				if i != len(device.Addresses) - 1 {
					bpfDstHost += " or "
					bpfSrcHost += " or "
				}
			}

			if bpfSupported {
				var bpf string

				if t.trackResponse {
					bpf = "(tcp dst port " + strconv.Itoa(int(t.port)) + " and (" + bpfDstHost + ")) or (" + "tcp src port " + strconv.Itoa(int(t.port)) + " and (" + bpfSrcHost + "))"
				} else {
					bpf = "tcp dst port " + strconv.Itoa(int(t.port)) + " and (" + bpfDstHost + ")"
				}

				if err := handle.SetBPFFilter(bpf); err != nil {
					log.Println("BPF filter error:", err, "Device:", device.Name, bpf)
					wg.Done()
					return
				}
			}
			t.mu.Unlock()

			var decoder gopacket.Decoder

			// Special case for tunnel interface https://github.com/google/gopacket/issues/99
			if handle.LinkType() == 12 {
			    decoder = layers.LayerTypeIPv4
			} else {
			    decoder = handle.LinkType()
			}

			source := gopacket.NewPacketSource(handle, decoder)
			source.Lazy = true
			source.NoCopy = true

			wg.Done()

			var data, srcIP, dstIP []byte

			for {
				packet, err := source.NextPacket()

				if err == io.EOF {
					break
				} else if err != nil {
					continue
				}

				// We should remove network layer before parsing TCP/IP data
				var of int
				switch decoder {
					case layers.LinkTypeEthernet:
						of = 14
					case layers.LinkTypePPP:
						of = 1
					case layers.LinkTypeFDDI:
						of = 13
					case layers.LinkTypeNull:
						of = 4
					case layers.LinkTypeLoop:
						of = 4
					case layers.LinkTypeRaw:
						of = 0
					case layers.LinkTypeLinuxSLL:
						of = 16
					default:
						log.Println("Unknown packet layer", packet)
						break
				}

				data = packet.Data()[of:]

				version := uint8(data[0]) >> 4

				if version == 4 {
					ihl := uint8(data[0]) & 0x0F

					// Truncated IP info
					if len(data) < int(ihl*4) {
						continue
					}

					srcIP = data[12:16]
					dstIP = data[16:20]
					data = data[ihl*4:]
				} else {
					// Truncated IP info
					if len(data) < 40 {
						continue
					}

					srcIP = data[8:24]
					dstIP = data[24:40]

					data = data[40:]
				}

				// Truncated TCP info
				if len(data) < 13 {
					continue
				}

				dataOffset := (data[12] & 0xF0) >> 4

				// We need only packets with data inside
				// Check that the buffer is larger than the size of the TCP header
				if len(data) > int(dataOffset*4) {
					if !bpfSupported {
						destPort := binary.BigEndian.Uint16(data[2:4])
						srcPort := binary.BigEndian.Uint16(data[0:2])

						var addrCheck []byte

						if destPort == t.port {
							addrCheck = dstIP
						}

						if t.trackResponse && srcPort == t.port {
							addrCheck = srcIP
						}

						if len(addrCheck) == 0 {
							continue
						}

						addrMatched := false
						for _, a := range device.Addresses {
							if a.IP.Equal(net.IP(addrCheck)) {
								addrMatched = true
								break
							}
						}

						if !addrMatched {
							continue
						}
					}

					newBuf := make([]byte, len(data)+16)
					copy(newBuf[:16], srcIP)
					copy(newBuf[16:], data)

					t.packetsChan <- newBuf
				}
			}
		}(d)
	}

	wg.Wait()
	t.readyCh <- true
}

func (t *Listener) readRAWSocket() {
	conn, e := net.ListenPacket("ip:tcp", t.addr)
	t.conn = conn

	if e != nil {
		log.Fatal(e)
	}

	defer t.conn.Close()

	buf := make([]byte, 64*1024) // 64kb

	t.readyCh <- true

	for {
		// Note: ReadFrom receive messages without IP header
		n, addr, err := t.conn.ReadFrom(buf)

		if err != nil {
			if strings.HasSuffix(err.Error(), "closed network connection") {
				return
			} else {
				continue
			}
		}

		if n > 0 {
			if t.isValidPacket(buf[:n]) {
				newBuf := make([]byte, n+16)
				copy(newBuf[16:], buf[:n])
				copy(newBuf[:16], []byte(addr.(*net.IPAddr).IP))

				t.packetsChan <- newBuf
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
	if destPort == t.port || (t.trackResponse && srcPort == t.port) {
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

var bExpectHeader = []byte("Expect:")
var bExpect100Value = []byte("100-continue")
var bPOST = []byte("POST")
var bCRLFx2 = []byte("\r\n\r\n")

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

	// log.Println("Processing packet:", packet.Ack, packet.Seq, packet.ID)

	var message *TCPMessage

	isIncoming := packet.DestPort == t.port

	// Seek for 100-expect chunks
	if parentAck, ok := t.seqWithData[packet.Seq]; ok {
		// In case if non-first data chunks comes first
		for _, m := range t.messages {
			if m.Ack == packet.Ack && bytes.Equal(m.packets[0].Addr, packet.Addr) {
				t.deleteMessage(m)

				if m.AssocMessage != nil {
					m.AssocMessage.AssocMessage = nil
				}

				for _, pkt := range m.packets {
					// log.Println("Updating ack", parentAck, pkt.Ack)
					pkt.UpdateAck(parentAck)
					// Re-queue this packets
					t.processTCPPacket(pkt)
				}
			}
		}

		t.ackAliases[packet.Ack] = parentAck
		packet.UpdateAck(parentAck)
	}

	if alias, ok := t.ackAliases[packet.Ack]; ok {
		packet.UpdateAck(alias)
	}

	var responseRequest *TCPMessage

	if !isIncoming {
		responseRequest, _ = t.respAliases[packet.Ack]
	}

	message, ok := t.messages[packet.ID]

	if !ok {
		message = NewTCPMessage(packet.Seq, packet.Ack, isIncoming)
		t.messages[packet.ID] = message

		if !isIncoming {
			if responseRequest != nil {
				message.AssocMessage = responseRequest
				responseRequest.AssocMessage = message
			} else {
				t.respWithoutReq[packet.Ack] = packet.ID
			}
		}
	}

	// Adding packet to message
	message.AddPacket(packet)

	// Handling Expect: 100-continue requests
	if len(packet.Data) > 4 && bytes.Equal(packet.Data[0:4], bPOST) {
		// reading last 8 bytes for double CRLF
		if bytes.Equal(packet.Data[len(packet.Data)-4:], bCRLFx2) {
			// look for an expect:100-continue header
			if bytes.Equal(bExpect100Value, proto.Header(packet.Data, bExpectHeader)) {
				seq := packet.Seq + uint32(len(packet.Data))
				t.seqWithData[seq] = packet.Ack
				message.DataSeq = seq

				// In case if sequence packet came first
				for _, m := range t.messages {
					if m.Seq == seq {
						t.deleteMessage(m)
						if m.AssocMessage != nil {
							message.AssocMessage = m.AssocMessage
						}
						// log.Println("2: Adding ack alias:", m.Ack, packet.Ack)
						t.ackAliases[m.Ack] = packet.Ack

						for _, pkt := range m.packets {
							pkt.UpdateAck(packet.Ack)
							message.AddPacket(pkt)
						}
					}
				}

				// Removing `Expect: 100-continue` header
				packet.Data = proto.DelHeader(packet.Data, bExpectHeader)

				// log.Println(string(packet.Data))
			}
		}
	}

	// log.Println("Received message:", string(message.Bytes()), message.ID(), t.messages)

	if isIncoming {
		// If message have multiple packets, delete previous alias
		if len(message.packets) > 1 {
			delete(t.respAliases, message.ResponseAck)
		}

		message.UpdateResponseAck()
		t.respAliases[message.ResponseAck] = message
	}

	// If message contains only single packet immediately dispatch it
	if message.IsFinished() {
		if isIncoming {
			// log.Println("I'm finished", string(message.Bytes()), message.ResponseID, t.messages)
			if t.trackResponse {
				if resp, ok := t.messages[message.ResponseID]; ok {
					t.dispatchMessage(message)
					if resp.IsFinished() {
						t.dispatchMessage(resp)
					}
				}
			} else {
				t.dispatchMessage(message)
			}
		} else {
			if message.AssocMessage == nil {
				return
			}

			if req, ok := t.messages[message.AssocMessage.ID()]; ok {
				if req.IsFinished() {
					t.dispatchMessage(req)
					t.dispatchMessage(message)
				}
			}
		}
	}
}

func (t *Listener) IsReady() bool {
	select {
	case <-t.readyCh:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

// Receive TCP messages from the listener channel
func (t *Listener) Receiver() chan *TCPMessage {
	return t.messagesChan
}

func (t *Listener) Close() {
	close(t.quit)
	if t.conn != nil {
		t.conn.Close()
	}

	for _, h := range t.pcapHandles {
		h.Close()
	}

	return
}
