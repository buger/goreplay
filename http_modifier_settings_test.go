package goreplay

import (
	"testing"
)

func TestHTTPHeaderFilters(t *testing.T) {
	filters := HTTPHeaderFilters{}

	err := filters.Set("Header1:^$")
	if err != nil {
		t.Error("Should not error on Header1:^$")
	}

	err = filters.Set("Header2:^:$")
	if err != nil {
		t.Error("Should not error on Header2:^:$")
	}

	// Missing colon
	err = filters.Set("Header3-^$")
	if err == nil {
		t.Error("Should error on Header2:^:$")
	}
}

func TestHTTPHashFilters(t *testing.T) {
	filters := HTTPHashFilters{}

	err := filters.Set("Header1:1/2")
	if err != nil {
		t.Error("Should support old syntax")
	}

	if filters[0].percent != 50 {
		t.Error("Wrong percentage", filters[0].percent)
	}

	err = filters.Set("Header2:1")
	if err == nil {
		t.Error("Should error on Header2 because no % symbol")
	}

	err = filters.Set("Header2:10%")
	if err != nil {
		t.Error("Should pass")
	}

	if filters[1].percent != 10 {
		t.Error("Wrong percentage", filters[1].percent)
	}
}

func TestUrlRewriteMap(t *testing.T) {
	var err error
	rewrites := URLRewriteMap{}

	if err = rewrites.Set("/v1/user/([^\\/]+)/ping:/v2/user/$1/ping"); err != nil {
		t.Error("Should set mapping", err)
	}

	if err = rewrites.Set("/v1/user/([^\\/]+)/ping"); err == nil {
		t.Error("Should not set mapping without :")
	}
}
