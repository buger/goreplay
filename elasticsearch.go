package main

import (
	"encoding/json"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/buger/goreplay/proto"

	elastigo "github.com/mattbaird/elastigo/lib"
)

type ESUriErorr struct{}

func (e *ESUriErorr) Error() string {
	return "Wrong ElasticSearch URL format. Expected to be: scheme://host/index_name"
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
// Proper format is: scheme://[userinfo@]host/index_name
// userinfo is: user[:password]
// net/url.Parse() does not fail if scheme is not provided but actualy does not
// handle URI properly.
// So we must 'validate' URI format to match requirements to use net/url.Parse()
func parseURI(URI string) (err error, index string) {

	parsedUrl, parseErr := url.Parse(URI)

	if parseErr != nil {
		err = new(ESUriErorr)
		return
	}

	//	check URL validity by extracting host and index values.
	host := parsedUrl.Host
	urlPathParts := strings.Split(parsedUrl.Path, "/")
	index = urlPathParts[len(urlPathParts)-1]

	// force index specification in uri : ie no implicit index
	if host == "" || index == "" {
		err = new(ESUriErorr)
	}

	return
}

func (p *ESPlugin) Init(URI string) {
	var err error

	err, p.Index = parseURI(URI)

	if err != nil {
		log.Fatal("Can't initialize ElasticSearch plugin.", err)
	}

	p.eConn = elastigo.NewConn()

	p.eConn.SetFromUrl(URI)

	p.indexor = p.eConn.NewBulkIndexerErrors(50, 60)
	p.done = make(chan bool)
	p.indexor.Start()

	go p.ErrorHandler()

	Debug(1, "Initialized Elasticsearch Plugin")
	return
}

func (p *ESPlugin) IndexerShutdown() {
	p.indexor.Stop()
	return
}

func (p *ESPlugin) ErrorHandler() {
	for {
		errBuf := <-p.indexor.ErrorChannel
		Debug(1, "[ELASTICSEARCH]", errBuf.Err)
	}
}

func (p *ESPlugin) RttDurationToMs(d time.Duration) int64 {
	sec := d / time.Second
	nsec := d % time.Second
	fl := float64(sec) + float64(nsec)*1e-6
	return int64(fl)
}

// ResponseAnalyze send req and resp to ES
func (p *ESPlugin) ResponseAnalyze(req, resp []byte, start, stop time.Time) {
	if len(resp) == 0 {
		// nil http response - skipped elasticsearch export for this request
		return
	}
	t := time.Now()
	rtt := p.RttDurationToMs(stop.Sub(start))

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
		Debug(0, "[ELASTIC-RESPONSE]", err)
	} else {
		p.indexor.Index(p.Index, "RequestResponse", "", "", "", &t, j)
	}
	return
}
