package tcp

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/buger/goreplay/proto"
	"net"
	"reflect"
	"sort"
	"time"
	"unsafe"
)

// TCPProtocol is a number to indicate type of protocol
type TCPProtocol uint8

const (
	// ProtocolHTTP ...
	ProtocolHTTP TCPProtocol = iota
	// ProtocolBinary ...
	ProtocolBinary
)

// Set is here so that TCPProtocol can implement flag.Var
func (protocol *TCPProtocol) Set(v string) error {
	switch v {
	case "", "http":
		*protocol = ProtocolHTTP
	case "binary":
		*protocol = ProtocolBinary
	default:
		return fmt.Errorf("unsupported protocol %s", v)
	}
	return nil
}

func (protocol *TCPProtocol) String() string {
	switch *protocol {
	case ProtocolBinary:
		return "binary"
	case ProtocolHTTP:
		return "http"
	default:
		return ""
	}
}

// Stats every message carry its own stats object
type Stats struct {
	LostData  int
	Length    int       // length of the data
	Start     time.Time // first packet's timestamp
	End       time.Time // last packet's timestamp
	SrcAddr   string
	DstAddr   string
	Direction Dir
	TimedOut  bool // timeout before getting the whole message
	Truncated bool // last packet truncated due to max message size
	IPversion byte
}

// Message is the representation of a tcp message
type Message struct {
	packets          []*Packet
	parser           *MessageParser
	feedback         interface{}
	continueAdjusted bool
	Stats
}

// UUID returns the UUID of a TCP request and its response.
func (m *Message) UUID() []byte {
	var streamID uint64
	pckt := m.packets[0]

	// check if response or request have generated the ID before.
	if m.Direction == DirIncoming {
		streamID = uint64(pckt.SrcPort)<<48 | uint64(pckt.DstPort)<<32 |
			uint64(ip2int(pckt.SrcIP))
	} else {
		streamID = uint64(pckt.DstPort)<<48 | uint64(pckt.SrcPort)<<32 |
			uint64(ip2int(pckt.DstIP))
	}

	id := make([]byte, 12)
	binary.BigEndian.PutUint64(id, streamID)

	if m.Direction == DirIncoming {
		binary.BigEndian.PutUint32(id[8:], pckt.Ack)
	} else {
		binary.BigEndian.PutUint32(id[8:], pckt.Seq)
	}

	uuidHex := make([]byte, 24)
	hex.Encode(uuidHex[:], id[:])

	return uuidHex
}

func (m *Message) add(packet *Packet) bool {
	// Skip duplicates
	for _, p := range m.packets {
		if p.Seq == packet.Seq {
			return false
		}
	}

	// Packets not always captured in same Seq order, and sometimes we need to prepend
	if len(m.packets) == 0 || packet.Seq > m.packets[len(m.packets)-1].Seq {
		m.packets = append(m.packets, packet)
	} else if packet.Seq < m.packets[0].Seq {
		m.packets = append([]*Packet{packet}, m.packets...)
	} else { // insert somewhere in the middle...
		for i, p := range m.packets {
			if packet.Seq < p.Seq {
				m.packets = append(m.packets[:i], append([]*Packet{packet}, m.packets[i:]...)...)
				break
			}
		}
	}

	m.Length += len(packet.Payload)
	m.LostData += int(packet.Lost)

	if packet.Timestamp.After(m.End) || m.End.IsZero() {
		m.End = packet.Timestamp
	}

	return true
}

// Packets returns packets of the message
func (m *Message) Packets() []*Packet {
	return m.packets
}

func (m *Message) MissingChunk() bool {
	nextSeq := m.packets[0].Seq

	for _, p := range m.packets {
		if p.Seq != nextSeq {
			return true
		}

		nextSeq += uint32(len(p.Payload))
	}

	return false
}

func (m *Message) PacketData() [][]byte {
	tmp := make([][]byte, len(m.packets))

	for i, p := range m.packets {
		tmp[i] = p.Payload
	}

	return tmp
}

