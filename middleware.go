package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type Middleware struct {
	command string

	data chan []byte

	mu sync.Mutex

	Stdin  io.Writer
	Stdout io.Reader

	stop chan bool // Channel used only to indicate goroutine should shutdown
}

func NewMiddleware(command string) *Middleware {
	m := new(Middleware)
	m.command = command
	m.data = make(chan []byte, 1000)
	m.stop = make(chan bool)

	commands := strings.Split(command, " ")
	cmd := exec.Command(commands[0], commands[1:]...)

	m.Stdout, _ = cmd.StdoutPipe()
	m.Stdin, _ = cmd.StdinPipe()

	cmd.Stderr = os.Stderr

	go m.read(m.Stdout)

	go func() {
		err := cmd.Start()

		if err != nil {
			log.Fatal(err)
		}

		err = cmd.Wait()

		if err != nil {
			log.Fatal(err)
		}
	}()

	return m
}

func (m *Middleware) ReadFrom(plugin io.Reader) {
	Debug("[MIDDLEWARE-MASTER] Starting reading from", plugin)
	go m.copy(m.Stdin, plugin)
}

func (m *Middleware) copy(to io.Writer, from io.Reader) {
	buf := make([]byte, 5*1024*1024)
	dst := make([]byte, len(buf)*4)

	for {
		nr, _ := from.Read(buf)
		if nr == 0 || nr > len(buf) {
			continue
		}

		payload := buf[0:nr]

		if Settings.prettifyHTTP() {
			payload = prettifyHTTP(payload)
			nr = len(payload)

			if nr*2 > len(dst) {
				continue
			}
		}

		if Settings.prettifyHTTP() {
			payload = prettifyHTTP(payload)
			nr = len(payload)
		}

		hex.Encode(dst, payload)
		dst[nr*2] = '\n'

		m.mu.Lock()
		to.Write(dst[0 : nr*2+1])
		m.mu.Unlock()

		if Settings.debug() {
			Debug("[MIDDLEWARE-MASTER] Sending:", string(buf[0:nr]), "From:", from)
		}
	}
}

func (m *Middleware) read(from io.Reader) {
	reader := bufio.NewReader(from)
	var line []byte
	var e error

	for {
		if line, e = reader.ReadBytes('\n'); e != nil {
			if e == io.EOF {
				continue
			} else {
				break
			}
		}

		buf := make([]byte, len(line)/2)
		if _, err := hex.Decode(buf, line[:len(line)-1]); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to decode input payload", err, len(line), string(line[:len(line)-1]))
		}

		if Settings.debug() {
			Debug("[MIDDLEWARE-MASTER] Received:", string(buf))
		}

		select {
		case <-m.stop:
			return
		case m.data <- buf:
		}
	}

	return
}

func (m *Middleware) Read(data []byte) (int, error) {
	var buf []byte
	select {
	case <-m.stop:
		return 0, ErrorStopped
	case buf = <-m.data:
	}

	copy(data, buf)
	return len(buf), nil
}

func (m *Middleware) String() string {
	return fmt.Sprintf("Modifying traffic using '%s' command", m.command)
}

func (m *Middleware) Close() error {
	close(m.stop)
	return nil
}
