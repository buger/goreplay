package rawSocket

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"github.com/buger/gor/proto"
	"log"
	"strconv"
	"time"
)

var _ = log.Println

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
}

// NewTCPMessage pointer created from a Acknowledgment number and a channel of messages readuy to be deleted
func NewTCPMessage(Seq, Ack uint32, IsIncoming bool) (msg *TCPMessage) {
	msg = &TCPMessage{Seq: Seq, Ack: Ack, IsIncoming: IsIncoming}
	msg.Start = time.Now()

	return
}

// Bytes return message content
func (t *TCPMessage) Bytes() (output []byte) {
	for _, p := range t.packets {
		output = append(output, p.Data...)
	}

	return output
}

// Size returns total body size
func (t *TCPMessage) BodySize() (size int) {
	if len(t.packets) == 0 {
		return 0
	}

	size += len(proto.Body(t.packets[0].Data))

	for _, p := range t.packets[1:] {
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
	packetFound := false

	for _, pkt := range t.packets {
		if packet.Seq == pkt.Seq {
			packetFound = true
			break
		}
	}

	if !packetFound {
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
		}

		if t.IsIncoming {
			t.End = time.Now()
		} else {
			t.End = time.Now().Add(time.Millisecond)
		}

		if packet.OrigAck != 0 {
			t.DataAck = packet.OrigAck
		}
	}
}

// Check if there is missing packet
func (t *TCPMessage) isSeqMissing() bool {
	if len(t.packets) == 1 {
		return false
	}

	for i, p := range t.packets {
		// If final packet
		if len(t.packets) == i+1 {
			return false
		}
		np := t.packets[i+1]

		if np.Seq != p.Seq+uint32(len(p.Data)) {
			return true
		}
	}

	return false
}

var EmptyLine = []byte("\r\n\r\n")
var ChunkEnd = []byte("0\r\n\r\n")

func (t *TCPMessage) isHeadersReceived() bool {
	for _, p := range t.packets {
		if bytes.LastIndex(p.Data, EmptyLine) != -1 {
			return true
		}
	}

	return false
}

// isMultipart returns true if message contains from multiple tcp packets
func (t *TCPMessage) IsFinished() bool {
	payload := t.packets[0].Data

	if len(payload) < 4 {
		return true
	}

	m := payload[:4]

	if t.IsIncoming {
		// If one GET, OPTIONS, or HEAD request
		if bytes.Equal(m, []byte("GET ")) || bytes.Equal(m, []byte("OPTI")) || bytes.Equal(m, []byte("HEAD")) {
			if !t.isSeqMissing() && t.isHeadersReceived() {
				return true
			} else {
				return false
			}
		} else {
			// Sometimes header comes after the body :(
			if bytes.Equal(m, []byte("POST")) || bytes.Equal(m, []byte("PUT ")) || bytes.Equal(m, []byte("PATC")) {

				if t.isHeadersReceived() {
					if length := proto.Header(payload, []byte("Content-Length")); len(length) > 0 {
						l, _ := strconv.Atoi(string(length))

						// If content-length equal current body length
						if l > 0 && l == t.BodySize() {
							return true
						}
					}
				}
			}
		}
	} else {
		// Request not found
		// Can be because response came first or request request was just missing
		if t.AssocMessage == nil {
			return false
		}

		if !bytes.Equal(m, []byte("HTTP")) {
			return false
		}

		if length := proto.Header(payload, []byte("Content-Length")); len(length) > 0 {
			if length[0] == '0' {
				return true
			}

			l, _ := strconv.Atoi(string(length))

			// If content-length equal current body length
			if l > 0 && l == t.BodySize() {
				return true
			}
		} else {
			if enc := proto.Header(payload, []byte("Transfer-Encoding")); len(enc) == 0 {
				return true
			} else {
				if len(t.packets) > 1 && bytes.LastIndex(t.packets[len(t.packets)-1].Data, ChunkEnd) != -1 {
					return true
				}
			}
		}
	}

	return false
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

// UpdateResponseAck should be called after packet is added
func (t *TCPMessage) UpdateResponseAck() uint32 {
	lastPacket := t.packets[len(t.packets)-1]
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

func (t *TCPMessage) ID() tcpID {
	return t.packets[0].ID
}
