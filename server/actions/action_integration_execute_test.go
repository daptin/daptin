package actions

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	gorillawebsocket "github.com/gorilla/websocket"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	grpc_testing "google.golang.org/grpc/reflection/grpc_testing"
)

func integrationResponderAttributes(t *testing.T, responder api2go.Responder, responseType string) map[string]interface{} {
	t.Helper()
	model, ok := responder.Result().(api2go.Api2GoModel)
	if !ok {
		t.Fatalf("integration responder result is %T, want api2go.Api2GoModel", responder.Result())
	}
	if model.GetName() != responseType {
		t.Fatalf("integration response model type = %q, want %q", model.GetName(), responseType)
	}
	return model.GetAttributes()
}

func assertRawIntegrationActionResponse(t *testing.T, responses []actionresponse.ActionResponse, responseType string) map[string]interface{} {
	t.Helper()
	if len(responses) < 1 || responses[0].ResponseType != responseType {
		t.Fatalf("unexpected integration action responses: %#v", responses)
	}
	attributes, ok := responses[0].Attributes.(map[string]interface{})
	if !ok {
		t.Fatalf("integration action response attributes are %T, want map[string]interface{}", responses[0].Attributes)
	}
	if _, exists := attributes["__type"]; exists {
		t.Fatalf("internal model type leaked into raw action response: %#v", attributes)
	}
	return attributes
}

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

func TestGraphQLIntegrationRequestBodyUsesOperationExtensions(t *testing.T) {
	operation := linearListIssuesOperation()
	transportConfig, err := integrationTransportConfigFromOperation(operation, "listIssues")
	if err != nil {
		t.Fatalf("integrationTransportConfigFromOperation returned error: %v", err)
	}
	body, err := createGraphQLIntegrationRequestBody(&openapi3.T{}, operation, transportConfig, map[string]interface{}{
		"first":          float64(10),
		"after":          "cursor-1",
		"oauth_token_id": "token-ref",
		"sessionUser":    &auth.SessionUser{UserId: 1},
	}, nil, nil)
	if err != nil {
		t.Fatalf("createGraphQLIntegrationRequestBody returned error: %v", err)
	}
	if body["query"] != "query ListIssues($first: Int, $after: String) { issues(first: $first, after: $after) { nodes { id } } }" {
		t.Fatalf("unexpected query: %#v", body["query"])
	}
	if body["operationName"] != "ListIssues" {
		t.Fatalf("unexpected operation name: %#v", body["operationName"])
	}
	variables, ok := body["variables"].(map[string]interface{})
	if !ok {
		t.Fatalf("variables missing or wrong type: %#v", body["variables"])
	}
	if variables["first"] != 10 || variables["after"] != "cursor-1" {
		t.Fatalf("variables were not built from input: %#v", variables)
	}
	if _, ok := variables["oauth_token_id"]; ok {
		t.Fatalf("oauth selector leaked into GraphQL variables: %#v", variables)
	}
	if _, ok := variables["sessionUser"]; ok {
		t.Fatalf("runtime session leaked into GraphQL variables: %#v", variables)
	}
}

