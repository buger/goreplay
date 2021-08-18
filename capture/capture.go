package capture

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/buger/goreplay/proto"
	"github.com/buger/goreplay/size"
	"github.com/buger/goreplay/tcp"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

var stats *expvar.Map

func init() {
	stats = expvar.NewMap("raw")
	stats.Init()
}

// PacketHandler is a function that is used to handle packets
type PacketHandler func(*tcp.Packet)

type PcapStatProvider interface {
	Stats() (*pcap.Stats, error)
}

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
	Handles    map[string]packetHandle
	Interfaces []pcap.Interface
	loopIndex  int
	Reading    chan bool // this channel is closed when the listener has started reading packets
	PcapOptions
	Engine          EngineType
	ports           []uint16 // src or/and dst ports
	trackResponse   bool
	expiry          time.Duration
	allowIncomplete bool
	messages        chan *tcp.Message
	protocol        tcp.TCPProtocol

	host string // pcap file name or interface (name, hardware addr, index or ip address)

	closeDone chan struct{}
	quit      chan struct{}
}

type packetHandle struct {
	handler gopacket.PacketDataSource
	ips     []net.IP
}

// EngineType ...
type EngineType uint8

// Available engines for intercepting traffic
const (
	EnginePcap EngineType = 1 << iota
	EnginePcapFile
	EngineRawSocket
	EngineAFPacket
)

// Set is here so that EngineType can implement flag.Var
func (eng *EngineType) Set(v string) error {
	switch v {
	case "", "libpcap":
		*eng = EnginePcap
	case "pcap_file":
		*eng = EnginePcapFile
	case "raw_socket":
		*eng = EngineRawSocket
	case "af_packet":
		*eng = EngineAFPacket
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
	case EngineAFPacket:
		e = "af_packet"
	default:
		e = ""
	}
	return e
}

