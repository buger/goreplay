package rawSocket

import (
	"bytes"
	"encoding/binary"
	_ "log"
	"testing"
)

func buildPacket(isIncoming bool, Ack, Seq uint32, Data []byte) (packet *TCPPacket) {
	var srcPort, destPort uint16

	// For tests `listening` port is 0
	if isIncoming {
		srcPort = 1
	} else {
		destPort = 1
	}

	buf := make([]byte, 16)
	binary.BigEndian.PutUint16(buf[2:4], destPort)
	binary.BigEndian.PutUint16(buf[0:2], srcPort)
	binary.BigEndian.PutUint32(buf[4:8], Seq)
	binary.BigEndian.PutUint32(buf[8:12], Ack)
	buf[12] = 64
	buf = append(buf, Data...)

	packet = ParseTCPPacket([]byte("123"), buf)

	return packet
}

func buildMessage(p *TCPPacket) *TCPMessage {
	isIncoming := false
	if p.SrcPort == 1 {
		isIncoming = true
	}

	m := NewTCPMessage(p.Seq, p.Ack, isIncoming)
	m.AddPacket(p)

	return m
}

func TestTCPMessagePacketsOrder(t *testing.T) {
	msg := buildMessage(buildPacket(true, 1, 1, []byte("a")))
	msg.AddPacket(buildPacket(true, 1, 2, []byte("b")))

	if !bytes.Equal(msg.Bytes(), []byte("ab")) {
		t.Error("Should contatenate packets in right order")
	}

	// When first packet have wrong order (Seq)
	msg = buildMessage(buildPacket(true, 1, 2, []byte("b")))
	msg.AddPacket(buildPacket(true, 1, 1, []byte("a")))

	if !bytes.Equal(msg.Bytes(), []byte("ab")) {
		t.Error("Should contatenate packets in right order")
	}

	// Should ignore packets with same sequence
	msg = buildMessage(buildPacket(true, 1, 1, []byte("a")))
	msg.AddPacket(buildPacket(true, 1, 1, []byte("a")))

	if !bytes.Equal(msg.Bytes(), []byte("a")) {
		t.Error("Should ignore packet with same Seq")
	}
}

func TestTCPMessageSize(t *testing.T) {
	msg := buildMessage(buildPacket(true, 1, 1, []byte("POST / HTTP/1.1\r\nContent-Length: 2\r\n\r\na")))
	msg.AddPacket(buildPacket(true, 1, 2, []byte("b")))

	if msg.BodySize() != 2 {
		t.Error("Should count only body", msg.BodySize())
	}

	if msg.Size() != 40 {
		t.Error("Should count all sizes", msg.Size())
	}
}

func TestTCPMessageIsFinished(t *testing.T) {
	methodsWithoutBodies := []string{"GET", "OPTIONS", "HEAD"}

	for _, m := range methodsWithoutBodies {
		msg := buildMessage(buildPacket(true, 1, 1, []byte(m+" / HTTP/1.1\r\n\r\n")))

		if !msg.IsFinished() {
			t.Error(m, " request should be finished")
		}
	}

	methodsWithBodies := []string{"POST", "PUT", "PATCH"}

	for _, m := range methodsWithBodies {
		msg := buildMessage(buildPacket(true, 1, 1, []byte(m+" / HTTP/1.1\r\nContent-Length: 1\r\n\r\na")))

		if !msg.IsFinished() {
			t.Error(m, " should be finished as body length == content length")
		}

		msg = buildMessage(buildPacket(true, 1, 1, []byte(m+" / HTTP/1.1\r\nContent-Length: 2\r\n\r\na")))

		if msg.IsFinished() {
			t.Error(m, " should not be finished as body length != content length")
		}
	}

	msg := buildMessage(buildPacket(true, 1, 1, []byte("UNKNOWN / HTTP/1.1\r\n\r\n")))
	if msg.IsFinished() {
		t.Error("non http or wrong methods considered as not finished")
	}

	// Responses
	msg = buildMessage(buildPacket(false, 1, 1, []byte("HTTP/1.1 200 OK\r\n\r\n")))
	msg.AssocMessage = &TCPMessage{}
	if !msg.IsFinished() {
		t.Error("Should mark simple response as finished")
	}

	msg = buildMessage(buildPacket(false, 1, 1, []byte("HTTP/1.1 200 OK\r\n\r\n")))
	msg.AssocMessage = nil
	if msg.IsFinished() {
		t.Error("Should not mark responses without associated requests")
	}

	msg = buildMessage(buildPacket(false, 1, 1, []byte("HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n")))
	msg.AssocMessage = &TCPMessage{}

	if msg.IsFinished() {
		t.Error("Should mark chunked response as non finished")
	}

	msg = buildMessage(buildPacket(false, 1, 1, []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")))
	msg.AssocMessage = &TCPMessage{}

	if !msg.IsFinished() {
		t.Error("Should mark Content-Length: 0 respones as finished")
	}

	msg = buildMessage(buildPacket(false, 1, 1, []byte("HTTP/1.1 200 OK\r\nContent-Length: 1\r\n\r\na")))
	msg.AssocMessage = &TCPMessage{}

	if !msg.IsFinished() {
		t.Error("Should mark valid Content-Length respones as finished")
	}

	msg = buildMessage(buildPacket(false, 1, 1, []byte("HTTP/1.1 200 OK\r\nContent-Length: 10\r\n\r\na")))
	msg.AssocMessage = &TCPMessage{}

	if msg.IsFinished() {
		t.Error("Should not mark not valid Content-Length respones as finished")
	}
}

func TestTCPMessageIsSeqMissing(t *testing.T) {
	p1 := buildPacket(false, 1, 1, []byte("HTTP/1.1 200 OK\r\n"))
	p2 := buildPacket(false, 1, p1.Seq+uint32(len(p1.Data)), []byte("Content-Length: 10\r\n\r\n"))
	p3 := buildPacket(false, 1, p2.Seq+uint32(len(p2.Data)), []byte("a"))

	msg := buildMessage(p1)
	if msg.isSeqMissing() {
		t.Error("Should be complete if have only 1 packet")
	}

	msg.AddPacket(p3)
	if !msg.isSeqMissing() {
		t.Error("Should be incomplete because missing middle component")
	}

	msg.AddPacket(p2)
	if msg.isSeqMissing() {
		t.Error("Should be complete once missing packet added")
	}
}

func TestTCPMessageIsHeadersReceived(t *testing.T) {
	p1 := buildPacket(false, 1, 1, []byte("HTTP/1.1 200 OK\r\n"))
	p2 := buildPacket(false, 1, p1.Seq+uint32(len(p1.Data)), []byte("Content-Length: 10\r\n\r\n"))

	msg := buildMessage(p1)
	if msg.isHeadersReceived() {
		t.Error("Should be complete if have only 1 packet")
	}

	msg.AddPacket(p2)
	if !msg.isHeadersReceived() {
		t.Error("Should found double new line: headers received")
	}
}
