package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

var dateFileNameFuncs = map[string]func() string{
	"%Y":  func() string { return time.Now().Format("2006") },
	"%m":  func() string { return time.Now().Format("01") },
	"%d":  func() string { return time.Now().Format("02") },
	"%H":  func() string { return time.Now().Format("15") },
	"%M":  func() string { return time.Now().Format("04") },
	"%S":  func() string { return time.Now().Format("05") },
	"%NS": func() string { return fmt.Sprint(time.Now().Nanosecond()) },
}

// FileOutput output plugin
type FileOutput struct {
	pathTemplate string
	currentName  string
	file         *os.File
	writer       io.Writer
}

// NewFileOutput constructor for FileOutput, accepts path
func NewFileOutput(pathTemplate string, flushInterval time.Duration) *FileOutput {
	o := new(FileOutput)
	o.pathTemplate = pathTemplate
	o.updateName()

	// Force flushing every minute
	go func() {
		for {
			time.Sleep(flushInterval)
			o.flush()
		}
	}()

	go func() {
		for {
			time.Sleep(time.Second)
			o.updateName()
		}
	}()

	return o
}

func (o *FileOutput) filename() string {
	path := o.pathTemplate

	for name, fn := range dateFileNameFuncs {
		path = strings.Replace(path, name, fn(), -1)
	}

	return path
}

func (o *FileOutput) updateName() {
	o.currentName = o.filename()
}

func (o *FileOutput) Write(data []byte) (n int, err error) {
	if !isOriginPayload(data) {
		return len(data), nil
	}

	if o.file == nil || o.currentName != o.file.Name() {
		o.Close()

		o.file, err = os.OpenFile(o.currentName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)

		if strings.HasSuffix(o.currentName, ".gz") {
			o.writer = gzip.NewWriter(o.file)
		} else {
			o.writer = bufio.NewWriter(o.file)
		}

		if err != nil {
			log.Fatal(o, "Cannot open file %q. Error: %s", o.currentName, err)
		}
	}

	o.writer.Write(data)
	o.writer.Write([]byte(payloadSeparator))

	return len(data), nil
}

func (o *FileOutput) flush() {
	if o.file != nil {
		if strings.HasSuffix(o.currentName, ".gz") {
			o.writer.(*gzip.Writer).Flush()
		} else {
			o.writer.(*bufio.Writer).Flush()
		}
	}
}

func (o *FileOutput) String() string {
	return "File output: " + o.file.Name()
}

func (o *FileOutput) Close() {
	if o.file != nil {
		if strings.HasSuffix(o.currentName, ".gz") {
			o.writer.(*gzip.Writer).Close()
		} else {
			o.writer.(*bufio.Writer).Flush()
		}
		o.file.Close()
	}
}
