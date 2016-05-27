package byteutils

import (
	"bytes"
	"testing"
)

func TestCut(t *testing.T) {
	if !bytes.Equal(Cut([]byte("123456"), 2, 4), []byte("1256")) {
		t.Error("Should properly cut")
	}
}

func TestInsert(t *testing.T) {
	if !bytes.Equal(Insert([]byte("123456"), 2, []byte("abcd")), []byte("12abcd3456")) {
		t.Error("Should insert into middle of slice")
	}
}

func TestReplace(t *testing.T) {
	if !bytes.Equal(Replace([]byte("123456"), 2, 4, []byte("ab")), []byte("12ab56")) {
		t.Error("Should replace when same length")
	}

	if !bytes.Equal(Replace([]byte("123456"), 2, 4, []byte("abcd")), []byte("12abcd56")) {
		t.Error("Should replace when replacement length bigger")
	}

	if !bytes.Equal(Replace([]byte("123456"), 2, 5, []byte("ab")), []byte("12ab6")) {
		t.Error("Should replace when replacement length bigger")
	}
}

func TestSwitchFirstCharCase(t *testing.T) {
	if !bytes.Equal([]byte("abc"), SwitchFirstCharCase([]byte("Abc"))) {
		t.Error("Should uppercase first character")
	}
	if !bytes.Equal([]byte("Abc"), SwitchFirstCharCase([]byte("abc"))) {
		t.Error("Should lowercase first character")
	}
	if !bytes.Equal([]byte("@bc"), SwitchFirstCharCase([]byte("@bc"))) {
		t.Error("Should ignore first character if not in a-zA-Z")
	}
	if !bytes.Equal([]byte(""), SwitchFirstCharCase([]byte(""))) {
		t.Error("Should do nothing if byte slice is empty")
	}
}
