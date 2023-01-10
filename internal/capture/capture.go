package capture

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"github.com/buger/goreplay/internal/size"
	"github.com/buger/goreplay/internal/tcp"
	"github.com/buger/goreplay/proto"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

type PcapSetFilter interface {
	SetBPFFilter(string) error
}

// PcapOptions options that can be set on a pcap capture handle,
// these options take effect on inactive pcap handles
type PcapOptions struct {
	BufferTimeout   time.Duration   `json:"input-raw-buffer-timeout"`
	TimestampType   string          `json:"input-raw-timestamp-type"`
	BPFFilter       string          `json:"input-raw-bpf-filter"`
	BufferSize      size.Size       `json:"input-raw-buffer-size"`
	Promiscuous     bool            `json:"input-raw-promisc"`
	Monitor         bool            `json:"input-raw-monitor"`
	Snaplen         bool            `json:"input-raw-override-snaplen"`
	Engine          EngineType      `json:"input-raw-engine"`
	VXLANPort       int             `json:"input-raw-vxlan-port"`
	VXLANVNIs       []int           `json:"input-raw-vxlan-vni"`
	VLAN            bool            `json:"input-raw-vlan"`
	VLANVIDs        []int           `json:"input-raw-vlan-vid"`
	Expire          time.Duration   `json:"input-raw-expire"`
	TrackResponse   bool            `json:"input-raw-track-response"`
	Protocol        tcp.TCPProtocol `json:"input-raw-protocol"`
	RealIPHeader    string          `json:"input-raw-realip-header"`
	Stats           bool            `json:"input-raw-stats"`
	AllowIncomplete bool            `json:"input-raw-allow-incomplete"`
	IgnoreInterface []string        `json:"input-raw-ignore-interface"`
	Transport       string
}

// Listener handle traffic capture, this is its representation.
type Listener struct {
	sync.Mutex

	config PcapOptions

	Activate   func() error // function is used to activate the engine. it must be called before reading packets
	Handles    map[string]packetHandle
	Interfaces []pcap.Interface
	loopIndex  int
	Reading    chan bool // this channel is closed when the listener has started reading packets
	messages   chan *tcp.Message

	ports []uint16
	host  string // pcap file name or interface (name, hardware addr, index or ip address)

	closeDone chan struct{}
	quit      chan struct{}
	closed    bool
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
	EngineVXLAN
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
	case "vxlan":
		*eng = EngineVXLAN
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
	case EngineVXLAN:
		e = "vxlan"
	default:
		e = ""
	}
	return e
}

// NewListener creates and initialize a new Listener. if transport or/and engine are invalid/unsupported
// is "tcp" and "pcap", are assumed. l.Engine and l.Transport can help to get the values used.
// if there is an error it will be associated with getting network interfaces
func NewListener(host string, ports []uint16, config PcapOptions) (l *Listener, err error) {
	l = &Listener{}

	l.host = host
	if l.host == "localhost" {
		l.host = "127.0.0.1"
	}
	l.ports = ports

	l.config = config
	l.config.Transport = "tcp"
	l.Handles = make(map[string]packetHandle)

	l.closeDone = make(chan struct{})
	l.quit = make(chan struct{})
	l.Reading = make(chan bool)
	l.messages = make(chan *tcp.Message, 10000)

	if strings.HasPrefix(l.host, "k8s://") {
		l.config.BPFFilter = l.Filter(pcap.Interface{}, k8sIPs(l.host[6:])...)
	}

	switch config.Engine {
	default:
		l.Activate = l.activatePcap
	case EngineRawSocket:
		l.Activate = l.activateRawSocket
	case EngineAFPacket:
		l.Activate = l.activateAFPacket
	case EnginePcapFile:
		l.Activate = l.activatePcapFile
		return
	case EngineVXLAN:
		l.Activate = l.activateVxLanSocket
		return
	}

	err = l.setInterfaces()
	if err != nil {
		return nil, err
	}
	return
}

