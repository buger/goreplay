package capture

import (
	"fmt"
	"net"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const (
	// ETHALL htons(ETH_P_ALL)
	ETHALL uint16 = unix.ETH_P_ALL<<8 | unix.ETH_P_ALL>>8
	// BLOCKSIZE ring buffer block_size
	BLOCKSIZE = 64 << 10
	// BLOCKNR ring buffer block_nr
	BLOCKNR = (2 << 20) / BLOCKSIZE // 2mb / 64kb
	// FRAMESIZE ring buffer frame_size
	FRAMESIZE = BLOCKSIZE
	// FRAMENR ring buffer frame_nr
	FRAMENR = BLOCKNR * BLOCKSIZE / FRAMESIZE
	// MAPHUGE2MB 2mb huge map
	MAPHUGE2MB = 21 << unix.MAP_HUGE_SHIFT
)

var tpacket2hdrlen = tpAlign(int(unsafe.Sizeof(unix.Tpacket2Hdr{})))

// SockRaw is a linux M'maped af_packet socket
type SockRaw struct {
	mu          sync.Mutex
	fd          int
	ifindex     int
	snaplen     int
	pollTimeout uintptr
	frame       uint32 // current frame
	buf         []byte // points to the memory space of the ring buffer shared with the kernel.
	loopIndex   int32  // this field must filled to avoid reading packet twice on a loopback device
}

// NewSocket returns new M'maped sock_raw on packet version 2.
func NewSocket(ifi net.Interface) (*SockRaw, error) {
	// sock create
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(ETHALL))
	if err != nil {
		return nil, err
	}
	sock := &SockRaw{
		fd:          fd,
		ifindex:     ifi.Index,
		snaplen:     FRAMESIZE,
		pollTimeout: ^uintptr(0),
	}

	// set packet version
	err = unix.SetsockoptInt(fd, unix.SOL_PACKET, unix.PACKET_VERSION, unix.TPACKET_V2)
	if err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("setsockopt packet_version: %v", err)
	}

	// bind to interface
	addr := unix.RawSockaddrLinklayer{
		Family:   unix.AF_PACKET,
		Protocol: ETHALL,
		Ifindex:  int32(ifi.Index),
	}
	_, _, e := unix.Syscall(
		unix.SYS_BIND,
		uintptr(fd),
		uintptr(unsafe.Pointer(&addr)),
		uintptr(unix.SizeofSockaddrLinklayer),
	)
	if e != 0 {
		unix.Close(fd)
		return nil, e
	}

	// create shared-memory ring buffer
	tp := &unix.TpacketReq{
		Block_size: BLOCKSIZE,
		Block_nr:   BLOCKNR,
		Frame_size: FRAMESIZE,
		Frame_nr:   FRAMENR,
	}
	err = unix.SetsockoptTpacketReq(sock.fd, unix.SOL_PACKET, unix.PACKET_RX_RING, tp)
	if err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("setsockopt packet_rx_ring: %v", err)
	}
	sock.buf, err = unix.Mmap(
		sock.fd,
		0,
		BLOCKSIZE*BLOCKNR,
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_SHARED|MAPHUGE2MB,
	)
	if err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("socket mmap error: %v", err)
	}
	return sock, nil
}

// ReadPacketData implements gopacket.PacketDataSource.
func (sock *SockRaw) ReadPacketData() (buf []byte, ci gopacket.CaptureInfo, err error) {
	sock.mu.Lock()
	defer sock.mu.Unlock()
	var tpHdr *unix.Tpacket2Hdr
	poll := &unix.PollFd{
		Fd:     int32(sock.fd),
		Events: unix.POLLIN,
	}
	var i int
read:
	i = int(sock.frame * FRAMESIZE)
	tpHdr = (*unix.Tpacket2Hdr)(unsafe.Pointer(&sock.buf[i]))
	sock.frame = (sock.frame + 1) % FRAMENR

	if tpHdr.Status&unix.TP_STATUS_USER == 0 {
		_, _, e := unix.Syscall(unix.SYS_POLL, uintptr(unsafe.Pointer(poll)), 1, sock.pollTimeout)
		if e != 0 && e != unix.EINTR {
			return buf, ci, e
		}
		// it might be some other frame with data!
		if tpHdr.Status&unix.TP_STATUS_USER == 0 {
			goto read
		}
	}
	tpHdr.Status = unix.TP_STATUS_KERNEL
	sockAddr := (*unix.RawSockaddrLinklayer)(unsafe.Pointer(&sock.buf[i+tpacket2hdrlen]))

	// parse out repeating packets on loopback
	if sockAddr.Ifindex == sock.loopIndex && sock.frame%2 != 0 {
		goto read
	}

	ci.Length = int(tpHdr.Len)
	ci.Timestamp = time.Unix(int64(tpHdr.Sec), int64(tpHdr.Nsec))
	ci.InterfaceIndex = int(sockAddr.Ifindex)
	buf = make([]byte, tpHdr.Snaplen)
	ci.CaptureLength = copy(buf, sock.buf[i+int(tpHdr.Mac):])

	return
}

