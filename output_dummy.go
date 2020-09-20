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
	return os.Stdout.Write(data)
}

func (i *DummyOutput) String() string {
	return "Dummy Output"
}
