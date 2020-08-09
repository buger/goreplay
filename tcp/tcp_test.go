package tcp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/buger/goreplay/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

var decodeOpts = gopacket.DecodeOptions{Lazy: true, NoCopy: true}

func headersIP4(seq uint32, length uint16) (headers [54]byte) {
	// set ethernet headers
	binary.BigEndian.PutUint16(headers[12:14], uint16(layers.EthernetTypeIPv4))

	// set ip header
	ip := headers[14:]
	copy(ip[0:2], []byte{4<<4 | 5, 0x28<<2 | 0x00})
	binary.BigEndian.PutUint16(ip[2:4], length+40)
	ip[9] = uint8(layers.IPProtocolTCP)
	copy(ip[12:16], []byte{192, 168, 1, 2})
	copy(ip[16:], []byte{192, 168, 1, 3})

	// set tcp header
	tcp := ip[20:]
	binary.BigEndian.PutUint16(tcp[0:2], 45678)
	binary.BigEndian.PutUint16(tcp[2:4], 8001)
	tcp[12] = 5 << 4
	return
}

func GetPackets(start uint32, _len int, payload []byte) []gopacket.Packet {
	var packets = make([]gopacket.Packet, _len)
	for i := start; i < start+uint32(_len); i++ {
		data := make([]byte, 54+len(payload))
		h := headersIP4(i, uint16(len(payload)))
		copy(data, h[:])
		copy(data[len(h):], payload)
		packets[i-start] = gopacket.NewPacket(data, layers.LinkTypeEthernet, decodeOpts)
	}
	return packets
}