// Listen listens for packets from the handles, and call handler on every packet received
// until the context done signal is sent or there is unrecoverable error on all handles.
// this function must be called after activating pcap handles
func (l *Listener) Listen(ctx context.Context) (err error) {
	l.Lock()
	for key, handle := range l.Handles {
		go l.readHandle(key, handle)
	}
	l.Unlock()

	go func() {
		for {
			time.Sleep(time.Second)

			if l.closed {
				return
			}

			// Check for Pod IP changes
			if strings.HasPrefix(l.host, "k8s://") {
				newFilter := l.Filter(pcap.Interface{}, k8sIPs(l.host[6:])...)
				if newFilter != l.config.BPFFilter {
					fmt.Println("k8s pods configuration changed, new filter: ", newFilter)
					for _, h := range l.Handles {
						if _, ok := h.handler.(PcapSetFilter); ok {
							h.handler.(PcapSetFilter).SetBPFFilter(newFilter)
						}
					}

					l.config.BPFFilter = newFilter
				}
			}

			var prevInterfaces []string
			for _, in := range l.Interfaces {
				prevInterfaces = append(prevInterfaces, in.Name)
			}
			l.setInterfaces()

			for _, in := range l.Interfaces {
				var found bool

				for _, prev := range prevInterfaces {
					if in.Name == prev {
						found = true
					}
				}

				if !found {
					fmt.Println("Found new interface:", in.Name)
					l.Lock()
					l.Activate()

					for key, handle := range l.Handles {
						if key == in.Name {
							fmt.Println("Activating capture on:", in.Name)
							go l.readHandle(key, handle)
							break
						}
					}
					l.Unlock()
				}
			}
		}
	}()

	close(l.Reading)
	done := ctx.Done()
	select {
	case <-done:
		close(l.quit) // signal close on all handles
		<-l.closeDone // wait all handles to be closed
		err = ctx.Err()
	case <-l.closeDone: // all handles closed voluntarily
	}

	l.closed = true
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

// Allowed format:
//
//	[namespace/]pod/[pod_name]
//	[namespace/]deployment/[deployment_name]
//	[namespace/]daemonset/[daemonset_name]
//	[namespace/]labelSelector/[selector]
//	[namespace/]fieldSelector/[selector]
func k8sIPs(addr string) []string {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	sections := strings.Split(addr, "/")

	if len(sections) < 2 {
		panic("Not supported k8s scheme. Allowed values: [namespace/]pod/[pod_name], [namespace/]deployment/[deployment_name], [namespace/]daemonset/[daemonset_name], [namespace/]label/[label-name]/[label-value]")
	}

	// If no namespace passed, assume it is ALL
	switch sections[0] {
	case "pod", "deployment", "daemonset", "labelSelector", "fieldSelector":
		sections = append([]string{""}, sections...)
	}

	namespace, selectorType, selectorValue := sections[0], sections[1], sections[2]

	labelSelector := ""
	fieldSelector := ""

	switch selectorType {
	case "pod":
		fieldSelector = "metadata.name=" + selectorValue
	case "deployment":
		labelSelector = "app=" + selectorValue
	case "daemonset":
		labelSelector = "pod-template-generation=1,name=" + selectorValue
	case "labelSelector":
		labelSelector = selectorValue
	case "fieldSelector":
		fieldSelector = selectorValue
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector})
	if err != nil {
		panic(err.Error())
	}

	var podIPs []string
	for _, pod := range pods.Items {
		for _, podIP := range pod.Status.PodIPs {
			podIPs = append(podIPs, podIP.IP)
		}
	}
	return podIPs
}

