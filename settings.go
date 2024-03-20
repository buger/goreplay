package goreplay

import (
	"flag"
	"fmt"
	"github.com/buger/goreplay/internal/size"
	"os"
	"strconv"
	"sync"
	"time"
)

// DEMO indicates that goreplay is running in demo mode
var DEMO string

// MultiOption allows to specify multiple flags with same name and collects all values into array
type MultiOption struct {
	a *[]string
}

func (h *MultiOption) String() string {
	if h.a == nil {
		return ""
	}
	return fmt.Sprint(*h.a)
}

// Set gets called multiple times for each flag with same name
func (h *MultiOption) Set(value string) error {
	if h.a == nil {
		return nil
	}

	*h.a = append(*h.a, value)
	return nil
}

// MultiOption allows to specify multiple flags with same name and collects all values into array
type MultiIntOption struct {
	a *[]int
}

func (h *MultiIntOption) String() string {
	if h.a == nil {
		return ""
	}

	return fmt.Sprint(*h.a)
}

// Set gets called multiple times for each flag with same name
func (h *MultiIntOption) Set(value string) error {
	if h.a == nil {
		return nil
	}

	val, _ := strconv.Atoi(value)
	*h.a = append(*h.a, val)
	return nil
}

// AppSettings is the struct of main configuration
type AppSettings struct {
	Verbose   int           `json:"verbose"`
	Stats     bool          `json:"stats"`
	ExitAfter time.Duration `json:"exit-after"`

	SplitOutput          bool   `json:"split-output"`
	RecognizeTCPSessions bool   `json:"recognize-tcp-sessions"`
	Pprof                string `json:"http-pprof"`

	CopyBufferSize size.Size `json:"copy-buffer-size"`

	InputDummy   []string `json:"input-dummy"`
	OutputDummy  []string
	OutputStdout bool `json:"output-stdout"`
	OutputNull   bool `json:"output-null"`

	InputTCP        []string `json:"input-tcp"`
	InputTCPConfig  TCPInputConfig
	OutputTCP       []string `json:"output-tcp"`
	OutputTCPConfig TCPOutputConfig
	OutputTCPStats  bool `json:"output-tcp-stats"`

	OutputWebSocket       []string `json:"output-ws"`
	OutputWebSocketConfig WebSocketOutputConfig
	OutputWebSocketStats  bool `json:"output-ws-stats"`

	InputFile          []string      `json:"input-file"`
	InputFileLoop      bool          `json:"input-file-loop"`
	InputFileReadDepth int           `json:"input-file-read-depth"`
	InputFileDryRun    bool          `json:"input-file-dry-run"`
	InputFileMaxWait   time.Duration `json:"input-file-max-wait"`
	OutputFile         []string      `json:"output-file"`
	OutputFileConfig   FileOutputConfig

	InputRAW       []string `json:"input_raw"`
	InputRAWConfig RAWInputConfig

	Middleware string `json:"middleware"`

	InputHTTP    []string
	OutputHTTP   []string `json:"output-http"`
	PrettifyHTTP bool     `json:"prettify-http"`

	OutputHTTPConfig HTTPOutputConfig

	OutputBinary       []string `json:"output-binary"`
	OutputBinaryConfig BinaryOutputConfig

	ModifierConfig HTTPModifierConfig

	InputKafkaConfig  InputKafkaConfig
	OutputKafkaConfig OutputKafkaConfig
	KafkaTLSConfig    KafkaTLSConfig
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
	flag.StringVar(&Settings.Pprof, "http-pprof", "", "Enable profiling. Starts  http server on specified port, exposing special /debug/pprof endpoint. Example: `:8181`")
	flag.IntVar(&Settings.Verbose, "verbose", 0, "set the level of verbosity, if greater than zero then it will turn on debug output")
	flag.BoolVar(&Settings.Stats, "stats", false, "Turn on queue stats output")

	if DEMO == "" {
		flag.DurationVar(&Settings.ExitAfter, "exit-after", 0, "exit after specified duration")
	} else {
		Settings.ExitAfter = 5 * time.Minute
	}

	flag.BoolVar(&Settings.SplitOutput, "split-output", false, "By default each output gets same traffic. If set to `true` it splits traffic equally among all outputs.")
	flag.BoolVar(&Settings.RecognizeTCPSessions, "recognize-tcp-sessions", false, "[PRO] If turned on http output will create separate worker for each TCP session. Splitting output will session based as well.")

	flag.Var(&MultiOption{&Settings.InputDummy}, "input-dummy", "Used for testing outputs. Emits 'Get /' request every 1s")
	flag.BoolVar(&Settings.OutputStdout, "output-stdout", false, "Used for testing inputs. Just prints to console data coming from inputs.")
	flag.BoolVar(&Settings.OutputNull, "output-null", false, "Used for testing inputs. Drops all requests.")

	flag.Var(&MultiOption{&Settings.InputTCP}, "input-tcp", "Used for internal communication between Gor instances. Example: \n\t# Receive requests from other Gor instances on 28020 port, and redirect output to staging\n\tgor --input-tcp :28020 --output-http staging.com")
	flag.BoolVar(&Settings.InputTCPConfig.Secure, "input-tcp-secure", false, "Turn on TLS security. Do not forget to specify certificate and key files.")
	flag.StringVar(&Settings.InputTCPConfig.CertificatePath, "input-tcp-certificate", "", "Path to PEM encoded certificate file. Used when TLS turned on.")
	flag.StringVar(&Settings.InputTCPConfig.KeyPath, "input-tcp-certificate-key", "", "Path to PEM encoded certificate key file. Used when TLS turned on.")

	flag.Var(&MultiOption{&Settings.OutputTCP}, "output-tcp", "Used for internal communication between Gor instances. Example: \n\t# Listen for requests on 80 port and forward them to other Gor instance on 28020 port\n\tgor --input-raw :80 --output-tcp replay.local:28020")
	flag.BoolVar(&Settings.OutputTCPConfig.Secure, "output-tcp-secure", false, "Use TLS secure connection. --input-file on another end should have TLS turned on as well.")
	flag.BoolVar(&Settings.OutputTCPConfig.SkipVerify, "output-tcp-skip-verify", false, "Don't verify hostname on TLS secure connection.")
	flag.BoolVar(&Settings.OutputTCPConfig.Sticky, "output-tcp-sticky", false, "Use Sticky connection. Request/Response with same ID will be sent to the same connection.")
	flag.IntVar(&Settings.OutputTCPConfig.Workers, "output-tcp-workers", 10, "Number of parallel tcp connections, default is 10")
	flag.BoolVar(&Settings.OutputTCPStats, "output-tcp-stats", false, "Report TCP output queue stats to console every 5 seconds.")

	flag.Var(&MultiOption{&Settings.OutputWebSocket}, "output-ws", "Just like output tcp, just with WebSocket. Example: \n\t# Listen for requests on 80 port and forward them to other Gor instance on 28020 port\n\tgor --input-raw :80 --output-ws wss://replay.local:28020/endpoint")
	flag.BoolVar(&Settings.OutputWebSocketConfig.SkipVerify, "output-ws-skip-verify", false, "Don't verify hostname on TLS secure connection.")
	flag.BoolVar(&Settings.OutputWebSocketConfig.Sticky, "output-ws-sticky", false, "Use Sticky connection. Request/Response with same ID will be sent to the same connection.")
	flag.IntVar(&Settings.OutputWebSocketConfig.Workers, "output-ws-workers", 10, "Number of parallel ws connections, default is 10")
	flag.BoolVar(&Settings.OutputWebSocketStats, "output-ws-stats", false, "Report WebSocket output queue stats to console every 5 seconds.")

	flag.Var(&MultiOption{&Settings.InputFile}, "input-file", "Read requests from file: \n\tgor --input-file ./requests.gor --output-http staging.com")
	flag.BoolVar(&Settings.InputFileLoop, "input-file-loop", false, "Loop input files, useful for performance testing.")
	flag.IntVar(&Settings.InputFileReadDepth, "input-file-read-depth", 100, "GoReplay tries to read and cache multiple records, in advance. In parallel it also perform sorting of requests, if they came out of order. Since it needs hold this buffer in memory, bigger values can cause worse performance")
	flag.BoolVar(&Settings.InputFileDryRun, "input-file-dry-run", false, "Simulate reading from the data source without replaying it. You will get information about expected replay time, number of found records etc.")
	flag.DurationVar(&Settings.InputFileMaxWait, "input-file-max-wait", 0, "Set the maximum time between requests. Can help in situations when you have too long periods between request, and you want to skip them. Example: --input-raw-max-wait 1s")

	flag.Var(&MultiOption{&Settings.OutputFile}, "output-file", "Write incoming requests to file: \n\tgor --input-raw :80 --output-file ./requests.gor")
	flag.DurationVar(&Settings.OutputFileConfig.FlushInterval, "output-file-flush-interval", time.Second, "Interval for forcing buffer flush to the file, default: 1s.")
	flag.BoolVar(&Settings.OutputFileConfig.Append, "output-file-append", false, "The flushed chunk is appended to existence file or not. ")
	flag.Var(&Settings.OutputFileConfig.SizeLimit, "output-file-size-limit", "Size of each chunk. Default: 32mb")
	flag.IntVar(&Settings.OutputFileConfig.QueueLimit, "output-file-queue-limit", 256, "The length of the chunk queue. Default: 256")
	flag.Var(&Settings.OutputFileConfig.OutputFileMaxSize, "output-file-max-size-limit", "Max size of output file, Default: 1TB")

	flag.StringVar(&Settings.OutputFileConfig.BufferPath, "output-file-buffer", "/tmp", "The path for temporary storing current buffer: \n\tgor --input-raw :80 --output-file s3://mybucket/logs/%Y-%m-%d.gz --output-file-buffer /mnt/logs")

	flag.BoolVar(&Settings.PrettifyHTTP, "prettify-http", false, "If enabled, will automatically decode requests and responses with: Content-Encoding: gzip and Transfer-Encoding: chunked. Useful for debugging, in conjunction with --output-stdout")

	flag.Var(&Settings.CopyBufferSize, "copy-buffer-size", "Set the buffer size for an individual request (default 5MB)")

	// input raw flags
	flag.Var(&MultiOption{&Settings.InputRAW}, "input-raw", "Capture traffic from given port (use RAW sockets and require *sudo* access):\n\t# Capture traffic from 8080 port\n\tgor --input-raw :8080 --output-http staging.com")
	flag.BoolVar(&Settings.InputRAWConfig.TrackResponse, "input-raw-track-response", false, "If turned on Gor will track responses in addition to requests, and they will be available to middleware and file output.")
	flag.IntVar(&Settings.InputRAWConfig.VXLANPort, "input-raw-vxlan-port", 4789, "VXLAN port. Can be used only when engine set to `vxlan`. Default: 4789")
	flag.Var(&MultiIntOption{&Settings.InputRAWConfig.VXLANVNIs}, "input-raw-vxlan-vni", "VXLAN VNI to capture. By default capture all VNIs. Ignore VNI by setting them with minus sign, example: `--input-raw-vxlan-vni -2`")
	flag.BoolVar(&Settings.InputRAWConfig.VLAN, "input-raw-vlan", false, "Enable VLAN (802.1Q) support")
	flag.Var(&MultiIntOption{&Settings.InputRAWConfig.VLANVIDs}, "input-raw-vlan-vid", "VLAN VID to capture. By default capture all VIDs")
	flag.Var(&Settings.InputRAWConfig.Engine, "input-raw-engine", "Intercept traffic using `libpcap` (default), `raw_socket`, `pcap_file`, `vxlan`")
	flag.Var(&Settings.InputRAWConfig.Protocol, "input-raw-protocol", "Specify application protocol of intercepted traffic. Possible values: http, binary")
	flag.StringVar(&Settings.InputRAWConfig.RealIPHeader, "input-raw-realip-header", "", "If not blank, injects header with given name and real IP value to the request payload. Usually this header should be named: X-Real-IP")
	flag.DurationVar(&Settings.InputRAWConfig.Expire, "input-raw-expire", time.Second*2, "How much it should wait for the last TCP packet, till consider that TCP message complete.")
	flag.StringVar(&Settings.InputRAWConfig.BPFFilter, "input-raw-bpf-filter", "", "BPF filter to write custom expressions. Can be useful in case of non standard network interfaces like tunneling or SPAN port. Example: --input-raw-bpf-filter 'dst port 80'")
	flag.StringVar(&Settings.InputRAWConfig.TimestampType, "input-raw-timestamp-type", "", "Possible values: PCAP_TSTAMP_HOST, PCAP_TSTAMP_HOST_LOWPREC, PCAP_TSTAMP_HOST_HIPREC, PCAP_TSTAMP_ADAPTER, PCAP_TSTAMP_ADAPTER_UNSYNCED. This values not supported on all systems, GoReplay will tell you available values of you put wrong one.")
	flag.BoolVar(&Settings.InputRAWConfig.Snaplen, "input-raw-override-snaplen", false, "Override the capture snaplen to be 64k. Required for some Virtualized environments")
	flag.DurationVar(&Settings.InputRAWConfig.BufferTimeout, "input-raw-buffer-timeout", 0, "set the pcap timeout. for immediate mode don't set this flag")
	flag.Var(&Settings.InputRAWConfig.BufferSize, "input-raw-buffer-size", "Controls size of the OS buffer which holds packets until they dispatched. Default value depends by system: in Linux around 2MB. If you see big package drop, increase this value.")
	flag.BoolVar(&Settings.InputRAWConfig.Promiscuous, "input-raw-promisc", false, "enable promiscuous mode")
	flag.BoolVar(&Settings.InputRAWConfig.Monitor, "input-raw-monitor", false, "enable RF monitor mode")
	flag.BoolVar(&Settings.InputRAWConfig.Stats, "input-raw-stats", false, "enable stats generator on raw TCP messages")
	flag.BoolVar(&Settings.InputRAWConfig.AllowIncomplete, "input-raw-allow-incomplete", false, "If turned on Gor will record HTTP messages with missing packets")
	flag.Var(&MultiOption{&Settings.InputRAWConfig.IgnoreInterface}, "input-raw-ignore-interface", "In case if you want listen for all interfaces except a few ones. Can be used in k8s environment. Example: --input-raw-ignore-interface cbr0 --input-raw-ignore-interface eth0 --input-raw-ignore-interface localhost")

	flag.StringVar(&Settings.Middleware, "middleware", "", "Used for modifying traffic using external command")

	flag.Var(&MultiOption{&Settings.OutputHTTP}, "output-http", "Forwards incoming requests to given http address.\n\t# Redirect all incoming requests to staging.com address \n\tgor --input-raw :80 --output-http http://staging.com")

	/* outputHTTPConfig */
	flag.Var(&Settings.OutputHTTPConfig.BufferSize, "output-http-response-buffer", "HTTP response buffer size, all data after this size will be discarded.")
	flag.IntVar(&Settings.OutputHTTPConfig.WorkersMin, "output-http-workers-min", 0, "Gor uses dynamic worker scaling. Enter a number to set a minimum number of workers. default = 1.")
	flag.IntVar(&Settings.OutputHTTPConfig.WorkersMax, "output-http-workers", 0, "Gor uses dynamic worker scaling. Enter a number to set a maximum number of workers. default = 0 = unlimited.")
	flag.IntVar(&Settings.OutputHTTPConfig.QueueLen, "output-http-queue-len", 1000, "Number of requests that can be queued for output, if all workers are busy. default = 1000")
	flag.BoolVar(&Settings.OutputHTTPConfig.SkipVerify, "output-http-skip-verify", false, "Don't verify hostname on TLS secure connection.")
	flag.DurationVar(&Settings.OutputHTTPConfig.WorkerTimeout, "output-http-worker-timeout", 2*time.Second, "Duration to rollback idle workers.")

	flag.IntVar(&Settings.OutputHTTPConfig.RedirectLimit, "output-http-redirects", 0, "Enable how often redirects should be followed.")
	flag.DurationVar(&Settings.OutputHTTPConfig.Timeout, "output-http-timeout", 5*time.Second, "Specify HTTP request/response timeout. By default 5s. Example: --output-http-timeout 30s")
	flag.BoolVar(&Settings.OutputHTTPConfig.TrackResponses, "output-http-track-response", false, "If turned on, HTTP output responses will be set to all outputs like stdout, file and etc.")

	flag.BoolVar(&Settings.OutputHTTPConfig.Stats, "output-http-stats", false, "Report http output queue stats to console every N milliseconds. See output-http-stats-ms")
	flag.IntVar(&Settings.OutputHTTPConfig.StatsMs, "output-http-stats-ms", 5000, "Report http output queue stats to console every N milliseconds. default: 5000")
	flag.BoolVar(&Settings.OutputHTTPConfig.OriginalHost, "http-original-host", false, "Normally gor replaces the Host http header with the host supplied with --output-http.  This option disables that behavior, preserving the original Host header.")
	flag.StringVar(&Settings.OutputHTTPConfig.ElasticSearch, "output-http-elasticsearch", "", "Send request and response stats to ElasticSearch:\n\tgor --input-raw :8080 --output-http staging.com --output-http-elasticsearch 'es_host:api_port/index_name'")
	/* outputHTTPConfig */

	flag.Var(&MultiOption{&Settings.OutputBinary}, "output-binary", "Forwards incoming binary payloads to given address.\n\t# Redirect all incoming requests to staging.com address \n\tgor --input-raw :80 --input-raw-protocol binary --output-binary staging.com:80")

	/* outputBinaryConfig */
	flag.Var(&Settings.OutputBinaryConfig.BufferSize, "output-tcp-response-buffer", "TCP response buffer size, all data after this size will be discarded.")
	flag.IntVar(&Settings.OutputBinaryConfig.Workers, "output-binary-workers", 0, "Gor uses dynamic worker scaling by default.  Enter a number to run a set number of workers.")
	flag.DurationVar(&Settings.OutputBinaryConfig.Timeout, "output-binary-timeout", 0, "Specify HTTP request/response timeout. By default 5s. Example: --output-binary-timeout 30s")
	flag.BoolVar(&Settings.OutputBinaryConfig.TrackResponses, "output-binary-track-response", false, "If turned on, Binary output responses will be set to all outputs like stdout, file and etc.")

	flag.BoolVar(&Settings.OutputBinaryConfig.Debug, "output-binary-debug", false, "Enables binary debug output.")
	/* outputBinaryConfig */

	flag.StringVar(&Settings.OutputKafkaConfig.Host, "output-kafka-host", "", "Read request and response stats from Kafka:\n\tgor --input-raw :8080 --output-kafka-host '192.168.0.1:9092,192.168.0.2:9092'")
	flag.StringVar(&Settings.OutputKafkaConfig.Topic, "output-kafka-topic", "", "Read request and response stats from Kafka:\n\tgor --input-raw :8080 --output-kafka-topic 'kafka-log'")
	flag.BoolVar(&Settings.OutputKafkaConfig.UseJSON, "output-kafka-json-format", false, "If turned on, it will serialize messages from GoReplay text format to JSON.")
	flag.BoolVar(&Settings.OutputKafkaConfig.SASLConfig.UseSASL, "output-kafka-use-sasl", false, "--output-kafka-use-sasl true")
	flag.StringVar(&Settings.OutputKafkaConfig.SASLConfig.Mechanism, "output-kafka-mechanism", "", "mechanism\n\tgor --input-raw :8080 --output-kafka-mechanism 'SCRAM-SHA-512'")
	flag.StringVar(&Settings.OutputKafkaConfig.SASLConfig.Username, "output-kafka-username", "", "username\n\tgor --input-raw :8080 --output-kafka-username 'username'")
	flag.StringVar(&Settings.OutputKafkaConfig.SASLConfig.Password, "output-kafka-password", "", "password\n\tgor --input-raw :8080 --output-kafka-password 'password'")

	flag.StringVar(&Settings.InputKafkaConfig.Host, "input-kafka-host", "", "Send request and response stats to Kafka:\n\tgor --output-stdout --input-kafka-host '192.168.0.1:9092,192.168.0.2:9092'")
	flag.StringVar(&Settings.InputKafkaConfig.Topic, "input-kafka-topic", "", "Send request and response stats to Kafka:\n\tgor --output-stdout --input-kafka-topic 'kafka-log'")
	flag.BoolVar(&Settings.InputKafkaConfig.UseJSON, "input-kafka-json-format", false, "If turned on, it will assume that messages coming in JSON format rather than  GoReplay text format.")
	flag.BoolVar(&Settings.InputKafkaConfig.SASLConfig.UseSASL, "input-kafka-use-sasl", false, "use-sasl\n\t--use-sasl true")
	flag.StringVar(&Settings.InputKafkaConfig.SASLConfig.Mechanism, "input-kafka-mechanism", "", "mechanism\n\tgor --input-raw :8080 --output-kafka-mechanism 'SCRAM-SHA-512'")
	flag.StringVar(&Settings.InputKafkaConfig.SASLConfig.Username, "input-kafka-username", "", "username\n\tgor --input-raw :8080 --output-kafka-username 'username'")
	flag.StringVar(&Settings.InputKafkaConfig.SASLConfig.Password, "input-kafka-password", "", "password\n\tgor --input-raw :8080 --output-kafka-password 'password'")
	flag.Int64Var(&Settings.InputKafkaConfig.Offset, "input-kafka-offset", -1, "Specify offset in Kafka partitions start to consume\n\t-1: Starts from newest, -2: Starts from oldest")

	flag.StringVar(&Settings.KafkaTLSConfig.CACert, "kafka-tls-ca-cert", "", "CA certificate for Kafka TLS Config:\n\tgor  --input-raw :3000 --output-kafka-host '192.168.0.1:9092' --output-kafka-topic 'topic' --kafka-tls-ca-cert cacert.cer.pem --kafka-tls-client-cert client.cer.pem --kafka-tls-client-key client.key.pem")
	flag.StringVar(&Settings.KafkaTLSConfig.ClientCert, "kafka-tls-client-cert", "", "Client certificate for Kafka TLS Config (mandatory with to kafka-tls-ca-cert and kafka-tls-client-key)")
	flag.StringVar(&Settings.KafkaTLSConfig.ClientKey, "kafka-tls-client-key", "", "Client Key for Kafka TLS Config (mandatory with to kafka-tls-client-cert and kafka-tls-client-key)")

	flag.Var(&Settings.ModifierConfig.Headers, "http-set-header", "Inject additional headers to http request:\n\tgor --input-raw :8080 --output-http staging.com --http-set-header 'User-Agent: Gor'")
	flag.Var(&Settings.ModifierConfig.HeaderRewrite, "http-rewrite-header", "Rewrite the request header based on a mapping:\n\tgor --input-raw :8080 --output-http staging.com --http-rewrite-header Host: (.*).example.com,$1.beta.example.com")
	flag.Var(&Settings.ModifierConfig.Params, "http-set-param", "Set request url param, if param already exists it will be overwritten:\n\tgor --input-raw :8080 --output-http staging.com --http-set-param api_key=1")
	flag.Var(&Settings.ModifierConfig.Methods, "http-allow-method", "Whitelist of HTTP methods to replay. Anything else will be dropped:\n\tgor --input-raw :8080 --output-http staging.com --http-allow-method GET --http-allow-method OPTIONS")
	flag.Var(&Settings.ModifierConfig.URLRegexp, "http-allow-url", "A regexp to match requests against. Filter get matched against full url with domain. Anything else will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-allow-url ^www.")
	flag.Var(&Settings.ModifierConfig.URLNegativeRegexp, "http-disallow-url", "A regexp to match requests against. Filter get matched against full url with domain. Anything else will be forwarded:\n\t gor --input-raw :8080 --output-http staging.com --http-disallow-url ^www.")
	flag.Var(&Settings.ModifierConfig.URLRewrite, "http-rewrite-url", "Rewrite the request url based on a mapping:\n\tgor --input-raw :8080 --output-http staging.com --http-rewrite-url /v1/user/([^\\/]+)/ping:/v2/user/$1/ping")
	flag.Var(&Settings.ModifierConfig.HeaderFilters, "http-allow-header", "A regexp to match a specific header against. Requests with non-matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-allow-header api-version:^v1")
	flag.Var(&Settings.ModifierConfig.HeaderNegativeFilters, "http-disallow-header", "A regexp to match a specific header against. Requests with matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-disallow-header \"User-Agent: Replayed by Gor\"")
	flag.Var(&Settings.ModifierConfig.HeaderBasicAuthFilters, "http-basic-auth-filter", "A regexp to match the decoded basic auth string against. Requests with non-matching headers will be dropped:\n\t gor --input-raw :8080 --output-http staging.com --http-basic-auth-filter \"^customer[0-9].*\"")
	flag.Var(&Settings.ModifierConfig.HeaderHashFilters, "http-header-limiter", "Takes a fraction of requests, consistently taking or rejecting a request based on the FNV32-1A hash of a specific header:\n\t gor --input-raw :8080 --output-http staging.com --http-header-limiter user-id:25%")
	flag.Var(&Settings.ModifierConfig.ParamHashFilters, "http-param-limiter", "Takes a fraction of requests, consistently taking or rejecting a request based on the FNV32-1A hash of a specific GET param:\n\t gor --input-raw :8080 --output-http staging.com --http-param-limiter user_id:25%")

	// default values, using for tests
	Settings.OutputFileConfig.SizeLimit = 33554432
	Settings.OutputFileConfig.OutputFileMaxSize = 1099511627776
	Settings.CopyBufferSize = 5242880

}

func CheckSettings() {
	if Settings.OutputFileConfig.SizeLimit < 1 {
		Settings.OutputFileConfig.SizeLimit.Set("32mb")
	}
	if Settings.OutputFileConfig.OutputFileMaxSize < 1 {
		Settings.OutputFileConfig.OutputFileMaxSize.Set("1tb")
	}
	if Settings.CopyBufferSize < 1 {
		Settings.CopyBufferSize.Set("5mb")
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
