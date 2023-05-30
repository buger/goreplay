package goreplay

import (
	"bufio"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTCPOutput(t *testing.T) {
	wg := new(sync.WaitGroup)

	listener := startTCP(func(data []byte) {
		wg.Done()
	})
	output := NewTCPOutput(listener.Addr().String(), &TCPOutputConfig{Workers: 10})
	runTCPOutput(wg, output, 10, false)
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

func getTestBytes() *Message {
	return &Message{
		Meta: payloadHeader(RequestPayload, uuid(), time.Now().UnixNano(), -1),
		Data: []byte("GET / HTTP/1.1\r\nHost: www.w3.org\r\nUser-Agent: Go 1.1 package http\r\nAccept-Encoding: gzip\r\n\r\n"),
	}
}

func TestTCPOutputGetInitMessage(t *testing.T) {
	wg := new(sync.WaitGroup)

	var dataList [][]byte
	listener := startTCP(func(data []byte) {
		dataList = append(dataList, data)
		wg.Done()
	})
	getInitMessage := func() *Message {
		return &Message{
			Meta: []byte{},
			Data: []byte("test1"),
		}
	}
	output := NewTCPOutput(listener.Addr().String(), &TCPOutputConfig{Workers: 1, GetInitMessage: getInitMessage})

	runTCPOutput(wg, output, 1, true)

	if assert.Equal(t, 2, len(dataList)) {
		assert.Equal(t, "test1", string(dataList[0]))
	}
}

func TestTCPOutputGetInitMessageAndWriteBeforeMessage(t *testing.T) {
	wg := new(sync.WaitGroup)

	var dataList [][]byte
	listener := startTCP(func(data []byte) {
		dataList = append(dataList, data)
		wg.Done()
	})
	getInitMessage := func() *Message {
		return &Message{
			Meta: []byte{},
			Data: []byte("test2"),
		}
	}
	writeBeforeMessage := func(conn net.Conn, _ *Message) error {
		_, err := conn.Write([]byte("before"))
		return err
	}
	output := NewTCPOutput(listener.Addr().String(), &TCPOutputConfig{Workers: 1, GetInitMessage: getInitMessage, WriteBeforeMessage: writeBeforeMessage})

	runTCPOutput(wg, output, 1, true)

	if assert.Equal(t, 2, len(dataList)) {
		assert.Equal(t, "beforetest2", string(dataList[0]))
		assert.True(t, strings.HasPrefix(string(dataList[1]), "before"))
	}
}

func runTCPOutput(wg *sync.WaitGroup, output PluginWriter, repeat int, initMessage bool) {
	input := NewTestInput()
	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}

	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)

	if initMessage {
		wg.Add(1)
	}
	for i := 0; i < repeat; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()
	emitter.Close()
}
