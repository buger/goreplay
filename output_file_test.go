package main

import (
	"sync"
	"testing"
)

func TestFileOutput(t *testing.T) {
	wg := new(sync.WaitGroup)
	quit := make(chan int)

	input := NewTestInput()
	output := NewFileOutput("/tmp/test_requests.gor")

	testPlugins(input, output)

	go Start(quit)

	for i := 0; i < 100; i++ {
		wg.Add(2)
		input.EmitGET()
		input.EmitPOST()
	}
	close(quit)

	quit = make(chan int)

	input2 := NewFileInput("/tmp/test_requests.gor")
	output2 := NewTestOutput(func(data []byte) {
		wg.Done()
	})

	testPlugins(input2, output2)

	go Start(quit)

	wg.Wait()
	close(quit)
}
