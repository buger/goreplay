// +build !linux

package capture

import (
	"errors"

	"github.com/google/gopacket/pcap"
)

// NewSocket returns new M'maped sock_raw on packet version 2.
func NewSocket(_ pcap.Interface) (Socket, error) {
	return nil, errors.New("afpacket socket is only available on linux")
}
