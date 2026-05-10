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

func TestCreateIntegrationRequestBodyFromSchemaRefIgnoresMissingSchemaWithoutBody(t *testing.T) {
	body, err := CreateIntegrationRequestBodyFromSchemaRef(ModeRequest, "application/json", nil, map[string]interface{}{})
	if err != nil {
		t.Fatalf("CreateIntegrationRequestBodyFromSchemaRef returned error: %v", err)
	}
	if body != nil {
		t.Fatalf("expected nil body for optional missing schema, got %#v", body)
	}
}

func TestCreateIntegrationRequestBodyFromSchemaRefUsesExplicitBodyForMissingSchema(t *testing.T) {
	expectedBody := map[string]interface{}{"name": "free-form"}
	body, err := CreateIntegrationRequestBodyFromSchemaRef(ModeRequest, "application/json", nil, map[string]interface{}{
		"body": expectedBody,
	})
	if err != nil {
		t.Fatalf("CreateIntegrationRequestBodyFromSchemaRef returned error: %v", err)
	}
	if !reflect.DeepEqual(body, expectedBody) {
		t.Fatalf("explicit body was not used for missing schema: %#v", body)
	}
}

func TestCreateIntegrationRequestBodyHandlesRootOneOfObjectBranch(t *testing.T) {
	requestSchema := &openapi3.Schema{
		OneOf: openapi3.SchemaRefs{
			{Value: openapi3.NewObjectSchema().WithProperty("labels", openapi3.NewArraySchema().WithItems(openapi3.NewStringSchema()))},
			{Value: openapi3.NewArraySchema().WithItems(openapi3.NewStringSchema())},
		},
	}

	body, err := CreateIntegrationRequestBody(ModeRequest, "application/json", requestSchema, map[string]interface{}{
		"labels": []interface{}{"bug", "integration"},
	})
	if err != nil {
		t.Fatalf("CreateIntegrationRequestBody returned error: %v", err)
	}

	bodyMap, ok := body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected object body, got %T", body)
	}
	if !reflect.DeepEqual(bodyMap["labels"], []interface{}{"bug", "integration"}) {
		t.Fatalf("oneOf object branch did not preserve labels: %#v", bodyMap)
	}
}

func TestCreateIntegrationRequestBodyHandlesExplicitBodyForRootOneOfArrayBranch(t *testing.T) {
	requestSchema := &openapi3.Schema{
		OneOf: openapi3.SchemaRefs{
			{Value: openapi3.NewObjectSchema().WithProperty("labels", openapi3.NewArraySchema().WithItems(openapi3.NewStringSchema()))},
			{Value: openapi3.NewArraySchema().WithItems(openapi3.NewStringSchema())},
		},
	}
	expectedBody := []interface{}{"bug", "integration"}

	body, err := CreateIntegrationRequestBody(ModeRequest, "application/json", requestSchema, map[string]interface{}{
		"body": expectedBody,
	})
	if err != nil {
		t.Fatalf("CreateIntegrationRequestBody returned error: %v", err)
	}
	if !reflect.DeepEqual(body, expectedBody) {
		t.Fatalf("explicit body was not used for oneOf array branch: %#v", body)
	}
}

func TestCreateIntegrationRequestBodyHandlesAllOfMerge(t *testing.T) {
	requestSchema := &openapi3.Schema{
		AllOf: openapi3.SchemaRefs{
			{Value: openapi3.NewObjectSchema().WithProperty("title", openapi3.NewStringSchema())},
			{Value: openapi3.NewObjectSchema().WithProperty("head", openapi3.NewStringSchema())},
		},
	}

	body, err := CreateIntegrationRequestBody(ModeRequest, "application/json", requestSchema, map[string]interface{}{
		"title": "Add integration support",
		"head":  "feature/openapi-composition",
	})
	if err != nil {
		t.Fatalf("CreateIntegrationRequestBody returned error: %v", err)
	}

	bodyMap, ok := body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected object body, got %T", body)
	}
	expected := map[string]interface{}{
		"title": "Add integration support",
		"head":  "feature/openapi-composition",
	}
	if !reflect.DeepEqual(bodyMap, expected) {
		t.Fatalf("allOf merge did not preserve fields: got %#v want %#v", bodyMap, expected)
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
