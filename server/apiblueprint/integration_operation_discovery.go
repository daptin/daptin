package apiblueprint

import (
	stdjson "encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/advance512/yaml"
	"github.com/daptin/daptin/server/resource"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/iancoleman/strcase"
	log "github.com/sirupsen/logrus"
)

type IntegrationOperationsDocument struct {
	Provider   string                        `json:"provider"`
	Auth       IntegrationOperationAuth      `json:"auth"`
	Operations []IntegrationOperationSummary `json:"operations"`
}

type IntegrationOperationSummary struct {
	OperationID string                   `json:"operation_id"`
	Method      string                   `json:"method"`
	Path        string                   `json:"path"`
	Summary     string                   `json:"summary,omitempty"`
	Description string                   `json:"description,omitempty"`
	Auth        IntegrationOperationAuth `json:"auth"`
}

type IntegrationOperationDetail struct {
	Provider       string                            `json:"provider"`
	OperationID    string                            `json:"operation_id"`
	Method         string                            `json:"method"`
	Path           string                            `json:"path"`
	Summary        string                            `json:"summary,omitempty"`
	Description    string                            `json:"description,omitempty"`
	Auth           IntegrationOperationAuth          `json:"auth"`
	Inputs         []IntegrationOperationInput       `json:"inputs"`
	RequestBody    *IntegrationOperationRequestBody  `json:"request_body,omitempty"`
	Responses      []IntegrationOperationResponse    `json:"responses"`
	InputSchema    map[string]interface{}            `json:"input_schema"`
	ResponseSchema map[string]interface{}            `json:"response_schema"`
	Extensions     map[string]map[string]interface{} `json:"extensions,omitempty"`
}

type IntegrationOperationAuth struct {
	Type           string `json:"type"`
	ExecutionField string `json:"execution_field,omitempty"`
	Required       bool   `json:"required"`
}