func TestGraphQLIntegrationExecutionPostsToUpstreamPath(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedAuth string
	var capturedBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		bodyBytes, _ := io.ReadAll(r.Body)
		capturedBody = string(bodyBytes)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"viewer":{"id":"user-1"}}}`))
	}))
	defer upstream.Close()

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	credentialRef, userRef, _, adminGroupRef, secret := setupIntegrationCredentialTestDB(t, db)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	authSpec, err := resource.Encrypt([]byte(secret), `{"scheme":"bearer","token_field":"token"}`)
	if err != nil {
		t.Fatalf("encrypt auth spec: %v", err)
	}
	operation := linearListIssuesOperation()
	router := &openapi3.T{
		Servers: openapi3.Servers{&openapi3.Server{URL: upstream.URL}},
		Paths: openapi3.Paths{
			"/issues/list": &openapi3.PathItem{Post: operation},
		},
	}
	addIntegrationTestBearerSecurity(router)
	performer := &integrationActionPerformer{
		cruds: map[string]*resource.DbResource{
			"credential": {
				ConfigStore:          &resource.ConfigStore{},
				AdministratorGroupId: adminGroupRef,
			},
		},
		integration: resource.Integration{
			Name:                        "linear.app",
			AuthenticationType:          "custom_credentials",
			AuthenticationSpecification: authSpec,
		},
		router:           router,
		commandMap:       map[string]*openapi3.Operation{"listIssues": operation},
		pathMap:          map[string]string{"listIssues": "/issues/list"},
		methodMap:        map[string]string{"listIssues": "post"},
		encryptionSecret: []byte(secret),
		runtimeState: func(*sqlx.Tx) (string, bool, error) {
			return "linear.app", true, nil
		},
	}

	responder, responses, errs := performer.DoAction(actionresponse.Outcome{
		Type:   "linear.app",
		Method: "listIssues",
	}, map[string]interface{}{
		"credential_id": credentialRef,
		"sessionUser":   &auth.SessionUser{UserId: 42, UserReferenceId: userRef},
		"first":         float64(5),
	}, tx)
	if len(errs) > 0 {
		t.Fatalf("DoAction returned errors: %v", errs)
	}
	if responder == nil || responder.StatusCode() != http.StatusOK {
		t.Fatalf("unexpected responder: %#v", responder)
	}
	attributes := integrationResponderAttributes(t, responder, "linear.app.listIssues.response")
	if _, ok := attributes["data"]; !ok {
		t.Fatalf("GraphQL response data missing from model: %#v", attributes)
	}
	assertRawIntegrationActionResponse(t, responses, "linear.app.listIssues.response")
	if capturedMethod != http.MethodPost {
		t.Fatalf("unexpected method: %s", capturedMethod)
	}
	if capturedPath != "/graphql" {
		t.Fatalf("expected GraphQL upstream path, got %s", capturedPath)
	}
	if capturedAuth != "Bearer owner-token" {
		t.Fatalf("auth header was not forwarded: %s", capturedAuth)
	}
	if !strings.Contains(capturedBody, `"query"`) || !strings.Contains(capturedBody, `"operationName":"ListIssues"`) || !strings.Contains(capturedBody, `"first":5`) {
		t.Fatalf("unexpected GraphQL body: %s", capturedBody)
	}
	if strings.Contains(capturedBody, "credential_id") || strings.Contains(capturedBody, "sessionUser") {
		t.Fatalf("runtime fields leaked into GraphQL body: %s", capturedBody)
	}
}

func TestNonRESTIntegrationTransportsRejectOtherUserCredentialBeforeUpstreamCall(t *testing.T) {
	transports := []struct {
		name        string
		operation   *openapi3.Operation
		operationID string
		path        string
		input       map[string]interface{}
	}{
		{
			name:        "graphql",
			operation:   linearListIssuesOperation(),
			operationID: "listIssues",
			path:        "/issues/list",
			input:       map[string]interface{}{"first": float64(5)},
		},
		{
			name:        "websocket",
			operation:   websocketSearchOperation(),
			operationID: "search",
			path:        "/socket",
			input:       map[string]interface{}{"query": "tickets"},
		},
		{
			name:        "grpc",
			operation:   grpcSearchOperation(),
			operationID: "Search",
			path:        "/grpc",
			input:       map[string]interface{}{"query": "daptin"},
		},
	}

	for _, tt := range transports {
		t.Run(tt.name, func(t *testing.T) {
			var upstreamCalled bool
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamCalled = true
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer upstream.Close()

			db, err := sqlx.Open("sqlite3", ":memory:")
			if err != nil {
				t.Fatalf("open sqlite: %v", err)
			}
			defer db.Close()
			credentialRef, _, otherUserRef, adminGroupRef, secret := setupIntegrationCredentialTestDB(t, db)
			tx, err := db.Beginx()
			if err != nil {
				t.Fatalf("begin tx: %v", err)
			}
			defer tx.Rollback()

			authSpec, err := resource.Encrypt([]byte(secret), `{"scheme":"bearer","token_field":"token"}`)
			if err != nil {
				t.Fatalf("encrypt auth spec: %v", err)
			}
			performer := integrationTestPerformer(upstream.URL, tt.operation, tt.operationID, tt.path, authSpec, secret, adminGroupRef)
			input := map[string]interface{}{
				"credential_id": credentialRef,
				"sessionUser":   &auth.SessionUser{UserId: 77, UserReferenceId: otherUserRef},
			}
			for key, value := range tt.input {
				input[key] = value
			}
			_, _, errs := performer.DoAction(actionresponse.Outcome{
				Type:   "integration.example",
				Method: tt.operationID,
			}, input, tx)
			if len(errs) == 0 {
				t.Fatalf("expected credential ownership check to fail")
			}
			if !strings.Contains(errs[0].Error(), "credential is not available") {
				t.Fatalf("unexpected error: %v", errs[0])
			}
			if upstreamCalled {
				t.Fatalf("upstream was called before credential ownership was enforced")
			}
		})
	}
}

func TestGraphQLOperationDefaultsUpstreamPath(t *testing.T) {
	operation := &openapi3.Operation{OperationID: "viewer"}
	operation.Extensions = map[string]interface{}{"x-daptin-graphql-document": []byte(`"query Viewer { viewer { id } }"`)}
	transportConfig, err := integrationTransportConfigFromOperation(operation, "viewer")
	if err != nil {
		t.Fatalf("integrationTransportConfigFromOperation returned error: %v", err)
	}
	if transportConfig.Transport != integrationTransportGraphQL || transportConfig.UpstreamPath != "/graphql" {
		t.Fatalf("unexpected transport metadata: %+v", transportConfig)
	}
}

func TestGraphQLOperationRejectsNonStringDocument(t *testing.T) {
	operation := &openapi3.Operation{OperationID: "viewer"}
	operation.Extensions = map[string]interface{}{"x-daptin-graphql-document": map[string]interface{}{"query": "bad"}}
	_, err := integrationTransportConfigFromOperation(operation, "viewer")
	if err == nil {
		t.Fatalf("expected non-string GraphQL document to fail")
	}
}

func TestRESTIntegrationExecutionStillUsesOperationMethodPathAndQuery(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedQuery string
	var capturedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedQuery = r.URL.Query().Get("opt_fields")
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"gid":"123"}}`))
	}))
	defer upstream.Close()

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	credentialRef, userRef, _, adminGroupRef, secret := setupIntegrationCredentialTestDB(t, db)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	authSpec, err := resource.Encrypt([]byte(secret), `{"scheme":"bearer","token_field":"token"}`)
	if err != nil {
		t.Fatalf("encrypt auth spec: %v", err)
	}
	operation := &openapi3.Operation{
		OperationID: "getTask",
		Parameters: openapi3.Parameters{
			&openapi3.ParameterRef{Value: &openapi3.Parameter{Name: "task_gid", In: "path", Required: true, Schema: openapi3.NewStringSchema().NewRef()}},
			&openapi3.ParameterRef{Value: &openapi3.Parameter{Name: "opt_fields", In: "query", Schema: openapi3.NewStringSchema().NewRef()}},
		},
		Responses: openapi3.Responses{"200": &openapi3.ResponseRef{Value: &openapi3.Response{Description: stringPointer("OK")}}},
	}
	router := &openapi3.T{
		Servers: openapi3.Servers{&openapi3.Server{URL: upstream.URL}},
		Paths: openapi3.Paths{
			"/tasks/{task_gid}": &openapi3.PathItem{Get: operation},
		},
	}
	addIntegrationTestBearerSecurity(router)
	performer := &integrationActionPerformer{
		cruds: map[string]*resource.DbResource{
			"credential": {
				ConfigStore:          &resource.ConfigStore{},
				AdministratorGroupId: adminGroupRef,
			},
		},
		integration: resource.Integration{
			Name:                        "asana.com",
			AuthenticationType:          "custom_credentials",
			AuthenticationSpecification: authSpec,
		},
		router:           router,
		commandMap:       map[string]*openapi3.Operation{"getTask": operation},
		pathMap:          map[string]string{"getTask": "/tasks/{task_gid}"},
		methodMap:        map[string]string{"getTask": "get"},
		encryptionSecret: []byte(secret),
		runtimeState: func(*sqlx.Tx) (string, bool, error) {
			return "asana.com", true, nil
		},
	}

	responder, responses, errs := performer.DoAction(actionresponse.Outcome{
		Type:   "asana.com",
		Method: "getTask",
	}, map[string]interface{}{
		"credential_id": credentialRef,
		"sessionUser":   &auth.SessionUser{UserId: 42, UserReferenceId: userRef},
		"task_gid":      "123",
		"opt_fields":    "gid,name",
	}, tx)
	if len(errs) > 0 {
		t.Fatalf("DoAction returned errors: %v", errs)
	}
	if responder == nil || responder.StatusCode() != http.StatusOK {
		t.Fatalf("unexpected responder: %#v", responder)
	}
	attributes := integrationResponderAttributes(t, responder, "asana.com.getTask.response")
	data, ok := attributes["data"].(map[string]interface{})
	if !ok || data["gid"] != "123" {
		t.Fatalf("REST response data missing from model: %#v", attributes)
	}
	assertRawIntegrationActionResponse(t, responses, "asana.com.getTask.response")
	if capturedMethod != http.MethodGet || capturedPath != "/tasks/123" || capturedQuery != "gid,name" {
		t.Fatalf("REST operation changed: method=%s path=%s query=%s", capturedMethod, capturedPath, capturedQuery)
	}
	if capturedAuth != "Bearer owner-token" {
		t.Fatalf("auth header was not forwarded: %s", capturedAuth)
	}
}

