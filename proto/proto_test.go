package proto

import (
	"bytes"
	"net/textproto"
	"reflect"
	"testing"
)

func TestHeader(t *testing.T) {
	var payload, val []byte
	var headerStart int

	// Value with space at start
	payload = []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if val = Header(payload, []byte("Content-Length")); !bytes.Equal(val, []byte("7")) {
		t.Error("Should find header value")
	}

	// Value with space at end
	payload = []byte("POST /post HTTP/1.1\r\nContent-Length: 7 \r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if val = Header(payload, []byte("Content-Length")); !bytes.Equal(val, []byte("7")) {
		t.Error("Should find header value without space after 7")
	}

	// Value without space at start
	payload = []byte("POST /post HTTP/1.1\r\nContent-Length:7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if val = Header(payload, []byte("Content-Length")); !bytes.Equal(val, []byte("7")) {
		t.Error("Should find header value without space after :")
	}

	// Value is empty
	payload = []byte("GET /p HTTP/1.1\r\nCookie:\r\nHost: www.w3.org\r\n\r\n")

	if val = Header(payload, []byte("Cookie")); len(val) > 0 {
		t.Error("Should return empty value")
	}

	// Header not found
	if _, headerStart, _, _, _ = header(payload, []byte("Not-Found")); headerStart != -1 {
		t.Error("Should not found header")
	}

	// Lower case headers
	payload = []byte("POST /post HTTP/1.1\r\ncontent-length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if val = Header(payload, []byte("Content-Length")); !bytes.Equal(val, []byte("7")) {
		t.Error("Should find lower case 2 word header")
	}

	payload = []byte("POST /post HTTP/1.1\r\ncontent-length: 7\r\nhost: www.w3.org\r\n\r\na=1&b=2")

	if val = Header(payload, []byte("host")); !bytes.Equal(val, []byte("www.w3.org")) {
		t.Error("Should find lower case 1 word header")
	}
}

func TestMIMEHeadersEndPos(t *testing.T) {
	head := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\n")
	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	end := MIMEHeadersEndPos(payload)

	if !bytes.Equal(payload[:end], head) {
		t.Error("Wrong headers end position:", end, head, payload[:end])
	}
}

func TestMIMEHeadersStartPos(t *testing.T) {
	headers := []byte("Content-Length: 7\r\nHost: www.w3.org")
	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	start := MIMEHeadersStartPos(payload)
	end := MIMEHeadersEndPos(payload) - 4

	if !bytes.Equal(payload[start:end], headers) {
		t.Error("Wrong headers end position:", start, end, payload[start:end])
	}
}

func TestSetHeader(t *testing.T) {
	var payload, payloadAfter []byte

	payload = []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter = []byte("POST /post HTTP/1.1\r\nContent-Length: 14\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = SetHeader(payload, []byte("Content-Length"), []byte("14")); !bytes.Equal(payload, payloadAfter) {
		t.Error("Should update header if it exists", string(payload))
	}

	payload = []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter = []byte("POST /post HTTP/1.1\r\nUser-Agent: Gor\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = SetHeader(payload, []byte("User-Agent"), []byte("Gor")); !bytes.Equal(payload, payloadAfter) {
		t.Error("Should add header if not found", string(payload))
	}
	invalidPayload := []byte("POST /post HTTP/1.1")
	if invalidPayload = SetHeader(invalidPayload, []byte("User-Agent"), []byte("Gor")); !bytes.Equal(invalidPayload, []byte("POST /post HTTP/1.1")) {
		t.Error("Should not modify payload if request is invalid", string(payload))
	}
}

func TestDeleteHeader(t *testing.T) {
	var payload, payloadAfter []byte

	payload = []byte("POST /post HTTP/1.1\r\nUser-Agent: Gor\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter = []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = DeleteHeader(payload, []byte("User-Agent")); !bytes.Equal(payload, payloadAfter) {
		t.Error("Should delete header if found", string(payload), string(payloadAfter))
	}

	//Whitespace at end of User-Agent
	payload = []byte("POST /post HTTP/1.1\r\nUser-Agent: Gor \r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter = []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = DeleteHeader(payload, []byte("User-Agent")); !bytes.Equal(payload, payloadAfter) {
		t.Error("Should delete header if found", string(payload), string(payloadAfter))
	}
}

