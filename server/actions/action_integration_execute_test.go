package actions

import (
	"reflect"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestCreateIntegrationRequestBodyUsesDiscoveryInputShape(t *testing.T) {
	requestSchema := asanaTaskRequestSchema()

	expectedData := map[string]interface{}{
		"name":     "Daptin integration request body repro",
		"notes":    "Created from provider-scoped integration endpoint repro",
		"projects": []interface{}{"1214661437122825"},
	}

	body, err := CreateIntegrationRequestBody(ModeRequest, "application/json", requestSchema, map[string]interface{}{
		"oauth_token_id": "token-ref",
		"data":           expectedData,
	})
	if err != nil {
		t.Fatalf("CreateIntegrationRequestBody returned error: %v", err)
	}

	bodyMap, ok := body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected object body, got %T", body)
	}
	if !reflect.DeepEqual(bodyMap["data"], expectedData) {
		t.Fatalf("request body did not preserve documented data field: %#v", bodyMap)
	}
	if _, ok := bodyMap["oauth_token_id"]; ok {
		t.Fatalf("auth selector leaked into provider request body: %#v", bodyMap)
	}
}

func TestCreateIntegrationRequestBodyFallsBackToLegacyRequestPrefix(t *testing.T) {
	requestSchema := asanaTaskRequestSchema()

	expectedData := map[string]interface{}{"name": "legacy caller"}

	body, err := CreateIntegrationRequestBody(ModeRequest, "application/json", requestSchema, map[string]interface{}{
		"request.data": expectedData,
	})
	if err != nil {
		t.Fatalf("CreateIntegrationRequestBody returned error: %v", err)
	}

	bodyMap, ok := body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected object body, got %T", body)
	}
	if !reflect.DeepEqual(bodyMap["data"], expectedData) {
		t.Fatalf("request-prefix fallback did not preserve data field: %#v", bodyMap)
	}
}

func TestCreateIntegrationRequestBodyUsesExplicitBodyForFreeFormRoot(t *testing.T) {
	body, err := CreateIntegrationRequestBody(ModeRequest, "application/json", openapi3.NewObjectSchema(), map[string]interface{}{
		"oauth_token_id": "token-ref",
		"body": map[string]interface{}{
			"name": "free-form body",
		},
	})
	if err != nil {
		t.Fatalf("CreateIntegrationRequestBody returned error: %v", err)
	}

	bodyMap, ok := body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected object body, got %T", body)
	}
	if bodyMap["name"] != "free-form body" {
		t.Fatalf("free-form body was not used: %#v", bodyMap)
	}
	if _, ok := bodyMap["oauth_token_id"]; ok {
		t.Fatalf("auth selector leaked into free-form provider request body: %#v", bodyMap)
	}
}

func asanaTaskRequestSchema() *openapi3.Schema {
	return &openapi3.Schema{
		Type: "object",
		Properties: map[string]*openapi3.SchemaRef{
			"data": {
				Value: &openapi3.Schema{
					Type: "object",
				},
			},
		},
		Required: []string{"data"},
	}
}