func TestMessageParserWithHint(t *testing.T) {
	var mssg = make(chan *Message, 3)
	pool := NewMessagePool(1<<20, time.Second, nil, func(m *Message) { mssg <- m })
	pool.Start = func(pckt *Packet) (bool, bool) {
		return proto.HasRequestTitle(pckt.Payload), proto.HasResponseTitle(pckt.Payload)
	}
	pool.End = func(m *Message) bool {
		return proto.HasFullPayload(m.Data())
	}
	packets := GetPackets(1, 30, nil)
	packets[0].Data()[14:][20:][13] = 2  // SYN flag
	packets[10].Data()[14:][20:][13] = 2 // SYN flag
	packets[29].Data()[14:][20:][13] = 1 // FIN flag
	packets[4] = GetPackets(5, 1, []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n7"))[0]
	packets[5] = GetPackets(6, 1, []byte("\r\nMozilla\r\n9\r\nDeveloper\r"))[0]
	packets[6] = GetPackets(7, 1, []byte("\n7\r\nNetwork\r\n0\r\n\r\n"))[0]
	packets[14] = GetPackets(5, 1, []byte("POST / HTTP/1.1\r\nContent-Type: text/plain\r\nContent-Length: 23\r\n\r\n"))[0]
	packets[15] = GetPackets(6, 1, []byte("MozillaDeveloper"))[0]
	packets[16] = GetPackets(7, 1, []byte("Network"))[0]
	packets[24] = GetPackets(5, 1, []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 0\r\n\r"))[0]
	for i := 0; i < 30; i++ {
		pool.Handler(packets[i])
	}
	var m *Message
	select {
	case <-time.After(time.Second):
		t.Errorf("can't parse packets fast enough")
		return
	case m = <-mssg:
	}
	if len(m.packets) != 3 {
		t.Errorf("expected to have 3 packets got %d", len(m.packets))
	}
	if !bytes.HasSuffix(m.Data(), []byte("\n7\r\nNetwork\r\n0\r\n\r\n")) {
		t.Errorf("expected to %q to have suffix %q", m.Data(), []byte("\n7\r\nNetwork\r\n0\r\n\r\n"))
	}

	select {
	case <-time.After(time.Second):
		t.Errorf("can't parse packets fast enough")
		return
	case m = <-mssg:
	}
	if len(m.packets) != 3 {
		t.Errorf("expected to have 3 packets got %d", len(m.packets))
	}
	if !bytes.HasSuffix(m.Data(), []byte("Network")) {
		t.Errorf("expected to %q to have suffix %q", m.Data(), []byte("Network"))
	}

	select {
	case <-time.After(time.Second):
		t.Errorf("can't parse packets fast enough")
		return
	case m = <-mssg:
	}
	if len(m.packets) != 6 {
		t.Errorf("expected to have 6 packets got %d", len(m.packets))
	}
	if !bytes.HasSuffix(m.Data(), []byte("Content-Length: 0\r\n\r")) {
		t.Errorf("expected to %q to have suffix %q", m.Data(), []byte("Content-Length: 0\r\n\r"))
	}

}

func TestMessageParserWithoutHint(t *testing.T) {
	var mssg = make(chan *Message, 1)
	var data [63 << 10]byte
	packets := GetPackets(1, 10, data[:])
	packets[0].Data()[14:][20:][13] = 2 // SYN flag
	packets[9].Data()[14:][20:][13] = 1 // FIN flag
	p := NewMessagePool(63<<10*10, time.Second, nil, func(m *Message) { mssg <- m })
	for _, v := range packets {
		p.Handler(v)
	}
	var m *Message
	select {
	case <-time.After(time.Second):
		t.Errorf("can't parse packets fast enough")
		return
	case m = <-mssg:
	}
	if m.Length != 63<<10*10 {
		t.Errorf("expected %d to equal %d", m.Length, 63<<10*10)
	}
}

func TestMessageMaxSizeReached(t *testing.T) {
	var mssg = make(chan *Message, 2)
	var data [63 << 10]byte
	packets := GetPackets(1, 2, data[:])
	packets = append(packets, GetPackets(3, 1, make([]byte, 63<<10+10))...)
	packets[0].Data()[14:][20:][13] = 2 // SYN flag
	packets[2].Data()[14:][20:][13] = 2 // SYN flag
	packets[2].Data()[14:][15] = 3      // changing address
	p := NewMessagePool(63<<10+10, time.Second, nil, func(m *Message) { mssg <- m })
	for _, v := range packets {
		p.Handler(v)
	}
	var m *Message
	select {
	case <-time.After(time.Second):
		t.Errorf("can't parse packets fast enough")
		return
	case m = <-mssg:
	}
	if m.Length != 63<<10+10 {
		t.Errorf("expected %d to equal %d", m.Length, 63<<10+10)
	}
	if !m.Truncated {
		t.Error("expected message to be truncated")
	}

	select {
	case <-time.After(time.Second):
		t.Errorf("can't parse packets fast enough")
		return
	case m = <-mssg:
	}
	if m.Length != 63<<10+10 {
		t.Errorf("expected %d to equal %d", m.Length, 63<<10+10)
	}
	if m.Truncated {
		t.Error("expected message to not be truncated")
	}
}

func TestMessageTimeoutReached(t *testing.T) {
	var mssg = make(chan *Message, 2)
	var data [63 << 10]byte
	packets := GetPackets(1, 2, data[:])
	packets[0].Data()[14:][20:][13] = 2 // SYN flag
	p := NewMessagePool(1<<20, 0, nil, func(m *Message) { mssg <- m })
	p.Handler(packets[0])
	time.Sleep(time.Millisecond * 200)
	p.Handler(packets[1])
	m := <-mssg
	if m.Length != 63<<10 {
		t.Errorf("expected %d to equal %d", m.Length, 63<<10)
	}
	if !m.TimedOut {
		t.Error("expected message to be timeout")
	}
}

func TestMessageUUID(t *testing.T) {
	m1 := &Message{}
	m1.IsIncoming = true
	m1.SrcAddr = "src"
	m1.DstAddr = "dst"
	m2 := &Message{}
	m2.SrcAddr = "dst"
	m2.DstAddr = "src"
	if string(m1.UUID()) != string(m2.UUID()) {
		t.Errorf("expected %s, to equal %s", m1.UUID(), m2.UUID())
	}
}

func BenchmarkPacketParseAndSort(b *testing.B) {
	if b.N < 3 {
		return
	}
	now := time.Now()
	m := new(Message)
	m.packets = make([]*Packet, b.N)
	for i, v := range GetPackets(1, b.N, nil) {
		m.packets[i], _ = ParsePacket(v)
	}
	m.Sort()
	b.Logf("%d packets in %s", b.N, time.Since(now))
}

func BenchmarkMessageParserWithoutHint(b *testing.B) {
	var mssg = make(chan *Message, 1)
	if b.N < 3 {
		return
	}
	now := time.Now()
	n := b.N
	packets := GetPackets(1, n, nil)
	packets[0].Data()[14:][20:][13] = 2     // SYN flag
	packets[b.N-1].Data()[14:][20:][13] = 1 // FIN flag
	p := NewMessagePool(1<<20, time.Second*2, nil, func(m *Message) {
		b.Logf("%d/%d packets in %s", len(m.packets), n, time.Since(now))
		mssg <- m
	})
	for _, v := range packets {
		p.Handler(v)
	}
	<-mssg
}

func BenchmarkMessageParserWithHint(b *testing.B) {
	if b.N < 3 {
		return
	}
	now := time.Now()
	n := b.N
	var mssg = make(chan *Message, 1)
	payload := make([]byte, 0xfc00)
	for i := 0; i < 0xfc00; i++ {
		payload[i] = '1'
	}
	pool := NewMessagePool(1<<30, time.Second*10, nil, func(m *Message) { mssg <- m })
	pool.Start = func(pckt *Packet) (bool, bool) {
		return proto.HasRequestTitle(pckt.Payload), proto.HasResponseTitle(pckt.Payload)
	}
	pool.End = func(m *Message) bool {
		return proto.HasFullPayload(m.Data())
	}
	pool.Handler(GetPackets(1, 1, []byte("POST / HTTP/1.1\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n"))[0])
	i := 0
	var d []byte
	for {
		select {
		case m := <-mssg:
			b.Logf("%d/%d packets, %dbytes, truncated: %v, timedout: %v in %s", len(m.packets), n, m.Length, m.Truncated, m.TimedOut, time.Since(now))
			return
		default:
			if i > n-2 {
				break
			} else if i < n-2 {
				d = []byte(fmt.Sprintf("fc00\r\n%s\r\n", payload))
			} else {
				d = []byte("0\r\n\r\n")
			}
			pool.Handler(GetPackets(1, i+2, d)[0])
			i++
		}
	}
}
