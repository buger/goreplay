package capture

import (
	"context"
	"encoding/binary"
	"io/ioutil"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

var LoopBack = func() net.Interface {
	ifis, _ := net.Interfaces()
	for _, v := range ifis {
		if v.Flags&net.FlagLoopback != 0 {
			return v
		}
	}
	return ifis[0]
}()

func TestSetInterfaces(t *testing.T) {
	l := &Listener{}
	l.host = "127.0.0.1"
	l.setInterfaces()
	if len(l.Interfaces) != 1 {
		t.Error("expected a single interface")
	}
	l.host = LoopBack.HardwareAddr.String()
	l.setInterfaces()
	if l.Interfaces[0].Name != LoopBack.Name && len(l.Interfaces) != 1 {
		t.Error("interface should be loop back interface")
	}
	l.host = ""
	l.setInterfaces()
	if len(l.Interfaces) < 1 {
		t.Error("should get all interfaces")
	}
}

func TestBPFFilter(t *testing.T) {
	l := &Listener{}
	l.host = "127.0.0.1"
	l.Transport = "tcp"
	l.setInterfaces()
	filter := l.Filter(l.Interfaces[0])
	if filter != "(tcp dst portrange 0-65535 and host 127.0.0.1)" {
		t.Error("wrong filter", filter)
	}
	l.port = 8000
	l.trackResponse = true
	filter = l.Filter(l.Interfaces[0])
	if filter != "(tcp port 8000 and host 127.0.0.1)" {
		t.Error("wrong filter")
	}
}

func TestPcapDump(t *testing.T) {
	f, err := ioutil.TempFile("", "pcap_file")
	if err != nil {
		t.Error(err)
	}
	h, _ := PcapDumpHandler(f, layers.LinkTypeLoop)
	packets := Packets(1, 5, 5, 4)
	for i := 0; i < len(packets); i++ {
		if i == 1 {
			// change dst port
			binary.BigEndian.PutUint16(packets[i].TransLayer[2:], 8001)
		}
		if i == 4 {
			packets[i].Info.CaptureLength = 40
		}
		h(packets[i])
	}
	name := f.Name()
	f.Close()
	testPcapDumpEngine(name, t)
}

func testPcapDumpEngine(f string, t *testing.T) {
	defer os.Remove(f)
	l, err := NewListener(f, 8000, "", EnginePcapFile, true)
	err = l.Activate()
	if err != nil {
		t.Errorf("expected error to be nil, got %q", err)
		return
	}
	pckts := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = l.Listen(ctx, func(packet *Packet) {
		if packet.Info.CaptureLength != 57 {
			t.Errorf("expected packet length to be %d, got %d", 57, packet.Info.CaptureLength)
		}
		pckts++
	})

	if err != nil {
		t.Errorf("expected error to be nil, got %q", err)
	}
	if pckts != 3 {
		t.Errorf("expected %d packets, got %d packets", 3, pckts)
	}
}

func TestPcapHandler(t *testing.T) {
	l, err := NewListener(LoopBack.Name, 8000, "", EnginePcap, true)
	if err != nil {
		t.Errorf("expected error to be nil, got %v", err)
		return
	}
	err = l.Activate()
	if err != nil {
		t.Errorf("expected error to be nil, got %v", err)
		return
	}
	defer l.Handles[LoopBack.Name].(*pcap.Handle).Close()
	if err != nil {
		t.Errorf("expected error to be nil, got %v", err)
		return
	}
	for i := 0; i < 5; i++ {
		_, _ = net.Dial("tcp", "127.0.0.1:8000")
	}
	sts, _ := l.Handles[LoopBack.Name].(*pcap.Handle).Stats()
	if sts.PacketsReceived < 5 {
		t.Errorf("expected >=5 packets got %d", sts.PacketsReceived)
	}
}

func TestSocketHandler(t *testing.T) {
	l, err := NewListener(LoopBack.Name, 8000, "", EngineRawSocket, true)
	err = l.Activate()
	if err != nil {
		return
	}
	defer l.Handles[LoopBack.Name].(*SockRaw).Close()
	if err != nil {
		t.Errorf("expected error to be nil, got %v", err)
		return
	}
	for i := 0; i < 5; i++ {
		_, _ = net.Dial("tcp", "127.0.0.1:8000")
	}
	sts, _ := l.Handles[LoopBack.Name].(*SockRaw).Stats()
	if sts.Packets < 5 {
		t.Errorf("expected >=5 packets got %d", sts.Packets)
	}
}

func BenchmarkPcapDump(b *testing.B) {
	f, err := ioutil.TempFile("", "pcap_file")
	if err != nil {
		b.Error(err)
		return
	}
	now := time.Now()
	defer os.Remove(f.Name())
	h, _ := PcapDumpHandler(f, layers.LinkTypeLoop)
	packets := Packets(1, b.N, 5, 4)
	for i := 0; i < len(packets); i++ {
		h(packets[i])
	}
	f.Close()
	b.Logf("%d packets in %s", b.N, time.Since(now))
}

func BenchmarkPcapFile(b *testing.B) {
	f, err := ioutil.TempFile("", "pcap_file")
	if err != nil {
		b.Error(err)
		return
	}
	defer os.Remove(f.Name())
	h, _ := PcapDumpHandler(f, layers.LinkTypeLoop)
	packets := Packets(1, b.N, 5, 4)
	for i := 0; i < len(packets); i++ {
		h(packets[i])
	}
	name := f.Name()
	f.Close()
	b.ResetTimer()
	var l *Listener
	l, err = NewListener(name, 8000, "", EnginePcapFile, true)
	if err != nil {
		b.Error(err)
		return
	}
	err = l.Activate()
	if err != nil {
		b.Error(err)
		return
	}
	now := time.Now()
	pckts := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err = l.Listen(ctx, func(packet *Packet) {
		if packet.Info.CaptureLength != 49 {
			b.Errorf("expected packet length to be %d, got %d", 49, packet.Info.CaptureLength)
		}
		pckts++
	}); err != nil {
		b.Error(err)
	}
	b.Logf("%d/%d packets in %s", pckts, b.N, time.Since(now))
}

// used to benchmark sock engine
var buf [1024]byte

func init() {
	for i := 0; i < len(buf); i++ {
		buf[i] = 0xff
	}
}

func handler(n, counter *int32) Handler {
	return func(p *Packet) {
		nn := int32(len(p.Data))
		atomic.AddInt32(n, nn)
		atomic.AddInt32(counter, 1)
	}
}

func BenchmarkPcap(b *testing.B) {
	var err error
	n := new(int32)
	counter := new(int32)
	l, err := NewListener(LoopBack.Name, 8000, "", EnginePcap, false)
	if err != nil {
		b.Error(err)
		return
	}
	l.PcapOptions.BPFFilter = "udp dst port 8000 and host 127.0.0.1"
	err = l.Activate()
	if err != nil {
		b.Error(err)
		return
	}
	errCh := l.ListenBackground(context.Background(), handler(n, counter))
	select {
	case <-l.Reading:
	case err = <-errCh:
		b.Error(err)
		return
	}
	var conn net.Conn
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		conn, err = net.Dial("udp", "127.0.0.1:8000")
		if err != nil {
			b.Error(err)
			return
		}
		b.StartTimer()
		_, err = conn.Write(buf[:])
		if err != nil {
			b.Error(err)
			return
		}
	}
	b.ReportMetric(float64(atomic.LoadInt32(n)), "buf")
	b.ReportMetric(float64(atomic.LoadInt32(counter)), "packets")
}

func BenchmarkRawSocket(b *testing.B) {
	var err error
	n := new(int32)
	counter := new(int32)
	l, err := NewListener(LoopBack.Name, 8000, "", EngineRawSocket, false)
	if err != nil {
		b.Error(err)
		return
	}
	l.PcapOptions.BPFFilter = "udp dst port 8000 and host 127.0.0.1"
	err = l.Activate()
	if err != nil {
		b.Error(err)
		return
	}
	errCh := l.ListenBackground(context.Background(), handler(n, counter))
	select {
	case <-l.Reading:
	case err = <-errCh:
		b.Error(err)
		return
	}
	var conn net.Conn
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		conn, err = net.Dial("udp", "127.0.0.1:8000")
		if err != nil {
			b.Error(err)
			return
		}
		b.StartTimer()
		_, err = conn.Write(buf[:])
		if err != nil {
			b.Error(err)
			return
		}
	}
	b.ReportMetric(float64(atomic.LoadInt32(n)), "buf")
	b.ReportMetric(float64(atomic.LoadInt32(counter)), "packets")
}