func TestIntegrationOperationSecurityEmptyOverrideAndOptionalAnonymous(t *testing.T) {
	tests := []struct {
		name     string
		security openapi3.SecurityRequirements
	}{
		{name: "empty operation override", security: openapi3.SecurityRequirements{}},
		{name: "anonymous alternative", security: openapi3.SecurityRequirements{{"bearerAuth": {}}, {}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var authorization string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authorization = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer upstream.Close()

			operation := &openapi3.Operation{
				OperationID: "publicOperation",
				Security:    &test.security,
				Parameters: openapi3.Parameters{{Value: &openapi3.Parameter{
					Name: "Authorization", In: "header", Schema: openapi3.NewStringSchema().NewRef(),
				}}},
				Responses: openapi3.Responses{"200": {Value: &openapi3.Response{Description: stringPointer("OK")}}},
			}
			router := &openapi3.T{
				Servers: openapi3.Servers{{URL: upstream.URL}},
				Paths:   openapi3.Paths{"/public": {Get: operation}},
			}
			addIntegrationTestBearerSecurity(router)
			performer := &integrationActionPerformer{
				integration: resource.Integration{Name: "public.example", AuthenticationType: "custom_credentials"},
				router:      router,
				commandMap:  map[string]*openapi3.Operation{"publicOperation": operation},
				pathMap:     map[string]string{"publicOperation": "/public"},
				methodMap:   map[string]string{"publicOperation": "get"},
				runtimeState: func(*sqlx.Tx) (string, bool, error) {
					return "public.example", true, nil
				},
			}
			responder, _, errs := performer.DoAction(actionresponse.Outcome{Method: "publicOperation"}, map[string]interface{}{
				"Authorization": "Bearer caller-controlled",
			}, nil)
			if len(errs) > 0 {
				t.Fatalf("anonymous operation failed: %v", errs)
			}
			if responder == nil || responder.StatusCode() != http.StatusOK {
				t.Fatalf("unexpected responder: %#v", responder)
			}
			if authorization != "" {
				t.Fatalf("anonymous operation sent authentication: %q", authorization)
			}
		})
	}
}

