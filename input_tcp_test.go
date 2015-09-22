package main

import (
	"log"
	"net"
	"sync"
	"testing"
)

func TestTCPInput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTCPInput(":0")
	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})

	testPlugins(input, output)

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

func BenchmarkTCPInput(b *testing.B) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTCPInput(":0")
	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})

	testPlugins(input, output)

	go Start(quit)

	tcpAddr, err := net.ResolveTCPAddr("tcp", input.listener.Addr().String())

	if err != nil {
		log.Fatal(err)
	}

	var connections []net.Conn

	// Creating simple pool of workers, same as output_tcp have
	dataChan := make(chan []byte, 1000)

	for i := 0; i < 10; i++ {
		conn, _ := net.DialTCP("tcp", nil, tcpAddr)
		connections = append(connections, conn)

		go func(conn net.Conn) {
			for {
				data := <-dataChan

				buf := make([]byte, len(data)+2)
				data = append(data, []byte("¶")...)
				copy(buf, data)
				conn.Write(buf)
			}
		}(conn)
	}

	if err != nil {
		log.Fatal(err)
	}

	msg := []byte("GET / HTTP/1.1\r\n\r\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		dataChan <- msg
	}

	wg.Wait()

	for _, conn := range connections {
		conn.Close()
	}

	close(quit)
}
