package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

const (
	VERSION = "0.8"
	DEFAULT_EMITTER_BUFFER = 32
)

type AppSettings struct {
	verbose bool

	splitOutput bool

	inputDummy  MultiOption
	outputDummy MultiOption

	inputTCP  MultiOption
	outputTCP MultiOption

	inputFile  MultiOption
	outputFile MultiOption

	inputRAW MultiOption

	outputHTTP              MultiOption
	outputHTTPHeaders       HTTPHeaders
	outputHTTPMethods       HTTPMethods
	outputHTTPElasticSearch string

	bufferSize	int
}

var Settings AppSettings = AppSettings{}

func usage() {
	fmt.Printf("Gor is a simple http traffic replication tool written in Go. Its main goal is to replay traffic from production servers to staging and dev environments.\nProject page: https://github.com/buger/gor\nAuthor: <Leonid Bugaev> leonsbox@gmail.com\nCurrent Version: %s\n\n", VERSION)
	flag.PrintDefaults()
	os.Exit(2)
}

func init() {
	flag.Usage = usage

	flag.BoolVar(&Settings.verbose, "verbose", false, "Turn on verbose/debug output")

	flag.BoolVar(&Settings.splitOutput, "split-output", false, "By default each output gets same traffic. If set to `true` it splits traffic equally among all outputs.")

	flag.Var(&Settings.inputDummy, "input-dummy", "Used for testing outputs. Emits 'Get /' request every 1s")
	flag.Var(&Settings.outputDummy, "output-dummy", "Used for testing inputs. Just prints data coming from inputs.")

	flag.Var(&Settings.inputTCP, "input-tcp", "Used for internal communication between Gor instances. Example: \n\t# Receive requests from other Gor instances on 28020 port, and redirect output to staging\n\tgor --input-tcp :28020 --output-http staging.com")
	flag.Var(&Settings.outputTCP, "output-tcp", "Used for internal communication between Gor instances. Example: \n\t# Listen for requests on 80 port and forward them to other Gor instance on 28020 port\n\tgor --input-raw :80 --output-tcp replay.local:28020")

	flag.Var(&Settings.inputFile, "input-file", "Read requests from file: \n\tgor --input-file ./requests.gor --output-http staging.com")
	flag.Var(&Settings.outputFile, "output-file", "Write incoming requests to file: \n\tgor --input-raw :80 --output-file ./requests.gor")

	flag.Var(&Settings.inputRAW, "input-raw", "Capture traffic from given port (use RAW sockets and require *sudo* access):\n\t# Capture traffic from 8080 port\n\tgor --input-raw :8080 --output-http staging.com")

	flag.Var(&Settings.outputHTTP, "output-http", "Forwards incoming requests to given http address.\n\t# Redirect all incoming requests to staging.com address \n\tgor --input-raw :80 --output-http http://staging.com")
	flag.Var(&Settings.outputHTTPHeaders, "output-http-header", "Inject additional headers to http reqest:\n\tgor --input-raw :8080 --output-http staging.com --output-http-header 'User-Agent: Gor'")
	flag.Var(&Settings.outputHTTPMethods, "output-http-method", "Whitelist of HTTP methods to replay. Anything else will be dropped:\n\tgor --input-raw :8080 --output-http staging.com --output-http-method GET --output-http-method OPTIONS")
	flag.StringVar(&Settings.outputHTTPElasticSearch, "output-http-elasticsearch", "", "Send request and response stats to ElasticSearch:\n\tgor --input-raw :8080 --output-http staging.com --output-http-elasticsearch 'es_host:api_port/index_name'")
	flag.IntVar(&Settings.bufferSize, "emitter-buffer-size", DEFAULT_EMITTER_BUFFER, string("Maximum request size in KB that gor can process. Default and minimum: ", DEFAULT_EMITTER_BUFFER))
	if &Settings.bufferSize < DEFAULT_EMITTER_BUFFER {
		&Settings.bufferSize = DEFAULT_EMITTER_BUFFER
	}
}

func Debug(args ...interface{}) {
	if Settings.verbose {
		log.Println(args...)
	}
}
