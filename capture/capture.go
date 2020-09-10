package capture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/buger/goreplay/size"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// Handler is a function that is used to handle packets
type Handler func(gopacket.Packet)

// PcapOptions options that can be set on a pcap capture handle,
// these options take effect on inactive pcap handles
type PcapOptions struct {
	BufferTimeout time.Duration `json:"input-raw-buffer-timeout"`
	TimestampType string        `json:"input-raw-timestamp-type"`
	BPFFilter     string        `json:"input-raw-bpf-filter"`
	BufferSize    size.Size     `json:"input-raw-buffer-size"`
	Promiscuous   bool          `json:"input-raw-promisc"`
	Monitor       bool          `json:"input-raw-monitor"`
	Snaplen       bool          `json:"input-raw-override-snaplen"`
}

// NetInterface represents network interface
type NetInterface struct {
	net.Interface
	IPs []string
}

// Listener handle traffic capture, this is its representation.
type Listener struct {
	sync.Mutex
	Transport  string       // transport layer default to tcp
	Activate   func() error // function is used to activate the engine. it must be called before reading packets
	Handles    map[string]gopacket.PacketDataSource
	Interfaces []NetInterface
	loopIndex  int
	Reading    chan bool // this channel is closed when the listener has started reading packets
	PcapOptions
	Engine        EngineType
	port          uint16 // src or/and dst port
	trackResponse bool

	host string // pcap file name or interface (name, hardware addr, index or ip address)

	quit    chan bool
	packets chan gopacket.Packet
}

// EngineType ...
type EngineType uint8

// Available engines for intercepting traffic
const (
	EnginePcap EngineType = 1 << iota
	EnginePcapFile
	EngineRawSocket
)

// Set is here so that EngineType can implement flag.Var
func (eng *EngineType) Set(v string) error {
	switch v {
	case "", "libpcap":
		*eng = EnginePcap
	case "pcap_file":
		*eng = EnginePcapFile
	case "raw_socket", "af_packet":
		*eng = EngineRawSocket
	default:
		return fmt.Errorf("invalid engine %s", v)
	}
	return nil
}

func (eng *EngineType) String() (e string) {
	switch *eng {
	case EnginePcapFile:
		e = "pcap_file"
	case EnginePcap:
		e = "libpcap"
	case EngineRawSocket:
		e = "raw_socket"
	default:
		e = ""
	}
	return e
}

// NewListener creates and initialize a new Listener. if transport or/and engine are invalid/unsupported
// is "tcp" and "pcap", are assumed. l.Engine and l.Transport can help to get the values used.
// if there is an error it will be associated with getting network interfaces
func NewListener(host string, port uint16, transport string, engine EngineType, trackResponse bool) (l *Listener, err error) {
	l = &Listener{}

	l.host = host
	l.port = port
	l.Transport = "tcp"
	if transport != "" {
		l.Transport = transport
	}
	l.Handles = make(map[string]gopacket.PacketDataSource)
	l.trackResponse = trackResponse
	l.packets = make(chan gopacket.Packet, 1000)
	l.quit = make(chan bool, 1)
	l.Reading = make(chan bool, 1)
	switch engine {
	default:
		l.Engine = EnginePcap
		l.Activate = l.activatePcap
	case EngineRawSocket:
		l.Engine = EngineRawSocket
		l.Activate = l.activateRawSocket
	case EnginePcapFile:
		l.Engine = EnginePcapFile
		l.Activate = l.activatePcapFile
		return
	}
	err = l.setInterfaces()
	if err != nil {
		return nil, err
	}
	return
}

// SetPcapOptions set pcap options for all yet to be actived pcap handles
// setting this on already activated handles will not have any effect
func (l *Listener) SetPcapOptions(opts PcapOptions) {
	l.PcapOptions = opts
}

// Listen listens for packets from the handles, and call handler on every packet received
// until the context done signal is sent or EOF on handles.
// this function should be called after activating pcap handles
func (l *Listener) Listen(ctx context.Context, handler Handler) (err error) {
	l.read()
	done := ctx.Done()
	var p gopacket.Packet
	var ok bool
	for {
		select {
		case <-done:
			l.quit <- true
			close(l.quit)
			err = ctx.Err()
			done = nil
		case p, ok = <-l.packets:
			if !ok {
				return
			}
			if p == nil {
				continue
			}
			handler(p)
		}
	}
}

// ListenBackground is like listen but can run concurrently and signal error through channel
func (l *Listener) ListenBackground(ctx context.Context, handler Handler) chan error {
	err := make(chan error, 1)
	go func() {
		defer close(err)
		if e := l.Listen(ctx, handler); err != nil {
			err <- e
		}
	}()
	return err
}

