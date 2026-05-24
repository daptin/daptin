package actions

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
	gorillawebsocket "github.com/gorilla/websocket"
	"github.com/imroc/req"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Mode defines a mode of operation for example generation.
type Mode int

const (
	// ModeRequest is for the request body (writes to the server)
	ModeRequest Mode = iota
	// ModeResponse is for the response body (reads from the server)
	ModeResponse
)

/*
*

	Integration action performer
*/
type integrationActionPerformer struct {
	cruds            map[string]*resource.DbResource
	integration      resource.Integration
	router           *openapi3.T
	commandMap       map[string]*openapi3.Operation
	pathMap          map[string]string
	methodMap        map[string]string
	encryptionSecret []byte
}

const (
	integrationTransportREST      = "rest"
	integrationTransportGraphQL   = "graphql"
	integrationTransportGRPC      = "grpc"
	integrationTransportWebSocket = "websocket"
)

type integrationTransportConfig struct {
	Transport                 string
	UpstreamPath              string
	Timeout                   time.Duration
	GraphQLDocument           string
	GraphQLOperationName      string
	GRPCService               string
	GRPCMethod                string
	GRPCDescriptorBase64      string
	WebSocketMessageTemplate  string
	WebSocketResponseSelector string
}

type integrationTransportAuth struct {
	Arguments            []interface{}
	ProtectedHeaders     map[string]bool
	ProtectedQueryParams map[string]bool
	Headers              map[string]string
	QueryParams          map[string]interface{}
}

// Name of the action
func (d *integrationActionPerformer) Name() string {
	return d.integration.Name
}

