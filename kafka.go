package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/Shopify/sarama"
	"github.com/buger/goreplay/proto"
)

// InputKafkaConfig should contains required information to
// build producers.
type InputKafkaConfig struct {
	producer sarama.AsyncProducer
	consumer sarama.Consumer
	Host     string `json:"input-kafka-host"`
	Topic    string `json:"input-kafka-topic"`
	UseJSON  bool   `json:"input-kafka-json-format"`
}

// OutputKafkaConfig is the representation of kfka output configuration
type OutputKafkaConfig struct {
	producer sarama.AsyncProducer
	consumer sarama.Consumer
	Host     string `json:"output-kafka-host"`
	Topic    string `json:"output-kafka-topic"`
	UseJSON  bool   `json:"output-kafka-json-format"`
}

// KafkaTLSConfig should contains TLS certificates for connecting to secured Kafka clusters
type KafkaTLSConfig struct {
	CACert     string `json:"kafka-tls-ca-cert"`
	ClientCert string `json:"kafka-tls-client-cert"`
	ClientKey  string `json:"kafka-tls-client-key"`
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

// NewTLSConfig loads TLS certificates
func NewTLSConfig(clientCertFile, clientKeyFile, caCertFile string) (*tls.Config, error) {
	tlsConfig := tls.Config{}

	if clientCertFile != "" && clientKeyFile == "" {
		return &tlsConfig, errors.New("Missing key of client certificate in kafka")
	}
	if clientCertFile == "" && clientKeyFile != "" {
		return &tlsConfig, errors.New("missing TLS client certificate in kafka")
	}
	// Load client cert
	if (clientCertFile != "") && (clientKeyFile != "") {
		cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			return &tlsConfig, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	// Load CA cert
	if caCertFile != "" {
		caCert, err := ioutil.ReadFile(caCertFile)
		if err != nil {
			return &tlsConfig, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}
	return &tlsConfig, nil
}

// NewKafkaConfig returns Kafka config with or without TLS
func NewKafkaConfig(tlsConfig *KafkaTLSConfig) *sarama.Config {
	config := sarama.NewConfig()
	// Configuration options go here
	if tlsConfig != nil && (tlsConfig.ClientCert != "" || tlsConfig.CACert != "") {
		config.Net.TLS.Enable = true
		tlsConfig, err := NewTLSConfig(tlsConfig.ClientCert, tlsConfig.ClientKey, tlsConfig.CACert)
		if err != nil {
			log.Fatal(err)
		}
		config.Net.TLS.Config = tlsConfig
	}
	return config
}

// Dump returns the given request in its HTTP/1.x wire
// representation.
func (m KafkaMessage) Dump() ([]byte, error) {
	var b bytes.Buffer

	b.WriteString(fmt.Sprintf("%s %s %s\n", m.ReqType, m.ReqID, m.ReqTs))
	b.WriteString(fmt.Sprintf("%s %s HTTP/1.1", m.ReqMethod, m.ReqURL))
	b.Write(proto.CRLF)
	for key, value := range m.ReqHeaders {
		b.WriteString(fmt.Sprintf("%s: %s", key, value))
		b.Write(proto.CRLF)
	}

	b.Write(proto.CRLF)
	b.WriteString(m.ReqBody)

	return b.Bytes(), nil
}
