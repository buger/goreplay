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
}

func NewMiddleware(command string) *Middleware {
	m := new(Middleware)
	m.command = command
	m.data = make(chan []byte, 1000)

	commands := strings.Split(command, " ")
	cmd := exec.Command(commands[0], commands[1:]...)

	m.Stdout, _ = cmd.StdoutPipe()
	m.Stdin, _ = cmd.StdinPipe()

	if Settings.verbose {
		cmd.Stderr = os.Stderr
	}

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
	dst := make([]byte, len(buf)*2)

	for {
		nr, _ := from.Read(buf)
		if nr > 0 && len(buf) > nr {

			hex.Encode(dst, buf[0:nr])
			dst[nr*2] = '\n'

			m.mu.Lock()
			to.Write(dst[0 : nr*2+1])
			m.mu.Unlock()

			if Settings.debug {
				Debug("[MIDDLEWARE-MASTER] Sending:", string(buf[0:nr]), "From:", from)
			}
		}
	}
}

func (m *Middleware) read(from io.Reader) {
	scanner := bufio.NewScanner(from)
	reader := bufio.NewReader(from)
        var bytes []byte
	var e error
	bytes, e = reader.ReadBytes('\n')
	for e == nil {
		buf := make([]byte, len(bytes)/2)
		if _, err := hex.Decode(buf, bytes[:len(bytes)-1]); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to decode input payload", err, len(bytes))
		}

		if Settings.debug {
			Debug("[MIDDLEWARE-MASTER] Received:", string(buf))
		}

		m.data <- buf
		bytes, e = reader.ReadBytes('\n')
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Traffic modifier command failed:", err)
	}

	return
}

func (m *Middleware) Read(data []byte) (int, error) {
	buf := <-m.data
	copy(data, buf)

	return len(buf), nil
}

func (m *Middleware) String() string {
	return fmt.Sprintf("Modifying traffic using '%s' command", m.command)
}
