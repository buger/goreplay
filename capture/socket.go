package capture

import (
	"time"

	"github.com/google/gopacket"
)

// Socket is any interface that defines the behaviors of Socket
type Socket interface {
	ReadPacketData() ([]byte, gopacket.CaptureInfo, error)
	WritePacketData([]byte) error
	SetBPFFilter(string) error
	SetPromiscuous(bool) error
	SetSnapLen(int) error
	GetSnapLen() int
	SetTimeout(time.Duration) error
	SetLoopbackIndex(i int32)
	Close() error
}