func TestParseHeaders(t *testing.T) {
	payload := [][]byte{[]byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.or"), []byte("g\r\nUser-Ag"), []byte("ent:Chrome\r\n\r\n"), []byte("Fake-Header: asda")}

	headers := ParseHeaders(bytes.Join(payload, nil))

	expected := textproto.MIMEHeader{
		"Content-Length": []string{"7"},
		"Host":           []string{"www.w3.org"},
		"User-Agent":     []string{"Chrome"},
	}

	if !reflect.DeepEqual(headers, expected) {
		t.Error("Headers do not properly parsed", headers)
	}
}

// See https://github.com/dvyukov/go-fuzz and fuzz.go
func TestFuzzCrashers(t *testing.T) {
	var crashers = []string{
		"\n:00\n",
	}

	for _, f := range crashers {
		ParseHeaders([]byte(f))
	}
}

func TestParseHeadersWithComplexUserAgent(t *testing.T) {
	// User-Agent could contain inside ':'
	// Parser should wait for \r\n
	payload := [][]byte{[]byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.or"), []byte("g\r\nUser-Ag"), []byte("ent:Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko\r\n\r\n"), []byte("Fake-Header: asda")}

	headers := ParseHeaders(bytes.Join(payload, nil))

	expected := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko",
	}

	if expected["User-Agent"] != headers["User-Agent"][0] {
		t.Errorf("Header 'User-Agent' expected '%s' and parsed: '%s'", expected["User-Agent"], headers["User-Agent"])
	}
}

func TestParseHeadersWithOrigin(t *testing.T) {
	// User-Agent could contain inside ':'
	// Parser should wait for \r\n
	payload := [][]byte{[]byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.or"), []byte("g\r\nReferrer: http://127.0.0.1:3000\r\nOrigi"), []byte("n: https://www.example.com\r\nUser-Ag"), []byte("ent:Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko\r\n\r\n"), []byte("in:https://www.example.com\r\n\r\n"), []byte("Fake-Header: asda")}

	headers := ParseHeaders(bytes.Join(payload, nil))

	expected := map[string]string{
		"Origin":     "https://www.example.com",
		"User-Agent": "Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko",
		"Referrer":   "http://127.0.0.1:3000",
	}

	if expected["Referrer"] != headers["Referrer"][0] {
		t.Errorf("Header 'Referrer' expected '%s' and parsed: '%s'", expected["Referrer"], headers["Referrer"])
	}

	if expected["Origin"] != headers["Origin"][0] {
		t.Errorf("Header 'Origin' expected '%s' and parsed: '%s'", expected["Origin"], headers["Origin"])
	}

	if expected["User-Agent"] != headers["User-Agent"][0] {
		t.Errorf("Header 'User-Agent' expected '%s' and parsed: '%s'", expected["User-Agent"], headers["User-Agent"])
	}
}

func TestPath(t *testing.T) {
	var path, payload []byte

	payload = []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if path = Path(payload); !bytes.Equal(path, []byte("/post")) {
		t.Error("Should find path", string(path))
	}

	payload = []byte("GET /get\r\n\r\nHost: www.w3.org\r\n\r\n")

	if path = Path(payload); !bytes.Equal(path, nil) {
		t.Error("1Should not find path", string(path))
	}

	payload = []byte("GET /get\n")

	if path = Path(payload); !bytes.Equal(path, nil) {
		t.Error("2Should not find path", string(path))
	}

	payload = []byte("GET /get")

	if path = Path(payload); !bytes.Equal(path, nil) {
		t.Error("3Should not find path", string(path))
	}
}

func TestSetPath(t *testing.T) {
	var payload, payloadAfter []byte

	payload = []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter = []byte("POST /new_path HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = SetPath(payload, []byte("/new_path")); !bytes.Equal(payload, payloadAfter) {
		t.Error("Should replace path", string(payload))
	}

}

func TestPathParam(t *testing.T) {
	var payload []byte

	payload = []byte("POST /post?param=test&user_id=1&d_type=1&type=2&d_type=3 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if val, _, _ := PathParam(payload, []byte("param")); !bytes.Equal(val, []byte("test")) {
		t.Error("Should detect attribute", string(val))
	}

	if val, _, _ := PathParam(payload, []byte("user_id")); !bytes.Equal(val, []byte("1")) {
		t.Error("Should detect attribute", string(val))
	}

	if val, _, _ := PathParam(payload, []byte("type")); !bytes.Equal(val, []byte("2")) {
		t.Error("Should detect attribute", string(val))
	}

	if val, _, _ := PathParam(payload, []byte("d_type")); !bytes.Equal(val, []byte("1")) {
		// this function is not designed for cases with duplicate param keys
		t.Error("Should detect attribute", string(val))
	}
}

func TestSetPathParam(t *testing.T) {
	var payload, payloadAfter []byte

	payload = []byte("POST /post?param=test&user_id=1 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter = []byte("POST /post?param=new&user_id=1 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = SetPathParam(payload, []byte("param"), []byte("new")); !bytes.Equal(payload, payloadAfter) {
		t.Error("Should replace existing value", string(payload))
	}

	payload = []byte("POST /post?param=test&user_id=1 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter = []byte("POST /post?param=test&user_id=2 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = SetPathParam(payload, []byte("user_id"), []byte("2")); !bytes.Equal(payload, payloadAfter) {
		t.Error("Should replace existing value", string(payload))
	}

	payload = []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter = []byte("POST /post?param=test HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = SetPathParam(payload, []byte("param"), []byte("test")); !bytes.Equal(payload, payloadAfter) {
		t.Error("Should set param if url have no params", string(payload))
	}

	payload = []byte("POST /post?param=test HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter = []byte("POST /post?param=test&user_id=1 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = SetPathParam(payload, []byte("user_id"), []byte("1")); !bytes.Equal(payload, payloadAfter) {
		t.Error("Should set param at the end if url params", string(payload))
	}
}

func TestSetHostHTTP10(t *testing.T) {
	var payload, payloadAfter []byte

	payload = []byte("POST http://example.com/post HTTP/1.0\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter = []byte("POST http://new.com/post HTTP/1.0\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = SetHost(payload, []byte("http://new.com"), []byte("new.com")); !bytes.Equal(payload, payloadAfter) {
		t.Error("Should replace host", string(payload))
	}

	payload = []byte("POST /post HTTP/1.0\r\nContent-Length: 7\r\nHost: example.com\r\n\r\na=1&b=2")
	payloadAfter = []byte("POST /post HTTP/1.0\r\nContent-Length: 7\r\nHost: new.com\r\n\r\na=1&b=2")

	if payload = SetHost(payload, nil, []byte("new.com")); !bytes.Equal(payload, payloadAfter) {
		t.Error("Should replace host", string(payload))
	}

	payload = []byte("POST /post HTTP/1.0\r\nContent-Length: 7\r\n\r\na=1&b=2")

	if payload = SetHost(payload, nil, []byte("new.com")); !bytes.Equal(payload, payload) {
		t.Error("Should replace host", string(payload))
	}
}

func TestHasResponseTitle(t *testing.T) {
	var m = map[string]bool{
		"HTTP":                      false,
		"":                          false,
		"HTTP/1.1 100 Continue":     false,
		"HTTP/1.1 100 Continue\r\n": true,
		"HTTP/1.1  \r\n":            false,
		"HTTP/4.0 100Continue\r\n":  false,
		"HTTP/1.0 10r Continue\r\n": false,
		"HTTP/1.1 200\r\n":          false,
		"HTTP/1.1 200\r\nServer: Tengine\r\nContent-Length: 0\r\nConnection: close\r\n\r\n": false,
	}
	for k, v := range m {
		if HasResponseTitle([]byte(k)) != v {
			t.Errorf("%q should yield %v", k, v)
			break
		}
	}
}

func TestHasRequestTitle(t *testing.T) {
	var m = map[string]bool{
		"POST /post HTTP/1.0\r\n": true,
		"":                        false,
		"POST /post HTTP/1.\r\n":  false,
		"POS /post HTTP/1.1\r\n":  false,
		"GET / HTTP/1.1\r\n":      true,
		"GET / HTTP/1.1\r":        false,
		"GET / HTTP/1.400\r\n":    false,
	}
	for k, v := range m {
		if HasRequestTitle([]byte(k)) != v {
			t.Errorf("%q should yield %v", k, v)
			break
		}
	}
}

func TestCheckChunks(t *testing.T) {
	var m = "4\r\nWiki\r\n5\r\npedia\r\nE\r\n in\r\n\r\nchunks.\r\n0\r\n\r\n"
	chunkEnd := CheckChunked([]byte(m))
	expected := bytes.Index([]byte(m), []byte("0\r\n")) + 5
	if chunkEnd != expected {
		t.Errorf("expected %d to equal %d", chunkEnd, expected)
	}

	m = "7\r\nMozia\r\n9\r\nDeveloper\r\n7\r\nNetwork\r\n0\r\n\r\n"
	chunkEnd = CheckChunked([]byte(m))
	if chunkEnd != -1 {
		t.Errorf("expected %d to equal %d", chunkEnd, -1)
	}

	// with trailers
	m = "4\r\nWiki\r\n5\r\npedia\r\nE\r\n in\r\n\r\nchunks.\r\n0\r\n\r\nEXpires"
	chunkEnd = CheckChunked([]byte(m))
	expected = bytes.Index([]byte(m), []byte("0\r\n")) + 5
	if chunkEnd != expected {
		t.Errorf("expected %d to equal %d", chunkEnd, expected)
	}

	// last chunk inside the the body
	// with trailers
	m = "4\r\nWiki\r\n5\r\npedia\r\nE\r\n in\r\n\r\nchunks.\r\n3\r\n0\r\n\r\n0\r\n\r\nEXpires"
	chunkEnd = CheckChunked([]byte(m))
	expected = bytes.Index([]byte(m), []byte("0\r\n")) + 10
	if chunkEnd != expected {
		t.Errorf("expected %d to equal %d", chunkEnd, expected)
	}

	// checks with chucks-extensions
	m = "4\r\nWiki\r\n5\r\npedia\r\nE; name='quoted string'\r\n in\r\n\r\nchunks.\r\n3\r\n0\r\n\r\n0\r\n\r\nEXpires"
	chunkEnd = CheckChunked([]byte(m))
	expected = bytes.Index([]byte(m), []byte("0\r\n")) + 10
	if chunkEnd != expected {
		t.Errorf("expected %d to equal %d", chunkEnd, expected)
	}
}

func TestHasFullPayload(t *testing.T) {
	var m = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n7\r\nMozilla\r\n9\r\nDeveloper\r\n7\r\nNetwork\r\n0\r\n\r\n"
	got := HasFullPayload([]byte(m), nil)
	expected := true
	if got != expected {
		t.Errorf("expected %v to equal %v", got, expected)
	}

	// check with invalid chunk format
	m = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n7\r\nMozia\r\n9\r\nDeveloper\r\n7\r\nNetwork\r\n0\r\n\r\n"
	got = HasFullPayload([]byte(m), nil)
	expected = false
	if got != expected {
		t.Errorf("expected %v to equal %v", got, expected)
	}

	// check chunks with trailers
	m = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\nTrailer: Expires\r\n\r\n7\r\nMozilla\r\n9\r\nDeveloper\r\n7\r\nNetwork\r\n0\r\n\r\nExpires: Wed, 21 Oct 2015 07:28:00 GMT\r\n\r\n"
	got = HasFullPayload([]byte(m), nil)
	expected = true
	if got != expected {
		t.Errorf("expected %v to equal %v", got, expected)
	}

	// check with missing trailers
	m = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\nTrailer: Expires\r\n\r\n7\r\nMozilla\r\n9\r\nDeveloper\r\n7\r\nNetwork\r\n0\r\n\r\nExpires: Wed, 21 Oct 2015 07:28:00"
	got = HasFullPayload([]byte(m), nil)
	expected = false
	if got != expected {
		t.Errorf("expected %v to equal %v", got, expected)
	}

	// check with content-length
	m = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 23\r\n\r\nMozillaDeveloperNetwork"
	got = HasFullPayload([]byte(m), nil)
	expected = true
	if got != expected {
		t.Errorf("expected %v to equal %v", got, expected)
	}

	// check missing total length
	m = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 23\r\n\r\nMozillaDeveloperNet"
	got = HasFullPayload([]byte(m), nil)
	expected = false
	if got != expected {
		t.Errorf("expected %v to equal %v", got, expected)
	}

	// check with no body
	m = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\n"
	got = HasFullPayload([]byte(m), nil)
	expected = true
	if got != expected {
		t.Errorf("expected %v to equal %v", got, expected)
	}
}

func BenchmarkHasFullPayload(b *testing.B) {
	var buf bytes.Buffer
	buf.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n"))
	var chunk = []byte("1e\r\n111111111111111111111111111111\r\n")
	for i := 0; i < 5000; i++ {
		buf.Write(chunk)
	}
	buf.Write([]byte("0\r\n\r\n"))
	data := buf.Bytes()
	b.ResetTimer() // ignores the upper initialization
	b.ReportMetric(float64(5000), "chunks/op")
	for i := 0; i < b.N; i++ {
		if !HasFullPayload(data, nil) {
			b.Fail()
		}
	}
}