// Filter returns automatic filter applied by goreplay
// to a pcap handle of a specific interface
func (l *Listener) Filter(ifi pcap.Interface, hosts ...string) (filter string) {
	// https://www.tcpdump.org/manpages/pcap-filter.7.html

	if len(hosts) == 0 {
		// If k8s have not found any IPs
		if strings.HasPrefix(l.host, "k8s://") {
			hosts = []string{}
		} else {
			hosts = []string{l.host}

			if listenAll(l.host) || isDevice(l.host, ifi) {
				hosts = interfaceAddresses(ifi)
			}
		}
	}

	filter = portsFilter(l.config.Transport, "dst", l.ports)

	if len(hosts) != 0 && !l.config.Promiscuous {
		filter = fmt.Sprintf("((%s) and (%s))", filter, hostsFilter("dst", hosts))
	} else {
		filter = fmt.Sprintf("(%s)", filter)
	}

	if l.config.TrackResponse {
		responseFilter := portsFilter(l.config.Transport, "src", l.ports)

		if len(hosts) != 0 && !l.config.Promiscuous {
			responseFilter = fmt.Sprintf("((%s) and (%s))", responseFilter, hostsFilter("src", hosts))
		} else {
			responseFilter = fmt.Sprintf("(%s)", responseFilter)
		}

		filter = fmt.Sprintf("%s or %s", filter, responseFilter)
	}

	if l.config.VLAN {
		if len(l.config.VLANVIDs) > 0 {
			for _, vi := range l.config.VLANVIDs {
				filter = fmt.Sprintf("vlan %d and ", vi) + filter
			}
		} else {
			filter = "vlan and " + filter
		}
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

	if l.config.TimestampType != "" && l.config.TimestampType != "go" {
		var ts pcap.TimestampSource
		ts, err = pcap.TimestampSourceFromString(l.config.TimestampType)
		fmt.Println("Setting custom Timestamp Source. Supported values: `go`, ", inactive.SupportedTimestamps())
		err = inactive.SetTimestampSource(ts)
		if err != nil {
			return nil, fmt.Errorf("%q: supported timestamps: %q, interface: %q", err, inactive.SupportedTimestamps(), ifi.Name)
		}
	}
	if l.config.Promiscuous {
		if err = inactive.SetPromisc(l.config.Promiscuous); err != nil {
			return nil, fmt.Errorf("promiscuous mode error: %q, interface: %q", err, ifi.Name)
		}
	}
	if l.config.Monitor {
		if err = inactive.SetRFMon(l.config.Monitor); err != nil && !errors.Is(err, pcap.CannotSetRFMon) {
			return nil, fmt.Errorf("monitor mode error: %q, interface: %q", err, ifi.Name)
		}
	}

	var snap int

	if !l.config.Snaplen {
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
	if l.config.BufferSize > 0 {
		err = inactive.SetBufferSize(int(l.config.BufferSize))
		if err != nil {
			return nil, fmt.Errorf("handle buffer size error: %q, interface: %q", err, ifi.Name)
		}
	}
	if l.config.BufferTimeout == 0 {
		l.config.BufferTimeout = 2000 * time.Millisecond
	}
	err = inactive.SetTimeout(l.config.BufferTimeout)
	if err != nil {
		return nil, fmt.Errorf("handle buffer timeout error: %q, interface: %q", err, ifi.Name)
	}
	handle, err = inactive.Activate()
	if err != nil {
		return nil, fmt.Errorf("PCAP Activate device error: %q, interface: %q", err, ifi.Name)
	}

	bpfFilter := l.config.BPFFilter
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
	if err = handle.SetPromiscuous(l.config.Promiscuous || l.config.Monitor); err != nil {
		return nil, fmt.Errorf("promiscuous mode error: %q, interface: %q", err, ifi.Name)
	}
	if l.config.BPFFilter == "" {
		l.config.BPFFilter = l.Filter(ifi)
	}
	fmt.Println("BPF Filter: ", l.config.BPFFilter)
	if err = handle.SetBPFFilter(l.config.BPFFilter); err != nil {
		handle.Close()
		return nil, fmt.Errorf("BPF filter error: %q%s, interface: %q", err, l.config.BPFFilter, ifi.Name)
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

	req, res := http1StartHint(m.Packets()[0])
	return proto.HasFullPayload(m, m.PacketData()...) && (req || res)
}

func (l *Listener) readHandle(key string, hndl packetHandle) {
	runtime.LockOSThread()

	defer l.closeHandles(key)
	linkSize := 14
	linkType := int(layers.LinkTypeEthernet)
	if _, ok := hndl.handler.(*pcap.Handle); ok {
		linkType = int(hndl.handler.(*pcap.Handle).LinkType())
		linkSize, ok = pcapLinkTypeLength(linkType, l.config.VLAN)
		if !ok {
			if os.Getenv("GORDEBUG") != "0" {
				log.Printf("can not identify link type of an interface '%s'\n", key)
			}
			return // can't find the linktype size
		}
	}

	messageParser := tcp.NewMessageParser(l.messages, l.ports, hndl.ips, l.config.Expire, l.config.AllowIncomplete)

	if l.config.Protocol == tcp.ProtocolHTTP {
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
				s, err := h.Stats()
				if err == nil {
					stats.Add("packets_received", int64(s.PacketsReceived))
					stats.Add("packets_dropped", int64(s.PacketsDropped))
					stats.Add("packets_if_dropped", int64(s.PacketsIfDropped))
				}
			}
		default:
			data, ci, err := hndl.handler.ReadPacketData()
			if err == nil {
				if l.config.TimestampType == "go" {
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
		if _, found := l.Handles[ifi.Name]; found {
			continue
		}

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

func (l *Listener) activateVxLanSocket() error {
	handler, err := newVXLANHandler(l.config.VXLANPort, l.config.VXLANVNIs)
	if err != nil {
		return err
	}
	l.Handles["vxlan"] = packetHandle{
		handler: handler,
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
		if _, found := l.Handles[ifi.Name]; found {
			continue
		}

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
	l.config.BPFFilter = l.Filter(pcap.Interface{})
	l.host = tmp

	if e = handle.SetBPFFilter(l.config.BPFFilter); e != nil {
		handle.Close()
		return fmt.Errorf("BPF filter error: %q, filter: %s", e, l.config.BPFFilter)
	}

	fmt.Println("BPF Filter:", l.config.BPFFilter)

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
		if _, found := l.Handles[ifi.Name]; found {
			continue
		}

		handle, err := newAfpacketHandle(ifi.Name, szFrame, szBlock, numBlocks, false, pcap.BlockForever)

		if err != nil {
			msg += ("\n" + err.Error())
			continue
		}

		if l.config.BPFFilter == "" {
			l.config.BPFFilter = l.Filter(ifi)
		}
		fmt.Println("Interface:", ifi.Name, ". BPF Filter:", l.config.BPFFilter)
		handle.SetBPFFilter(l.config.BPFFilter, 64<<10)

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
	l.Interfaces = []pcap.Interface{}

	if err != nil {
		return
	}

	for _, pi := range pifis {
		ignore := false
		for _, ig := range l.config.IgnoreInterface {
			if pi.Name == ig {
				ignore = true
				break
			}
		}

		if ignore {
			continue
		}

		if strings.HasPrefix(l.host, "k8s://") {
			if !strings.HasPrefix(pi.Name, "veth") {
				continue
			}
		}

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

	if strings.HasSuffix(addr, "*") {
		if strings.HasPrefix(ifi.Name, addr[:len(addr)-1]) {
			return true
		}
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

func pcapLinkTypeLength(lType int, vlan bool) (int, bool) {
	switch layers.LinkType(lType) {
	case layers.LinkTypeEthernet:
		if vlan {
			return 18, true
		} else {
			return 14, true
		}
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
