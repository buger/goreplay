package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/buger/goreplay/size"
)

const (
	initialDynamicWorkers = 10
	readChunkSize         = 64 * 1024
	maxResponseSize       = 1073741824
)

type response struct {
	payload       []byte
	uuid          []byte
	roundTripTime int64
	startedAt     int64
}

// HTTPOutputConfig struct for holding http output configuration
type HTTPOutputConfig struct {
	TrackResponses bool          `json:"output-http-track-response"`
	Stats          bool          `json:"output-http-stats"`
	OriginalHost   bool          `json:"output-http-original-host"`
	RedirectLimit  int           `json:"output-http-redirect-limit"`
	WorkersMin     int           `json:"output-http-workers-min"`
	WorkersMax     int           `json:"output-http-workers"`
	StatsMs        int           `json:"output-http-stats-ms"`
	QueueLen       int           `json:"output-http-queue-len"`
	ElasticSearch  string        `json:"output-http-elasticsearch"`
	Timeout        time.Duration `json:"output-http-timeout"`
	WorkerTimeout  time.Duration `json:"output-http-worker-timeout"`
	BufferSize     size.Size     `json:"output-http-response-buffer"`
	SkipVerify     bool          `json:"output-http-skip-verify"`
	rawURL         string
	url            *url.URL
}

// HTTPOutput plugin manage pool of workers which send request to replayed server
// By default workers pool is dynamic and starts with 1 worker or workerMin workers
// You can specify maximum number of workers using `--output-http-workers`
type HTTPOutput struct {
	activeWorkers int32
	config        *HTTPOutputConfig
	queueStats    *GorStat
	elasticSearch *ESPlugin
	client        *HTTPClient
	stopWorker    chan struct{}
	queue         chan []byte
	responses     chan response
	stop          chan bool // Channel used only to indicate goroutine should shutdown
}

// NewHTTPOutput constructor for HTTPOutput
// Initialize workers
func NewHTTPOutput(address string, config *HTTPOutputConfig) io.Writer {
	o := new(HTTPOutput)
	var err error
	config.url, err = url.Parse(address)
	if err != nil {
		log.Fatal(fmt.Sprintf("[OUTPUT-HTTP] parse HTTP output URL error[%q]", err))
	}
	if config.url.Scheme == "" {
		config.url.Scheme = "http"
	}
	config.rawURL = config.url.String()
	if config.Timeout < time.Millisecond*100 {
		config.Timeout = time.Second
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 100 * 1024 // 100kb
	}
	if config.WorkersMin <= 0 {
		config.WorkersMin = 1
	}
	if config.WorkersMin > 1000 {
		config.WorkersMin = 1000
	}
	if config.WorkersMax <= 0 {
		config.WorkersMax = math.MaxInt32 // idealy so large
	}
	if config.WorkersMax < config.WorkersMin {
		config.WorkersMax = config.WorkersMin
	}
	if config.QueueLen <= 0 {
		config.QueueLen = 1000
	}
	if config.RedirectLimit < 0 {
		config.RedirectLimit = 0
	}
	if config.WorkerTimeout <= 0 {
		config.WorkerTimeout = time.Second * 2
	}
	o.config = config
	o.stop = make(chan bool)
	if o.config.Stats {
		o.queueStats = NewGorStat("output_http", o.config.StatsMs)
	}

	o.queue = make(chan []byte, o.config.QueueLen)
	o.responses = make(chan response, o.config.QueueLen)
	// it should not be buffered to avoid races
	o.stopWorker = make(chan struct{})

	if o.config.ElasticSearch != "" {
		o.elasticSearch = new(ESPlugin)
		o.elasticSearch.Init(o.config.ElasticSearch)
	}
	o.client = NewHTTPClient(o.config)
	o.activeWorkers += int32(o.config.WorkersMin)
	for i := 0; i < o.config.WorkersMin; i++ {
		go o.startWorker()
	}
	go o.workerMaster()
	return o
}

func (o *HTTPOutput) workerMaster() {
	var timer = time.NewTimer(o.config.WorkerTimeout)
	defer func() {
		// recover from panics caused by trying to send in
		// a closed chan(o.stopWorker)
		recover()
	}()
	defer timer.Stop()
	for {
		select {
		case <-o.stop:
			return
		default:
			<-timer.C
		}
		// rollback workers
	rollback:
		if atomic.LoadInt32(&o.activeWorkers) > int32(o.config.WorkersMin) && len(o.queue) < 1 {
			// close one worker
			o.stopWorker <- struct{}{}
			atomic.AddInt32(&o.activeWorkers, -1)
			goto rollback
		}
		timer.Reset(o.config.WorkerTimeout)
	}
}

