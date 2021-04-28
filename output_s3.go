package main

import (
	_ "bufio"
	"fmt"
	_ "io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	_ "github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// S3Output output plugin
type S3Output struct {
	pathTemplate string

	buffer  *FileOutput
	session *session.Session
	config  *FileOutputConfig
	closeCh chan struct{}
}

// NewS3Output constructor for FileOutput, accepts path
func NewS3Output(pathTemplate string, config *FileOutputConfig) *S3Output {
	if !PRO {
		log.Fatal("Using S3 output and input requires PRO license")
		return nil
	}

	o := new(S3Output)
	o.pathTemplate = pathTemplate
	o.config = config
	o.config.onClose = o.onBufferUpdate

	if config.BufferPath == "" {
		config.BufferPath = "/tmp"
	}

	rnd := rand.Int63()
	bufferName := fmt.Sprintf("gor_output_s3_%d_buf_", rnd)

	pathParts := strings.Split(pathTemplate, "/")
	bufferName += pathParts[len(pathParts)-1]

	if strings.HasSuffix(o.pathTemplate, ".gz") {
		bufferName += ".gz"
	}

	bufferPath := filepath.Join(config.BufferPath, bufferName)

	o.buffer = NewFileOutput(bufferPath, config)
	o.connect()

	return o
}

func (o *S3Output) connect() {
	if o.session == nil {
		o.session = session.Must(session.NewSession(awsConfig()))
		log.Println("[S3 Output] S3 connection succesfully initialized")
	}
}

// PluginWrite writes message to this plugin
func (o *S3Output) PluginWrite(msg *Message) (n int, err error) {
	return o.buffer.PluginWrite(msg)
}

func (o *S3Output) String() string {
	return "S3 output: " + o.pathTemplate
}

// Close close the buffer of the S3 connection
func (o *S3Output) Close() error {
	return o.buffer.Close()
}

func parseS3Url(path string) (bucket, key string) {
	path = path[5:] // stripping `s3://`
	sep := strings.IndexByte(path, '/')

	bucket = path[:sep]
	key = path[sep+1:]

	return bucket, key
}

func (o *S3Output) keyPath(idx int) (bucket, key string) {
	bucket, key = parseS3Url(o.pathTemplate)

	for name, fn := range dateFileNameFuncs {
		key = strings.Replace(key, name, fn(o.buffer), -1)
	}

	key = setFileIndex(key, idx)

	return
}

func (o *S3Output) onBufferUpdate(path string) {
	svc := s3.New(o.session)
	idx := getFileIndex(path)
	bucket, key := o.keyPath(idx)

	file, err := os.Open(path)
	if err != nil {
		Debug(0, fmt.Sprintf("[S3 Output] Failed to open file %q. err: %q", path, err))
		return
	}
	defer os.Remove(path)

	_, err = svc.PutObject(&s3.PutObjectInput{
		Body:   file,
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		Debug(0, fmt.Sprintf("[S3 Output] Failed to upload data to %q/%q, %q", bucket, key, err))
		return
	}

	if o.closeCh != nil {
		o.closeCh <- struct{}{}
	}
}
