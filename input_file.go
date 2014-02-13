package gor

import (
	"encoding/gob"
	"log"
	"os"
	"time"
)

type FileInput struct {
	data    chan []byte
	path    string
	decoder *gob.Decoder
}

func NewFileInput(path string) (i *FileInput) {
	i = new(FileInput)
	i.data = make(chan []byte)
	i.path = path
	i.Init(path)

	go i.emit()

	return
}

func (i *FileInput) Init(path string) {
	file, err := os.Open(path)

	if err != nil {
		log.Fatal(i, "Cannot open file %q. Error: %s", path, err)
	}

	i.decoder = gob.NewDecoder(file)
}

func (i *FileInput) Read() ([]byte, bool) {
	buf, ok := <-i.data

	return buf, ok
}

func (i *FileInput) String() string {
	return "File input: " + i.path
}

func (i *FileInput) emit() {
	var lastTime int64

	for {
		raw := new(RawRequest)
		err := i.decoder.Decode(raw)

		if err != nil {
			return
		}

		if lastTime != 0 {
			time.Sleep(time.Duration(raw.Timestamp - lastTime))
			lastTime = raw.Timestamp
		}

		i.data <- raw.Request
	}
}
