package kafka

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/buger/goreplay/internal/byteutils"
	"github.com/buger/goreplay/pkg/http_proto"
	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/proto"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"

	"github.com/rs/zerolog/log"
)

// KafkaOutput is used for sending payloads to kafka in JSON format.
type KafkaOutput struct {
	config   *OutputKafkaConfig
	producer sarama.AsyncProducer
}

// KafkaOutputFrequency in milliseconds
const KafkaOutputFrequency = 500

// NewKafkaOutput creates instance of kafka producer client  with TLS config
func NewKafkaOutput(_ string, config *OutputKafkaConfig, tlsConfig *KafkaTLSConfig) plugin.Writer {
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
			log.Fatal().Err(err).Msg("Failed to start Sarama(Kafka) producer")
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
		log.Error().Err(err).Msg("Failed to write access log entry")
	}
}

// PluginWrite writes a message to this plugin
func (o *KafkaOutput) PluginWrite(msg *plugin.Message) (n int, err error) {
	var message sarama.StringEncoder

	if !o.config.UseJSON {
		message = sarama.StringEncoder(byteutils.SliceToString(msg.Meta) + byteutils.SliceToString(msg.Data))
	} else {
		mimeHeader := http_proto.ParseHeaders(msg.Data)
		header := make(map[string]string)
		for k, v := range mimeHeader {
			header[k] = strings.Join(v, ", ")
		}

		meta := proto.PayloadMeta(msg.Meta)
		req := msg.Data

		kafkaMessage := KafkaMessage{
			ReqURL:     byteutils.SliceToString(http_proto.Path(req)),
			ReqType:    byteutils.SliceToString(meta[0]),
			ReqID:      byteutils.SliceToString(meta[1]),
			ReqTs:      byteutils.SliceToString(meta[2]),
			ReqMethod:  byteutils.SliceToString(http_proto.Method(req)),
			ReqBody:    byteutils.SliceToString(http_proto.Body(req)),
			ReqHeaders: header,
		}
		jsonMessage, _ := json.Marshal(&kafkaMessage)
		message = sarama.StringEncoder(byteutils.SliceToString(jsonMessage))
	}

	o.producer.Input() <- &sarama.ProducerMessage{
		Topic: o.config.Topic,
		Value: message,
	}

	return len(message), nil
}
