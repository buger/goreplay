package listener

import (
	"sort"
	"time"
)

// TCPMessage ensure that all TCP packets for given request is received, and processed in right sequence
// Its needed because all TCP message can be fragmented or re-transmitted
//
// Each TCP Packet have 2 ids: asknowledgement - message_id, and sequence - packet_id
// Message can be compiled from unique packets with same message_id which sorted by sequence
// Message is received if we did't receive any packets for 200ms OR if we received packet with "fin" flag
type TCPMessage struct {
	ask     uint32             // Message ID
	packets map[int]*TCPPacket // map[packet.sequence]*TCPPacket
	updated int64              // time of last packet
}

func NewTCPMessage(ask uint32) (msg *TCPMessage) {
	msg = &TCPMessage{}
	msg.packets = make(map[int]*TCPPacket)
	msg.updated = time.Now().UnixNano()
	msg.ask = ask
	return
}

// Sort packets in right orders and return message content
func (t *TCPMessage) Bytes() (output []byte) {
	mk := make([]int, len(t.packets))

	i := 0
	for k, _ := range t.packets {
		mk[i] = k
		i++
	}

	sort.Ints(mk)

	for _, k := range mk {
		output = append(output, t.packets[k].data...)
	}

	return
}

// Add packet to the message
func (t *TCPMessage) AddPacket(packet *TCPPacket) {
	seq := int(packet.sequence)

	if _, ok := t.packets[seq]; !ok {
		t.packets[seq] = packet
	} else {
		Debug("Received packet with same sequence")
	}

	t.updated = time.Now().UnixNano()
}

// TCP message is complete if we not received any packets for 200ms since last packet
func (t *TCPMessage) Complete() bool {
	ns := time.Now().UnixNano()
	return (ns - t.updated) > int64(200*time.Millisecond)
}
