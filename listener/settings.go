package listener

import (
	"flag"
	"os"
	"strings"
)

const (
	defaultPort             = 80
	defaultNetworkInterface = "any"

	defaultReplayAddress = "localhost:28020"
)

type ListenerSettings struct {
	networkInterface string
	port             int

	replayAddress string

	verbose bool
}

var Settings ListenerSettings = ListenerSettings{}

func (s *ListenerSettings) ReplayServer() string {
	if !strings.Contains(s.replayAddress, ":") {
		return s.replayAddress + ":28020"
	}

	return s.replayAddress
}

func init() {
	if len(os.Args) < 2 || os.Args[1] != "listen" {
		return
	}

	flag.IntVar(&Settings.port, "p", defaultPort, "Specify the http server port whose traffic you want to capture")

	flag.StringVar(&Settings.networkInterface, "i", defaultNetworkInterface, "By default it try to listen on all network interfaces.To get list of interfaces run `ifconfig`")

	flag.StringVar(&Settings.replayAddress, "r", defaultReplayAddress, "Address of replay server.")

	flag.BoolVar(&Settings.verbose, "verbose", false, "Log requests")
}
