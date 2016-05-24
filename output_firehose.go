package main

import (
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/firehose"
)

// FirehoseOutput plugin
type FirehoseOutput struct {
	fh         *firehose.Firehose
	streamName string
	buffer     []*firehose.Record
}

// number of records batched before being sent through firehose
var batchSize = 50

// NewFirehoseOutput constructor for FirehoseOutput, accepts firehose stream name
func NewFirehoseOutput(streamName string) io.Writer {
	fatalUnlessCredentials()
	f := new(FirehoseOutput)
	f.fh = firehose.New(session.New(), aws.NewConfig().WithRegion(os.Getenv("AWS_REGION")))
	f.streamName = streamName
	return f
}

func (f *FirehoseOutput) Write(data []byte) (n int, err error) {
	if !isOriginPayload(data) {
		return len(data), nil
	}

	data = append(data, []byte(payloadSeparator)...)
	f.buffer = append(f.buffer, &firehose.Record{Data: data})

	if len(f.buffer) == batchSize {
		_, err := f.fh.PutRecordBatch(
			&firehose.PutRecordBatchInput{
				DeliveryStreamName: &f.streamName,
				Records:            f.buffer,
			},
		)
		if err != nil {
			KV.Error("gor-firehose-put-failed")
		}
		f.buffer = []*firehose.Record{}
	}
	return len(data), nil
}

// fatalUnlessCredentials logs and exits unless required env vars are present
// There are other ways of providing AWS authentication but we don't support them at present
func fatalUnlessCredentials() {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" && os.Getenv("AWS_ACCESS_KEY") == "" {
		log.Fatal("Required env var: AWS_ACCESS_KEY or AWS_ACCESS_KEY_ID not found")
	}
	if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" && os.Getenv("AWS_SECRET_KEY") == "" {
		log.Fatal("Required env var: AWS_SECRET_KEY or AWS_SECRET_ACCESS_KEY not found")
	}
	if os.Getenv("AWS_REGION") == "" {
		log.Fatal("Required env var: AWS_REGION")
	}
}
