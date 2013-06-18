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
	"log"
	"os"
	"runtime/pprof"
	"time"
)

const (
	VERSION = "0.3.2"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

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

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)

		time.AfterFunc(60*time.Second, func() {
			pprof.StopCPUProfile()
			f.Close()
			log.Println("Stop profiling after 60 seconds")
		})
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		time.AfterFunc(60*time.Second, func() {
			pprof.WriteHeapProfile(f)
			f.Close()
		})
	}

	switch mode {
	case "listen":
		listener.Run()
	case "replay":
		replay.Run()
	}

}
