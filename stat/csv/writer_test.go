package csv

import (
	"testing"
)

func TestFormatHeader(t *testing.T) {
	m := map[string]string{
		"hello":                       "#hello\n",
		"":                            "#\n",
		"hello\n":                     "#hello\n",
		"hello\nhello\n":              "#hello\n#hello\n",
		"hello\nhello":                "#hello\n#hello\n",
		"hello\nhello\nhello":         "#hello\n#hello\n#hello\n",
		"hello\nhello\nhello\n":       "#hello\n#hello\n#hello\n",
		"hello\nhello\r\n\r\nhello\n": "#hello\n#hello\r\n#\r\n#hello\n",
	}
	for k, v := range m {
		if vl := formatHeader(k); vl != v {
			t.Errorf("expected %q to equal %q", vl, v)
		}
	}
}
