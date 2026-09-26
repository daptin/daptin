package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	grpc_testing "google.golang.org/grpc/reflection/grpc_testing"
)

func TestFirstAdminClosesPublicSignupRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{})
	baseURL, client := fixture.URL, fixture.Client
	adminToken := fixture.SignupAdmin(t)
	blockedEmail := "post-admin-guest@test.local"
	accessGroupsE2EAssertStatus(t, client, http.MethodPost, baseURL+"/action/user_account/signup", "", map[string]interface{}{
		"attributes": map[string]interface{}{
			"name":            "Blocked Guest",
			"email":           blockedEmail,
			"password":        "testpass123",
			"passwordConfirm": "testpass123",
		},
	}, http.StatusForbidden)

	query := fmt.Sprintf(`[{"column":"email","operator":"is","value":%q}]`, blockedEmail)
	response := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		baseURL+"/api/user_account?query="+url.QueryEscape(query), adminToken, nil, http.StatusOK)
	if got := len(accessGroupsE2EDataArray(t, response)); got != 0 {
		t.Fatalf("unauthenticated post-admin signup persisted %d user rows: %#v", got, response)
	}
}

func TestIntegrationOperationActionAuthorizationRealE2E(t *testing.T) {
	requireRealE2E(t)

	var upstreamCalls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		upstreamCalls.Add(1)
		transportE2EWriteJSON(w, map[string]interface{}{"ok": true})
	}))
	defer upstream.Close()

	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	databasePath := filepath.Join(t.TempDir(), "integration-authorization.db")
	options := transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: databasePath, schema: "EnableGraphQL: true\n",
	}
	daptinProcess := startTransportE2EDaptin(t, port, httpsPort, baseURL, options)
	defer func() { daptinProcess.stopProcess() }()

	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	userToken := accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "integration-caller")
	userReference := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "user_account", "email", "integration-caller@test.local")
	credentialReference := transportE2ECreateCredential(t, client, baseURL, adminToken)
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/credential/"+credentialReference, adminToken,
		accessGroupsE2ERecordPayload("credential", credentialReference, map[string]interface{}{"user_account_id": userReference}), http.StatusOK)
	providerName := "authorization.example"
	integrationReference := transportE2ECreateIntegration(t, client, baseURL, adminToken, providerName, transportE2EBaseSpec(
		"Authorization integration", upstream.URL, map[string]interface{}{
			"/allowed": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "invoke",
					"responses":   transportE2EJSONResponses(),
				},
			},
		},
	))
	transportE2EInstallIntegration(t, client, baseURL, adminToken, integrationReference)

	worldReference := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "world", "table_name", "integration")
	actionReference := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "action", "action_name", providerName+"/invoke")
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/world/"+worldReference, adminToken,
		accessGroupsE2ERecordPayload("world", worldReference, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)}), http.StatusOK)
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/action/"+actionReference, adminToken,
		accessGroupsE2ERecordPayload("action", actionReference, map[string]interface{}{"permission": int64(auth.None)}), http.StatusOK)

	directPayload := map[string]interface{}{"credential_id": credentialReference, "input": map[string]interface{}{}}
	directStatus, _ := postLLMActionForStatus(t, client, baseURL+"/integration/"+providerName+"/invoke", userToken, directPayload)
	actionStatus, _ := postLLMActionForStatus(t, client, baseURL+"/action/integration/"+providerName+"/invoke", userToken, map[string]interface{}{
		"attributes": map[string]interface{}{"credential_id": credentialReference},
	})
	if directStatus != http.StatusForbidden || actionStatus != http.StatusForbidden || upstreamCalls.Load() != 0 {
		t.Fatalf("denied integration execution: direct=%d action=%d upstream=%d", directStatus, actionStatus, upstreamCalls.Load())
	}

	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/action/"+actionReference, adminToken,
		accessGroupsE2ERecordPayload("action", actionReference, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)}), http.StatusOK)
	allowedStatus, _ := postLLMActionForStatus(t, client, baseURL+"/integration/"+providerName+"/invoke", userToken, directPayload)
	if allowedStatus != http.StatusOK || upstreamCalls.Load() != 1 {
		t.Fatalf("allowed integration execution: status=%d upstream=%d", allowedStatus, upstreamCalls.Load())
	}

	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/action/"+actionReference, adminToken,
		accessGroupsE2ERecordPayload("action", actionReference, map[string]interface{}{"permission": int64(auth.None)}), http.StatusOK)
	daptinProcess.stopProcess()
	daptinProcess = startTransportE2EDaptin(t, port, httpsPort, baseURL, options)

	graphQLPayload := map[string]interface{}{
		"query": fmt.Sprintf(`mutation { executeInvokeOnAuthorizationexample(credential_id: %q) { ResponseType } }`, credentialReference),
	}
	deniedGraphQL := transportE2EPostJSON(t, client, baseURL+"/graphql", userToken, graphQLPayload)
	if !transportE2EGraphQLHasErrors(deniedGraphQL) || upstreamCalls.Load() != 1 {
		t.Fatalf("denied GraphQL integration execution: response=%#v upstream=%d", deniedGraphQL, upstreamCalls.Load())
	}
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/action/"+actionReference, adminToken,
		accessGroupsE2ERecordPayload("action", actionReference, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)}), http.StatusOK)
	allowedGraphQL := transportE2EPostJSON(t, client, baseURL+"/graphql", userToken, graphQLPayload)
	if transportE2EGraphQLHasErrors(allowedGraphQL) || upstreamCalls.Load() != 2 {
		t.Fatalf("allowed GraphQL integration execution: response=%#v upstream=%d", allowedGraphQL, upstreamCalls.Load())
	}

	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/integration/"+integrationReference, adminToken,
		accessGroupsE2ERecordPayload("integration", integrationReference, map[string]interface{}{"enable": false}), http.StatusOK)
	disabledDirectStatus, _ := postLLMActionForStatus(t, client, baseURL+"/integration/"+providerName+"/invoke", userToken, directPayload)
	disabledActionStatus, _ := postLLMActionForStatus(t, client, baseURL+"/action/integration/"+providerName+"/invoke", userToken, map[string]interface{}{
		"attributes": map[string]interface{}{"credential_id": credentialReference},
	})
	if disabledDirectStatus < http.StatusBadRequest || disabledActionStatus < http.StatusBadRequest || upstreamCalls.Load() != 2 {
		t.Fatalf("disabled integration execution: direct=%d action=%d upstream=%d", disabledDirectStatus, disabledActionStatus, upstreamCalls.Load())
	}
}

