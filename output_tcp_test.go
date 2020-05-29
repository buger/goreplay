package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"sync"
	"testing"
	"time"
)

func TestTCPOutput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	listener := startTCP(func(data []byte) {
		wg.Done()
	})
	input := NewTestInput()
	output := NewTCPOutput(listener.Addr().String(), &TCPOutputConfig{})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	go Start(plugins, quit)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()

	close(quit)
}

func startTCP(cb func([]byte)) net.Listener {
	listener, err := net.Listen("tcp", "127.0.0.1:0")

	if err != nil {
		log.Fatal("Can't start:", err)
	}

	go func() {
		for {
			conn, _ := listener.Accept()
			defer conn.Close()

			go func() {
				reader := bufio.NewReader(conn)
				scanner := bufio.NewScanner(reader)
				scanner.Split(payloadScanner)

				for scanner.Scan() {
					cb(scanner.Bytes())
				}
			}()
		}
	}()

	return listener
}

func BenchmarkTCPOutput(b *testing.B) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	listener := startTCP(func(data []byte) {
		wg.Done()
	})
	input := NewTestInput()
	output := NewTCPOutput(listener.Addr().String(), &TCPOutputConfig{})

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	go Start(plugins, quit)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()

	close(quit)
}

func TestStickyDisable(t *testing.T) {
	tcpOutput := TCPOutput{config: &TCPOutputConfig{sticky: false}}

	for i := 0; i < 1000; i++ {
		index := tcpOutput.getBufferIndex(getTestBytes())
		if index != 0 {
			t.Errorf("Sticky is disable. Got: %d want 0", index)
		}
	}
}

func TestBufferDistribution(t *testing.T) {
	numberOfWorkers := 10
	numberOfMessages := 1000000
	percentDistributionErrorRange := 20

	buffer := make([]int, numberOfWorkers)
	tcpOutput := TCPOutput{config: &TCPOutputConfig{sticky: true}}
	for i := 0; i < numberOfMessages; i++ {
		buffer[tcpOutput.getBufferIndex(getTestBytes())]++
	}

	expectedDistribution := numberOfMessages / numberOfWorkers
	lowerDistribution := expectedDistribution - (expectedDistribution * percentDistributionErrorRange / 100)
	upperDistribution := expectedDistribution + (expectedDistribution * percentDistributionErrorRange / 100)
	for i := 0; i < numberOfWorkers; i++ {
		if buffer[i] < lowerDistribution {
			t.Errorf("Under expected distribution. Got %d expected lower distribution %d", buffer[i], lowerDistribution)
		}
		if buffer[i] > upperDistribution {
			t.Errorf("Under expected distribution. Got %d expected upper distribution %d", buffer[i], upperDistribution)
		}
	}
}

func getTestBytes() []byte {
	reqh := payloadHeader(RequestPayload, uuid(), time.Now().UnixNano(), -1)
	reqb := append(reqh, []byte("GET / HTTP/1.1\r\nHost: www.w3.org\r\nUser-Agent: Go 1.1 package http\r\nAccept-Encoding: gzip\r\n\r\n")...)
	return reqb
}