func (o *HTTPOutput) startWorker() {
	for {
		select {
		case <-o.stopWorker:
			return
		case data := <-o.queue:
			o.sendRequest(o.client, data)
		}
	}
}

func (o *HTTPOutput) Write(data []byte) (n int, err error) {
	if !isRequestPayload(data) {
		return len(data), nil
	}

	buf := make([]byte, len(data))
	copy(buf, data)
	select {
	case <-o.stop:
		return 0, ErrorStopped
	case o.queue <- buf:
	}

	if o.config.Stats {
		o.queueStats.Write(len(o.queue))
	}
	if len(o.queue) > 0 {
		// try to start a new worker to serve
		if atomic.LoadInt32(&o.activeWorkers) < int32(o.config.WorkersMax) {
			go o.startWorker()
			atomic.AddInt32(&o.activeWorkers, 1)
		}
	}
	return len(data), nil
}

func (o *HTTPOutput) Read(data []byte) (int, error) {
	var resp response
	select {
	case <-o.stop:
		return 0, ErrorStopped
	case resp = <-o.responses:
	}

	header := payloadHeader(ReplayedResponsePayload, resp.uuid, resp.roundTripTime, resp.startedAt)
	n := copy(data, header)
	if len(data) > len(header) {
		n += copy(data[len(header):], resp.payload)
	}
	dis := len(header) + len(data) - n
	if dis > 0 {
		Debug(2, fmt.Sprintf("[OUTPUT-HTTP] %dB discarded increase copy buffer size", dis))
	}

	return n, nil
}

func (o *HTTPOutput) sendRequest(client *HTTPClient, request []byte) {
	if !isRequestPayload(request) {
		return
	}
	uuid := payloadID(request)
	body := payloadBody(request)
	start := time.Now()
	resp, err := client.Send(body)
	stop := time.Now()

	if err != nil {
		Debug(1, fmt.Sprintf("[HTTP-OUTPUT] error when sending: %q", err))
		return
	}

	if o.config.TrackResponses {
		o.responses <- response{resp, uuid, start.UnixNano(), stop.UnixNano() - start.UnixNano()}
	}

	if o.elasticSearch != nil {
		o.elasticSearch.ResponseAnalyze(request, resp, start, stop)
	}
}

func (o *HTTPOutput) String() string {
	return "HTTP output: " + o.config.rawURL
}

// Close closes the data channel so that data
func (o *HTTPOutput) Close() error {
	close(o.stop)
	close(o.stopWorker)
	return nil
}

// HTTPClient holds configurations for a single HTTP client
type HTTPClient struct {
	config *HTTPOutputConfig
	Client *http.Client
}

// NewHTTPClient returns new http client with check redirects policy
func NewHTTPClient(config *HTTPOutputConfig) *HTTPClient {
	client := new(HTTPClient)
	client.config = config
	var transport *http.Transport
	client.Client = &http.Client{
		Timeout: client.config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= client.config.RedirectLimit {
				Debug(1, fmt.Sprintf("[HTTPCLIENT] maximum output-http-redirects[%d] reached!", client.config.RedirectLimit))
				return http.ErrUseLastResponse
			}
			lastReq := via[len(via)-1]
			resp := req.Response
			Debug(2, fmt.Sprintf("[HTTPCLIENT] HTTP redirects from %q to %q with %q", lastReq.Host, req.Host, resp.Status))
			return nil
		},
	}
	if config.SkipVerify {
		// clone to avoid modying global default RoundTripper
		transport = http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		client.Client.Transport = transport
	}

	return client
}

// Send sends an http request using client create by NewHTTPClient
func (c *HTTPClient) Send(data []byte) ([]byte, error) {
	var req *http.Request
	var resp *http.Response
	var err error

	req, err = http.ReadRequest(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		return nil, err
	}
	// we don't send CONNECT or OPTIONS request
	if req.Method == http.MethodConnect {
		return nil, nil
	}

	if !c.config.OriginalHost {
		req.Host = c.config.url.Host
	}

	req.URL = c.config.url
	// force connection to not be closed, which can affect the global client
	req.Close = false
	// it's an error if this is not equal to empty string
	req.RequestURI = ""

	resp, err = c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if c.config.TrackResponses {
		return httputil.DumpResponse(resp, true)
	}
	return nil, nil
}
