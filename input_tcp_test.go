package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

func TestTCPInput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTCPInput("127.0.0.1:0", &TCPInputConfig{})
	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})

	Plugins.Inputs = []io.Reader{input}
	Plugins.Outputs = []io.Writer{output}

	go Start(quit)

	tcpAddr, err := net.ResolveTCPAddr("tcp", input.listener.Addr().String())

	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)

	if err != nil {
		log.Fatal(err)
	}

	msg := []byte("1 1 1\nGET / HTTP/1.1\r\n\r\n")

	for i := 0; i < 100; i++ {
		wg.Add(1)
		conn.Write(msg)
		conn.Write([]byte(payloadSeparator))
	}

	wg.Wait()

	close(quit)
}

func genCertificate(template *x509.Certificate) ([]byte, []byte) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, serialNumberLimit)
	template.SerialNumber = serialNumber
	template.BasicConstraintsValid = true
	template.NotBefore = time.Now()
	template.NotAfter = time.Now().Add(time.Hour)

	derBytes, _ := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)

	var certPem, keyPem bytes.Buffer
	pem.Encode(&certPem, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	pem.Encode(&keyPem, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certPem.Bytes(), keyPem.Bytes()
}

func TestTCPInputSecure(t *testing.T) {
	serverCertPem, serverPrivPem := genCertificate(&x509.Certificate{
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::")},
	})

	serverCertPemFile, _ := ioutil.TempFile("", "server.crt")
	serverCertPemFile.Write(serverCertPem)
	serverCertPemFile.Close()

	serverPrivPemFile, _ := ioutil.TempFile("", "server.key")
	serverPrivPemFile.Write(serverPrivPem)
	serverPrivPemFile.Close()

	defer func() {
		os.Remove(serverPrivPemFile.Name())
		os.Remove(serverCertPemFile.Name())
	}()

	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTCPInput("127.0.0.1:0", &TCPInputConfig{
		secure:          true,
		certificatePath: serverCertPemFile.Name(),
		keyPath:         serverPrivPemFile.Name(),
	})
	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})

	Plugins.Inputs = []io.Reader{input}
	Plugins.Outputs = []io.Writer{output}

	go Start(quit)

	conf := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", input.listener.Addr().String(), conf)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	msg := []byte("1 1 1\nGET / HTTP/1.1\r\n\r\n")

	for i := 0; i < 100; i++ {
		wg.Add(1)
		conn.Write(msg)
		conn.Write([]byte(payloadSeparator))
	}

	wg.Wait()

	close(quit)
}
