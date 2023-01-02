package goreplay

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/buger/goreplay/internal/size"
	"log"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"
)

const (
	initialDynamicWorkers = 10
	readChunkSize         = 64 * 1024
	maxResponseSize       = 1073741824
)

type response struct {
	payload       []byte
	uuid          []byte
	startedAt     int64
	roundTripTime int64
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

func (hoc *HTTPOutputConfig) Copy() *HTTPOutputConfig {
	return &HTTPOutputConfig{
		TrackResponses: hoc.TrackResponses,
		Stats:          hoc.Stats,
		OriginalHost:   hoc.OriginalHost,
		RedirectLimit:  hoc.RedirectLimit,
		WorkersMin:     hoc.WorkersMin,
		WorkersMax:     hoc.WorkersMax,
		StatsMs:        hoc.StatsMs,
		QueueLen:       hoc.QueueLen,
		ElasticSearch:  hoc.ElasticSearch,
		Timeout:        hoc.Timeout,
		WorkerTimeout:  hoc.WorkerTimeout,
		BufferSize:     hoc.BufferSize,
		SkipVerify:     hoc.SkipVerify,
	}
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
	queue         chan *Message
	responses     chan *response
	stop          chan bool // Channel used only to indicate goroutine should shutdown
}

// NewHTTPOutput constructor for HTTPOutput
// Initialize workers
func NewHTTPOutput(address string, config *HTTPOutputConfig) PluginReadWriter {
	o := new(HTTPOutput)
	var err error
	newConfig := config.Copy()
	newConfig.url, err = url.Parse(address)
	if err != nil {
		log.Fatal(fmt.Sprintf("[OUTPUT-HTTP] parse HTTP output URL error[%q]", err))
	}
	if newConfig.url.Scheme == "" {
		newConfig.url.Scheme = "http"
	}
	newConfig.rawURL = newConfig.url.String()
	if newConfig.Timeout < time.Millisecond*100 {
		newConfig.Timeout = time.Second
	}
	if newConfig.BufferSize <= 0 {
		newConfig.BufferSize = 100 * 1024 // 100kb
	}
	if newConfig.WorkersMin <= 0 {
		newConfig.WorkersMin = 1
	}
	if newConfig.WorkersMin > 1000 {
		newConfig.WorkersMin = 1000
	}
	if newConfig.WorkersMax <= 0 {
		newConfig.WorkersMax = math.MaxInt32 // idealy so large
	}
	if newConfig.WorkersMax < newConfig.WorkersMin {
		newConfig.WorkersMax = newConfig.WorkersMin
	}
	if newConfig.QueueLen <= 0 {
		newConfig.QueueLen = 1000
	}
	if newConfig.RedirectLimit < 0 {
		newConfig.RedirectLimit = 0
	}
	if newConfig.WorkerTimeout <= 0 {
		newConfig.WorkerTimeout = time.Second * 2
	}
	o.config = newConfig
	o.stop = make(chan bool)
	if o.config.Stats {
		o.queueStats = NewGorStat("output_http", o.config.StatsMs)
	}

	o.queue = make(chan *Message, o.config.QueueLen)
	if o.config.TrackResponses {
		o.responses = make(chan *response, o.config.QueueLen)
	}
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
		case msg := <-o.queue:
			o.sendRequest(o.client, msg)
		}
	}
}

// PluginWrite writes message to this plugin
func (o *HTTPOutput) PluginWrite(msg *Message) (n int, err error) {
	if !isRequestPayload(msg.Meta) {
		return len(msg.Data), nil
	}

	select {
	case <-o.stop:
		return 0, ErrorStopped
	case o.queue <- msg:
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
	return len(msg.Data) + len(msg.Meta), nil
}

// PluginRead reads message from this plugin
func (o *HTTPOutput) PluginRead() (*Message, error) {
	if !o.config.TrackResponses {
		return nil, ErrorStopped
	}
	var resp *response
	var msg Message
	select {
	case <-o.stop:
		return nil, ErrorStopped
	case resp = <-o.responses:
		msg.Data = resp.payload
	}

	msg.Meta = payloadHeader(ReplayedResponsePayload, resp.uuid, resp.startedAt, resp.roundTripTime)

	return &msg, nil
}

func (o *HTTPOutput) sendRequest(client *HTTPClient, msg *Message) {
	if !isRequestPayload(msg.Meta) {
		return
	}

	uuid := payloadID(msg.Meta)
	start := time.Now()
	resp, err := client.Send(msg.Data)
	stop := time.Now()

	if err != nil {
		Debug(1, fmt.Sprintf("[HTTP-OUTPUT] error when sending: %q", err))
		return
	}
	if resp == nil {
		return
	}

	if o.config.TrackResponses {
		o.responses <- &response{resp, uuid, start.UnixNano(), stop.UnixNano() - start.UnixNano()}
	}

	if o.elasticSearch != nil {
		o.elasticSearch.ResponseAnalyze(msg.Data, resp, start, stop)
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

	// fix #862
	if c.config.url.Path == "" && c.config.url.RawQuery == "" {
		req.URL.Scheme = c.config.url.Scheme
		req.URL.Host = c.config.url.Host
	} else {
		req.URL = c.config.url
	}

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
	_ = resp.Body.Close()
	return nil, nil
}