func TestIntegrationSwitchedUserCredentialRealE2E(t *testing.T) {
	requireRealE2E(t)

	var upstreamCalls atomic.Int64
	var upstreamAuthorization atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		upstreamCalls.Add(1)
		upstreamAuthorization.Store(request.Header.Get("Authorization"))
		transportE2EWriteJSON(w, map[string]interface{}{"ok": true})
	}))
	defer upstream.Close()

	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	databasePath := filepath.Join(t.TempDir(), "integration-switched-user.db")
	options := transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: databasePath, schema: "EnableGraphQL: true\n",
	}
	daptinProcess := startTransportE2EDaptin(t, port, httpsPort, baseURL, options)
	defer func() { daptinProcess.stopProcess() }()

	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	callerToken := accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "integration-workflow-caller")
	callerReference := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "user_account", "email", "integration-workflow-caller@test.local")
	_ = accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "integration-service-user")
	serviceUserReference := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "user_account", "email", "integration-service-user@test.local")

	serviceCredentialReference := transportE2ECreateCredential(t, client, baseURL, adminToken)
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/credential/"+serviceCredentialReference, adminToken,
		accessGroupsE2ERecordPayload("credential", serviceCredentialReference, map[string]interface{}{"user_account_id": serviceUserReference}), http.StatusOK)

	providerName := "authorization.example"
	integrationReference := transportE2ECreateIntegration(t, client, baseURL, adminToken, providerName, transportE2EBaseSpec(
		"Switched-user credential integration", upstream.URL, map[string]interface{}{
			"/allowed": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "invoke",
					"responses":   transportE2EJSONResponses(),
				},
			},
		},
	))
	transportE2EInstallIntegration(t, client, baseURL, adminToken, integrationReference)

	worldReference := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "world", "table_name", "integration")
	generatedActionReference := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "action", "action_name", providerName+"/invoke")
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/world/"+worldReference, adminToken,
		accessGroupsE2ERecordPayload("world", worldReference, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)}), http.StatusOK)
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/action/"+generatedActionReference, adminToken,
		accessGroupsE2ERecordPayload("action", generatedActionReference, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)}), http.StatusOK)

	options.schema = integrationSwitchedUserActionSchema(providerName, serviceUserReference, serviceCredentialReference)
	daptinProcess.stopProcess()
	daptinProcess = startTransportE2EDaptin(t, port, httpsPort, baseURL, options)

	forgedRuntimeInput := map[string]interface{}{
		"credential_id":      serviceCredentialReference,
		"sessionUser":        map[string]interface{}{"UserReferenceId": serviceUserReference},
		"httpRequest":        map[string]interface{}{"user": serviceUserReference},
		"httpRequestHeaders": map[string]interface{}{"Authorization": "Bearer forged"},
	}
	directStatus, _ := postLLMActionForStatus(t, client, baseURL+"/integration/"+providerName+"/invoke", callerToken, map[string]interface{}{
		"credential_id": serviceCredentialReference,
		"input":         forgedRuntimeInput,
	})
	generatedActionStatus, _ := postLLMActionForStatus(t, client, baseURL+"/action/integration/"+providerName+"/invoke", callerToken, map[string]interface{}{
		"attributes": forgedRuntimeInput,
	})
	graphQLResponse := transportE2EPostJSON(t, client, baseURL+"/graphql", callerToken, map[string]interface{}{
		"query": fmt.Sprintf(`mutation { executeInvokeOnAuthorizationexample(credential_id: %q) { ResponseType } }`, serviceCredentialReference),
	})
	if directStatus < http.StatusBadRequest || generatedActionStatus < http.StatusBadRequest || !transportE2EGraphQLHasErrors(graphQLResponse) {
		t.Fatalf("direct cross-user credential execution was not rejected: REST=%d action=%d GraphQL=%#v", directStatus, generatedActionStatus, graphQLResponse)
	}
	if upstreamCalls.Load() != 0 {
		t.Fatalf("direct cross-user credential execution reached the provider: calls=%d", upstreamCalls.Load())
	}

	wrapperStatus, wrapperBody := postLLMActionForStatus(t, client, baseURL+"/action/integration/issue275_service_workflow", callerToken, map[string]interface{}{
		"attributes": map[string]interface{}{
			"credential_id":     "00000000-0000-0000-0000-000000000000",
			"user_reference_id": callerReference,
			"sessionUser":       map[string]interface{}{"UserReferenceId": callerReference},
		},
	})
	if wrapperStatus != http.StatusOK {
		t.Fatalf("switched-user integration workflow returned %d: %s", wrapperStatus, string(wrapperBody))
	}
	if upstreamCalls.Load() != 1 {
		t.Fatalf("switched-user integration workflow provider calls=%d, want 1", upstreamCalls.Load())
	}
	if authorization, _ := upstreamAuthorization.Load().(string); authorization != "Bearer owner-token" {
		t.Fatalf("switched-user integration workflow authorization=%q, want service credential", authorization)
	}
}

