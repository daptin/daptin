package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// This file owns process and HTTP setup shared by cmd/daptin E2E tests.

func requireRealE2E(t testing.TB) {
	t.Helper()
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run end-to-end tests")
	}
}

type daptinE2EFixture struct {
	URL     string
	Client  *http.Client
	Process *transportE2EDaptinProcess
}

// startDaptinE2E provides an isolated server for a single test. Tests that
// restart a server or form a cluster can use startTransportE2EDaptin directly.
func startDaptinE2E(t testing.TB, options transportE2EDaptinOptions) *daptinE2EFixture {
	t.Helper()
	requireRealE2E(t)
	usedPorts := make(map[int]bool, 4)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	options.olricPort = freeTransportE2EPortPair(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	process := startTransportE2EDaptin(t, port, httpsPort, baseURL, options)
	return &daptinE2EFixture{
		URL: baseURL, Client: &http.Client{Timeout: 20 * time.Second}, Process: process,
	}
}

func (fixture *daptinE2EFixture) SignupAdmin(t testing.TB) string {
	t.Helper()
	return transportE2ESignupSigninAdmin(t, fixture.Client, fixture.URL)
}

type transportE2EDaptinOptions struct {
	databaseType     string
	connectionString string
	olricPort        int
	olricPeers       string
	schema           string
}

type transportE2EDaptinProcess struct {
	processGroupID int
	stop           func()
	stopOnce       sync.Once
	logs           *lockedTransportE2EBuffer
}

func (process *transportE2EDaptinProcess) stopProcess() {
	process.stopOnce.Do(process.stop)
}

func startTransportE2EDaptin(t testing.TB, port int, httpsPort int, baseURL string, requested ...transportE2EDaptinOptions) *transportE2EDaptinProcess {
	t.Helper()
	if len(requested) > 1 {
		t.Fatal("start Daptin E2E accepts at most one options value")
	}

	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "storage"), 0o755); err != nil {
		t.Fatalf("create daptin storage: %v", err)
	}
	options := transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: filepath.Join(tmpDir, "daptin.db"),
	}
	if len(requested) == 1 {
		provided := requested[0]
		if provided.databaseType != "" {
			options.databaseType = provided.databaseType
		}
		if provided.connectionString != "" {
			options.connectionString = provided.connectionString
		}
		options.olricPort = provided.olricPort
		options.olricPeers = provided.olricPeers
		options.schema = provided.schema
	}
	if options.databaseType == "" || options.connectionString == "" {
		t.Fatal("Daptin E2E database type and connection string are required")
	}
	if options.schema != "" {
		if err := os.WriteFile(filepath.Join(tmpDir, "schema_transport_e2e.yaml"), []byte(options.schema), 0o600); err != nil {
			t.Fatalf("write Daptin E2E schema: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	logs := &lockedTransportE2EBuffer{}
	arguments := []string{"run", "./cmd/daptin",
		"-port", fmt.Sprintf(":%d", port),
		"-https_port", fmt.Sprintf(":%d", httpsPort),
		"-db_type", options.databaseType,
		"-db_connection_string", options.connectionString,
		"-local_storage_path", filepath.Join(tmpDir, "storage"),
		"-runtime", "test",
		"-log_level", "error",
	}
	if options.olricPort > 0 {
		arguments = append(arguments, "-olric_port", fmt.Sprint(options.olricPort))
	}
	if options.olricPeers != "" {
		arguments = append(arguments, "-olric_peers", options.olricPeers)
	}
	cmd := exec.CommandContext(ctx, "go", arguments...)
	if options.schema != "" {
		cmd.Env = transportE2EEnvironment("DAPTIN_SCHEMA_FOLDER", tmpDir)
	}
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
				process := &transportE2EDaptinProcess{processGroupID: cmd.Process.Pid, logs: logs, stop: func() {
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
				}}
				t.Cleanup(process.stopProcess)
				return process
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	cancel()
	t.Fatalf("daptin did not become ready\n%s", logs.String())
	return nil
}

func transportE2EEnvironment(name, value string) []string {
	prefix := name + "="
	environment := os.Environ()
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}

func transportE2ESignupSigninAdmin(t testing.TB, client *http.Client, baseURL string) string {
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

func transportE2EPostJSON(t testing.TB, client *http.Client, url string, token string, payload interface{}) interface{} {
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

func transportE2EGetJSON(t testing.TB, client *http.Client, url string, token string) interface{} {
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

func transportE2EDoJSON(t testing.TB, client *http.Client, req *http.Request) interface{} {
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

func transportE2EReferenceID(t testing.TB, response interface{}) string {
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

func freeTransportE2EPort(t testing.TB, used map[int]bool) int {
	t.Helper()
	for attempt := 0; attempt < 100; attempt++ {
		listener, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()
		if !used[port] {
			used[port] = true
			return port
		}
	}
	t.Fatal("allocate distinct Daptin E2E port")
	return 0
}

func freeTransportE2EPortPair(t testing.TB, used map[int]bool) int {
	t.Helper()
	for attempt := 0; attempt < 100; attempt++ {
		first, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			t.Fatal(err)
		}
		port := first.Addr().(*net.TCPAddr).Port
		if port == 65535 || used[port] || used[port+1] {
			_ = first.Close()
			continue
		}
		second, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", fmt.Sprint(port+1)))
		_ = first.Close()
		if err != nil {
			continue
		}
		_ = second.Close()
		used[port] = true
		used[port+1] = true
		return port
	}
	t.Fatal("allocate adjacent Daptin E2E ports")
	return 0
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
