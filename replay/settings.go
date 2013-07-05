package replay

import (
	"flag"
	"os"
	"strconv"
	"strings"
)

type ForwardHost struct {
	Url   string
	Limit int

	Stat *SiteStats
}

type ReplaySettings struct {
	Port int
	Host string

	ForwardAddress string

	Verbose bool

	hosts []*ForwardHost

	StatPath string
}

// ForwardedHosts implements forwardAddress syntax support for multiple hosts (coma separated), and rate limiting by specifing "|maxRps" after host name.
//
//    -f "host1,http://host2|10,host3"
//
func (r *ReplaySettings) ForwardedHosts() (hosts []*ForwardHost) {
	// If already parsed
	if len(r.hosts) > 0 {
		return r.hosts
	}

	for _, address := range strings.Split(r.ForwardAddress, ",") {
		host_info := strings.Split(address, "|")

		if strings.Index(host_info[0], "http") == -1 {
			host_info[0] = "http://" + host_info[0]
		}

		host := &ForwardHost{Url: host_info[0]}
		host.Stat = NewSiteStats()

		if len(host_info) > 1 {
			host.Limit, _ = strconv.Atoi(host_info[1])
		}

		r.hosts = append(r.hosts, host)
	}

	return r.hosts
}

// Helper to return address with port, e.g.: 127.0.0.1:28020
func (r *ReplaySettings) Address() string {
	return r.Host + ":" + strconv.Itoa(r.Port)
}

var Settings ReplaySettings = ReplaySettings{}

func init() {
	if len(os.Args) < 2 || os.Args[1] != "replay" {
		return
	}

	const (
		defaultPort = 28020
		defaultHost = "0.0.0.0"

		defaultAddress = "http://localhost:8080"
	)

	flag.IntVar(&Settings.Port, "p", defaultPort, "specify port number")

	flag.StringVar(&Settings.Host, "ip", defaultHost, "ip addresses to listen on")

	flag.StringVar(&Settings.StatPath, "stat", "", "Path for storing stats; If empty don't store")

	flag.StringVar(&Settings.ForwardAddress, "f", defaultAddress, "http address to forward traffic.\n\tYou can limit requests per second by adding `|num` after address.\n\tIf you have multiple addresses with different limits. For example: http://staging.example.com|100,http://dev.example.com|10")

	flag.BoolVar(&Settings.Verbose, "verbose", false, "Log requests")
}
