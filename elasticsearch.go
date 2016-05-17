package main

import (
	"encoding/json"
	"github.com/buger/elastigo/api"
	"github.com/buger/elastigo/core"
	"github.com/Clever/gor/proto"
	"log"
	"regexp"
	"time"
)

type ESUriErorr struct{}

func (e *ESUriErorr) Error() string {
	return "Wrong ElasticSearch URL format. Expected to be: host:port/index_name"
}

type ESPlugin struct {
	Active  bool
	ApiPort string
	Host    string
	Index   string
	indexor *core.BulkIndexer
	done    chan bool
}

type ESRequestResponse struct {
	ReqUrl               []byte `json:"Req_URL"`
	ReqMethod            []byte `json:"Req_Method"`
	ReqUserAgent         []byte `json:"Req_User-Agent"`
	ReqAcceptLanguage    []byte `json:"Req_Accept-Language,omitempty"`
	ReqAccept            []byte `json:"Req_Accept,omitempty"`
	ReqAcceptEncoding    []byte `json:"Req_Accept-Encoding,omitempty"`
	ReqIfModifiedSince   []byte `json:"Req_If-Modified-Since,omitempty"`
	ReqConnection        []byte `json:"Req_Connection,omitempty"`
	ReqCookies           []byte `json:"Req_Cookies,omitempty"`
	RespStatus           []byte `json:"Resp_Status"`
	RespStatusCode       []byte `json:"Resp_Status-Code"`
	RespProto            []byte `json:"Resp_Proto,omitempty"`
	RespContentLength    []byte `json:"Resp_Content-Length,omitempty"`
	RespContentType      []byte `json:"Resp_Content-Type,omitempty"`
	RespTransferEncoding []byte `json:"Resp_Transfer-Encoding,omitempty"`
	RespContentEncoding  []byte `json:"Resp_Content-Encoding,omitempty"`
	RespExpires          []byte `json:"Resp_Expires,omitempty"`
	RespCacheControl     []byte `json:"Resp_Cache-Control,omitempty"`
	RespVary             []byte `json:"Resp_Vary,omitempty"`
	RespSetCookie        []byte `json:"Resp_Set-Cookie,omitempty"`
	Rtt                  int64  `json:"RTT"`
	Timestamp            time.Time
}

// Parse ElasticSearch URI
//
// Proper format is: host:port/index_name
func parseURI(URI string) (err error, host string, port string, index string) {
	rURI := regexp.MustCompile("(.+):([0-9]+)/(.+)")
	match := rURI.FindAllStringSubmatch(URI, -1)

	if len(match) == 0 {
		err = new(ESUriErorr)
	} else {
		host = match[0][1]
		port = match[0][2]
		index = match[0][3]
	}

	return
}

func (p *ESPlugin) Init(URI string) {
	var err error

	err, p.Host, p.ApiPort, p.Index = parseURI(URI)

	if err != nil {
		log.Fatal("Can't initialize ElasticSearch plugin.", err)
	}

	api.Domain = p.Host
	api.Port = p.ApiPort

	p.indexor = core.NewBulkIndexerErrors(50, 60)
	p.done = make(chan bool)
	p.indexor.Run(p.done)

	// Only start the ErrorHandler goroutine when in verbose mode
	// no need to burn ressources otherwise
	// go p.ErrorHandler()

	log.Println("Initialized Elasticsearch Plugin")
	return
}

func (p *ESPlugin) IndexerShutdown() {
	p.done <- true
	return
}

func (p *ESPlugin) ErrorHandler() {
	for {
		errBuf := <-p.indexor.ErrorChannel
		log.Println(errBuf.Err)
	}
}

func (p *ESPlugin) RttDurationToMs(d time.Duration) int64 {
	sec := d / time.Second
	nsec := d % time.Second
	fl := float64(sec) + float64(nsec)*1e-6
	return int64(fl)
}

func (p *ESPlugin) ResponseAnalyze(req, resp []byte, start, stop time.Time) {
	if len(resp) == 0 {
		// nil http response - skipped elasticsearch export for this request
		return
	}
	t := time.Now()
	rtt := p.RttDurationToMs(stop.Sub(start))

	esResp := ESRequestResponse{
		ReqUrl:               proto.Path(req),
		ReqMethod:            proto.Method(req),
		ReqUserAgent:         proto.Header(req, []byte("User-Agent")),
		ReqAcceptLanguage:    proto.Header(req, []byte("Accept-Language")),
		ReqAccept:            proto.Header(req, []byte("Accept")),
		ReqAcceptEncoding:    proto.Header(req, []byte("Accept-Encoding")),
		ReqIfModifiedSince:   proto.Header(req, []byte("If-Modified-Since")),
		ReqConnection:        proto.Header(req, []byte("Connection")),
		ReqCookies:           proto.Header(req, []byte("Cookie")),
		RespStatus:           proto.Status(resp),
		RespStatusCode:       proto.Status(resp),
		RespProto:            proto.Method(resp),
		RespContentLength:    proto.Header(resp, []byte("Content-Length")),
		RespContentType:      proto.Header(resp, []byte("Content-Type")),
		RespTransferEncoding: proto.Header(resp, []byte("Transfer-Encoding")),
		RespContentEncoding:  proto.Header(resp, []byte("Content-Encoding")),
		RespExpires:          proto.Header(resp, []byte("Expires")),
		RespCacheControl:     proto.Header(resp, []byte("Cache-Control")),
		RespVary:             proto.Header(resp, []byte("Vary")),
		RespSetCookie:        proto.Header(resp, []byte("Set-Cookie")),
		Rtt:                  rtt,
		Timestamp:            t,
	}
	j, err := json.Marshal(&esResp)
	if err != nil {
		log.Println(err)
	} else {
		p.indexor.Index(p.Index, "RequestResponse", "", "", &t, j, true)
	}
	return
}