// Perform integration api
func (d *integrationActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	operation, ok := d.commandMap[request.Method]
	method := d.methodMap[request.Method]
	path, ok := d.pathMap[request.Method]
	pathItem := d.router.Paths.Find(path)

	securitySchemaMap := d.router.Components.SecuritySchemes
	//selectedSecuritySchema := &openapi3.SecuritySchemeRef{}

	if !ok || pathItem == nil {
		return nil, nil, []error{errors.New("no such method")}
	}

	transportConfig, err := integrationTransportConfigFromOperation(operation, request.Method)
	if err != nil {
		return nil, nil, []error{err}
	}

	r := req.New()

	decryptedSpec, err := resource.Decrypt(d.encryptionSecret, d.integration.AuthenticationSpecification)

	if err != nil {
		log.Errorf("Failed to decrypted auth spec: %v", err)
	}
	authKeys := make(map[string]interface{})
	err = json.Unmarshal([]byte(decryptedSpec), &authKeys)
	resource.CheckErr(err, "Failed to unmarshal authentication specification")

	if d.router.Servers == nil || len(d.router.Servers) == 0 {
		log.Errorf("No servers found in integration spec of [%s]", d.integration.Name)
		return nil, nil, []error{errors.New("No servers found in integration spec of [" + d.integration.Name + "]")}
	}

	basePath := selectedIntegrationServerBaseURL(d.router)
	requestPath := path
	if transportConfig.UpstreamPath != "" {
		requestPath = transportConfig.UpstreamPath
	}
	url := integrationOperationURL(basePath, requestPath)

	matches, err := GetParametersNames(url)

	if err != nil {
		return nil, nil, []error{err}
	}

	for _, matc := range matches {
		value := inFieldMap[matc]
		if value == nil {
			log.Errorf("No value found for key: %s", matc)
			return nil, nil, []error{fmt.Errorf("No value found for key: %s", matc)}
		}

		url = strings.Replace(url, "{"+matc+"}", value.(string), -1)
	}

	urlValue, err := resource.EvaluateString(url, inFieldMap)
	resource.CheckErr(err, "Error while evaluating action url [%s]", url)
	url = urlValue.(string)

	var resp *req.Resp
	arguments := make([]interface{}, 0)
	authArguments := make([]interface{}, 0)
	protectedHeaders := make(map[string]bool)
	protectedQueryParams := make(map[string]bool)

	if transportConfig.Transport == integrationTransportREST && operation.RequestBody != nil {

		requestBodyRef := operation.RequestBody.Value
		requestContent := requestBodyRef.Content

		for mediaType, spec := range requestContent {
			if spec == nil {
				continue
			}
			switch mediaType {
			case "application/json":

				requestBody, err := CreateIntegrationRequestBodyFromSchemaRef(ModeRequest, mediaType, spec.Schema, inFieldMap)
				if err != nil {
					log.Errorf("Failed to create request body for calling [%v][%v]: %v", d.integration.Name, request.Method, err)
				} else if requestBody != nil {
					arguments = append(arguments, req.BodyJSON(requestBody))
				}

			case "application/x-www-form-urlencoded":
				requestBody, err := CreateIntegrationRequestBodyFromSchemaRef(ModeRequest, mediaType, spec.Schema, inFieldMap)
				if err != nil {
					log.Errorf("Failed to create request body for calling [%v][%v]: %v", d.integration.Name, request.Method, err)
				} else {
					m := strings.ToLower(method)

					if requestBody != nil {

						if m == "get" {
							arguments = append(arguments, req.Param(requestBody.(map[string]interface{})))
						} else {
							arguments = append(arguments, req.Param(requestBody.(map[string]interface{})))

						}
					}
				}

			}
		}
		//hasRequestBody = true
	}

	authDone := false
	authType := strings.ToLower(d.integration.AuthenticationType)

	if operation.Security != nil {
		secMethods := make(openapi3.SecurityRequirements, 0, len(*operation.Security)+len(d.router.Security))
		secMethods = append(secMethods, *operation.Security...)

		secMethods = append(secMethods, d.router.Security...)

		for _, security := range secMethods {

			for secName := range security {
				spec := securitySchemaMap[secName]

				done := false
				switch spec.Value.Type {

				case "oauth2":
					if authType == "oauth2" {
						var oauthAuthorizationHeader req.Header
						oauthAuthorizationHeader, done, err = d.oauth2AuthorizationHeader(request, inFieldMap, authKeys, transaction, false)
						if err != nil {
							return nil, nil, []error{err}
						}
						if done {
							authArguments = append(authArguments, oauthAuthorizationHeader)
							protectedHeaders["authorization"] = true
						}
					}

				case "http":
					if authType == "custom_credentials" {
						var headers map[string]bool
						var queries map[string]bool
						authArguments, headers, queries, done, err = d.customCredentialAuthArguments(inFieldMap, authKeys, spec.Value, transaction, false)
						if err != nil {
							return nil, nil, []error{err}
						}
						mergeStringBoolMaps(protectedHeaders, headers)
						mergeStringBoolMaps(protectedQueryParams, queries)
					}

				case "apiKey":
					if authType == "custom_credentials" {
						var headers map[string]bool
						var queries map[string]bool
						authArguments, headers, queries, done, err = d.customCredentialAuthArguments(inFieldMap, authKeys, spec.Value, transaction, false)
						if err != nil {
							return nil, nil, []error{err}
						}
						mergeStringBoolMaps(protectedHeaders, headers)
						mergeStringBoolMaps(protectedQueryParams, queries)
					}
				}

				if done {
					authDone = true
					break
				}
			}

			if authDone {
				break
			}
		}
	}

	if !authDone {
		switch authType {

		case "oauth2":
			var oauthAuthorizationHeader req.Header
			oauthAuthorizationHeader, authDone, err = d.oauth2AuthorizationHeader(request, inFieldMap, authKeys, transaction, true)
			if err != nil {
				return nil, nil, []error{err}
			}
			if authDone {
				authArguments = append(authArguments, oauthAuthorizationHeader)
				protectedHeaders["authorization"] = true
			}

		case "custom_credentials":
			var headers map[string]bool
			var queries map[string]bool
			authArguments, headers, queries, authDone, err = d.customCredentialAuthArguments(inFieldMap, authKeys, nil, transaction, true)
			if err != nil {
				return nil, nil, []error{err}
			}
			mergeStringBoolMaps(protectedHeaders, headers)
			mergeStringBoolMaps(protectedQueryParams, queries)

		default:
			return nil, nil, []error{fmt.Errorf("integration authentication_type [%s] is not supported; use oauth2 or custom_credentials", d.integration.AuthenticationType)}
		}
	}

	parameters := operation.Parameters
	for _, param := range parameters {

		if param.Value.In == "path" {
			continue
		}

		if param.Value.In == "body" {
			continue
		}
		if transportConfig.Transport == integrationTransportREST && param.Value.In == "header" {
			if protectedHeaders[strings.ToLower(param.Value.Name)] {
				continue
			}
			parameterValues := make(map[string]string)
			value, err := CreateRequestBody(ModeRequest, "application/json", param.Value.Name, param.Value.Schema.Value, inFieldMap)
			if err != nil {
				log.Errorf("Failed to create parameters for calling [%v][%v]", d.integration.Name, request.Method)
				return nil, nil, []error{err}
			}
			if value == nil {
				continue
			}
			parameterValues[param.Value.Name] = value.(string)
			arguments = append(arguments, req.Header(parameterValues))

		}

		if transportConfig.Transport == integrationTransportREST && param.Value.In == "query" {
			if protectedQueryParams[param.Value.Name] {
				continue
			}
			parameterValues := make(map[string]interface{})
			value, err := CreateRequestBody(ModeRequest, "application/x-www-form-urlencoded", param.Value.Name, param.Value.Schema.Value, inFieldMap)
			if err != nil {
				log.Errorf("Failed to create parameters for calling [%v][%v]", d.integration.Name, request.Method)
				return nil, nil, []error{err}
			}
			if value == nil {
				continue
			}
			parameterValues[param.Value.Name] = value
			arguments = append(arguments, req.QueryParam(parameterValues))
		}

	}

	arguments = append(arguments, authArguments...)

	transportAuth := integrationTransportAuthFromArguments(authArguments, protectedHeaders, protectedQueryParams)

	switch transportConfig.Transport {
	case integrationTransportGraphQL:
		graphqlBody, err := createGraphQLIntegrationRequestBody(d.router, operation, transportConfig, inFieldMap, protectedHeaders, protectedQueryParams)
		if err != nil {
			return nil, nil, []error{err}
		}
		arguments = append(arguments, req.BodyJSON(graphqlBody))
		resp, err = r.Post(url, arguments...)
		method = "post"
	case integrationTransportWebSocket:
		res, statusCode, err := executeWebSocketIntegrationTransport(url, operation, transportConfig, inFieldMap, transportAuth)
		if err != nil {
			return nil, nil, []error{err}
		}
		responder := resource.NewResponse(nil, res, statusCode, nil)
		return responder, []actionresponse.ActionResponse{
			resource.NewActionResponse(d.integration.Name+"."+request.Method+".response", res),
			resource.NewActionResponse(d.integration.Name+"."+request.Method+".statusCode", statusCode),
		}, nil
	case integrationTransportGRPC:
		res, statusCode, err := executeGRPCIntegrationTransport(basePath, operation, transportConfig, inFieldMap, transportAuth)
		if err != nil {
			return nil, nil, []error{err}
		}
		responder := resource.NewResponse(nil, res, statusCode, nil)
		return responder, []actionresponse.ActionResponse{
			resource.NewActionResponse(d.integration.Name+"."+request.Method+".response", res),
			resource.NewActionResponse(d.integration.Name+"."+request.Method+".statusCode", statusCode),
		}, nil
	case integrationTransportREST:
		switch strings.ToLower(method) {
		case "post":
			resp, err = r.Post(url, arguments...)

		case "get":
			resp, err = r.Get(url, arguments...)
		case "delete":
			resp, err = r.Delete(url, arguments...)
		case "patch":
			resp, err = r.Patch(url, arguments...)
		case "put":
			resp, err = r.Put(url, arguments...)
		case "options":
			resp, err = r.Options(url, arguments...)

		}
	default:
		return nil, nil, []error{fmt.Errorf("integration transport [%s] is not supported", transportConfig.Transport)}
	}
	resource.CheckErr(err, "Action execution failed")
	if err != nil {
		return nil, nil, []error{err}
	}

	var res map[string]interface{}
	err = resp.ToJSON(&res)
	resource.CheckErr(err, "[432] Failed to read value as json: [%s]", resp.String())
	if err != nil {
		res = map[string]interface{}{
			"body": resp.String(),
		}
		log.Printf("API Response [%s][%s]: %v %v", method, url, resp.Response().Status, resp.String())
		return nil, nil, []error{err}
	}
	responder := resource.NewResponse(nil, res, resp.Response().StatusCode, nil)
	return responder, []actionresponse.ActionResponse{
		resource.NewActionResponse(d.integration.Name+"."+request.Method+".response", res),
		resource.NewActionResponse(d.integration.Name+"."+request.Method+".statusCode", resp.Response().StatusCode),
	}, nil
}