// Close closes the underlying socket
func (sock *SockRaw) Close() (err error) {
	sock.mu.Lock()
	defer sock.mu.Unlock()
	if sock.fd != -1 {
		unix.Munmap(sock.buf)
		sock.buf = nil
		err = unix.Close(sock.fd)
		sock.fd = -1
	}
	return
}

// SetSnapLen sets the maximum capture length to the given value.
// for this to take effects on the kernel level SetBPFilter should be called too.
func (sock *SockRaw) SetSnapLen(snap int) error {
	sock.mu.Lock()
	defer sock.mu.Unlock()
	if snap < 0 {
		return fmt.Errorf("expected %d snap length to be at least 0", snap)
	}
	if snap > FRAMESIZE {
		snap = FRAMESIZE
	}
	sock.snaplen = snap
	return nil
}

// SetTimeout sets poll wait timeout for the socket.
// negative value will block forever
func (sock *SockRaw) SetTimeout(t time.Duration) error {
	sock.mu.Lock()
	defer sock.mu.Unlock()
	sock.pollTimeout = uintptr(t)
	return nil
}

// GetSnapLen returns the maximum capture length
func (sock *SockRaw) GetSnapLen() int {
	sock.mu.Lock()
	defer sock.mu.Unlock()
	return sock.snaplen
}

// SetBPFFilter compiles and sets a BPF filter for the socket handle.
func (sock *SockRaw) SetBPFFilter(expr string) error {
	sock.mu.Lock()
	defer sock.mu.Unlock()
	if expr == "" {
		return unix.SetsockoptInt(sock.fd, unix.SOL_SOCKET, unix.SO_DETACH_FILTER, 0)
	}
	filter, err := pcap.CompileBPFFilter(layers.LinkTypeEthernet, sock.snaplen, expr)
	if err != nil {
		return err
	}
	if len(filter) > int(^uint16(0)) {
		return fmt.Errorf("filters out of range 0-%d", ^uint16(0))
	}
	if len(filter) == 0 {
		return unix.SetsockoptInt(sock.fd, unix.SOL_SOCKET, unix.SO_DETACH_FILTER, 0)
	}
	fprog := &unix.SockFprog{
		Len:    uint16(len(filter)),
		Filter: &(*(*[]unix.SockFilter)(unsafe.Pointer(&filter)))[0],
	}
	return unix.SetsockoptSockFprog(sock.fd, unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, fprog)
}

// SetPromiscuous sets promiscous mode to the required value. for better result capture on all interfaces instead.
// If it is enabled, traffic not destined for the interface will also be captured.
func (sock *SockRaw) SetPromiscuous(b bool) error {
	sock.mu.Lock()
	defer sock.mu.Unlock()
	mreq := unix.PacketMreq{
		Ifindex: int32(sock.ifindex),
		Type:    unix.PACKET_MR_PROMISC,
	}

	opt := unix.PACKET_ADD_MEMBERSHIP
	if !b {
		opt = unix.PACKET_DROP_MEMBERSHIP
	}

	return unix.SetsockoptPacketMreq(sock.fd, unix.SOL_PACKET, opt, &mreq)
}

// Stats returns number of packets and dropped packets. This will be the number of packets/dropped packets since the last call to stats (not the cummulative sum!).
func (sock *SockRaw) Stats() (*unix.TpacketStats, error) {
	sock.mu.Lock()
	defer sock.mu.Unlock()
	return unix.GetsockoptTpacketStats(sock.fd, unix.SOL_PACKET, unix.PACKET_STATISTICS)
}

// SetLoopbackIndex necessary to avoid reading packet twice on a loopback device
func (sock *SockRaw) SetLoopbackIndex(i int32) {
	sock.mu.Lock()
	defer sock.mu.Unlock()
	sock.loopIndex = i
}

// WritePacketData transmits a raw packet.
func (sock *SockRaw) WritePacketData(pkt []byte) error {
	_, err := unix.Write(sock.fd, pkt)
	return err
}

func tpAlign(x int) int {
	return int((uint(x) + unix.TPACKET_ALIGNMENT - 1) &^ (unix.TPACKET_ALIGNMENT - 1))
}
