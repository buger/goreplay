package capture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/buger/goreplay/size"
	"github.com/buger/goreplay/tcp"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"golang.org/x/sys/unix"
)

// PacketHandler is a function that is used to handle packets
type PacketHandler func(*tcp.Packet)

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

// Listener handle traffic capture, this is its representation.
type Listener struct {
	sync.Mutex
	Transport  string       // transport layer default to tcp
	Activate   func() error // function is used to activate the engine. it must be called before reading packets
	Handles    map[string]gopacket.ZeroCopyPacketDataSource
	Interfaces []net.Interface
	loopIndex  int
	Reading    chan bool // this channel is closed when the listener has started reading packets
	PcapOptions
	Engine        EngineType
	port          uint16 // src or/and dst port
	trackResponse bool

	host string // pcap file name or interface (name, hardware addr, index or ip address)

	closeDone chan struct{}
	quit      chan struct{}
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
	l.Handles = make(map[string]gopacket.ZeroCopyPacketDataSource)
	l.trackResponse = trackResponse
	l.closeDone = make(chan struct{})
	l.quit = make(chan struct{})
	l.Reading = make(chan bool)
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
// until the context done signal is sent or there is unrecoverable error on all handles.
// this function must be called after activating pcap handles
func (l *Listener) Listen(ctx context.Context, handler PacketHandler) (err error) {
	l.read(handler)
	done := ctx.Done()
	select {
	case <-done:
		close(l.quit) // signal close on all handles
		<-l.closeDone // wait all handles to be closed
		err = ctx.Err()
	case <-l.closeDone: // all handles closed voluntarily
	}
	return
}

// ListenBackground is like listen but can run concurrently and signal error through channel
func (l *Listener) ListenBackground(ctx context.Context, handler PacketHandler) chan error {
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
func (l *Listener) Filter(ifi net.Interface) (filter string) {
	// https://www.tcpdump.org/manpages/pcap-filter.7.html

	port := fmt.Sprintf("portrange 0-%d", 1<<16-1)
	if l.port != 0 {
		port = fmt.Sprintf("port %d", l.port)
	}
	filter = fmt.Sprintf("%s dst %s", l.Transport, port)
	if l.trackResponse {
		filter = fmt.Sprintf("%s %s", l.Transport, port)
	}

	if listenAll(l.host) || isDevice(l.host, ifi) {
		return "(" + filter + ")"
	}
	filter = fmt.Sprintf("(host %s and (%s))", l.host, filter)

	log.Println("BPF filter: " + filter)
	return
}

// PcapDumpHandler returns a handler to write packet data in PCAP
// format, See http://wiki.wireshark.org/Development/LibpcapFileFormathandler.
// if link layer is invalid Ethernet is assumed
// func PcapDumpHandler(file *os.File, link layers.LinkType) (handler func(packet *tcp.Packet) error, err error) {
// 	if link.String() == "" {
// 		link = layers.LinkTypeEthernet
// 	}
// 	w := NewWriterNanos(file)
// 	err = w.WriteFileHeader(64<<10, link)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return func(packet *tcp.Packet) error {
// 		return w.WritePacket(*packet.Info, packet.Data)
// 	}, nil
// }

// PcapHandle returns new pcap Handle from dev on success.
// this function should be called after setting all necessary options for this listener
func (l *Listener) PcapHandle(ifi net.Interface) (handle *pcap.Handle, err error) {
	var inactive *pcap.InactiveHandle
	inactive, err = pcap.NewInactiveHandle(ifi.Name)
	if err != nil {
		return nil, fmt.Errorf("inactive handle error: %q, interface: %q", err, ifi.Name)
	}
	defer inactive.CleanUp()
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
	fmt.Println("Interface:", ifi.Name, ". BPF Filter:", l.BPFFilter)
	err = handle.SetBPFFilter(l.BPFFilter)
	if err != nil {
		handle.Close()
		return nil, fmt.Errorf("BPF filter error: %q%s, interface: %q", err, l.BPFFilter, ifi.Name)
	}
	return
}

// SocketHandle returns new unix ethernet handle associated with this listener settings
func (l *Listener) SocketHandle(ifi net.Interface) (handle Socket, err error) {
	handle, err = NewSocket(ifi)
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
	fmt.Println("BPF Filter: ", l.BPFFilter)
	if err = handle.SetBPFFilter(l.BPFFilter); err != nil {
		handle.Close()
		return nil, fmt.Errorf("BPF filter error: %q%s, interface: %q", err, l.BPFFilter, ifi.Name)
	}
	handle.SetLoopbackIndex(int32(l.loopIndex))
	return
}

func (l *Listener) read(handler PacketHandler) {
	l.Lock()
	defer l.Unlock()
	for key, handle := range l.Handles {
		go func(key string, hndl gopacket.ZeroCopyPacketDataSource) {
			defer l.closeHandles(key)
			linkSize := 14
			linkType := int(layers.LinkTypeEthernet)
			if _, ok := hndl.(*pcap.Handle); ok {
				linkType = int(hndl.(*pcap.Handle).LinkType())
				linkSize, ok = pcapLinkTypeLength(linkType)
				if !ok {
					if os.Getenv("GORDEBUG") != "0" {
						log.Printf("can not identify link type of an interface '%s'\n", key)
					}
					return // can't find the linktype size
				}
			}

			for {
				select {
				case <-l.quit:
					return
				default:
					data, ci, err := hndl.ZeroCopyReadPacketData()
					if err == nil {
						pckt, err := tcp.ParsePacket(data, linkType, linkSize, &ci)
						if err == nil {
							handler(pckt)
						}
						continue
					}
					if enext, ok := err.(pcap.NextError); ok && enext == pcap.NextErrorTimeoutExpired {
						continue
					}
					if eno, ok := err.(unix.Errno); ok && eno.Temporary() {
						continue
					}
					if enet, ok := err.(*net.OpError); ok && (enet.Temporary() || enet.Timeout()) {
						continue
					}
					if err == io.EOF || err == io.ErrClosedPipe {
						log.Printf("stopped reading from %s interface with error %s\n", key, err)
						return
					}

					log.Printf("stopped reading from %s interface with error %s\n", key, err)
					return
				}
			}
		}(key, handle)
	}
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
			close(l.closeDone)
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
	if len(l.Handles) == 0 {
		return fmt.Errorf("raw socket handles error:%s", msg)
	}
	return nil
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
		l.BPFFilter = l.Filter(net.Interface{})
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
	var ifis []net.Interface
	ifis, err = net.Interfaces()
	if err != nil {
		return
	}
	for i := range ifis {
		if ifis[i].Flags&net.FlagLoopback != 0 {
			l.loopIndex = ifis[i].Index
		}
		if ifis[i].Flags&net.FlagUp == 0 {
			continue
		}
		if isDevice(l.host, ifis[i]) {
			l.Interfaces = []net.Interface{ifis[i]}
			return
		}
		addrs, e := ifis[i].Addrs()
		if e != nil {
			// don't give up on a failure from a single interface
			continue
		}
		for _, addr := range addrs {
			if cutMask(addr) == l.host {
				l.Interfaces = []net.Interface{ifis[i]}
				return
			}
		}
	}
	l.Interfaces = ifis
	return
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

func isDevice(addr string, ifi net.Interface) bool {
	return addr == ifi.Name || addr == fmt.Sprintf("%d", ifi.Index) || (addr != "" && addr == ifi.HardwareAddr.String())
}

func listenAll(addr string) bool {
	switch addr {
	case "", "0.0.0.0", "[::]", "::":
		return true
	}
	return false
}

func pcapLinkTypeLength(lType int) (int, bool) {
	switch layers.LinkType(lType) {
	case layers.LinkTypeEthernet:
		return 14, true
	case layers.LinkTypeNull, layers.LinkTypeLoop:
		return 4, true
	case layers.LinkTypeRaw, 12, 14:
		return 0, true
	case layers.LinkTypeIPv4, layers.LinkTypeIPv6:
		// (TODO:) look out for IP encapsulation?
		return 0, true
	case layers.LinkTypeLinuxSLL:
		return 16, true
	case layers.LinkTypeFDDI:
		return 13, true
	case 226 /*DLT_IPNET*/ :
		// https://www.tcpdump.org/linktypes/LINKTYPE_IPNET.html
		return 24, true
	default:
		return 0, false
	}
}
