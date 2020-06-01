package main

// NullOutput used for debugging, prints nothing
type NullOutput struct {
}

// NewNullOutput constructor for NullOutput
func NewNullOutput() (o *NullOutput) {
	return new(NullOutput)
}

func (o *NullOutput) Write(data []byte) (int, error) {
	return len(data), nil
}

func (o *NullOutput) String() string {
	return "Null Output"
}
