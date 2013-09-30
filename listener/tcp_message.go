package listener

import (
	"sort"
	"time"
)

const MSG_EXPIRE = 200 * time.Millisecond

// TCPMessage ensure that all TCP packets for given request is received, and processed in right sequence
// Its needed because all TCP message can be fragmented or re-transmitted
//
// Each TCP Packet have 2 ids: acknowledgment - message_id, and sequence - packet_id
// Message can be compiled from unique packets with same message_id which sorted by sequence
// Message is received if we didn't receive any packets for 200ms
type TCPMessage struct {
	Ack     uint32 // Message ID
	packets []*TCPPacket

	timer *time.Timer // Used for expire check

	c_packets chan *TCPPacket

	c_del_message chan *TCPMessage
}

// NewTCPMessage pointer created from a Acknowledgment number and a channel of messages readuy to be deleted
func NewTCPMessage(Ack uint32, c_del chan *TCPMessage) (msg *TCPMessage) {
	msg = &TCPMessage{Ack: Ack}

	msg.c_packets = make(chan *TCPPacket)
	msg.c_del_message = c_del // used for notifying that message completed or expired

	// Every time we receive packet we reset this timer
	msg.timer = time.AfterFunc(MSG_EXPIRE, msg.Timeout)

	go msg.listen()

	return
}

func (t *TCPMessage) listen() {
	for {
		select {
		case packet, more := <-t.c_packets:
			if more {
				t.AddPacket(packet)
			} else {
				// Stop loop if channel closed
				return
			}
		}
	}
}

// Timeout notifies message to stop listening, close channel and message ready to be sent
func (t *TCPMessage) Timeout() {
	close(t.c_packets)   // Notify to stop listen loop and close channel
	t.c_del_message <- t // Notify RAWListener that message is ready to be send to replay server
}

// Bytes sorts packets in right orders and return message content
func (t *TCPMessage) Bytes() (output []byte) {
	mk := make([]int, len(t.packets))

	i := 0
	for k, _ := range t.packets {
		mk[i] = k
		i++
	}

	sort.Ints(mk)

	for _, k := range mk {
		output = append(output, t.packets[k].Data...)
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
		Debug("Received packet with same sequence")
	} else {
		t.packets = append(t.packets, packet)
	}

	// Reset message timeout timer
	t.timer.Reset(MSG_EXPIRE)
}