func integrationTransportConfigFromOperation(operation *openapi3.Operation, operationID string) (integrationTransportConfig, error) {
	config := integrationTransportConfig{
		Transport: integrationTransportREST,
		Timeout:   10 * time.Second,
	}
	if operation == nil {
		return config, nil
	}
	transport, ok, err := openAPIExtensionString(operation.Extensions, "x-daptin-transport")
	if err != nil {
		return config, err
	}
	if ok && strings.TrimSpace(transport) != "" {
		config.Transport = strings.ToLower(strings.TrimSpace(transport))
	}

	upstreamPath, _, err := openAPIExtensionString(operation.Extensions, "x-daptin-upstream-path")
	if err != nil {
		return config, err
	}
	config.UpstreamPath = normalizeIntegrationUpstreamPath(upstreamPath)

	timeoutMillis, _, err := openAPIExtensionInt(operation.Extensions, "x-daptin-timeout-ms")
	if err != nil {
		return config, err
	}
	if timeoutMillis > 0 {
		config.Timeout = time.Duration(timeoutMillis) * time.Millisecond
	}

	config.GraphQLDocument, _, err = openAPIExtensionString(operation.Extensions, "x-daptin-graphql-document")
	if err != nil {
		return config, err
	}
	config.GraphQLDocument = strings.TrimSpace(config.GraphQLDocument)
	config.GraphQLOperationName, _, err = openAPIExtensionString(operation.Extensions, "x-daptin-graphql-operation-name")
	if err != nil {
		return config, err
	}
	config.GraphQLOperationName = strings.TrimSpace(config.GraphQLOperationName)
	if config.Transport == integrationTransportREST && config.GraphQLDocument != "" {
		config.Transport = integrationTransportGraphQL
	}
	if config.Transport == integrationTransportGraphQL && config.UpstreamPath == "" {
		config.UpstreamPath = "/graphql"
	}

	config.GRPCService, _, err = openAPIExtensionString(operation.Extensions, "x-daptin-grpc-service")
	if err != nil {
		return config, err
	}
	config.GRPCService = strings.TrimSpace(config.GRPCService)
	config.GRPCMethod, _, err = openAPIExtensionString(operation.Extensions, "x-daptin-grpc-method")
	if err != nil {
		return config, err
	}
	config.GRPCMethod = strings.Trim(strings.TrimSpace(config.GRPCMethod), "/")
	if config.GRPCMethod == "" {
		config.GRPCMethod = operationID
	}
	config.GRPCDescriptorBase64, _, err = openAPIExtensionString(operation.Extensions, "x-daptin-grpc-descriptor-base64")
	if err != nil {
		return config, err
	}
	config.GRPCDescriptorBase64 = strings.TrimSpace(config.GRPCDescriptorBase64)

	config.WebSocketMessageTemplate, _, err = openAPIExtensionString(operation.Extensions, "x-daptin-websocket-message-template")
	if err != nil {
		return config, err
	}
	config.WebSocketResponseSelector, _, err = openAPIExtensionString(operation.Extensions, "x-daptin-websocket-response-selector")
	if err != nil {
		return config, err
	}
	config.WebSocketResponseSelector = strings.TrimSpace(config.WebSocketResponseSelector)

	switch config.Transport {
	case integrationTransportREST, integrationTransportGraphQL, integrationTransportGRPC, integrationTransportWebSocket:
	default:
		return config, fmt.Errorf("integration transport [%s] is not supported", config.Transport)
	}
	if config.Transport == integrationTransportGraphQL && config.GraphQLDocument == "" {
		return config, errors.New("x-daptin-graphql-document is required for graphql integration transport")
	}
	if config.Transport == integrationTransportGRPC && config.GRPCService == "" {
		return config, errors.New("x-daptin-grpc-service is required for grpc integration transport")
	}
	return config, nil
}

func normalizeIntegrationUpstreamPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path[0] != '/' {
		path = "/" + path
	}
	return path
}

func openAPIExtensionString(extensions map[string]interface{}, key string) (string, bool, error) {
	if len(extensions) == 0 {
		return "", false, nil
	}
	value, ok := extensions[key]
	if !ok || value == nil {
		return "", false, nil
	}
	switch typedValue := value.(type) {
	case string:
		return typedValue, true, nil
	case stdjson.RawMessage:
		return stringFromOpenAPIExtensionBytes(key, typedValue)
	case []byte:
		return stringFromOpenAPIExtensionBytes(key, typedValue)
	default:
		return "", true, fmt.Errorf("%s must be a string", key)
	}
}

func openAPIExtensionInt(extensions map[string]interface{}, key string) (int64, bool, error) {
	if len(extensions) == 0 {
		return 0, false, nil
	}
	value, ok := extensions[key]
	if !ok || value == nil {
		return 0, false, nil
	}
	switch typedValue := value.(type) {
	case int:
		return int64(typedValue), true, nil
	case int64:
		return typedValue, true, nil
	case float64:
		return int64(typedValue), true, nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typedValue), 10, 64)
		if err != nil {
			return 0, true, fmt.Errorf("%s must be an integer", key)
		}
		return parsed, true, nil
	case stdjson.RawMessage:
		return intFromOpenAPIExtensionBytes(key, typedValue)
	case []byte:
		return intFromOpenAPIExtensionBytes(key, typedValue)
	default:
		return 0, true, fmt.Errorf("%s must be an integer", key)
	}
}

func intFromOpenAPIExtensionBytes(key string, value []byte) (int64, bool, error) {
	var decoded int64
	if err := stdjson.Unmarshal(value, &decoded); err == nil {
		return decoded, true, nil
	}
	var decodedString string
	if err := stdjson.Unmarshal(value, &decodedString); err == nil {
		parsed, parseErr := strconv.ParseInt(strings.TrimSpace(decodedString), 10, 64)
		if parseErr != nil {
			return 0, true, fmt.Errorf("%s must be an integer", key)
		}
		return parsed, true, nil
	}
	return 0, true, fmt.Errorf("%s must be an integer", key)
}

func stringFromOpenAPIExtensionBytes(key string, value []byte) (string, bool, error) {
	var decoded string
	if err := stdjson.Unmarshal(value, &decoded); err == nil {
		return decoded, true, nil
	}
	if stdjson.Valid(value) {
		return "", true, fmt.Errorf("%s must be a string", key)
	}
	if len(value) == 0 {
		return "", true, nil
	}
	if value[0] == '"' || value[0] == '{' || value[0] == '[' {
		return "", true, fmt.Errorf("%s must be a string", key)
	}
	return string(value), true, nil
}

func selectedIntegrationServerBaseURL(router *openapi3.T) string {
	basePath := router.Servers[0].URL
	// prefer https path over http paths
	if len(router.Servers) > 1 && strings.Index(basePath, "https://") != 0 {
		for _, apiServer := range router.Servers[1:] {
			if strings.HasPrefix(apiServer.URL, "https://") {
				basePath = apiServer.URL
			}
		}
	}
	return strings.TrimSuffix(basePath, "/")
}

func integrationOperationURL(basePath string, requestPath string) string {
	if requestPath == "" {
		return basePath
	}
	if requestPath[0] != '/' {
		requestPath = "/" + requestPath
	}
	return basePath + requestPath
}

func createGraphQLIntegrationRequestBody(
	router *openapi3.T,
	operation *openapi3.Operation,
	transportConfig integrationTransportConfig,
	inFieldMap map[string]interface{},
	protectedHeaders map[string]bool,
	protectedQueryParams map[string]bool,
) (map[string]interface{}, error) {
	variables, err := createIntegrationOperationVariables(router, operation, inFieldMap, protectedHeaders, protectedQueryParams)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{
		"query":     transportConfig.GraphQLDocument,
		"variables": variables,
	}
	if transportConfig.GraphQLOperationName != "" {
		body["operationName"] = transportConfig.GraphQLOperationName
	}
	return body, nil
}

