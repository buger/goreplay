package replay

import (
	"github.com/buger/go-httpclient"
	"net/http"
	"net/url"
	"time"
)

type HttpResponse struct {
	host *ForwardHost
	req  *HttpRequest
	resp *http.Response
	err  error

	created int64
}

type HttpRequest struct {
	req     *http.Request
	created int64
}

// Class for processing requests
//
// Basic workflow:
//
// 1. When request added via Add() it get pushed to `responses` chan
// 2. handleRequest() listen for `responses` chan and decide where request should be forwarded, and apply rate-limit if needed
// 3. sendRequest() forwards request and returns response info to `responses` chan
// 4. handleRequest() listen for `response` channel and updates stats
type RequestFactory struct {
	c_responses chan *HttpResponse
	c_requests  chan *http.Request
}

// RequestFactory contstuctor
// One created, it starts listening for incoming requests: requests channel
func NewRequestFactory() (factory *RequestFactory) {
	factory = &RequestFactory{}
	factory.c_responses = make(chan *HttpResponse)
	factory.c_requests = make(chan *http.Request)

	go factory.handleRequests()

	return
}

// Forward http request to given host
func (f *RequestFactory) sendRequest(host *ForwardHost, request *HttpRequest) {
	transport := &httpclient.Transport{
		ConnectTimeout: 60 * time.Second,
	}
	defer transport.Close()

	client := &http.Client{Transport: transport}

	// Change HOST of original request
	URL := host.Url + request.req.URL.Path + "?" + request.req.URL.RawQuery

	request.req.RequestURI = ""
	request.req.URL, _ = url.ParseRequestURI(URL)

	Debug("Sending request:", host.Url, request)

	resp, err := client.Do(request.req)

	if err == nil {
		defer resp.Body.Close()
	} else {
		Debug("Request error:", err)
	}

	f.c_responses <- &HttpResponse{host, request, resp, err, time.Now().UnixNano()}
}

// Handle incoming requests, and they responses
func (f *RequestFactory) handleRequests() {
	hosts := Settings.ForwardedHosts()

	for {
		select {
		case req := <-f.c_requests:
			for _, host := range hosts {
				if host.Limit == 0 || host.Stat.Count() < host.Limit {
					request := &HttpRequest{}
					request.req = req
					request.created = time.Now().UnixNano()

					go f.sendRequest(host, request)

					// Increment Stat.Count
					host.Stat.IncReq(request)
				}
			}
		case resp := <-f.c_responses:
			// Increment returned http code stats, and elapsed time
			resp.host.Stat.IncResp(resp)
		}
	}
}

// Add request to channel for further processing
func (f *RequestFactory) Add(request *http.Request) {
	f.c_requests <- request
}
