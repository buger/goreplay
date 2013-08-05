package listener

import (
	"flag"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPort    = 80
	defaultAddress = "0.0.0.0"

	defaultReplayAddress = "localhost:28020"
)

type ListenerSettings struct {
	Port    int
	Address string

	ReplayAddress string

	ReplayLimit int

	Verbose bool
}

var Settings ListenerSettings = ListenerSettings{}

func (s *ListenerSettings) ReplayServer() string {
	host_info := strings.Split(s.ReplayAddress, "|")

	if len(host_info) > 1 {
		s.ReplayLimit, _ = strconv.Atoi(host_info[1])
	}

	return host_info[0]
}

func init() {
	if len(os.Args) < 2 || os.Args[1] != "listen" {
		return
	}

	flag.IntVar(&Settings.Port, "p", defaultPort, "Specify the http server port whose traffic you want to capture")

	flag.StringVar(&Settings.Address, "ip", defaultAddress, "Specifi IP address to listen")

	flag.StringVar(&Settings.ReplayAddress, "r", defaultReplayAddress, "Address of replay server.")

	flag.BoolVar(&Settings.Verbose, "verbose", false, "Log requests")
}
