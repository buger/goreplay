package replay

import (
	"testing"
)

func TestAddress(t *testing.T) {
	settings := &ReplaySettings{
		Host: "local",
		Port: 2,
	}

	settings.Parse()

	if settings.Address != "local:2" {
		t.Error("Address not match")
	}
}

func TestForwardAddress(t *testing.T) {
	settings := &ReplaySettings{
		Host:           "local",
		Port:           2,
		ForwardAddress: "host1:1,host2:2|10",
	}

	settings.Parse()

	forward_hosts := settings.ForwardedHosts()

	if len(forward_hosts) != 2 {
		t.Error("Should have 2 forward hosts")
	}

	if forward_hosts[0].Limit != 0 && forward_hosts[0].Url != "host1:1" {
		t.Error("Host should be host1:1 with no limit")
	}

	if forward_hosts[1].Limit != 10 && forward_hosts[0].Url != "host2:2" {
		t.Error("Host should be host2:2 with 10 limit")
	}
}

func TestElasticSearchSettings(t *testing.T) {
	settings := &ReplaySettings{
		Host:            "local",
		Port:            2,
		ForwardAddress:  "host1:1,host2:2|10",
		ElastiSearchURI: "host:10/index_name",
	}

	settings.Parse()

	esp := &ESPlugin{}
	esp.Init(Settings.ElastiSearchURI)

	if esp.ApiPort != "10" {
		t.Error("Port not match")
	}

	if esp.Host != "host" {
		t.Error("Host not match")
	}

	if esp.Index != "index_name" {
		t.Error("Index not match")
	}
}
