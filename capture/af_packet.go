// +build !linux

package capture

import (
	"fmt"
	"time"

	"github.com/google/gopacket"
)

func newAfpacketHandle(device string, snaplen int, block_size int, num_blocks int,
	useVLAN bool, timeout time.Duration) (*afpacketHandle, error) {
	return nil, fmt.Errorf("Not implemented")
}

func afpacketComputeSize(targetSizeMb int, snaplen int, pageSize int) (
	frameSize int, blockSize int, numBlocks int, err error) {
	return 0, 0, 0, fmt.Errorf("Not implemented")
}

type afpacketHandle struct{}

// ZeroCopyReadPacketData satisfies ZeroCopyPacketDataSource interface
func (h *afpacketHandle) ZeroCopyReadPacketData() (data []byte, ci gopacket.CaptureInfo, err error) {
	return nil, gopacket.CaptureInfo{}, fmt.Errorf("Not implemented")
}

// SetBPFFilter translates a BPF filter string into BPF RawInstruction and applies them.
func (h *afpacketHandle) SetBPFFilter(filter string, snaplen int) (err error) {
	return fmt.Errorf("Not implemented")
}
