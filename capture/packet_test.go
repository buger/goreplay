package capture

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func generateHeader4(seq uint32, length uint16) []byte {
	hdr := make([]byte, 4+24+24, 4+24+24)
	binary.BigEndian.PutUint32(hdr, uint32(layers.ProtocolFamilyIPv4))

	ip := hdr[4:]
	ip[0] = 4<<4 | 6
	binary.BigEndian.PutUint16(ip[2:4], length+24+24)
	ip[9] = uint8(layers.IPProtocolTCP)
	copy(ip[12:16], []byte{127, 0, 0, 1})
	copy(ip[16:], []byte{127, 0, 0, 1})

	// set tcp header
	tcp := ip[24:]
	tcp[12] = 6 << 4
	binary.BigEndian.PutUint16(tcp, 5535)
	binary.BigEndian.PutUint16(tcp[2:], 8000)
	binary.BigEndian.PutUint32(tcp[4:], seq)
	return hdr
}

func generateHeader6(seq uint32, length uint16) []byte {
	hdr := make([]byte, 4+40+32+24, 4+40+32+24)
	binary.BigEndian.PutUint32(hdr, uint32(layers.ProtocolFamilyIPv6Linux))

	ip := hdr[4:]
	ip[0] = 6 << 4
	binary.BigEndian.PutUint16(ip[4:], length+32+24)
	var ipAddr [16]byte
	ipAddr[15] = 0x01
	copy(ip[8:], ip[:])
	copy(ip[24:], ip[:])
	copy(ip[40:], []byte{
		// net-layer (IPv6-Opts)
		0x2b, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		uint8(layers.IPProtocolTCP), 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	})

	// set tcp header
	tcp := ip[40+32:]
	tcp[12] = 6 << 4
	binary.BigEndian.PutUint16(tcp, 5535)
	binary.BigEndian.PutUint16(tcp[2:], 8000)
	binary.BigEndian.PutUint32(tcp[4:], seq)
	return hdr
}

func Packets(start uint32, _len int, length uint16, version byte) []*Packet {
	var packets = make([]*Packet, _len)
	for i := start; i < start+uint32(_len); i++ {
		var h []byte
		if version == 4 {
			h = generateHeader4(i, length)
		} else {
			h = generateHeader6(i, length)
		}
		d := append(h, make([]byte, int(length))...)
		ci := &gopacket.CaptureInfo{Length: len(d), CaptureLength: len(d), Timestamp: time.Now()}
		packets[i-start] = NewPacket(d, int(layers.LinkTypeLoop), 4, ci)
	}
	return packets
}

func TestIPv4Packet(t *testing.T) {
	pckt := packet(append(generateHeader4(1024, 10), make([]byte, 10)...))
	if pckt.Err != nil {
		t.Error(pckt)
		return
	}
	if err := packet(pckt.Data[:2]).Err; !errors.Is(err, ErrHdrLength("Link")) {
		t.Errorf("should fail with %q, got %q", ErrHdrLength("Link"), err)
		return
	}
	if err := packet(pckt.Data[:20]).Err; !errors.Is(err, ErrHdrLength("IPv4")) {
		t.Errorf("should fail with %q, got %q", ErrHdrLength("IPv4"), err)
		return
	}
	if err := packet(pckt.Data[:27]).Err; !errors.Is(err, ErrHdrLength("IPv4 opts")) {
		t.Errorf("should fail with %q, got %q", ErrHdrLength("IPv4 opts"), err)
		return
	}
	if err := packet(pckt.Data[:40]).Err; !errors.Is(err, ErrHdrLength("TCP")) {
		t.Errorf("should fail with %q, got %q", ErrHdrLength("TCP opts"), err)
		return
	}
	if err := packet(pckt.Data[:50]).Err; !errors.Is(err, ErrHdrLength("TCP opts")) {
		t.Errorf("should fail with %q, got %q", ErrHdrLength("TCP opts"), err)
		return
	}
	pckt.TransLayer[12] = 0x10
	if err := packet(pckt.Data[:50]).Err; !errors.Is(err, ErrHdrInvalid("TCP's data offset")) {
		t.Errorf("should fail with %q, got %q", ErrHdrInvalid("TCP's data offset"), err)
		return
	}
	pckt.TransLayer[12] = 0x60
	if err := packet(pckt.Data[:28]).Err; !errors.Is(err, ErrHdrMissing("TCP")) {
		t.Errorf("should fail with %q, got %q", ErrHdrMissing("TCP"), err)
		return
	}
	pckt.NetLayer[9] = 0x02
	if err := packet(pckt.Data).Err; !errors.Is(err, ErrHdrExpected("TCP")) {
		t.Errorf("should fail with %q, got %q", ErrHdrExpected("TCP"), err)
		return
	}
	pckt.NetLayer[9] = 0x06
	pckt.NetLayer[0] = 0x44
	if err := packet(pckt.Data).Err; !errors.Is(err, ErrHdrInvalid("IPv4's IHL")) {
		t.Errorf("should fail with %q, got %q", ErrHdrInvalid("IPv4's IHL"), err)
		return
	}
	pckt.NetLayer[0] = 0x56
	if err := packet(pckt.Data).Err; !errors.Is(err, ErrHdrExpected("IPv4 or IPv6")) {
		t.Errorf("should fail with %q, got %q", ErrHdrExpected("IPv4 or IPv6"), err)
		return
	}
}

