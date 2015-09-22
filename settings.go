package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"
)

var VERSION string

// MultiOption allows to specify multiple flags with same name and collects all values into array
type MultiOption []string

func (h *MultiOption) String() string {
	return fmt.Sprint(*h)
}

// Set gets called multiple times for each flag with same name
func (h *MultiOption) Set(value string) error {
	*h = append(*h, value)
	return nil
}

// AppSettings is the struct of main configuration
type AppSettings struct {
	verbose bool
	debug   bool
	stats   bool

	splitOutput bool

	inputDummy  MultiOption
	outputDummy MultiOption

	inputTCP       MultiOption
	outputTCP      MultiOption
	outputTCPStats bool

	inputFile  MultiOption
	outputFile MultiOption

	inputRAW MultiOption

	middleware MultiOption

	inputHTTP  MultiOption
	outputHTTP MultiOption

	outputHTTPConfig HTTPOutputConfig
	modifierConfig   HTTPModifierConfig
}

// Settings holds Gor configuration
var Settings AppSettings

func usage() {
	fmt.Printf("Gor is a simple http traffic replication tool written in Go. Its main goal is to replay traffic from production servers to staging and dev environments.\nProject page: https://github.com/buger/gor\nAuthor: <Leonid Bugaev> leonsbox@gmail.com\nCurrent Version: %s\n\n", VERSION)
	flag.PrintDefaults()
	os.Exit(2)
}