func createIntegrationOperationVariables(
	router *openapi3.T,
	operation *openapi3.Operation,
	inFieldMap map[string]interface{},
	protectedHeaders map[string]bool,
	protectedQueryParams map[string]bool,
) (map[string]interface{}, error) {
	variables := make(map[string]interface{})
	if operation == nil {
		return variables, nil
	}
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		if requestBody, err := createVariablesFromIntegrationRequestBody(operation.RequestBody.Value, inFieldMap); err != nil {
			return nil, err
		} else if requestBody != nil {
			if requestBodyMap, ok := requestBody.(map[string]interface{}); ok {
				for key, value := range requestBodyMap {
					if !isIntegrationRuntimeInputKey(key) {
						variables[key] = value
					}
				}
			} else {
				variables["body"] = requestBody
			}
		}
	}
	for _, parameterRef := range operation.Parameters {
		if parameterRef == nil || parameterRef.Value == nil {
			continue
		}
		parameter := parameterRef.Value
		if isIntegrationRuntimeParameter(router, parameter, protectedHeaders, protectedQueryParams) {
			continue
		}
		value, ok := inFieldMap[parameter.Name]
		if !ok || value == nil {
			continue
		}
		if parameter.Schema != nil && parameter.Schema.Value != nil {
			convertedValue, err := CreateRequestBody(ModeRequest, "application/json", parameter.Name, parameter.Schema.Value, inFieldMap)
			if err != nil {
				return nil, err
			}
			if convertedValue != nil {
				value = convertedValue
			}
		}
		variables[parameter.Name] = value
	}
	return variables, nil
}

func createVariablesFromIntegrationRequestBody(requestBody *openapi3.RequestBody, inFieldMap map[string]interface{}) (interface{}, error) {
	if requestBody == nil {
		return nil, nil
	}
	if jsonMedia := requestBody.Content.Get("application/json"); jsonMedia != nil {
		return CreateIntegrationRequestBodyFromSchemaRef(ModeRequest, "application/json", jsonMedia.Schema, inFieldMap)
	}
	for mediaType, media := range requestBody.Content {
		if media == nil {
			continue
		}
		return CreateIntegrationRequestBodyFromSchemaRef(ModeRequest, mediaType, media.Schema, inFieldMap)
	}
	return nil, nil
}

func isIntegrationRuntimeParameter(
	router *openapi3.T,
	parameter *openapi3.Parameter,
	protectedHeaders map[string]bool,
	protectedQueryParams map[string]bool,
) bool {
	if parameter == nil || isIntegrationRuntimeInputKey(parameter.Name) {
		return true
	}
	switch strings.ToLower(parameter.In) {
	case "header":
		if protectedHeaders[strings.ToLower(parameter.Name)] || strings.EqualFold(parameter.Name, "authorization") {
			return true
		}
	case "query":
		if protectedQueryParams[parameter.Name] {
			return true
		}
	}
	if router == nil || router.Components.SecuritySchemes == nil {
		return false
	}
	for _, securityRef := range router.Components.SecuritySchemes {
		if securityRef == nil || securityRef.Value == nil {
			continue
		}
		scheme := securityRef.Value
		if scheme.Type == "apiKey" && strings.EqualFold(scheme.In, parameter.In) && strings.EqualFold(scheme.Name, parameter.Name) {
			return true
		}
	}
	return false
}

func isIntegrationRuntimeInputKey(key string) bool {
	switch key {
	case "oauth_token_id", "credential_id", "sessionUser", "requestSessionUser", "httpRequest", "httpRequestHeaders":
		return true
	default:
		return false
	}
}

func integrationTransportAuthFromArguments(arguments []interface{}, protectedHeaders map[string]bool, protectedQueryParams map[string]bool) integrationTransportAuth {
	auth := integrationTransportAuth{
		Arguments:            arguments,
		ProtectedHeaders:     protectedHeaders,
		ProtectedQueryParams: protectedQueryParams,
		Headers:              make(map[string]string),
		QueryParams:          make(map[string]interface{}),
	}
	for _, argument := range arguments {
		switch typedArgument := argument.(type) {
		case req.Header:
			for key, value := range typedArgument {
				auth.Headers[key] = value
			}
		case req.QueryParam:
			for key, value := range typedArgument {
				auth.QueryParams[key] = value
			}
		}
	}
	return auth
}

func executeWebSocketIntegrationTransport(
	requestURL string,
	operation *openapi3.Operation,
	transportConfig integrationTransportConfig,
	inFieldMap map[string]interface{},
	auth integrationTransportAuth,
) (map[string]interface{}, int, error) {
	websocketURL, err := websocketURLFromIntegrationURL(requestURL, auth.QueryParams)
	if err != nil {
		return nil, 0, err
	}
	header := http.Header{}
	for key, value := range auth.Headers {
		header.Set(key, value)
	}
	dialer := gorillawebsocket.Dialer{HandshakeTimeout: transportConfig.Timeout}
	conn, _, err := dialer.Dial(websocketURL, header)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close()

	message, err := createWebSocketIntegrationMessage(operation, transportConfig, inFieldMap, auth)
	if err != nil {
		return nil, 0, err
	}
	if err := conn.SetWriteDeadline(time.Now().Add(transportConfig.Timeout)); err != nil {
		return nil, 0, err
	}
	if err := conn.WriteJSON(message); err != nil {
		return nil, 0, err
	}
	if err := conn.SetReadDeadline(time.Now().Add(transportConfig.Timeout)); err != nil {
		return nil, 0, err
	}
	_, responseBytes, err := conn.ReadMessage()
	if err != nil {
		return nil, 0, err
	}
	response := map[string]interface{}{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		response["body"] = string(responseBytes)
	}
	if transportConfig.WebSocketResponseSelector != "" {
		selected, ok := selectIntegrationResponseValue(response, transportConfig.WebSocketResponseSelector)
		if !ok {
			return nil, 0, fmt.Errorf("websocket response selector [%s] did not match response", transportConfig.WebSocketResponseSelector)
		}
		response = map[string]interface{}{"value": selected}
	}
	return response, http.StatusOK, nil
}