// Data returns data in this message
func (m *Message) Data() []byte {
	packetData := m.PacketData()
	tmp := packetData[0]

	if len(packetData) > 0 {
		tmp, _ = copySlice(tmp, len(packetData[0]), packetData[1:]...)
	}

	// Remove Expect header, since its replay not fully supported
	if state, ok := m.feedback.(*proto.HTTPState); ok {
		if state.Continue100 {
			tmp = proto.DeleteHeader(tmp, []byte("Expect"))
		}
	}

	return tmp
}

// SetProtocolState set feedback/data that can be used later, e.g with End or Start hint
func (m *Message) SetProtocolState(feedback interface{}) {
	m.feedback = feedback
}

// ProtocolState returns feedback associated to this message
func (m *Message) ProtocolState() interface{} {
	return m.feedback
}

// Sort a helper to sort packets
func (m *Message) Sort() {
	sort.SliceStable(m.packets, func(i, j int) bool { return m.packets[i].Seq < m.packets[j].Seq })
}

// Emitter message handler
type Emitter func(*Message)

// HintEnd hints the parser to stop the session, see MessageParser.End
// when set, it will be executed before checking FIN or RST flag
type HintEnd func(*Message) bool

// HintStart hints the parser to start the reassembling the message, see MessageParser.Start
// when set, it will be called after checking SYN flag
type HintStart func(*Packet) (IsRequest, IsOutgoing bool)

// MessageParser holds data of all tcp messages in progress(still receiving/sending packets).
// message is identified by its source port and dst port, and last 4bytes of src IP.
type MessageParser struct {
	m map[uint64]*Message

	messageExpire  time.Duration // the maximum time to wait for the final packet, minimum is 100ms
	allowIncompete bool
	End            HintEnd
	Start          HintStart
	ticker         *time.Ticker
	messages       chan *Message
	packets        chan *PcapPacket
	close          chan struct{} // to signal that we are able to close
	ports          []uint16
	ips            []net.IP
}

// NewMessageParser returns a new instance of message parser
func NewMessageParser(messages chan *Message, ports []uint16, ips []net.IP, messageExpire time.Duration, allowIncompete bool) (parser *MessageParser) {
	parser = new(MessageParser)

	parser.messageExpire = messageExpire
	if parser.messageExpire == 0 {
		parser.messageExpire = time.Millisecond * 1000
	}

	parser.allowIncompete = allowIncompete

	parser.packets = make(chan *PcapPacket, 10000)

	if messages == nil {
		messages = make(chan *Message, 1000)
	}
	parser.messages = messages

	parser.m = make(map[uint64]*Message)
	parser.ticker = time.NewTicker(time.Millisecond * 100)
	parser.close = make(chan struct{}, 1)

	parser.ports = ports
	parser.ips = ips

	go parser.wait()
	return parser
}

var packetLen int

// Packet returns packet handler
func (parser *MessageParser) PacketHandler(packet *PcapPacket) {
	packetLen++
	parser.packets <- packet
}

func (parser *MessageParser) wait() {
	var (
		now time.Time
	)
	for {
		select {
		case pckt := <-parser.packets:
			parser.processPacket(parser.parsePacket(pckt))
		case now = <-parser.ticker.C:
			parser.timer(now)
		case <-parser.close:
			parser.ticker.Stop()
			// parser.Close should wait for this function to return
			parser.close <- struct{}{}
			return
			// default:
		}
	}
}

func (parser *MessageParser) parsePacket(pcapPkt *PcapPacket) *Packet {
	pckt, err := ParsePacket(pcapPkt.Data, pcapPkt.LType, pcapPkt.LTypeLen, pcapPkt.Ci, false)
	if err != nil {
		if _, empty := err.(EmptyPacket); !empty {
			stats.Add("packet_error", 1)
		}
		return nil
	}

	for _, p := range parser.ports {
		if pckt.DstPort == p && containsOrEmpty(pckt.DstIP, parser.ips) {
			pckt.Direction = DirIncoming
			break
		} else if pckt.SrcPort == p && containsOrEmpty(pckt.SrcIP, parser.ips) {
			pckt.Direction = DirOutcoming
			break
		}
	}

	return pckt
}