func TestIPv6Packet(t *testing.T) {
	pckt := packet(append(generateHeader6(1024, 10), make([]byte, 10)...))
	if pckt.Err != nil {
		t.Error(pckt)
		return
	}
	if err := packet(pckt.Data[:4]).Err; !errors.Is(err, ErrHdrMissing("IPv4 or IPv6")) {
		t.Errorf("should fail with %q, got %q", ErrHdrMissing("IPv4 or IPv6"), err)
		return
	}
	if err := packet(pckt.Data[:40]).Err; !errors.Is(err, ErrHdrLength("IPv6")) {
		t.Errorf("should fail with %q, got %q", ErrHdrLength("IPv6"), err)
		return
	}
	if err := packet(pckt.Data[:52]).Err; !errors.Is(err, ErrHdrLength("IPv6 opts")) {
		t.Errorf("should fail with %q, got %q", ErrHdrLength("IPv6 opts"), err)
		return
	}
	if err := packet(pckt.Data[:80]).Err; !errors.Is(err, ErrHdrLength("TCP")) {
		t.Errorf("should fail with %q, got %q", ErrHdrLength("TCP opts"), err)
		return
	}
	if err := packet(pckt.Data[:98]).Err; !errors.Is(err, ErrHdrLength("TCP opts")) {
		t.Errorf("should fail with %q, got %q", ErrHdrLength("TCP opts"), err)
		return
	}
	pckt.TransLayer[12] = 0x10
	if err := packet(pckt.Data).Err; !errors.Is(err, ErrHdrInvalid("TCP's data offset")) {
		t.Errorf("should fail with %q, got %q", ErrHdrInvalid("TCP's data offset"), err)
		return
	}
	pckt.TransLayer[12] = 0x60
	if err := packet(pckt.Data[:76]).Err; !errors.Is(err, ErrHdrMissing("TCP")) {
		t.Errorf("should fail with %q, got %q", ErrHdrMissing("TCP"), err)
		return
	}
	pckt.NetLayer[56] = 0x02
	if err := packet(pckt.Data).Err; !errors.Is(err, ErrHdrExpected("TCP")) {
		t.Errorf("should fail with %q, got %q", ErrHdrExpected("TCP"), err)
		return
	}
	pckt.NetLayer[56] = 0x06
}

func packet(data []byte) *Packet {
	return NewPacket(data, int(layers.LinkTypeLoop), 4, &gopacket.CaptureInfo{})
}

func BenchmarkNewPacketIPv4(b *testing.B) {
	data := append(generateHeader4(1204, 10), make([]byte, 10)...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := packet(data).Err; err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkNewPacketIPv6(b *testing.B) {
	data := append(generateHeader6(1024, 10), make([]byte, 10)...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := packet(data).Err; err != nil {
			b.Error(err)
		}
	}
}
