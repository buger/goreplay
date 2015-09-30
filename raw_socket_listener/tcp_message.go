package rawSocket

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"github.com/buger/gor/proto"
	"log"
	"strconv"
	"time"
)

// TCPMessage ensure that all TCP packets for given request is received, and processed in right sequence
// Its needed because all TCP message can be fragmented or re-transmitted
//
// Each TCP Packet have 2 ids: acknowledgment - message_id, and sequence - packet_id
// Message can be compiled from unique packets with same message_id which sorted by sequence
// Message is received if we didn't receive any packets for 2000ms
type TCPMessage struct {
	ID           string // Message ID
	Ack          uint32
	ResponseAck  uint32
	RequestStart time.Time
	RequestAck   uint32
	Start        time.Time
	End          time.Time
	IsIncoming   bool

	packets []*TCPPacket

	delChan chan *TCPMessage
}

// NewTCPMessage pointer created from a Acknowledgment number and a channel of messages readuy to be deleted
func NewTCPMessage(ID string, Ack uint32, IsIncoming bool) (msg *TCPMessage) {
	msg = &TCPMessage{ID: ID, Ack: Ack, IsIncoming: IsIncoming}
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

// Size returns total size of message
func (t *TCPMessage) Size() (size int) {
	size += len(proto.Body(t.packets[0].Data))

	for _, p := range t.packets[1:] {
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

	if packetFound {
		log.Println("Received packet with same sequence")
	} else {
		// Packets not always captured in same Seq order, and sometimes we need to prepend
		if len(t.packets) == 0 || packet.Seq > t.packets[len(t.packets)-1].Seq {
			t.packets = append(t.packets, packet)
		} else {
			t.packets = append([]*TCPPacket{packet}, t.packets...)
		}

		t.End = time.Now()
	}
}

// isMultipart returns true if message contains from multiple tcp packets
func (t *TCPMessage) IsMultipart() bool {
	if len(t.packets) > 1 {
		return true
	}

	payload := t.packets[0].Data

	if len(payload) < 4 {
		return false
	}

	m := payload[:4]

	if t.IsIncoming {
		// If one GET, OPTIONS, or HEAD request
		if bytes.Equal(m, []byte("GET ")) || bytes.Equal(m, []byte("OPTI")) || bytes.Equal(m, []byte("HEAD")) {
			return false
		} else {
			// Sometimes header comes after the body :(
			if bytes.Equal(m, []byte("POST")) || bytes.Equal(m, []byte("PUT ")) || bytes.Equal(m, []byte("PATC")) {
				if length := proto.Header(payload, []byte("Content-Length")); len(length) > 0 {
					l, _ := strconv.Atoi(string(length))

					// If content-length equal current body length
					if l > 0 && l == t.Size() {
						return false
					}
				}
			}
		}
	} else {
		if length := proto.Header(payload, []byte("Content-Length")); len(length) > 0 {
			if length[0] == '0' {
				return false
			}

			l, _ := strconv.Atoi(string(length))

			// If content-length equal current body length
			if l > 0 && l == t.Size() {
				return false
			}
		}
	}

	return true
}

func (t *TCPMessage) UUID() []byte {
	var key []byte

	if t.IsIncoming {
		key = strconv.AppendInt(key, t.Start.UnixNano(), 10)
		key = strconv.AppendUint(key, uint64(t.Ack), 10)
	} else {
		key = strconv.AppendInt(key, t.RequestStart.UnixNano(), 10)
		key = strconv.AppendUint(key, uint64(t.RequestAck), 10)
	}

	uuid := make([]byte, 40)
	sha := sha1.Sum(key)
	hex.Encode(uuid, sha[:20])

	return uuid
}
