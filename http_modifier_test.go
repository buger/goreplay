package main

import (
	"bytes"
	"github.com/buger/gor/proto"
	"testing"
)

func TestHTTPModifierWithoutConfig(t *testing.T) {
	if NewHTTPModifier(&HTTPModifierConfig{}) != nil {
		t.Error("If no config specified should not be initialized")
	}
}

func TestHTTPModifierHeaderFilters(t *testing.T) {
	filters := HTTPHeaderFilters{}
	filters.Set("Host:^www.w3.org$")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		headerFilters: filters,
	})

	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if len(modifier.Rewrite(payload)) == 0 {
		t.Error("Request should pass filters")
	}

	filters = HTTPHeaderFilters{}
	// Setting filter that not match our header
	filters.Set("Host:^www.w4.org$")

	modifier = NewHTTPModifier(&HTTPModifierConfig{
		headerFilters: filters,
	})

	if len(modifier.Rewrite(payload)) != 0 {
		t.Error("Request should not pass filters")
	}
}

func TestHTTPModifierHeaderNegativeFilters(t *testing.T) {
	filters := HTTPHeaderFilters{}
	filters.Set("Host:^www.w3.org$")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		headerNegativeFilters: filters,
	})

	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w4.org\r\n\r\na=1&b=2")

	if len(modifier.Rewrite(payload)) == 0 {
		t.Error("Request should pass filters")
	}

	filters = HTTPHeaderFilters{}
	// Setting filter that not match our header
	filters.Set("Host:^www.w4.org$")

	modifier = NewHTTPModifier(&HTTPModifierConfig{
		headerNegativeFilters: filters,
	})

	if len(modifier.Rewrite(payload)) != 0 {
		t.Error("Request should not pass filters")
	}

	filters = HTTPHeaderFilters{}
	// Setting filter that not match our header
	filters.Set("Host: www*")

	modifier = NewHTTPModifier(&HTTPModifierConfig{
		headerNegativeFilters: filters,
	})

	if len(modifier.Rewrite(payload)) != 0 {
		t.Error("Request should not pass filters")
	}
}

func TestHTTPModifierURLRewrite(t *testing.T) {
	var url, newURL []byte

	rewrites := UrlRewriteMap{}

	payload := func(url []byte) []byte {
		return []byte("POST " + string(url) + " HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	}

	err := rewrites.Set("/v1/user/([^\\/]+)/ping:/v2/user/$1/ping")
	if err != nil {
		t.Error("Should not error on /v1/user/([^\\/]+)/ping:/v2/user/$1/ping")
	}

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		urlRewrite: rewrites,
	})

	url = []byte("/v1/user/joe/ping")
	if newURL = proto.Path(modifier.Rewrite(payload(url))); bytes.Equal(newURL, url) {
		t.Error("Request url should have been rewritten, wasn't", string(newURL))
	}

	url = []byte("/v1/user/ping")
	if newURL = proto.Path(modifier.Rewrite(payload(url))); !bytes.Equal(newURL, url) {
		t.Error("Request url should have been rewritten, wasn't", string(newURL))
	}
}

func TestHTTPModifierHeaderHashFilters(t *testing.T) {
	filters := HTTPHashFilters{}
	filters.Set("Header2:1/2")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		headerHashFilters: filters,
	})

	payload := func(header []byte) []byte {
		return []byte("POST / HTTP/1.1\r\n" + string(header) + "Content-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	}

	if p := modifier.Rewrite(payload([]byte(""))); len(p) == 0 {
		t.Error("Request should pass filters if Header does not exist")
	}

	if p := modifier.Rewrite(payload([]byte("Header2: 3\r\n"))); len(p) > 0 {
		t.Error("Request should not pass filters, Header2 hash too high")
	}

	if p := modifier.Rewrite(payload([]byte("Header2: 1\r\n"))); len(p) == 0 {
		t.Error("Request should pass filters")
	}
}

func TestHTTPModifierParamHashFilters(t *testing.T) {
	filters := HTTPHashFilters{}
	filters.Set("user_id:1/2")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		paramHashFilters: filters,
	})

	payload := func(value []byte) []byte {
		return []byte("POST /" + string(value) + " HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	}

	if p := modifier.Rewrite(payload([]byte(""))); len(p) == 0 {
		t.Error("Request should pass filters if param does not exist")
	}

	if p := modifier.Rewrite(payload([]byte("?user_id=3"))); len(p) > 0 {
		t.Error("Request should not pass filters", string(p))
	}

	if p := modifier.Rewrite(payload([]byte("?user_id=1"))); len(p) == 0 {
		t.Error("Request should pass filters")
	}
}

func TestHTTPModifierHeaders(t *testing.T) {
	headers := HTTPHeaders{}
	headers.Set("Header1:1")
	headers.Set("Host:localhost")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		headers: headers,
	})

	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	newPayload := []byte("POST /post HTTP/1.1\r\nHeader1: 1\r\nContent-Length: 7\r\nHost: localhost\r\n\r\na=1&b=2")

	if payload = modifier.Rewrite(payload); !bytes.Equal(payload, newPayload) {
		t.Error("Should update request headers", string(payload))
	}
}

func TestHTTPModifierURLRegexp(t *testing.T) {
	filters := HTTPUrlRegexp{}
	filters.Set("/v1/app")
	filters.Set("/v1/api")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		urlRegexp: filters,
	})

	payload := func(url string) []byte {
		return []byte("POST " + url + " HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	}

	if len(modifier.Rewrite(payload("/v1/app/test"))) == 0 {
		t.Error("Should pass url")
	}

	if len(modifier.Rewrite(payload("/v1/api/test"))) == 0 {
		t.Error("Should pass url")
	}

	if len(modifier.Rewrite(payload("/other"))) > 0 {
		t.Error("Should not pass url")
	}
}

func TestHTTPModifierURLNegativeRegexp(t *testing.T) {
	filters := HTTPUrlRegexp{}
	filters.Set("/restricted1")
	filters.Set("/some/restricted2")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		urlNegativeRegexp: filters,
	})

	payload := func(url string) []byte {
		return []byte("POST " + url + " HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	}

	if len(modifier.Rewrite(payload("/v1/app/test"))) == 0 {
		t.Error("Should pass url")
	}

	if len(modifier.Rewrite(payload("/restricted1"))) > 0 {
		t.Error("Should not pass url")
	}

	if len(modifier.Rewrite(payload("/some/restricted2"))) > 0 {
		t.Error("Should not pass url")
	}
}

func TestHTTPModifierSetHeader(t *testing.T) {
	filters := HTTPHeaders{}
	filters.Set("User-Agent:Gor")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		headers: filters,
	})

	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter := []byte("POST /post HTTP/1.1\r\nUser-Agent: Gor\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = modifier.Rewrite(payload); !bytes.Equal(payloadAfter, payload) {
		t.Error("Should add new header", string(payload))
	}
}

func TestHTTPModifierSetParam(t *testing.T) {
	filters := HTTPParams{}
	filters.Set("api_key=1")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		params: filters,
	})

	payload := []byte("POST /post?api_key=1234 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter := []byte("POST /post?api_key=1 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = modifier.Rewrite(payload); !bytes.Equal(payloadAfter, payload) {
		t.Error("Should override param", string(payload))
	}
}
