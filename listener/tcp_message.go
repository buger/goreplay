package listener

import (
	"github.com/akrennmair/gopcap"
	"sort"
	"time"
)

// TCPMessage ensure that all TCP packets for given request is received, and processed in right sequence
// Its needed because all TCP message can be fragmented or re-transmitted
//
// Each TCP Packet have 2 ids: acknowledgement - message_id, and sequence - packet_id
// Message can be compiled from unique packets with same message_id which sorted by sequence
// Message is received if we did't receive any packets for 200ms OR if we received packet with "fin" flag
type TCPMessage struct {
	ack     uint32               // Message ID
	packets map[int]*pcap.Packet // map[packet.sequence]*TCPPacket
	updated int64                // time of last packet
}

func NewTCPMessage(ack uint32) (msg *TCPMessage) {
	msg = &TCPMessage{}
	msg.packets = make(map[int]*pcap.Packet)
	msg.updated = time.Now().UnixNano()
	msg.ack = ack
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
		output = append(output, t.packets[k].Payload...)
	}

	return
}

// Add packet to the message
func (t *TCPMessage) AddPacket(packet *pcap.Packet) {
	seq := int(packet.Headers[1].(*pcap.Tcphdr).Seq)

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
