package stat

import "testing"

type testWriter struct {
	f  func([]string)
	fl func()
}

func (t testWriter) WriteHeader(_ string) error { return nil }

func (t testWriter) Flush() { t.fl() }

func (t testWriter) Error() error { return nil }

func (t testWriter) Write(r []string) error {
	t.f(r)
	return nil
}

func TestTempSize(t *testing.T) {
	count := 0
	tW := testWriter{f: func(_ []string) { count++ }, fl: func() {
		if count != flushSpan {
			t.Errorf("should flush after %d records", flushSpan)
		}
	}}
	s := NewWriter(tW, "")
	defer s.Close()
	for i := 0; i < flushSpan; i++ {
		s.Write([]string{"1"})
	}
}
