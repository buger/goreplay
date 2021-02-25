package csv

import (
	"encoding/csv"
	"os"
	"strings"
)

// Writer check encoding/csv.Writer
type Writer struct {
	*csv.Writer
	f *os.File
}

// NewWriter returns an csv writer with file opened
// error returned will be associated with opening the file
func NewWriter(path string) (*Writer, error) {
	var wr Writer
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	wr.f = f
	wr.Writer = csv.NewWriter(f)
	return &wr, nil
}

// WriteHeader will write infos about this docs in form of comments
// it uses "#" to indicate that this should be ignored by the reader
func (wr *Writer) WriteHeader(hdr string) error {
	_, err := wr.f.WriteString(formatHeader(hdr))
	return err
}

// Close the underlying file
func (wr *Writer) Close() error {
	return wr.f.Close()
}

func formatHeader(hdr string) string {
	if !strings.HasSuffix(hdr, "\n") {
		hdr += "\n"
	}
	// add comments before every line
	hdr = strings.ReplaceAll("#"+hdr, "\n", "\n#")
	return hdr[:len(hdr)-1]
}
