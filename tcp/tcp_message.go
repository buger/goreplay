package tcp

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/buger/goreplay/size"
	"github.com/google/gopacket"
)

// Stats every message carry its own stats object
type Stats struct {
	LostData   int
	Length     int       // length of the data
	Start      time.Time // first packet's timestamp
	End        time.Time // last packet's timestamp
	SrcAddr    string
	DstAddr    string
	IsIncoming bool
	TimedOut   bool // timeout before getting the whole message
	Truncated  bool // last packet truncated due to max message size
	IPversion  byte
}

// Message is the representation of a tcp message
type Message struct {
	packets []*Packet
	done    chan bool
	buf     bytes.Buffer
	Stats
}

// NewMessage ...
func NewMessage(srcAddr, dstAddr string, ipVersion uint8) (m *Message) {
	m = new(Message)
	m.DstAddr = dstAddr
	m.SrcAddr = srcAddr
	m.IPversion = ipVersion
	m.done = make(chan bool)
	return
}

// UUID the unique id of a TCP session it is not granted to be unique!
func (m *Message) UUID() []byte {
	var src, dst string
	if m.IsIncoming {
		src = m.SrcAddr
		dst = m.DstAddr
	} else {
		src = m.DstAddr
		dst = m.SrcAddr
	}

	length := len(src) + len(dst)
	uuid := make([]byte, length)
	copy(uuid, src)
	copy(uuid[len(src):], dst)
	sha := sha1.Sum(uuid)
	uuid = make([]byte, 40)
	hex.Encode(uuid, sha[:])

	return uuid
}

func (m *Message) add(pckt *Packet) {
	m.Length += len(pckt.Payload)
	m.LostData += int(pckt.Lost)
	m.packets = append(m.packets, pckt)
	if len(pckt.Payload) > 0 {
		m.buf.Write(pckt.Payload)
	}
	m.End = pckt.Timestamp
}

// Packets returns packets of this message
func (m *Message) Packets() []*Packet {
	return m.packets
}

// Data returns data in this message
func (m *Message) Data() []byte {
	return m.buf.Bytes()
}

// Sort a helper to sort packets
func (m *Message) Sort() {
	sort.SliceStable(m.packets, func(i, j int) bool { return m.packets[i].Seq < m.packets[j].Seq })
}

// Handler message handler
type Handler func(*Message)

// Debugger is the debugger function. first params is the indicator of the issue's priority
// the higher the number, the lower the priority. it can be 4 <= level <= 6.
type Debugger func(int, ...interface{})

// HintEnd hints the pool to stop the session, see MessagePool.End
// when set, it will be executed before checking FIN or RST flag
type HintEnd func(*Message) bool

// HintStart hints the pool to start the reassembling the message, see MessagePool.Start
// when set, it will be used instead of checking SYN flag
type HintStart func(*Packet) (IsIncoming, IsOutgoing bool)

// MessagePool holds data of all tcp messages in progress(still receiving/sending packets).
// message is identified by its source port and dst port, and last 4bytes of src IP.
type MessagePool struct {
	sync.Mutex
	debug         Debugger
	maxSize       size.Size // maximum message size, default 5mb
	pool          map[uint64]*Message
	handler       Handler
	messageExpire time.Duration // the maximum time to wait for the final packet, minimum is 100ms
	End           HintEnd
	Start         HintStart
}

// NewMessagePool returns a new instance of message pool
func NewMessagePool(maxSize size.Size, messageExpire time.Duration, debugger Debugger, handler Handler) (pool *MessagePool) {
	pool = new(MessagePool)
	pool.debug = debugger
	pool.handler = handler
	pool.messageExpire = time.Millisecond * 100
	if pool.messageExpire < messageExpire {
		pool.messageExpire = messageExpire
	}
	pool.maxSize = maxSize
	if pool.maxSize < 1 {
		pool.maxSize = 5 << 20
	}
	pool.pool = make(map[uint64]*Message)
	return pool
}

// Handler returns packet handler
func (pool *MessagePool) Handler(packet gopacket.Packet) {
	var in, out bool
	pckt, err := ParsePacket(packet)
	if err != nil || pckt == nil {
		go pool.say(4, fmt.Sprintf("error decoding packet(%dBytes):%s\n", packet.Metadata().CaptureLength, err))
		return
	}
	pool.Lock()
	defer pool.Unlock()
	lst := 3
	if pckt.Version == 6 {
		lst = 15
	}
	key := uint64(pckt.SrcPort)<<48 | uint64(pckt.DstPort)<<32 |
		uint64(pckt.SrcIP[lst])<<24 | uint64(pckt.SrcIP[lst-1])<<16 |
		uint64(pckt.SrcIP[lst-2])<<8 | uint64(pckt.SrcIP[lst-3])
	m, ok := pool.pool[key]
	if pckt.RST {
		if ok {
			m.done <- true
			<-m.done
		}
		key = uint64(pckt.DstPort)<<48 | uint64(pckt.SrcPort)<<32 |
			uint64(pckt.DstIP[lst])<<24 | uint64(pckt.DstIP[lst-1])<<16 |
			uint64(pckt.DstIP[lst-2])<<8 | uint64(pckt.DstIP[lst-3])
		m, ok = pool.pool[key]
		if ok {
			m.done <- true
			<-m.done
		}
		go pool.say(4, fmt.Sprintf("RST flag from %s to %s at %s\n", pckt.Src(), pckt.Dst(), pckt.Timestamp))
		return
	}
	switch {
	case ok:
		pool.addPacket(m, pckt)
		return
	case pckt.SYN:
		in = !pckt.ACK
	case pool.Start != nil:
		if in, out = pool.Start(pckt); !(in || out) {
			return
		}
	default:
		return
	}
	m = NewMessage(pckt.Src(), pckt.Dst(), pckt.Version)
	m.IsIncoming = in
	pool.pool[key] = m
	m.Start = pckt.Timestamp
	go pool.dispatch(key, m)
	pool.addPacket(m, pckt)
}

func (pool *MessagePool) dispatch(key uint64, m *Message) {
	select {
	case <-m.done:
		defer func() { m.done <- true }() // signal that message was dispatched
	case <-time.After(pool.messageExpire):
		pool.Lock()
		defer pool.Unlock()
		// avoid dispathing message twice
		if _, ok := pool.pool[key]; !ok {
			return
		}
		m.TimedOut = true
	}
	delete(pool.pool, key)
	pool.handler(m)
}

func (pool *MessagePool) addPacket(m *Message, pckt *Packet) {
	trunc := m.Length + len(pckt.Payload) - int(pool.maxSize)
	if trunc > 0 {
		m.Truncated = true
		pckt.Payload = pckt.Payload[:int(pool.maxSize)-m.Length]
	}
	m.add(pckt)
	switch {
	case trunc >= 0:
	case pckt.FIN:
	case pool.End != nil && pool.End(m):
	default:
		return
	}
	m.done <- true
	<-m.done
}

// this function should not block other pool operations
func (pool *MessagePool) say(level int, args ...interface{}) {
	if pool.debug != nil {
		pool.debug(level, args...)
	}
}
