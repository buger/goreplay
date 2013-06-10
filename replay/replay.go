// Replay server receive requests objects from Listeners and forward it to given address.
// Basic usage:
//
//     gor replay -f http://staging.server
//
//
// Rate limiting
//
// It can be useful if you want forward only part of production traffic, not to overload staging environment. You can specify desired request per second using "|" operator after server address:
//
//     # staging.server not get more than 10 requests per second
//     gor replay -f "http://staging.server|10"
//
//
// Forward to multiple addresses
//
// Just separate addresses by coma:
//    gor replay -f "http://staging.server|10,http://dev.server|20"
//
//
//  For more help run:
//
//     gor replay -h
//
package replay

import (
	"bufio"
	"bytes"
	"log"
	"net"
	"net/http"
)

const bufSize = 1024 * 10

// Enable debug logging only if "--verbose" flag passed
func Debug(v ...interface{}) {
	if Settings.verbose {
		log.Println(v...)
	}
}

func ParseRequest(data []byte) (request *http.Request, err error) {
	buf := bytes.NewBuffer(data)
	reader := bufio.NewReader(buf)

	request, err = http.ReadRequest(reader)
	return
}

// Because its sub-program, Run acts as `main`
// Replay server listen to UDP traffic from Listeners
// Each request processed by RequestFactory
func Run() {
	var buf [bufSize]byte

	addr, err := net.ResolveUDPAddr("udp", Settings.Address())
	if err != nil {
		log.Fatal("Can't start:", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	log.Println("Starting replay server at:", Settings.Address())

	if err != nil {
		log.Fatal("Can't start:", err)
	}

	defer conn.Close()

	for _, host := range Settings.ForwardedHosts() {
		log.Println("Forwarding requests to:", host.Url, "limit:", host.Limit)
	}

	requestFactory := NewRequestFactory()

	for {
		n, _, err := conn.ReadFromUDP(buf[0:])

		if err != nil {
			continue
		}

		if n > 0 {
			if n > bufSize {
				Debug("Too large udp packet", bufSize)
			}

			if request, err := ParseRequest(buf[0:n]); err != nil {
				Debug("Error while parsing request", err, buf[0:n])
			} else {
				requestFactory.Add(request)
			}
		}
	}

}