type IntegrationOperationInput struct {
	Name        string                 `json:"name"`
	In          string                 `json:"in"`
	Required    bool                   `json:"required"`
	Type        string                 `json:"type,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
}

type IntegrationOperationRequestBody struct {
	Required     bool                   `json:"required"`
	ContentTypes []string               `json:"content_types"`
	Schema       map[string]interface{} `json:"schema,omitempty"`
}

type IntegrationOperationResponse struct {
	Status       string                 `json:"status"`
	Description  string                 `json:"description,omitempty"`
	ContentTypes []string               `json:"content_types,omitempty"`
	Schema       map[string]interface{} `json:"schema,omitempty"`
}

type integrationOperationRecord struct {
	Method    string
	Path      string
	Operation *openapi3.Operation
}

func ListIntegrationOperations(integration resource.Integration) (*IntegrationOperationsDocument, error) {
	router, err := loadIntegrationOpenAPIRouter(integration)
	if err != nil {
		log.Warnf("Failed to load provider OpenAPI spec for operation list provider=[%s]: %v", integration.Name, err)
		return nil, err
	}

	records := sortedIntegrationOperationRecords(integration, router)
	operations := make([]IntegrationOperationSummary, 0, len(records))
	auth := integrationOperationAuth(integration)
	for _, record := range records {
		operations = append(operations, IntegrationOperationSummary{
			OperationID: record.Operation.OperationID,
			Method:      strings.ToUpper(record.Method),
			Path:        record.Path,
			Summary:     record.Operation.Summary,
			Description: record.Operation.Description,
			Auth:        auth,
		})
	}
	log.Debugf("Built integration operation list provider=[%s] count=%d", integration.Name, len(operations))
	return &IntegrationOperationsDocument{
		Provider:   integration.Name,
		Auth:       auth,
		Operations: operations,
	}, nil
}

func DescribeIntegrationOperation(integration resource.Integration, operationID string) (*IntegrationOperationDetail, error) {
	if operationID == "" {
		return nil, fmt.Errorf("operation is required")
	}
	router, err := loadIntegrationOpenAPIRouter(integration)
	if err != nil {
		log.Warnf("Failed to load provider OpenAPI spec for operation description provider=[%s] operation=[%s]: %v", integration.Name, operationID, err)
		return nil, err
	}

	for _, record := range sortedIntegrationOperationRecords(integration, router) {
		if record.Operation.OperationID != operationID {
			continue
		}
		detail := &IntegrationOperationDetail{
			Provider:       integration.Name,
			OperationID:    record.Operation.OperationID,
			Method:         strings.ToUpper(record.Method),
			Path:           record.Path,
			Summary:        record.Operation.Summary,
			Description:    record.Operation.Description,
			Auth:           integrationOperationAuth(integration),
			Inputs:         integrationOperationInputs(router, record.Operation),
			RequestBody:    integrationOperationRequestBody(record.Operation),
			Responses:      integrationOperationResponses(record.Operation),
			InputSchema:    operationInputSchema(router, record.Operation),
			ResponseSchema: integrationOperationResponseSchema(record.Operation),
			Extensions:     integrationOperationExtensions(record.Operation),
		}
		log.Tracef("Built integration operation description provider=[%s] operation=[%s] inputs=%d responses=%d",
			integration.Name, operationID, len(detail.Inputs), len(detail.Responses))
		return detail, nil
	}
	return nil, fmt.Errorf("operation [%s] was not found in integration [%s]", operationID, integration.Name)
}

func BuildIntegrationOpenAPI(integration resource.Integration) (string, error) {
	router, err := loadIntegrationOpenAPIRouter(integration)
	if err != nil {
		log.Warnf("Failed to load provider OpenAPI spec for scoped OpenAPI provider=[%s]: %v", integration.Name, err)
		return "", err
	}

	resourcesMap := map[string]map[string]interface{}{}
	typeMap := map[string]map[string]interface{}{}
	registered := addIntegrationOperationPathsForRouter(integration, router, resourcesMap, typeMap)
	log.Debugf("Built scoped integration OpenAPI provider=[%s] paths=%d", integration.Name, registered)

	apiDefinition := yaml.MapSlice{
		{Key: "openapi", Value: "3.0.0"},
		{Key: "info", Value: map[string]interface{}{
			"version":     "1.0.0",
			"title":       fmt.Sprintf("%s integration API", integration.Name),
			"description": fmt.Sprintf("Provider-scoped Daptin integration operation endpoints for [%s].", integration.Name),
		}},
		{Key: "paths", Value: resourcesMap},
		{Key: "components", Value: integrationOperationComponents(typeMap)},
	}
	out, err := yaml.Marshal(apiDefinition)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func sortedIntegrationOperationRecords(integration resource.Integration, router *openapi3.T) []integrationOperationRecord {
	if router == nil {
		return nil
	}
	seen := make(map[string]bool)
	records := make([]integrationOperationRecord, 0)
	for providerPath, pathItem := range router.Paths {
		if pathItem == nil {
			log.Warnf("Skipping nil path item in operation discovery provider=[%s] path=[%s]", integration.Name, providerPath)
			continue
		}
		for method, operation := range pathItem.Operations() {
			if operation == nil {
				log.Warnf("Skipping nil operation in operation discovery provider=[%s] method=[%s] path=[%s]", integration.Name, method, providerPath)
				continue
			}
			if operation.OperationID == "" {
				log.Debugf("Skipping provider operation without operationId in operation discovery provider=[%s] method=[%s] path=[%s]", integration.Name, method, providerPath)
				continue
			}
			if seen[operation.OperationID] {
				log.Warnf("Skipping duplicate operationId in operation discovery provider=[%s] operation=[%s]", integration.Name, operation.OperationID)
				continue
			}
			seen[operation.OperationID] = true
			records = append(records, integrationOperationRecord{Method: method, Path: providerPath, Operation: operation})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Operation.OperationID == records[j].Operation.OperationID {
			if records[i].Path == records[j].Path {
				return records[i].Method < records[j].Method
			}
			return records[i].Path < records[j].Path
		}
		return records[i].Operation.OperationID < records[j].Operation.OperationID
	})
	return records
}

func integrationOperationAuth(integration resource.Integration) IntegrationOperationAuth {
	authType := integration.AuthenticationType
	auth := IntegrationOperationAuth{Type: authType}
	switch strings.ToLower(authType) {
	case "oauth2":
		auth.ExecutionField = "oauth_token_id"
		auth.Required = true
	case "custom_credentials":
		auth.ExecutionField = "credential_id"
		auth.Required = true
	default:
		auth.Required = authType != ""
	}
	return auth
}

func integrationOperationInputs(router *openapi3.T, operation *openapi3.Operation) []IntegrationOperationInput {
	if operation == nil {
		return nil
	}
	inputs := make([]IntegrationOperationInput, 0)
	for _, parameterRef := range operation.Parameters {
		if parameterRef == nil || parameterRef.Value == nil || isIntegrationAuthParameter(router, parameterRef.Value) {
			continue
		}
		parameter := parameterRef.Value
		schema := openAPI3SchemaToMap(parameter.Schema)
		inputs = append(inputs, IntegrationOperationInput{
			Name:        parameter.Name,
			In:          parameter.In,
			Required:    parameter.Required,
			Type:        stringFromSchema(schema, "type"),
			Format:      stringFromSchema(schema, "format"),
			Description: parameter.Description,
			Schema:      schema,
		})
	}
	sort.Slice(inputs, func(i, j int) bool {
		if inputs[i].In == inputs[j].In {
			return inputs[i].Name < inputs[j].Name
		}
		return inputs[i].In < inputs[j].In
	})
	return inputs
}

func integrationOperationRequestBody(operation *openapi3.Operation) *IntegrationOperationRequestBody {
	if operation == nil || operation.RequestBody == nil || operation.RequestBody.Value == nil {
		return nil
	}
	requestBody := operation.RequestBody.Value
	contentTypes := make([]string, 0, len(requestBody.Content))
	for contentType := range requestBody.Content {
		contentTypes = append(contentTypes, contentType)
	}
	sort.Strings(contentTypes)
	return &IntegrationOperationRequestBody{
		Required:     requestBody.Required,
		ContentTypes: contentTypes,
		Schema:       requestBodyInputSchema(requestBody),
	}
}

func integrationOperationResponses(operation *openapi3.Operation) []IntegrationOperationResponse {
	if operation == nil || operation.Responses == nil {
		return nil
	}
	statuses := make([]string, 0, len(operation.Responses))
	for status := range operation.Responses {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	responses := make([]IntegrationOperationResponse, 0, len(statuses))
	for _, status := range statuses {
		responseRef := operation.Responses[status]
		if responseRef == nil || responseRef.Value == nil {
			continue
		}
		response := responseRef.Value
		contentTypes := make([]string, 0, len(response.Content))
		for contentType := range response.Content {
			contentTypes = append(contentTypes, contentType)
		}
		sort.Strings(contentTypes)
		var schema map[string]interface{}
		if content := response.Content.Get("application/json"); content != nil && content.Schema != nil {
			schema = openAPI3SchemaToMap(content.Schema)
		} else {
			for _, contentType := range contentTypes {
				content := response.Content.Get(contentType)
				if content != nil && content.Schema != nil {
					schema = openAPI3SchemaToMap(content.Schema)
					break
				}
			}
		}
		responses = append(responses, IntegrationOperationResponse{
			Status:       status,
			Description:  stringFromPointer(response.Description),
			ContentTypes: contentTypes,
			Schema:       schema,
		})
	}
	return responses
}

func integrationOperationExtensions(operation *openapi3.Operation) map[string]map[string]interface{} {
	if operation == nil || operation.Extensions == nil {
		return nil
	}
	transportType, ok := integrationOperationExtensionString(operation.Extensions, "x-daptin-transport")
	if !ok || strings.TrimSpace(transportType) == "" {
		transportType = "rest"
	}
	transportType = strings.ToLower(strings.TrimSpace(transportType))
	document, ok := integrationOperationExtensionString(operation.Extensions, "x-daptin-graphql-document")
	if transportType == "rest" && ok && strings.TrimSpace(document) != "" {
		transportType = "graphql"
	}
	if transportType == "rest" {
		return nil
	}
	metadata := map[string]interface{}{
		"type": transportType,
	}
	upstreamPath, upstreamSet := integrationOperationExtensionString(operation.Extensions, "x-daptin-upstream-path")
	if transportType == "graphql" && (!upstreamSet || strings.TrimSpace(upstreamPath) == "") {
		upstreamPath = "/graphql"
	}
	if strings.TrimSpace(upstreamPath) != "" {
		upstreamPath = strings.TrimSpace(upstreamPath)
		if upstreamPath[0] != '/' {
			upstreamPath = "/" + upstreamPath
		}
		metadata["upstream_path"] = upstreamPath
	}
	if operationName, ok := integrationOperationExtensionString(operation.Extensions, "x-daptin-graphql-operation-name"); ok && strings.TrimSpace(operationName) != "" {
		metadata["operation_name"] = strings.TrimSpace(operationName)
	}
	if grpcService, ok := integrationOperationExtensionString(operation.Extensions, "x-daptin-grpc-service"); ok && strings.TrimSpace(grpcService) != "" {
		metadata["grpc_service"] = strings.TrimSpace(grpcService)
	}
	if grpcMethod, ok := integrationOperationExtensionString(operation.Extensions, "x-daptin-grpc-method"); ok && strings.TrimSpace(grpcMethod) != "" {
		metadata["grpc_method"] = strings.Trim(strings.TrimSpace(grpcMethod), "/")
	}
	if responseSelector, ok := integrationOperationExtensionString(operation.Extensions, "x-daptin-websocket-response-selector"); ok && strings.TrimSpace(responseSelector) != "" {
		metadata["response_selector"] = strings.TrimSpace(responseSelector)
	}
	return map[string]map[string]interface{}{
		"daptin_transport": metadata,
	}
}

func integrationOperationExtensionString(extensions map[string]interface{}, key string) (string, bool) {
	value, ok := extensions[key]
	if !ok || value == nil {
		return "", false
	}
	switch typedValue := value.(type) {
	case string:
		return typedValue, true
	case stdjson.RawMessage:
		var decoded string
		if err := stdjson.Unmarshal(typedValue, &decoded); err == nil {
			return decoded, true
		}
		if stdjson.Valid(typedValue) {
			return "", false
		}
		return string(typedValue), true
	case []byte:
		var decoded string
		if err := stdjson.Unmarshal(typedValue, &decoded); err == nil {
			return decoded, true
		}
		if stdjson.Valid(typedValue) {
			return "", false
		}
		return string(typedValue), true
	default:
		return "", false
	}
}

func integrationOperationComponents(typeMap map[string]map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"schemas": typeMap,
		"responses": map[string]interface{}{
			"BadRequest":   integrationOperationErrorResponse("Bad request"),
			"Unauthorized": integrationOperationErrorResponse("Unauthorized"),
			"Forbidden":    integrationOperationErrorResponse("Forbidden"),
			"NotFound":     integrationOperationErrorResponse("Not found"),
		},
		"securitySchemes": map[string]interface{}{
			"bearerAuth": map[string]interface{}{
				"type":         "http",
				"scheme":       "bearer",
				"bearerFormat": "JWT",
			},
		},
	}
}

func integrationOperationErrorResponse(description string) map[string]interface{} {
	return map[string]interface{}{
		"description": description,
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"error": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}
}

func stringFromSchema(schema map[string]interface{}, key string) string {
	if value, ok := schema[key].(string); ok {
		return value
	}
	return ""
}

func stringFromPointer(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func IntegrationOperationExecutionPath(providerName string, operationID string) string {
	return fmt.Sprintf("/integration/%s/%s", url.PathEscape(providerName), url.PathEscape(operationID))
}

func IntegrationOperationRequestComponentName(providerName string, operationID string) string {
	return "Integration" + integrationProviderIdentifier(providerName) + strcase.ToCamel(operationID) + "RequestObject"
}

func integrationProviderIdentifier(providerName string) string {
	parts := strings.FieldsFunc(providerName, func(char rune) bool {
		return !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9'))
	})
	if len(parts) == 0 {
		return "Provider"
	}
	var builder strings.Builder
	for _, part := range parts {
		builder.WriteString(strcase.ToCamel(part))
	}
	return builder.String()
}
