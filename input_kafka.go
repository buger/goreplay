package main

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
)

// KafkaInput is used for recieving Kafka messages and
// transforming them into HTTP payloads.
type KafkaInput struct {
	config    *InputKafkaConfig
	consumers []sarama.PartitionConsumer
	messages  chan *sarama.ConsumerMessage
}

// NewKafkaInput creates instance of kafka consumer client.
func NewKafkaInput(address string, config *InputKafkaConfig) *KafkaInput {
	return NewKafkaInputWithTLS(address, config, nil)
}

// NewKafkaInputWithTLS creates instance of kafka consumer client with TLS
func NewKafkaInputWithTLS(address string, config *InputKafkaConfig, tlsConfig *KafkaTLSConfig) *KafkaInput {
	c := NewKafkaConfig(tlsConfig)

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
	}

	for index, partition := range partitions {
		consumer, err := con.ConsumePartition(config.Topic, partition, sarama.OffsetNewest)
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

func (i *KafkaInput) Read(data []byte) (int, error) {
	message := <-i.messages

	if !i.config.UseJSON {
		copy(data, message.Value)
		return len(message.Value), nil
	}

	var kafkaMessage KafkaMessage
	json.Unmarshal(message.Value, &kafkaMessage)

	buf, err := kafkaMessage.Dump()
	if err != nil {
		Debug(1, "Failed to decode access log entry:", err)
		return 0, err
	}

	n := copy(data, buf)

	return n, nil

}

func (i *KafkaInput) String() string {
	return "Kafka Input: " + i.config.Host + "/" + i.config.Topic
}
