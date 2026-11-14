package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// These tests use the production image and the shared E2E HTTP/resource helpers.
// Build the candidate image first, then set DAPTIN_E2E_DOCKER_IMAGE and DAPTIN_REAL_E2E=1.
func dockerRestartCommand(t *testing.T, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func startDockerRestartE2E(t *testing.T, database string, hardened bool, delay string) (*daptinE2EFixture, string) {
	t.Helper()
	requireRealE2E(t)
	image := os.Getenv("DAPTIN_E2E_DOCKER_IMAGE")
	if image == "" {
		t.Skip("set DAPTIN_E2E_DOCKER_IMAGE to a locally built candidate image")
	}
	name := fmt.Sprintf("daptin-admin-restart-%d", time.Now().UnixNano())
	var network, backend string
	t.Cleanup(func() {
		if t.Failed() {
			output, _ := exec.Command("docker", "logs", "--tail", "100", name).CombinedOutput()
			t.Logf("container logs:\n%s", output)
		}
		dockerRestartCommand(t, "rm", "-f", "-v", name)
		if backend != "" {
			dockerRestartCommand(t, "rm", "-f", "-v", backend)
		}
		if network != "" {
			dockerRestartCommand(t, "network", "rm", network)
		}
	})
	args := []string{"run", "-d", "--name", name, "--restart=no", "-v", "/var/lib/daptin"}
	if hardened {
		args = append(args, "--init", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges",
			"--tmpfs", "/tmp:rw,nosuid,nodev", "--tmpfs", "/var/cache/daptin:rw,uid=10001,gid=10001")
	}
	dbArgs := []string{}
	if database != "sqlite3" {
		network, backend = name+"-net", name+"-db"
		dockerRestartCommand(t, "network", "create", network)
		backendArgs := []string{"run", "-d", "--name", backend, "--network", network}
		if database == "postgres" {
			backendArgs = append(backendArgs, "-e", "POSTGRES_USER=daptin", "-e", "POSTGRES_PASSWORD=test-only", "-e", "POSTGRES_DB=daptin", "postgres:17-bookworm")
			dbArgs = []string{"-db_type", "postgres", "-db_connection_string", "host=" + backend + " port=5432 user=daptin password=test-only dbname=daptin sslmode=disable"}
		} else {
			backendArgs = append(backendArgs, "-e", "MARIADB_ROOT_PASSWORD=test-only", "-e", "MARIADB_USER=daptin", "-e", "MARIADB_PASSWORD=test-only", "-e", "MARIADB_DATABASE=daptin", "mariadb:10.11")
			dbArgs = []string{"-db_type", "mysql", "-db_connection_string", "daptin:test-only@tcp(" + backend + ":3306)/daptin"}
		}
		dockerRestartCommand(t, backendArgs...)
		deadline := time.Now().Add(time.Minute)
		ready := false
		for time.Now().Before(deadline) {
			var check *exec.Cmd
			if database == "postgres" {
				check = exec.Command("docker", "exec", backend, "pg_isready", "-U", "daptin")
			} else {
				check = exec.Command("docker", "exec", backend, "healthcheck.sh", "--connect", "--innodb_initialized")
			}
			if check.Run() == nil {
				ready = true
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
		if !ready {
			t.Fatal("database did not become ready")
		}
		args = append(args, "--network", network)
	}
	port := freeTransportE2EPort(t, map[int]bool{})
	args = append(args, "-p", fmt.Sprintf("127.0.0.1:%d:8080", port), image, "-shutdown_readiness_delay", delay)
	args = append(args, dbArgs...)
	dockerRestartCommand(t, args...)
	f := &daptinE2EFixture{URL: fmt.Sprintf("http://127.0.0.1:%d", port), Client: &http.Client{Timeout: 10 * time.Second}}
	waitDockerRestartReady(t, f)
	return f, name
}

func waitDockerRestartReady(t *testing.T, f *daptinE2EFixture) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		response, err := f.Client.Get(f.URL + "/ready")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("container did not become ready")
}

func assertDockerRestartTasks(t *testing.T, f *daptinE2EFixture, token string) {
	t.Helper()
	adminID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, token, "user_account", "email", "admin@test.local")
	response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/api/task?page%5Bsize%5D=200", token, nil, http.StatusOK)
	for _, name := range []string{"__daptin_outbox_process", "__daptin_calendar_delivery", "process-data-exchange-executions"} {
		count := 0
		for _, item := range accessGroupsE2EDataArray(t, response) {
			attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
			if attrs["name"] == name {
				count++
				if attrs["as_user_id"] != adminID {
					t.Fatalf("task %s belongs to %v, want %s", name, attrs["as_user_id"], adminID)
				}
			}
		}
		if count != 1 {
			t.Fatalf("task %s count=%d, want 1", name, count)
		}
	}
}

func TestAdminRestartDockerEnvironmentsRealE2E(t *testing.T) {
	for _, tc := range []struct {
		name, database string
		hardened       bool
	}{
		{"pid1-sqlite-no-supervisor", "sqlite3", false},
		{"init-readonly-nonroot-sqlite", "sqlite3", true},
		{"pid1-postgres", "postgres", false},
		{"pid1-mariadb", "mysql", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, name := startDockerRestartE2E(t, tc.database, tc.hardened, "2s")
			before := dockerRestartCommand(t, "inspect", "-f", "{{.State.Pid}} {{.State.StartedAt}} {{.RestartCount}}", name)
			if uid := dockerRestartCommand(t, "exec", name, "id", "-u"); uid != "10001" {
				t.Fatalf("uid=%s", uid)
			}
			if !tc.hardened {
				if process := dockerRestartCommand(t, "exec", name, "cat", "/proc/1/comm"); process != "daptin" {
					t.Fatalf("PID 1=%s", process)
				}
			}
			token := accessGroupsE2ESignupSigninAdmin(t, f.Client, f.URL)
			after := dockerRestartCommand(t, "inspect", "-f", "{{.State.Pid}} {{.State.StartedAt}} {{.RestartCount}}", name)
			if after != before {
				t.Fatalf("container restarted externally: before=%s after=%s", before, after)
			}
			assertDockerRestartTasks(t, f, token)
			// Rejected claims must not restart a running deployment.
			response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost, f.URL+"/action/world/become_an_administrator", token, map[string]interface{}{}, http.StatusInternalServerError)
			if message, _ := transportE2EFindString(response, "message"); message != "Unauthorized" {
				t.Fatalf("claim not denied: %#v", response)
			}
			logs := dockerRestartCommand(t, "logs", name)
			if count := strings.Count(logs, "Listening at: ["); count != 2 {
				t.Fatalf("startup count=%d, want 2", count)
			}
			dockerRestartCommand(t, "stop", "--time", "35", name)
			dockerRestartCommand(t, "start", name)
			waitDockerRestartReady(t, f)
			assertDockerRestartTasks(t, f, token)
			t.Log("automatic restart preserved container PID/start time, token and task ownership; persisted restart produced no duplicate tasks")
		})
	}
}

