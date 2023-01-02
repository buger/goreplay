package goreplay

import (
	"encoding/json"
	"testing"
)

func TestAppSettings(t *testing.T) {
	a := AppSettings{}
	_, err := json.Marshal(&a)
	if err != nil {
		t.Error(err)
	}
}