func TestIntegrationOperationSecurityRequiresEveryScheme(t *testing.T) {
	var capturedHeader string
	var capturedQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get("X-First")
		capturedQuery = r.URL.Query().Get("second_key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	credentialRef, userRef, _, adminGroupRef, secret := setupIntegrationCredentialTestDB(t, db)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	authSpec, err := resource.Encrypt([]byte(secret), `{}`)
	if err != nil {
		t.Fatal(err)
	}
	operation := &openapi3.Operation{
		OperationID: "compound",
		Responses:   openapi3.Responses{"200": {Value: &openapi3.Response{Description: stringPointer("OK")}}},
	}
	router := &openapi3.T{
		Servers:  openapi3.Servers{{URL: upstream.URL}},
		Security: openapi3.SecurityRequirements{{"oauth": {}}, {"first": {}, "second": {}}},
		Components: openapi3.Components{SecuritySchemes: openapi3.SecuritySchemes{
			"first":  {Value: &openapi3.SecurityScheme{Type: "apiKey", In: "header", Name: "X-First"}},
			"second": {Value: &openapi3.SecurityScheme{Type: "apiKey", In: "query", Name: "second_key"}},
			"oauth":  {Value: &openapi3.SecurityScheme{Type: "oauth2"}},
		}},
		Paths: openapi3.Paths{"/compound": {Get: operation}},
	}
	performer := &integrationActionPerformer{
		cruds: map[string]*resource.DbResource{"credential": {
			ConfigStore: &resource.ConfigStore{}, AdministratorGroupId: adminGroupRef,
		}},
		integration: resource.Integration{
			Name: "compound.example", AuthenticationType: "custom_credentials", AuthenticationSpecification: authSpec,
		},
		router: router, commandMap: map[string]*openapi3.Operation{"compound": operation},
		pathMap: map[string]string{"compound": "/compound"}, methodMap: map[string]string{"compound": "get"},
		encryptionSecret: []byte(secret),
		runtimeState:     func(*sqlx.Tx) (string, bool, error) { return "compound.example", true, nil },
	}
	_, _, errs := performer.DoAction(actionresponse.Outcome{Method: "compound"}, map[string]interface{}{
		"credential_id": credentialRef,
		"sessionUser":   &auth.SessionUser{UserId: 42, UserReferenceId: userRef},
	}, tx)
	if len(errs) > 0 {
		t.Fatalf("compound authentication failed: %v", errs)
	}
	if capturedHeader != "first-value" || capturedQuery != "second-value" {
		t.Fatalf("compound authentication = header %q query %q", capturedHeader, capturedQuery)
	}
}

func TestIntegrationSecurityCookieMergeIsExact(t *testing.T) {
	merged, err := mergeIntegrationCookieHeaders("first=one", "second=two")
	if err != nil {
		t.Fatal(err)
	}
	if merged != "first=one; second=two" {
		t.Fatalf("merged cookies = %q", merged)
	}
	if _, err := mergeIntegrationCookieHeaders("token=one", "token=two"); err == nil {
		t.Fatal("expected conflicting cookie values to fail")
	}
}

func TestProtectedIntegrationRejectsMalformedAuthenticationSpecification(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	authSpec, err := resource.Encrypt([]byte(secret), `not-json`)
	if err != nil {
		t.Fatal(err)
	}
	operation := &openapi3.Operation{}
	router := &openapi3.T{}
	addIntegrationTestBearerSecurity(router)
	performer := &integrationActionPerformer{
		integration: resource.Integration{
			AuthenticationType:          "custom_credentials",
			AuthenticationSpecification: authSpec,
		},
		router:           router,
		encryptionSecret: []byte(secret),
	}
	_, err = performer.resolveIntegrationTransportAuth(operation, map[string]interface{}{
		"credential_id": daptinid.DaptinReferenceId(uuid.New()),
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "failed to parse integration authentication specification") {
		t.Fatalf("malformed authentication specification error = %v", err)
	}
}

func TestWebSocketIntegrationExecutionUsesShortLivedRequestResponse(t *testing.T) {
	upgrader := gorillawebsocket.Upgrader{}
	var capturedAuth string
	var capturedMessage map[string]interface{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer conn.Close()
		if err := conn.ReadJSON(&capturedMessage); err != nil {
			t.Errorf("read websocket message: %v", err)
			return
		}
		_ = conn.WriteJSON(map[string]interface{}{"ok": true, "echo": capturedMessage["query"]})
	}))
	defer upstream.Close()

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	credentialRef, userRef, _, adminGroupRef, secret := setupIntegrationCredentialTestDB(t, db)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	operation := websocketSearchOperation()
	authSpec, err := resource.Encrypt([]byte(secret), `{"scheme":"bearer","token_field":"token"}`)
	if err != nil {
		t.Fatalf("encrypt auth spec: %v", err)
	}
	performer := integrationTestPerformer(upstream.URL, operation, "search", "/socket", authSpec, secret, adminGroupRef)
	responder, responses, errs := performer.DoAction(actionresponse.Outcome{
		Type:   "realtime.example",
		Method: "search",
	}, map[string]interface{}{
		"credential_id": credentialRef,
		"sessionUser":   &auth.SessionUser{UserId: 42, UserReferenceId: userRef},
		"query":         "tickets",
	}, tx)
	if len(errs) > 0 {
		t.Fatalf("DoAction returned errors: %v", errs)
	}
	if responder == nil || responder.StatusCode() != http.StatusOK {
		t.Fatalf("unexpected responder: %#v", responder)
	}
	attributes := integrationResponderAttributes(t, responder, "integration.example.search.response")
	if attributes["ok"] != true || attributes["echo"] != "tickets" {
		t.Fatalf("websocket response data missing from model: %#v", attributes)
	}
	assertRawIntegrationActionResponse(t, responses, "integration.example.search.response")
	if capturedAuth != "Bearer owner-token" {
		t.Fatalf("auth header was not forwarded: %s", capturedAuth)
	}
	if capturedMessage["query"] != "tickets" {
		t.Fatalf("request body fields were not sent as websocket message: %#v", capturedMessage)
	}
}

