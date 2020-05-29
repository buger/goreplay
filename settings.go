package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"
)

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
	verbose   bool
	debug     bool
	stats     bool
	exitAfter time.Duration

	pprof string

	splitOutput bool

	inputDummy   MultiOption
	outputDummy  MultiOption
	outputStdout bool
	outputNull   bool

	inputTCP        MultiOption
	inputTCPConfig  TCPInputConfig
	outputTCP       MultiOption
	outputTCPConfig TCPOutputConfig
	outputTCPStats  bool

	inputFile        MultiOption
	inputFileLoop    bool
	outputFile       MultiOption
	outputFileConfig FileOutputConfig

	inputRAW                MultiOption
	inputRAWEngine          string
	inputRAWTrackResponse   bool
	inputRAWRealIPHeader    string
	inputRAWExpire          time.Duration
	inputRAWBpfFilter       string
	inputRAWTimestampType   string
	copyBufferSize          int64
	inputRAWImmediateMode   bool
	inputRawBufferSize      int64
	inputRAWOverrideSnapLen bool

	middleware string

	inputHTTP  MultiOption
	outputHTTP MultiOption

	prettifyHTTP bool

	outputHTTPConfig HTTPOutputConfig
	modifierConfig   HTTPModifierConfig

	inputKafkaConfig  KafkaConfig
	outputKafkaConfig KafkaConfig
}

// Settings holds Gor configuration
var Settings AppSettings

func usage() {
	fmt.Printf("Gor is a simple http traffic replication tool written in Go. Its main goal is to replay traffic from production servers to staging and dev environments.\nProject page: https://github.com/buger/gor\nAuthor: <Leonid Bugaev> leonsbox@gmail.com\nCurrent Version: v%s\n\n", VERSION)
	flag.PrintDefaults()
	os.Exit(2)
}