func integrationSwitchedUserActionSchema(providerName string, serviceUserReference string, serviceCredentialReference string) string {
	return fmt.Sprintf(`EnableGraphQL: true
Actions:
  - Name: issue275_service_workflow
    Label: Issue 275 service workflow
    OnType: integration
    InstanceOptional: true
    Permission: %d
    InFields: []
    OutFields:
      - Type: __as_user
        Method: SWITCH_USER
        SkipInResponse: true
        Attributes:
          user_reference_id: %q
      - Type: %s
        Method: invoke
        Attributes:
          credential_id: %q
`, auth.AuthenticatedExecute, serviceUserReference, providerName, serviceCredentialReference)
}

func transportE2EGraphQLHasErrors(response interface{}) bool {
	object, ok := response.(map[string]interface{})
	if !ok {
		return true
	}
	errors, exists := object["errors"]
	if !exists {
		return false
	}
	list, ok := errors.([]interface{})
	return !ok || len(list) > 0
}

func TestRealIntegrationTransportE2E(t *testing.T) {
	requireRealE2E(t)

	httpUpstream := startTransportE2EHTTPUpstream(t)
	defer httpUpstream.Close()

	grpcAddress, stopGRPC := startTransportE2EGRPCUpstream(t)
	defer stopGRPC()

	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	daptinBaseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	options := transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: filepath.Join(t.TempDir(), "integration-transports.db"), schema: "EnableGraphQL: true\n",
	}
	daptinProcess := startTransportE2EDaptin(t, port, httpsPort, daptinBaseURL, options)
	defer func() { daptinProcess.stopProcess() }()

	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := transportE2ESignupSigninAdmin(t, client, daptinBaseURL)
	credentialRef := transportE2ECreateCredential(t, client, daptinBaseURL, adminToken)

	httpIntegrationRef := transportE2ECreateIntegration(t, client, daptinBaseURL, adminToken, "e2e-http-protocols", httpTransportE2ESpec(t, httpUpstream.URL))
	transportE2EInstallIntegration(t, client, daptinBaseURL, adminToken, httpIntegrationRef)

	grpcIntegrationRef := transportE2ECreateIntegration(t, client, daptinBaseURL, adminToken, "e2e-grpc-protocols", grpcTransportE2ESpec(t, grpcAddress))
	transportE2EInstallIntegration(t, client, daptinBaseURL, adminToken, grpcIntegrationRef)

	daptinProcess.stopProcess()
	daptinProcess = startTransportE2EDaptin(t, port, httpsPort, daptinBaseURL, options)

	rest := transportE2EPostJSON(t, client, daptinBaseURL+"/integration/e2e-http-protocols/getTask", adminToken, map[string]interface{}{
		"credential_id": credentialRef,
		"input": map[string]interface{}{
			"task_gid":   "TASK-123",
			"opt_fields": "gid,name",
		},
	})
	assertTransportE2EString(t, rest, "transport", "rest")
	assertTransportE2EString(t, rest, "authorization", "Bearer owner-token")
	assertTransportE2EString(t, rest, "task_gid", "TASK-123")

	graphQL := transportE2EPostJSON(t, client, daptinBaseURL+"/integration/e2e-http-protocols/listIssues", adminToken, map[string]interface{}{
		"credential_id": credentialRef,
		"input": map[string]interface{}{
			"first": float64(2),
			"after": "cursor-1",
		},
	})
	assertTransportE2EString(t, graphQL, "transport", "graphql")
	assertTransportE2EString(t, graphQL, "authorization", "Bearer owner-token")
	assertTransportE2EString(t, graphQL, "operationName", "ListIssues")

	ws := transportE2EPostJSON(t, client, daptinBaseURL+"/integration/e2e-http-protocols/wsSearch", adminToken, map[string]interface{}{
		"credential_id": credentialRef,
		"input": map[string]interface{}{
			"query": "tickets",
		},
	})
	assertTransportE2EString(t, ws, "transport", "websocket")
	assertTransportE2EString(t, ws, "authorization", "Bearer owner-token")
	assertTransportE2EString(t, ws, "query", "tickets")

	grpcResult := transportE2EPostJSON(t, client, daptinBaseURL+"/integration/e2e-grpc-protocols/Search", adminToken, map[string]interface{}{
		"credential_id": credentialRef,
		"input": map[string]interface{}{
			"query": "daptin",
		},
	})
	assertTransportE2EString(t, grpcResult, "results.0.title", "daptin")
	assertTransportE2EString(t, grpcResult, "results.0.snippets.0", "authorization:ok")

	publicDirect := transportE2EPostJSON(t, client, daptinBaseURL+"/integration/e2e-http-protocols/publicStatus", adminToken, map[string]interface{}{
		"input": map[string]interface{}{},
	})
	assertTransportE2EString(t, publicDirect, "transport", "public")
	assertTransportE2EString(t, publicDirect, "authorization", "")

	publicAction := transportE2EPostJSON(t, client, daptinBaseURL+"/action/integration/e2e-http-protocols/publicStatus", adminToken, map[string]interface{}{
		"attributes": map[string]interface{}{},
	})
	if publicAction == nil {
		t.Fatal("anonymous generated integration action returned no response")
	}

	publicGraphQL := transportE2EPostJSON(t, client, daptinBaseURL+"/graphql", adminToken, map[string]interface{}{
		"query": `mutation { executePublicStatusOnE2EHttpProtocols { ResponseType } }`,
	})
	if transportE2EGraphQLHasErrors(publicGraphQL) {
		t.Fatalf("anonymous GraphQL integration operation failed: %#v", publicGraphQL)
	}

	graphQLDetails := transportE2EGetJSON(t, client, daptinBaseURL+"/integration/e2e-http-protocols/operations/listIssues", adminToken)
	assertTransportE2EString(t, graphQLDetails, "extensions.daptin_transport.type", "graphql")
	assertTransportE2EString(t, graphQLDetails, "extensions.daptin_transport.upstream_path", "/graphql")

	wsDetails := transportE2EGetJSON(t, client, daptinBaseURL+"/integration/e2e-http-protocols/operations/wsSearch", adminToken)
	assertTransportE2EString(t, wsDetails, "extensions.daptin_transport.type", "websocket")

	grpcDetails := transportE2EGetJSON(t, client, daptinBaseURL+"/integration/e2e-grpc-protocols/operations/Search", adminToken)
	assertTransportE2EString(t, grpcDetails, "extensions.daptin_transport.type", "grpc")
	assertTransportE2EString(t, grpcDetails, "extensions.daptin_transport.grpc_service", "grpc.testing.SearchService")
}

