package main

import (
	"fmt"
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
	fmt.Println(string(data))

	return len(data), nil
}

func (i *DummyOutput) String() string {
	return "Dummy Output"
}
