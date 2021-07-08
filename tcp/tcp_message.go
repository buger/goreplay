package tcp

import (
	"encoding/binary"
	"encoding/hex"
	_ "fmt"
	"reflect"
	"sort"
	"time"
	"unsafe"

	"github.com/buger/goreplay/size"
)

var bufferPool = NewBufferPool(1000, 1)

type buf struct {
	b       []byte
	created time.Time
	gc      bool
}

type bufPool struct {
	buffers chan *buf
	ttl     int
}

func NewBufferPool(max int, ttl int) *bufPool {
	pool := &bufPool{
		buffers: make(chan *buf, max),
		ttl:     ttl,
	}

	// Ensure that memory released over time
	go func() {
		var released int
		// GC
		for {
			for i := 0; i < 100; i++ {
				select {
				case c := <-pool.buffers:
					if now.Sub(c.created) < time.Duration(ttl)*time.Second {
						select {
						case pool.buffers <- c:
						default:
							stats.Add("active_buffer_count", -1)
							c.b = nil
							c.gc = true
							released++
						}
					} else {
						stats.Add("active_buffer_count", -1)
						// Else GC
						c.b = nil
						c.gc = true
						released++
					}
				default:
					break
				}
			}

			bufPoolCount.Set(int64(len(pool.buffers)))
			releasedCount.Set(int64(released))

			time.Sleep(1000 * time.Millisecond)
		}
	}()

	return pool
}

// Borrow a Client from the pool.
func (p *bufPool) Get() *buf {
	var c *buf
	select {
	case c = <-p.buffers:
	default:
		stats.Add("total_alloc_buffer_count", 1)
		stats.Add("active_buffer_count", 1)

		c = new(buf)
		c.b = make([]byte, 1024)
		c.created = now

		// Use this technique to find if pool leaks, and objects get GCd
		//
		// runtime.SetFinalizer(c, func(p *buf) {
		// 	if !p.gc {
		// 		panic("Pool leak")
		// 	}
		// })
	}
	return c
}

// Return returns a Client to the pool.
func (p *bufPool) Put(c *buf) {
	select {
	case p.buffers <- c:
	default:
		stats.Add("active_buffers", -1)
		c.gc = true
		c.b = nil
		// if pool overloaded, let it go
	}
}

func (p *bufPool) Len() int {
	return len(p.buffers)
}

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
	dataBuf  *buf
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

func (m *Message) add(packet *Packet) bool {
	// fmt.Println("SEQ:", packet.Seq, " - ", len(packet.Payload))

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
	m.dataBuf = bufferPool.Get()

	// var totalLen int
	// for _, p := range m.packets {
	// 	totalLen += len(p.Payload)
	// }
	// tmp := make([]byte, totalLen)
	var n int
	if m.dataBuf == nil {
		panic("asdsd")
	}
	m.dataBuf.b, n = copySlice(m.dataBuf.b, m.PacketData()...)

	return m.dataBuf.b[:n]
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

	if m.dataBuf != nil {
		bufferPool.Put(m.dataBuf)
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
	debug   Debugger
	maxSize size.Size // maximum message size, default 5mb
	m       map[uint64]*Message

	messageExpire  time.Duration // the maximum time to wait for the final packet, minimum is 100ms
	allowIncompete bool
	End            HintEnd
	Start          HintStart
	ticker         *time.Ticker
	messages       chan *Message
	packets        chan *Packet
	close          chan struct{} // to signal that we are able to close
}

// NewMessageParser returns a new instance of message parser
func NewMessageParser(maxSize size.Size, messageExpire time.Duration, allowIncompete bool, debugger Debugger) (parser *MessageParser) {
	parser = new(MessageParser)
	parser.debug = debugger

	parser.messageExpire = messageExpire
	if parser.messageExpire == 0 {
		parser.messageExpire = time.Millisecond * 1000
	}

	parser.allowIncompete = allowIncompete
	parser.maxSize = maxSize
	if parser.maxSize < 1 {
		parser.maxSize = 5 << 20
	}

	parser.packets = make(chan *Packet, 10000)
	parser.messages = make(chan *Message, 10000)

	parser.m = make(map[uint64]*Message)
	parser.ticker = time.NewTicker(time.Millisecond * 100)
	parser.close = make(chan struct{}, 1)
	go parser.wait()
	return parser
}

var packetLen int

// Packet returns packet handler
func (parser *MessageParser) PacketHandler(packet *Packet) {
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
			parser.processPacket(pckt)
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

func (parser *MessageParser) processPacket(pckt *Packet) {
	var in, out bool

	// Trying to build unique hash, but there is small chance of collision
	// No matter if it is request or response, all packets in the same message have same
	m, ok := parser.m[pckt.MessageID()]
	switch {
	case ok:
		if !parser.addPacket(m, pckt) {
			packetPool.Put(pckt)
		}
		return
	case parser.Start != nil:
		if in, out = parser.Start(pckt); !(in || out) {
			// Packet can be received out of order, so give it another chance
			if pckt.Retry < 2 && len(pckt.Payload) > 0 {
				// Requeue not known packets
				pckt.Retry++

				select {
				case parser.packets <- pckt:
					return
				default:
				}
			}

			packetPool.Put(pckt)

			return
		}
	default:
		in = pckt.Incoming
	}

	m = new(Message)
	m.IsRequest = in
	m.SrcAddr = pckt.Src()
	m.DstAddr = pckt.Dst()
	parser.m[pckt.MessageID()] = m
	m.Start = pckt.Timestamp
	m.parser = parser
	parser.addPacket(m, pckt)
}

func (parser *MessageParser) addPacket(m *Message, pckt *Packet) bool {
	trunc := m.Length + len(pckt.Payload) - int(parser.maxSize)
	if trunc > 0 {
		m.Truncated = true
		stats.Add("message_timeout_count", 1)
		pckt.Payload = pckt.Payload[:int(parser.maxSize)-m.Length]
	}
	if !m.add(pckt) {
		return false
	}

	if trunc > 0 {
		return false
	}

	// If we are using protocol parsing, like HTTP, depend on its parsing func.
	// For the binary procols wait for message to expire
	if parser.End != nil {
		if parser.End(m) {
			parser.Emit(m)
		}
	}

	return true
}

func (parser *MessageParser) Read() *Message {
	m := <-parser.messages
	return m
}

func (parser *MessageParser) Messages() chan *Message {
	return parser.messages
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

	for _, m := range parser.m {
		if now.Sub(m.End) > parser.messageExpire {
			m.TimedOut = true
			stats.Add("message_timeout_count", 1)
			failMsg++
			if parser.End == nil || parser.allowIncompete {
				parser.Emit(m)
			} else {
				// Just remove
				delete(parser.m, m.packets[0].MessageID())
				m.Finalize()
			}
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
