package rawSocket

import (
	"encoding/binary"
	"log"
	"strconv"
	"strings"
	"time"
)

var _ = log.Println

// TCP Flags
const (
	fFIN = 1 << iota
	fSYN
	fRST
	fPSH
	fACK
	fURG
	fECE
	fCWR
	fNS
)

type tcpID [24]byte

// TCPPacket provides tcp packet parser
// Packet structure: http://en.wikipedia.org/wiki/Transmission_Control_Protocol
type TCPPacket struct {
	SrcPort    uint16
	DestPort   uint16
	Seq        uint32
	Ack        uint32
	OrigAck    uint32
	DataOffset uint8
	IsFIN      bool

	Raw       []byte
	Data      []byte
	Addr      []byte
	timestamp time.Time
	ID        tcpID
}

// ParseTCPPacket takes address and tcp payload and returns parsed TCPPacket
func ParseTCPPacket(addr []byte, data []byte, timestamp time.Time) (p *TCPPacket) {
	p = &TCPPacket{Raw: data}
	p.ParseBasic()
	p.Addr = addr
	p.timestamp = timestamp
	p.GenID()

	return
}

func (p *TCPPacket) GenID() {
	copy(p.ID[:16], p.Addr)
	copy(p.ID[16:], p.Raw[0:2])  // Src port
	copy(p.ID[18:], p.Raw[2:4])  // Dest port
	copy(p.ID[20:], p.Raw[8:12]) // Ack
}

func (p *TCPPacket) UpdateAck(ack uint32) {
	p.OrigAck = p.Ack
	p.Ack = ack
	binary.BigEndian.PutUint32(p.Raw[8:12], ack)
	p.GenID()
}

// ParseBasic set of fields
func (t *TCPPacket) ParseBasic() {
	t.DestPort = binary.BigEndian.Uint16(t.Raw[2:4])
	t.SrcPort = binary.BigEndian.Uint16(t.Raw[0:2])
	t.Seq = binary.BigEndian.Uint32(t.Raw[4:8])
	t.Ack = binary.BigEndian.Uint32(t.Raw[8:12])
	t.DataOffset = (t.Raw[12] & 0xF0) >> 4
	t.IsFIN = t.Raw[13]&0x01 != 0

	if len(t.Raw) >= int(t.DataOffset*4) {
		t.Data = t.Raw[t.DataOffset*4:]
	}
}

func (t *TCPPacket) dump() *packet {

	packetSrcIP := make([]byte, 16)
	packetData := make([]byte, len(t.Data)+16)

	copy(packetSrcIP, t.Addr)

	binary.BigEndian.PutUint16(packetData[0:2], t.SrcPort)
	binary.BigEndian.PutUint16(packetData[2:4], t.DestPort)

	binary.BigEndian.PutUint32(packetData[4:8], t.Seq)
	binary.BigEndian.PutUint32(packetData[8:12], t.Ack)

	packetData[12] = 64

	if t.IsFIN {
		packetData[13] = packetData[13] | 0x01
	}

	copy(packetData[16:], t.Data)

	return &packet{
		srcIP:     packetSrcIP,
		data:      packetData,
		timestamp: t.timestamp,
	}

}

// String output for a TCP Packet
func (t *TCPPacket) String() string {
	maxLen := len(t.Data)
	if maxLen > 200 {
		maxLen = 200
	}

	return strings.Join([]string{
		"Addr: " + string(t.Addr),
		"Source port: " + strconv.Itoa(int(t.SrcPort)),
		"Dest port:" + strconv.Itoa(int(t.DestPort)),
		"Sequence:" + strconv.Itoa(int(t.Seq)),
		"Acknowledgment:" + strconv.Itoa(int(t.Ack)),
		"Header len:" + strconv.Itoa(int(t.DataOffset)),
		"FIN:" + strconv.FormatBool(t.IsFIN),

		"Data size:" + strconv.Itoa(len(t.Data)),
		"Data:" + string(t.Data[:maxLen]),
	}, "\n")
}
