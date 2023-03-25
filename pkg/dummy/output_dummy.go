package dummy

import (
	"os"

	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/proto"
)

// DummyOutput used for debugging, prints all incoming requests
type DummyOutput struct {
}

// NewDummyOutput constructor for DummyOutput
func NewDummyOutput() (di *DummyOutput) {
	di = new(DummyOutput)

	return
}

// PluginWrite writes message to this plugin
func (i *DummyOutput) PluginWrite(msg *plugin.Message) (int, error) {
	var n, nn int
	n, _ = os.Stdout.Write(msg.Meta)
	nn, _ = os.Stdout.Write(msg.Data)
	n += nn
	nn, _ = os.Stdout.Write(proto.PayloadSeparatorAsBytes)
	n += nn

	return n, nil
}

func (i *DummyOutput) String() string {
	return "Dummy Output"
}