func startTransportE2EHTTPUpstream(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/rest/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer owner-token" {
			http.Error(w, "missing credential authorization", http.StatusUnauthorized)
			return
		}
		transportE2EWriteJSON(w, map[string]interface{}{
			"transport":     "rest",
			"authorization": r.Header.Get("Authorization"),
			"task_gid":      strings.TrimPrefix(r.URL.Path, "/rest/tasks/"),
			"opt_fields":    r.URL.Query().Get("opt_fields"),
		})
	})
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer owner-token" {
			http.Error(w, "missing credential authorization", http.StatusUnauthorized)
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		variables, _ := body["variables"].(map[string]interface{})
		transportE2EWriteJSON(w, map[string]interface{}{
			"transport":     "graphql",
			"authorization": r.Header.Get("Authorization"),
			"operationName": body["operationName"],
			"variables":     variables,
			"data": map[string]interface{}{
				"issues": map[string]interface{}{
					"nodes": []map[string]interface{}{{"id": "ISS-1", "title": fmt.Sprintf("%v-%v", variables["after"], variables["first"])}},
				},
			},
		})
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer owner-token" {
			http.Error(w, "missing credential authorization", http.StatusUnauthorized)
			return
		}
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var message map[string]interface{}
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		_ = conn.WriteJSON(map[string]interface{}{
			"transport":     "websocket",
			"authorization": r.Header.Get("Authorization"),
			"query":         message["query"],
		})
	})
	mux.HandleFunc("/public", func(w http.ResponseWriter, r *http.Request) {
		transportE2EWriteJSON(w, map[string]interface{}{
			"transport":     "public",
			"authorization": r.Header.Get("Authorization"),
		})
	})

	return httptest.NewServer(mux)
}