func init() {
	flag.Usage = usage
	var (
		inputRawBufferSize, outputFileMaxSize, copyBufferSize, outputFileSize string
	)

	flag.StringVar(&Settings.pprof, "http-pprof", "", "Enable profiling. Starts  http server on specified port, exposing special /debug/pprof endpoint. Example: `:8181`")
	flag.BoolVar(&Settings.verbose, "verbose", false, "Turn on more verbose output")
	flag.BoolVar(&Settings.debug, "debug", false, "Turn on debug output, shows all intercepted traffic. Works only when with `verbose` flag")
	flag.BoolVar(&Settings.stats, "stats", false, "Turn on queue stats output")
	flag.DurationVar(&Settings.exitAfter, "exit-after", 0, "exit after specified duration")

	flag.BoolVar(&Settings.splitOutput, "split-output", false, "By default each output gets same traffic. If set to `true` it splits traffic equally among all outputs.")

	flag.Var(&Settings.inputDummy, "input-dummy", "Used for testing outputs. Emits 'Get /' request every 1s")
	flag.Var(&Settings.outputDummy, "output-dummy", "DEPRECATED: use --output-stdout instead")

	flag.BoolVar(&Settings.outputStdout, "output-stdout", false, "Used for testing inputs. Just prints to console data coming from inputs.")

	flag.BoolVar(&Settings.outputNull, "output-null", false, "Used for testing inputs. Drops all requests.")

	flag.Var(&Settings.inputTCP, "input-tcp", "Used for internal communication between Gor instances. Example: \n\t# Receive requests from other Gor instances on 28020 port, and redirect output to staging\n\tgor --input-tcp :28020 --output-http staging.com")
	flag.BoolVar(&Settings.inputTCPConfig.secure, "input-tcp-secure", false, "Turn on TLS security. Do not forget to specify certificate and key files.")
	flag.StringVar(&Settings.inputTCPConfig.certificatePath, "input-tcp-certificate", "", "Path to PEM encoded certificate file. Used when TLS turned on.")
	flag.StringVar(&Settings.inputTCPConfig.keyPath, "input-tcp-certificate-key", "", "Path to PEM encoded certificate key file. Used when TLS turned on.")

	flag.Var(&Settings.outputTCP, "output-tcp", "Used for internal communication between Gor instances. Example: \n\t# Listen for requests on 80 port and forward them to other Gor instance on 28020 port\n\tgor --input-raw :80 --output-tcp replay.local:28020")
	flag.BoolVar(&Settings.outputTCPConfig.secure, "output-tcp-secure", false, "Use TLS secure connection. --input-file on another end should have TLS turned on as well.")
	flag.BoolVar(&Settings.outputTCPConfig.sticky, "output-tcp-sticky", false, "Use Sticky connection. Request/Response with same ID will be sent to the same connection.")
	flag.BoolVar(&Settings.outputTCPStats, "output-tcp-stats", false, "Report TCP output queue stats to console every 5 seconds.")

	flag.Var(&Settings.inputFile, "input-file", "Read requests from file: \n\tgor --input-file ./requests.gor --output-http staging.com")
	flag.BoolVar(&Settings.inputFileLoop, "input-file-loop", false, "Loop input files, useful for performance testing.")

	flag.Var(&Settings.outputFile, "output-file", "Write incoming requests to file: \n\tgor --input-raw :80 --output-file ./requests.gor")
	flag.DurationVar(&Settings.outputFileConfig.flushInterval, "output-file-flush-interval", time.Second, "Interval for forcing buffer flush to the file, default: 1s.")
	flag.BoolVar(&Settings.outputFileConfig.append, "output-file-append", false, "The flushed chunk is appended to existence file or not. ")
	flag.StringVar(&outputFileSize, "output-file-size-limit", "32mb", "Size of each chunk. Default: 32mb")
	{
		n, err := bufferParser(outputFileSize, "32MB")
		if err != nil {
			log.Fatalf("output-file-size-limit error: %v\n", err)
		}
		Settings.outputFileConfig.sizeLimit = n
	}
	flag.IntVar(&Settings.outputFileConfig.queueLimit, "output-file-queue-limit", 256, "The length of the chunk queue. Default: 256")
	flag.StringVar(&outputFileMaxSize, "output-file-max-size-limit", "1TB", "Max size of output file, Default: 1TB")
	{
		n, err := bufferParser(outputFileMaxSize, "1TB")
		if err != nil {
			log.Fatalf("output-file-max-size-limit error: %v\n", err)
		}
		Settings.outputFileConfig.outputFileMaxSize = n
	}

	flag.BoolVar(&Settings.prettifyHTTP, "prettify-http", false, "If enabled, will automatically decode requests and responses with: Content-Encodning: gzip and Transfer-Encoding: chunked. Useful for debugging, in conjuction with --output-stdout")

	flag.Var(&Settings.inputRAW, "input-raw", "Capture traffic from given port (use RAW sockets and require *sudo* access):\n\t# Capture traffic from 8080 port\n\tgor --input-raw :8080 --output-http staging.com")

	flag.BoolVar(&Settings.inputRAWTrackResponse, "input-raw-track-response", false, "If turned on Gor will track responses in addition to requests, and they will be available to middleware and file output.")

	flag.StringVar(&Settings.inputRAWEngine, "input-raw-engine", "libpcap", "Intercept traffic using `libpcap` (default), and `raw_socket`")

	flag.StringVar(&Settings.inputRAWRealIPHeader, "input-raw-realip-header", "", "If not blank, injects header with given name and real IP value to the request payload. Usually this header should be named: X-Real-IP")

	flag.DurationVar(&Settings.inputRAWExpire, "input-raw-expire", time.Second*2, "How much it should wait for the last TCP packet, till consider that TCP message complete.")

	flag.StringVar(&Settings.inputRAWBpfFilter, "input-raw-bpf-filter", "", "BPF filter to write custom expressions. Can be useful in case of non standard network interfaces like tunneling or SPAN port. Example: --input-raw-bpf-filter 'dst port 80'")

	flag.StringVar(&Settings.inputRAWTimestampType, "input-raw-timestamp-type", "", "Possible values: PCAP_TSTAMP_HOST, PCAP_TSTAMP_HOST_LOWPREC, PCAP_TSTAMP_HOST_HIPREC, PCAP_TSTAMP_ADAPTER, PCAP_TSTAMP_ADAPTER_UNSYNCED. This values not supported on all systems, GoReplay will tell you available values of you put wrong one.")
	flag.StringVar(&copyBufferSize, "copy-buffer-size", "5mb", "Set the buffer size for an individual request (default 5MB)")
	{
		n, err := bufferParser(copyBufferSize, "5mb")
		if err != nil {
			log.Fatalf("copy-buffer-size error: %v\n", err)
		}
		Settings.copyBufferSize = n
	}
	flag.BoolVar(&Settings.inputRAWOverrideSnapLen, "input-raw-override-snaplen", false, "Override the capture snaplen to be 64k. Required for some Virtualized environments")
	flag.BoolVar(&Settings.inputRAWImmediateMode, "input-raw-immediate-mode", false, "Set pcap interface to immediate mode.")

	flag.StringVar(&inputRawBufferSize, "input-raw-buffer-size", "", "Controls size of the OS buffer which holds packets until they dispatched. Default value depends by system: in Linux around 2MB. If you see big package drop, increase this value.")
	{
		n, err := bufferParser(inputRawBufferSize, "0")
		if err != nil {
			log.Fatalf("input-raw-buffer-size error: %v\n", err)
		}
		Settings.inputRawBufferSize = n
	}

	flag.StringVar(&Settings.middleware, "middleware", "", "Used for modifying traffic using external command")

	// flag.Var(&Settings.inputHTTP, "input-http", "Read requests from HTTP, should be explicitly sent from your application:\n\t# Listen for http on 9000\n\tgor --input-http :9000 --output-http staging.com")

	flag.Var(&Settings.outputHTTP, "output-http", "Forwards incoming requests to given http address.\n\t# Redirect all incoming requests to staging.com address \n\tgor --input-raw :80 --output-http http://staging.com")
	flag.IntVar(&Settings.outputHTTPConfig.BufferSize, "output-http-response-buffer", 0, "HTTP response buffer size, all data after this size will be discarded.")
	flag.BoolVar(&Settings.outputHTTPConfig.CompatibilityMode, "output-http-compatibility-mode", false, "Use standard Go client, instead of built-in implementation. Can be slower, but more compatible.")

	flag.IntVar(&Settings.outputHTTPConfig.workersMin, "output-http-workers-min", 0, "Gor uses dynamic worker scaling. Enter a number to set a minimum number of workers. default = 1.")
	flag.IntVar(&Settings.outputHTTPConfig.workersMax, "output-http-workers", 0, "Gor uses dynamic worker scaling. Enter a number to set a maximum number of workers. default = 0 = unlimited.")
	flag.IntVar(&Settings.outputHTTPConfig.queueLen, "output-http-queue-len", 1000, "Number of requests that can be queued for output, if all workers are busy. default = 1000")

	flag.IntVar(&Settings.outputHTTPConfig.redirectLimit, "output-http-redirects", 0, "Enable how often redirects should be followed.")
	flag.DurationVar(&Settings.outputHTTPConfig.Timeout, "output-http-timeout", 5*time.Second, "Specify HTTP request/response timeout. By default 5s. Example: --output-http-timeout 30s")
	flag.BoolVar(&Settings.outputHTTPConfig.TrackResponses, "output-http-track-response", false, "If turned on, HTTP output responses will be set to all outputs like stdout, file and etc.")

	flag.BoolVar(&Settings.outputHTTPConfig.stats, "output-http-stats", false, "Report http output queue stats to console every N milliseconds. See output-http-stats-ms")
	flag.IntVar(&Settings.outputHTTPConfig.statsMs, "output-http-stats-ms", 5000, "Report http output queue stats to console every N milliseconds. default: 5000")
	flag.BoolVar(&Settings.outputHTTPConfig.OriginalHost, "http-original-host", false, "Normally gor replaces the Host http header with the host supplied with --output-http.  This option disables that behavior, preserving the original Host header.")
	flag.BoolVar(&Settings.outputHTTPConfig.Debug, "output-http-debug", false, "Enables http debug output.")

	flag.StringVar(&Settings.outputHTTPConfig.elasticSearch, "output-http-elasticsearch", "", "Send request and response stats to ElasticSearch:\n\tgor --input-raw :8080 --output-http staging.com --output-http-elasticsearch 'es_host:api_port/index_name'")

	flag.StringVar(&Settings.outputKafkaConfig.host, "output-kafka-host", "", "Read request and response stats from Kafka:\n\tgor --input-raw :8080 --output-kafka-host '192.168.0.1:9092,192.168.0.2:9092'")
	flag.StringVar(&Settings.outputKafkaConfig.topic, "output-kafka-topic", "", "Read request and response stats from Kafka:\n\tgor --input-raw :8080 --output-kafka-topic 'kafka-log'")
	flag.BoolVar(&Settings.outputKafkaConfig.useJSON, "output-kafka-json-format", false, "If turned on, it will serialize messages from GoReplay text format to JSON.")

	flag.StringVar(&Settings.inputKafkaConfig.host, "input-kafka-host", "", "Send request and response stats to Kafka:\n\tgor --output-stdout --input-kafka-host '192.168.0.1:9092,192.168.0.2:9092'")
	flag.StringVar(&Settings.inputKafkaConfig.topic, "input-kafka-topic", "", "Send request and response stats to Kafka:\n\tgor --output-stdout --input-kafka-topic 'kafka-log'")
	flag.BoolVar(&Settings.inputKafkaConfig.useJSON, "input-kafka-json-format", false, "If turned on, it will assume that messages coming in JSON format rather than  GoReplay text format.")

	flag.Var(&Settings.modifierConfig.headers, "http-set-header", "Inject additional headers to http reqest:\n\tgor --input-raw :8080 --output-http staging.com --http-set-header 'User-Agent: Gor'")
	flag.Var(&Settings.modifierConfig.headers, "output-http-header", "WARNING: `--output-http-header` DEPRECATED, use `--http-set-header` instead")

	flag.Var(&Settings.modifierConfig.headerRewrite, "http-rewrite-header", "Rewrite the request header based on a mapping:\n\tgor --input-raw :8080 --output-http staging.com --http-rewrite-header Host: (.*).example.com,$1.beta.example.com")

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

	flag.Var(&Settings.modifierConfig.headerNegativeFilters, "http-disallow-header", "A regexp to match a specific header against. Requests with matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-disallow-header \"User-Agent: Replayed by Gor\"")

	flag.Var(&Settings.modifierConfig.headerBasicAuthFilters, "http-basic-auth-filter", "A regexp to match the decoded basic auth string against. Requests with non-matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-basic-auth-filter \"^customer[0-9].*\"")

	flag.Var(&Settings.modifierConfig.headerHashFilters, "http-header-limiter", "Takes a fraction of requests, consistently taking or rejecting a request based on the FNV32-1A hash of a specific header:\n\t gor --input-raw :8080 --output-http staging.com --http-header-limiter user-id:25%")

	flag.Var(&Settings.modifierConfig.headerHashFilters, "output-http-header-hash-filter", "WARNING: `output-http-header-hash-filter` DEPRECATED, use `--http-header-hash-limiter` instead")

	flag.Var(&Settings.modifierConfig.paramHashFilters, "http-param-limiter", "Takes a fraction of requests, consistently taking or rejecting a request based on the FNV32-1A hash of a specific GET param:\n\t gor --input-raw :8080 --output-http staging.com --http-param-limiter user_id:25%")
}

