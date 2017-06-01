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
	return &FirehoseOutput{
		fh:         firehose.New(session.New(), aws.NewConfig().WithRegion(os.Getenv("AWS_REGION"))),
		streamName: streamName,
		buffer:     []*firehose.Record{},
	}
}

func (f *FirehoseOutput) Write(data []byte) (n int, err error) {
	if !isOriginPayload(data) {
		return len(data), nil
	}

	dataCopy := make([]byte, len(data)+len(payloadSeparator))

	copy(dataCopy, data)
	dataCopy = append(dataCopy, []byte(payloadSeparator)...)
	f.buffer = append(f.buffer, &firehose.Record{Data: dataCopy})

	if len(f.buffer) >= batchSize {
		_, err := f.fh.PutRecordBatch(
			&firehose.PutRecordBatchInput{
				DeliveryStreamName: &f.streamName,
				Records:            f.buffer,
			},
		)
		if err != nil {
			KV.ErrorD("gor-firehose-put-failed", map[string]interface{}{
				"error": err.Error(),
			})
		}
		f.buffer = []*firehose.Record{}
	}
	return len(dataCopy), nil
}

// fatalUnlessCredentials logs and exits unless required env vars are present
// There are other ways of providing AWS authentication but we don't support them at present
func fatalUnlessCredentials() {
	if os.Getenv("AWS_REGION") == "" {
		log.Fatal("Required env var: AWS_REGION")
	}
}
