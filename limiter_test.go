// +build !race

package main

import (
	"io"
	"sync"
	"testing"
)

func TestOutputLimiter(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()
	output := NewLimiter(NewTestOutput(func(data []byte) {
		wg.Done()
	}), "10")
	wg.Add(10)

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	go Start(plugins, quit)

	for i := 0; i < 100; i++ {
		input.EmitGET()
	}

	wg.Wait()

	close(quit)
}

func TestInputLimiter(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewLimiter(NewTestInput(), "10")
	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})
	wg.Add(10)

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	go Start(plugins, quit)

	for i := 0; i < 100; i++ {
		input.(*Limiter).plugin.(*TestInput).EmitGET()
	}

	wg.Wait()

	close(quit)
}

// Should limit all requests
func TestPercentLimiter1(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()
	output := NewLimiter(NewTestOutput(func(data []byte) {
		wg.Done()
	}), "0%")

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	go Start(plugins, quit)

	for i := 0; i < 100; i++ {
		input.EmitGET()
	}

	wg.Wait()

	close(quit)
}

// Should not limit at all
func TestPercentLimiter2(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()
	output := NewLimiter(NewTestOutput(func(data []byte) {
		wg.Done()
	}), "100%")
	wg.Add(100)

	plugins := &InOutPlugins{
		Inputs:  []io.Reader{input},
		Outputs: []io.Writer{output},
	}

	go Start(plugins, quit)

	for i := 0; i < 100; i++ {
		input.EmitGET()
	}

	wg.Wait()

	close(quit)
}
