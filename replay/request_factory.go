package replay

import (
	"net/http"
	"net/url"
)

type HttpResponse struct {
	host *ForwardHost
	req  *http.Request
	resp *http.Response
	err  error
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
	responses chan *HttpResponse
	requests  chan *http.Request
}

// RequestFactory contstuctor
// One created, it starts listening for incoming requests: requests channel
func NewRequestFactory() (factory *RequestFactory) {
	factory = &RequestFactory{}
	factory.responses = make(chan *HttpResponse)
	factory.requests = make(chan *http.Request)

	go factory.handleRequests()

	return
}

// Forward http request to given host
func (f *RequestFactory) sendRequest(host *ForwardHost, request *http.Request) {
	client := &http.Client{}

	URL := host.Url + request.URL.String()

	request.RequestURI = ""
	request.URL, _ = url.ParseRequestURI(URL)

	if Settings.verbose {
		Debug("Sending request:", request)
	}

	resp, err := client.Do(request)

	if err == nil {
		defer resp.Body.Close()
	} else {
		Debug("Request error:", err)
	}

	f.responses <- &HttpResponse{host, request, resp, err}
}

// Handle incoming requests, and they responses
func (f *RequestFactory) handleRequests() {
	hosts := Settings.ForwardedHosts()

	for {
		select {
		case req := <-f.requests:
			for _, host := range hosts {
				// Ensure that we have actual stats for given timestamp
				host.Stat.Touch()

				if host.Limit == 0 || host.Stat.Count < host.Limit {
					// Increment Stat.Count
					host.Stat.IncReq()

					go f.sendRequest(host, req)
				}
			}
		case resp := <-f.responses:
			// Increment returned http code stats, and elapsed time
			resp.host.Stat.IncResp(resp)
		}
	}
}

// Add request to channel for further processing
func (f *RequestFactory) Add(request *http.Request) {
	f.requests <- request
}
