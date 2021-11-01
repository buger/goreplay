// +build pro

package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func TestS3Output(t *testing.T) {
	bucket := aws.String("test-gor")
	rnd := rand.Int63()
	path := fmt.Sprintf("s3://test-gor/%d/requests.gz", rnd)

	output := NewS3Output(path, &FileOutputConfig{queueLimit: 2})

	svc := s3.New(output.session)

	output.Write([]byte("1 1 1\ntest"))
	output.Write([]byte("1 1 1\ntest"))
	output.buffer.updateName()
	output.Write([]byte("1 1 1\ntest"))
	output.Write([]byte("1 1 1\ntest"))
	output.buffer.updateName()
	output.Write([]byte("1 1 1\ntest"))

	time.Sleep(time.Second)

	params := &s3.ListObjectsInput{
		Bucket: bucket,
		Prefix: aws.String(fmt.Sprintf("%d", rnd)),
	}

	resp, _ := svc.ListObjects(params)
	if len(resp.Contents) != 2 {
		t.Error("Should create 2 objects", len(resp.Contents))
	} else {
		if *resp.Contents[0].Key != fmt.Sprintf("%d/requests_0.gz", rnd) ||
			*resp.Contents[1].Key != fmt.Sprintf("%d/requests_1.gz", rnd) {
			t.Error("Should assign proper names", resp.Contents)
		}
	}

	for _, c := range resp.Contents {
		svc.DeleteObject(&s3.DeleteObjectInput{Bucket: bucket, Key: c.Key})
	}

	matches, _ := filepath.Glob(fmt.Sprintf("/tmp/gor_output_s3_*"))
	for _, m := range matches {
		os.Remove(m)
	}
}

func TestS3OutputQueueLimit(t *testing.T) {
	bucket := aws.String("test-gor")
	rnd := rand.Int63()
	path := fmt.Sprintf("s3://test-gor/%d/requests.gz", rnd)

	output := NewS3Output(path, &FileOutputConfig{queueLimit: 100})
	output.closeCh = make(chan struct{}, 3)

	svc := s3.New(output.session)

	for i := 0; i < 3; i++ {
		for i := 0; i < 100; i++ {
			output.Write([]byte("1 1 1\ntest"))
		}
		output.buffer.updateName()
	}
	output.buffer.updateName()
	output.Write([]byte("1 1 1\ntest"))

	for i := 0; i < 3; i++ {
		<-output.closeCh
	}

	params := &s3.ListObjectsInput{
		Bucket: bucket,
		Prefix: aws.String(fmt.Sprintf("%d", rnd)),
	}

	resp, _ := svc.ListObjects(params)
	if len(resp.Contents) != 3 {
		t.Error("Should create 3 object", len(resp.Contents))
	} else {
		if *resp.Contents[0].Key != fmt.Sprintf("%d/requests_0.gz", rnd) ||
			*resp.Contents[1].Key != fmt.Sprintf("%d/requests_1.gz", rnd) {
			t.Error("Should assign proper names", resp.Contents)
		}
	}

	for _, c := range resp.Contents {
		svc.DeleteObject(&s3.DeleteObjectInput{Bucket: bucket, Key: c.Key})
	}

	matches, _ := filepath.Glob(fmt.Sprintf("/tmp/gor_output_s3_*"))
	for _, m := range matches {
		os.Remove(m)
	}
}

func TestInputFileFromS3(t *testing.T) {
	rnd := rand.Int63()
	path := fmt.Sprintf("s3://test-gor-eu/%d/requests.gz", rnd)

	output := NewS3Output(path, &FileOutputConfig{queueLimit: 5000})
	output.closeCh = make(chan struct{}, 10)

	for i := 0; i <= 20000; i++ {
		output.Write([]byte("1 1 1\ntest"))

		if i%5000 == 0 {
			output.buffer.updateName()
		}
	}

	output.Write([]byte("1 1 1\ntest"))

	for i := 0; i < 2; i++ {
		<-output.closeCh
	}

	input := NewFileInput(fmt.Sprintf("s3://test-gor-eu/%d", rnd), false, 100, 0, false)

	buf := make([]byte, 1000)
	for i := 0; i <= 19999; i++ {
		input.Read(buf)
	}

	// Cleanup artifacts
	bucket := aws.String("test-gor")
	svc := s3.New(output.session)
	params := &s3.ListObjectsInput{
		Bucket: bucket,
		Prefix: aws.String(fmt.Sprintf("%d", rnd)),
	}

	resp, _ := svc.ListObjects(params)

	for _, c := range resp.Contents {
		svc.DeleteObject(&s3.DeleteObjectInput{Bucket: bucket, Key: c.Key})
	}
}
