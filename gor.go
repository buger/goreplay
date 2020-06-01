// Gor is simple http traffic replication tool written in Go. Its main goal to replay traffic from production servers to staging and dev environments.
// Now you can test your code on real user sessions in an automated and repeatable fashion.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	_ "runtime/debug"
	"runtime/pprof"
	"syscall"
	"time"
)

var (
	mode       string
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile = flag.String("memprofile", "", "write memory profile to this file")
)

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rb, _ := httputil.DumpRequest(r, false)
		log.Println(string(rb))
		next.ServeHTTP(w, r)
	})
}

var closeCh chan int

func main() {
	closeCh = make(chan int)
	// // Don't exit on panic
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		fmt.Printf("PANIC: pkg: %v %s \n", r, debug.Stack())
	// 	}
	// }()

	// If not set via env cariable
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	}

	args := os.Args[1:]
	var plugins *InOutPlugins
	if len(args) > 0 && args[0] == "file-server" {
		if len(args) != 2 {
			log.Fatal("You should specify port and IP (optional) for the file server. Example: `gor file-server :80`")
		}
		dir, _ := os.Getwd()

		log.Println("Started example file server for current directory on address ", args[1])

		log.Fatal(http.ListenAndServe(args[1], loggingMiddleware(http.FileServer(http.Dir(dir)))))
	} else {
		flag.Parse()
		plugins = InitPlugins()
	}

	fmt.Println("Version:", VERSION)

	if len(plugins.Inputs) == 0 || len(plugins.Outputs) == 0 {
		log.Fatal("Required at least 1 input and 1 output")
	}

	if *memprofile != "" {
		profileMEM(*memprofile)
	}

	if *cpuprofile != "" {
		profileCPU(*cpuprofile)
	}

	if Settings.pprof != "" {
		go func() {
			log.Println(http.ListenAndServe(Settings.pprof, nil))
		}()
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		finalize(plugins)
		os.Exit(1)
	}()

	if Settings.exitAfter > 0 {
		log.Println("Running gor for a duration of", Settings.exitAfter)

		time.AfterFunc(Settings.exitAfter, func() {
			log.Println("Stopping gor after", Settings.exitAfter)
			close(closeCh)
		})
	}

	Start(plugins, closeCh)
}

func finalize(plugins *InOutPlugins) {
	for _, p := range plugins.All {
		if cp, ok := p.(io.Closer); ok {
			cp.Close()
		}
	}
}

func profileCPU(cpuprofile string) {
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)

		time.AfterFunc(30*time.Second, func() {
			pprof.StopCPUProfile()
			f.Close()
			log.Println("Stop profiling after 30 seconds")
		})
	}
}

func profileMEM(memprofile string) {
	if memprofile != "" {
		f, err := os.Create(memprofile)
		if err != nil {
			log.Fatal(err)
		}
		time.AfterFunc(30*time.Second, func() {
			pprof.WriteHeapProfile(f)
			f.Close()
		})
	}
}
