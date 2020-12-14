package capture

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// Packet properties of a pcaket
type Packet struct {
	Data         []byte
	LinkLayer    []byte
	NetLayer     []byte
	NetOptsLen   int // length of extension headers(IPV6) or options(IPV4)
	TransLayer   []byte
	TransOptsLen int // length of tcp options
	Payload      []byte

	LinkType int
	Info     *gopacket.CaptureInfo
	Err      error
}

// NewPacket parses packet and stop at the first error encountered
// pckt.Error will be nil if packet was parsed successfully
func NewPacket(data []byte, lType, lTypeLen int, cp *gopacket.CaptureInfo) (p *Packet) {
	p = new(Packet)
	p.Info = cp
	p.Data = data
	if len(data) < lTypeLen {
		p.Err = ErrHdrLength("Link")
		return
	}
	p.LinkType = lType
	p.LinkLayer = data[:lTypeLen]
	if len(data) <= lTypeLen {
		p.Err = ErrHdrMissing("IPv4 or IPv6")
		return
	}
	data = data[lTypeLen:]
	var proto byte
	if data[0]>>4 == 4 {
		// IPv4 header
		if len(data) < 20 {
			p.Err = ErrHdrLength("IPv4")
			return
		}
		proto = data[9]
		ihl := int(data[0]&0x0F) * 4
		if ihl < 20 {
			p.Err = ErrHdrInvalid("IPv4's IHL")
			return
		}
		if len(data) < ihl {
			p.Err = ErrHdrLength("IPv4 opts")
			p.NetLayer = data[:20]
			return
		}
		p.NetOptsLen = ihl - 20
		p.NetLayer = data[:ihl]
	} else if data[0]>>4 == 6 {
		if len(data) < 40 {
			p.Err = ErrHdrLength("IPv6")
			return
		}
		proto = data[6]
		totalLen := 40
		for ipv6ExtensionHdr(proto) {
			hdr := len(data) - totalLen
			if hdr < 8 {
				p.Err = ErrHdrExpected("IPv6 opts")
				break
			}
			extLen := 8
			if proto != 44 {
				extLen = int(data[totalLen+1]+1) * 8
			}
			if hdr < extLen {
				p.Err = ErrHdrLength("IPv6 opts")
				break
			}
			p.NetOptsLen += extLen
			proto = data[totalLen]
			totalLen += extLen
		}
		p.NetLayer = data[:totalLen]
		if p.Err != nil {
			return
		}
	} else {
		p.Err = ErrHdrExpected("IPv4 or IPv6")
		return
	}
	if proto != 6 {
		p.Err = ErrHdrExpected("TCP")
		return
	}
	if len(data) <= len(p.NetLayer) {
		p.Err = ErrHdrMissing("TCP")
		return
	}
	data = data[len(p.NetLayer):]
	// TCP header
	if len(data) < 20 {
		p.Err = ErrHdrLength("TCP")
		return
	}
	dOf := int(data[12]>>4) * 4
	if dOf < 20 {
		p.Err = ErrHdrInvalid("TCP's data offset")
		return
	}
	if len(data) < dOf {
		p.Err = ErrHdrLength("TCP opts")
		p.TransLayer = data[:20]
		return
	}
	p.TransLayer = data[:dOf]
	p.TransOptsLen = dOf - 20
	if len(data) > dOf {
		p.Payload = data[dOf:]
	}
	return
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

func pcapLinkTypeLength(lType int) (int, bool) {
	switch layers.LinkType(lType) {
	case layers.LinkTypeEthernet:
		return 14, true
	case layers.LinkTypeNull, layers.LinkTypeLoop:
		return 4, true
	case layers.LinkTypeRaw, 12, 14:
		return 0, true
	case layers.LinkTypeIPv4, layers.LinkTypeIPv6:
		// (TODO:) look out for IP encapsulation?
		return 0, true
	case layers.LinkTypeLinuxSLL:
		return 16, true
	case layers.LinkTypeFDDI:
		return 13, true
	case 226 /*DLT_IPNET*/ :
		// https://www.tcpdump.org/linktypes/LINKTYPE_IPNET.html
		return 24, true
	default:
		return 0, false
	}
}

// https://en.wikipedia.org/wiki/IPv6_packet#Extension_headers
func ipv6ExtensionHdr(b byte) bool {
	// TODO: support all extension headers
	return b == 0 || b == 43 || b == 44
}
