package main

import (
	"testing"
)

// Argument host:port/index_name
// i.e : localhost:9200/gor
func TestElasticConnectionBuildFailWithoutScheme(t *testing.T) {
	uri := "localhost:9200/gor"
	expectedErr := new(ESUriErorr)

  err, _, _, _ := parseURI(uri)

	if expectedErr != err {
		t.Errorf("Expected err %s but got %s", expectedErr, err)
	}
}

// Argument scheme://host/index_name
// i.e : http://localhost/gor
func TestElasticConnectionBuildFailWithoutPort(t *testing.T) {
	uri := "http://localhost/gor"
	expectedErr := new(ESUriErorr)

  err, _, _, _ := parseURI(uri)

	if expectedErr != err {
		t.Errorf("Expected err %s but got %s", expectedErr, err)
	}
}

// Argument scheme://host:port
// i.e : http://localhost:9200
func TestElasticConnectionBuildFailWithoutIndex(t *testing.T) {
	uri := "http://localhost:9200"
	expectedErr := new(ESUriErorr)

  err, _, _, _ := parseURI(uri)

	if expectedErr != err {
		t.Errorf("Expected err %s but got %s", expectedErr, err)
	}
}

// Argument host:port/index_name
// i.e : localhost.local:9200/gor
func TestElasticLocalConnectionBuild(t *testing.T) {
	uri := "http://localhost:9200/gor"
	expectedHost := "http://localhost"
	expectedApiPort := "9200"
	expectedIndex := "gor"

	err, host, apiPort, index := parseURI(uri)

	if nil != err {
		t.Fatalf("Expected err %s but got %s", nil, err)
	}
	if expectedHost != host {
		t.Fatalf("Expected host %s but got %s", expectedHost, host)
	}
	if expectedApiPort != apiPort {
		t.Fatalf("Expected port %s but got %s", expectedApiPort, apiPort)
	}
	if expectedIndex != index {
		t.Fatalf("Expected index %s but got %s", expectedIndex, index)
	}
}

// Argument host:port/index_name
// i.e : http://localhost.local:9200/gor or https://localhost.local:9200/gor
func TestElasticSimpleLocalWithSchemeConnectionBuild(t *testing.T) {
	uri := "http://localhost.local:9200/gor"
	expectedHost := "http://localhost.local"
	expectedApiPort := "9200"
	expectedIndex := "gor"

	err, host, apiPort, index := parseURI(uri)

	if nil != err {
		t.Fatalf("Expected err %s but got %s", nil, err)
	}
	if expectedHost != host {
		t.Fatalf("Expected host %s but got %s", expectedHost, host)
	}
	if expectedApiPort != apiPort {
		t.Fatalf("Expected port %s but got %s", expectedApiPort, apiPort)
	}
	if expectedIndex != index {
		t.Fatalf("Expected index %s but got %s", expectedIndex, index)
	}
}

// Argument host:port/index_name
// i.e : http://localhost.local:9200/gor or https://localhost.local:9200/gor
func TestElasticSimpleLocalWithHTTPSConnectionBuild(t *testing.T) {
	uri := "https://localhost.local:9200/gor"
	expectedHost := "https://localhost.local"
	expectedApiPort := "9200"
	expectedIndex := "gor"

	err, host, apiPort, index := parseURI(uri)

	if nil != err {
		t.Fatalf("Expected err %s but got %s", nil, err)
	}
	if expectedHost != host {
		t.Fatalf("Expected host %s but got %s", expectedHost, host)
	}
	if expectedApiPort != apiPort {
		t.Fatalf("Expected port %s but got %s", expectedApiPort, apiPort)
	}
	if expectedIndex != index {
		t.Fatalf("Expected index %s but got %s", expectedIndex, index)
	}
}

// Argument host:port/index_name
// i.e : localhost.local:9200/pathtoElastic/gor
func TestElasticLongPathConnectionBuild(t *testing.T) {
	uri := "http://localhost.local:9200/pathtoElastic/gor"
	expectedHost := "http://localhost.local"
	expectedApiPort := "9200"
	expectedIndex := "gor"

	err, host, apiPort, index := parseURI(uri)

	if nil != err {
		t.Fatalf("Expected err %s but got %s", nil, err)
	}
	if expectedHost != host {
		t.Fatalf("Expected host %s but got %s", expectedHost, host)
	}
	if expectedApiPort != apiPort {
		t.Fatalf("Expected port %s but got %s", expectedApiPort, apiPort)
	}
	if expectedIndex != index {
		t.Fatalf("Expected index %s but got %s", expectedIndex, index)
	}
}

// Argument host:port/index_name
// i.e : http://user:password@localhost.local:9200/gor
func TestElasticBasicAuthConnectionBuild(t *testing.T) {
	uri := "http://user:password@localhost.local:9200/gor"
	expectedHost := "http://localhost.local"
	expectedApiPort := "9200"
	expectedIndex := "gor"

	err, host, apiPort, index := parseURI(uri)

	if nil != err {
		t.Fatalf("Expected err %s but got %s", nil, err)
	}
	if expectedHost != host {
		t.Fatalf("Expected host %s but got %s", expectedHost, host)
	}
	if expectedApiPort != apiPort {
		t.Fatalf("Expected port %s but got %s", expectedApiPort, apiPort)
	}
	if expectedIndex != index {
		t.Fatalf("Expected index %s but got %s", expectedIndex, index)
	}
}
