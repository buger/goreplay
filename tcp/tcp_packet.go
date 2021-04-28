package tcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/buger/goreplay/capture"
)

/*
Packet represent data and layers of packet.
parser extracts information from pcap Packet. functions of *Packet doesn't validate if packet is nil,
calllers must make sure that ParsePacket has'nt returned any error before calling any other
function.
*/
type Packet struct {
	SrcIP, DstIP       net.IP
	Version            uint8
	SrcPort, DstPort   uint16
	Ack, Seq           uint32
	ACK, SYN, FIN, RST bool
	Lost               uint32
	Timestamp          time.Time
	Payload            []byte
}

// ParsePacket parse raw packets
func ParsePacket(packet *capture.Packet) (pckt *Packet, err error) {
	// early check of error
	if packet == nil {
		return nil, errors.New("empty packet")
	}
	if packet.Err != nil {
		return nil, packet.Err
	}

	var t Packet
	pckt = &t
	// TODO: check resolution
	pckt.Timestamp = packet.Info.Timestamp
	if (packet.NetLayer[0] >> 4) == 4 {
		// IPv4 header
		pckt.Version = 4
		pckt.SrcIP = packet.NetLayer[12:16]
		pckt.DstIP = packet.NetLayer[16:20]
	} else {
		// IPv6 header
		pckt.Version = 6
		pckt.SrcIP = packet.NetLayer[8:24]
		pckt.DstIP = packet.NetLayer[24:40]
	}
	pckt.SrcPort = binary.BigEndian.Uint16(packet.TransLayer[0:2])
	pckt.DstPort = binary.BigEndian.Uint16(packet.TransLayer[2:4])
	pckt.Seq = binary.BigEndian.Uint32(packet.TransLayer[4:8])
	pckt.Ack = binary.BigEndian.Uint32(packet.TransLayer[8:12])
	pckt.FIN = packet.TransLayer[13]&0x01 != 0
	pckt.SYN = packet.TransLayer[13]&0x02 != 0
	pckt.RST = packet.TransLayer[13]&0x04 != 0
	pckt.ACK = packet.TransLayer[13]&0x10 != 0
	pckt.Lost = uint32(packet.Info.Length - packet.Info.CaptureLength)
	pckt.Payload = packet.Payload
	return
}

// Src returns the source socket of a packet
func (pckt *Packet) Src() string {
	return fmt.Sprintf("%s:%d", pckt.SrcIP, pckt.SrcPort)
}

// Dst returns destination socket
func (pckt *Packet) Dst() string {
	return fmt.Sprintf("%s:%d", pckt.DstIP, pckt.DstPort)
}