func init() {
	flag.Usage = usage

	flag.BoolVar(&Settings.verbose, "verbose", false, "Turn on more verbose output")
	flag.BoolVar(&Settings.debug, "debug", false, "Turn on debug output, shows all itercepted traffic. Works only when with `verbose` flag")
	flag.BoolVar(&Settings.stats, "stats", false, "Turn on queue stats output")

	flag.BoolVar(&Settings.splitOutput, "split-output", false, "By default each output gets same traffic. If set to `true` it splits traffic equally among all outputs.")

	flag.Var(&Settings.inputDummy, "input-dummy", "Used for testing outputs. Emits 'Get /' request every 1s")
	flag.Var(&Settings.outputDummy, "output-dummy", "Used for testing inputs. Just prints data coming from inputs.")

	flag.Var(&Settings.inputTCP, "input-tcp", "Used for internal communication between Gor instances. Example: \n\t# Receive requests from other Gor instances on 28020 port, and redirect output to staging\n\tgor --input-tcp :28020 --output-http staging.com")
	flag.Var(&Settings.outputTCP, "output-tcp", "Used for internal communication between Gor instances. Example: \n\t# Listen for requests on 80 port and forward them to other Gor instance on 28020 port\n\tgor --input-raw :80 --output-tcp replay.local:28020")
	flag.BoolVar(&Settings.outputTCPStats, "output-tcp-stats", false, "Report TCP output queue stats to console every 5 seconds.")

	flag.Var(&Settings.inputFile, "input-file", "Read requests from file: \n\tgor --input-file ./requests.gor --output-http staging.com")
	flag.Var(&Settings.outputFile, "output-file", "Write incoming requests to file: \n\tgor --input-raw :80 --output-file ./requests.gor")

	flag.Var(&Settings.inputRAW, "input-raw", "Capture traffic from given port (use RAW sockets and require *sudo* access):\n\t# Capture traffic from 8080 port\n\tgor --input-raw :8080 --output-http staging.com")

	flag.Var(&Settings.middleware, "middleware", "Used for modifying traffic using external command")

	flag.Var(&Settings.inputHTTP, "input-http", "Read requests from HTTP, should be explicitly sent from your application:\n\t# Listen for http on 9000\n\tgor --input-http :9000 --output-http staging.com")

	flag.Var(&Settings.outputHTTP, "output-http", "Forwards incoming requests to given http address.\n\t# Redirect all incoming requests to staging.com address \n\tgor --input-raw :80 --output-http http://staging.com")
	flag.IntVar(&Settings.outputHTTPConfig.workers, "output-http-workers", 0, "Gor uses dynamic worker scaling by default.  Enter a number to run a set number of workers.")
	flag.IntVar(&Settings.outputHTTPConfig.redirectLimit, "output-http-redirects", 0, "Enable how often redirects should be followed.")
	flag.DurationVar(&Settings.outputHTTPConfig.Timeout, "output-http-timeout", 0, "Specify HTTP request/response timeout. By default 5s. Example: --output-http-timeout 30s")

	flag.BoolVar(&Settings.outputHTTPConfig.stats, "output-http-stats", false, "Report http output queue stats to console every 5 seconds.")
	flag.BoolVar(&Settings.outputHTTPConfig.OriginalHost, "http-original-host", false, "Normally gor replaces the Host http header with the host supplied with --output-http.  This option disables that behavior, preserving the original Host header.")

	flag.StringVar(&Settings.outputHTTPConfig.elasticSearch, "output-http-elasticsearch", "", "Send request and response stats to ElasticSearch:\n\tgor --input-raw :8080 --output-http staging.com --output-http-elasticsearch 'es_host:api_port/index_name'")

	flag.Var(&Settings.modifierConfig.headers, "http-set-header", "Inject additional headers to http reqest:\n\tgor --input-raw :8080 --output-http staging.com --http-set-header 'User-Agent: Gor'")
	flag.Var(&Settings.modifierConfig.headers, "output-http-header", "WARNING: `--output-http-header` DEPRECATED, use `--http-set-header` instead")

	flag.Var(&Settings.modifierConfig.params, "http-set-param", "Set request url param, if param already exists it will be overwritten:\n\tgor --input-raw :8080 --output-http staging.com --http-set-param api_key=1")

	flag.Var(&Settings.modifierConfig.methods, "http-allow-method", "Whitelist of HTTP methods to replay. Anything else will be dropped:\n\tgor --input-raw :8080 --output-http staging.com --http-allow-method GET --http-allow-method OPTIONS")
	flag.Var(&Settings.modifierConfig.methods, "output-http-method", "WARNING: `--output-http-method` DEPRECATED, use `--http-allow-method` instead")

	flag.Var(&Settings.modifierConfig.urlRegexp, "http-allow-url", "A regexp to match requests against. Filter get matched against full url with domain. Anything else will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-allow-url ^www.")
	flag.Var(&Settings.modifierConfig.urlRegexp, "output-http-url-regexp", "WARNING: `--output-http-url-regexp` DEPRECATED, use `--http-allow-url` instead")

	flag.Var(&Settings.modifierConfig.urlNegativeRegexp, "http-disallow-url", "A regexp to match requests against. Filter get matched against full url with domain. Anything else will be forwarded:\n\t gor --input-raw :8080 --output-http staging.com --http-disallow-url ^www.")

	flag.Var(&Settings.modifierConfig.urlRewrite, "http-rewrite-url", "Rewrite the request url based on a mapping:\n\tgor --input-raw :8080 --output-http staging.com --http-rewrite-url /v1/user/([^\\/]+)/ping:/v2/user/$1/ping")
	flag.Var(&Settings.modifierConfig.urlRewrite, "output-http-rewrite-url", "WARNING: `--output-http-rewrite-url` DEPRECATED, use `--http-rewrite-url` instead")

	flag.Var(&Settings.modifierConfig.headerFilters, "http-allow-header", "A regexp to match a specific header against. Requests with non-matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-allow-header api-version:^v1")
	flag.Var(&Settings.modifierConfig.headerFilters, "output-http-header-filter", "WARNING: `--output-http-header-filter` DEPRECATED, use `--http-allow-header` instead")

	flag.Var(&Settings.modifierConfig.headerFilters, "http-disallow-header", "A regexp to match a specific header against. Requests with matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-disallow-header \"User-Agent: Replayed by Gor\"")

	flag.Var(&Settings.modifierConfig.headerHashFilters, "http-header-limiter", "Takes a fraction of requests, consistently taking or rejecting a request based on the FNV32-1A hash of a specific header:\n\t gor --input-raw :8080 --output-http staging.com --http-header-imiter user-id:25%")
	flag.Var(&Settings.modifierConfig.headerHashFilters, "output-http-header-hash-filter", "WARNING: `output-http-header-hash-filter` DEPRECATED, use `--http-header-hash-limiter` instead")

	flag.Var(&Settings.modifierConfig.paramHashFilters, "http-param-limiter", "Takes a fraction of requests, consistently taking or rejecting a request based on the FNV32-1A hash of a specific GET param:\n\t gor --input-raw :8080 --output-http staging.com --http-param-limiter user_id:25%")
}

var previousDebugTime int64
var debugMutex sync.Mutex

// Debug gets called only if --verbose flag specified
func Debug(args ...interface{}) {
	if Settings.verbose {
		debugMutex.Lock()
		now := time.Now()
		diff := float64(now.UnixNano()-previousDebugTime) / 1000000
		previousDebugTime = now.UnixNano()
		debugMutex.Unlock()

		fmt.Printf("[DEBUG][PID %d][%d][%fms] ", os.Getpid(), now.UnixNano(), diff)
		fmt.Println(args...)
	}
}

func stringLimit(buf []byte) string {
	if len(buf) > 500 {
		return string(buf[:500])
	}

	return string(buf)
}
