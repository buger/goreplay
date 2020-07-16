package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"
	"time"

	"github.com/buger/goreplay/proto"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
)

const maxPending = 100

// KafkaOutput is used for sending payloads to kafka in JSON format.
type KafkaOutput struct {
	config   *KafkaConfig
	producer sarama.AsyncProducer
	pending  map[string][]byte
}

// KafkaOutputFrequency in milliseconds
const KafkaOutputFrequency = 500

// NewKafkaOutput creates instance of kafka producer client.
func NewKafkaOutput(address string, config *KafkaConfig) io.Writer {
	c := sarama.NewConfig()

	var producer sarama.AsyncProducer

	if mock, ok := config.producer.(*mocks.AsyncProducer); ok && mock != nil {
		producer = config.producer
	} else {
		c.Producer.RequiredAcks = sarama.WaitForLocal
		c.Producer.Compression = sarama.CompressionSnappy
		c.Producer.Flush.Frequency = KafkaOutputFrequency * time.Millisecond

		brokerList := strings.Split(config.host, ",")

		var err error
		producer, err = sarama.NewAsyncProducer(brokerList, c)
		if err != nil {
			log.Fatalln("Failed to start Sarama(Kafka) producer:", err)
		}
	}

	o := &KafkaOutput{
		config:   config,
		producer: producer,
	}

	if config.trackResponseHeaders {
		if !o.config.useJSON {
			log.Fatalln("output-kafka-json-format is required to use output-kafka-track-response-headers")
		}

		o.pending = make(map[string][]byte)
	}

	if Settings.verbose {
		// Start infinite loop for tracking errors for kafka producer.
		go o.ErrorHandler()
	}

	return o
}

// ErrorHandler should receive errors
func (o *KafkaOutput) ErrorHandler() {
	for err := range o.producer.Errors() {
		log.Println("Failed to write access log entry:", err)
	}
}
func (o *KafkaOutput) writeTrackResponses(in []byte) (n int, err error) {

	if len(o.pending) == maxPending {
		return 0, errors.New("Check that your input is tracking responses")
	}

	data := make([]byte, len(in))
	copy(data, in)

	meta := payloadMeta(data)

	id := string(meta[1])

	p, ok := o.pending[id]

	if !ok {

		o.pending[id] = data

		return len(data), nil

	}

	delete(o.pending, id)

	var req, res []byte
	if isRequestPayload(data) {
		req = data
		res = p
	} else {
		req = p
		res = data
	}

	kafkaMessage := newKafkaMessage(req)
	kafkaMessage.ResHeaders = payloadHeaders(res)
	kafkaMessage.ResStatus = string(proto.Status(payloadBody(res)))
	jsonMessage, _ := json.Marshal(&kafkaMessage)
	message := sarama.StringEncoder(jsonMessage)

	o.producer.Input() <- &sarama.ProducerMessage{
		Topic: o.config.topic,
		Value: message,
	}

	return len(data), nil
}

func payloadHeaders(data []byte) map[string]string {
	headers := make(map[string]string)
	proto.ParseHeaders([][]byte{data}, func(header []byte, value []byte) bool {
		headers[string(header)] = string(value)
		return true
	})
	return headers
}

func newKafkaMessage(data []byte) KafkaMessage {

	meta := payloadMeta(data)
	req := payloadBody(data)
	headers := payloadHeaders(data)

	return KafkaMessage{
		ReqURL:     string(proto.Path(req)),
		ReqType:    string(meta[0]),
		ReqID:      string(meta[1]),
		ReqTs:      string(meta[2]),
		ReqMethod:  string(proto.Method(req)),
		ReqBody:    string(proto.Body(req)),
		ReqHeaders: headers,
	}

}

func (o *KafkaOutput) Write(data []byte) (n int, err error) {

	if o.config.trackResponseHeaders {
		return o.writeTrackResponses(data)
	}

	var message sarama.StringEncoder

	if !o.config.useJSON {
		message = sarama.StringEncoder(data)
	} else {
		kafkaMessage := newKafkaMessage(data)
		jsonMessage, _ := json.Marshal(&kafkaMessage)
		message = sarama.StringEncoder(jsonMessage)
	}

	o.producer.Input() <- &sarama.ProducerMessage{
		Topic: o.config.topic,
		Value: message,
	}

	return len(message), nil
}
