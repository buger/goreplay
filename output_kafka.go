package main

import (
	"encoding/json"
	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	"github.com/buger/gor/proto"
	"io"
	"log"
	"strings"
	"time"
)

// KafkaOutput is used for sending payloads to kafka in JSON format.
type KafkaOutput struct {
	config   *KafkaConfig
	producer sarama.AsyncProducer
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

func (o *KafkaOutput) Write(data []byte) (n int, err error) {
	var message sarama.StringEncoder

	if !o.config.useJSON {
		message = sarama.StringEncoder(data)
	} else {
		headers := make(map[string]string)
		proto.ParseHeaders([][]byte{data}, func(header []byte, value []byte) bool {
			headers[string(header)] = string(value)
			return true
		})

		meta := payloadMeta(data)
		req := payloadBody(data)

		kafkaMessage := KafkaMessage{
			ReqURL:     string(proto.Path(req)),
			ReqType:    string(meta[0]),
			ReqID:      string(meta[1]),
			ReqTs:      string(meta[2]),
			ReqMethod:  string(proto.Method(req)),
			ReqBody:    string(proto.Body(req)),
			ReqHeaders: headers,
		}
		jsonMessage, _ := json.Marshal(&kafkaMessage)
		message = sarama.StringEncoder(jsonMessage)
	}

	o.producer.Input() <- &sarama.ProducerMessage{
		Topic: o.config.topic,
		Value: message,
	}

	return len(message), nil
}