func websocketURLFromIntegrationURL(requestURL string, queryParams map[string]interface{}) (string, error) {
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return "", err
	}
	switch parsedURL.Scheme {
	case "http":
		parsedURL.Scheme = "ws"
	case "https":
		parsedURL.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("websocket integration requires http, https, ws, or wss server URL, got [%s]", parsedURL.Scheme)
	}
	query := parsedURL.Query()
	for key, value := range queryParams {
		query.Set(key, fmt.Sprintf("%v", value))
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func createWebSocketIntegrationMessage(operation *openapi3.Operation, transportConfig integrationTransportConfig, inFieldMap map[string]interface{}, auth integrationTransportAuth) (interface{}, error) {
	variables, err := createIntegrationOperationVariables(nil, operation, inFieldMap, auth.ProtectedHeaders, auth.ProtectedQueryParams)
	if err != nil {
		return nil, err
	}
	template := strings.TrimSpace(transportConfig.WebSocketMessageTemplate)
	if template == "" {
		return variables, nil
	}
	evaluated, err := resource.EvaluateString(template, variables)
	if err != nil {
		return nil, err
	}
	if evaluatedString, ok := evaluated.(string); ok {
		var jsonMessage interface{}
		if err := json.Unmarshal([]byte(evaluatedString), &jsonMessage); err == nil {
			return jsonMessage, nil
		}
		return evaluatedString, nil
	}
	return evaluated, nil
}

func selectIntegrationResponseValue(response map[string]interface{}, selector string) (interface{}, bool) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return response, true
	}
	var current interface{} = response
	for _, part := range strings.Split(selector, ".") {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = currentMap[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func executeGRPCIntegrationTransport(
	basePath string,
	operation *openapi3.Operation,
	transportConfig integrationTransportConfig,
	inFieldMap map[string]interface{},
	auth integrationTransportAuth,
) (map[string]interface{}, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), transportConfig.Timeout)
	defer cancel()

	target, secure, err := grpcTargetFromIntegrationBaseURL(basePath)
	if err != nil {
		return nil, 0, err
	}
	dialOptions := []grpc.DialOption{}
	if secure {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.DialContext(ctx, target, dialOptions...)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close()

	if len(auth.Headers) > 0 {
		md := metadata.MD{}
		for key, value := range auth.Headers {
			md.Append(strings.ToLower(key), value)
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	files, err := grpcDescriptorFiles(ctx, conn, transportConfig)
	if err != nil {
		return nil, 0, err
	}
	methodDescriptor, err := grpcMethodDescriptor(files, transportConfig)
	if err != nil {
		return nil, 0, err
	}
	variables, err := createIntegrationOperationVariables(nil, operation, inFieldMap, auth.ProtectedHeaders, auth.ProtectedQueryParams)
	if err != nil {
		return nil, 0, err
	}
	inputMessage := dynamicpb.NewMessage(methodDescriptor.Input())
	inputJSON, err := json.Marshal(variables)
	if err != nil {
		return nil, 0, err
	}
	if err := protojson.Unmarshal(inputJSON, inputMessage); err != nil {
		return nil, 0, err
	}
	outputMessage := dynamicpb.NewMessage(methodDescriptor.Output())
	fullMethodName := fmt.Sprintf("/%s/%s", transportConfig.GRPCService, methodDescriptor.Name())
	if err := conn.Invoke(ctx, fullMethodName, inputMessage, outputMessage); err != nil {
		return nil, 0, err
	}
	responseJSON, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(outputMessage)
	if err != nil {
		return nil, 0, err
	}
	response := map[string]interface{}{}
	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, 0, err
	}
	return response, http.StatusOK, nil
}

func grpcTargetFromIntegrationBaseURL(basePath string) (string, bool, error) {
	parsedURL, err := url.Parse(basePath)
	if err != nil {
		return "", false, err
	}
	if parsedURL.Scheme == "" {
		return basePath, false, nil
	}
	switch parsedURL.Scheme {
	case "http":
		return parsedURL.Host, false, nil
	case "https":
		return parsedURL.Host, true, nil
	default:
		return "", false, fmt.Errorf("grpc integration requires http or https server URL, got [%s]", parsedURL.Scheme)
	}
}

func grpcDescriptorFiles(ctx context.Context, conn *grpc.ClientConn, transportConfig integrationTransportConfig) (*protoregistryFiles, error) {
	if transportConfig.GRPCDescriptorBase64 != "" {
		descriptorBytes, err := base64.StdEncoding.DecodeString(transportConfig.GRPCDescriptorBase64)
		if err != nil {
			return nil, err
		}
		descriptorSet := &descriptorpb.FileDescriptorSet{}
		if err := proto.Unmarshal(descriptorBytes, descriptorSet); err != nil {
			return nil, err
		}
		files, err := protodesc.NewFiles(descriptorSet)
		if err != nil {
			return nil, err
		}
		return &protoregistryFiles{files: files}, nil
	}
	descriptorSet, err := grpcDescriptorSetFromReflection(ctx, conn, transportConfig.GRPCService)
	if err != nil {
		return nil, err
	}
	files, err := protodesc.NewFiles(descriptorSet)
	if err != nil {
		return nil, err
	}
	return &protoregistryFiles{files: files}, nil
}

type protoregistryFiles struct {
	files *protoregistry.Files
}

func grpcMethodDescriptor(files *protoregistryFiles, transportConfig integrationTransportConfig) (protoreflect.MethodDescriptor, error) {
	serviceDescriptor, err := files.files.FindDescriptorByName(protoreflect.FullName(transportConfig.GRPCService))
	if err != nil {
		return nil, err
	}
	service, ok := serviceDescriptor.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("[%s] is not a grpc service", transportConfig.GRPCService)
	}
	methodName := protoreflect.Name(transportConfig.GRPCMethod)
	methodDescriptor := service.Methods().ByName(methodName)
	if methodDescriptor == nil {
		return nil, fmt.Errorf("grpc method [%s] was not found in service [%s]", transportConfig.GRPCMethod, transportConfig.GRPCService)
	}
	if methodDescriptor.IsStreamingClient() || methodDescriptor.IsStreamingServer() {
		return nil, fmt.Errorf("grpc streaming method [%s] is not supported", transportConfig.GRPCMethod)
	}
	return methodDescriptor, nil
}

func grpcDescriptorSetFromReflection(ctx context.Context, conn *grpc.ClientConn, symbol string) (*descriptorpb.FileDescriptorSet, error) {
	client := reflectionv1alpha.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(&reflectionv1alpha.ServerReflectionRequest{
		MessageRequest: &reflectionv1alpha.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: symbol,
		},
	}); err != nil {
		return nil, err
	}
	response, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	fileResponse := response.GetFileDescriptorResponse()
	if fileResponse == nil {
		return nil, fmt.Errorf("grpc reflection did not return descriptors for [%s]", symbol)
	}
	descriptorSet := &descriptorpb.FileDescriptorSet{}
	for _, fileBytes := range fileResponse.FileDescriptorProto {
		fileDescriptor := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(fileBytes, fileDescriptor); err != nil {
			return nil, err
		}
		descriptorSet.File = append(descriptorSet.File, fileDescriptor)
	}
	return descriptorSet, nil
}

func (d *integrationActionPerformer) oauth2AuthorizationHeader(
	request actionresponse.Outcome,
	inFieldMap map[string]interface{},
	authKeys map[string]interface{},
	transaction *sqlx.Tx,
	requireToken bool,
) (req.Header, bool, error) {
	oauthTokenId := daptinid.InterfaceToDIR(inFieldMap["oauth_token_id"])
	if oauthTokenId != daptinid.NullReferenceId {
		oauthConnectId := daptinid.InterfaceToDIR(authKeys["oauth_connect_id"])
		sessionUser := integrationExecutionSessionUser(inFieldMap)
		if sessionUser == nil {
			return nil, false, errors.New("oauth2 integration execution requires an authenticated user")
		}
		err := d.cruds["oauth_token"].ValidateOAuthTokenForIntegrationExecution(oauthTokenId, sessionUser.UserId, oauthConnectId, transaction)
		if err != nil {
			return nil, false, err
		}
		return d.oauth2HeaderForTokenReference(oauthTokenId, transaction)
	}

	if requireToken {
		return nil, false, errors.New("oauth_token_id is required for oauth2 integration execution")
	}

	return nil, false, nil
}

