// +build !linux

package capture

import (
	"errors"
	"net"
)

// NewSocket returns new M'maped sock_raw on packet version 2.
func NewSocket(_ net.Interface) (Socket, error) {
	return nil, errors.New("afpacket socket is only available on linux")
}
