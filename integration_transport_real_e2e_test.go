package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	grpc_testing "google.golang.org/grpc/reflection/grpc_testing"
)

func TestRealIntegrationTransportE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run the real Daptin integration transport e2e")
	}

	httpUpstream := startTransportE2EHTTPUpstream(t)
	defer httpUpstream.Close()

	grpcAddress, stopGRPC := startTransportE2EGRPCUpstream(t)
	defer stopGRPC()

	port := freeTransportE2EPort(t)
	httpsPort := freeTransportE2EPort(t)
	daptinBaseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	stopDaptin := startTransportE2EDaptin(t, port, httpsPort, daptinBaseURL)
	defer stopDaptin()

	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := transportE2ESignupSigninAdmin(t, client, daptinBaseURL)
	credentialRef := transportE2ECreateCredential(t, client, daptinBaseURL, adminToken)

	httpIntegrationRef := transportE2ECreateIntegration(t, client, daptinBaseURL, adminToken, "e2e-http-protocols", httpTransportE2ESpec(t, httpUpstream.URL))
	transportE2EInstallIntegration(t, client, daptinBaseURL, adminToken, httpIntegrationRef)

	grpcIntegrationRef := transportE2ECreateIntegration(t, client, daptinBaseURL, adminToken, "e2e-grpc-protocols", grpcTransportE2ESpec(t, grpcAddress))
	transportE2EInstallIntegration(t, client, daptinBaseURL, adminToken, grpcIntegrationRef)

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

func startTransportE2EDaptin(t *testing.T, port int, httpsPort int, baseURL string) func() {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "storage"), 0o755); err != nil {
		t.Fatalf("create daptin storage: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	logs := &lockedTransportE2EBuffer{}
	cmd := exec.CommandContext(ctx, "go", "run", ".",
		"-port", fmt.Sprintf(":%d", port),
		"-https_port", fmt.Sprintf(":%d", httpsPort),
		"-db_type", "sqlite3",
		"-db_connection_string", filepath.Join(tmpDir, "daptin.db"),
		"-local_storage_path", filepath.Join(tmpDir, "storage"),
		"-runtime", "test",
		"-log_level", "error",
	)
	cmd.Stdout = logs
	cmd.Stderr = logs
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start daptin: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			cancel()
			t.Fatalf("daptin exited before readiness: %v\n%s", err, logs.String())
		default:
		}
		resp, err := http.Get(baseURL + "/api/world")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return func() {
					if cmd.Process != nil {
						_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
					}
					select {
					case <-done:
					case <-time.After(10 * time.Second):
						if cmd.Process != nil {
							_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
						}
						<-done
					}
					cancel()
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	cancel()
	t.Fatalf("daptin did not become ready\n%s", logs.String())
	return func() {}
}

func transportE2ESignupSigninAdmin(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()

	email := fmt.Sprintf("admin-%d@test.local", time.Now().UnixNano())
	password := "testpass123"
	transportE2EPostJSON(t, client, baseURL+"/action/user_account/signup", "", map[string]interface{}{
		"attributes": map[string]interface{}{
			"email":           email,
			"password":        password,
			"passwordConfirm": password,
			"name":            "E2E Admin",
		},
	})

	signin := transportE2EPostJSON(t, client, baseURL+"/action/user_account/signin", "", map[string]interface{}{
		"attributes": map[string]interface{}{
			"email":    email,
			"password": password,
		},
	})
	token, ok := transportE2EFindString(signin, "value")
	if !ok || token == "" {
		t.Fatalf("signin response did not include token: %#v", signin)
	}

	transportE2EPostJSON(t, client, baseURL+"/action/world/become_an_administrator", token, map[string]interface{}{})
	return token
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

func transportE2EPostJSON(t *testing.T, client *http.Client, url string, token string, payload interface{}) interface{} {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if strings.Contains(url, "/api/") {
		req.Header.Set("Content-Type", "application/vnd.api+json")
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return transportE2EDoJSON(t, client, req)
}

func transportE2EGetJSON(t *testing.T, client *http.Client, url string, token string) interface{} {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return transportE2EDoJSON(t, client, req)
}

func transportE2EDoJSON(t *testing.T, client *http.Client, req *http.Request) interface{} {
	t.Helper()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", req.Method, req.URL.String(), err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		t.Fatalf("%s %s returned %d: %s", req.Method, req.URL.String(), resp.StatusCode, string(body))
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	var decoded interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("%s %s returned invalid JSON: %v\n%s", req.Method, req.URL.String(), err, string(body))
	}
	return decoded
}

func transportE2EReferenceID(t *testing.T, response interface{}) string {
	t.Helper()
	if ref, ok := transportE2EFindString(response, "reference_id"); ok && ref != "" {
		return ref
	}
	t.Fatalf("response did not include reference_id: %#v", response)
	return ""
}

func transportE2EFindString(value interface{}, key string) (string, bool) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for k, v := range typed {
			if strings.EqualFold(k, key) {
				if str, ok := v.(string); ok {
					return str, true
				}
			}
			if str, ok := transportE2EFindString(v, key); ok {
				return str, true
			}
		}
	case []interface{}:
		for _, item := range typed {
			if str, ok := transportE2EFindString(item, key); ok {
				return str, true
			}
		}
	}
	return "", false
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

func freeTransportE2EPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

type lockedTransportE2EBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedTransportE2EBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedTransportE2EBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
