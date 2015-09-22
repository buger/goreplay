package main

import (
	"bufio"
	"log"
	"net"
	"sync"
	"testing"
)

func TestTCPOutput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	listener := startTCP(func(data []byte) {
		wg.Done()
	})
	input := NewTestInput()
	output := NewTCPOutput(listener.Addr().String())

	testPlugins(input, output)

	go Start(quit)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()

	close(quit)
}

func startTCP(cb func([]byte)) net.Listener {
	listener, err := net.Listen("tcp", ":0")

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
	output := NewTCPOutput(listener.Addr().String())

	testPlugins(input, output)

	go Start(quit)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()

	close(quit)
}
