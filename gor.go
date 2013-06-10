// Gor is simple http traffic replication tool written in Go. Its main goal to replay traffic from production servers to staging and dev environments.
// Now you can test your code on real user sessions in an automated and repeatable fashion.
//
// Gor consists of 2 parts: listener and replay servers.
// Listener catch http traffic from given port in real-time and send it to replay server via UDP. Replay server forwards traffic to given address.
package main

import (
	"flag"
	"fmt"
	"github.com/buger/gor/listener"
	"github.com/buger/gor/replay"
	"os"
)

const (
	VERSION = "0.3"
)

func main() {
	fmt.Println("Version:", VERSION)

	mode := "unknown"

	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	if mode != "listen" && mode != "replay" {
		fmt.Println("Usage: \n\tgor listen -h\n\tgor replay -h")
		return
	}

	// Remove mode attr
	os.Args = append(os.Args[:1], os.Args[2:]...)

	flag.Parse()

	switch mode {
	case "listen":
		listener.Run()
	case "replay":
		replay.Run()
	}

}
