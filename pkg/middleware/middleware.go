package middleware

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

	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/prettify"
	"github.com/buger/goreplay/pkg/proto"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Middleware represents a middleware object
type Middleware struct {
	Command       string
	Data          chan *plugin.Message
	Stdin         io.Writer
	Stdout        io.Reader
	CommandCancel context.CancelFunc
	Stop          chan bool // Channel used only to indicate goroutine should shutdown
	closed        bool
	config        *MiddlewareConfig
	mu            sync.RWMutex
}

// MiddlewareConfig represents a middleware configuration
type MiddlewareConfig struct {
	PrettifyHTTP bool
}

// NewMiddleware returns new middleware
func NewMiddleware(command string, config *MiddlewareConfig) *Middleware {
	m := new(Middleware)
	m.Command = command
	m.Data = make(chan *plugin.Message, 1000)
	m.Stop = make(chan bool)
	m.config = config
	if m.config == nil {
		m.config = &MiddlewareConfig{}
	}

	commands := strings.Split(command, " ")
	ctx, cancl := context.WithCancel(context.Background())
	m.CommandCancel = cancl
	cmd := exec.CommandContext(ctx, commands[0], commands[1:]...)

	m.Stdout, _ = cmd.StdoutPipe()
	m.Stdin, _ = cmd.StdinPipe()

	cmd.Stderr = os.Stderr

	go m.Read(m.Stdout)

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

			log.Error().Err(err).Msgf("[MIDDLEWARE] command[%q] error", command)
		}
	}()

	return m
}

// ReadFrom start a worker to read from this plugin
func (m *Middleware) ReadFrom(plugin plugin.Reader) {
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		log.Debug().Msgf("command[%q] Starting reading from %q", m.Command, plugin)
	}

	go m.copy(m.Stdin, plugin)
}

func (m *Middleware) copy(to io.Writer, from plugin.Reader) {
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
		if m.config != nil && m.config.PrettifyHTTP {
			buf = prettify.PrettifyHTTP(msg.Data)
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

// Read reads from this plugin
func (m *Middleware) Read(from io.Reader) {
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
			log.Error().Err(err).Msgf("[MIDDLEWARE] command[%q] failed to decode", m.Command)
			continue
		}
		var msg plugin.Message
		msg.Meta, msg.Data = proto.PayloadMetaWithBody(buf)
		select {
		case <-m.Stop:
			return
		case m.Data <- &msg:
		}
	}

}

// PluginRead reads message from this plugin
func (m *Middleware) PluginRead() (msg *plugin.Message, err error) {
	select {
	case <-m.Stop:
		return nil, plugin.ErrorStopped
	case msg = <-m.Data:
	}

	return
}

func (m *Middleware) String() string {
	return fmt.Sprintf("Modifying traffic using %q command", m.Command)
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
	m.CommandCancel()
	close(m.Stop)
	m.closed = true
	return nil
}
