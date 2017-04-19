package main

import (
	"bytes"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/buger/gor/proto"
)

// KafkaConfig should contains required information to
// build producers.
type KafkaConfig struct {
	host     string
	topic    string
	producer sarama.AsyncProducer
	consumer sarama.Consumer
	useJSON  bool
}

// KafkaMessage should contains catched request information that should be
// passed as Json to Apache Kafka.
type KafkaMessage struct {
	ReqURL     string            `json:"Req_URL"`
	ReqType    string            `json:"Req_Type"`
	ReqID      string            `json:"Req_ID"`
	ReqTs      string            `json:"Req_Ts"`
	ReqMethod  string            `json:"Req_Method"`
	ReqBody    string            `json:"Req_Body,omitempty"`
	ReqHeaders map[string]string `json:"Req_Headers,omitempty"`
}

// Dump returns the given request in its HTTP/1.x wire
// representation.
func (m KafkaMessage) Dump() ([]byte, error) {
	var b bytes.Buffer

	b.WriteString(fmt.Sprintf("%s %s %s\n", m.ReqType, m.ReqID, m.ReqTs))
	b.WriteString(fmt.Sprintf("%s %s HTTP/1.1", m.ReqMethod, m.ReqURL))
	b.Write(proto.CLRF)
	for key, value := range m.ReqHeaders {
		b.WriteString(fmt.Sprintf("%s: %s", key, value))
		b.Write(proto.CLRF)
	}

	b.Write(proto.CLRF)
	b.WriteString(m.ReqBody)

	return b.Bytes(), nil
}
