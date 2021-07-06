// +build linux

package capture

import (
	"flag"
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/bpf"

	_ "github.com/google/gopacket/layers"
)

var (
	iface      = flag.String("i", "any", "Interface to read from")
	cpuprofile = flag.String("cpuprofile", "", "If non-empty, write CPU profile here")
	snaplen    = flag.Int("s", 0, "Snaplen, if <= 0, use 65535")
	bufferSize = flag.Int("b", 8, "Interface buffersize (MB)")
	filter     = flag.String("f", "port not 22", "BPF filter")
	count      = flag.Int64("c", -1, "If >= 0, # of packets to capture before returning")
	verbose    = flag.Int64("log_every", 1, "Write a log every X packets")
	addVLAN    = flag.Bool("add_vlan", false, "If true, add VLAN header")
)

type afpacketHandle struct {
	TPacket *afpacket.TPacket
}

func newAfpacketHandle(device string, snaplen int, block_size int, num_blocks int,
	useVLAN bool, timeout time.Duration) (*afpacketHandle, error) {

	h := &afpacketHandle{}
	var err error

	if device == "any" {
		h.TPacket, err = afpacket.NewTPacket(
			afpacket.OptFrameSize(snaplen),
			afpacket.OptBlockSize(block_size),
			afpacket.OptNumBlocks(num_blocks),
			afpacket.OptAddVLANHeader(useVLAN),
			afpacket.OptPollTimeout(timeout),
			afpacket.SocketRaw,
			afpacket.TPacketVersion3)
	} else {
		h.TPacket, err = afpacket.NewTPacket(
			afpacket.OptInterface(device),
			afpacket.OptFrameSize(snaplen),
			afpacket.OptBlockSize(block_size),
			afpacket.OptNumBlocks(num_blocks),
			afpacket.OptAddVLANHeader(useVLAN),
			afpacket.OptPollTimeout(timeout),
			afpacket.SocketRaw,
			afpacket.TPacketVersion3)
	}
	return h, err
}

// ReadPacketData satisfies PacketDataSource interface
func (h *afpacketHandle) ReadPacketData() (data []byte, ci gopacket.CaptureInfo, err error) {
	return h.TPacket.ReadPacketData()
}

// SetBPFFilter translates a BPF filter string into BPF RawInstruction and applies them.
func (h *afpacketHandle) SetBPFFilter(filter string, snaplen int) (err error) {
	pcapBPF, err := pcap.CompileBPFFilter(layers.LinkTypeEthernet, snaplen, filter)
	if err != nil {
		return err
	}
	bpfIns := []bpf.RawInstruction{}
	for _, ins := range pcapBPF {
		bpfIns2 := bpf.RawInstruction{
			Op: ins.Code,
			Jt: ins.Jt,
			Jf: ins.Jf,
			K:  ins.K,
		}
		bpfIns = append(bpfIns, bpfIns2)
	}
	if h.TPacket.SetBPF(bpfIns); err != nil {
		return err
	}
	return h.TPacket.SetBPF(bpfIns)
}

// LinkType returns ethernet link type.
func (h *afpacketHandle) LinkType() layers.LinkType {
	return layers.LinkTypeEthernet
}

// Close will close afpacket source.
func (h *afpacketHandle) Close() {
	h.TPacket.Close()
}

// SocketStats prints received, dropped, queue-freeze packet stats.
func (h *afpacketHandle) SocketStats() (as afpacket.SocketStats, asv afpacket.SocketStatsV3, err error) {
	return h.TPacket.SocketStats()
}

// afpacketComputeSize computes the block_size and the num_blocks in such a way that the
// allocated mmap buffer is close to but smaller than target_size_mb.
// The restriction is that the block_size must be divisible by both the
// frame size and page size.
func afpacketComputeSize(targetSizeMb int, snaplen int, pageSize int) (
	frameSize int, blockSize int, numBlocks int, err error) {

	if snaplen < pageSize {
		frameSize = pageSize / (pageSize / snaplen)
	} else {
		frameSize = (snaplen/pageSize + 1) * pageSize
	}

	// 128 is the default from the gopacket library so just use that
	blockSize = frameSize * 128
	numBlocks = (targetSizeMb * 1024 * 1024) / blockSize

	if numBlocks == 0 {
		return 0, 0, 0, fmt.Errorf("Interface buffersize is too small")
	}

	return frameSize, blockSize, numBlocks, nil
}
