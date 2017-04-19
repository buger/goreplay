package rawSocket

import (
	"bytes"
	"encoding/binary"
	_ "log"
	"testing"
	"time"
)

func buildPacket(isIncoming bool, Ack, Seq uint32, Data []byte, timestamp time.Time) (packet *TCPPacket) {
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

	packet = ParseTCPPacket([]byte("123"), buf, timestamp)

	return packet
}

func buildMessage(p *TCPPacket) *TCPMessage {
	isIncoming := false
	if p.SrcPort == 1 {
		isIncoming = true
	}

	m := NewTCPMessage(p.Seq, p.Ack, isIncoming, p.timestamp)
	m.AddPacket(p)

	return m
}

func TestTCPMessagePacketsOrder(t *testing.T) {
	msg := buildMessage(buildPacket(true, 1, 1, []byte("a"), time.Now()))
	msg.AddPacket(buildPacket(true, 1, 2, []byte("b"), time.Now()))

	if !bytes.Equal(msg.Bytes(), []byte("ab")) {
		t.Error("Should contatenate packets in right order")
	}

	// When first packet have wrong order (Seq)
	msg = buildMessage(buildPacket(true, 1, 2, []byte("b"), time.Now()))
	msg.AddPacket(buildPacket(true, 1, 1, []byte("a"), time.Now()))

	if !bytes.Equal(msg.Bytes(), []byte("ab")) {
		t.Error("Should contatenate packets in right order")
	}

	// Should ignore packets with same sequence
	msg = buildMessage(buildPacket(true, 1, 1, []byte("a"), time.Now()))
	msg.AddPacket(buildPacket(true, 1, 1, []byte("a"), time.Now()))

	if !bytes.Equal(msg.Bytes(), []byte("a")) {
		t.Error("Should ignore packet with same Seq")
	}
}

func TestTCPMessageSize(t *testing.T) {
	msg := buildMessage(buildPacket(true, 1, 1, []byte("POST / HTTP/1.1\r\nContent-Length: 2\r\n\r\na"), time.Now()))
	msg.AddPacket(buildPacket(true, 1, 2, []byte("b"), time.Now()))

	if msg.BodySize() != 2 {
		t.Error("Should count only body", msg.BodySize())
	}

	if msg.Size() != 40 {
		t.Error("Should count all sizes", msg.Size())
	}
}

