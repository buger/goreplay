// Gor is simple http traffic replication tool written in Go. Its main goal to replay traffic from production servers to staging and dev environments.
// Now you can test your code on real user sessions in an automated and repeatable fashion.
package main

import (
	"expvar"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	httppptof "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/buger/goreplay/pkg/binary"
	"github.com/buger/goreplay/pkg/dummy"
	"github.com/buger/goreplay/pkg/emitter"
	"github.com/buger/goreplay/pkg/file"
	gor_http "github.com/buger/goreplay/pkg/http"
	"github.com/buger/goreplay/pkg/kafka"
	"github.com/buger/goreplay/pkg/null"
	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/raw"
	"github.com/buger/goreplay/pkg/settings"
	"github.com/buger/goreplay/pkg/tcp"
	"github.com/buger/goreplay/pkg/ws"

	"github.com/rs/zerolog/log"
)

// Settings used for quick access to CLI flags
var Settings = settings.Settings

// NewPlugins specify and initialize all available plugins
func NewPlugins() *plugin.InOutPlugins {
	plugins := new(plugin.InOutPlugins)

	for _, options := range Settings.InputDummy {
		plugins.RegisterPlugin(dummy.NewDummyInput, options)
	}

	for range Settings.OutputDummy {
		plugins.RegisterPlugin(dummy.NewDummyOutput)
	}

	if Settings.OutputStdout {
		plugins.RegisterPlugin(dummy.NewDummyOutput)
	}

	if Settings.OutputNull {
		plugins.RegisterPlugin(null.NewNullOutput)
	}

	for _, options := range Settings.InputRAW {
		plugins.RegisterPlugin(raw.NewRAWInput, options, Settings.InputRAWConfig)
	}

	for _, options := range Settings.InputTCP {
		plugins.RegisterPlugin(tcp.NewTCPInput, options, &Settings.InputTCPConfig)
	}

	for _, options := range Settings.OutputTCP {
		plugins.RegisterPlugin(tcp.NewTCPOutput, options, &Settings.OutputTCPConfig)
	}

	for _, options := range Settings.OutputWebSocket {
		plugins.RegisterPlugin(ws.NewWebSocketOutput, options, &Settings.OutputWebSocketConfig)
	}

	for _, options := range Settings.InputFile {
		plugins.RegisterPlugin(file.NewFileInput, options, Settings.InputFileLoop, Settings.InputFileReadDepth, Settings.InputFileMaxWait, Settings.InputFileDryRun)
	}

	for _, path := range Settings.OutputFile {
		if strings.HasPrefix(path, "s3://") {
			plugins.RegisterPlugin(file.NewS3Output, path, &Settings.OutputFileConfig)
		} else {
			plugins.RegisterPlugin(file.NewFileOutput, path, &Settings.OutputFileConfig)
		}
	}

	for _, options := range Settings.InputHTTP {
		plugins.RegisterPlugin(gor_http.NewHTTPInput, options)
	}

	// If we explicitly set Host header http output should not rewrite it
	// Fix: https://github.com/buger/gor/issues/174
	for _, header := range Settings.ModifierConfig.Headers {
		if header.Name == "Host" {
			Settings.OutputHTTPConfig.OriginalHost = true
			break
		}
	}

	for _, options := range Settings.OutputHTTP {
		plugins.RegisterPlugin(gor_http.NewHTTPOutput, options, &Settings.OutputHTTPConfig)
	}

	for _, options := range Settings.OutputBinary {
		plugins.RegisterPlugin(binary.NewBinaryOutput, options, &Settings.OutputBinaryConfig)
	}

	if Settings.OutputKafkaConfig.Host != "" && Settings.OutputKafkaConfig.Topic != "" {
		plugins.RegisterPlugin(kafka.NewKafkaOutput, "", &Settings.OutputKafkaConfig, &Settings.KafkaTLSConfig)
	}

	if Settings.InputKafkaConfig.Host != "" && Settings.InputKafkaConfig.Topic != "" {
		plugins.RegisterPlugin(kafka.NewKafkaInput, "", &Settings.InputKafkaConfig, &Settings.KafkaTLSConfig)
	}

	return plugins
}

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile = flag.String("memprofile", "", "write memory profile to this file")
)

