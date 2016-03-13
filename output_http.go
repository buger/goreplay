package main

import (
	"io"
	"sync/atomic"
	"time"

	"github.com/buger/gor/proto"
)

const initialDynamicWorkers = 10

type response struct {
	payload       []byte
	uuid          []byte
	roundTripTime int64
}

// HTTPOutputConfig struct for holding http output configuration
type HTTPOutputConfig struct {
	redirectLimit int

	stats   bool
	workers int

	elasticSearch string

	Timeout      time.Duration
	OriginalHost bool

	Debug bool

	TrackResponses bool

	idleWorkers int
	recycle     int
}

// HTTPOutput plugin manage pool of workers which send request to replayed server
// By default workers pool is dynamic and starts with 10 workers
// You can specify fixed number of workers using `--output-http-workers`
type HTTPOutput struct {
	// Keep this as first element of struct because it guarantees 64bit
	// alignment. atomic.* functions crash on 32bit machines if operand is not
	// aligned at 64bit. See https://github.com/golang/go/issues/599
	activeWorkers int64
	idleWorkers   int32

	address string
	limit   int
	queue   chan []byte

	responses chan response

	needWorker chan int

	config *HTTPOutputConfig

	queueStats *GorStat

	elasticSearch *ESPlugin
}

// NewHTTPOutput constructor for HTTPOutput
// Initialize workers
func NewHTTPOutput(address string, config *HTTPOutputConfig) io.Writer {
	o := new(HTTPOutput)

	o.address = address
	o.config = config

	if o.config.stats {
		o.queueStats = NewGorStat("output_http")
	}

	o.queue = make(chan []byte, 1000)
	o.responses = make(chan response, 1000)
	o.needWorker = make(chan int, 1)

	// Initial workers count
	if o.config.workers == 0 {
		if o.config.idleWorkers > 0 {
			o.needWorker <- o.config.idleWorkers * 3 / 2 // 1.5 min idle workers
		} else {
			o.needWorker <- initialDynamicWorkers
		}
	} else {
		o.needWorker <- o.config.workers
	}

	if o.config.elasticSearch != "" {
		o.elasticSearch = new(ESPlugin)
		o.elasticSearch.Init(o.config.elasticSearch)
	}

	if len(Settings.middleware) > 0 {
		o.config.TrackResponses = true
	}

	go o.workerMaster()

	return o
}

func (o *HTTPOutput) workerMaster() {
	for {
		newWorkers := <-o.needWorker

		// Must calculate the number of new workers if it's dynamic
		if o.config.workers == 0 && o.config.idleWorkers > 0 {
			idleCount := int(atomic.LoadInt32(&o.idleWorkers)) // Current idle workers

			if idleCount >= o.config.idleWorkers {
				continue // Not new workers needed
			}
			minForIdle := o.config.idleWorkers*3/2 - idleCount // Total number of idle workers = 1.5 + min idle
			if minForIdle > newWorkers {
				newWorkers = minForIdle
			}

		}
		for i := 0; i < newWorkers; i++ {
			go o.startWorker()
		}

		// Disable dynamic scaling if workers poll fixed size
		if o.config.workers != 0 {
			return
		}
	}
}

func (o *HTTPOutput) startWorker() {
	client := NewHTTPClient(o.address, &HTTPClientConfig{
		FollowRedirects: o.config.redirectLimit,
		Debug:           o.config.Debug,
		OriginalHost:    o.config.OriginalHost,
		Timeout:         o.config.Timeout,
	})

	deathCount := 0
	iterations := 0

	atomic.AddInt64(&o.activeWorkers, 1)
	defer atomic.AddInt64(&o.activeWorkers, -1)

	for {
		atomic.AddInt32(&o.idleWorkers, 1)
		select {
		case data := <-o.queue:
			atomic.AddInt32(&o.idleWorkers, -1)
			o.sendRequest(client, data)
			deathCount = 0
		case <-time.After(time.Millisecond * 100):
			atomic.AddInt32(&o.idleWorkers, -1)
			// When dynamic scaling enabled workers die after 2s of inactivity
			if o.config.workers == 0 && o.config.idleWorkers == 0 {
				deathCount++
				// If too many timeouts
				if deathCount > 20 {
					workersCount := atomic.LoadInt64(&o.activeWorkers)

					// At least 1 startWorker should be alive
					if workersCount != 1 {
						return
					}
				}
			} else {
				continue
			}
		}
		iterations++

		if (o.config.recycle > 0 && iterations > o.config.recycle) || // If it reached the maximum number of operations
			(o.config.idleWorkers > 0 && int(atomic.LoadInt32(&o.idleWorkers)) > o.config.idleWorkers*2) { // Too many idle process
			o.needWorker <- 0 // Signal 0 to calculate the number of workers to create
			return
		}
	}
}

func (o *HTTPOutput) Write(data []byte) (n int, err error) {
	if !isRequestPayload(data) {
		return len(data), nil
	}

	buf := make([]byte, len(data))
	copy(buf, data)

	o.queue <- buf

	if o.config.stats {
		o.queueStats.Write(len(o.queue))
	}

	if o.config.workers == 0 {
		workersCount := atomic.LoadInt64(&o.activeWorkers)

		if len(o.queue) > int(workersCount) {
			o.needWorker <- len(o.queue)
		}
	}

	return len(data), nil
}

func (o *HTTPOutput) Read(data []byte) (int, error) {
	resp := <-o.responses

	Debug("[OUTPUT-HTTP] Received response:", string(resp.payload))

	header := payloadHeader(ReplayedResponsePayload, resp.uuid, resp.roundTripTime)
	copy(data[0:len(header)], header)
	copy(data[len(header):], resp.payload)

	return len(resp.payload) + len(header), nil
}

func (o *HTTPOutput) sendRequest(client *HTTPClient, request []byte) {
	meta := payloadMeta(request)
	if len(meta) < 2 {
		return
	}
	uuid := meta[1]

	body := payloadBody(request)
	if !proto.IsHTTPPayload(body) {
		return
	}

	start := time.Now()
	resp, err := client.Send(body)
	stop := time.Now()

	if err != nil {
		Debug("Request error:", err)
	}

	if o.config.TrackResponses {
		o.responses <- response{resp, uuid, stop.UnixNano() - start.UnixNano()}
	}

	if o.elasticSearch != nil {
		o.elasticSearch.ResponseAnalyze(request, resp, start, stop)
	}
}

func (o *HTTPOutput) String() string {
	return "HTTP output: " + o.address
}
