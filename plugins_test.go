package main

import (
	"github.com/buger/gor/requestfiles"
	"io"
	"testing"
)

func TestPluginsRegistration(t *testing.T) {
	Plugins.Inputs = []io.Reader{}
	Plugins.Outputs = []io.Writer{}

	Settings.inputDummy = MultiOption{"[]"}
	Settings.outputDummy = MultiOption{"[]"}
	Settings.outputHTTP = MultiOption{"www.example.com|10"}
	Settings.inputFile = MultiOption{"/dev/null"}

	InitPlugins()

	if len(Plugins.Inputs) != 2 {
		t.Errorf("Should be 2 inputs %d", len(Plugins.Inputs))
	}

	if _, ok := Plugins.Inputs[0].(*DummyInput); !ok {
		t.Errorf("First input should be DummyInput")
	}

	if _, ok := Plugins.Inputs[1].(*requestfiles.FileInput); !ok {
		t.Errorf("Second input should be FileInput")
	}

	if len(Plugins.Outputs) != 2 {
		t.Errorf("Should be 2 output %d", len(Plugins.Outputs))
	}

	if _, ok := Plugins.Outputs[0].(*DummyOutput); !ok {
		t.Errorf("First output should be DummyOutput")
	}

	if l, ok := Plugins.Outputs[1].(*Limiter); ok {
		if _, ok := l.plugin.(*HTTPOutput); !ok {
			t.Errorf("HTTPOutput should be wrapped in limiter")
		}
	} else {
		t.Errorf("Second output should be Limiter")
	}

}
