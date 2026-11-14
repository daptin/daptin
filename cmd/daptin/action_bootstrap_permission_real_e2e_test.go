package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/daptin/daptin/server/auth"
)

const bootstrapActionPolicySchema = `
Actions:
  - Name: bootstrap_policy_public
    OnType: world
    InstanceOptional: true
    Permission: 32
    OutFields:
      - Method: ACTIONRESPONSE
        Type: client.notify
        Attributes:
          message: public action executed
  - Name: bootstrap_policy_private
    OnType: world
    InstanceOptional: true
    Permission: 0
    OutFields:
      - Method: ACTIONRESPONSE
        Type: client.notify
        Attributes:
          message: private action executed
  - Name: bootstrap_policy_default
    OnType: world
    InstanceOptional: true
    OutFields:
      - Method: ACTIONRESPONSE
        Type: client.notify
        Attributes:
          message: bootstrap action executed
`

func TestActionLockdownAfterOnboardingRealE2E(t *testing.T) {
	options := transportE2EDaptinOptions{
		connectionString: filepath.Join(t.TempDir(), "schema.db"),
		schema:           bootstrapActionPolicySchema,
	}
	f := startDaptinE2E(t, options)
	admin := accessGroupsE2ESignupSigninAdmin(t, f.Client, f.URL)
	user := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "schema-observer")
	adminID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "admin@test.local")

	assertPolicy := func(stage string) {
		t.Helper()
		endpoint := f.URL + "/action/world/download_system_schema"
		for _, token := range []string{"", user} {
			accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, endpoint, token, map[string]interface{}{}, http.StatusForbidden)
			accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, f.URL+"/action/world/bootstrap_policy_default", token, map[string]interface{}{}, http.StatusForbidden)
			accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, f.URL+"/action/world/bootstrap_policy_private", token, map[string]interface{}{}, http.StatusForbidden)
			accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, f.URL+"/action/world/bootstrap_policy_public", token, map[string]interface{}{}, http.StatusOK)
		}
		response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost, endpoint, admin, map[string]interface{}{}, http.StatusOK)
		content, ok := transportE2EFindString(response, "content")
		if !ok {
			t.Fatal("administrator schema download has no content")
		}
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil || !json.Valid(decoded) {
			t.Fatalf("invalid schema JSON: %v", err)
		}

		restricted := int64(auth.UserRead | auth.UserExecute | auth.GroupCRUD | auth.GroupExecute | auth.GroupRefer)
		for name, want := range map[string]int64{
			"download_system_schema":   restricted,
			"bootstrap_policy_default": restricted,
			"bootstrap_policy_public":  int64(auth.GuestExecute),
			"bootstrap_policy_private": int64(auth.None),
			"signup":                   restricted,
			"signin":                   int64(auth.DEFAULT_PERMISSION),
		} {
			actionID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "action", "action_name", name)
			row := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/api/action/"+actionID, admin, nil, http.StatusOK)
			attrs := row.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
			if got := int64(attrs["permission"].(float64)); got != want {
				t.Fatalf("%s: %s permission=%d, want %d", stage, name, got, want)
			}
			if attrs["user_account_id"] != adminID {
				t.Fatalf("%s: %s owner=%v, want %s", stage, name, attrs["user_account_id"], adminID)
			}
		}
		accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, f.URL+"/action/user_account/signup", "",
			map[string]interface{}{"attributes": map[string]interface{}{
				"email": "blocked@test.local", "password": "testpass123", "passwordConfirm": "testpass123", "name": "blocked",
			}}, http.StatusForbidden)
		t.Logf("%s: bootstrap actions locked, explicit policies and admin ownership preserved, schema download admin-only", stage)
	}
	assertPolicy("after onboarding")
	f.Process.stopProcess()
	f = startDaptinE2E(t, options)
	assertPolicy("after restart")
}
