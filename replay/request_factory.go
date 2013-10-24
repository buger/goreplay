package replay

import (
	"bufio"
	"bytes"
	"container/ring"
	"errors"
	"fmt"
	"log"

	"net/http"
	"net/url"
	"time"
)

// HttpTiming contains timestamps for http requests and responses
type HttpTiming struct {
	reqStart time.Time
	respDone time.Time
}

// HttpResponse contains a host, a http request,
// a http response and an error
type HttpResponse struct {
	host   *ForwardHost
	req    *http.Request
	resp   *http.Response
	err    error
	timing *HttpTiming
}

// RequestFactory processes requests
//
// Basic workflow:
//
// 1. When request added via Add() it get pushed to `responses` chan
// 2. handleRequest() listen for `responses` chan and decide where request should be forwarded, and apply rate-limit if needed
// 3. sendRequest() forwards request and returns response info to `responses` chan
// 4. handleRequest() listen for `response` channel and updates stats
type RequestFactory struct {
	c_responses chan *HttpResponse
	c_requests  chan []byte

	reqBuf        *ring.Ring
	reqBufForSend *ring.Ring
}

// NewRequestFactory returns a RequestFactory pointer
// One created, it starts listening for incoming requests: requests channel
func NewRequestFactory() (factory *RequestFactory) {
	factory = &RequestFactory{}
	factory.c_responses = make(chan *HttpResponse)

	factory.c_requests = make(chan []byte)
	factory.reqBuf = ring.New(1000)
	factory.reqBufForSend = factory.reqBuf

	go factory.handleRequests()
	go factory.sendRequests()

	return
}

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

// sendRequest forwards http request to a given host
func (f *RequestFactory) sendRequest(host *ForwardHost, requestBytes []byte) {
	client := &http.Client{
		CheckRedirect: customCheckRedirect,
	}

	request, _ := ParseRequest(requestBytes)

	// Change HOST of original request
	URL := host.Url + request.URL.Path + "?" + request.URL.RawQuery

	request.RequestURI = ""
	request.URL, _ = url.ParseRequestURI(URL)

	for _, header := range Settings.AdditionalHeaders {
		request.Header.Set(header.Name, header.Value)
	}

	Debug("Sending request:", host.Url, request)

	tstart := time.Now()
	resp, err := client.Do(request)
	tstop := time.Now()

	// We should not count Redirect as errors
	if _, ok := err.(*RedirectNotAllowed); ok {
		err = nil
	}

	if err == nil {
		defer resp.Body.Close()
	} else {
		Debug("Request error:", err)
	}

	f.c_responses <- &HttpResponse{host, request, resp, err, &HttpTiming{tstart, tstop}}
}

// handleRequests and their responses
func (f *RequestFactory) handleRequests() {
	hosts := Settings.ForwardedHosts()

	for {
		select {
		case req := <-f.c_requests:
			f.reqBuf.Value = req
			f.reqBuf = f.reqBuf.Next()

			for _, host := range hosts {
				if host.Limit == 0 {
					go f.sendRequest(host, req)
				}
			}
		case resp := <-f.c_responses:
			// Increment returned http code stats, and elapsed time
			resp.host.Stat.IncResp(resp)

			// Send data to StatsD, ElasticSearch, etc...
			for _, rap := range Settings.ResponseAnalyzePlugins {
				go rap.ResponseAnalyze(resp)
			}
		}
	}

}

func (f *RequestFactory) sendRequests() {
	hosts := Settings.ForwardedHosts()

	for _, host := range hosts {
		// Ensure that we have actual stats for given timestamp
		if host.Limit != 0 {
			go func() {
				for {
					host.Stat.Touch()
					// Increment Stat.Count
					host.Stat.IncReq()

					req, err := f.getReqFromBuf(0)

					if err == nil {
						go f.sendRequest(host, req)
					} else {
						fmt.Println(err)
					}

					delay := float64(1) / float64(host.Limit) * float64(time.Second)

					fmt.Println(int(time.Duration(delay) / time.Millisecond))
					time.Sleep(time.Duration(delay))
				}
			}()
		}
	}
}

func (f *RequestFactory) getReqFromBuf(retries int) ([]byte, error) {
	f.reqBufForSend = f.reqBufForSend.Prev()

	if f.reqBufForSend.Value != nil {
		return f.reqBufForSend.Value.([]byte), nil
	} else {
		if retries <= f.reqBufForSend.Len() {
			return f.getReqFromBuf(retries + 1)
		} else {
			return nil, errors.New("Requests not found")
		}
	}
}

// Add request to channel for further processing
func (f *RequestFactory) Add(request []byte) {
	f.c_requests <- request
}

// ParseRequest in []byte returns a http request or an error
func ParseRequest(data []byte) (request *http.Request, err error) {
	buf := bytes.NewBuffer(data)
	reader := bufio.NewReader(buf)

	request, err = http.ReadRequest(reader)

	if err != nil {
		log.Println("Can not parse request", string(data), err)
	}

	return
}
