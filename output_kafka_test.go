package main

import (
	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	"testing"
)

func TestOutputKafkaRAW(t *testing.T) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	producer := mocks.NewAsyncProducer(t, config)
	producer.ExpectInputAndSucceed()

	output := NewKafkaOutput("", &KafkaConfig{
		producer: producer,
		topic:    "test",
		useJSON:  false,
	})

	output.Write([]byte("1 2 3\nGET / HTTP1.1\r\nHeader: 1\r\n\r\n"))

	resp := <-producer.Successes()

	data, _ := resp.Value.Encode()

	if string(data) != "1 2 3\nGET / HTTP1.1\r\nHeader: 1\r\n\r\n" {
		t.Error("Message not properly encoded: ", string(data))
	}
}

func TestOutputKafkaJSON(t *testing.T) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	producer := mocks.NewAsyncProducer(t, config)
	producer.ExpectInputAndSucceed()

	output := NewKafkaOutput("", &KafkaConfig{
		producer: producer,
		topic:    "test",
		useJSON:  true,
	})

	output.Write([]byte("1 2 3\nGET / HTTP1.1\r\nHeader: 1\r\n\r\n"))

	resp := <-producer.Successes()

	data, _ := resp.Value.Encode()

	if string(data) != `{"Req_URL":"/","Req_Type":"1","Req_ID":"2","Req_Ts":"3","Req_Method":"GET","Req_Headers":{"Header":"1"}}` {
		t.Error("Message not properly encoded: ", string(data))
	}
}
