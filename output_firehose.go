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
}

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

	// Gor will ignore errors here
	f.fh.PutRecord(
		&firehose.PutRecordInput{
			DeliveryStreamName: aws.String(f.streamName),
			Record: &firehose.Record{
				Data: data,
			},
		},
	)
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
