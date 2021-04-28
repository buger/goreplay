package main

import (
	"bufio"
	"log"
	"net"
	"sync"
	"testing"
	"time"
)

func TestTCPOutput(t *testing.T) {
	wg := new(sync.WaitGroup)

	listener := startTCP(func(data []byte) {
		wg.Done()
	})
	input := NewTestInput()
	output := NewTCPOutput(listener.Addr().String(), &TCPOutputConfig{Workers: 10})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}

	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()
	emitter.Close()
}

func startTCP(cb func([]byte)) net.Listener {
	listener, err := net.Listen("tcp", "127.0.0.1:0")

	if err != nil {
		log.Fatal("Can't start:", err)
	}

	go func() {
		for {
			conn, _ := listener.Accept()

			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				scanner := bufio.NewScanner(reader)
				scanner.Split(payloadScanner)

				for scanner.Scan() {
					cb(scanner.Bytes())
				}
			}(conn)
		}
	}()

	return listener
}

func BenchmarkTCPOutput(b *testing.B) {
	wg := new(sync.WaitGroup)

	listener := startTCP(func(data []byte) {
		wg.Done()
	})
	input := NewTestInput()
	input.data = make(chan []byte, b.N)
	for i := 0; i < b.N; i++ {
		input.EmitGET()
	}
	wg.Add(b.N)
	output := NewTCPOutput(listener.Addr().String(), &TCPOutputConfig{Workers: 10})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}

	emitter := NewEmitter()
	// avoid counting above initialization
	b.ResetTimer()
	go emitter.Start(plugins, Settings.Middleware)

	wg.Wait()
	emitter.Close()
}

func TestStickyDisable(t *testing.T) {
	tcpOutput := TCPOutput{config: &TCPOutputConfig{Sticky: false, Workers: 10}}

	for i := 0; i < 10; i++ {
		index := tcpOutput.getBufferIndex(getTestBytes())
		if index != (i+1)%10 {
			t.Errorf("Sticky is disable. Got: %d want %d", index, (i+1)%10)
		}
	}
}

func TestBufferDistribution(t *testing.T) {
	numberOfWorkers := 10
	numberOfMessages := 10000
	percentDistributionErrorRange := 20

	buffer := make([]int, numberOfWorkers)
	tcpOutput := TCPOutput{config: &TCPOutputConfig{Sticky: true, Workers: 10}}
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
