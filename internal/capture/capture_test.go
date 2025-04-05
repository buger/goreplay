package capture

import (
	"sync"
	"testing"

	"github.com/buger/goreplay/internal/tcp"
	"github.com/google/gopacket/pcap"
	"github.com/stretchr/testify/assert"
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

func TestListener_Filter(t *testing.T) {
	type fields struct {
		Mutex      sync.Mutex
		config     PcapOptions
		Activate   func() error
		Handles    map[string]packetHandle
		Interfaces []pcap.Interface
		loopIndex  int
		Reading    chan bool
		messages   chan *tcp.Message
		ports      []uint16
		host       string
		closeDone  chan struct{}
		quit       chan struct{}
		closed     bool
	}
	type args struct {
		ifi   pcap.Interface
		hosts []string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantFilter string
	}{
		{
			name: "ProcessFilter",
			fields: fields{
				config: PcapOptions{
					TrackResponse: true,
					VLAN:          true,
					VLANVIDs:      []int{11, 12},
					ProcessFilter: func(filter string, config PcapOptions, portsFilter func(transport string, direction string, ports []uint16) string, ports []uint16, hostsFilter func(direction string, hosts []string) string, hosts []string) string {
						// TODO: do we want "(( dst portrange 0-65535) or ( src portrange 0-65535))"
						assert.Equal(t, "( dst portrange 0-65535) or ( src portrange 0-65535)", filter)
						return "(dst portrange 0-2)"
					},
				},
			},
			wantFilter: "vlan 12 and vlan 11 and (dst portrange 0-2)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Listener{
				Mutex:      tt.fields.Mutex,
				config:     tt.fields.config,
				Activate:   tt.fields.Activate,
				Handles:    tt.fields.Handles,
				Interfaces: tt.fields.Interfaces,
				loopIndex:  tt.fields.loopIndex,
				Reading:    tt.fields.Reading,
				messages:   tt.fields.messages,
				ports:      tt.fields.ports,
				host:       tt.fields.host,
				closeDone:  tt.fields.closeDone,
				quit:       tt.fields.quit,
				closed:     tt.fields.closed,
			}
			gotFilter := l.Filter(tt.args.ifi, tt.args.hosts...)
			assert.Equal(t, tt.wantFilter, gotFilter)
		})
	}
}
