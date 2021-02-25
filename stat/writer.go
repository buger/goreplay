package stat

import (
	"errors"
	"sync"
	"time"
)

// ErrNotImplemented returned when impls requested are not implemnted
var ErrNotImplemented = errors.New("method not implemented")

// RecordWriter represents a stat writer it should have all this method.
// for rationale  behind this interface check encoding/csv or
type RecordWriter interface {
	Write(record []string) error
	Flush()
	Error() error
}

const flushSpan = 10

// Writer represents a single instance of goreplay stats writer
type Writer struct {
	name    string
	records [flushSpan][]string // a temporary stats holder
	off     int
	quit    chan struct{}
	mu      sync.Mutex
	w       RecordWriter
	err     error
}

// NewWriter returns a new instance of stats associated with the writer "wr"
// and "name" as the stats identity
func NewWriter(w RecordWriter, name string) (wr *Writer) {
	wr = new(Writer)
	wr.name = name
	wr.w = w
	wr.quit = make(chan struct{})
	return
}

// WriteHeader writes header of the documents to the underlying writer.
// Header can be any kind of data like json unmarshal or file info.
// underlying writer should sets a different way to represents the header.
func (wr *Writer) WriteHeader(header string) error {
	if wHdr, ok := wr.w.(interface{ WriteHeader(header string) error }); ok {
		return wHdr.WriteHeader(header)
	}
	return ErrNotImplemented
}

// SetReader sets another source of records reader to use along with (*Writer).Write([]string)
// rd function will be called every "rate" duration time
func (wr *Writer) SetReader(rd func() []string, rate time.Duration) {
	go func() {
		timer := time.NewTicker(rate)
		defer timer.Stop()
		for {
			select {
			case <-wr.quit:
				return
			case <-timer.C:
				err := wr.Write(rd())
				if err != nil {
					return
				}
			}
		}
	}()
}

func (wr *Writer) Write(record []string) error {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	if wr.err != nil {
		return wr.err
	}
	if len(record) > 0 {
		wr.records[wr.off] = record
		wr.off++
	}
	return wr.flush(wr.off >= flushSpan)
}

func (wr *Writer) String() string {
	return "stats: " + wr.name
}

// Error reports any error that has occurred during a previous Write or Flush.
func (wr *Writer) Error() error {
	wr.mu.Lock()
	wr.mu.Unlock()
	return wr.err
}

// Close closes and flush all data of the stat
func (wr *Writer) Close() error {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	close(wr.quit)
	wr.err = wr.flush(true)
	if cl, ok := wr.w.(interface{ Close() error }); ok {
		wr.err = cl.Close()
	}
	return wr.err
}

func (wr *Writer) flush(f bool) error {
	if !f {
		return nil
	}
	i := 0
	for i < wr.off {
		if wr.err = wr.w.Write(wr.records[i]); wr.err != nil {
			break
		}
		i++
	}
	wr.off -= i
	wr.w.Flush()
	wr.err = wr.w.Error()
	return wr.err
}
