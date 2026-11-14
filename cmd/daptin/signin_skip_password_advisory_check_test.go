package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestSigninSkipPasswordInjectionAdvisoryRealE2E(t *testing.T) {
	options := transportE2EDaptinOptions{connectionString: filepath.Join(t.TempDir(), "signin.db")}
	f := startDaptinE2E(t, options)
	admin := accessGroupsE2ESignupSigninAdmin(t, f.Client, f.URL)
	accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "signin-victim")
	f.Process.stopProcess()
	f = startDaptinE2E(t, options)
	for _, email := range []string{"admin@test.local", "signin-victim@test.local"} {
		for _, attempt := range []struct {
			name, query, contentType, body string
		}{
			{"reported top-level empty password", "", "application/json", `{"email":%q,"password":"","skipPasswordCheck":true}`},
			{"top-level wrong password", "", "application/json", `{"email":%q,"password":"incorrect-password","skipPasswordCheck":true}`},
			{"nested wrong password", "", "application/json", `{"attributes":{"email":%q,"password":"incorrect-password","skipPasswordCheck":true}}`},
			{"mixed top-level flag", "", "application/json", `{"attributes":{"email":%q,"password":"incorrect-password"},"skipPasswordCheck":true}`},
			{"query flag", "?skipPasswordCheck=true", "application/json", `{"attributes":{"email":%q,"password":"incorrect-password"}}`},
			{"form flag", "", "application/x-www-form-urlencoded", ""},
		} {
			var body string
			if attempt.contentType == "application/x-www-form-urlencoded" {
				body = url.Values{"email": {email}, "password": {"incorrect-password"}, "skipPasswordCheck": {"true"}}.Encode()
			} else {
				encoded, _ := json.Marshal(email)
				body = strings.Replace(attempt.body, "%q", string(encoded), 1)
			}
			req, err := http.NewRequest(http.MethodPost, f.URL+"/action/user_account/signin"+attempt.query, strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", attempt.contentType)
			response, err := f.Client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			var decoded interface{}
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("invalid signin response: status=%d", response.StatusCode)
			}
			if token, ok := transportE2EFindString(decoded, "value"); ok && token != "" {
				t.Fatalf("%s minted a token for %s", attempt.name, email)
			}
			if strings.Contains(response.Header.Get("Set-Cookie"), "token=") {
				t.Fatalf("%s set a token cookie", attempt.name)
			}
			t.Logf("%s / %s: HTTP %d, no token", email, attempt.name, response.StatusCode)
		}
		response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost, f.URL+"/action/user_account/signin", "",
			map[string]interface{}{"attributes": map[string]interface{}{"email": email, "password": "testpass123", "skipPasswordCheck": true}}, http.StatusOK)
		token, ok := transportE2EFindString(response, "value")
		if !ok || token == "" {
			t.Fatal("correct password control produced no token")
		}
		accessGroupsE2EAssertStatus(t, f.Client, http.MethodGet, f.URL+"/api/user_account/mine", token, nil, http.StatusOK)
		t.Logf("%s: correct password returns a usable token", email)
	}
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, f.URL+"/action/user_account/jwt.token", "",
		map[string]interface{}{"email": "admin@test.local", "skipPasswordCheck": true}, http.StatusNotFound)
	t.Log("Direct jwt.token action URL: HTTP 404")
}
