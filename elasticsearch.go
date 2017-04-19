package main

import (
	"encoding/json"
	"github.com/buger/gor/proto"
	"github.com/mattbaird/elastigo/lib"
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
	eConn   *elastigo.Conn
	Host    string
	Index   string
	indexor *elastigo.BulkIndexer
	done    chan bool
}

type ESRequestResponse struct {
	ReqURL               string `json:"Req_URL"`
	ReqMethod            string `json:"Req_Method"`
	ReqUserAgent         string `json:"Req_User-Agent"`
	ReqAcceptLanguage    string `json:"Req_Accept-Language,omitempty"`
	ReqAccept            string `json:"Req_Accept,omitempty"`
	ReqAcceptEncoding    string `json:"Req_Accept-Encoding,omitempty"`
	ReqIfModifiedSince   string `json:"Req_If-Modified-Since,omitempty"`
	ReqConnection        string `json:"Req_Connection,omitempty"`
	ReqCookies           string `json:"Req_Cookies,omitempty"`
	RespStatus           string `json:"Resp_Status"`
	RespStatusCode       string `json:"Resp_Status-Code"`
	RespProto            string `json:"Resp_Proto,omitempty"`
	RespContentLength    string `json:"Resp_Content-Length,omitempty"`
	RespContentType      string `json:"Resp_Content-Type,omitempty"`
	RespTransferEncoding string `json:"Resp_Transfer-Encoding,omitempty"`
	RespContentEncoding  string `json:"Resp_Content-Encoding,omitempty"`
	RespExpires          string `json:"Resp_Expires,omitempty"`
	RespCacheControl     string `json:"Resp_Cache-Control,omitempty"`
	RespVary             string `json:"Resp_Vary,omitempty"`
	RespSetCookie        string `json:"Resp_Set-Cookie,omitempty"`
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
	p.eConn = elastigo.NewConn()
	p.eConn.SetPort(p.ApiPort)
	p.eConn.SetHosts([]string{p.Host})

	p.indexor = p.eConn.NewBulkIndexerErrors(50, 60)
	p.done = make(chan bool)
	p.indexor.Start()

	if Settings.verbose {
		// Only start the ErrorHandler goroutine when in verbose mode
		// no need to burn ressources otherwise
		go p.ErrorHandler()
	}

	log.Println("Initialized Elasticsearch Plugin")
	return
}

func (p *ESPlugin) IndexerShutdown() {
	p.indexor.Stop()
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
	req = payloadBody(req)

	esResp := ESRequestResponse{
		ReqURL:               string(proto.Path(req)),
		ReqMethod:            string(proto.Method(req)),
		ReqUserAgent:         string(proto.Header(req, []byte("User-Agent"))),
		ReqAcceptLanguage:    string(proto.Header(req, []byte("Accept-Language"))),
		ReqAccept:            string(proto.Header(req, []byte("Accept"))),
		ReqAcceptEncoding:    string(proto.Header(req, []byte("Accept-Encoding"))),
		ReqIfModifiedSince:   string(proto.Header(req, []byte("If-Modified-Since"))),
		ReqConnection:        string(proto.Header(req, []byte("Connection"))),
		ReqCookies:           string(proto.Header(req, []byte("Cookie"))),
		RespStatus:           string(proto.Status(resp)),
		RespStatusCode:       string(proto.Status(resp)),
		RespProto:            string(proto.Method(resp)),
		RespContentLength:    string(proto.Header(resp, []byte("Content-Length"))),
		RespContentType:      string(proto.Header(resp, []byte("Content-Type"))),
		RespTransferEncoding: string(proto.Header(resp, []byte("Transfer-Encoding"))),
		RespContentEncoding:  string(proto.Header(resp, []byte("Content-Encoding"))),
		RespExpires:          string(proto.Header(resp, []byte("Expires"))),
		RespCacheControl:     string(proto.Header(resp, []byte("Cache-Control"))),
		RespVary:             string(proto.Header(resp, []byte("Vary"))),
		RespSetCookie:        string(proto.Header(resp, []byte("Set-Cookie"))),
		Rtt:                  rtt,
		Timestamp:            t,
	}
	j, err := json.Marshal(&esResp)
	if err != nil {
		log.Println(err)
	} else {
		p.indexor.Index(p.Index, "RequestResponse", "", "", "", &t, j)
	}
	return
}
