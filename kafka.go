package goreplay

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/buger/goreplay/proto"
	"io/ioutil"
	"log"

	"github.com/Shopify/sarama"
	"github.com/xdg-go/scram"
)

// SASLKafkaConfig SASL configuration
type SASLKafkaConfig struct {
	UseSASL   bool   `json:"input-kafka-use-sasl"`
	Mechanism string `json:"input-kafka-mechanism"`
	Username  string `json:"input-kafka-username"`
	Password  string `json:"input-kafka-password"`
}

// InputKafkaConfig should contains required information to
// build producers.
type InputKafkaConfig struct {
	consumer   sarama.Consumer
	Host       string `json:"input-kafka-host"`
	Topic      string `json:"input-kafka-topic"`
	UseJSON    bool   `json:"input-kafka-json-format"`
	Offset     int64  `json:"input-kafka-offset"`
	SASLConfig SASLKafkaConfig
}

// OutputKafkaConfig is the representation of kfka output configuration
type OutputKafkaConfig struct {
	producer   sarama.AsyncProducer
	Host       string `json:"output-kafka-host"`
	Topic      string `json:"output-kafka-topic"`
	UseJSON    bool   `json:"output-kafka-json-format"`
	SASLConfig SASLKafkaConfig
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
func NewKafkaConfig(saslConfig *SASLKafkaConfig, tlsConfig *KafkaTLSConfig) *sarama.Config {
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
	if saslConfig.UseSASL {
		mechanism := sarama.SASLMechanism(saslConfig.Mechanism)
		config.Net.SASL.Enable = saslConfig.UseSASL
		config.Net.SASL.Mechanism = mechanism
		config.Net.SASL.User = saslConfig.Username
		config.Net.SASL.Password = saslConfig.Password
		if mechanism == sarama.SASLTypeSCRAMSHA256 {
			config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
		} else if mechanism == sarama.SASLTypeSCRAMSHA512 {
			config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
		}
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

var (
	// SHA256 SASLMechanism
	SHA256 scram.HashGeneratorFcn = sha256.New
	// SHA512 SASLMechanism
	SHA512 scram.HashGeneratorFcn = sha512.New
)

// XDGSCRAMClient for SASL-Protocol
type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

// Begin of XDGSCRAMClient
func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

// Step of XDGSCRAMClient
func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

// Done of XDGSCRAMClient
func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