// Filter returns automatic filter applied by goreplay
// to a pcap handle of a specific interface
func (l *Listener) Filter(ifi NetInterface) (filter string) {
	// https://www.tcpdump.org/manpages/pcap-filter.7.html

	port := fmt.Sprintf("portrange 0-%d", 1<<16-1)
	if l.port != 0 {
		port = fmt.Sprintf("port %d", l.port)
	}
	dir := " dst " // direction
	if l.trackResponse {
		dir = " "
	}
	filter = fmt.Sprintf("(%s%s%s)", l.Transport, dir, port)
	if listenAll(l.host) || isDevice(l.host, ifi) {
		return
	}
	filter = fmt.Sprintf("(%s%s%s and host %s)", l.Transport, dir, port, l.host)
	return
}

// PcapDumpHandler returns a handler to write packet data in PCAP
// format, See http://wiki.wireshark.org/Development/LibpcapFileFormathandler.
// if link layer is invalid Ethernet is assumed
func PcapDumpHandler(file *os.File, link layers.LinkType, debugger func(int, ...interface{})) (handler func(packet gopacket.Packet), err error) {
	if link.String() == "" {
		link = layers.LinkTypeEthernet
	}
	w := NewWriterNanos(file)
	err = w.WriteFileHeader(64<<10, link)
	if err != nil {
		return nil, err
	}
	return func(packet gopacket.Packet) {
		err = w.WritePacket(packet.Metadata().CaptureInfo, packet.Data())
		if err != nil && debugger != nil {
			go debugger(3, err)
		}
	}, nil
}

// PcapHandle returns new pcap Handle from dev on success.
// this function should be called after setting all necessary options for this listener
func (l *Listener) PcapHandle(ifi NetInterface) (handle *pcap.Handle, err error) {
	var inactive *pcap.InactiveHandle
	inactive, err = pcap.NewInactiveHandle(ifi.Name)
	if inactive != nil && err != nil {
		defer inactive.CleanUp()
	}
	if err != nil {
		return nil, fmt.Errorf("inactive handle error: %q, interface: %q", err, ifi.Name)
	}
	if l.TimestampType != "" {
		var ts pcap.TimestampSource
		ts, err = pcap.TimestampSourceFromString(l.TimestampType)
		err = inactive.SetTimestampSource(ts)
		if err != nil {
			return nil, fmt.Errorf("%q: supported timestamps: %q, interface: %q", err, inactive.SupportedTimestamps(), ifi.Name)
		}
	}
	if l.Promiscuous {
		if err = inactive.SetPromisc(l.Promiscuous); err != nil {
			return nil, fmt.Errorf("promiscuous mode error: %q, interface: %q", err, ifi.Name)
		}
	}
	if l.Monitor {
		if err = inactive.SetRFMon(l.Monitor); err != nil && !errors.Is(err, pcap.CannotSetRFMon) {
			return nil, fmt.Errorf("monitor mode error: %q, interface: %q", err, ifi.Name)
		}
	}
	var snap int
	if l.Snaplen {
		snap = 64<<10 + 200
	} else if ifi.MTU > 0 {
		snap = ifi.MTU + 200
	}
	err = inactive.SetSnapLen(snap)
	if err != nil {
		return nil, fmt.Errorf("snapshot length error: %q, interface: %q", err, ifi.Name)
	}
	if l.BufferSize > 0 {
		err = inactive.SetBufferSize(int(l.BufferSize))
		if err != nil {
			return nil, fmt.Errorf("handle buffer size error: %q, interface: %q", err, ifi.Name)
		}
	}
	if l.BufferTimeout == 0 {
		l.BufferTimeout = pcap.BlockForever
	}
	err = inactive.SetTimeout(l.BufferTimeout)
	if err != nil {
		return nil, fmt.Errorf("handle buffer timeout error: %q, interface: %q", err, ifi.Name)
	}
	handle, err = inactive.Activate()
	if err != nil {
		return nil, fmt.Errorf("PCAP Activate device error: %q, interface: %q", err, ifi.Name)
	}
	if l.BPFFilter != "" {
		if l.BPFFilter[0] != '(' || l.BPFFilter[len(l.BPFFilter)-1] != ')' {
			l.BPFFilter = "(" + l.BPFFilter + ")"
		}
	} else {
		l.BPFFilter = l.Filter(ifi)
	}
	err = handle.SetBPFFilter(l.BPFFilter)
	if err != nil {
		handle.Close()
		return nil, fmt.Errorf("BPF filter error: %q%s, interface: %q", err, l.BPFFilter, ifi.Name)
	}
	return
}

// SocketHandle returns new unix ethernet handle associated with this listener settings
func (l *Listener) SocketHandle(ifi NetInterface) (handle Socket, err error) {
	handle, err = NewSocket(ifi.Interface)
	if err != nil {
		return nil, fmt.Errorf("sock raw error: %q, interface: %q", err, ifi.Name)
	}
	if err = handle.SetPromiscuous(l.Promiscuous || l.Monitor); err != nil {
		return nil, fmt.Errorf("promiscuous mode error: %q, interface: %q", err, ifi.Name)
	}
	if l.BPFFilter != "" {
		if l.BPFFilter[0] != '(' || l.BPFFilter[len(l.BPFFilter)-1] != ')' {
			l.BPFFilter = "(" + l.BPFFilter + ")"
		}
	} else {
		l.BPFFilter = l.Filter(ifi)
	}
	if err = handle.SetBPFFilter(l.BPFFilter); err != nil {
		handle.Close()
		return nil, fmt.Errorf("BPF filter error: %q%s, interface: %q", err, l.BPFFilter, ifi.Name)
	}
	handle.SetLoopbackIndex(int32(l.loopIndex))
	return
}