func TestGRPCIntegrationExecutionUsesReflectionUnaryCall(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen grpc: %v", err)
	}
	grpcServer := grpc.NewServer()
	grpc_testing.RegisterSearchServiceServer(grpcServer, searchServiceServer{})
	reflection.Register(grpcServer)
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	credentialRef, userRef, _, adminGroupRef, secret := setupIntegrationCredentialTestDB(t, db)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	operation := grpcSearchOperation()
	authSpec, err := resource.Encrypt([]byte(secret), `{"scheme":"bearer","token_field":"token"}`)
	if err != nil {
		t.Fatalf("encrypt auth spec: %v", err)
	}
	performer := integrationTestPerformer("http://"+listener.Addr().String(), operation, "Search", "/grpc", authSpec, secret, adminGroupRef)
	responder, responses, errs := performer.DoAction(actionresponse.Outcome{
		Type:   "grpc.example",
		Method: "Search",
	}, map[string]interface{}{
		"credential_id": credentialRef,
		"sessionUser":   &auth.SessionUser{UserId: 42, UserReferenceId: userRef},
		"query":         "daptin",
	}, tx)
	if len(errs) > 0 {
		t.Fatalf("DoAction returned errors: %v", errs)
	}
	if responder == nil || responder.StatusCode() != http.StatusOK {
		t.Fatalf("unexpected responder: %#v", responder)
	}
	result := integrationResponderAttributes(t, responder, "integration.example.Search.response")
	assertRawIntegrationActionResponse(t, responses, "integration.example.Search.response")
	results, ok := result["results"].([]interface{})
	if !ok || len(results) != 1 {
		t.Fatalf("unexpected grpc response: %#v", result)
	}
	firstResult := results[0].(map[string]interface{})
	if firstResult["title"] != "Result for daptin" {
		t.Fatalf("unexpected grpc result: %#v", firstResult)
	}
}

