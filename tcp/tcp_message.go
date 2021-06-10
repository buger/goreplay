package tcp

import (
	"encoding/binary"
	"encoding/hex"
	_ "fmt"
	"sort"
	"time"

	"github.com/buger/goreplay/size"
)

// Stats every message carry its own stats object
type Stats struct {
	LostData  int
	Length    int       // length of the data
	Start     time.Time // first packet's timestamp
	End       time.Time // last packet's timestamp
	SrcAddr   string
	DstAddr   string
	IsRequest bool
	TimedOut  bool // timeout before getting the whole message
	Truncated bool // last packet truncated due to max message size
	IPversion byte
}

// Message is the representation of a tcp message
type Message struct {
	packets  []*Packet
	parser   *MessageParser
	feedback interface{}
	Stats
}

// UUID returns the UUID of a TCP request and its response.
func (m *Message) UUID() []byte {
	var streamID uint64
	pckt := m.packets[0]

	// check if response or request have generated the ID before.
	if m.IsRequest {
		streamID = uint64(pckt.SrcPort)<<48 | uint64(pckt.DstPort)<<32 |
			uint64(ip2int(pckt.SrcIP))
	} else {
		streamID = uint64(pckt.DstPort)<<48 | uint64(pckt.SrcPort)<<32 |
			uint64(ip2int(pckt.DstIP))
	}

	id := make([]byte, 12)
	binary.BigEndian.PutUint64(id, streamID)

	if m.IsRequest {
		binary.BigEndian.PutUint32(id[8:], pckt.Ack)
	} else {
		binary.BigEndian.PutUint32(id[8:], pckt.Seq)
	}

	uuidHex := make([]byte, 24)
	hex.Encode(uuidHex[:], id[:])

	return uuidHex
}

func (m *Message) add(packet *Packet) {
	// fmt.Println("SEQ:", packet.Seq, " - ", len(packet.Payload))

	// Skip duplicates
	for _, p := range m.packets {
		if p.Seq == packet.Seq {
			return
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
	var totalLen int
	for _, p := range m.packets {
		totalLen += len(p.Payload)
	}
	tmp := make([]byte, totalLen)

	var i int
	for _, p := range m.packets {
		i += copy(tmp[i:], p.Payload)
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

func (m *Message) Finalize() {
	// Allow re-use memory
	for _, p := range m.packets {
		packetPool.Put(p)
	}
}

// Emitter message handler
type Emitter func(*Message)

// Debugger is the debugger function. first params is the indicator of the issue's priority
// the higher the number, the lower the priority. it can be 4 <= level <= 6.
type Debugger func(int, ...interface{})

// HintEnd hints the parser to stop the session, see MessageParser.End
// when set, it will be executed before checking FIN or RST flag
type HintEnd func(*Message) bool

// HintStart hints the parser to start the reassembling the message, see MessageParser.Start
// when set, it will be called after checking SYN flag
type HintStart func(*Packet) (IsRequest, IsOutgoing bool)

// MessageParser holds data of all tcp messages in progress(still receiving/sending packets).
// message is identified by its source port and dst port, and last 4bytes of src IP.
type MessageParser struct {
	debug         Debugger
	maxSize       size.Size // maximum message size, default 5mb
	m             map[uint64]*Message
	emit          Emitter
	messageExpire time.Duration // the maximum time to wait for the final packet, minimum is 100ms
	End           HintEnd
	Start         HintStart
	ticker        *time.Ticker
	packets       chan *Packet
	msgs          int32         // messages in the parser
	close         chan struct{} // to signal that we are able to close
}

// NewMessageParser returns a new instance of message parser
func NewMessageParser(maxSize size.Size, messageExpire time.Duration, debugger Debugger, emitHandler Emitter) (parser *MessageParser) {
	parser = new(MessageParser)
	parser.debug = debugger
	parser.emit = emitHandler
	parser.messageExpire = time.Millisecond * 100
	if parser.messageExpire < messageExpire {
		parser.messageExpire = messageExpire
	}
	parser.maxSize = maxSize
	if parser.maxSize < 1 {
		parser.maxSize = 5 << 20
	}
	parser.packets = make(chan *Packet, 1000)
	parser.m = make(map[uint64]*Message)
	parser.ticker = time.NewTicker(time.Millisecond * 50)
	parser.close = make(chan struct{}, 1)
	go parser.wait()
	return parser
}

// Packet returns packet handler
func (parser *MessageParser) PacketHandler(packet *Packet) {
	parser.packets <- packet
}

func (parser *MessageParser) wait() {
	var (
		pckt *Packet
		now  time.Time
	)
	for {
		select {
		case pckt = <-parser.packets:
			parser.processPacket(pckt)
		case now = <-parser.ticker.C:
			parser.timer(now)
		case <-parser.close:
			parser.ticker.Stop()
			// parser.Close should wait for this function to return
			parser.close <- struct{}{}
			return
		}
	}
}

func (parser *MessageParser) processPacket(pckt *Packet) {
	var in, out bool

	// Trying to build unique hash, but there is small chance of collision
	// No matter if it is request or response, all packets in the same message have same
	m, ok := parser.m[pckt.MessageID()]
	switch {
	case ok:
		parser.addPacket(m, pckt)
		return
	case parser.Start != nil:
		if in, out = parser.Start(pckt); !(in || out) {
			// Packet can be received out of order, so give it another chance
			if pckt.Retry < 1 && len(pckt.Payload) > 0 {
				// Requeue not known packets
				pckt.Retry++

				select {
				case parser.packets <- pckt:
				default:
					packetPool.Put(pckt)
					// fmt.Println("Skipping packet")
				}
			}
			return
		}
	}

	m = new(Message)
	m.IsRequest = in
	parser.m[pckt.MessageID()] = m
	m.Start = pckt.Timestamp
	m.parser = parser
	parser.addPacket(m, pckt)
}

func (parser *MessageParser) addPacket(m *Message, pckt *Packet) {
	trunc := m.Length + len(pckt.Payload) - int(parser.maxSize)
	if trunc > 0 {
		m.Truncated = true
		pckt.Payload = pckt.Payload[:int(parser.maxSize)-m.Length]
	}
	m.add(pckt)
	switch {
	// if one of this cases matches, we dispatch the message
	case trunc >= 0:
	case parser.End != nil && parser.End(m):
	default:
		// continue to receive packets
		return
	}

	parser.Emit(m)
}

func (parser *MessageParser) Emit(m *Message) {
	delete(parser.m, m.packets[0].MessageID())
	parser.emit(m)
}

func (parser *MessageParser) timer(now time.Time) {
	for _, m := range parser.m {
		if now.Sub(m.End) > parser.messageExpire {
			m.TimedOut = true
			parser.Emit(m)
		}
	}
}

// this function should not block other parser operations
func (parser *MessageParser) Debug(level int, args ...interface{}) {
	if parser.debug != nil {
		parser.debug(level, args...)
	}
}

func (parser *MessageParser) Close() error {
	parser.close <- struct{}{}
	<-parser.close // wait for timer to be closed!
	return nil
}