func (d *integrationActionPerformer) oauth2HeaderForTokenReference(oauthTokenId daptinid.DaptinReferenceId, transaction *sqlx.Tx) (req.Header, bool, error) {
	oauthToken, _, err := d.cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId, transaction)
	if err != nil {
		return nil, false, err
	}

	return req.Header{
		"Authorization": "Bearer " + oauthToken.AccessToken,
	}, true, nil
}

func (d *integrationActionPerformer) customCredentialAuthArguments(
	inFieldMap map[string]interface{},
	authKeys map[string]interface{},
	securityScheme *openapi3.SecurityScheme,
	transaction *sqlx.Tx,
	requireCredential bool,
) ([]interface{}, map[string]bool, map[string]bool, bool, error) {
	credentialId := daptinid.InterfaceToDIR(inFieldMap["credential_id"])
	if credentialId == daptinid.NullReferenceId {
		if requireCredential {
			return nil, nil, nil, false, errors.New("credential_id is required for custom credential integration execution")
		}
		return nil, nil, nil, false, nil
	}

	sessionUser := integrationExecutionSessionUser(inFieldMap)
	if sessionUser == nil {
		return nil, nil, nil, false, errors.New("custom credential integration execution requires an authenticated user")
	}

	credential, err := d.cruds["credential"].GetCredentialByReferenceIdForIntegrationExecution(credentialId, sessionUser, transaction)
	if err != nil {
		return nil, nil, nil, false, err
	}

	protectedHeaders := make(map[string]bool)
	protectedQueryParams := make(map[string]bool)

	scheme := strings.ToLower(stringValue(authKeys["scheme"]))
	if scheme == "" && securityScheme != nil && securityScheme.Type == "http" {
		scheme = strings.ToLower(securityScheme.Scheme)
	}

	switch scheme {
	case "basic":
		usernameField := stringValue(authKeys["username_field"])
		if usernameField == "" {
			usernameField = "username"
		}
		passwordField := stringValue(authKeys["password_field"])
		if passwordField == "" {
			passwordField = "password"
		}
		username, ok := credential.DataMap[usernameField].(string)
		if !ok || username == "" {
			return nil, nil, nil, false, fmt.Errorf("credential is missing [%s]", usernameField)
		}
		password, ok := credential.DataMap[passwordField].(string)
		if !ok || password == "" {
			return nil, nil, nil, false, fmt.Errorf("credential is missing [%s]", passwordField)
		}
		header := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
		protectedHeaders["authorization"] = true
		return []interface{}{req.Header{"Authorization": "Basic " + header}}, protectedHeaders, protectedQueryParams, true, nil

	case "bearer":
		tokenField := stringValue(authKeys["token_field"])
		if tokenField == "" {
			tokenField = "token"
		}
		token, ok := credential.DataMap[tokenField].(string)
		if !ok || token == "" {
			return nil, nil, nil, false, fmt.Errorf("credential is missing [%s]", tokenField)
		}
		protectedHeaders["authorization"] = true
		return []interface{}{req.Header{"Authorization": "Bearer " + token}}, protectedHeaders, protectedQueryParams, true, nil
	}

	keyLocation := strings.ToLower(stringValue(authKeys["in"]))
	keyName := stringValue(authKeys["name"])
	if securityScheme != nil && securityScheme.Type == "apiKey" {
		if keyLocation == "" {
			keyLocation = strings.ToLower(securityScheme.In)
		}
		if keyName == "" {
			keyName = securityScheme.Name
		}
	}
	valueField := stringValue(authKeys["value_field"])
	if valueField == "" {
		valueField = keyName
	}
	if keyLocation == "" || keyName == "" || valueField == "" {
		return nil, nil, nil, false, errors.New("custom credential authentication_specification must define either scheme or api key placement")
	}

	value, ok := credential.DataMap[valueField].(string)
	if !ok || value == "" {
		return nil, nil, nil, false, fmt.Errorf("credential is missing [%s]", valueField)
	}

	switch keyLocation {
	case "header":
		protectedHeaders[strings.ToLower(keyName)] = true
		return []interface{}{req.Header{keyName: value}}, protectedHeaders, protectedQueryParams, true, nil
	case "query":
		protectedQueryParams[keyName] = true
		return []interface{}{req.QueryParam{keyName: value}}, protectedHeaders, protectedQueryParams, true, nil
	case "cookie":
		protectedHeaders["cookie"] = true
		return []interface{}{req.Header{"Cookie": fmt.Sprintf("%s=%s", keyName, value)}}, protectedHeaders, protectedQueryParams, true, nil
	}

	return nil, nil, nil, false, fmt.Errorf("unsupported custom credential location [%s]", keyLocation)
}

func stringValue(val interface{}) string {
	str, _ := val.(string)
	return str
}

func integrationExecutionSessionUser(inFieldMap map[string]interface{}) *auth.SessionUser {
	if sessionUser, ok := inFieldMap["requestSessionUser"].(*auth.SessionUser); ok && sessionUser != nil {
		return sessionUser
	}
	sessionUser, _ := inFieldMap["sessionUser"].(*auth.SessionUser)
	return sessionUser
}

func mergeStringBoolMaps(target map[string]bool, source map[string]bool) {
	for key, val := range source {
		target[key] = val
	}
}

func GetParametersNames(s string) ([]string, error) {
	ret := make([]string, 0)
	templateVar, err := regexp.Compile(`\{([^}]+)\}`)
	if err != nil {
		return ret, err
	}

	matches := templateVar.FindAllStringSubmatch(s, -1)

	for _, match := range matches {
		ret = append(ret, match[1])
	}
	return ret, nil
}

func CreateIntegrationRequestBody(mode Mode, mediaType string, schema *openapi3.Schema, values map[string]interface{}) (interface{}, error) {
	body, err := CreateRequestBody(mode, mediaType, "", schema, values)
	if err != nil {
		return nil, err
	}
	if body != nil {
		return body, nil
	}
	legacyValues := stripRequestPrefix(values)
	if len(legacyValues) == 0 {
		return nil, nil
	}
	return CreateRequestBody(mode, mediaType, "", schema, legacyValues)
}

func CreateIntegrationRequestBodyFromSchemaRef(mode Mode, mediaType string, schemaRef *openapi3.SchemaRef, values map[string]interface{}) (interface{}, error) {
	if schemaRef == nil {
		if body, ok := values["body"]; ok && body != nil {
			return body, nil
		}
		return nil, nil
	}
	if schemaRef.Value == nil {
		if schemaRef.Ref != "" {
			return nil, fmt.Errorf("not a valid schema: unresolved schema ref %s", schemaRef.Ref)
		}
		if body, ok := values["body"]; ok && body != nil {
			return body, nil
		}
		return nil, nil
	}
	return CreateIntegrationRequestBody(mode, mediaType, schemaRef.Value, values)
}

