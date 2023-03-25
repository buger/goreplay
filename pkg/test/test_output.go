package test

import "github.com/buger/goreplay/pkg/plugin"

// WriteCallback callback for TestOutput
type WriteCallback func(*plugin.Message)

// TestOutput used in testing to intercept any output into callback
type TestOutput struct {
	cb WriteCallback
}

// NewTestOutput constructor for TestOutput, accepts callback which get called on each incoming Write
func NewTestOutput(cb WriteCallback) plugin.Writer {
	i := new(TestOutput)
	i.cb = cb

	return i
}

// PluginWrite write message to this plugin
func (i *TestOutput) PluginWrite(msg *plugin.Message) (int, error) {
	i.cb(msg)

	return len(msg.Data) + len(msg.Meta), nil
}

func (i *TestOutput) String() string {
	return "Test Output"
}