var previousDebugTime = time.Now()
var debugMutex sync.Mutex
var pID = os.Getpid()

// Debug take an effect only if --verbose flag specified
func Debug(args ...interface{}) {
	if Settings.verbose {
		debugMutex.Lock()
		defer debugMutex.Unlock()
		now := time.Now()
		diff := now.Sub(previousDebugTime).String()
		previousDebugTime = now
		fmt.Printf("[DEBUG][PID %d][%s][elapsed %s] ", pID, now.Format(time.StampNano), diff)
		fmt.Println(args...)
	}
}

// bufferParser parses buffer to bytes from different bases and data units
// size is the buffer in string, rpl act as a replacement for empty buffer.
// e.g: (--output-file-size-limit "") may override default 32mb with empty buffer,
// which can be solved by setting rpl by bufferParser(buffer, "32mb")
func bufferParser(size, rpl string) (buffer int64, err error) {
	const (
		_ = 1 << (iota * 10)
		KB
		MB
		GB
		TB
	)

	var (
		// the following regexes follow Go semantics https://golang.org/ref/spec#Letters_and_digits
		rB   = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+$`)
		rKB  = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+kb$`)
		rMB  = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+mb$`)
		rGB  = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+gb$`)
		rTB  = regexp.MustCompile(`(?i)^(?:0b|0x|0o)?[\da-f_]+tb$`)
		empt = regexp.MustCompile(`^[\n\t\r 0.\f\a]*$`)

		lmt = len(size) - 2
		s   = []byte(size)
	)

	if empt.Match(s) {
		size = rpl
		s = []byte(size)
	}

	// recover, especially when buffer size overflows int64 i.e ~8019PBs
	defer func() {
		if e, ok := recover().(error); ok {
			err = e.(error)
		}
	}()

	switch {
	case rB.Match(s):
		buffer, err = strconv.ParseInt(size, 0, 64)
	case rKB.Match(s):
		buffer, err = strconv.ParseInt(size[:lmt], 0, 64)
		buffer *= KB
	case rMB.Match(s):
		buffer, err = strconv.ParseInt(size[:lmt], 0, 64)
		buffer *= MB
	case rGB.Match(s):
		buffer, err = strconv.ParseInt(size[:lmt], 0, 64)
		buffer *= GB
	case rTB.Match(s):
		buffer, err = strconv.ParseInt(size[:lmt], 0, 64)
		buffer *= TB
	default:
		return 0, fmt.Errorf("invalid buffer %q", size)
	}
	return
}