func TestTCPMessageIsComplete(t *testing.T) {
	testCases := []struct {
		direction         bool
		payload           string
		assocMessage      bool
		expectedCompleted bool
	}{
		{true, "GET / HTTP/1.1\r\n\r\n", false, true},
		{true, "HEAD / HTTP/1.1\r\n\r\n", false, true},
		{false, "HTTP/1.1 200 OK\r\n\r\n", true, true},
		{true, "POST / HTTP/1.1\r\nContent-Length: 1\r\n\r\na", false, true},
		{true, "PUT / HTTP/1.1\r\nContent-Length: 1\r\n\r\na", false, true},
		{false, "HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n", true, true},
		{false, "HTTP/1.1 200 OK\r\nContent-Length: 1\r\n\r\na", true, true},
		{false, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n", true, true},

		// chunked not finished
		{false, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n", true, false},

		// content-length != actual length
		{true, "POST / HTTP/1.1\r\nContent-Length: 2\r\n\r\na", false, false},
		{false, "HTTP/1.1 200 OK\r\nContent-Length: 10\r\n\r\na", true, false},
		// non-valid http request
		{true, "UNKNOWN asd HTTP/1.1\r\n\r\n", false, false},

		// response without associated request
		{false, "HTTP/1.1 200 OK\r\n\r\n", false, false},
	}

	for _, tc := range testCases {
		msg := buildMessage(buildPacket(tc.direction, 1, 1, []byte(tc.payload), time.Now()))
		if tc.assocMessage {
			msg.AssocMessage = &TCPMessage{}
		}
		msg.checkIfComplete()

		if msg.complete != tc.expectedCompleted {
			t.Errorf("Payload %s: Expected %t, got %t.", tc.payload, tc.expectedCompleted, msg.complete)
		}
	}
}

func TestTCPMessageIsSeqMissing(t *testing.T) {
	p1 := buildPacket(false, 1, 1, []byte("HTTP/1.1 200 OK\r\n"), time.Now())
	p2 := buildPacket(false, 1, p1.Seq+uint32(len(p1.Data)), []byte("Content-Length: 10\r\n\r\n"), time.Now())
	p3 := buildPacket(false, 1, p2.Seq+uint32(len(p2.Data)), []byte("a"), time.Now())

	msg := buildMessage(p1)
	if msg.seqMissing {
		t.Error("Should be complete if have only 1 packet")
	}

	msg.AddPacket(p3)
	if !msg.seqMissing {
		t.Error("Should be incomplete because missing middle component")
	}

	msg.AddPacket(p2)
	if msg.seqMissing {
		t.Error("Should be complete once missing packet added")
	}
}

func TestTCPMessageIsHeadersReceived(t *testing.T) {
	p1 := buildPacket(false, 1, 1, []byte("HTTP/1.1 200 OK\r\n\r\n"), time.Now())
	p2 := buildPacket(false, 1, p1.Seq+uint32(len(p1.Data)), []byte("Content-Length: 10\r\n\r\n"), time.Now())

	msg := buildMessage(p1)
	if msg.headerPacket == -1 {
		t.Error("Should be complete if have only 1 packet", msg.headerPacket)
	}

	msg.AddPacket(p2)
	if msg.headerPacket == -1 {
		t.Error("Should found double new line: headers received")
	}

	msg = buildMessage(buildPacket(true, 1, 1, []byte("GET / HTTP/1.1\r\nContent-Length: 1\r\n"), time.Now()))
	if msg.headerPacket != -1 {
		t.Error("Should not find headers end")
	}
}

func TestTCPMessageMethodType(t *testing.T) {
	testCases := []struct {
		direction          bool
		payload            string
		expectedMethodType httpMethodType
	}{
		{true, "GET / HTTP/1.1\r\n\r\n", httpMethodWithoutBody},
		{true, "GET * HTTP/1.1\r\n\r\n", httpMethodWithoutBody},
		{true, "UNKNOWN / HTTP/1.1\r\n\r\n", httpMethodWithoutBody},
		{true, "GET http://example.com HTTP/1.1\r\n\r\n", httpMethodWithoutBody},
		{true, "POST / HTTP/1.1\r\n\r\n", httpMethodWithBody},
		{true, "PUT / HTTP/1.1\r\n\r\n", httpMethodWithBody},
		{true, "GET zxc HTTP/1.1\r\n\r\n", httpMethodNotFound},
		{true, "GET / HTTP\r\n\r\n", httpMethodNotFound},
		{true, "VERYLONGMETHOD / HTTP/1.1\r\n\r\n", httpMethodNotFound},
		{false, "HTTP/1.1 200 OK\r\n\r\n", httpMethodWithBody},
		{false, "HTTP /1.1 200 OK\r\n\r\n", httpMethodNotFound},
	}

	for _, tc := range testCases {
		msg := buildMessage(buildPacket(tc.direction, 1, 1, []byte(tc.payload), time.Now()))

		if msg.methodType != tc.expectedMethodType {
			t.Errorf("Expected %d, got %d", tc.expectedMethodType, msg.methodType)
		}
	}
}

func TestTCPMessageBodyType(t *testing.T) {
	testCases := []struct {
		direction        bool
		payload          string
		expectedBodyType httpBodyType
	}{
		{true, "GET / HTTP/1.1\r\n\r\n", httpBodyEmpty},
		{true, "POST / HTTP/1.1\r\n\r\n", httpBodyEmpty},
		{true, "POST / HTTP/1.1\r\nUser-Agent: zxc\r\n\r\n", httpBodyEmpty},
		{false, "HTTP/1.1 200 OK\r\n\r\n", httpBodyEmpty},
		{true, "POST / HTTP/1.1\r\nContent-Length: 2\r\n\r\nab", httpBodyContentLength},
		{false, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nab", httpBodyContentLength},
		{true, "POST / HTTP/1.1\r\nTransfer-Encoding: chunked\r\n\r\n2\r\nab\r\n0\r\n\r\n", httpBodyChunked},
		{false, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n2\r\nab\r\n0\r\n\r\n", httpBodyChunked},
	}

	for _, tc := range testCases {
		msg := buildMessage(buildPacket(tc.direction, 1, 1, []byte(tc.payload), time.Now()))

		if msg.bodyType != tc.expectedBodyType {
			t.Errorf("Expected %d, got %d", tc.expectedBodyType, msg.bodyType)
		}
	}
}

func TestTCPMessageBodySize(t *testing.T) {
	testCases := []struct {
		direction    bool
		payloads     []string
		expectedSize int
	}{
		{true, []string{"GET / HTTP/1.1\r\n\r\n"}, 0},
		{true, []string{"POST / HTTP/1.1\r\nContent-Length: 2\r\n\r\nab"}, 2},
		{true, []string{"GET / HTTP/1.1\r\n", "Content-Length: 2\r\n\r\nab"}, 2},
		{true, []string{"GET / HTTP/1.1\r\n", "Content-Length: 2\r\n\r\n", "ab"}, 2},
	}

	for _, tc := range testCases {
		msg := buildMessage(buildPacket(tc.direction, 1, 1, []byte(tc.payloads[0]), time.Now()))

		if len(tc.payloads) > 1 {
			for _, p := range tc.payloads[1:] {
				seq := uint32(1 + msg.Size())
				msg.AddPacket(buildPacket(tc.direction, 1, seq, []byte(p), time.Now()))
			}
		}

		if msg.BodySize() != tc.expectedSize {
			t.Errorf("Expected %d, got %d", tc.expectedSize, msg.BodySize())
		}
	}
}

func TestTcpMessageStart(t *testing.T) {
	start := time.Now().Add(-1 * time.Second)

	msg := buildMessage(buildPacket(true, 1, 2, []byte("b"), time.Now()))
	msg.AddPacket(buildPacket(true, 1, 1, []byte("POST / HTTP/1.1\r\nContent-Length: 2\r\n\r\na"), start))

	if msg.Start != start {
		t.Error("Message timestamp should be equal to the lowest related packet timestamp", start, msg.Start)
	}
}