func stripRequestPrefix(values map[string]interface{}) map[string]interface{} {
	legacyValues := make(map[string]interface{})
	for key, value := range values {
		if key == "request" {
			legacyValues["body"] = value
			continue
		}
		if strings.HasPrefix(key, "request.") {
			legacyValues[strings.TrimPrefix(key, "request.")] = value
		}
	}
	return legacyValues
}

// OpenAPIExample creates an example structure from an OpenAPI 3 schema
// object, which is an extended subset of JSON Schema.
// https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.1.md#schemaObject
func CreateRequestBody(mode Mode, mediaType string, name string, schema *openapi3.Schema, values map[string]interface{}) (interface{}, error) {
	if schema == nil {
		return nil, errors.New("not a valid schema: nil schema")
	}
	if name == "" {
		if body, ok := values["body"]; ok && body != nil {
			return body, nil
		}
	}

	switch {
	case schema.Type == "boolean":
		value, ok := values[name]

		if !ok {
			return nil, nil
		}

		valString, ok := value.(string)

		if ok {
			if strings.ToLower(valString) == "true" {
				return true, nil
			}
		}
		valBool, ok := value.(bool)

		if ok {
			return valBool, nil
		}

		return false, nil
	case schema.Type == "number", schema.Type == "integer":

		value := values[name]
		intValue := int64(0)
		var err error

		if value == nil {
			return value, nil
		}

		switch value.(type) {
		case string:
			intValue, err = strconv.ParseInt(value.(string), 10, 64)
			resource.CheckErr(err, "Failed to parse string value as int [%v]", value)
			return intValue, nil
		default:
			valueInt, ok := values[name].(int64)
			if ok {
				value = float64(valueInt)
			}
		}

		if schema.Type == "integer" {
			return int(value.(float64)), nil
		}

		return value, nil
	case schema.Type == "string":
		str := values[name]
		if str == nil {
			return nil, nil
		}

		example := str.(string)
		return example, nil
	case schema.Type == "array", schema.Items != nil:

		val := values[name]

		if val == nil {
			return nil, nil
			//val = []map[string]interface{}{values}
		}

		var ok bool
		var mapVal []map[string]interface{}
		mapVal, ok = val.([]map[string]interface{})
		if !ok {
			arrayVal, ok := val.([]interface{})
			if !ok {
				return []interface{}{}, errors.New(fmt.Sprintf("type not array type [%v]: %v", name, val))
			}
			mapVal = make([]map[string]interface{}, 0)

			for _, row := range arrayVal {

				switch row.(type) {
				case map[string]interface{}:
					mapVal = append(mapVal, row.(map[string]interface{}))
				default:
					mapVal = append(mapVal, map[string]interface{}{
						"": row,
					})
				}

			}
		}

		var items []interface{}

		if schema.Items != nil && schema.Items.Value != nil {

			for _, item := range mapVal {

				ex, err := CreateRequestBody(mode, mediaType, "", schema.Items.Value, item)

				if err != nil {
					return nil, errors.New(fmt.Sprintf("failed to convert item to body: [%v][%v] == %v", name, item, err))
				}

				items = append(items, ex)
			}
		}

		return items, nil
	case schema.Type == "object":
		if len(schema.Properties) == 0 {
			if name == "" {
				value := values["body"]
				if value == nil {
					return nil, nil
				}
				return value, nil
			}
			value := values[name]
			if value == nil {
				return nil, nil
			}
			return value, nil
		}

		newObjectInstance := map[string]interface{}{}
		isEmpty := true

		suffix := name + "."
		if name == "" {
			suffix = ""
		}

		for k, v := range schema.Properties {
			if excludeFromMode(mode, v.Value) {
				continue
			}

			ex, err := CreateRequestBody(mode, mediaType, suffix+k, v.Value, values)
			if err != nil {
				return nil, fmt.Errorf("can't get newObjectInstance for '%s'", k)
			}
			if ex == nil {
				continue
			}

			isEmpty = false
			if mediaType == "application/x-www-form-urlencoded" {

				if v.Value.Type == "array" {

					for _, val := range ex.([]interface{}) {

						switch val.(type) {
						case map[string]interface{}:
							mapVal := val.(map[string]interface{})
							for subKey, subVal := range mapVal {
								newObjectInstance[fmt.Sprintf("%s[][%s]", suffix+k, subKey)] = subVal
							}
						default:
							newObjectInstance[fmt.Sprintf("%s[]", suffix+k)] = val
						}
					}
				} else if v.Value.Type == "object" {
					if suffix == "" {
						for k1, v := range ex.(map[string]interface{}) {
							newObjectInstance[fmt.Sprintf("%s[%s]", k, k1)] = v
						}
					} else {
						newObjectInstance[fmt.Sprintf("%s[%s]", suffix, k)] = v
					}
				} else {
					newObjectInstance[suffix+k] = ex
				}
			} else {
				newObjectInstance[suffix+k] = ex
			}

		}

		if schema.AdditionalProperties != nil && schema.AdditionalProperties.Value != nil {
			addl := schema.AdditionalProperties.Value

			if !excludeFromMode(mode, addl) {
				ex, err := CreateRequestBody(mode, mediaType, suffix+name, addl, values)
				resource.CheckErr(err, "can't get newObjectInstance for additional properties")
				if ex != nil {
					isEmpty = false
					for k, v := range ex.(map[string]interface{}) {
						newObjectInstance[k] = v
					}
				}
			}
		}
		if schema.AdditionalPropertiesAllowed != nil && *schema.AdditionalPropertiesAllowed {
			valMap, ok := values[name].(map[string]interface{})
			if ok {
				for key, val := range valMap {
					newObjectInstance[key] = val
					isEmpty = false
				}
			}
		}
		composed, err := createComposedRequestBody(mode, mediaType, name, schema, values)
		if err != nil {
			return nil, err
		}
		if composed != nil {
			isEmpty = false
			composedMap, ok := composed.(map[string]interface{})
			if !ok {
				if name == "" {
					return composed, nil
				}
				return nil, fmt.Errorf("can't merge composed request body for '%s'", name)
			}
			for key, val := range composedMap {
				newObjectInstance[key] = val
			}
		}
		if isEmpty {
			return nil, nil
		}

		return newObjectInstance, nil

	case len(schema.OneOf) > 0:
		return createFirstMatchingComposedRequestBody(mode, mediaType, name, schema.OneOf, values, "oneOf")
	case len(schema.AnyOf) > 0:
		return createFirstMatchingComposedRequestBody(mode, mediaType, name, schema.AnyOf, values, "anyOf")
	case len(schema.AllOf) > 0:
		return createMergedComposedRequestBody(mode, mediaType, name, schema.AllOf, values, "allOf")
	}

	return nil, errors.New("not a valid schema")
}

