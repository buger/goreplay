package listener

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"strings"
)

// Simple TCP packet parser
//
// Packet structure: http://en.wikipedia.org/wiki/Transmission_Control_Protocol
type TCPPacket struct {
	packet []byte

	buf *bytes.Buffer

	source_port uint16
	dest_port   uint16

	sequence        uint32
	asknowledgement uint32

	doff_reserved uint16
	tcph_length   uint16

	f_ns  bool
	f_crw bool
	f_ece bool
	f_urg bool
	f_ask bool
	f_psh bool
	f_rst bool
	f_syn bool
	f_fin bool

	window_size uint16
	checksum    uint16

	data []byte
}

func NewTCPPacket(b []byte) (p *TCPPacket) {
	buf := bytes.NewBuffer(b)
	p = &TCPPacket{packet: b, buf: buf}
	p.Parse()

	return p
}

// Helper for binary.Read
func (t *TCPPacket) read(data interface{}) {
	binary.Read(t.buf, binary.BigEndian, data)
}

// Inspired by: https://gist.github.com/clicube/4978853
func (t *TCPPacket) Parse() {
	t.read(&t.source_port)
	t.read(&t.dest_port)

	t.read(&t.sequence)
	t.read(&t.asknowledgement)
	t.read(&t.doff_reserved)

	t.tcph_length = t.doff_reserved >> 12 * 4

	t.f_ns = (t.doff_reserved & 256) != 0
	t.f_crw = (t.doff_reserved & 128) != 0
	t.f_ece = (t.doff_reserved & 64) != 0
	t.f_urg = (t.doff_reserved & 32) != 0
	t.f_ask = (t.doff_reserved & 16) != 0
	t.f_psh = (t.doff_reserved & 8) != 0
	t.f_rst = (t.doff_reserved & 4) != 0
	t.f_syn = (t.doff_reserved & 2) != 0
	t.f_fin = (t.doff_reserved & 1) != 0

	t.read(&t.window_size)
	t.read(&t.checksum)

	t.data = t.packet[t.tcph_length:]
}

func (t *TCPPacket) String() string {
	return strings.Join([]string{
		"Source port: " + strconv.Itoa(int(t.source_port)),
		"Dest port:" + strconv.Itoa(int(t.dest_port)),
		"Sequence:" + strconv.Itoa(int(t.sequence)),
		"Acknowledgement:" + strconv.Itoa(int(t.asknowledgement)),
		"Header len:" + strconv.Itoa(int(t.tcph_length)),

		"Flag ns:" + strconv.FormatBool(t.f_ns),
		"Flag crw:" + strconv.FormatBool(t.f_crw),
		"Flag ece:" + strconv.FormatBool(t.f_ece),
		"Flag urg:" + strconv.FormatBool(t.f_urg),
		"Flag ask:" + strconv.FormatBool(t.f_ask),
		"Flag psh:" + strconv.FormatBool(t.f_psh),
		"Flag rst:" + strconv.FormatBool(t.f_rst),
		"Flag syn:" + strconv.FormatBool(t.f_syn),
		"Flag fin:" + strconv.FormatBool(t.f_fin),

		"Window size:" + strconv.Itoa(int(t.window_size)),
		"Checksum:" + strconv.Itoa(int(t.checksum)),

		"Data:" + string(t.data),
	}, "\n")
}
