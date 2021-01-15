package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

// Middleware represents a middleware object
type Middleware struct {
	command       string
	data          chan *Message
	Stdin         io.Writer
	Stdout        io.Reader
	commandCancel context.CancelFunc
	stop          chan bool // Channel used only to indicate goroutine should shutdown
	closed        bool
	mu            sync.RWMutex
}

// NewMiddleware returns new middleware
func NewMiddleware(command string) *Middleware {
	m := new(Middleware)
	m.command = command
	m.data = make(chan *Message, 1000)
	m.stop = make(chan bool)

	commands := strings.Split(command, " ")
	ctx, cancl := context.WithCancel(context.Background())
	m.commandCancel = cancl
	cmd := exec.CommandContext(ctx, commands[0], commands[1:]...)

	m.Stdout, _ = cmd.StdoutPipe()
	m.Stdin, _ = cmd.StdinPipe()

	cmd.Stderr = os.Stderr

	go m.read(m.Stdout)

	go func() {
		defer m.Close()
		var err error
		if err = cmd.Start(); err == nil {
			err = cmd.Wait()
		}
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				status := e.Sys().(syscall.WaitStatus)
				if status.Signal() == syscall.SIGKILL /*killed or context canceld */ {
					return
				}
			}
			Debug(0, fmt.Sprintf("[MIDDLEWARE] command[%q] error: %q", command, err.Error()))
		}
	}()

	return m
}

// ReadFrom start a worker to read from this plugin
func (m *Middleware) ReadFrom(plugin PluginReader) {
	Debug(2, fmt.Sprintf("[MIDDLEWARE] command[%q] Starting reading from %q", m.command, plugin))
	go m.copy(m.Stdin, plugin)
}

func (m *Middleware) copy(to io.Writer, from PluginReader) {
	var buf, dst []byte

	for {
		msg, err := from.PluginRead()
		if err != nil {
			return
		}
		if msg == nil || len(msg.Data) == 0 {
			continue
		}
		buf = msg.Data
		if Settings.PrettifyHTTP {
			buf = prettifyHTTP(msg.Data)
		}
		dstLen := (len(buf)+len(msg.Meta))*2 + 1
		// if enough space was previously allocated use it instead
		if dstLen > len(dst) {
			dst = make([]byte, dstLen)
		}
		n := hex.Encode(dst, msg.Meta)
		n += hex.Encode(dst[n:], buf)
		dst[n] = '\n'

		n, err = to.Write(dst[:n+1])
		if err == nil {
			continue
		}
		if m.isClosed() {
			return
		}
	}
}

func (m *Middleware) read(from io.Reader) {
	reader := bufio.NewReader(from)
	var line []byte
	var e error
	for {
		if line, e = reader.ReadBytes('\n'); e != nil {
			if m.isClosed() {
				return
			}
			continue
		}
		buf := make([]byte, (len(line)-1)/2)
		if _, err := hex.Decode(buf, line[:len(line)-1]); err != nil {
			Debug(0, fmt.Sprintf("[MIDDLEWARE] command[%q] failed to decode err: %q", m.command, err))
			continue
		}
		var msg Message
		msg.Meta, msg.Data = payloadMetaWithBody(buf)
		select {
		case <-m.stop:
			return
		case m.data <- &msg:
		}
	}

}

// PluginRead reads message from this plugin
func (m *Middleware) PluginRead() (msg *Message, err error) {
	select {
	case <-m.stop:
		return nil, ErrorStopped
	case msg = <-m.data:
	}

	return
}

func (m *Middleware) String() string {
	return fmt.Sprintf("Modifying traffic using %q command", m.command)
}

func (m *Middleware) isClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closed
}

// Close closes this plugin
func (m *Middleware) Close() error {
	if m.isClosed() {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commandCancel()
	close(m.stop)
	m.closed = true
	return nil
}
