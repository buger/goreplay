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

type ExternalMiddleware struct {
	command string

	output chan []byte

	mu sync.Mutex

	Stdin  io.Writer
	Stdout io.Reader
}

func NewExternalMiddleware(command string) *ExternalMiddleware {
	m := new(ExternalMiddleware)
	m.command = command

	m.output = make(chan []byte, 1000)

	commands := strings.Split(command, " ")
	cmd := exec.Command(commands[0], commands[1:]...)

	m.Stdout, _ = cmd.StdoutPipe()
	m.Stdin, _ = cmd.StdinPipe()

	if Settings.verbose {
		cmd.Stderr = os.Stderr
	}

	go m.read()

	go func() {
		err := cmd.Start()

		if err != nil {
			log.Fatal(err)
		}

		cmd.Wait()
	}()

	return m
}

func (m *ExternalMiddleware) read() {
	scanner := bufio.NewScanner(m.Stdout)

	for scanner.Scan() {
		bytes := scanner.Bytes()
		buf := make([]byte, len(bytes)/2)
		if _, err := hex.Decode(buf, bytes); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to decode input payload", err, len(bytes))
		}

		if Settings.debug {
			Debug("[MIDDLEWARE-MASTER] Received:", string(buf))
		}

		m.output <- buf
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Traffic modifier command failed:", err)
	}

	return
}

func (m *ExternalMiddleware) Read(data []byte) (int, error) {
	buf := <-m.output
	copy(data, buf)

	return len(buf), nil
}

func (m *ExternalMiddleware) Write(data []byte) (int, error) {
	dst := make([]byte, len(data)*2+1)

	hex.Encode(dst, data)
	dst[len(dst)-1] = '\n'

	m.mu.Lock()
	m.Stdin.Write(dst)
	m.mu.Unlock()

	if Settings.debug {
		Debug("[MIDDLEWARE-MASTER] Sending:", string(data))
	}

	return len(data), nil
}

func (m *ExternalMiddleware) String() string {
	return fmt.Sprintf("Modifying traffic using '%s' command", m.command)
}