func linearListIssuesOperation() *openapi3.Operation {
	operation := &openapi3.Operation{
		OperationID: "listIssues",
		RequestBody: &openapi3.RequestBodyRef{Value: &openapi3.RequestBody{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: openapi3.NewObjectSchema().
						WithProperty("first", openapi3.NewIntegerSchema()).
						WithProperty("after", openapi3.NewStringSchema()).
						NewRef(),
				},
			},
		}},
		Responses: openapi3.Responses{
			"200": &openapi3.ResponseRef{Value: &openapi3.Response{Description: stringPointer("OK")}},
		},
	}
	operation.Extensions = map[string]interface{}{
		"x-daptin-upstream-path":          "/graphql",
		"x-daptin-graphql-operation-name": "ListIssues",
		"x-daptin-graphql-document":       "query ListIssues($first: Int, $after: String) { issues(first: $first, after: $after) { nodes { id } } }",
	}
	return operation
}

func websocketSearchOperation() *openapi3.Operation {
	operation := &openapi3.Operation{
		OperationID: "search",
		RequestBody: &openapi3.RequestBodyRef{Value: &openapi3.RequestBody{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: openapi3.NewObjectSchema().WithProperty("query", openapi3.NewStringSchema()).NewRef(),
				},
			},
		}},
		Responses: openapi3.Responses{"200": &openapi3.ResponseRef{Value: &openapi3.Response{Description: stringPointer("OK")}}},
	}
	operation.Extensions = map[string]interface{}{
		"x-daptin-transport":     "websocket",
		"x-daptin-upstream-path": "/socket",
	}
	return operation
}

