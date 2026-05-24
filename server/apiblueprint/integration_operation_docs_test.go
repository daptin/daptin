package apiblueprint

import (
	"strings"
	"testing"

	"github.com/daptin/daptin/server/resource"
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

func TestListIntegrationOperationsUsesProviderSpec(t *testing.T) {
	document, err := ListIntegrationOperations(testAsanaIntegration())
	if err != nil {
		t.Fatalf("failed to list operations: %v", err)
	}
	if document.Provider != "asana.com" {
		t.Fatalf("unexpected provider: %s", document.Provider)
	}
	if document.Auth.ExecutionField != "oauth_token_id" || !document.Auth.Required {
		t.Fatalf("oauth selector was not exposed: %+v", document.Auth)
	}
	if len(document.Operations) != 1 {
		t.Fatalf("expected one operation, got %d", len(document.Operations))
	}
	operation := document.Operations[0]
	if operation.OperationID != "getTask" || operation.Method != "GET" || operation.Path != "/tasks/{task_gid}" {
		t.Fatalf("unexpected operation summary: %+v", operation)
	}
}

func TestDescribeIntegrationOperationIncludesInputAndResponseHints(t *testing.T) {
	document, err := DescribeIntegrationOperation(testAsanaIntegration(), "getTask")
	if err != nil {
		t.Fatalf("failed to describe operation: %v", err)
	}
	if len(document.Inputs) != 2 {
		t.Fatalf("expected path/query inputs only, got %+v", document.Inputs)
	}
	if !containsInput(document.Inputs, "task_gid", "path", true) {
		t.Fatalf("missing required path input: %+v", document.Inputs)
	}
	if !containsInput(document.Inputs, "opt_fields", "query", false) {
		t.Fatalf("missing optional query input: %+v", document.Inputs)
	}
	if _, ok := document.InputSchema["properties"].(map[string]interface{})["Authorization"]; ok {
		t.Fatalf("authorization header should not be exposed as operation input")
	}
	if len(document.Responses) == 0 || document.ResponseSchema["type"] != "object" {
		t.Fatalf("response hints were not preserved: %+v", document.Responses)
	}
}

func TestDescribeIntegrationOperationIncludesGraphQLExtensionMetadata(t *testing.T) {
	document, err := DescribeIntegrationOperation(testLinearGraphQLIntegration(), "listIssues")
	if err != nil {
		t.Fatalf("failed to describe operation: %v", err)
	}
	transportExtensions := document.Extensions["daptin_transport"]
	if transportExtensions == nil {
		t.Fatalf("missing GraphQL extension metadata: %+v", document.Extensions)
	}
	if transportExtensions["type"] != "graphql" || transportExtensions["upstream_path"] != "/graphql" || transportExtensions["operation_name"] != "ListIssues" {
		t.Fatalf("unexpected GraphQL extension metadata: %+v", transportExtensions)
	}
	if _, ok := transportExtensions["document"]; ok {
		t.Fatalf("GraphQL document should not be exposed in operation detail metadata")
	}
}

func TestDescribeIntegrationOperationIncludesProtocolTransportMetadata(t *testing.T) {
	document, err := DescribeIntegrationOperation(testProtocolTransportIntegration(), "Search")
	if err != nil {
		t.Fatalf("failed to describe grpc operation: %v", err)
	}
	transportExtensions := document.Extensions["daptin_transport"]
	if transportExtensions["type"] != "grpc" || transportExtensions["grpc_service"] != "grpc.testing.SearchService" || transportExtensions["grpc_method"] != "Search" {
		t.Fatalf("unexpected grpc transport metadata: %+v", transportExtensions)
	}

	document, err = DescribeIntegrationOperation(testProtocolTransportIntegration(), "listen")
	if err != nil {
		t.Fatalf("failed to describe websocket operation: %v", err)
	}
	transportExtensions = document.Extensions["daptin_transport"]
	if transportExtensions["type"] != "websocket" || transportExtensions["upstream_path"] != "/events" || transportExtensions["response_selector"] != "data" {
		t.Fatalf("unexpected websocket transport metadata: %+v", transportExtensions)
	}
}

func TestBuildIntegrationOpenAPIIsScopedToProvider(t *testing.T) {
	document, err := BuildIntegrationOpenAPI(testAsanaIntegration())
	if err != nil {
		t.Fatalf("failed to build scoped openapi: %v", err)
	}
	if !strings.Contains(document, "/integration/asana.com/getTask") {
		t.Fatalf("scoped operation path not generated:\n%s", document)
	}
	if !strings.Contains(document, "IntegrationAsanaComGetTaskRequestObject") {
		t.Fatalf("request component not generated:\n%s", document)
	}
}

func TestIntegrationOpenAPIPathGenerationRecoversFromPanic(t *testing.T) {
	integration := testAsanaIntegration()
	router, err := loadIntegrationOpenAPIRouter(integration)
	if err != nil {
		t.Fatalf("failed to load test integration: %v", err)
	}

	_, err = addIntegrationOperationPathsForRouter(integration, router, map[string]map[string]interface{}{}, nil)
	if err == nil {
		t.Fatalf("expected panic recovery error")
	}
	if !strings.Contains(err.Error(), "failed to generate provider-scoped OpenAPI paths") {
		t.Fatalf("unexpected recovery error: %v", err)
	}
}

func testLinearGraphQLIntegration() resource.Integration {
	return resource.Integration{
		Name:                  "linear.app",
		SpecificationLanguage: "openapiv3",
		SpecificationFormat:   "json",
		AuthenticationType:    "oauth2",
		Enable:                true,
		Specification: `{
  "openapi": "3.0.0",
  "info": {"title": "Linear", "version": "1.0.0"},
  "servers": [{"url": "https://api.linear.app"}],
  "paths": {
    "/issues/list": {
      "post": {
        "operationId": "listIssues",
        "x-daptin-upstream-path": "/graphql",
        "x-daptin-graphql-operation-name": "ListIssues",
        "x-daptin-graphql-document": "query ListIssues($first: Int) { issues(first: $first) { nodes { id } } }",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "first": {"type": "integer"}
                }
              }
            }
          }
        },
        "responses": {
          "200": {"description": "OK"}
        }
      }
    }
  }
}`,
	}
}

func testProtocolTransportIntegration() resource.Integration {
	return resource.Integration{
		Name:                  "protocol.example",
		SpecificationLanguage: "openapiv3",
		SpecificationFormat:   "json",
		AuthenticationType:    "custom_credentials",
		Enable:                true,
		Specification: `{
  "openapi": "3.0.0",
  "info": {"title": "Protocol", "version": "1.0.0"},
  "servers": [{"url": "http://localhost:9090"}],
  "paths": {
    "/grpc": {
      "post": {
        "operationId": "Search",
        "x-daptin-transport": "grpc",
        "x-daptin-grpc-service": "grpc.testing.SearchService",
        "x-daptin-grpc-method": "Search",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/ws": {
      "post": {
        "operationId": "listen",
        "x-daptin-transport": "websocket",
        "x-daptin-upstream-path": "/events",
        "x-daptin-websocket-response-selector": "data",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`,
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

func containsInput(values []IntegrationOperationInput, name string, in string, required bool) bool {
	for _, value := range values {
		if value.Name == name && value.In == in && value.Required == required {
			return true
		}
	}
	return false
}

func testAsanaIntegration() resource.Integration {
	return resource.Integration{
		Name:                  "asana.com",
		SpecificationLanguage: "openapiv3",
		SpecificationFormat:   "json",
		AuthenticationType:    "oauth2",
		Enable:                true,
		Specification: `{
  "openapi": "3.0.0",
  "info": {"title": "Asana", "version": "1.0.0"},
  "paths": {
    "/tasks/{task_gid}": {
      "get": {
        "operationId": "getTask",
        "summary": "Get a task",
        "parameters": [
          {"name": "task_gid", "in": "path", "required": true, "schema": {"type": "string"}, "description": "The task gid."},
          {"name": "opt_fields", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Fields to return."},
          {"name": "Authorization", "in": "header", "required": true, "schema": {"type": "string"}, "description": "Bearer token."}
        ],
        "responses": {
          "200": {
            "description": "Successfully retrieved task.",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "data": {"type": "object", "properties": {"gid": {"type": "string"}}}
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`,
	}
}
