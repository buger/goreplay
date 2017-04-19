package main

import (
	"encoding/json"
	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	"log"
)

// KafkaInput is used for recieving Kafka messages and
// transforming them into HTTP payloads.
type KafkaInput struct {
	config    *KafkaConfig
	consumers []sarama.PartitionConsumer
	messages  chan *sarama.ConsumerMessage
}

// NewKafkaInput creates instance of kafka consumer client.
func NewKafkaInput(address string, config *KafkaConfig) *KafkaInput {
	c := sarama.NewConfig()
	// Configuration options go here

	var con sarama.Consumer

	if mock, ok := config.consumer.(*mocks.Consumer); ok && mock != nil {
		con = config.consumer
	} else {
		var err error
		con, err = sarama.NewConsumer([]string{config.host}, c)

		if err != nil {
			log.Fatalln("Failed to start Sarama(Kafka) consumer:", err)
		}
	}

	partitions, err := con.Partitions(config.topic)
	if err != nil {
		log.Fatalln("Failed to collect Sarama(Kafka) partitions:", err)
	}

	i := &KafkaInput{
		config:    config,
		consumers: make([]sarama.PartitionConsumer, len(partitions)),
		messages:  make(chan *sarama.ConsumerMessage, 256),
	}

	for index, partition := range partitions {
		consumer, err := con.ConsumePartition(config.topic, partition, sarama.OffsetNewest)
		if err != nil {
			log.Fatalln("Failed to start Sarama(Kafka) partition consumer:", err)
		}

		go func(consumer sarama.PartitionConsumer) {
			defer consumer.Close()

			for message := range consumer.Messages() {
				i.messages <- message
			}
		}(consumer)

		if Settings.verbose {
			// Start infinite loop for tracking errors for kafka producer.
			go i.ErrorHandler(consumer)
		}

		i.consumers[index] = consumer
	}

	return i
}

// ErrorHandler should receive errors
func (i *KafkaInput) ErrorHandler(consumer sarama.PartitionConsumer) {
	for err := range consumer.Errors() {
		log.Println("Failed to read access log entry:", err)
	}
}

func (i *KafkaInput) Read(data []byte) (int, error) {
	message := <-i.messages

	if !i.config.useJSON {
		copy(data, message.Value)
		return len(message.Value), nil
	} else {
		var kafkaMessage KafkaMessage
		json.Unmarshal(message.Value, &kafkaMessage)

		buf, err := kafkaMessage.Dump()
		if err != nil {
			log.Println("Failed to decode access log entry:", err)
			return 0, err
		}

		copy(data, buf)

		return len(buf), nil
	}
}

func (i *KafkaInput) String() string {
	return "Kafka Input: " + i.config.host + "/" + i.config.topic
}