func grpcSearchOperation() *openapi3.Operation {
	operation := &openapi3.Operation{
		OperationID: "Search",
		RequestBody: &openapi3.RequestBodyRef{Value: &openapi3.RequestBody{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: openapi3.NewObjectSchema().WithProperty("query", openapi3.NewStringSchema()).NewRef(),
				},
			},
		}},
		Responses: openapi3.Responses{"200": &openapi3.ResponseRef{Value: &openapi3.Response{Description: stringPointer("OK")}}},
	}
	operation.Extensions = map[string]interface{}{
		"x-daptin-transport":    "grpc",
		"x-daptin-grpc-service": "grpc.testing.SearchService",
		"x-daptin-grpc-method":  "Search",
	}
	return operation
}

func integrationTestPerformer(baseURL string, operation *openapi3.Operation, operationID string, path string, authSpec string, secret string, adminGroupRef daptinid.DaptinReferenceId) *integrationActionPerformer {
	router := &openapi3.T{
		Servers: openapi3.Servers{&openapi3.Server{URL: baseURL}},
		Paths: openapi3.Paths{
			path: &openapi3.PathItem{Post: operation},
		},
	}
	addIntegrationTestBearerSecurity(router)
	return &integrationActionPerformer{
		cruds: map[string]*resource.DbResource{
			"credential": {
				ConfigStore:          &resource.ConfigStore{},
				AdministratorGroupId: adminGroupRef,
			},
		},
		integration: resource.Integration{
			Name:                        "integration.example",
			AuthenticationType:          "custom_credentials",
			AuthenticationSpecification: authSpec,
		},
		router:           router,
		commandMap:       map[string]*openapi3.Operation{operationID: operation},
		pathMap:          map[string]string{operationID: path},
		methodMap:        map[string]string{operationID: "post"},
		encryptionSecret: []byte(secret),
		runtimeState: func(*sqlx.Tx) (string, bool, error) {
			return "integration.example", true, nil
		},
	}
}

