package main

import (
	"fmt"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
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

func TestOutputKafkaJSONWithResponseHeaders(t *testing.T) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	producer := mocks.NewAsyncProducer(t, config)
	producer.ExpectInputAndSucceed()

	output := NewKafkaOutput("", &KafkaConfig{
		producer:             producer,
		topic:                "test",
		useJSON:              true,
		trackResponseHeaders: true,
	})

	output.Write([]byte("1 2 3\nGET / HTTP1.1\r\nHeader: 1\r\n\r\n"))
	output.Write([]byte("2 2 3\nHTTP/1.1 200 OK\r\nResponse-Header: 1\r\n\r\n"))

	resp := <-producer.Successes()

	data, _ := resp.Value.Encode()

	if string(data) != `{"Req_URL":"/","Req_Type":"1","Req_ID":"2","Req_Ts":"3","Req_Method":"GET","Req_Headers":{"Header":"1"},"Res_Headers":{"Response-Header":"1"},"Res_Status":"200"}` {
		t.Error("Message not properly encoded: ", string(data))
	}
}

func TestOutputKafkaJSONWithResponseHeadersReverseOrder(t *testing.T) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	producer := mocks.NewAsyncProducer(t, config)
	producer.ExpectInputAndSucceed()

	output := NewKafkaOutput("", &KafkaConfig{
		producer:             producer,
		topic:                "test",
		useJSON:              true,
		trackResponseHeaders: true,
	})

	output.Write([]byte("2 2 3\nHTTP/1.1 200 OK\r\nResponse-Header: 1\r\n\r\n"))
	output.Write([]byte("1 2 3\nGET / HTTP1.1\r\nHeader: 1\r\n\r\n"))

	resp := <-producer.Successes()

	data, _ := resp.Value.Encode()

	if string(data) != `{"Req_URL":"/","Req_Type":"1","Req_ID":"2","Req_Ts":"3","Req_Method":"GET","Req_Headers":{"Header":"1"},"Res_Headers":{"Response-Header":"1"},"Res_Status":"200"}` {
		t.Error("Message not properly encoded: ", string(data))
	}
}

func TestOutputKafkaJSONWithResponseHeadersNoInputResponses(t *testing.T) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	producer := mocks.NewAsyncProducer(t, config)

	output := NewKafkaOutput("", &KafkaConfig{
		producer:             producer,
		topic:                "test",
		useJSON:              true,
		trackResponseHeaders: true,
	})

	for i := 0; i < maxPending; i++ {
		output.Write([]byte(fmt.Sprintf("1 %v 3\nGET / HTTP1.1\r\nHeader: 1\r\n\r\n", i)))
	}

	_, e := output.Write([]byte(fmt.Sprintf("1 %v 3\nGET / HTTP1.1\r\nHeader: 1\r\n\r\n", maxPending+1)))
	if e == nil {
		t.Error("Expected write to fail")
	}
}
