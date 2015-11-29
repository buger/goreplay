package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
)

// A pcap-based input, with steam parsing borrowed from gopacket httpassembly example.
// Once gopacket's re-assembly provides a reader, net/http takes care of reading messages,
// at which point implementation is closer to input-http.
type PcapInput struct {
	iface            string
	port             string
	captureResponses bool
	out              chan CapturedMsg
	times            map[uint64]int64
}

type CapturedMsg struct {
	id      uint64
	timing  int64
	kind    byte
	payload []byte
}

func NewPcapInput(listen string, captureResponses bool) *PcapInput {
	parts := strings.Split(listen, ":")
	if len(parts) != 2 {
		log.Fatal("must supply iface:port for pcap listener", parts)
	}

	p := &PcapInput{parts[0], parts[1], captureResponses, make(chan CapturedMsg, 10000), make(map[uint64]int64)}
	go p.startCapture()
	return p
}

func (p *PcapInput) recordMsg(kind byte, id uint64, timing int64, payload []byte) {
	select {
	case p.out <- CapturedMsg{id, timing, kind, payload}:
	default: // drop messages if they aren't consumed.
	}
}

func (p *PcapInput) Read(data []byte) (int, error) {
	msg := <-p.out

	idBytes := []byte(strconv.FormatUint(msg.id, 10))
	timingBytes := []byte(strconv.FormatInt(msg.timing, 10))

	headerLen := 1 + 1 + len(idBytes) + 1 + len(timingBytes)

	// TODO: maybe check len(data) <= totalLen to avoid out-of-bounds?
	totalLen := headerLen + 1 + len(msg.payload)

	// write the message type and sep
	data[0] = msg.kind
	data[1] = ' '

	// writes the id and sep
	copy(data[2:], idBytes)
	data[2+len(idBytes)] = ' '

	// writes the timing and end-of-header newline
	copy(data[2+len(idBytes)+1:], timingBytes)
	data[headerLen] = '\n'

	// and now the actual message
	copy(data[headerLen+1:], msg.payload)

	return totalLen, nil
}

func (p *PcapInput) New(net, transport gopacket.Flow) tcpassembly.Stream {
	r := tcpreader.NewReaderStream()

	// a->b and b->a have same FastHash, transport.FastHash is how we correlate req and resp streams.
	streamId := transport.FastHash()

	// TODO: don't rely on String() for comp.
	incoming := transport.Dst().String() == p.port

	if incoming {
		go p.readIncomingStream(&r, streamId)
	} else if p.captureResponses {
		go p.readOutgoingStream(&r, streamId)
	} else {
		go p.discardStream(&r)
	}

	return &r
}

func (p *PcapInput) discardStream(r *tcpreader.ReaderStream) {
	tcpreader.DiscardBytesToFirstError(r)
}

func (p *PcapInput) readIncomingStream(r *tcpreader.ReaderStream, streamId uint64) {
	reader := bufio.NewReader(r)
	count := uint64(0)
	for {
		req, err := http.ReadRequest(reader)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		} else if err != nil {
			log.Println("Error reading request:", err)
			tcpreader.DiscardBytesToFirstError(r)
		} else {
			count++
			id := streamId + count
			out, _ := httputil.DumpRequest(req, true)
			req.Body.Close()
			t := time.Now().UnixNano()
			p.times[id] = t
			p.recordMsg('1', id, t, out)
		}
	}
}

func (p *PcapInput) readOutgoingStream(r *tcpreader.ReaderStream, streamId uint64) {
	reader := bufio.NewReader(r)
	count := uint64(0)
	for {
		resp, err := http.ReadResponse(reader, nil)

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		} else if err != nil {
			log.Println(err)
			tcpreader.DiscardBytesToFirstError(r)
		} else {
			count++
			id := streamId + count
			out, _ := httputil.DumpResponse(resp, true)
			resp.Body.Close()
			t := time.Now().UnixNano()
			if st, ok := p.times[id]; ok {
				rtt := t - st
				p.recordMsg('2', id, rtt, out)
			} else {
				log.Println("Response to missing req", id, t, string(out))
			}
		}
	}
}

// borrowed mostly from https://github.com/google/gopacket/tree/master/examples/httpassembly
func (p *PcapInput) startCapture() {
	// Set up pcap packet capture
	log.Printf("Starting pcap capture on interface %q:%s", p.iface, p.port)
	handle, err := pcap.OpenLive(p.iface, int32(1600), true, pcap.BlockForever)
	if err != nil {
		log.Fatal(err)
	}

	if err := handle.SetBPFFilter(fmt.Sprintf("tcp and port %s", p.port)); err != nil {
		log.Fatal(err)
	}

	// Set up assembly
	streamPool := tcpassembly.NewStreamPool(p)
	assembler := tcpassembly.NewAssembler(streamPool)

	log.Println("reading in packets")
	// Read in packets, pass to assembler.
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packets := packetSource.Packets()
	ticker := time.Tick(time.Minute)
	for {
		select {
		case packet := <-packets:
			// A nil packet indicates the end of a pcap file.
			if packet == nil {
				return
			}
			if packet.NetworkLayer() == nil || packet.TransportLayer() == nil || packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
				log.Println("Unusable packet")
				continue
			}
			tcp := packet.TransportLayer().(*layers.TCP)
			assembler.AssembleWithTimestamp(packet.NetworkLayer().NetworkFlow(), tcp, packet.Metadata().Timestamp)

		case <-ticker:
			// Every minute, flush connections that haven't seen activity in the past 2 minutes.
			assembler.FlushOlderThan(time.Now().Add(time.Minute * -2))
		}
	}
}

func (i *PcapInput) String() string {
	return fmt.Sprintf("pcap input: %s:%s", i.iface, i.port)
}
