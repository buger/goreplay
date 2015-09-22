package main

import (
	"testing"
)

func TestPluginsRegistration(t *testing.T) {
	Plugins = make([]interface{}, 0)

	Settings.inputDummy = MultiOption{"[]"}
	Settings.outputDummy = MultiOption{"[]"}
	Settings.inputFile = MultiOption{"/dev/null"}
	Settings.outputHTTP = MultiOption{"www.example.com|10"}

	InitPlugins()

	if len(Plugins) != 4 {
		t.Errorf("Should be 2 inputs and 2 outputs %d", len(Plugins))
	}

	if _, ok := Plugins[0].(*DummyInput); !ok {
		t.Errorf("First input should be DummyInput")
	}

	if _, ok := Plugins[1].(*DummyOutput); !ok {
		t.Errorf("First output should be DummyOutput")
	}

	if _, ok := Plugins[2].(*FileInput); !ok {
		t.Errorf("Second input should be FileInput")
	}

	if l, ok := Plugins[3].(*Limiter); ok {
		if _, ok := l.plugin.(*HTTPOutput); !ok {
			t.Errorf("HTTPOutput should be wrapped in limiter")
		}
	} else {
		t.Errorf("Second output should be Limiter")
	}

}