func init() {
	var defaultServeMux http.ServeMux
	http.DefaultServeMux = &defaultServeMux

	http.HandleFunc("/debug/vars", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintf(w, "{\n")
		first := true
		expvar.Do(func(kv expvar.KeyValue) {
			if kv.Key == "memstats" || kv.Key == "cmdline" {
				return
			}

			if !first {
				fmt.Fprintf(w, ",\n")
			}
			first = false
			fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
		})
		fmt.Fprintf(w, "\n}\n")
	})

	http.HandleFunc("/debug/pprof/", httppptof.Index)
	http.HandleFunc("/debug/pprof/cmdline", httppptof.Cmdline)
	http.HandleFunc("/debug/pprof/profile", httppptof.Profile)
	http.HandleFunc("/debug/pprof/symbol", httppptof.Symbol)
	http.HandleFunc("/debug/pprof/trace", httppptof.Trace)
}

func loggingMiddleware(addr string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/loop" {
			_, err := http.Get("http://" + addr)

			if err != nil {
				log.Error().Err(err).Msg("Error while calling loop endpoint")
			}
		}

		rb, _ := httputil.DumpRequest(r, false)
		log.Info().Msg(string(rb))
		next.ServeHTTP(w, r)
	})
}

func main() {
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	}

	args := os.Args[1:]
	var plugins *plugin.InOutPlugins
	if len(args) > 0 && args[0] == "file-server" {
		if len(args) != 2 {
			log.Fatal().Msg("You should specify port and IP (optional) for the file server. Example: `gor file-server :80`")
		}
		dir, _ := os.Getwd()

		log.Info().Msgf("Started example file server for current directory on address %s", args[1])

		if err := http.ListenAndServe(args[1], loggingMiddleware(args[1], http.FileServer(http.Dir(dir)))); err != nil {
			log.Fatal().Err(err).Msg("Failed to start file server")
		}
	} else {
		flag.Parse()
		settings.CheckSettings()
		plugins = NewPlugins()
	}

	log.Printf("[PPID %d and PID %d] Version:%s\n", os.Getppid(), os.Getpid(), settings.VERSION)

	if len(plugins.Inputs) == 0 || len(plugins.Outputs) == 0 {
		log.Fatal().Msg("Required at least 1 input and 1 output")
	}

	if *memprofile != "" {
		profileMEM(*memprofile)
	}

	if *cpuprofile != "" {
		profileCPU(*cpuprofile)
	}

	if settings.Settings.Pprof != "" {
		go func() {
			if err := http.ListenAndServe(settings.Settings.Pprof, nil); err != nil {
				log.Fatal().Err(err).Msg("Failed to start pprof server")
			}
		}()
	}

	closeCh := make(chan int)
	emitter := emitter.New(&settings.Settings.EmitterConfig)
	go emitter.Start(plugins)
	if settings.Settings.ExitAfter > 0 {
		log.Printf("Running gor for a duration of %s\n", settings.Settings.ExitAfter)

		time.AfterFunc(settings.Settings.ExitAfter, func() {
			log.Printf("gor run timeout %s\n", settings.Settings.ExitAfter)
			close(closeCh)
		})
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	exit := 0
	select {
	case <-c:
		exit = 1
	case <-closeCh:
		exit = 0
	}
	emitter.Close()
	os.Exit(exit)
}

func profileCPU(cpuprofile string) {
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create cpu profile file")
		}
		pprof.StartCPUProfile(f)

		time.AfterFunc(30*time.Second, func() {
			pprof.StopCPUProfile()
			f.Close()
		})
	}
}

func profileMEM(memprofile string) {
	if memprofile != "" {
		f, err := os.Create(memprofile)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create memory profile file")
		}
		time.AfterFunc(30*time.Second, func() {
			pprof.WriteHeapProfile(f)
			f.Close()
		})
	}
}