type transportE2ESearchServer struct {
	grpc_testing.UnimplementedSearchServiceServer
}

func (transportE2ESearchServer) Search(ctx context.Context, req *grpc_testing.SearchRequest) (*grpc_testing.SearchResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	authOK := "authorization:missing"
	if values := md.Get("authorization"); len(values) > 0 && values[0] == "Bearer owner-token" {
		authOK = "authorization:ok"
	}
	return &grpc_testing.SearchResponse{
		Results: []*grpc_testing.SearchResponse_Result{{
			Url:      "grpc://search/" + req.GetQuery(),
			Title:    req.GetQuery(),
			Snippets: []string{authOK},
		}},
	}, nil
}

func startTransportE2EGRPCUpstream(t *testing.T) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen grpc upstream: %v", err)
	}
	server := grpc.NewServer()
	grpc_testing.RegisterSearchServiceServer(server, transportE2ESearchServer{})
	reflection.Register(server)

	done := make(chan error, 1)
	go func() {
		done <- server.Serve(listener)
	}()

	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
		select {
		case err := <-done:
			if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
				t.Logf("grpc upstream stopped: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Log("grpc upstream did not stop within timeout")
		}
	}
}

func transportE2ECreateCredential(t *testing.T, client *http.Client, baseURL string, token string) string {
	t.Helper()

	response := transportE2EPostJSON(t, client, baseURL+"/api/credential", token, map[string]interface{}{
		"data": map[string]interface{}{
			"type": "credential",
			"attributes": map[string]interface{}{
				"name":    "e2e-owner-token",
				"content": `{"token":"owner-token"}`,
			},
		},
	})
	return transportE2EReferenceID(t, response)
}

func transportE2ECreateIntegration(t *testing.T, client *http.Client, baseURL string, token string, name string, spec map[string]interface{}) string {
	t.Helper()

	specBytes, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal integration spec: %v", err)
	}
	response := transportE2EPostJSON(t, client, baseURL+"/api/integration", token, map[string]interface{}{
		"data": map[string]interface{}{
			"type": "integration",
			"attributes": map[string]interface{}{
				"name":                         name,
				"specification_language":       "openapiv3",
				"specification_format":         "json",
				"specification":                string(specBytes),
				"authentication_type":          "custom_credentials",
				"authentication_specification": `{"scheme":"bearer","token_field":"token"}`,
				"enable":                       true,
			},
		},
	})
	return transportE2EReferenceID(t, response)
}

