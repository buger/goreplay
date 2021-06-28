package tcp

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/google/gopacket"
)

func copySlice(b, a []byte) []byte {
	if cap(b) < len(a) {
		b = make([]byte, len(a))
	}
	copy(b, a)
	return b[:len(a)]
}

var packetPool = NewPool(10000)

// Pool holds Clients.
type Pool struct {
	pool chan *Packet
}

// NewPool creates a new pool of Clients.
func NewPool(max int) *Pool {
	return &Pool{
		pool: make(chan *Packet, max),
	}
}

// Borrow a Client from the pool.
func (p *Pool) Get() *Packet {
	var c *Packet
	select {
	case c = <-p.pool:
	default:
		c = new(Packet)
	}
	return c
}

// Return returns a Client to the pool.
func (p *Pool) Put(c *Packet) {
	select {
	case p.pool <- c:
	default:
		// let it go, let it go...
	}
}

/*
Packet represent data and layers of packet.
parser extracts information from pcap Packet. functions of *Packet doesn't validate if packet is nil,
calllers must make sure that ParsePacket has'nt returned any error before calling any other
function.
*/
type Packet struct {
	Incoming           bool
	messageID          uint64
	SrcIP, DstIP       net.IP
	Version            uint8
	SrcPort, DstPort   uint16
	Ack, Seq           uint32
	ACK, SYN, FIN, RST bool
	Lost               uint32
	Retry              int
	CaptureLength      int
	Timestamp          time.Time
	Payload            []byte
}

// ParsePacket parse raw packets
func ParsePacket(data []byte, lType, lTypeLen int, cp *gopacket.CaptureInfo, allowEmpty bool) (pckt *Packet, err error) {
	pckt = packetPool.Get()
	if err := pckt.parse(data, lType, lTypeLen, cp, allowEmpty); err != nil {
		packetPool.Put(pckt)
		return nil, err
	}

	return pckt, nil
}

func (pckt *Packet) parse(data []byte, lType, lTypeLen int, cp *gopacket.CaptureInfo, allowEmpty bool) error {
	pckt.Retry = 0
	pckt.messageID = 0

	// TODO: check resolution
	pckt.Timestamp = cp.Timestamp

	if len(data) < lTypeLen {
		return ErrHdrLength("Link")
	}
	if len(data) <= lTypeLen {
		return ErrHdrMissing("IPv4 or IPv6")
	}

	ldata := data[lTypeLen:]
	var proto byte
	var netLayer, transLayer []byte

	if ldata[0]>>4 == 4 {
		// IPv4 header
		if len(ldata) < 20 {
			return ErrHdrLength("IPv4")
		}
		proto = ldata[9]
		ihl := int(ldata[0]&0x0F) * 4
		if ihl < 20 {
			return ErrHdrInvalid("IPv4's IHL")
		}
		if len(ldata) < ihl {
			return ErrHdrLength("IPv4 opts")
		}
		netLayer = ldata[:ihl]
	} else if ldata[0]>>4 == 6 {
		if len(ldata) < 40 {
			return ErrHdrLength("IPv6")
		}
		proto = ldata[6]
		totalLen := 40
		for ipv6ExtensionHdr(proto) {
			hdr := len(ldata) - totalLen
			if hdr < 8 {
				return ErrHdrExpected("IPv6 opts")
			}
			extLen := 8
			if proto != 44 {
				extLen = int(ldata[totalLen+1]+1) * 8
			}
			if hdr < extLen {
				return ErrHdrLength("IPv6 opts")
			}
			proto = ldata[totalLen]
			totalLen += extLen
		}
		netLayer = ldata[:totalLen]
	} else {
		return ErrHdrExpected("IPv4 or IPv6")
	}
	if proto != 6 {
		return ErrHdrExpected("TCP")
	}
	if len(data) <= len(netLayer) {
		return ErrHdrMissing("TCP")
	}
	ndata := ldata[len(netLayer):]
	// TCP header
	if len(ndata) < 20 {
		return ErrHdrLength("TCP")
	}
	dOf := int(ndata[12]>>4) * 4
	if dOf < 20 {
		return ErrHdrInvalid("TCP's ndata offset")
	}
	if len(ndata) < dOf {
		return ErrHdrLength("TCP opts")
	}

	if !allowEmpty && len(ndata[dOf:]) == 0 {
		return EmptyPacket("")
	}

	if (netLayer[0] >> 4) == 4 {
		// IPv4 header
		pckt.Version = 4
		pckt.SrcIP = netLayer[12:16]
		pckt.DstIP = netLayer[16:20]
	} else {
		// IPv6 header
		pckt.Version = 6
		pckt.SrcIP = netLayer[8:24]
		pckt.DstIP = netLayer[24:40]
	}

	transLayer = ndata[:dOf]

	pckt.CaptureLength = cp.CaptureLength
	pckt.SrcPort = binary.BigEndian.Uint16(transLayer[0:2])
	pckt.DstPort = binary.BigEndian.Uint16(transLayer[2:4])
	pckt.Seq = binary.BigEndian.Uint32(transLayer[4:8])
	pckt.Ack = binary.BigEndian.Uint32(transLayer[8:12])
	pckt.FIN = transLayer[13]&0x01 != 0
	pckt.SYN = transLayer[13]&0x02 != 0
	pckt.RST = transLayer[13]&0x04 != 0
	pckt.ACK = transLayer[13]&0x10 != 0
	pckt.Lost = uint32(cp.Length - cp.CaptureLength)
	pckt.Payload = copySlice(pckt.Payload, ndata[dOf:])

	return nil
}

func (pckt *Packet) MessageID() uint64 {
	if pckt.messageID == 0 {
		// All packets in the same message will share the same ID
		pckt.messageID = uint64(pckt.SrcPort)<<48 | uint64(pckt.DstPort)<<32 |
			(uint64(ip2int(pckt.SrcIP)) + uint64(ip2int(pckt.DstIP)) + uint64(pckt.Ack))
	}

	return pckt.messageID
}

// Src returns the source socket of a packet
func (pckt *Packet) Src() string {
	return fmt.Sprintf("%s:%d", pckt.SrcIP, pckt.SrcPort)
}

// Dst returns destination socket
func (pckt *Packet) Dst() string {
	return fmt.Sprintf("%s:%d", pckt.DstIP, pckt.DstPort)
}

type EmptyPacket string

func (err EmptyPacket) Error() string {
	return "Empty packet"
}

// ErrHdrLength returned on short header length
type ErrHdrLength string

func (err ErrHdrLength) Error() string {
	return "short " + string(err) + " length"
}

// ErrHdrMissing returned on missing header(s)
type ErrHdrMissing string

func (err ErrHdrMissing) Error() string {
	return "missing " + string(err) + " header(s)"
}

// ErrHdrExpected returned when header(s) are different from the one expected
type ErrHdrExpected string

func (err ErrHdrExpected) Error() string {
	return "expected " + string(err) + " header(s)"
}

// ErrHdrInvalid returned when header(s) are different from the one expected
type ErrHdrInvalid string

func (err ErrHdrInvalid) Error() string {
	return "invalid " + string(err) + " value"
}

// https://en.wikipedia.org/wiki/IPv6_packet#Extension_headers
func ipv6ExtensionHdr(b byte) bool {
	// TODO: support all extension headers
	return b == 0 || b == 43 || b == 44
}

func ip2int(ip net.IP) uint32 {
	if len(ip) == 0 {
		return 0
	}

	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}
