package capture

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/buger/goreplay/proto"
)

// TCPProtocol is a number to indicate type of protocol
type TCPProtocol uint8

const (
	// ProtocolHTTP ...
	ProtocolHTTP TCPProtocol = 0
	// ProtocolBinary ...
	ProtocolBinary TCPProtocol = 1
)

// TCPMessage ensure that all TCP packets for given request is received, and processed in right sequence
// Its needed because all TCP message can be fragmented or re-transmitted
//
// Each TCP Packet have 2 ids: acknowledgment - message_id, and sequence - packet_id
// Message can be compiled from unique packets with same message_id which sorted by sequence
// Message is received if we didn't receive any packets for 2000ms
type TCPMessage struct {
	Seq         uint32
	Ack         uint32
	ResponseAck uint32
	ResponseID  tcpID
	DataAck     uint32
	DataSeq     uint32

	AssocMessage *TCPMessage
	Start        time.Time
	End          time.Time
	IsIncoming   bool

	packets []*TCPPacket

	delChan chan *TCPMessage

	protocol TCPProtocol

	/* HTTP specific variables */
	methodType    httpMethodType
	bodyType      httpBodyType
	expectType    httpExpectType
	seqMissing    bool
	headerPacket  int
	contentLength int
	complete      bool
}

// NewTCPMessage pointer created from a Acknowledgment number and a channel of messages readuy to be deleted
func NewTCPMessage(Seq, Ack uint32, IsIncoming bool, protocol TCPProtocol, timestamp time.Time) (msg *TCPMessage) {
	msg = &TCPMessage{Seq: Seq, Ack: Ack, IsIncoming: IsIncoming, protocol: protocol, Start: timestamp}
	msg.Start = time.Now()

	return
}

func (t *TCPMessage) packetsData() (d [][]byte) {
	d = make([][]byte, len(t.packets))
	for i, p := range t.packets {
		d[i] = p.Data
	}

	return
}

// Bytes return message content
func (t *TCPMessage) Bytes() (output []byte) {
	for _, p := range t.packets {
		output = append(output, p.Data...)
	}

	return output
}

// BodySize returns total body size
func (t *TCPMessage) BodySize() (size int) {
	if len(t.packets) == 0 || t.headerPacket == -1 {
		return 0
	}

	size += len(proto.Body(t.packets[t.headerPacket].Data))

	for _, p := range t.packets[t.headerPacket+1:] {
		size += len(p.Data)
	}

	return
}

// Size returns total size of message
func (t *TCPMessage) Size() (size int) {
	if len(t.packets) == 0 {
		return 0
	}

	for _, p := range t.packets {
		size += len(p.Data)
	}

	return
}

// AddPacket to the message and ensure packet uniqueness
// TCP allows that packet can be re-send multiple times
func (t *TCPMessage) AddPacket(packet *TCPPacket) {
	for _, pkt := range t.packets {
		if packet.Seq == pkt.Seq {
			return
		}
	}

	// Packets not always captured in same Seq order, and sometimes we need to prepend
	if len(t.packets) == 0 || packet.Seq > t.packets[len(t.packets)-1].Seq {
		t.packets = append(t.packets, packet)
	} else if packet.Seq < t.packets[0].Seq {
		t.packets = append([]*TCPPacket{packet}, t.packets...)
		t.Seq = packet.Seq // Message Seq should indicated starting seq
	} else { // insert somewhere in the middle...
		for i, p := range t.packets {
			if packet.Seq < p.Seq {
				t.packets = append(t.packets[:i], append([]*TCPPacket{packet}, t.packets[i:]...)...)
				break
			}
		}

		if packet.OrigAck != 0 {
			t.DataAck = packet.OrigAck
		}

		if packet.timestamp.Before(t.Start) || t.Start.IsZero() {
			t.Start = packet.timestamp
		}

		if packet.timestamp.After(t.End) || t.End.IsZero() {
			t.End = packet.timestamp
		}
	}

	t.checkSeqIntegrity()

	if t.protocol == ProtocolHTTP {
		t.updateHeadersPacket()
		t.updateMethodType()
		t.updateBodyType()
		t.check100Continue()
		t.checkIfComplete()
	}
}

// Check if there is missing packet
func (t *TCPMessage) checkSeqIntegrity() {
	if len(t.packets) == 1 {
		t.seqMissing = false
	}

	offset := len(t.packets) - 1

	if t.packets[offset].IsFIN {
		offset--

		if offset < 0 {
			return
		}
	}

	for i, p := range t.packets[:offset] {
		if p.IsFIN {
			continue
		}

		// If final packet
		if len(t.packets) == i+1 {
			t.seqMissing = false
			return
		}
		np := t.packets[i+1]

		nextSeq := p.Seq + uint32(len(p.Data))

		if np.Seq != nextSeq {
			if t.protocol == ProtocolHTTP && t.expectType == httpExpect100Continue {
				if np.Seq != nextSeq+22 {
					t.seqMissing = true
					return
				}
			} else {
				t.seqMissing = true
				return
			}
		}
	}

	t.seqMissing = false
}

