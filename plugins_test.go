package main

import (
	"testing"
)

func TestPluginsRegistration(t *testing.T) {
	Settings.inputDummy = MultiOption{"[]"}
	Settings.outputDummy = MultiOption{"[]"}
	Settings.outputHTTP = MultiOption{"www.example.com|10"}
	Settings.inputFile = MultiOption{"/dev/null"}

	plugins := InitPlugins()

	if len(plugins.Inputs) != 2 {
		t.Errorf("Should be 2 inputs %d", len(plugins.Inputs))
	}

	if _, ok := plugins.Inputs[0].(*DummyInput); !ok {
		t.Errorf("First input should be DummyInput")
	}

	if _, ok := plugins.Inputs[1].(*FileInput); !ok {
		t.Errorf("Second input should be FileInput")
	}

	if len(plugins.Outputs) != 2 {
		t.Errorf("Should be 2 output %d", len(plugins.Outputs))
	}

	if _, ok := plugins.Outputs[0].(*DummyOutput); !ok {
		t.Errorf("First output should be DummyOutput")
	}

	if l, ok := plugins.Outputs[1].(*Limiter); ok {
		if _, ok := l.plugin.(*HTTPOutput); !ok {
			t.Errorf("HTTPOutput should be wrapped in limiter")
		}
	} else {
		t.Errorf("Second output should be Limiter")
	}

}
