package main

import (
	"context"
	"log"
	"time"
	"github.com/buger/goreplay/proto"
	"github.com/olivere/elastic"
)

type ESPlugin struct {
	Url   string
}

type ESRequestResponse struct {
	ReqHost              string `json:"Req_Host"`
	ReqMethod            string `json:"Req_Method"`
	ReqURL               string `json:"Req_URL"`
	ReqBody              string `json:"Req_Body"`
	ReqUserAgent         string `json:"Req_User-Agent"`
	ReqXRealIP           string `json:"Req_X-Real-IP"`
	ReqXForwardedFor     string `json:"Req_X-Forwarded-For"`
	ReqConnection        string `json:"Req_Connection,omitempty"`
	ReqCookies           string `json:"Req_Cookies,omitempty"`
	RespStatusCode       string `json:"Resp_Status-Code"`
	RespBody             string `json:"Resp_Body"`
	RespProto            string `json:"Resp_Proto,omitempty"`
	RespContentLength    string `json:"Resp_Content-Length,omitempty"`
	RespContentType      string `json:"Resp_Content-Type,omitempty"`
	RespSetCookie        string `json:"Resp_Set-Cookie,omitempty"`
	Timestamp            time.Time
}

func (p *ESPlugin) Init(URI string) {
	p.Url = URI
	log.Println("Initialized Elasticsearch Plugin")
	return
}

func (p *ESPlugin) ResponseAnalyze(req, resp []byte) {
	if len(resp) == 0 {
		// nil http response - skipped elasticsearch export for this request
		return
	}

	t := time.Now()
	index := "gor-" + t.Format("2006-01-02")
	req = payloadBody(req)

	host := ESRequestResponse{
	    ReqHost:              string(proto.Header(req, []byte("Host"))),
	    ReqMethod:            string(proto.Method(req)),
		ReqURL:               string(proto.Path(req)),
		ReqBody:              string(proto.Body(req)),
		ReqUserAgent:         string(proto.Header(req, []byte("User-Agent"))),
		ReqXRealIP:           string(proto.Header(req, []byte("X-Real-IP"))),
		ReqXForwardedFor:     string(proto.Header(req, []byte("X-Forwarded-For"))),
		ReqConnection:        string(proto.Header(req, []byte("Connection"))),
		ReqCookies:           string(proto.Header(req, []byte("Cookie"))),
		RespStatusCode:       string(proto.Status(resp)),
		RespProto:            string(proto.Method(resp)),
		RespBody:             string(proto.Body(resp)),
		RespContentLength:    string(proto.Header(resp, []byte("Content-Length"))),
		RespContentType:      string(proto.Header(resp, []byte("Content-Type"))),
		RespSetCookie:        string(proto.Header(resp, []byte("Set-Cookie"))),
		Timestamp:            t,
	}

	client, err := elastic.NewSimpleClient(elastic.SetURL(p.Url))
	if err != nil {
		log.Println(err)
	}

	exists, err := client.IndexExists(index).Do(context.Background())
	if err != nil {
		log.Println(err)
	}

	if !exists {
		_, err := client.CreateIndex(index).Do(context.Background())
		if err != nil {
			log.Println(err)
		}
	}

	h, err := client.Index().Index(index).Type("ESRequestResponse").BodyJson(host).Do(context.Background())
	if err != nil {
		log.Println(err)
	}
	log.Printf("Indexed data with ID %s to index %s, type %s\n", h.Id, h.Index, h.Type)
	return
}