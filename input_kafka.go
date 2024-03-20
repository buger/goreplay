package goreplay

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
)

// KafkaInput is used for receiving Kafka messages and
// transforming them into HTTP payloads.
type KafkaInput struct {
	config    *InputKafkaConfig
	consumers []sarama.PartitionConsumer
	messages  chan *sarama.ConsumerMessage
	quit      chan struct{}
}

// NewKafkaInput creates instance of kafka consumer client with TLS config
func NewKafkaInput(_ string, config *InputKafkaConfig, tlsConfig *KafkaTLSConfig) *KafkaInput {
	c := NewKafkaConfig(&config.SASLConfig, tlsConfig)

	var con sarama.Consumer

	if mock, ok := config.consumer.(*mocks.Consumer); ok && mock != nil {
		con = config.consumer
	} else {
		var err error
		con, err = sarama.NewConsumer(strings.Split(config.Host, ","), c)

		if err != nil {
			log.Fatalln("Failed to start Sarama(Kafka) consumer:", err)
		}
	}

	partitions, err := con.Partitions(config.Topic)
	if err != nil {
		log.Fatalln("Failed to collect Sarama(Kafka) partitions:", err)
	}

	i := &KafkaInput{
		config:    config,
		consumers: make([]sarama.PartitionConsumer, len(partitions)),
		messages:  make(chan *sarama.ConsumerMessage, 256),
		quit:      make(chan struct{}),
	}

	for index, partition := range partitions {
		consumer, err := con.ConsumePartition(config.Topic, partition, i.config.Offset)
		if err != nil {
			log.Fatalln("Failed to start Sarama(Kafka) partition consumer:", err)
		}

		go func(consumer sarama.PartitionConsumer) {
			defer consumer.Close()

			for message := range consumer.Messages() {
				i.messages <- message
			}
		}(consumer)

		go i.ErrorHandler(consumer)

		i.consumers[index] = consumer
	}

	return i
}

// ErrorHandler should receive errors
func (i *KafkaInput) ErrorHandler(consumer sarama.PartitionConsumer) {
	for err := range consumer.Errors() {
		Debug(1, "Failed to read access log entry:", err)
	}
}

// PluginRead a reads message from this plugin
func (i *KafkaInput) PluginRead() (*Message, error) {
	var message *sarama.ConsumerMessage
	var msg Message
	select {
	case <-i.quit:
		return nil, ErrorStopped
	case message = <-i.messages:
	}

	msg.Data = message.Value
	if i.config.UseJSON {

		var kafkaMessage KafkaMessage
		json.Unmarshal(message.Value, &kafkaMessage)

		var err error
		msg.Data, err = kafkaMessage.Dump()
		if err != nil {
			Debug(1, "[INPUT-KAFKA] failed to decode access log entry:", err)
			return nil, err
		}
	}

	// does it have meta
	if isOriginPayload(msg.Data) {
		msg.Meta, msg.Data = payloadMetaWithBody(msg.Data)
	}

	return &msg, nil

}

func (i *KafkaInput) String() string {
	return "Kafka Input: " + i.config.Host + "/" + i.config.Topic
}

// Close closes this plugin
func (i *KafkaInput) Close() error {
	close(i.quit)
	return nil
}
