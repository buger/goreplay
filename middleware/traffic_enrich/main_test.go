package main

import (
	"fmt"
	"testing"
)

func TestParsePayload(t *testing.T) {
	paylaod := []byte(`{"operationName":"GetNumUnseenNotifications",
	"variables":{"filters":"{\"seen\":false,\"created_at_gte\":null}"},
	"query":"query GetNumUnseenNotifications($filters: JSONString) {  search(    objectType: \"notification\"    params: {filters: $filters}    skipObjects: true  ) {    facets    __typename  }}"}`)
	p, err := GetGrapqhlPayload(paylaod)
	if err != nil {
		t.Error(err)
	}
	if p.Operation != "GetNumUnseenNotifications" {
		t.Error("Operation name is wrong.")
	}
	if !p.IsQuery {
		t.Error("This should be a query.")
	}
}

func TestParseMutation(t *testing.T) {
	payload := []byte(`{"operationName":"CreateWorkflowRequest",
	"variables":{"workflowConfigId":"1498d983-9313-425a-9043-e23c36a51cca","requestType":0},
	"query":"mutation CreateWorkflowRequest($workflowConfigId: String!, $requestType: Int) {\n  createWorkflowRequest(\n    workflowConfigId: $workflowConfigId\n    requestType: $requestType\n  ) {\n    request {\n      id\n      __typename\n    }\n    __typename\n  }\n}\n"}`)
	p, err := GetGrapqhlPayload(payload)
	if err != nil {
		t.Error(err)
	}
	if p.IsQuery {
		t.Error("This should be a mutation")
	}
}

func TestXxx(t *testing.T) {
	x := []byte("abcd")
	y := append(x[:2], []byte("dfad")...)
	fmt.Println(string(y))
}