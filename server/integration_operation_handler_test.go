package server

import (
	"reflect"
	"testing"

	"github.com/daptin/daptin/server/actionresponse"
)

func TestIntegrationOperationActionResultUsesExactResponseContract(t *testing.T) {
	result, status, err := integrationOperationActionResult("example", "create", []actionresponse.ActionResponse{
		{ResponseType: "example.create.response", Attributes: map[string]interface{}{"id": "example_123"}},
		{ResponseType: "example.create.statusCode", Attributes: 201},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"id": "example_123"}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("integration operation result = %#v, want %#v", result, want)
	}
	if status != 201 {
		t.Fatalf("integration operation status = %d, want 201", status)
	}
}

func TestIntegrationOperationActionResultRejectsIncompleteResponse(t *testing.T) {
	_, _, err := integrationOperationActionResult("example", "create", []actionresponse.ActionResponse{
		{ResponseType: "example.create.response", Attributes: map[string]interface{}{"id": "example_123"}},
	})
	if err == nil {
		t.Fatal("expected missing provider status to fail")
	}
}
