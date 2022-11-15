package main

import (
	"fmt"
	"os"

	"zip/infra/traffic_enrich/logs"

	"github.com/DataDog/datadog-go/v5/statsd"
)

var DDClient *statsd.Client

func newDataDog() *statsd.Client {
	host, found := os.LookupEnv("DD_AGENT_HOST")
	if !found {
		host = "127.0.0.1"
	}
	client, err := statsd.New(fmt.Sprintf("%s:%s", host, "8125"))
	if err != nil {
		logs.Fatal(err)
	}
	return client
}

func init() {
	DDClient = newDataDog()
}