var bEmptyLine = []byte("\r\n\r\n")
var bBR = []byte("\r\n")

// last-chunk always is 0\r\n\r\n\. More info https://tools.ietf.org/html/rfc2616#section-3.6.1
var bChunkEnd = []byte("0\r\n\r\n")

func (t *TCPMessage) updateHeadersPacket() {
	if len(t.packets) == 1 {
		t.headerPacket = -1
	}

	if t.headerPacket != -1 {
		return
	}

	if t.seqMissing {
		return
	}

	for i, p := range t.packets {
		if len(p.Data) >= len(bEmptyLine) {
			if bytes.LastIndex(p.Data, bEmptyLine) != -1 {
				t.headerPacket = i
				return
			}
		} else if i > 0 && bytes.Equal(p.Data, bBR) {
			idx := bytes.LastIndex(t.packets[i-1].Data, bBR)
			if idx != -1 && idx == len(t.packets[i-1].Data)-len(bBR) {
				t.headerPacket = i
				return
			}
		}
	}

	return
}

// checkIfComplete returns true if all of the packets that compse the message arrived.
func (t *TCPMessage) checkIfComplete() {
	if t.seqMissing || t.headerPacket == -1 {
		// log.Println("Seq missing", t.seqMissing, t.packets)
		return
	}

	if t.methodType == httpMethodNotFound {
		// log.Println("Method missing", t.methodType, t.packets)
		return
	}

	// Responses can be emitted only if we found request
	if !t.IsIncoming && t.AssocMessage == nil {
		// log.Println("Assoc not found", t)
		return
	}

	// log.Println("Found?", t)

	switch t.bodyType {
	case httpBodyEmpty:
		t.complete = true
	case httpBodyContentLength:
		if t.contentLength == 0 || t.contentLength == t.BodySize() {
			t.complete = true
		}
	case httpBodyChunked:
		lastPacket := t.packets[len(t.packets)-1]
		if bytes.LastIndex(lastPacket.Data, bChunkEnd) != -1 {
			t.complete = true
		}
	default:
		if len(t.packets) == 0 {
			return
		}

		last := t.packets[len(t.packets)-1]
		if last.IsFIN {
			t.complete = true
		}
	}
}

type httpMethodType uint8

const (
	httpMethodNotSet   httpMethodType = 0
	httpMethodKnown    httpMethodType = 1
	httpMethodNotFound httpMethodType = 2
)

func (t *TCPMessage) updateMethodType() {
	// if there is cache
	if t.methodType != httpMethodNotSet && t.methodType != httpMethodNotFound {
		return
	}

	d := t.packets[0].Data

	// Minimum length fo request: GET / HTTP/1.1\r\n

	if len(d) < 16 {
		t.methodType = httpMethodNotFound
		return
	}

	if t.IsIncoming {
		if mIdx := bytes.IndexByte(d[:8], ' '); mIdx != -1 {
			// Check that after method we have absolute or relative path
			switch d[mIdx+1] {
			case '/', 'h', '*':
			default:
				t.methodType = httpMethodNotFound
				return
			}
		} else {
			t.methodType = httpMethodNotFound
			return
		}

		t.methodType = httpMethodKnown
	} else {
		if !bytes.Equal(d[:6], []byte("HTTP/1")) {
			t.methodType = httpMethodNotFound
			return
		}

		t.methodType = httpMethodKnown
	}
}

type httpBodyType uint8

const (
	httpBodyNotSet          httpBodyType = 0
	httpBodyEmpty           httpBodyType = 1
	httpBodyContentLength   httpBodyType = 2
	httpBodyChunked         httpBodyType = 3
	httpBodyConnectionClose httpBodyType = 4
)

