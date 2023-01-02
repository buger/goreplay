package capture

import (
	"errors"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"net"
	"time"
)

const VxLanPacketSize = 1526 //vxlan 8 B + ethernet II 1518 B

type vxlanHandle struct {
	connection    *net.UDPConn
	packetChannel chan gopacket.Packet
	vnis          []int
}

func newVXLANHandler(port int, vnis []int) (*vxlanHandle, error) {
	if port == 0 {
		port = 4789
	}

	addr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP("0.0.0.0"),
	}

	vxlanHandle := &vxlanHandle{}
	con, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return nil, fmt.Errorf(err.Error())
	}
	vxlanHandle.connection = con
	vxlanHandle.packetChannel = make(chan gopacket.Packet, 1000)
	vxlanHandle.vnis = vnis
	go vxlanHandle.reader()

	return vxlanHandle, nil
}

func (v *vxlanHandle) reader() {
	for {
		inputBytes := make([]byte, VxLanPacketSize)
		length, _, err := v.connection.ReadFromUDP(inputBytes)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		packet := gopacket.NewPacket(inputBytes[:length], layers.LayerTypeVXLAN, gopacket.NoCopy)
		ci := packet.Metadata()
		ci.Timestamp = time.Now()
		ci.CaptureLength = length
		ci.Length = length

		if len(v.vnis) > 0 && !v.vniIsAllowed(packet) {
			continue
		}

		v.packetChannel <- packet
	}
}

func (v *vxlanHandle) vniIsAllowed(packet gopacket.Packet) bool {
	defaultState := false
	if layer := packet.Layer(layers.LayerTypeVXLAN); layer != nil {
		vxlan, _ := layer.(*layers.VXLAN)
		for _, vn := range v.vnis {
			if vn > 0 && int(vxlan.VNI) == vn {
				return true
			}

			if vn < 0 {
				if int(vxlan.VNI) == -vn {
					return false
				}
				defaultState = true
			}
		}
	}
	return defaultState
}

func (v *vxlanHandle) ReadPacketData() ([]byte, gopacket.CaptureInfo, error) {
	packet := <-v.packetChannel
	layer := packet.Layer(layers.LayerTypeVXLAN)
	bytes := layer.LayerPayload()

	return bytes, packet.Metadata().CaptureInfo, nil
}

func (v *vxlanHandle) Close() error {
	if v.connection != nil {
		return v.connection.Close()
	}
	return nil
}
