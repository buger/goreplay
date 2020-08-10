package main

import (
	"io"
	"sync/atomic"
	"time"
)

// BinaryOutputConfig struct for holding binary output configuration
type BinaryOutputConfig struct {
	Workers        int           `json:"output-binary-workers" mapstructure:"output-binary-workers"`
	Timeout        time.Duration `json:"output-binary-timeout" mapstructure:"output-binary-timeout"`
	BufferSize     int           `json:"output-tcp-response-buffer" mapstructure:"output-tcp-response-buffer"`
	Debug          bool          `json:"output-binary-debug" mapstructure:"output-binary-debug"`
	TrackResponses bool          `json:"output-binary-track-response" mapstructure:"output-binary-track-response"`
}

// BinaryOutput plugin manage pool of workers which send request to replayed server
// By default workers pool is dynamic and starts with 10 workers
// You can specify fixed number of workers using `--output-tcp-workers`
type BinaryOutput struct {
	// Keep this as first element of struct because it guarantees 64bit
	// alignment. atomic.* functions crash on 32bit machines if operand is not
	// aligned at 64bit. See https://github.com/golang/go/issues/599
	activeWorkers int64

	address string
	queue   chan []byte

	responses chan response

	needWorker chan int

	config *BinaryOutputConfig

	queueStats *GorStat
}

// NewBinaryOutput constructor for BinaryOutput
// Initialize workers
func NewBinaryOutput(address string, config *BinaryOutputConfig) io.Writer {
	o := new(BinaryOutput)

	o.address = address
	o.config = config

	o.queue = make(chan []byte, 1000)
	o.responses = make(chan response, 1000)
	o.needWorker = make(chan int, 1)

	// Initial workers count
	if o.config.Workers == 0 {
		o.needWorker <- initialDynamicWorkers
	} else {
		o.needWorker <- o.config.Workers
	}

	go o.workerMaster()

	return o
}

func (o *BinaryOutput) workerMaster() {
	for {
		newWorkers := <-o.needWorker
		for i := 0; i < newWorkers; i++ {
			go o.startWorker()
		}

		// Disable dynamic scaling if workers poll fixed size
		if o.config.Workers != 0 {
			return
		}
	}
}

func (o *BinaryOutput) startWorker() {
	client := NewTCPClient(o.address, &TCPClientConfig{
		Debug:              o.config.Debug,
		Timeout:            o.config.Timeout,
		ResponseBufferSize: o.config.BufferSize,
	})

	deathCount := 0

	atomic.AddInt64(&o.activeWorkers, 1)

	for {
		select {
		case data := <-o.queue:
			o.sendRequest(client, data)
			deathCount = 0
		case <-time.After(time.Millisecond * 100):
			// When dynamic scaling enabled workers die after 2s of inactivity
			if o.config.Workers == 0 {
				deathCount++
			} else {
				continue
			}

			if deathCount > 20 {
				workersCount := atomic.LoadInt64(&o.activeWorkers)

				// At least 1 startWorker should be alive
				if workersCount != 1 {
					atomic.AddInt64(&o.activeWorkers, -1)
					return
				}
			}
		}
	}
}

func (o *BinaryOutput) Write(data []byte) (n int, err error) {
	if !isRequestPayload(data) {
		return len(data), nil
	}

	buf := make([]byte, len(data))
	copy(buf, data)

	o.queue <- buf

	if o.config.Workers == 0 {
		workersCount := atomic.LoadInt64(&o.activeWorkers)

		if len(o.queue) > int(workersCount) {
			o.needWorker <- len(o.queue)
		}
	}

	return len(data), nil
}

func (o *BinaryOutput) Read(data []byte) (int, error) {
	resp := <-o.responses

	Debug("[OUTPUT-TCP] Received response:", string(resp.payload))

	header := payloadHeader(ReplayedResponsePayload, resp.uuid, resp.startedAt, resp.roundTripTime)
	copy(data[0:len(header)], header)
	copy(data[len(header):], resp.payload)

	return len(resp.payload) + len(header), nil
}

func (o *BinaryOutput) sendRequest(client *TCPClient, request []byte) {
	meta := payloadMeta(request)
	if len(meta) < 2 {
		return
	}

	if !isRequestPayload(request) {
		return
	}

	uuid := meta[1]

	body := payloadBody(request)

	start := time.Now()
	resp, err := client.Send(body)
	stop := time.Now()

	if err != nil {
		Debug("Request error:", err)
	}

	if o.config.TrackResponses {
		o.responses <- response{resp, uuid, start.UnixNano(), stop.UnixNano() - start.UnixNano()}
	}
}

func (o *BinaryOutput) String() string {
	return "Binary output: " + o.address
}
