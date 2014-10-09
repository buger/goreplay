package main

import (
	"bufio"
	"bytes"	
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
	reader := bufio.NewReader(buf)

	request, err = http.ReadRequest(reader)

	return
}

type HTTPOutput struct {
	address string
	limit   int
	buf     chan []byte
	deathRecord chan int
	needWorker chan int
	activeWorkers int

	urlRegexp             HTTPUrlRegexp
	headerFilters         HTTPHeaderFilters
	headerHashFilters     HTTPHeaderHashFilters
    outputHTTPUrlRewrite  UrlRewriteMap

	headers HTTPHeaders
	methods HTTPMethods

	elasticSearch *ESPlugin

	bufStats *GorStat
}

func NewHTTPOutput(options string, headers HTTPHeaders, methods HTTPMethods, urlRegexp HTTPUrlRegexp, headerFilters HTTPHeaderFilters, headerHashFilters HTTPHeaderHashFilters, elasticSearchAddr string, outputHTTPUrlRewrite UrlRewriteMap) io.Writer {

	o := new(HTTPOutput)

	optionsArr := strings.Split(options, "|")
	address := optionsArr[0]

	if !strings.HasPrefix(address, "http") {
		address = "http://" + address
	}

	o.address = address
	o.headers = headers
	o.methods = methods

	o.urlRegexp = urlRegexp
	o.headerFilters = headerFilters
	o.headerHashFilters = headerHashFilters
	o.outputHTTPUrlRewrite = outputHTTPUrlRewrite

	o.buf = make(chan []byte, 100)
	o.activeWorkers = 0
	o.deathRecord = make(chan int, 20480)
	if Settings.outputHTTPStats {
		o.bufStats = NewGorStat("output_http")
	}
	if Settings.outputHTTPWorkers == -1 {
		o.needWorker = make(chan int)
	}

	if elasticSearchAddr != "" {
		o.elasticSearch = new(ESPlugin)
		o.elasticSearch.Init(elasticSearchAddr)
	}

	if len(optionsArr) > 1 {
		o.limit, _ = strconv.Atoi(optionsArr[1])
	}

	go o.WorkerMaster(Settings.outputHTTPWorkers)

	if o.limit > 0 {
		return NewLimiter(o, o.limit)
	} else {
		return o
	}
}

func (o *HTTPOutput) WorkerMaster(n int) {
	for i := 0; i < n; i++ {
		go o.Worker()
		o.activeWorkers += 1
	}

	if Settings.outputHTTPWorkers == -1 {
		for {
				new_workers := <-o.needWorker
				for i := 0; i < new_workers; i++ {
					go o.Worker()
					o.deathRecord <- 1
				}
		}
	}
}

func (o *HTTPOutput) Worker() {
	client := &http.Client{
		CheckRedirect: customCheckRedirect,
	}
	death_count := 0
	Loop:
		for {
			select {
				case data := <-o.buf:
				o.sendRequest(client, data)
				death_count = 0
			default:
				if Settings.outputHTTPWorkers == -1 {
					death_count += 1
				}
				if death_count > 20 {
					break Loop
				} else {
					time.Sleep(time.Millisecond * 100)
				}

			}
		}
	o.deathRecord <- -1
}

func (o *HTTPOutput) Write(data []byte) (n int, err error) {
	buf := make([]byte, len(data))
	copy(buf, data)

	o.buf <- buf
	buf_len := len(o.buf)
	if Settings.outputHTTPStats {
		o.bufStats.Write(len(o.buf))
	}

	if Settings.outputHTTPWorkers == -1 {
		select {
		case worker_died := <-o.deathRecord:
			o.activeWorkers += worker_died
		default:
		}
		if buf_len > 10 || (o.activeWorkers == 0 && buf_len > 0)    {
			if len(o.needWorker) == 0 {
				o.needWorker <- buf_len
			}
		}
	}
	return len(data), nil
}

func (o *HTTPOutput) sendRequest(client *http.Client, data []byte) {
	request, err := ParseRequest(data)

	if err != nil {
		log.Println("Cannot parse request", string(data), err)
		return
	}

	if len(o.methods) > 0 && !o.methods.Contains(request.Method) {
		return
	}

	if !(o.urlRegexp.Good(request) && o.headerFilters.Good(request) && o.headerHashFilters.Good(request)) {
		return
	}

    // Rewrite the path as necessary
    request.URL.Path = o.outputHTTPUrlRewrite.Rewrite(request.URL.Path)

	// Change HOST of original request
	URL := o.address + request.URL.Path + "?" + request.URL.RawQuery

	request.RequestURI = ""
	request.URL, _ = url.ParseRequestURI(URL)

	for _, header := range o.headers {
		SetHeader(request, header.Name, header.Value)
	}

	start := time.Now()
	resp, err := client.Do(request)
	stop := time.Now()

	// We should not count Redirect as errors
	if urlErr, ok := err.(*url.Error); ok {
		if _, ok := urlErr.Err.(*RedirectNotAllowed); ok {
			err = nil
		}
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

func SetHeader(request *http.Request, name string, value string) {

	// Need to check here for the Host header as it needs to be set on the request and not as a separate header
	// http.ReadRequest sets it by default to the URL Host of the request being read
	if name == "Host" {
		request.Host = value
	} else {
		request.Header.Set(name, value)
	}

	return

}

func (o *HTTPOutput) String() string {
	return "HTTP output: " + o.address
}
