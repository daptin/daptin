package apiblueprint

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPI3SchemaToMapHandlesNilAndEmptySchemas(t *testing.T) {
	nilSchema := openAPI3SchemaToMap(nil)
	if nilSchema["type"] != "string" {
		t.Fatalf("expected nil schema ref to default to string, got %+v", nilSchema)
	}

	nilValueSchema := openAPI3SchemaToMap(&openapi3.SchemaRef{})
	if nilValueSchema["type"] != "string" {
		t.Fatalf("expected nil schema value to default to string, got %+v", nilValueSchema)
	}

	emptySchema := openAPI3SchemaToMap(&openapi3.SchemaRef{Value: &openapi3.Schema{}})
	if emptySchema["type"] != "object" || emptySchema["additionalProperties"] != true {
		t.Fatalf("expected empty schema fallback, got %+v", emptySchema)
	}
}

func TestOpenAPI3SchemaToMapPreservesAcyclicSchema(t *testing.T) {
	grandchildSchema := openapi3.NewObjectSchema().WithProperty("nickname", openapi3.NewStringSchema())
	childSchema := openapi3.NewObjectSchema().
		WithProperty("age", openapi3.NewIntegerSchema()).
		WithProperty("grandchild", grandchildSchema)
	schema := openapi3.NewObjectSchema().
		WithProperty("name", openapi3.NewStringSchema()).
		WithProperty("labels", openapi3.NewArraySchema().WithItems(openapi3.NewStringSchema())).
		WithProperty("child", childSchema)
	schema.Required = []string{"name"}

	mapped := openAPI3SchemaToMap(schema.NewRef())
	if mapped["type"] != "object" {
		t.Fatalf("expected object schema, got %+v", mapped)
	}
	if required := mapped["required"].([]string); !containsString(required, "name") {
		t.Fatalf("required fields were not preserved: %+v", mapped)
	}

	properties := mapped["properties"].(map[string]interface{})
	nameProperty := properties["name"].(map[string]interface{})
	if nameProperty["type"] != "string" {
		t.Fatalf("expected scalar property to be preserved, got %+v", nameProperty)
	}

	labelsProperty := properties["labels"].(map[string]interface{})
	labelItems := labelsProperty["items"].(map[string]interface{})
	if labelsProperty["type"] != "array" || labelItems["type"] != "string" {
		t.Fatalf("expected array items to be preserved, got %+v", labelsProperty)
	}

	childProperty := properties["child"].(map[string]interface{})
	childProperties := childProperty["properties"].(map[string]interface{})
	ageProperty := childProperties["age"].(map[string]interface{})
	if ageProperty["type"] != "integer" {
		t.Fatalf("expected nested acyclic property to be expanded, got %+v", childProperty)
	}
	grandchildProperty := childProperties["grandchild"].(map[string]interface{})
	grandchildProperties := grandchildProperty["properties"].(map[string]interface{})
	nicknameProperty := grandchildProperties["nickname"].(map[string]interface{})
	if nicknameProperty["type"] != "string" {
		t.Fatalf("expected deeply nested acyclic property to be expanded, got %+v", grandchildProperty)
	}
}

func TestOpenAPI3SchemaToMapCutsSelfReferentialSchema(t *testing.T) {
	schema := openapi3.NewObjectSchema()
	schema.Required = []string{"self"}
	schema.WithPropertyRef("self", schema.NewRef())

	mapped := openAPI3SchemaToMap(schema.NewRef())
	if mapped["type"] != "object" {
		t.Fatalf("top-level type was not preserved: %+v", mapped)
	}
	if required := mapped["required"].([]string); !containsString(required, "self") {
		t.Fatalf("top-level required fields were not preserved: %+v", mapped)
	}

	properties := mapped["properties"].(map[string]interface{})
	selfProperty := properties["self"].(map[string]interface{})
	if selfProperty["type"] != "object" || selfProperty["description"] != "<circular reference>" {
		t.Fatalf("expected self reference to be cut with placeholder, got %+v", selfProperty)
	}
	if _, ok := selfProperty["properties"]; ok {
		t.Fatalf("circular placeholder should not keep recursing: %+v", selfProperty)
	}
}

func TestOpenAPI3SchemaToMapCutsTwoNodeCycle(t *testing.T) {
	aSchema := openapi3.NewObjectSchema()
	bSchema := openapi3.NewObjectSchema()
	aSchema.WithPropertyRef("b", bSchema.NewRef())
	bSchema.WithPropertyRef("a", aSchema.NewRef())

	mapped := openAPI3SchemaToMap(aSchema.NewRef())
	properties := mapped["properties"].(map[string]interface{})
	bProperty := properties["b"].(map[string]interface{})
	bProperties := bProperty["properties"].(map[string]interface{})
	aProperty := bProperties["a"].(map[string]interface{})

	if aProperty["type"] != "object" || aProperty["description"] != "<circular reference>" {
		t.Fatalf("expected two-node cycle to be cut with placeholder, got %+v", aProperty)
	}
}
