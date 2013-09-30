package listener

import (
	"encoding/binary"
	"strconv"
	"strings"
)

// TCP Flags
const (
	TCP_FIN = 1 << iota
	TCP_SYN
	TCP_RST
	TCP_PSH
	TCP_ACK
	TCP_URG
	TCP_ECE
	TCP_CWR
	TCP_NS
)

// Simple TCP packet parser
//
// Packet structure: http://en.wikipedia.org/wiki/Transmission_Control_Protocol
type TCPPacket struct {
	SrcPort    uint16
	DestPort   uint16
	Seq        uint32
	Ack        uint32
	DataOffset uint8
	Flags      uint16
	Window     uint16
	Checksum   uint16
	Urgent     uint16

	Data []byte
}

func ParseTCPPacket(b []byte) (p *TCPPacket) {
	p = &TCPPacket{Data: b}
	p.ParseBasic()

	return p
}

// Parse TCP Packet, inspired by: https://github.com/miekg/pcap/blob/master/packet.go
func (t *TCPPacket) Parse() {
	t.ParseBasic()
	t.SrcPort = binary.BigEndian.Uint16(t.Data[0:2])
	t.DestPort = binary.BigEndian.Uint16(t.Data[2:4])
	t.Flags = binary.BigEndian.Uint16(t.Data[12:14]) & 0x1FF
	t.Window = binary.BigEndian.Uint16(t.Data[14:16])
	t.Checksum = binary.BigEndian.Uint16(t.Data[16:18])
	t.Urgent = binary.BigEndian.Uint16(t.Data[18:20])
}

// ParseBasic set of fields
func (t *TCPPacket) ParseBasic() {
	t.Seq = binary.BigEndian.Uint32(t.Data[4:8])
	t.Ack = binary.BigEndian.Uint32(t.Data[8:12])
	t.DataOffset = (t.Data[12] & 0xF0) >> 4

	t.Data = t.Data[t.DataOffset*4:]
}

// String output for a TCP Packet
func (t *TCPPacket) String() string {
	return strings.Join([]string{
		"Source port: " + strconv.Itoa(int(t.SrcPort)),
		"Dest port:" + strconv.Itoa(int(t.DestPort)),
		"Sequence:" + strconv.Itoa(int(t.Seq)),
		"Acknowledgment:" + strconv.Itoa(int(t.Ack)),
		"Header len:" + strconv.Itoa(int(t.DataOffset)),

		"Flag ns:" + strconv.FormatBool(t.Flags&TCP_NS != 0),
		"Flag crw:" + strconv.FormatBool(t.Flags&TCP_CWR != 0),
		"Flag ece:" + strconv.FormatBool(t.Flags&TCP_ECE != 0),
		"Flag urg:" + strconv.FormatBool(t.Flags&TCP_URG != 0),
		"Flag ack:" + strconv.FormatBool(t.Flags&TCP_ACK != 0),
		"Flag psh:" + strconv.FormatBool(t.Flags&TCP_PSH != 0),
		"Flag rst:" + strconv.FormatBool(t.Flags&TCP_RST != 0),
		"Flag syn:" + strconv.FormatBool(t.Flags&TCP_SYN != 0),
		"Flag fin:" + strconv.FormatBool(t.Flags&TCP_FIN != 0),

		"Window size:" + strconv.Itoa(int(t.Window)),
		"Checksum:" + strconv.Itoa(int(t.Checksum)),

		"Data:" + string(t.Data),
	}, "\n")
}
