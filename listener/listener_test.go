package listener

import (
	"bytes"
	"fmt"
	"net"
	"testing"
)

func getTCPMessage() (msg *TCPMessage) {
	packet := &TCPPacket{Data: []byte("GET /pub/WWW/ HTTP/1.1\nHost: www.w3.org\r\n\r\n")}

	return &TCPMessage{packets: []*TCPPacket{packet}}
}

func mockReplayServer() (listener net.Listener) {
	listener, _ = net.Listen("tcp", "127.0.0.1:0")

	Settings.ReplayAddress = listener.Addr().String()

	fmt.Println(listener.Addr().String())

	return
}

func TestSendMessage(t *testing.T) {
	Settings.Verbose = true

	listener := mockReplayServer()

	msg := getTCPMessage()

	sendMessage(msg)

	conn, _ := listener.Accept()
	defer conn.Close()

	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	buf = buf[0:n]

	if bytes.Compare(buf, msg.Bytes()) != 0 {
		t.Errorf("Original and received requests does not match")
	}
}
