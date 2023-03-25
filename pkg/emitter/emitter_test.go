package emitter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/buger/goreplay/pkg/http_modifier"
	"github.com/buger/goreplay/pkg/middleware"
	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/pro"
	"github.com/buger/goreplay/pkg/proto"
	"github.com/buger/goreplay/pkg/test"
)

func TestMain(m *testing.M) {
	pro.Enable()
	code := m.Run()
	os.Exit(code)
}

func TestEmitter(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := test.NewTestInput()
	output := test.NewTestOutput(func(*plugin.Message) {
		wg.Done()
	})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := New()
	go emitter.Start(plugins)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()
	emitter.Close()
}

func TestEmitterFiltered(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := test.NewTestInput()
	input.SkipHeader = true

	output := test.NewTestOutput(func(*plugin.Message) {
		wg.Done()
	})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	methods := http_modifier.HTTPMethods{[]byte("GET")}
	emitter := New(&Config{
		ModifierConfig: http_modifier.HTTPModifierConfig{Methods: methods},
	})
	go emitter.Start(plugins)

	wg.Add(2)

	id := proto.UUID()
	reqh := proto.PayloadHeader(proto.RequestPayload, id, time.Now().UnixNano(), -1)
	reqb := append(reqh, []byte("POST / HTTP/1.1\r\nHost: www.w3.org\r\nUser-Agent: Go 1.1 package http\r\nAccept-Encoding: gzip\r\n\r\n")...)

	resh := proto.PayloadHeader(proto.ResponsePayload, id, time.Now().UnixNano()+1, 1)
	respb := append(resh, []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")...)

	input.EmitBytes(reqb)
	input.EmitBytes(respb)

	id = proto.UUID()
	reqh = proto.PayloadHeader(proto.RequestPayload, id, time.Now().UnixNano(), -1)
	reqb = append(reqh, []byte("GET / HTTP/1.1\r\nHost: www.w3.org\r\nUser-Agent: Go 1.1 package http\r\nAccept-Encoding: gzip\r\n\r\n")...)

	resh = proto.PayloadHeader(proto.ResponsePayload, id, time.Now().UnixNano()+1, 1)
	respb = append(resh, []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")...)

	input.EmitBytes(reqb)
	input.EmitBytes(respb)

	wg.Wait()
	emitter.Close()
}

func TestEmitterSplitRoundRobin(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := test.NewTestInput()

	var counter1, counter2 int32

	output1 := test.NewTestOutput(func(*plugin.Message) {
		atomic.AddInt32(&counter1, 1)
		wg.Done()
	})

	output2 := test.NewTestOutput(func(*plugin.Message) {
		atomic.AddInt32(&counter2, 1)
		wg.Done()
	})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output1, output2},
	}

	emitter := New(&Config{
		SplitOutput: true,
	})
	go emitter.Start(plugins)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()

	emitter.Close()

	if counter1 == 0 || counter2 == 0 || counter1 != counter2 {
		t.Errorf("Round robin should split traffic equally: %d vs %d", counter1, counter2)
	}
}

func TestEmitterRoundRobin(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := test.NewTestInput()

	var counter1, counter2 int32

	output1 := test.NewTestOutput(func(*plugin.Message) {
		counter1++
		wg.Done()
	})

	output2 := test.NewTestOutput(func(*plugin.Message) {
		counter2++
		wg.Done()
	})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output1, output2},
	}
	plugins.All = append(plugins.All, input, output1, output2)

	emitter := New(&Config{
		SplitOutput: true,
	})
	go emitter.Start(plugins)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()
	emitter.Close()

	if counter1 == 0 || counter2 == 0 {
		t.Errorf("Round robin should split traffic equally: %d vs %d", counter1, counter2)
	}
}

func TestEmitterSplitSession(t *testing.T) {
	wg := new(sync.WaitGroup)
	wg.Add(200)

	input := test.NewTestInput()
	input.SkipHeader = true

	var counter1, counter2 int32

	output1 := test.NewTestOutput(func(msg *plugin.Message) {
		if proto.PayloadID(msg.Meta)[0] == 'a' {
			counter1++
		}
		wg.Done()
	})

	output2 := test.NewTestOutput(func(msg *plugin.Message) {
		if proto.PayloadID(msg.Meta)[0] == 'b' {
			counter2++
		}
		wg.Done()
	})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output1, output2},
	}

	emitter := New(&Config{
		SplitOutput:          true,
		RecognizeTCPSessions: true,
	})
	go emitter.Start(plugins)

	for i := 0; i < 200; i++ {
		// Keep session but randomize
		id := make([]byte, 20)
		if i&1 == 0 { // for recognizeTCPSessions one should be odd and other will be even number
			id[0] = 'a'
		} else {
			id[0] = 'b'
		}
		input.EmitBytes([]byte(fmt.Sprintf("1 %s 1 1\nGET / HTTP/1.1\r\n\r\n", id[:20])))
	}

	wg.Wait()

	if counter1 != counter2 {
		t.Errorf("Round robin should split traffic equally: %d vs %d", counter1, counter2)
	}

	emitter.Close()
}

func BenchmarkEmitter(b *testing.B) {
	wg := new(sync.WaitGroup)

	input := test.NewTestInput()

	output := test.NewTestOutput(func(*plugin.Message) {
		wg.Done()
	})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := New(&Config{})
	go emitter.Start(plugins)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()
	emitter.Close()
}

const echoSh = "./examples/middleware/echo.sh"
const tokenModifier = "go run ./examples/middleware/token_modifier.go"

var withDebug = append(syscall.Environ(), "GOR_TEST=1")

func initMiddleware(cmd *exec.Cmd, cancl context.CancelFunc, l plugin.Reader, c func(error)) *middleware.Middleware {
	var m middleware.Middleware
	m.Data = make(chan *plugin.Message, 1000)
	m.Stop = make(chan bool)
	m.CommandCancel = cancl
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
			c(err)
		}
	}()
	m.ReadFrom(l)
	return &m
}

func initCmd(command string, env []string) (*exec.Cmd, context.CancelFunc) {
	commands := strings.Split(command, " ")
	ctx, cancl := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, commands[0], commands[1:]...)
	cmd.Env = env
	return cmd, cancl
}

func TestMiddlewareEarlyClose(t *testing.T) {
	t.Skip()
	quit := make(chan struct{})
	in := test.NewTestInput()
	cmd, cancl := initCmd(echoSh, withDebug)
	midd := initMiddleware(cmd, cancl, in, func(err error) {
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				status := e.Sys().(syscall.WaitStatus)
				if status.Signal() != syscall.SIGKILL {
					t.Errorf("expected error to be signal killed. got %s", status.Signal().String())
				}
			}
		}
		quit <- struct{}{}
	})
	var body = []byte("OPTIONS / HTTP/1.1\r\nHost: example.org\r\n\r\n")
	count := uint32(0)
	out := test.NewTestOutput(func(msg *plugin.Message) {
		if !bytes.Equal(body, msg.Data) {
			t.Errorf("expected %q to equal %q", body, msg.Data)
		}
		atomic.AddUint32(&count, 1)
		if atomic.LoadUint32(&count) == 5 {
			quit <- struct{}{}
		}
	})
	pl := &plugin.InOutPlugins{}
	pl.Inputs = []plugin.Reader{midd, in}
	pl.Outputs = []plugin.Writer{out}
	pl.All = []interface{}{midd, out, in}
	e := New()
	go e.Start(pl)
	for i := 0; i < 5; i++ {
		in.EmitBytes(body)
	}
	<-quit
	midd.Close()
	<-quit
}
