package apiblueprint

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestOperationInputSchemaUsesProviderSpecFields(t *testing.T) {
	router := &openapi3.T{
		Components: openapi3.Components{
			SecuritySchemes: openapi3.SecuritySchemes{
				"api_key": &openapi3.SecuritySchemeRef{
					Value: &openapi3.SecurityScheme{
						Type: "apiKey",
						In:   "header",
						Name: "X-API-Key",
					},
				},
			},
		},
	}
	operation := &openapi3.Operation{
		Parameters: openapi3.Parameters{
			&openapi3.ParameterRef{Value: &openapi3.Parameter{
				Name:        "workspace_id",
				In:          "path",
				Required:    true,
				Description: "Workspace identifier",
				Schema:      openapi3.NewStringSchema().NewRef(),
			}},
			&openapi3.ParameterRef{Value: &openapi3.Parameter{
				Name:        "opt_fields",
				In:          "query",
				Description: "Fields to return",
				Schema:      openapi3.NewStringSchema().NewRef(),
			}},
			&openapi3.ParameterRef{Value: &openapi3.Parameter{
				Name:   "X-API-Key",
				In:     "header",
				Schema: openapi3.NewStringSchema().NewRef(),
			}},
		},
		RequestBody: &openapi3.RequestBodyRef{Value: &openapi3.RequestBody{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: namedTaskSchemaRef(),
				},
			},
		}},
	}

	schema := operationInputSchema(router, operation)
	properties := schema["properties"].(map[string]interface{})

	if _, ok := properties["workspace_id"]; !ok {
		t.Fatalf("path parameter was not included")
	}
	if _, ok := properties["opt_fields"]; !ok {
		t.Fatalf("query parameter was not included")
	}
	if _, ok := properties["name"]; !ok {
		t.Fatalf("request body property was not included")
	}
	if _, ok := properties["X-API-Key"]; ok {
		t.Fatalf("auth parameter should not be exposed as operation input")
	}

	required := schema["required"].([]string)
	if !containsString(required, "workspace_id") || !containsString(required, "name") {
		t.Fatalf("required fields were not preserved: %v", required)
	}
}

func namedTaskSchemaRef() *openapi3.SchemaRef {
	nameSchema := openapi3.NewStringSchema()
	nameSchema.Description = "Task name"
	schema := openapi3.NewObjectSchema().WithProperty("name", nameSchema)
	schema.Required = []string{"name"}
	return schema.NewRef()
}

func TestOperationInputSchemaFallsBackOnlyWithoutSpecFields(t *testing.T) {
	schema := operationInputSchema(&openapi3.T{}, &openapi3.Operation{})
	if schema["additionalProperties"] != true {
		t.Fatalf("expected free-form fallback when provider operation declares no input")
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
