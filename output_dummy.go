package main

import (
	"os"
)

// DummyOutput used for debugging, prints all incoming requests
type DummyOutput struct {
}

// NewDummyOutput constructor for DummyOutput
func NewDummyOutput() (di *DummyOutput) {
	di = new(DummyOutput)

	return
}

func (i *DummyOutput) Write(data []byte) (int, error) {
	n, err := os.Stdout.Write(data)
	os.Stdout.Write([]byte{'\n'})
	return n, err
}

func (i *DummyOutput) String() string {
	return "Dummy Output"
}
