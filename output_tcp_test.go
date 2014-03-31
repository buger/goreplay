package main

import (
	"bufio"
	"io"
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

	Plugins.Inputs = []io.Reader{input}
	Plugins.Outputs = []io.Writer{output}

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
			
			go func() {
				reader := bufio.NewReader(conn)
				for {
					buf,err := reader.ReadBytes('¶')
					new_buf_len := len(buf) - 2
					new_buf := make([]byte, new_buf_len)
					copy(new_buf, buf[:new_buf_len])
					if err != nil {
						if err != io.EOF {
							log.Printf("error: %s\n", err)
						}
					}
					cb(new_buf)
				}
				conn.Close()
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

	Plugins.Inputs = []io.Reader{input}
	Plugins.Outputs = []io.Writer{output}

	go Start(quit)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()

	close(quit)
}
