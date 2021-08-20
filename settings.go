package main

import (
	"fmt"
	"github.com/spf13/pflag"
	"os"
	"sync"
	"time"
)

// DEMO indicates that goreplay is running in demo mode
var DEMO string

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

// Type is here so that MultiOption can implement pflag.StringSlice
func (h *MultiOption) Type() string {
	return "multiOption"
}

// AppSettings is the struct of main configuration
type ServiceSettings struct {
	Verbose   int           `json:"verbose"`
	Stats     bool          `json:"stats"`
	ExitAfter time.Duration `json:"exit-after" mapstructure:"exit-after"`

	SplitOutput          bool   `json:"split-output" mapstructure:"split-output"`
	RecognizeTCPSessions bool   `json:"recognize-tcp-sessions" mapstructure:"recognize-tcp-sessions"`
	Pprof                string `json:"http-pprof" mapstructure:"http-pprof"`

	InputDummy   MultiOption `json:"input-dummy" mapstructure:"input-dummy"`
	OutputDummy  MultiOption `json:"output-dummy" mapstructure:"output-dummy"`
	OutputStdout bool        `json:"output-stdout" mapstructure:"output-stdout"`
	OutputNull   bool        `json:"output-null" mapstructure:"output-null"`

	InputTCP        MultiOption     `json:"input-tcp" mapstructure:"input-tcp"`
	InputTCPConfig  TCPInputConfig  `mapstructure:",squash"`
	OutputTCP       MultiOption     `json:"output-tcp" mapstructure:"output-tcp"`
	OutputTCPConfig TCPOutputConfig `mapstructure:",squash"`
	OutputTCPStats  bool            `json:"output-tcp-stats" mapstructure:"output-tcp-stats"`

	InputFile          MultiOption      `json:"input-file" mapstructure:"input-file"`
	InputFileLoop      bool             `json:"input-file-loop" mapstructure:"input-file-loop"`
	InputFileReadDepth int              `json:"input-file-read-depth" mapstructure:"input-file-read-depth"`
	InputFileDryRun    bool             `json:"input-file-dry-run" mapstructure:"input-file-dry-run"`
	InputFileMaxWait   time.Duration    `json:"input-file-max-wait" mapstructure:"input-file-max-wait"`
	OutputFile         MultiOption      `json:"output-file" mapstructure:"output-file"`
	OutputFileConfig   FileOutputConfig `mapstructure:",squash"`

	InputRAW       MultiOption    `json:"input-raw" mapstructure:"input-raw"`
	InputRAWConfig RAWInputConfig `mapstructure:",squash"`

	Middleware string `json:"middleware"`

	InputHTTP    MultiOption `json:"input-http" mapstructure:"input-http"`
	OutputHTTP   MultiOption `json:"output-http" mapstructure:"output-http"`
	PrettifyHTTP bool        `json:"prettify-http" mapstructure:"prettify-http"`

	OutputHTTPConfig HTTPOutputConfig `mapstructure:",squash"`

	OutputBinary       MultiOption        `json:"output-binary" mapstructure:"output-binary"`
	OutputBinaryConfig BinaryOutputConfig `mapstructure:",squash"`

	ModifierConfig HTTPModifierConfig `mapstructure:",squash"`

	InputKafkaConfig  InputKafkaConfig  `mapstructure:",squash"`
	OutputKafkaConfig OutputKafkaConfig `mapstructure:",squash"`
	KafkaTLSConfig    KafkaTLSConfig    `mapstructure:",squash"`
}

type AppSettings struct {
	ServiceSettings `mapstructure:",squash"`
	Service         string `mapstructure:",service"`

	Services map[string]ServiceSettings `json:"services" mapstructure:"services"`
}

// New ServiceSettings
func NewServiceSettings() *ServiceSettings {
	return &ServiceSettings{
		InputDummy: make([]string, 0),
		OutputDummy: make([]string, 0),
		InputTCP: make([]string, 0),
		OutputTCP: make([]string, 0),
		InputFile: make([]string, 0),
		OutputFile: make([]string, 0),
		InputRAW: make([]string, 0),
		InputHTTP: make([]string, 0),
		OutputHTTP: make([]string, 0),
		OutputBinary: make([]string, 0),
	}
}