func transportE2EInstallIntegration(t *testing.T, client *http.Client, baseURL string, token string, referenceID string) {
	t.Helper()

	transportE2EPostJSON(t, client, baseURL+"/action/integration/install_integration", token, map[string]interface{}{
		"attributes": map[string]interface{}{
			"integration_id": referenceID,
		},
	})
}

func httpTransportE2ESpec(t *testing.T, serverURL string) map[string]interface{} {
	t.Helper()

	return transportE2EBaseSpec("E2E HTTP protocols", serverURL, map[string]interface{}{
		"/rest/tasks/{task_gid}": map[string]interface{}{
			"get": map[string]interface{}{
				"operationId": "getTask",
				"parameters": []map[string]interface{}{
					{"name": "task_gid", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					{"name": "opt_fields", "in": "query", "schema": map[string]interface{}{"type": "string"}},
				},
				"responses": transportE2EJSONResponses(),
			},
		},
		"/linear/listIssues": map[string]interface{}{
			"post": map[string]interface{}{
				"operationId":                     "listIssues",
				"x-daptin-transport":              "graphql",
				"x-daptin-upstream-path":          "/graphql",
				"x-daptin-graphql-operation-name": "ListIssues",
				"x-daptin-graphql-document":       "query ListIssues($first: Int!, $after: String) { issues(first: $first, after: $after) { nodes { id title } } }",
				"requestBody":                     transportE2EObjectRequestBody(map[string]interface{}{"first": map[string]interface{}{"type": "integer"}, "after": map[string]interface{}{"type": "string"}}),
				"responses":                       transportE2EJSONResponses(),
			},
		},
		"/ws/search": map[string]interface{}{
			"post": map[string]interface{}{
				"operationId":            "wsSearch",
				"x-daptin-transport":     "websocket",
				"x-daptin-upstream-path": "/ws",
				"requestBody":            transportE2EObjectRequestBody(map[string]interface{}{"query": map[string]interface{}{"type": "string"}}),
				"responses":              transportE2EJSONResponses(),
			},
		},
		"/public": map[string]interface{}{
			"get": map[string]interface{}{
				"operationId": "publicStatus",
				"security":    []interface{}{},
				"responses":   transportE2EJSONResponses(),
			},
		},
	})
}

func grpcTransportE2ESpec(t *testing.T, grpcAddress string) map[string]interface{} {
	t.Helper()

	return transportE2EBaseSpec("E2E gRPC protocols", "http://"+grpcAddress, map[string]interface{}{
		"/grpc/search": map[string]interface{}{
			"post": map[string]interface{}{
				"operationId":           "Search",
				"x-daptin-transport":    "grpc",
				"x-daptin-grpc-service": "grpc.testing.SearchService",
				"x-daptin-grpc-method":  "Search",
				"requestBody":           transportE2EObjectRequestBody(map[string]interface{}{"query": map[string]interface{}{"type": "string"}}),
				"responses":             transportE2EJSONResponses(),
			},
		},
	})
}

func transportE2EBaseSpec(title string, serverURL string, paths map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":   title,
			"version": "1.0.0",
		},
		"servers": []map[string]interface{}{{"url": serverURL}},
		"security": []map[string]interface{}{
			{"bearerAuth": []interface{}{}},
		},
		"paths": paths,
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{"type": "http", "scheme": "bearer"},
			},
		},
	}
}

func transportE2EObjectRequestBody(properties map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"required": true,
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type":       "object",
					"properties": properties,
				},
			},
		},
	}
}

func transportE2EJSONResponses() map[string]interface{} {
	return map[string]interface{}{
		"200": map[string]interface{}{
			"description": "OK",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{"type": "object"},
				},
			},
		},
	}
}

func assertTransportE2EString(t *testing.T, value interface{}, dottedPath string, want string) {
	t.Helper()
	got, ok := transportE2EPath(value, dottedPath)
	if !ok {
		t.Fatalf("missing path %s in %#v", dottedPath, value)
	}
	gotString, ok := got.(string)
	if !ok {
		t.Fatalf("path %s is %T, want string: %#v", dottedPath, got, value)
	}
	if gotString != want {
		t.Fatalf("path %s = %q, want %q; response=%#v", dottedPath, gotString, want, value)
	}
}

func transportE2EPath(value interface{}, dottedPath string) (interface{}, bool) {
	current := value
	for _, part := range strings.Split(dottedPath, ".") {
		switch typed := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = typed[part]
			if !ok {
				return nil, false
			}
		case []interface{}:
			index := -1
			_, err := fmt.Sscanf(part, "%d", &index)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func transportE2EWriteJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
