package goreplay

import (
	"encoding/json"
	"github.com/buger/goreplay/internal/byteutils"
	"github.com/buger/goreplay/proto"
	"log"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
)

// KafkaOutput is used for sending payloads to kafka in JSON format.
type KafkaOutput struct {
	config   *OutputKafkaConfig
	producer sarama.AsyncProducer
}

// KafkaOutputFrequency in milliseconds
const KafkaOutputFrequency = 500

// NewKafkaOutput creates instance of kafka producer client  with TLS config
func NewKafkaOutput(_ string, config *OutputKafkaConfig, tlsConfig *KafkaTLSConfig) PluginWriter {
	c := NewKafkaConfig(&config.SASLConfig, tlsConfig)

	var producer sarama.AsyncProducer

	if mock, ok := config.producer.(*mocks.AsyncProducer); ok && mock != nil {
		producer = config.producer
	} else {
		c.Producer.RequiredAcks = sarama.WaitForLocal
		c.Producer.Compression = sarama.CompressionSnappy
		c.Producer.Flush.Frequency = KafkaOutputFrequency * time.Millisecond

		brokerList := strings.Split(config.Host, ",")

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

	// Start infinite loop for tracking errors for kafka producer.
	go o.ErrorHandler()

	return o
}

// ErrorHandler should receive errors
func (o *KafkaOutput) ErrorHandler() {
	for err := range o.producer.Errors() {
		Debug(1, "Failed to write access log entry:", err)
	}
}

// PluginWrite writes a message to this plugin
func (o *KafkaOutput) PluginWrite(msg *Message) (n int, err error) {
	var message sarama.StringEncoder
	meta := payloadMeta(msg.Meta)
	msgKey := sarama.StringEncoder(meta[1])

	if !o.config.UseJSON {
		message = sarama.StringEncoder(byteutils.SliceToString(msg.Meta) + byteutils.SliceToString(msg.Data))
	} else {
		mimeHeader := proto.ParseHeaders(msg.Data)
		header := make(map[string]string)
		for k, v := range mimeHeader {
			header[k] = strings.Join(v, ", ")
		}

		req := msg.Data

		kafkaMessage := KafkaMessage{
			ReqURL:     byteutils.SliceToString(proto.Path(req)),
			ReqType:    byteutils.SliceToString(meta[0]),
			ReqID:      byteutils.SliceToString(meta[1]),
			ReqTs:      byteutils.SliceToString(meta[2]),
			ReqMethod:  byteutils.SliceToString(proto.Method(req)),
			ReqBody:    byteutils.SliceToString(proto.Body(req)),
			ReqHeaders: header,
		}
		jsonMessage, _ := json.Marshal(&kafkaMessage)
		message = sarama.StringEncoder(byteutils.SliceToString(jsonMessage))
	}

	o.producer.Input() <- &sarama.ProducerMessage{
		Topic: o.config.Topic,
		Value: message,
		Key:   msgKey,
	}

	return len(message), nil
}
