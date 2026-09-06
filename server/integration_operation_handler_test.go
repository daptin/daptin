package server

import (
	"reflect"
	"testing"

	"github.com/artpar/api2go/v2"
)

func TestIntegrationOperationResultSerializesModelAttributes(t *testing.T) {
	model := api2go.NewApi2GoModelWithData("example.create.response", nil, 0, nil, map[string]interface{}{
		"id":     "example_123",
		"status": "created",
	})

	result := integrationOperationResult(api2go.Response{Res: model})
	want := map[string]interface{}{
		"id":     "example_123",
		"status": "created",
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("integration operation result = %#v, want %#v", result, want)
	}
	if _, exists := model.GetAttributes()["__type"]; !exists {
		t.Fatal("serializing the integration result mutated the response model")
	}
}

func TestIntegrationOperationResultPreservesLegacyResponderPayload(t *testing.T) {
	want := map[string]interface{}{"id": "legacy_123"}
	result := integrationOperationResult(api2go.Response{Res: want})
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("legacy integration operation result = %#v, want %#v", result, want)
	}
}
