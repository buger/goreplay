package replay

import (
	"io"
	"log"
	"net"
)

const bufSize = 4096

func RunReplayFromNetwork(rf *RequestFactory) {
	listener, err := net.Listen("tcp", Settings.Address)

	log.Println("Starting replay server at:", Settings.Address)

	if err != nil {
		log.Fatal("Can't start:", err)
	}

	for _, host := range Settings.ForwardedHosts() {
		log.Println("Forwarding requests to:", host.Url, "limit:", host.Limit)
	}

	for {
		conn, err := listener.Accept()

		if err != nil {
			log.Println("Error while Accept()", err)
			continue
		}

		go handleConnection(conn, rf)
	}
}

func handleConnection(conn net.Conn, rf *RequestFactory) error {
	defer conn.Close()

	var read = true
	var response []byte
	var buf []byte

	buf = make([]byte, bufSize)

	for read {
		n, err := conn.Read(buf)

		switch err {
		case io.EOF:
			read = false
		case nil:
			response = append(response, buf[:n]...)
			if n < bufSize {
				read = false
			}
		default:
			read = false
		}
	}

	rf.Add(response)

	return nil
}
