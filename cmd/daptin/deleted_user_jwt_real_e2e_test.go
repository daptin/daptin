package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestDeletedUserJWTRevocationRealE2E(t *testing.T) {
	options := transportE2EDaptinOptions{connectionString: filepath.Join(t.TempDir(), "revocation.db")}
	f := startDaptinE2E(t, options)
	admin := accessGroupsE2ESignupSigninAdmin(t, f.Client, f.URL)
	token := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "deleted-jwt")
	userID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "deleted-jwt@test.local")
	firstFrame := func(token string) map[string]interface{} {
		t.Helper()
		config, err := websocket.NewConfig("ws"+strings.TrimPrefix(f.URL, "http")+"/live", f.URL)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			config.Header.Set("Authorization", "Bearer "+token)
		}
		ws, err := websocket.DialConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		defer ws.Close()
		if err := ws.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var raw string
		if err := websocket.Message.Receive(ws, &raw); err != nil {
			t.Fatal(err)
		}
		var frame map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &frame); err != nil {
			t.Fatal(err)
		}
		return frame
	}
	identity := func(frame map[string]interface{}) string {
		t.Helper()
		content, ok := frame["data"].(string)
		if !ok {
			t.Fatalf("session has no data: %#v", frame)
		}
		data, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			t.Fatal(err)
		}
		var session map[string]interface{}
		if err := json.Unmarshal(data, &session); err != nil {
			t.Fatal(err)
		}
		t.Logf("session identity=%v groups=%v", session["user"], session["groups"])
		return session["user"].(string)
	}
	if frame := firstFrame(""); frame["message"] != "unauthorized" {
		t.Fatalf("anonymous control: %#v", frame)
	}
	t.Log("Anonymous /live control: unauthorized frame")
	if got := identity(firstFrame(token)); got != userID {
		t.Fatalf("active user identity=%s, want %s", got, userID)
	}
	assertRevoked := func(stage string) {
		t.Helper()
		for _, path := range []string{"/live", "/yjs/deleted-jwt-noncanonical-room", "/api/world"} {
			req, err := http.NewRequest(http.MethodGet, f.URL+path, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			if path == "/live" {
				req.Header.Set("Connection", "Upgrade")
				req.Header.Set("Upgrade", "websocket")
				req.Header.Set("Sec-WebSocket-Version", "13")
				req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
			}
			response, err := f.Client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("%s: %s status=%d, want 401", stage, path, response.StatusCode)
			}
			t.Logf("%s: deleted-user JWT %s = 401", stage, path)
		}
		// A valid administrator still authenticates and the old token has not recreated the account.
		if frame := firstFrame(admin); frame["type"] != "session" || frame["status"] != "open" {
			t.Fatalf("administrator control: %#v", frame)
		}
		response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet,
			f.URL+"/api/user_account?page%5Bsize%5D=100", admin, nil, http.StatusOK)
		for _, item := range accessGroupsE2EDataArray(t, response) {
			attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
			if attrs["email"] == "deleted-jwt@test.local" {
				t.Fatal("deleted account was recreated")
			}
		}
	}
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodDelete, f.URL+"/api/user_account/"+userID, admin, nil, http.StatusOK)
	assertRevoked("immediately after resource deletion")
	f.Process.stopProcess()
	f = startDaptinE2E(t, options)
	assertRevoked("after restart")
}
