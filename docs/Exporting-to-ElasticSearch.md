Gor can export requests and replayed response data to ElasticSearch:

```
./gor --input-raw :8000 --output-http http://staging.com  --output-http-elasticsearch localhost:9200/gor
```

You don't have to create the index upfront. That will be done for you automatically.

### Format

Following structure represents ES format:

```
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
```