func createComposedRequestBody(mode Mode, mediaType string, name string, schema *openapi3.Schema, values map[string]interface{}) (interface{}, error) {
	if len(schema.AllOf) > 0 {
		return createMergedComposedRequestBody(mode, mediaType, name, schema.AllOf, values, "allOf")
	}
	if len(schema.OneOf) > 0 {
		return createFirstMatchingComposedRequestBody(mode, mediaType, name, schema.OneOf, values, "oneOf")
	}
	if len(schema.AnyOf) > 0 {
		return createFirstMatchingComposedRequestBody(mode, mediaType, name, schema.AnyOf, values, "anyOf")
	}
	return nil, nil
}

func createFirstMatchingComposedRequestBody(mode Mode, mediaType string, name string, schemaRefs openapi3.SchemaRefs, values map[string]interface{}, construct string) (interface{}, error) {
	var lastErr error
	for index, schemaRef := range schemaRefs {
		if schemaRef == nil || schemaRef.Value == nil {
			return nil, fmt.Errorf("not a valid schema: %s branch %d is empty", construct, index)
		}
		value, err := CreateRequestBody(mode, mediaType, name, schemaRef.Value, values)
		if err != nil {
			lastErr = fmt.Errorf("%s branch %d: %w", construct, index, err)
			continue
		}
		if value != nil {
			return value, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}

func createMergedComposedRequestBody(mode Mode, mediaType string, name string, schemaRefs openapi3.SchemaRefs, values map[string]interface{}, construct string) (interface{}, error) {
	merged := map[string]interface{}{}
	for index, schemaRef := range schemaRefs {
		if schemaRef == nil || schemaRef.Value == nil {
			return nil, fmt.Errorf("not a valid schema: %s branch %d is empty", construct, index)
		}
		value, err := CreateRequestBody(mode, mediaType, name, schemaRef.Value, values)
		if err != nil {
			return nil, fmt.Errorf("%s branch %d: %w", construct, index, err)
		}
		if value == nil {
			continue
		}
		valueMap, ok := value.(map[string]interface{})
		if !ok {
			if len(merged) == 0 {
				return value, nil
			}
			return nil, fmt.Errorf("can't merge non-object %s branch %d", construct, index)
		}
		for key, val := range valueMap {
			merged[key] = val
		}
	}
	if len(merged) == 0 {
		return nil, nil
	}
	return merged, nil
}

// excludeFromMode will exclude a schema if the mode is request and the schema
// is read-only, or if the mode is response and the schema is write only.
func excludeFromMode(mode Mode, schema *openapi3.Schema) bool {
	if schema == nil {
		return true
	}

	if mode == ModeRequest && schema.ReadOnly {
		return true
	} else if mode == ModeResponse && schema.WriteOnly {
		return true
	}

	return false
}

// Create a new action performer for becoming administrator action
func NewIntegrationActionPerformer(integration resource.Integration, initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, configStore *resource.ConfigStore, transaction *sqlx.Tx) (performer actionresponse.ActionPerformerInterface, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Errorf("Recovered panic while creating integration action performer provider=[%s]: %v", integration.Name, recovered)
			performer = nil
			err = fmt.Errorf("failed to create integration action performer for [%s]: %v", integration.Name, recovered)
		}
	}()

	yamlBytes := []byte(integration.Specification)
	var router *openapi3.T

	if integration.SpecificationLanguage == "openapiv2" {
		openapiv2Spec := openapi2.T{}
		if integration.SpecificationFormat == "json" {

			err = json.Unmarshal(yamlBytes, &openapiv2Spec)
			if err != nil {
				log.Errorf("Failed to unmarshal json for integration: %v", err)
				return nil, err
			}

		} else if integration.SpecificationFormat == "yaml" {
			err = yaml.Unmarshal(yamlBytes, &openapiv2Spec)
			if err != nil {
				log.Errorf("Failed to unmarshal yaml for integration: %v", err)
				return nil, err
			}

		}
		router, err = openapi2conv.ToV3(&openapiv2Spec)
	} else if integration.SpecificationLanguage == "openapiv3" {
		if integration.SpecificationFormat == "json" {

			err = json.Unmarshal(yamlBytes, &router)
			if err != nil {
				log.Errorf("Failed to unmarshal json for integration: %v", err)
				return nil, err
			}

		} else if integration.SpecificationFormat == "yaml" {
			err = yaml.Unmarshal(yamlBytes, &router)
			if err != nil {
				log.Errorf("Failed to unmarshal yaml for integration: %v", err)
				return nil, err
			}

		}
	}

	// Check if router was successfully parsed
	if router == nil {
		log.Errorf("Failed to parse OpenAPI specification for integration [%s]: specification could not be loaded (language=%s, format=%s)",
			integration.Name, integration.SpecificationLanguage, integration.SpecificationFormat)
		return nil, fmt.Errorf("OpenAPI specification is invalid or could not be parsed for integration [%s]", integration.Name)
	}

	// Resolve references with panic recovery for incomplete/invalid specs
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Failed to resolve OpenAPI references for integration [%s]: %v", integration.Name, r)
				err = fmt.Errorf("OpenAPI specification has invalid or unresolved references: %v", r)
			}
		}()
		err = openapi3.NewLoader().ResolveRefsIn(router, nil)
	}()

	if err != nil {
		log.Errorf("Failed to load swagger spec for integration [%s]: %v", integration.Name, err)
		return nil, err
	}
	if router.Servers == nil || len(router.Servers) == 0 {
		log.Errorf("No servers found in integration spec of [%s]", integration.Name)
	}

	commandMap := make(map[string]*openapi3.Operation)
	pathMap := make(map[string]string)
	methodMap := make(map[string]string)
	count := 0
	for path, pathItem := range router.Paths {
		if pathItem == nil {
			log.Warnf("Skipping nil path item in integration spec [%s] path=[%s]", integration.Name, path)
			continue
		}
		for method, command := range pathItem.Operations() {
			if command == nil {
				log.Warnf("Skipping nil operation in integration spec [%s] method=[%s] path=[%s]", integration.Name, method, path)
				continue
			}
			count += 1
			operationID := command.OperationID
			if len(operationID) == 0 {
				operationID = method + " " + path
			}
			if _, exists := commandMap[operationID]; exists {
				return nil, fmt.Errorf("duplicate operationId [%s] in integration [%s]", operationID, integration.Name)
			}
			commandMap[operationID] = command
			pathMap[operationID] = path
			methodMap[operationID] = method
			log.Debugf("Mapped integration operation [%s] in [%s]", operationID, integration.Name)
		}
	}
	log.Infof("Registered %d integration actions from [%s]", count, integration.Name)

	encryptionSecret, err := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)
	if err != nil {
		log.Errorf("Failed to get encryption secret from config store: %v", err)
	}

	handler := integrationActionPerformer{
		cruds:            cruds,
		integration:      integration,
		router:           router,
		commandMap:       commandMap,
		pathMap:          pathMap,
		methodMap:        methodMap,
		encryptionSecret: []byte(encryptionSecret),
	}

	return &handler, nil

}