func TestAdminRestartDockerDrainsInflightRequestRealE2E(t *testing.T) {
	f, name := startDockerRestartE2E(t, "sqlite3", false, "500ms")
	token := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, "", "admin")
	address := strings.TrimPrefix(f.URL, "http://")
	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatal(err)
	}
	body := `{"attributes":{"email":"admin@test.local","password":"testpass123"}}`
	_, err = fmt.Fprintf(connection, "POST /action/user_account/signin HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", address, len(body), body[:len(body)-2])
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // Let the handler admit the request before quiescing.
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, f.URL+"/action/world/become_an_administrator", token, map[string]interface{}{}, http.StatusOK)
	// This exceeds the readiness delay, so HTTP Shutdown must wait for this request.
	time.Sleep(time.Second)
	if running := dockerRestartCommand(t, "inspect", "-f", "{{.State.Running}}", name); running != "true" {
		t.Fatal("process exited while request was active")
	}
	if _, err := io.WriteString(connection, body[len(body)-2:]); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(data), "client.store.set") {
		t.Fatalf("inflight signin: status=%d err=%v body=%s", response.StatusCode, err, data)
	}
	waitDockerRestartReady(t, f)
	assertDockerRestartTasks(t, f, token)
}

func TestAdminRestartDockerSIGTERMCancelsRestartRealE2E(t *testing.T) {
	f, name := startDockerRestartE2E(t, "sqlite3", false, "5s")
	token := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, "", "admin")
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, f.URL+"/action/world/become_an_administrator", token, map[string]interface{}{}, http.StatusOK)
	response, err := f.Client.Get(f.URL + "/ready")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("not draining: %d", response.StatusCode)
	}
	dockerRestartCommand(t, "kill", "--signal", "TERM", name)
	if code := dockerRestartCommand(t, "wait", name); code != "0" {
		t.Fatalf("shutdown exit code=%s", code)
	}
	logs := dockerRestartCommand(t, "logs", name)
	if count := strings.Count(logs, "Listening at: ["); count != 1 {
		t.Fatalf("SIGTERM allowed replacement: startup count=%d", count)
	}
	dockerRestartCommand(t, "start", name)
	waitDockerRestartReady(t, f)
	assertDockerRestartTasks(t, f, token)
}
