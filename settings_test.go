package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestAppSettings(t *testing.T) {
	a := AppSettings{}
	data, err := json.Marshal(&a)
	if err != nil {
		panic(err)
	}
	fmt.Printf(string(data))
}

func TestUpdateConfig(t *testing.T) {
	input := `{
			"verbose": true,
			"debug": false,
			"stats": false,
			"exit-after": 0,
			"split-output": false,
			"recognize-tcp-sessions": false,
			"http-pprof": "",
			"input-dummy": null,
			"OutputDummy": null,
			"output-stdout": false,
			"output-null": false,
			"input-tcp": null,
			"InputTCPConfig": {
				"input-tcp-secure": false,
				"input-tcp-certificate": "",
				"input-tcp-certificate-key": ""
			},
			"output-tcp": null,
			"OutputTCPConfig": {
				"output-tcp-secure": false,
				"output-tcp-sticky": false
			},
			"output-tcp-stats": false,
			"input-file": null,
			"input-file-loop": false,
			"output-file": null,
			"OutputFileConfig": {
				"output-file-flush-interval": 1000000000,
				"output-file-queue-limit": 256,
				"output-file-append": false,
				"output-file-buffer": "/tmp"
			},
			"input_raw": null,
			"InputRAWConfig": {
				"input-raw-engine": "libpcap",
				"input-raw-track-response": false,
				"input-raw-realip-header": "",
				"input-raw-expire": 2000000000,
				"input-raw-protocol": "http",
				"input-raw-bpf-filter": "",
				"input-raw-timestamp-type": "",
				"input-raw-immediate-mode": false,
				"BufferSize": 0,
				"input-raw-override-snaplen": false,
				"input-raw-buffer-size": "0"
			},
			"output-file-size-limit": "32mb",
			"output-file-max-size-limit": "1TB",
			"copy-buffer-size": "5mb",
			"middleware": "",
			"InputHTTP": null,
			"output-http": null,
			"prettify-http": false,
			"OutputHTTPConfig": {
				"output-http-redirect-limit": 0,
				"output-http-stats": false,
				"output-http-workers-min": 0,
				"output-http-workers": 0,
				"output-http-stats-ms": 5000,
				"Workers": 0,
				"output-http-queue-len": 1000,
				"output-http-elasticsearch": "",
				"output-http-timeout": 5000000000,
				"output-http-original-host": false,
				"output-http-response-buffer": 0,
				"output-http-compatibility-mode": false,
				"RequestGroup": "",
				"output-http-debug": false,
				"output-http-track-response": false
			},
			"output-binary": null,
			"OutputBinaryConfig": {
				"output-binary-workers": 0,
				"output-binary-timeout": 0,
				"output-tcp-response-buffer": 0,
				"output-binary-debug": false,
				"output-binary-track-response": false
			},
			"ModifierConfig": {
				"http-disallow-url": null,
				"http-allow-url": null,
				"http-rewrite-url": null,
				"http-rewrite-header": null,
				"http-allow-header": null,
				"http-disallow-header": null,
				"http-basic-auth-filter": null,
				"http-header-limiter": null,
				"http-param-limiter": null,
				"http-set-param": null,
				"http-set-header": null,
				"http-allow-method": null
			},
			"InputKafkaConfig": {
				"input-kafka-host": "",
				"input-kafka-topic": "",
				"input-kafka-json-format": false
			},
			"OutputKafkaConfig": {
				"output-kafka-host": "",
				"output-kafka-topic": "",
				"output-kafka-json-format": false
			},
			"config-file": "config.json",
			"config-server-address": ":9999",
			"config-remote-host": ":8000"
		}`
	updateConfig([]byte(input))
}
