// Listener capture TCP traffic using RAW SOCKETS.
// Note: it requires sudo or root access.
//
// Rigt now it suport only HTTP
package listener

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
)

// Enable debug logging only if "--verbose" flag passed
func Debug(v ...interface{}) {
	if Settings.Verbose {
		log.Println(v...)
	}
}

func ReplayServer() (conn net.Conn, err error) {
	// Connection to reaplay server
	conn, err = net.Dial("tcp", Settings.ReplayAddress)

	if err != nil {
		log.Println("Connection error ", err, Settings.ReplayAddress)
	}

	return
}

// Because its sub-program, Run acts as `main`
func Run() {
	if os.Getuid() != 0 {
		fmt.Println("Please start the listener as root or sudo!")
		fmt.Println("This is required since listener sniff traffic on given port.")
		os.Exit(1)
	}

	fmt.Println("Listening for HTTP traffic on", Settings.Address+":"+strconv.Itoa(Settings.Port))
	fmt.Println("Forwarding requests to replay server:", Settings.ReplayAddress)

	// Sniffing traffic from given address
	listener := RAWTCPListen(Settings.Address, Settings.Port)

	for {
		// Receiving TCPMessage object
		m := listener.Receive()

		go sendMessage(m)
	}
}

func sendMessage(m *TCPMessage) {
	conn, err := ReplayServer()

	if err != nil {
		log.Println("Failed to send message. Replay server not respond.")
		return
	} else {
		defer conn.Close()
	}

	// For debugging purpose
	// Usually request parsing happens in replay part
	if Settings.Verbose {
		buf := bytes.NewBuffer(m.Bytes())
		reader := bufio.NewReader(buf)

		request, err := http.ReadRequest(reader)

		if err != nil {
			Debug("Error while parsing request:", err, string(m.Bytes()))
		} else {
			request.ParseMultipartForm(32 << 20)
			Debug("Forwarding request:", request)
		}
	}

	_, err = conn.Write(m.Bytes())

	if err != nil {
		log.Println("Error while sending requests", err)
	}
}
