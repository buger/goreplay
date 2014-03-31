package main

import (
	"bufio"
	"bytes"
	es "github.com/buger/gor/elasticsearch"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type RedirectNotAllowed struct{}

func (e *RedirectNotAllowed) Error() string {
	return "Redirects not allowed"
}

// customCheckRedirect disables redirects https://github.com/buger/gor/pull/15
func customCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 0 {
		return new(RedirectNotAllowed)
	}
	return nil
}

// ParseRequest in []byte returns a http request or an error
func ParseRequest(data []byte) (request *http.Request, err error) {
	buf := bytes.NewBuffer(data)
	reader := bufio.NewReaderSize(buf, len(data))

	request, err = http.ReadRequest(reader)

	return
}

type HTTPOutput struct {
	address string
	limit   int
	buf     chan []byte

	headers HTTPHeaders
	methods HTTPMethods

	elasticSearch *es.ESPlugin
	bufferSize int
}

func NewHTTPOutput(options string, headers HTTPHeaders, methods HTTPMethods, elasticSearchAddr string) io.Writer {
	o := new(HTTPOutput)

	optionsArr := strings.Split(options, "|")
	address := optionsArr[0]

	if !strings.HasPrefix(address, "http") {
		address = "http://" + address
	}

	o.address = address
	o.headers = headers
	o.methods = methods

	o.buf = make(chan []byte, 100)

	if elasticSearchAddr != "" {
		o.elasticSearch = new(es.ESPlugin)
		o.elasticSearch.Init(elasticSearchAddr)
	}

	if len(optionsArr) > 1 {
		o.limit, _ = strconv.Atoi(optionsArr[1])
	}

	for i := 0; i < 10; i++ {
		go o.worker(i)
	}

	if o.limit > 0 {
		return NewLimiter(o, o.limit)
	} else {
		return o
	}
}

func (o *HTTPOutput) worker(n int) {
	client := &http.Client{
		CheckRedirect: customCheckRedirect,
	}

	for {
		data := <-o.buf
		o.sendRequest(client, data)
	}
}

func (o *HTTPOutput) Write(data []byte) (n int, err error) {
	buf := make([]byte, len(data))
	copy(buf, data)

	o.buf <- buf

	return len(data), nil
}

func (o *HTTPOutput) sendRequest(client *http.Client, data []byte) {
	request, err := ParseRequest(data)

	if err != nil {
		log.Println("Can not parse request", string(data), err)
		return
	}

	if len(o.methods) > 0 && !o.methods.Contains(request.Method) {
		return
	}

	// Change HOST of original request
	URL := o.address + request.URL.Path + "?" + request.URL.RawQuery

	request.RequestURI = ""
	request.URL, _ = url.ParseRequestURI(URL)

	for _, header := range o.headers {
		request.Header.Set(header.Name, header.Value)
	}

	start := time.Now()
	resp, err := client.Do(request)
	stop := time.Now()

	// We should not count Redirect as errors
	if _, ok := err.(*RedirectNotAllowed); ok {
		err = nil
	}

	if err == nil {
		defer resp.Body.Close()
	} else {
		log.Println("Request error:", err)
	}

	if o.elasticSearch != nil {
		o.elasticSearch.ResponseAnalyze(request, resp, start, stop)
	}
}

func (o *HTTPOutput) String() string {
	return "HTTP output: " + o.address
}
