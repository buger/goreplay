package main

type writeCallback func(*Message)

// TestOutput used in testing to intercept any output into callback
type TestOutput struct {
	cb writeCallback
}

// NewTestOutput constructor for TestOutput, accepts callback which get called on each incoming Write
func NewTestOutput(cb writeCallback) PluginWriter {
	i := new(TestOutput)
	i.cb = cb

	return i
}

// PluginWrite write message to this plugin
func (i *TestOutput) PluginWrite(msg *Message) (int, error) {
	i.cb(msg)

	return len(msg.Data) + len(msg.Meta), nil
}

func (i *TestOutput) String() string {
	return "Test Output"
}
