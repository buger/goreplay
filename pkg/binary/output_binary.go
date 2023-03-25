package binary

import (
	"sync/atomic"
	"time"

	"github.com/buger/goreplay/internal/size"
	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/proto"

	"github.com/rs/zerolog/log"
)

const (
	initialDynamicWorkers = 10
)

var outputLogger = log.With().Str("component", "output_binary").Logger()

// BinaryOutputConfig struct for holding binary output configuration
type BinaryOutputConfig struct {
	Workers        int           `json:"output-binary-workers"`
	Timeout        time.Duration `json:"output-binary-timeout"`
	BufferSize     size.Size     `json:"output-tcp-response-buffer"`
	Debug          bool          `json:"output-binary-debug"`
	TrackResponses bool          `json:"output-binary-track-response"`
}

// BinaryOutput plugin manage pool of workers which send request to replayed server
// By default workers pool is dynamic and starts with 10 workers
// You can specify fixed number of workers using `--output-tcp-workers`
type BinaryOutput struct {
	// Keep this as first element of struct because it guarantees 64bit
	// alignment. atomic.* functions crash on 32bit machines if operand is not
	// aligned at 64bit. See https://github.com/golang/go/issues/599
	activeWorkers int64
	address       string
	queue         chan *plugin.Message
	responses     chan plugin.Response
	needWorker    chan int
	quit          chan struct{}
	config        *BinaryOutputConfig
}

// NewBinaryOutput constructor for BinaryOutput
// Initialize workers
func NewBinaryOutput(address string, config *BinaryOutputConfig) plugin.ReadWriter {
	o := new(BinaryOutput)

	o.address = address
	o.config = config

	o.queue = make(chan *plugin.Message, 1000)
	o.responses = make(chan plugin.Response, 1000)
	o.needWorker = make(chan int, 1)
	o.quit = make(chan struct{})

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
		ResponseBufferSize: int(o.config.BufferSize),
	})

	deathCount := 0

	atomic.AddInt64(&o.activeWorkers, 1)

	for {
		select {
		case msg := <-o.queue:
			o.sendRequest(client, msg)
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

// PluginWrite writes a message tothis plugin
func (o *BinaryOutput) PluginWrite(msg *plugin.Message) (n int, err error) {
	if !proto.IsRequestPayload(msg.Meta) {
		return len(msg.Data), nil
	}

	o.queue <- msg

	if o.config.Workers == 0 {
		workersCount := atomic.LoadInt64(&o.activeWorkers)

		if len(o.queue) > int(workersCount) {
			o.needWorker <- len(o.queue)
		}
	}

	return len(msg.Data) + len(msg.Meta), nil
}

// PluginRead reads a message from this plugin
func (o *BinaryOutput) PluginRead() (*plugin.Message, error) {
	var resp plugin.Response
	var msg plugin.Message
	select {
	case <-o.quit:
		return nil, plugin.ErrorStopped
	case resp = <-o.responses:
	}
	msg.Data = resp.Payload
	msg.Meta = proto.PayloadHeader(proto.ReplayedResponsePayload, resp.UUID, resp.StartedAt, resp.RoundTripTime)

	return &msg, nil
}

func (o *BinaryOutput) sendRequest(client *TCPClient, msg *plugin.Message) {
	if !proto.IsRequestPayload(msg.Meta) {
		return
	}

	uuid := proto.PayloadID(msg.Meta)

	start := time.Now()
	resp, err := client.Send(msg.Data)
	stop := time.Now()

	if err != nil {
		outputLogger.Error().Err(err).Msg("Request error")
	}

	if o.config.TrackResponses {
		o.responses <- plugin.Response{resp, uuid, start.UnixNano(), stop.UnixNano() - start.UnixNano()}
	}
}

func (o *BinaryOutput) String() string {
	return "Binary output: " + o.address
}

// Close closes this plugin for reading
func (o *BinaryOutput) Close() error {
	close(o.quit)
	return nil
}
