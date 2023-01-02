package capture

import (
	"testing"
)

func TestSetInterfaces(t *testing.T) {
	listener := &Listener{
		loopIndex: 99999,
	}
	listener.setInterfaces()

	for _, nic := range listener.Interfaces {
		if (len(nic.Addresses)) == 0 {
			t.Errorf("nic %s was captured with 0 addresses", nic.Name)
		}
	}

	if listener.loopIndex == 99999 {
		t.Errorf("loopback nic index was not found")
	}
}