func (l *Listener) read() {
	l.Lock()
	defer l.Unlock()
	for key, handle := range l.Handles {
		var source *gopacket.PacketSource
		linkType := layers.LinkTypeEthernet
		if _, ok := handle.(*pcap.Handle); ok {
			linkType = handle.(*pcap.Handle).LinkType()
		}
		source = gopacket.NewPacketSource(handle, linkType)
		source.Lazy = true
		source.NoCopy = true
		ch := source.Packets()
		go func(key string) {
			defer l.closeHandles(key)
			for {
				select {
				case <-l.quit:
					return
				case p, ok := <-ch:
					if !ok {
						return
					}
					l.packets <- p
				}
			}
		}(key)
	}
	l.Reading <- true
	close(l.Reading)
}

func (l *Listener) closeHandles(key string) {
	l.Lock()
	defer l.Unlock()
	if handle, ok := l.Handles[key]; ok {
		if _, ok = handle.(Socket); ok {
			handle.(Socket).Close()
		} else {
			handle.(*pcap.Handle).Close()
		}
		delete(l.Handles, key)
		if len(l.Handles) == 0 {
			close(l.packets)
		}
	}
}

func (l *Listener) activatePcap() error {
	var e error
	var msg string
	for _, ifi := range l.Interfaces {
		var handle *pcap.Handle
		handle, e = l.PcapHandle(ifi)
		if e != nil {
			msg += ("\n" + e.Error())
			continue
		}
		l.Handles[ifi.Name] = handle
	}
	if len(l.Handles) == 0 {
		return fmt.Errorf("pcap handles error:%s", msg)
	}
	return nil
}

func (l *Listener) activateRawSocket() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("sock_raw is not stabilized on OS other than linux")
	}
	var msg string
	var e error
	for _, ifi := range l.Interfaces {
		var handle Socket
		handle, e = l.SocketHandle(ifi)
		if e != nil {
			msg += ("\n" + e.Error())
			continue
		}
		l.Handles[ifi.Name] = handle
	}
	return e
}

func (l *Listener) activatePcapFile() (err error) {
	var handle *pcap.Handle
	var e error
	if handle, e = pcap.OpenOffline(l.host); e != nil {
		return fmt.Errorf("open pcap file error: %q", e)
	}
	if l.BPFFilter != "" {
		if l.BPFFilter[0] != '(' || l.BPFFilter[len(l.BPFFilter)-1] != ')' {
			l.BPFFilter = "(" + l.BPFFilter + ")"
		}
	} else {
		addr := l.host
		l.host = ""
		l.BPFFilter = l.Filter(NetInterface{})
		l.host = addr
	}
	if e = handle.SetBPFFilter(l.BPFFilter); e != nil {
		handle.Close()
		return fmt.Errorf("BPF filter error: %q, filter: %s", e, l.BPFFilter)
	}
	l.Handles["pcap_file"] = handle
	return
}

func (l *Listener) setInterfaces() (err error) {
	var Ifis []NetInterface
	var ifis []net.Interface
	ifis, err = net.Interfaces()
	if err != nil {
		return err
	}

	for i := 0; i < len(ifis); i++ {
		if ifis[i].Flags&net.FlagLoopback != 0 {
			l.loopIndex = ifis[i].Index
		}
		if ifis[i].Flags&net.FlagUp == 0 {
			continue
		}
		var addrs []net.Addr
		addrs, err = ifis[i].Addrs()
		if err != nil {
			return err
		}
		if len(addrs) == 0 {
			continue
		}
		ifi := NetInterface{}
		ifi.Interface = ifis[i]
		ifi.IPs = make([]string, len(addrs))
		for j, addr := range addrs {
			ifi.IPs[j] = cutMask(addr)
		}
		Ifis = append(Ifis, ifi)
	}

	if listenAll(l.host) {
		l.Interfaces = Ifis
		return
	}
	found := false
	for _, ifi := range Ifis {
		if isDevice(l.host, ifi) {
			found = true
		}
		for _, ip := range ifi.IPs {
			if ip == l.host {
				found = true
				break
			}
		}
		if found {
			l.Interfaces = []NetInterface{ifi}
			return
		}
	}
	err = fmt.Errorf("can not find interface with addr, name or index %s", l.host)
	return err
}

func cutMask(addr net.Addr) string {
	mask := addr.String()
	for i, v := range mask {
		if v == '/' {
			return mask[:i]
		}
	}
	return mask
}

func isDevice(addr string, ifi NetInterface) bool {
	return addr == ifi.Name || addr == fmt.Sprintf("%d", ifi.Index) || addr == ifi.HardwareAddr.String()
}

func listenAll(addr string) bool {
	switch addr {
	case "", "0.0.0.0", "[::]", "::":
		return true
	}
	return false
}
