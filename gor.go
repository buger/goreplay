// Gor is simple http traffic replication tool written in Go. Its main goal to replay traffic from production servers to staging and dev environments.
// Now you can test your code on real user sessions in an automated and repeatable fashion.
package main

import (
	"bytes"
	"expvar"
	"flag"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"log"
	"net/http"
	"net/http/httputil"
	httppptof "net/http/pprof"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

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
			log.Println(err)
		}

		rb, _ := httputil.DumpRequest(r, false)
		log.Println(string(rb))
		next.ServeHTTP(w, r)
	})
}

type FlagSetter interface {
	Set(string) error
}

func MultiOptionDecoder(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	// fmt.Printf("%v %v %#v \n", f, t, data)

	val := reflect.New(t).Interface()

	if fs, ok := val.(FlagSetter); ok {
		if reflect.TypeOf(data).Kind() == reflect.Slice {
			s := reflect.ValueOf(data)
			for i := 0; i < s.Len(); i++ {
				v := fmt.Sprintf("%v", s.Index(i).Interface())
				if v == "[]" {
					continue
				}

				fs.Set(v)
			}
		} else {
			fs.Set(fmt.Sprintf("%v", data))
		}
		return val, nil
	}

	return data, nil
}

func loadConfig(rawConfig []byte) {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	viper.SetConfigName("config") // config file name without extension
	viper.SetConfigType("yaml")

	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/goreplay/")
	viper.AddConfigPath("$HOME/.goreplay")

	viper.SetEnvPrefix("GR")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	var err error
	// Used for tests
	if len(rawConfig) > 0 {
		err := viper.ReadConfig(bytes.NewBuffer(rawConfig))
		if err != nil {
			log.Fatal("Error loading config:", err)
		}
	} else {
		// Error can happen if file not found
		err = viper.ReadInConfig()
		if err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				log.Fatal("Error loading config:", err)
			}
		}
	}

	err = viper.Unmarshal(&Settings, func(cfg *mapstructure.DecoderConfig) {
		cfg.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			cfg.DecodeHook,
			MultiOptionDecoder,
		)
	})

	if err != nil {
		log.Fatal("Error loading config:", err)
	}
}

func main() {
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	}

	args := os.Args[1:]
	var plugins *InOutPlugins
	if len(args) > 0 && args[0] == "file-server" {
		if len(args) != 2 {
			log.Fatal("You should specify port and IP (optional) for the file server. Example: `gor file-server :80`")
		}
		dir, _ := os.Getwd()

		Debug(0, "Started example file server for current directory on address ", args[1])

		log.Fatal(http.ListenAndServe(args[1], loggingMiddleware(args[1], http.FileServer(http.Dir(dir)))))
		return
	} else {
		// viper.WatchConfig()
		loadConfig(nil)

		plugins = NewPlugins("", Settings.ServiceSettings, nil)
		if len(Settings.Services) > 0 {
			for service, config := range Settings.Services {
				NewPlugins(service, config, plugins)
			}
		}
	}

	log.Printf("[PPID %d and PID %d] Version:%s\n", os.Getppid(), os.Getpid(), VERSION)

	if len(plugins.Inputs) == 0 || len(plugins.Outputs) == 0 || len(plugins.All) < 2 {
		log.Fatal("Required at least 1 input and 1 output, and 2 all")
	}

	if *memprofile != "" {
		profileMEM(*memprofile)
	}

	if *cpuprofile != "" {
		profileCPU(*cpuprofile)
	}

	if Settings.Pprof != "" {
		go func() {
			log.Println(http.ListenAndServe(Settings.Pprof, nil))
		}()
	}

	closeCh := make(chan int)
	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)
	if Settings.ExitAfter > 0 {
		log.Printf("Running gor for a duration of %s\n", Settings.ExitAfter)

		time.AfterFunc(Settings.ExitAfter, func() {
			log.Printf("gor run timeout %s\n", Settings.ExitAfter)
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
			log.Fatal(err)
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
			log.Fatal(err)
		}
		time.AfterFunc(30*time.Second, func() {
			pprof.WriteHeapProfile(f)
			f.Close()
		})
	}
}
