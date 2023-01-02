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

func BenchmarkStringtoSlice(b *testing.B) {
	var s string
	var buf [1 << 20]byte
	for i := 0; i < b.N; i++ {
		s = SliceToString(buf[:])
	}
	_ = s // avoid gc to optimize away the loop body
}