// New AppSettings
func NewAppSettings() *AppSettings {
	return &AppSettings{
		ServiceSettings: *NewServiceSettings(),
		Services : make(map[string]ServiceSettings, 0),
	}
}

// Settings holds Gor configuration
var Settings = *NewAppSettings()

func usage() {
	fmt.Printf("Gor is a simple http traffic replication tool written in Go. Its main goal is to replay traffic from production servers to staging and dev environments.\nProject page: https://github.com/buger/gor\nAuthor: <Leonid Bugaev> leonsbox@gmail.com\nCurrent Version: v%s\n\n", VERSION)
	pflag.PrintDefaults()
	os.Exit(2)
}

func init() {
	pflag.Usage = usage
	pflag.StringVar(&Settings.Pprof, "http-pprof", "", "Enable profiling. Starts  http server on specified port, " +
		"exposing special /debug/pprof endpoint. Example: `:8181`")
	pflag.IntVar(&Settings.Verbose, "verbose", 0, "set the level of verbosity, if greater than zero then it will turn on debug output")
	pflag.BoolVar(&Settings.Stats, "stats", false, "Turn on queue stats output")

	if DEMO == "" {
		pflag.DurationVar(&Settings.ExitAfter, "exit-after", 0, "exit after specified duration")
	} else {
		Settings.ExitAfter = 5 * time.Minute
	}

	pflag.StringVar(&Settings.Service, "service", "", "Identified of currently capture service. Can be used if you need to record and replay multi services at once.")

	pflag.BoolVar(&Settings.SplitOutput, "split-output", false, "By default each output gets same traffic. If set to `true` it splits traffic equally among all outputs.")
	pflag.BoolVar(&Settings.RecognizeTCPSessions, "recognize-tcp-sessions", false, "[PRO] If turned on http output will create separate worker for each TCP session. Splitting output will session based as well.")

	pflag.StringSlice("input-dummy", Settings.InputDummy,"Used for testing outputs. Emits 'Get /' request every 1s")
	pflag.StringSlice("output-dummy", Settings.OutputDummy,"Used for testing inputs. Emits 'Get /' request every 1s")
	pflag.BoolVar(&Settings.OutputStdout, "output-stdout", false, "Used for testing inputs. Just prints to console data coming from inputs.")
	pflag.BoolVar(&Settings.OutputNull, "output-null", false, "Used for testing inputs. Drops all requests.")

	pflag.StringSlice("input-tcp", Settings.InputTCP, "Used for internal communication between Gor instances. " +
		"Example: \n\t# Receive requests from other Gor instances on 28020 port, and redirect output to staging\n\tgor --input-tcp :28020 --output-http staging.com")
	pflag.BoolVar(&Settings.InputTCPConfig.Secure, "input-tcp-secure", false, "Turn on TLS security. Do not forget to specify certificate and key files.")
	pflag.StringVar(&Settings.InputTCPConfig.CertificatePath, "input-tcp-certificate", "", "Path to PEM encoded certificate file. Used when TLS turned on.")
	pflag.StringVar(&Settings.InputTCPConfig.KeyPath, "input-tcp-certificate-key", "", "Path to PEM encoded certificate key file. Used when TLS turned on.")

	pflag.StringSlice("output-tcp", Settings.OutputTCP,  "Used for internal communication between Gor instances. " +
		"Example: \n\t# Listen for requests on 80 port and forward them to other Gor instance on 28020 port\n\tgor --input-raw :80 --output-tcp replay.local:28020")
	pflag.BoolVar(&Settings.OutputTCPConfig.Secure, "output-tcp-secure", false, "Use TLS secure connection. --input-file on another end should have TLS turned on as well.")
	pflag.BoolVar(&Settings.OutputTCPConfig.SkipVerify, "output-tcp-skip-verify", false, "Don't verify hostname on TLS secure connection.")
	pflag.BoolVar(&Settings.OutputTCPConfig.Sticky, "output-tcp-sticky", false, "Use Sticky connection. Request/Response with same ID will be sent to the same connection.")
	pflag.IntVar(&Settings.OutputTCPConfig.Workers, "output-tcp-workers", 10, "Number of parallel tcp connections, default is 10")
	pflag.BoolVar(&Settings.OutputTCPStats, "output-tcp-stats", false, "Report TCP output queue stats to console every 5 seconds.")

	pflag.StringSlice("input-file", Settings.InputFile, "Read requests from file: \n\tgor --input-file ./requests." +
		"gor --output-http staging.com")
	pflag.BoolVar(&Settings.InputFileLoop, "input-file-loop", false, "Loop input files, useful for performance testing.")
	pflag.IntVar(&Settings.InputFileReadDepth, "input-file-read-depth", 100,
		"GoReplay tries to read and cache multiple records, in advance. In parallel it also perform sorting of requests, if they came out of order. Since it needs hold this buffer in memory, bigger values can cause worse performance")
	pflag.BoolVar(&Settings.InputFileDryRun, "input-file-dry-run", false, "Simulate reading from the data source without replaying it. You will get information about expected replay time, number of found records etc.")
	pflag.DurationVar(&Settings.InputFileMaxWait, "input-file-max-wait", 0, "Set the maximum time between requests. Can help in situations when you have too long periods between request, and you want to skip them. Example: --input-raw-max-wait 1s")

	pflag.StringSlice( "output-file", Settings.OutputFile,
		"Write incoming requests to file: \n\tgor --input-raw :80 --output-file ./requests.gor")
	pflag.DurationVar(&Settings.OutputFileConfig.FlushInterval, "output-file-flush-interval", time.Second, "Interval for forcing buffer flush to the file, default: 1s.")
	pflag.BoolVar(&Settings.OutputFileConfig.Append, "output-file-append", false, "The flushed chunk is appended to existence file or not. ")
	pflag.Var(&Settings.OutputFileConfig.SizeLimit, "output-file-size-limit", "Size of each chunk. Default: 32mb")
	pflag.IntVar(&Settings.OutputFileConfig.QueueLimit, "output-file-queue-limit", 256, "The length of the chunk queue. Default: 256")
	pflag.Var(&Settings.OutputFileConfig.OutputFileMaxSize, "output-file-max-size-limit", "Max size of output file, Default: 1TB")

	pflag.StringVar(&Settings.OutputFileConfig.BufferPath, "output-file-buffer", "/tmp", "The path for temporary storing current buffer: \n\tgor --input-raw :80 --output-file s3://mybucket/logs/%Y-%m-%d.gz --output-file-buffer /mnt/logs")

	pflag.BoolVar(&Settings.PrettifyHTTP, "prettify-http", false, "If enabled, will automatically decode requests and responses with: Content-Encoding: gzip and Transfer-Encoding: chunked. Useful for debugging, in conjunction with --output-stdout")

	// input raw flags
	pflag.StringSlice("input-raw", Settings.InputRAW,"Capture traffic from given port (" +
		"use RAW sockets and require *sudo* access):\n\t# Capture traffic from 8080 port\n\tgor --input-raw :8080 --output-http staging.com")
	pflag.BoolVar(&Settings.InputRAWConfig.TrackResponse, "input-raw-track-response", false, "If turned on Gor will track responses in addition to requests, and they will be available to middleware and file output.")
	pflag.Var(&Settings.InputRAWConfig.Engine, "input-raw-engine", "Intercept traffic using `libpcap` (default), `raw_socket` or `pcap_file`")
	pflag.Var(&Settings.InputRAWConfig.Protocol, "input-raw-protocol", "Specify application protocol of intercepted traffic. Possible values: http, binary")
	pflag.StringVar(&Settings.InputRAWConfig.RealIPHeader, "input-raw-realip-header", "", "If not blank, injects header with given name and real IP value to the request payload. Usually this header should be named: X-Real-IP")
	pflag.DurationVar(&Settings.InputRAWConfig.Expire, "input-raw-expire", time.Second*2, "How much it should wait for the last TCP packet, till consider that TCP message complete. Default: 2s")
	pflag.StringVar(&Settings.InputRAWConfig.BPFFilter, "input-raw-bpf-filter", "", "BPF filter to write custom expressions. Can be useful in case of non standard network interfaces like tunneling or SPAN port. Example: --input-raw-bpf-filter 'dst port 80'")
	pflag.StringVar(&Settings.InputRAWConfig.TimestampType, "input-raw-timestamp-type", "", "Possible values: PCAP_TSTAMP_HOST, PCAP_TSTAMP_HOST_LOWPREC, PCAP_TSTAMP_HOST_HIPREC, PCAP_TSTAMP_ADAPTER, PCAP_TSTAMP_ADAPTER_UNSYNCED. This values not supported on all systems, GoReplay will tell you available values of you put wrong one.")
	pflag.Var(&Settings.InputRAWConfig.CopyBufferSize, "input-row-copy-buffer-size", "Set the buffer size for an individual request (default 5MB)")
	pflag.BoolVar(&Settings.InputRAWConfig.Snaplen, "input-raw-override-snaplen", false, "Override the capture snaplen to be 64k. Required for some Virtualized environments")
	pflag.DurationVar(&Settings.InputRAWConfig.BufferTimeout, "input-raw-buffer-timeout", 0, "set the pcap timeout. for immediate mode don't set this pflag")
	pflag.Var(&Settings.InputRAWConfig.BufferSize, "input-raw-buffer-size", "Controls size of the OS buffer which holds packets until they dispatched. Default value depends by system: in Linux around 2MB. If you see big package drop, increase this value.")
	pflag.BoolVar(&Settings.InputRAWConfig.Promiscuous, "input-raw-promisc", false, "enable promiscuous mode")
	pflag.BoolVar(&Settings.InputRAWConfig.Monitor, "input-raw-monitor", false, "enable RF monitor mode")
	pflag.BoolVar(&Settings.InputRAWConfig.Stats, "input-raw-stats", false, "enable stats generator on raw TCP messages")
	pflag.BoolVar(&Settings.InputRAWConfig.AllowIncomplete, "input-raw-allow-incomplete", false, "If turned on Gor will record HTTP messages with missing packets")

	pflag.StringVar(&Settings.Middleware, "middleware", "", "Used for modifying traffic using external command")

	pflag.StringSlice("input-http", Settings.InputHTTP, "Used for internal communication between Gor instances. " +
		"Example: \n\t# Receive requests from other Gor instances on 80 port, " +
		"and redirect output to staging\n\tgor --input-http :80 --output-http staging.com")
	pflag.StringSlice( "output-http", Settings.OutputHTTP, "Forwards incoming requests to given http address." +
		"\n\t# Redirect all incoming requests to staging.com address \n\tgor --input-raw :80 --output-http http://staging.com")

	/* outputHTTPConfig */
	pflag.Var(&Settings.OutputHTTPConfig.BufferSize, "output-http-response-buffer", "HTTP response buffer size, all data after this size will be discarded.")
	pflag.IntVar(&Settings.OutputHTTPConfig.WorkersMin, "output-http-workers-min", 0,
		"Gor uses dynamic worker scaling. Enter a number to set a minimum number of workers. default = 1.")
	pflag.IntVar(&Settings.OutputHTTPConfig.WorkersMax, "output-http-workers", 0,
		"Gor uses dynamic worker scaling. Enter a number to set a maximum number of workers. default = 0 = unlimited.")
	pflag.IntVar(&Settings.OutputHTTPConfig.QueueLen, "output-http-queue-len", 1000, "Number of requests that can be queued for output, if all workers are busy. default = 1000")
	pflag.BoolVar(&Settings.OutputHTTPConfig.SkipVerify, "output-http-skip-verify", false, "Don't verify hostname on TLS secure connection.")
	pflag.DurationVar(&Settings.OutputHTTPConfig.WorkerTimeout, "output-http-worker-timeout", 2*time.Second, "Duration to rollback idle workers.")

	pflag.IntVar(&Settings.OutputHTTPConfig.RedirectLimit, "output-http-redirects", 0, "Enable how often redirects should be followed.")
	pflag.DurationVar(&Settings.OutputHTTPConfig.Timeout, "output-http-timeout", 5*time.Second, "Specify HTTP request/response timeout. By default 5s. Example: --output-http-timeout 30s")
	pflag.BoolVar(&Settings.OutputHTTPConfig.TrackResponses, "output-http-track-response", false, "If turned on, HTTP output responses will be set to all outputs like stdout, file and etc.")

	pflag.BoolVar(&Settings.OutputHTTPConfig.Stats, "output-http-stats", false, "Report http output queue stats to console every N milliseconds. See output-http-stats-ms")
	pflag.IntVar(&Settings.OutputHTTPConfig.StatsMs, "output-http-stats-ms", 5000, "Report http output queue stats to console every N milliseconds. default: 5000")
	pflag.BoolVar(&Settings.OutputHTTPConfig.OriginalHost, "http-original-host", false, "Normally gor replaces the Host http header with the host supplied with --output-http.  This option disables that behavior, preserving the original Host header.")
	pflag.StringVar(&Settings.OutputHTTPConfig.ElasticSearch, "output-http-elasticsearch", "", "Send request and response stats to ElasticSearch:\n\tgor --input-raw :8080 --output-http staging.com --output-http-elasticsearch 'es_host:api_port/index_name'")
	/* outputHTTPConfig */

	pflag.StringSlice( "output-binary", Settings.OutputBinary, "Forwards incoming binary payloads to given address." +
		"\n\t# Redirect all incoming requests to staging.com address \n\tgor --input-raw :80 --input-raw-protocol binary --output-binary staging.com:80")

	/* outputBinaryConfig */
	pflag.Var(&Settings.OutputBinaryConfig.BufferSize, "output-tcp-response-buffer", "TCP response buffer size, all data after this size will be discarded.")
	pflag.IntVar(&Settings.OutputBinaryConfig.Workers, "output-binary-workers", 0, "Gor uses dynamic worker scaling by default.  Enter a number to run a set number of workers.")
	pflag.DurationVar(&Settings.OutputBinaryConfig.Timeout, "output-binary-timeout", 0, "Specify HTTP request/response timeout. By default 5s. Example: --output-binary-timeout 30s")
	pflag.BoolVar(&Settings.OutputBinaryConfig.TrackResponses, "output-binary-track-response", false, "If turned on, Binary output responses will be set to all outputs like stdout, file and etc.")

	pflag.BoolVar(&Settings.OutputBinaryConfig.Debug, "output-binary-debug", false, "Enables binary debug output.")
	/* outputBinaryConfig */

	pflag.StringVar(&Settings.OutputKafkaConfig.Host, "output-kafka-host", "", "Read request and response stats from Kafka:\n\tgor --input-raw :8080 --output-kafka-host '192.168.0.1:9092,192.168.0.2:9092'")
	pflag.StringVar(&Settings.OutputKafkaConfig.Topic, "output-kafka-topic", "", "Read request and response stats from Kafka:\n\tgor --input-raw :8080 --output-kafka-topic 'kafka-log'")
	pflag.BoolVar(&Settings.OutputKafkaConfig.UseJSON, "output-kafka-json-format", false, "If turned on, it will serialize messages from GoReplay text format to JSON.")

	pflag.StringVar(&Settings.InputKafkaConfig.Host, "input-kafka-host", "", "Send request and response stats to Kafka:\n\tgor --output-stdout --input-kafka-host '192.168.0.1:9092,192.168.0.2:9092'")
	pflag.StringVar(&Settings.InputKafkaConfig.Topic, "input-kafka-topic", "", "Send request and response stats to Kafka:\n\tgor --output-stdout --input-kafka-topic 'kafka-log'")
	pflag.BoolVar(&Settings.InputKafkaConfig.UseJSON, "input-kafka-json-format", false, "If turned on, it will assume that messages coming in JSON format rather than  GoReplay text format.")

	pflag.StringVar(&Settings.KafkaTLSConfig.CACert, "kafka-tls-ca-cert", "", "CA certificate for Kafka TLS Config:\n\tgor  --input-raw :3000 --output-kafka-host '192.168.0.1:9092' --output-kafka-topic 'topic' --kafka-tls-ca-cert cacert.cer.pem --kafka-tls-client-cert client.cer.pem --kafka-tls-client-key client.key.pem")
	pflag.StringVar(&Settings.KafkaTLSConfig.ClientCert, "kafka-tls-client-cert", "", "Client certificate for Kafka TLS Config (mandatory with to kafka-tls-ca-cert and kafka-tls-client-key)")
	pflag.StringVar(&Settings.KafkaTLSConfig.ClientKey, "kafka-tls-client-key", "", "Client Key for Kafka TLS Config (mandatory with to kafka-tls-client-cert and kafka-tls-client-key)")


	pflag.Var(&Settings.ModifierConfig.Headers, "http-set-header",
		"Inject additional headers to http request:\n\tgor --input-raw :8080 --output-http staging.com --http-set-header 'User-Agent: Gor'")
	pflag.Var(&Settings.ModifierConfig.HeaderRewrite, "http-rewrite-header", "Rewrite the request header based on a mapping:\n\tgor --input-raw :8080 --output-http staging.com --http-rewrite-header Host: (.*).example.com,$1.beta.example.com")
	pflag.Var(&Settings.ModifierConfig.Params, "http-set-param", "Set request url param, if param already exists it will be overwritten:\n\tgor --input-raw :8080 --output-http staging.com --http-set-param api_key=1")
	pflag.Var(&Settings.ModifierConfig.Methods, "http-allow-method", "Whitelist of HTTP methods to replay. Anything else will be dropped:\n\tgor --input-raw :8080 --output-http staging.com --http-allow-method GET --http-allow-method OPTIONS")
	pflag.Var(&Settings.ModifierConfig.URLRegexp, "http-allow-url", "A regexp to match requests against. Filter get matched against full url with domain. Anything else will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-allow-url ^www.")
	pflag.Var(&Settings.ModifierConfig.URLNegativeRegexp, "http-disallow-url", "A regexp to match requests against. Filter get matched against full url with domain. Anything else will be forwarded:\n\t gor --input-raw :8080 --output-http staging.com --http-disallow-url ^www.")
	pflag.Var(&Settings.ModifierConfig.URLRewrite, "http-rewrite-url", "Rewrite the request url based on a mapping:\n\tgor --input-raw :8080 --output-http staging.com --http-rewrite-url /v1/user/([^\\/]+)/ping:/v2/user/$1/ping")
	pflag.Var(&Settings.ModifierConfig.HeaderFilters, "http-allow-header", "A regexp to match a specific header against. Requests with non-matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-allow-header api-version:^v1")
	pflag.Var(&Settings.ModifierConfig.HeaderNegativeFilters, "http-disallow-header", "A regexp to match a specific header against. Requests with matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-disallow-header \"User-Agent: Replayed by Gor\"")
	pflag.Var(&Settings.ModifierConfig.HeaderBasicAuthFilters, "http-basic-auth-filter", "A regexp to match the decoded basic auth string against. Requests with non-matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-basic-auth-filter \"^customer[0-9].*\"")
	pflag.Var(&Settings.ModifierConfig.HeaderHashFilters, "http-header-limiter", "Takes a fraction of requests, consistently taking or rejecting a request based on the FNV32-1A hash of a specific header:\n\t gor --input-raw :8080 --output-http staging.com --http-header-limiter user-id:25%")
	pflag.Var(&Settings.ModifierConfig.ParamHashFilters, "http-param-limiter", "Takes a fraction of requests, consistently taking or rejecting a request based on the FNV32-1A hash of a specific GET param:\n\t gor --input-raw :8080 --output-http staging.com --http-param-limiter user_id:25%")

	// default values, using for tests
	Settings.OutputFileConfig.SizeLimit = 33554432
	Settings.OutputFileConfig.OutputFileMaxSize = 1099511627776
	Settings.InputRAWConfig.CopyBufferSize = 5242880

}

func checkSettings() {
	if Settings.OutputFileConfig.SizeLimit < 1 {
		Settings.OutputFileConfig.SizeLimit.Set("32mb")
	}
	if Settings.OutputFileConfig.OutputFileMaxSize < 1 {
		Settings.OutputFileConfig.OutputFileMaxSize.Set("1tb")
	}
	if Settings.InputRAWConfig.CopyBufferSize < 1 {
		Settings.InputRAWConfig.CopyBufferSize.Set("5mb")
	}
}

var previousDebugTime = time.Now()
var debugMutex sync.Mutex

// Debug take an effect only if --verbose greater than 0 is specified
func Debug(level int, args ...interface{}) {
	if Settings.Verbose >= level {
		debugMutex.Lock()
		defer debugMutex.Unlock()
		now := time.Now()
		diff := now.Sub(previousDebugTime)
		previousDebugTime = now
		fmt.Fprintf(os.Stderr, "[DEBUG][elapsed %s]: ", diff)
		fmt.Fprintln(os.Stderr, args...)
	}
}