// NewListener creates and initialize a new Listener. if transport or/and engine are invalid/unsupported
// is "tcp" and "pcap", are assumed. l.Engine and l.Transport can help to get the values used.
// if there is an error it will be associated with getting network interfaces
func NewListener(host string, ports []uint16, transport string, engine EngineType, protocol tcp.TCPProtocol, trackResponse bool, expiry time.Duration, allowIncomplete bool) (l *Listener, err error) {
	l = &Listener{}

	l.host = host
	if l.host == "localhost" {
		l.host = "127.0.0.1"
	}
	l.ports = ports

	l.Transport = "tcp"
	if transport != "" {
		l.Transport = transport
	}
	l.Handles = make(map[string]packetHandle)
	l.trackResponse = trackResponse
	l.closeDone = make(chan struct{})
	l.quit = make(chan struct{})
	l.Reading = make(chan bool)
	l.expiry = expiry
	l.allowIncomplete = allowIncomplete
	l.protocol = protocol
	l.messages = make(chan *tcp.Message, 10000)

	switch engine {
	default:
		l.Engine = EnginePcap
		l.Activate = l.activatePcap
	case EngineRawSocket:
		l.Engine = EngineRawSocket
		l.Activate = l.activateRawSocket
	case EngineAFPacket:
		l.Engine = EngineAFPacket
		l.Activate = l.activateAFPacket
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
func (l *Listener) Listen(ctx context.Context) (err error) {
	l.read()
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
func (l *Listener) ListenBackground(ctx context.Context) chan error {
	err := make(chan error, 1)
	go func() {
		defer close(err)
		if e := l.Listen(ctx); err != nil {
			err <- e
		}
	}()
	return err
}

// Filter returns automatic filter applied by goreplay
// to a pcap handle of a specific interface
func (l *Listener) Filter(ifi pcap.Interface) (filter string) {
	// https://www.tcpdump.org/manpages/pcap-filter.7.html

	hosts := []string{l.host}
	if listenAll(l.host) || isDevice(l.host, ifi) {
		hosts = interfaceAddresses(ifi)
	}

	filter = portsFilter(l.Transport, "dst", l.ports)

	if len(hosts) != 0 && !l.Promiscuous {
		filter = fmt.Sprintf("((%s) and (%s))", filter, hostsFilter("dst", hosts))
	} else {
		filter = fmt.Sprintf("(%s)", filter)
	}

	if l.trackResponse {
		responseFilter := portsFilter(l.Transport, "src", l.ports)

		if len(hosts) != 0 && !l.Promiscuous {
			responseFilter = fmt.Sprintf("((%s) and (%s))", responseFilter, hostsFilter("src", hosts))
		} else {
			responseFilter = fmt.Sprintf("(%s)", responseFilter)
		}

		filter = fmt.Sprintf("%s or %s", filter, responseFilter)
	}

	return
}

// PcapHandle returns new pcap Handle from dev on success.
// this function should be called after setting all necessary options for this listener
func (l *Listener) PcapHandle(ifi pcap.Interface) (handle *pcap.Handle, err error) {
	var inactive *pcap.InactiveHandle
	inactive, err = pcap.NewInactiveHandle(ifi.Name)
	if err != nil {
		return nil, fmt.Errorf("inactive handle error: %q, interface: %q", err, ifi.Name)
	}
	defer inactive.CleanUp()

	if l.TimestampType != "" && l.TimestampType != "go" {
		var ts pcap.TimestampSource
		ts, err = pcap.TimestampSourceFromString(l.TimestampType)
		fmt.Println("Setting custom Timestamp Source. Supported values: `go`, ", inactive.SupportedTimestamps())
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

	if !l.Snaplen {
		infs, _ := net.Interfaces()
		for _, i := range infs {
			if i.Name == ifi.Name {
				snap = i.MTU + 200
			}
		}
	}

	if snap == 0 {
		snap = 64<<10 + 200
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
		l.BufferTimeout = 2000 * time.Millisecond
	}
	err = inactive.SetTimeout(l.BufferTimeout)
	if err != nil {
		return nil, fmt.Errorf("handle buffer timeout error: %q, interface: %q", err, ifi.Name)
	}
	handle, err = inactive.Activate()
	if err != nil {
		return nil, fmt.Errorf("PCAP Activate device error: %q, interface: %q", err, ifi.Name)
	}

	bpfFilter := l.BPFFilter
	if bpfFilter == "" {
		bpfFilter = l.Filter(ifi)
	}
	fmt.Println("Interface:", ifi.Name, ". BPF Filter:", bpfFilter)
	err = handle.SetBPFFilter(bpfFilter)
	if err != nil {
		handle.Close()
		return nil, fmt.Errorf("BPF filter error: %q%s, interface: %q", err, bpfFilter, ifi.Name)
	}
	return
}

// SocketHandle returns new unix ethernet handle associated with this listener settings
func (l *Listener) SocketHandle(ifi pcap.Interface) (handle Socket, err error) {
	handle, err = NewSocket(ifi)
	if err != nil {
		return nil, fmt.Errorf("sock raw error: %q, interface: %q", err, ifi.Name)
	}
	if err = handle.SetPromiscuous(l.Promiscuous || l.Monitor); err != nil {
		return nil, fmt.Errorf("promiscuous mode error: %q, interface: %q", err, ifi.Name)
	}
	if l.BPFFilter == "" {
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

func http1StartHint(pckt *tcp.Packet) (isRequest, isResponse bool) {
	if proto.HasRequestTitle(pckt.Payload) {
		return true, false
	}

	if proto.HasResponseTitle(pckt.Payload) {
		return false, true
	}

	// No request or response detected
	return false, false
}

func http1EndHint(m *tcp.Message) bool {
	if m.MissingChunk() {
		return false
	}

	return proto.HasFullPayload(m, m.PacketData()...)
}

func (l *Listener) read() {
	l.Lock()
	defer l.Unlock()
	for key, handle := range l.Handles {
		go func(key string, hndl packetHandle) {
			runtime.LockOSThread()

			defer l.closeHandles(key)
			linkSize := 14
			linkType := int(layers.LinkTypeEthernet)
			if _, ok := hndl.handler.(*pcap.Handle); ok {
				linkType = int(hndl.handler.(*pcap.Handle).LinkType())
				linkSize, ok = pcapLinkTypeLength(linkType)
				if !ok {
					if os.Getenv("GORDEBUG") != "0" {
						log.Printf("can not identify link type of an interface '%s'\n", key)
					}
					return // can't find the linktype size
				}
			}

			messageParser := tcp.NewMessageParser(l.messages, l.ports, hndl.ips, l.expiry, l.allowIncomplete)

			if l.protocol == tcp.ProtocolHTTP {
				messageParser.Start = http1StartHint
				messageParser.End = http1EndHint
			}

			timer := time.NewTicker(1 * time.Second)

			for {
				select {
				case <-l.quit:
					return
				case <-timer.C:
					if h, ok := hndl.handler.(PcapStatProvider); ok {
						s, _ := h.Stats()
						stats.Add("packets_received", int64(s.PacketsReceived))
						stats.Add("packets_dropped", int64(s.PacketsDropped))
						stats.Add("packets_if_dropped", int64(s.PacketsIfDropped))
					}
				default:
					data, ci, err := hndl.handler.ReadPacketData()
					if err == nil {
						if l.TimestampType == "go" {
							ci.Timestamp = time.Now()
						}

						messageParser.PacketHandler(&tcp.PcapPacket{
							Data:     data,
							LType:    linkType,
							LTypeLen: linkSize,
							Ci:       &ci,
						})
						continue
					}
					if enext, ok := err.(pcap.NextError); ok && enext == pcap.NextErrorTimeoutExpired {
						continue
					}
					if eno, ok := err.(syscall.Errno); ok && eno.Temporary() {
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

func (l *Listener) Messages() chan *tcp.Message {
	return l.messages
}

func (l *Listener) closeHandles(key string) {
	l.Lock()
	defer l.Unlock()
	if handle, ok := l.Handles[key]; ok {
		if c, ok := handle.handler.(io.Closer); ok {
			c.Close()
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
		l.Handles[ifi.Name] = packetHandle{
			handler: handle,
			ips:     interfaceIPs(ifi),
		}
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
		l.Handles[ifi.Name] = packetHandle{
			handler: handle,
			ips:     interfaceIPs(ifi),
		}
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

	tmp := l.host
	l.host = ""
	l.BPFFilter = l.Filter(pcap.Interface{})
	l.host = tmp

	if e = handle.SetBPFFilter(l.BPFFilter); e != nil {
		handle.Close()
		return fmt.Errorf("BPF filter error: %q, filter: %s", e, l.BPFFilter)
	}
	l.Handles["pcap_file"] = packetHandle{
		handler: handle,
	}
	return
}

func (l *Listener) activateAFPacket() error {
	szFrame, szBlock, numBlocks, err := afpacketComputeSize(32, 32<<10, os.Getpagesize())
	if err != nil {
		return err
	}

	var msg string
	for _, ifi := range l.Interfaces {
		handle, err := newAfpacketHandle(ifi.Name, szFrame, szBlock, numBlocks, false, pcap.BlockForever)

		if err != nil {
			msg += ("\n" + err.Error())
			continue
		}

		if l.BPFFilter == "" {
			l.BPFFilter = l.Filter(ifi)
		}
		fmt.Println("Interface:", ifi.Name, ". BPF Filter:", l.BPFFilter)
		handle.SetBPFFilter(l.BPFFilter, 64<<10)

		l.Handles[ifi.Name] = packetHandle{
			handler: handle,
			ips:     interfaceIPs(ifi),
		}
	}

	if len(l.Handles) == 0 {
		return fmt.Errorf("pcap handles error:%s", msg)
	}

	return nil
}

func (l *Listener) setInterfaces() (err error) {
	var pifis []pcap.Interface
	pifis, err = pcap.FindAllDevs()
	ifis, _ := net.Interfaces()
	if err != nil {
		return
	}

	for _, pi := range pifis {
		if isDevice(l.host, pi) {
			l.Interfaces = []pcap.Interface{pi}
			return
		}

		var ni net.Interface
		for _, i := range ifis {
			if i.Name == pi.Name {
				ni = i
				break
			}

			addrs, _ := i.Addrs()
			for _, a := range addrs {
				for _, pa := range pi.Addresses {
					if a.String() == pa.IP.String() {
						ni = i
						break
					}
				}
			}
		}

		if ni.Flags&net.FlagLoopback != 0 {
			l.loopIndex = ni.Index
		}

		if runtime.GOOS != "windows" {
			if len(pi.Addresses) == 0 {
				continue
			}

			if ni.Flags&net.FlagUp == 0 {
				continue
			}
		}

		l.Interfaces = append(l.Interfaces, pi)
	}
	return
}

func isDevice(addr string, ifi pcap.Interface) bool {
	// Windows npcap loopback have no IPs
	if addr == "127.0.0.1" && ifi.Name == `\Device\NPF_Loopback` {
		return true
	}

	if addr == ifi.Name {
		return true
	}

	for _, _addr := range ifi.Addresses {
		if _addr.IP.String() == addr {
			return true
		}
	}

	return false
}

func interfaceAddresses(ifi pcap.Interface) []string {
	var hosts []string
	for _, addr := range ifi.Addresses {
		hosts = append(hosts, addr.IP.String())
	}
	return hosts
}

func interfaceIPs(ifi pcap.Interface) []net.IP {
	var ips []net.IP
	for _, addr := range ifi.Addresses {
		ips = append(ips, addr.IP)
	}
	return ips
}

func listenAll(addr string) bool {
	switch addr {
	case "", "0.0.0.0", "[::]", "::":
		return true
	}
	return false
}

func portsFilter(transport string, direction string, ports []uint16) string {
	if len(ports) == 0 || ports[0] == 0 {
		return fmt.Sprintf("%s %s portrange 0-%d", transport, direction, 1<<16-1)
	}

	var filters []string
	for _, port := range ports {
		filters = append(filters, fmt.Sprintf("%s %s port %d", transport, direction, port))
	}
	return strings.Join(filters, " or ")
}

func hostsFilter(direction string, hosts []string) string {
	var hostsFilters []string
	for _, host := range hosts {
		hostsFilters = append(hostsFilters, fmt.Sprintf("%s host %s", direction, host))
	}

	return strings.Join(hostsFilters, " or ")
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
