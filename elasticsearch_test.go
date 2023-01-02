package goreplay

import (
	"testing"
)

const expectedIndex = "gor"

func assertExpectedGorIndex(index string, t *testing.T) {
	if expectedIndex != index {
		t.Fatalf("Expected index %s but got %s", expectedIndex, index)
	}
}

func assertExpectedIndex(expectedIndex string, index string, t *testing.T) {
	if expectedIndex != index {
		t.Fatalf("Expected index %s but got %s", expectedIndex, index)
	}
}

func assertExpectedError(returnedError error, t *testing.T) {
	expectedError := new(ESUriErorr)

	if expectedError != returnedError {
		t.Errorf("Expected err %s but got %s", expectedError, returnedError)
	}
}

func assertNoError(returnedError error, t *testing.T) {
	if nil != returnedError {
		t.Errorf("Expected no err but got %s", returnedError)
	}
}

// Argument host:port/index_name
// i.e : localhost:9200/gor
// Fail because scheme is mandatory
func TestElasticConnectionBuildFailWithoutScheme(t *testing.T) {
	uri := "localhost:9200/" + expectedIndex

	err, _ := parseURI(uri)
	assertExpectedError(err, t)
}

// Argument scheme://Host:port
// i.e : http://localhost:9200
// Fail : explicit index is required
func TestElasticConnectionBuildFailWithoutIndex(t *testing.T) {
	uri := "http://localhost:9200"

	err, index := parseURI(uri)

	assertExpectedIndex("", index, t)

	assertExpectedError(err, t)
}

// Argument scheme://Host/index_name
// i.e : http://localhost/gor
func TestElasticConnectionBuildFailWithoutPort(t *testing.T) {
	uri := "http://localhost/" + expectedIndex

	err, index := parseURI(uri)

	assertNoError(err, t)

	assertExpectedGorIndex(index, t)
}

// Argument scheme://Host:port/index_name
// i.e : http://localhost:9200/gor
func TestElasticLocalConnectionBuild(t *testing.T) {
	uri := "http://localhost:9200/" + expectedIndex

	err, index := parseURI(uri)

	assertNoError(err, t)

	assertExpectedGorIndex(index, t)
}

// Argument scheme://Host:port/index_name
// i.e : http://localhost.local:9200/gor or https://localhost.local:9200/gor
func TestElasticSimpleLocalWithSchemeConnectionBuild(t *testing.T) {
	uri := "http://localhost.local:9200/" + expectedIndex

	err, index := parseURI(uri)

	assertNoError(err, t)

	assertExpectedGorIndex(index, t)
}

// Argument scheme://Host:port/index_name
// i.e : http://localhost.local:9200/gor or https://localhost.local:9200/gor
func TestElasticSimpleLocalWithHTTPSConnectionBuild(t *testing.T) {
	uri := "https://localhost.local:9200/" + expectedIndex

	err, index := parseURI(uri)

	assertNoError(err, t)

	assertExpectedGorIndex(index, t)
}

// Argument scheme://Host:port/index_name
// i.e : localhost.local:9200/pathtoElastic/gor
func TestElasticLongPathConnectionBuild(t *testing.T) {
	uri := "http://localhost.local:9200/pathtoElastic/" + expectedIndex

	err, index := parseURI(uri)

	assertNoError(err, t)

	assertExpectedGorIndex(index, t)
}

// Argument scheme://Host:userinfo@port/index_name
// i.e : http://user:password@localhost.local:9200/gor
func TestElasticBasicAuthConnectionBuild(t *testing.T) {
	uri := "http://user:password@localhost.local:9200/" + expectedIndex

	err, index := parseURI(uri)

	assertNoError(err, t)

	assertExpectedGorIndex(index, t)
}

// Argument scheme://Host:port/path/index_name
// i.e : http://localhost.local:9200/path/gor or https://localhost.local:9200/path/gor
func TestElasticComplexPathConnectionBuild(t *testing.T) {
	uri := "http://localhost.local:9200/path/" + expectedIndex

	err, index := parseURI(uri)

	assertNoError(err, t)

	assertExpectedGorIndex(index, t)
}
