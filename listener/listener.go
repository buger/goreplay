// Listener capture TCP traffic using RAW SOCKETS.
// Note: it requires sudo or root access.
//
// Rigt now it suport only HTTP, and only GET requests.
package listener

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
)

// Enable debug logging only if "--verbose" flag passed
func Debug(v ...interface{}) {
	if Settings.verbose {
		log.Println(v...)
	}
}

func greeting() {

}

// Because its sub-program, Run acts as `main`
func Run() {
	if os.Getuid() != 0 {
		fmt.Println("Please start the listener as root or sudo!")
		fmt.Println("This is required since listener sniff traffic on given port.")
		os.Exit(1)
	}

	fmt.Println("Listening for HTTP traffic on", Settings.port, "port")
	fmt.Println("Forwarding requests to replay server:", Settings.ReplayServer())

	// Connection to reaplay server
	serverAddr, err := net.ResolveUDPAddr("udp4", Settings.ReplayServer())
	conn, err := net.DialUDP("udp", nil, serverAddr)

	if err != nil {
		log.Fatal("Connection error", err)
	}

	// Sniffing traffic from given port
	listener := RAWTCPListen("0.0.0.0", Settings.port)

	for {
		message := listener.Receive()

		go func(m *TCPMessage) {
			if Settings.verbose {
				buf := bytes.NewBuffer(m.Bytes())
				reader := bufio.NewReader(buf)

				request, err := http.ReadRequest(reader)

				if err != nil {
					Debug("Error while parsing request:", string(m.Bytes()))
				} else {
					request.ParseMultipartForm(32 << 20)
					Debug("Forwarding request:", request)
				}
			}

			conn.Write(m.Bytes())
		}(message)
	}

	conn.Close()
}
