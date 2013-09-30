package listener

import (
	"encoding/binary"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"testing"
)

func createHeader(ack uint32, port int) (header []byte, o_ack uint32) {
	if ack == 0 {
		ack = rand.Uint32()
	}

	seq := rand.Uint32()

	header = make([]byte, 256)

	binary.BigEndian.PutUint16(header[2:4], uint16(port))
	binary.BigEndian.PutUint32(header[4:8], seq)
	binary.BigEndian.PutUint32(header[8:12], ack)
	header[12] = 4 << 4
	header[13] = 8 // Setting PSH flag

	return header, ack
}

func getPackets(port int) [][]byte {
	if rand.Int()%2 == 0 {
		tcp, _ := createHeader(uint32(0), port)
		tcp = append(tcp, []byte("GET /pub/WWW/ HTTP/1.1\nHost: www.w3.org\r\n\r\n")...)

		return [][]byte{tcp}
	} else {
		tcp1, ack := createHeader(uint32(0), port)
		tcp1 = append(tcp1, []byte("POST /pub/WWW/ HTTP/1.1\nHost: www.w3.org\r\n\r\n")...)

		tcp2, _ := createHeader(ack, port)
		tcp2 = append(tcp2, []byte("a=1&b=2\r\n\r\n")...)

		return [][]byte{tcp1, tcp2}
	}

}

func TestRawTCPListener(t *testing.T) {
	Settings.Verbose = true

	server := mockServer()
	//server_addr := server.Addr().String()
	host, port_str, _ := net.SplitHostPort(server.Addr().String())
	port, _ := strconv.Atoi(port_str)

	// Accept all packets
	go func() {
		conn, _ := server.Accept()
		conn.Close()
	}()

	listener := RAWTCPListen(host, port)

	var wg sync.WaitGroup

	go func() {
		for {
			listener.Receive()
			wg.Done()
		}
	}()

	for i := 0; i < 10000; i++ {
		wg.Add(1)

		packets := getPackets(port)

		for _, packet := range packets {
			listener.parsePacket(packet)
		}
	}

	wg.Wait()
}
