package main

import (
	"testing"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
)

func TestInputKafkaRAW(t *testing.T) {
	consumer := mocks.NewConsumer(t, nil)
	defer consumer.Close()

	consumer.ExpectConsumePartition("test", 0, mocks.AnyOffset).YieldMessage(&sarama.ConsumerMessage{Value: []byte("1 2 3\nGET / HTTP1.1\r\nHeader: 1\r\n\r\n")})
	consumer.SetTopicMetadata(
		map[string][]int32{"test": {0}},
	)

	input := NewKafkaInput("", &InputKafkaConfig{
		consumer: consumer,
		Topic:    "test",
		UseJSON:  false,
	}, nil)

	msg, err := input.PluginRead()

	if err != nil {
		t.Fatal(err)
	}

	if string(append(msg.Meta, msg.Data...)) != "1 2 3\nGET / HTTP1.1\r\nHeader: 1\r\n\r\n" {
		t.Error("Message not properly decoded")
	}
}

func TestInputKafkaJSON(t *testing.T) {
	consumer := mocks.NewConsumer(t, nil)
	defer consumer.Close()

	consumer.ExpectConsumePartition("test", 0, mocks.AnyOffset).YieldMessage(&sarama.ConsumerMessage{Value: []byte(`{"Req_URL":"/","Req_Type":"1","Req_ID":"2","Req_Ts":"3","Req_Method":"GET","Req_Headers":{"Header":"1"}}`)})
	consumer.SetTopicMetadata(
		map[string][]int32{"test": {0}},
	)

	input := NewKafkaInput("", &InputKafkaConfig{
		consumer: consumer,
		Topic:    "test",
		UseJSON:  true,
	}, nil)

	msg, err := input.PluginRead()

	if err != nil {
		t.Fatal(err)
	}

	if string(append(msg.Meta, msg.Data...)) != "1 2 3\nGET / HTTP/1.1\r\nHeader: 1\r\n\r\n" {
		t.Error("Message not properly decoded")
	}
}