func addIntegrationTestBearerSecurity(router *openapi3.T) {
	router.Security = openapi3.SecurityRequirements{{"bearerAuth": {}}}
	router.Components.SecuritySchemes = openapi3.SecuritySchemes{
		"bearerAuth": {Value: &openapi3.SecurityScheme{Type: "http", Scheme: "bearer"}},
	}
}

type searchServiceServer struct {
	grpc_testing.UnimplementedSearchServiceServer
}

func (searchServiceServer) Search(_ context.Context, request *grpc_testing.SearchRequest) (*grpc_testing.SearchResponse, error) {
	return &grpc_testing.SearchResponse{
		Results: []*grpc_testing.SearchResponse_Result{
			{Title: "Result for " + request.GetQuery(), Url: "https://example.com/" + request.GetQuery()},
		},
	}, nil
}

func (searchServiceServer) StreamingSearch(grpc.BidiStreamingServer[grpc_testing.SearchRequest, grpc_testing.SearchResponse]) error {
	return nil
}

func setupIntegrationCredentialTestDB(t *testing.T, db *sqlx.DB) (daptinid.DaptinReferenceId, daptinid.DaptinReferenceId, daptinid.DaptinReferenceId, daptinid.DaptinReferenceId, string) {
	t.Helper()
	statements := []string{
		`create table _config (
			id integer primary key,
			name text,
			configtype text,
			configstate text,
			configenv text,
			value text
		)`,
		`create table credential (
			id integer primary key,
			name text not null,
			content text not null,
			user_account_id integer,
			reference_id blob not null unique,
			permission integer not null
		)`,
		`create table user_account (
			id integer primary key,
			reference_id blob not null unique
		)`,
		`create table usergroup (
			id integer primary key,
			reference_id blob not null unique
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}
	secret := "0123456789abcdef0123456789abcdef"
	if _, err := db.Exec(`insert into _config (name, configtype, configstate, configenv, value) values (?, ?, ?, ?, ?)`, "encryption.secret", "backend", "enabled", "", secret); err != nil {
		t.Fatalf("insert config: %v", err)
	}
	credentialRef := daptinid.DaptinReferenceId(uuid.New())
	userRef := daptinid.DaptinReferenceId(uuid.New())
	otherUserRef := daptinid.DaptinReferenceId(uuid.New())
	adminGroupRef := daptinid.DaptinReferenceId(uuid.New())
	encryptedContent, err := resource.Encrypt([]byte(secret), `{"token":"owner-token","X-First":"first-value","second_key":"second-value"}`)
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}
	if _, err := db.Exec(`insert into user_account (id, reference_id) values (?, ?)`, 42, userRef[:]); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(`insert into user_account (id, reference_id) values (?, ?)`, 77, otherUserRef[:]); err != nil {
		t.Fatalf("insert other user: %v", err)
	}
	if _, err := db.Exec(`insert into credential (id, name, content, user_account_id, reference_id, permission) values (?, ?, ?, ?, ?, ?)`, 20, "owner-cred", encryptedContent, 42, credentialRef[:], int64(auth.DEFAULT_PERMISSION)); err != nil {
		t.Fatalf("insert credential: %v", err)
	}
	return credentialRef, userRef, otherUserRef, adminGroupRef, secret
}

func stringPointer(value string) *string {
	return &value
}