func (t *TCPMessage) updateBodyType() {
	// if there is cache
	if t.bodyType != httpBodyNotSet {
		return
	}

	// Headers not received
	if t.headerPacket == -1 {
		return
	}

	var lengthB, encB, connB []byte

	proto.ParseHeaders(t.packetsData(), func(header, value []byte) bool {
		if proto.HeadersEqual(header, []byte("Content-Length")) {
			lengthB = value
			return false
		}

		if proto.HeadersEqual(header, []byte("Transfer-Encoding")) {
			encB = value
			return false
		}

		if proto.HeadersEqual(header, []byte("Connection")) {
			connB = value
		}

		return true
	})

	switch t.methodType {
	case httpMethodNotFound:
		return
	case httpMethodKnown:

		if !t.IsIncoming &&
			t.AssocMessage != nil &&
			bytes.IndexByte(t.AssocMessage.Bytes(), ' ') > -1 &&
			bytes.Equal([]byte("HEAD"), proto.Method(t.AssocMessage.Bytes())) {
			// Need to check if this is a response to a head request,
			// in which case the body has to be empty regardless.
			t.bodyType = httpBodyEmpty
			return
		}

		if len(lengthB) > 0 {
			t.contentLength, _ = strconv.Atoi(string(lengthB))
			if t.contentLength == 0 {
				t.bodyType = httpBodyEmpty
			} else {
				t.bodyType = httpBodyContentLength
			}
			return
		}

		if len(encB) > 0 {
			t.bodyType = httpBodyChunked
			return
		}

		if len(connB) > 0 && bytes.Equal(connB, []byte("close")) {
			t.bodyType = httpBodyConnectionClose
			return
		}
	}

	t.bodyType = httpBodyEmpty
}

type httpExpectType uint8

const (
	httpExpectNotSet      httpExpectType = 0
	httpExpectEmpty       httpExpectType = 1
	httpExpect100Continue httpExpectType = 2
)

var bExpectHeader = []byte("Expect")
var bExpect100Value = []byte("100-continue")

func (t *TCPMessage) check100Continue() {
	if t.expectType != httpExpectNotSet || len(t.packets[0].Data) < 25 {
		return
	}

	if t.seqMissing || t.headerPacket == -1 {
		return
	}

	last := t.packets[len(t.packets)-1]
	// reading last 4 bytes for double CRLF
	if !bytes.HasSuffix(last.Data, bEmptyLine) {
		return
	}

	var expectB []byte
	proto.ParseHeaders(t.packetsData(), func(header, value []byte) bool {
		if proto.HeadersEqual(header, bExpectHeader) {
			expectB = value
			return false
		}

		return true
	})

	if len(expectB) > 0 && bytes.Equal(bExpect100Value, expectB) {
		t.expectType = httpExpect100Continue
		return
	}

	t.expectType = httpExpectEmpty
}

func (t *TCPMessage) setAssocMessage(m *TCPMessage) {
	t.AssocMessage = m
	t.checkIfComplete()
}

// UpdateResponseAck should be called after packet is added
func (t *TCPMessage) UpdateResponseAck() uint32 {
	lastPacket := t.packets[len(t.packets)-1]
	if lastPacket.IsFIN && len(t.packets) > 1 {
		lastPacket = t.packets[len(t.packets)-2]
	}

	respAck := lastPacket.Seq + uint32(len(lastPacket.Data))

	if t.ResponseAck != respAck {
		t.ResponseAck = lastPacket.Seq + uint32(len(lastPacket.Data))

		// We swappwed src and dst port
		copy(t.ResponseID[:16], lastPacket.Addr)
		copy(t.ResponseID[16:], lastPacket.Raw[2:4]) // Src port
		copy(t.ResponseID[18:], lastPacket.Raw[0:2]) // Dest port
		binary.BigEndian.PutUint32(t.ResponseID[20:24], t.ResponseAck)
	}

	return t.ResponseAck
}

func (t *TCPMessage) UUID() []byte {
	var key []byte

	if t.IsIncoming {
		// log.Println("UUID:", t.Ack, t.Start.UnixNano())
		key = strconv.AppendInt(key, t.Start.UnixNano(), 10)
		key = strconv.AppendUint(key, uint64(t.Ack), 10)
	} else {
		// log.Println("RequestMessage:", t.AssocMessage.Ack, t.AssocMessage.Start.UnixNano())
		key = strconv.AppendInt(key, t.AssocMessage.Start.UnixNano(), 10)
		key = strconv.AppendUint(key, uint64(t.AssocMessage.Ack), 10)
	}

	uuid := make([]byte, 40)
	sha := sha1.Sum(key)
	hex.Encode(uuid, sha[:20])

	return uuid
}

func (t *TCPMessage) ID() tcpID {
	return t.packets[0].ID
}

func (t *TCPMessage) IP() net.IP {
	return net.IP(t.packets[0].Addr)
}

func (t *TCPMessage) String() string {
	return strings.Join([]string{
		"Len packets: " + strconv.Itoa(len(t.packets)),
		"Data size:" + strconv.Itoa(len(t.Bytes())),
		"Data:" + string(t.Bytes()),
	}, "\n")
}