func containsOrEmpty(element net.IP, ipList []net.IP) bool {
	if len(ipList) == 0 {
		return true
	}
	for _, ip := range ipList {
		if ip.Equal(element) {
			return true
		}
	}
	return false
}

func (parser *MessageParser) processPacket(pckt *Packet) {
	if pckt == nil {
		return
	}

	// Trying to build unique hash, but there is small chance of collision
	// No matter if it is request or response, all packets in the same message have same
	m, ok := parser.m[pckt.MessageID()]
	switch {
	case ok:
		if m.Direction == DirUnknown {
			if in, out := parser.Start(pckt); in || out {
				if in {
					m.Direction = DirIncoming
				} else {
					m.Direction = DirOutcoming
				}
			}
		}
		parser.addPacket(m, pckt)
		return
	case pckt.Direction == DirUnknown && parser.Start != nil:
		if in, out := parser.Start(pckt); in || out {
			if in {
				pckt.Direction = DirIncoming
			} else {
				pckt.Direction = DirOutcoming
			}
		}
	}

	m = new(Message)
	m.Direction = pckt.Direction
	m.SrcAddr = pckt.SrcIP.String()
	m.DstAddr = pckt.DstIP.String()

	parser.m[pckt.MessageID()] = m

	m.Start = pckt.Timestamp
	m.parser = parser
	parser.addPacket(m, pckt)
}

func (parser *MessageParser) addPacket(m *Message, pckt *Packet) bool {
	if !m.add(pckt) {
		return false
	}

	// If we are using protocol parsing, like HTTP, depend on its parsing func.
	// For the binary procols wait for message to expire
	if parser.End != nil {
		if parser.End(m) {
			parser.Emit(m)
			return true
		}

		parser.Fix100Continue(m)
	}

	return true
}

func (parser *MessageParser) Fix100Continue(m *Message) {
	// Only adjust a message once
	if state, ok := m.feedback.(*proto.HTTPState); ok && state.Continue100 && !m.continueAdjusted {
		// Shift Ack by given offset
		// Size of "HTTP/1.1 100 Continue\r\n\r\n" message
		for _, p := range m.packets {
			p.messageID = 0
			p.Ack += 25
		}

		// If next section was aready approved and received, merge messages
		if next, found := parser.m[m.packets[0].MessageID()]; found {
			for _, p := range next.packets {
				parser.addPacket(m, p)
			}
		}

		// Re-add (or override) again with new message and ID
		parser.m[m.packets[0].MessageID()] = m
		m.continueAdjusted = true
	}
}

func (parser *MessageParser) Read() *Message {
	m := <-parser.messages
	return m
}

func (parser *MessageParser) Emit(m *Message) {
	stats.Add("message_count", 1)

	delete(parser.m, m.packets[0].MessageID())

	parser.messages <- m
}

func GetUnexportedField(field reflect.Value) interface{} {
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
}

var failMsg int

func (parser *MessageParser) timer(now time.Time) {
	packetLen = 0

	packetQueueLen.Set(int64(len(parser.packets)))
	messageQueueLen.Set(int64(len(parser.m)))

	for _, m := range parser.m {
		if now.Sub(m.End) > parser.messageExpire {
			m.TimedOut = true
			stats.Add("message_timeout_count", 1)
			failMsg++
			if parser.End == nil || parser.allowIncompete {
				parser.Emit(m)
			}

			delete(parser.m, m.packets[0].MessageID())
		}
	}
}

func (parser *MessageParser) Close() error {
	parser.close <- struct{}{}
	<-parser.close // wait for timer to be closed!
	return nil
}
