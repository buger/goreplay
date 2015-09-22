package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestEmitter(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()
	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})

	testPlugins(input, output)

	go Start(quit)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()

	close(quit)
}

func TestEmitterRoundRobin(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()

	var counter1, counter2 int32

	output1 := NewTestOutput(func(data []byte) {
		atomic.AddInt32(&counter1, 1)
		wg.Done()
	})

	output2 := NewTestOutput(func(data []byte) {
		atomic.AddInt32(&counter2, 1)
		wg.Done()
	})

	testPlugins(input, output1, output2)

	Settings.splitOutput = true

	go Start(quit)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()

	close(quit)

	if counter1 == 0 || counter2 == 0 {
		t.Errorf("Round robin should split traffic equally: %d vs %d", counter1, counter2)
	}

	Settings.splitOutput = false
}

func BenchmarkEmitter(b *testing.B) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()

	output := NewTestOutput(func(data []byte) {
		wg.Done()
	})

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
