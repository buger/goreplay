package requestfiles

import (
	"encoding/gob"
	"io"
	"log"
	"os"
	"time"
)

// RawRequest stores original start time and request payload
type RawRequest struct {
	Timestamp int64
	Request   []byte
}

// FileOutput output plugin
type FileOutput struct {
	path    string
	encoder *gob.Encoder
	file    *os.File
}

// NewFileOutput constructor for FileOutput, accepts path
func NewFileOutput(path string) io.Writer {
	o := new(FileOutput)
	o.path = path
	o.init(path)

	return o
}

func (o *FileOutput) init(path string) {
	var err error

	o.file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)

	if err != nil {
		log.Fatal(o, "Cannot open file %q. Error: %s", path, err)
	}

	o.encoder = gob.NewEncoder(o.file)
}

func (o *FileOutput) Write(data []byte) (n int, err error) {
	raw := RawRequest{time.Now().UnixNano(), data}

	o.encoder.Encode(raw)

	return len(data), nil
}

func (o *FileOutput) String() string {
	return "File output: " + o.path
}
