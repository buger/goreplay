package listener

import (
	"bytes"
	"fmt"
	"net"
	"testing"
	//"time"
)

func getTCPMessage() (msg *TCPMessage) {
	packet1 := &TCPPacket{Data: []byte("GET /pub/WWW/ HTTP/1.1\nHost: www.w3.org\r\n\r\n")}
	packet2 := &TCPPacket{Data: []byte("asd=asdasd&zxc=qwe\r\n\r\n")}

	return &TCPMessage{packets: []*TCPPacket{packet1, packet2}}
}

func mockServer() (replay net.Listener) {
	replay, _ = net.Listen("tcp", "127.0.0.1:0")

	fmt.Println(replay.Addr().String())

	return
}

func TestSendMessage(t *testing.T) {
	Settings.Verbose = false

	replay := mockServer()

	Settings.ReplayAddress = replay.Addr().String()

	msg := getTCPMessage()

	sendMessage(msg)

	conn, _ := replay.Accept()
	defer conn.Close()

	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	buf = buf[0:n]

	if bytes.Compare(buf, msg.Bytes()) != 0 {
		t.Errorf("Original and received requests does not match")
	}
}

/*
func TestPerformance(t *testing.T) {
	Settings.Verbose = true

	replay := mockReplayServer()

	msg := getTCPMessage()

	for y := 0; y < 10; y++ {
		go func() {
			for {
				conn, _ := replay.Accept()

				go func() {
					buf := make([]byte, 1024)
					n, _ := conn.Read(buf)
					buf = buf[0:n]

					conn.Close()
				}()
			}
		}()
	}

	for y := 0; y < 10; y++ {
		for i := 0; i < 500; i++ {
			go sendMessage(msg)
		}

		time.Sleep(time.Millisecond * 1000)
	}
}
*/
