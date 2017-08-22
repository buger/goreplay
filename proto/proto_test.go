package proto

import (
	"bytes"
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

	// Wrong delimeter
	payload = []byte("GET /p HTTP/1.1\r\nCookie: 123\nHost: www.w3.org\r\n\r\n")

	if val = Header(payload, []byte("Cookie")); !bytes.Equal(val, []byte("123")) {
		t.Error("Should handle wrong header delimeter")
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

	headers := make(map[string]string)

	ParseHeaders(payload, func(header []byte, value []byte) bool {
		headers[string(header)] = string(value)
		return true
	})

	expected := map[string]string{
		"Content-Length": "7",
		"Host":           "www.w3.org",
		"User-Agent":     "Chrome",
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
		ParseHeaders([][]byte{[]byte(f)}, func(header []byte, value []byte) bool {
			return true
		})
	}
}

func TestParseHeadersWithComplexUserAgent(t *testing.T) {
	// User-Agent could contain inside ':'
	// Parser should wait for \r\n
	payload := [][]byte{[]byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.or"), []byte("g\r\nUser-Ag"), []byte("ent:Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko\r\n\r\n"), []byte("Fake-Header: asda")}

	headers := make(map[string]string)

	ParseHeaders(payload, func(header []byte, value []byte) bool {
		headers[string(header)] = string(value)
		return true
	})

	expected := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko",
	}

	if expected["User-Agent"] != headers["User-Agent"] {
		t.Errorf("Header 'User-Agent' expected '%s' and parsed: '%s'", expected["User-Agent"], headers["User-Agent"])
	}
}

func TestParseHeadersWithOrigin(t *testing.T) {
	// User-Agent could contain inside ':'
	// Parser should wait for \r\n
	payload := [][]byte{[]byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.or"), []byte("g\r\nReferrer: http://127.0.0.1:3000\r\nOrigi"), []byte("n: https://www.example.com\r\nUser-Ag"), []byte("ent:Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko\r\n\r\n"), []byte("in:https://www.example.com\r\n\r\n"), []byte("Fake-Header: asda")}

	headers := make(map[string]string)

	ParseHeaders(payload, func(header []byte, value []byte) bool {
		headers[string(header)] = string(value)
		return true
	})

	expected := map[string]string{
		"Origin":     "https://www.example.com",
		"User-Agent": "Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko",
		"Referrer":   "http://127.0.0.1:3000",
	}

	if expected["Referrer"] != headers["Referrer"] {
		t.Errorf("Header 'Referrer' expected '%s' and parsed: '%s'", expected["Referrer"], headers["Referrer"])
	}

	if expected["Origin"] != headers["Origin"] {
		t.Errorf("Header 'Origin' expected '%s' and parsed: '%s'", expected["Origin"], headers["Origin"])
	}

	if expected["User-Agent"] != headers["User-Agent"] {
		t.Errorf("Header 'User-Agent' expected '%s' and parsed: '%s'", expected["User-Agent"], headers["User-Agent"])
	}
}

func TestHeaderEquals(t *testing.T) {
	tests := []struct {
		h1     string
		h2     string
		equals bool
	}{
		{"Content-Length", "content-length", true},
		{"content-length", "Content-Length", true},
		{"content-Pength", "Content-Length", false},
		{"Host", "Content-Length", false},
	}

	for _, tc := range tests {
		if HeadersEqual([]byte(tc.h1), []byte(tc.h2)) != tc.equals {
			t.Error(tc)
		}
	}
}

func TestPath(t *testing.T) {
	var path, payload []byte

	payload = []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if path = Path(payload); !bytes.Equal(path, []byte("/post")) {
		t.Error("Should find path", string(path))
	}

	payload = []byte("GET /get\r\n\r\nHost: www.w3.org\r\n\r\n")

	if path = Path(payload); !bytes.Equal(path, []byte("/get")) {
		t.Error("Should find path", string(path))
	}

	payload = []byte("GET /get\n")

	if path = Path(payload); !bytes.Equal(path, []byte("/get")) {
		t.Error("Should find path", string(path))
	}

	payload = []byte("GET /get")

	if path = Path(payload); !bytes.Equal(path, []byte("/get")) {
		t.Error("Should find path", string(path))
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

	payload = []byte("POST /post?param=test&user_id=1 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if val, _, _ := PathParam(payload, []byte("param")); !bytes.Equal(val, []byte("test")) {
		t.Error("Should detect attribute", string(val))
	}

	if val, _, _ := PathParam(payload, []byte("user_id")); !bytes.Equal(val, []byte("1")) {
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
}
