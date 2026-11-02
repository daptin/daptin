package main

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/daptin/daptin/server/auth"
)

func TestUserDirectoryPrivateAfterOnboardingRealE2E(t *testing.T) {
	options := transportE2EDaptinOptions{connectionString: filepath.Join(t.TempDir(), "directory.db")}
	f := startDaptinE2E(t, options)
	admin := accessGroupsE2ESignupSigninAdmin(t, f.Client, f.URL)
	t.Log("Completed become_an_administrator with HTTP 200")
	f.Process.stopProcess()
	f = startDaptinE2E(t, options)
	t.Log("Restarted server against the same database after administrator onboarding")
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, f.URL+"/action/user_account/signup", "",
		map[string]interface{}{"attributes": map[string]interface{}{
			"email": "blocked@test.local", "password": "testpass123", "passwordConfirm": "testpass123", "name": "blocked",
		}}, http.StatusForbidden)
	t.Log("Unauthenticated signup after onboarding: HTTP 403")
	alice := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "local-alice")
	bob := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "local-bob")
	aliceID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "local-alice@test.local")
	bobID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "local-bob@test.local")
	for _, check := range []struct{ name, token, id string }{
		{"Alice reads self", alice, aliceID},
		{"Admin reads Bob", admin, bobID},
		{"Bob reads self", bob, bobID},
		{"Admin reads Alice", admin, aliceID},
	} {
		response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet,
			f.URL+"/api/user_account/"+check.id, check.token, nil, http.StatusOK)
		data := response.(map[string]interface{})["data"].(map[string]interface{})
		if data["id"] != check.id {
			t.Fatalf("unexpected account: %#v", data)
		}
		attrs := data["attributes"].(map[string]interface{})
		t.Logf("%s: HTTP 200, id=%s email=%v name=%v confirmed=%v owner=%v permission=%v", check.name, check.id,
			attrs["email"], attrs["name"], attrs["confirmed"], attrs["user_account_id"], attrs["permission"])
	}
	for _, check := range []struct{ name, token, ownID, otherID string }{
		{"Alice", alice, aliceID, bobID},
		{"Bob", bob, bobID, aliceID},
	} {
		accessGroupsE2EAssertStatus(t, f.Client, http.MethodGet,
			f.URL+"/api/user_account/"+check.otherID, check.token, nil, http.StatusForbidden)
		response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet,
			f.URL+"/api/user_account?page%5Bsize%5D=100", check.token, nil, http.StatusOK)
		rows := accessGroupsE2EDataArray(t, response)
		foundSelf := false
		for _, item := range rows {
			row := item.(map[string]interface{})
			if row["id"] == check.ownID {
				foundSelf = true
				continue
			}
			// The built-in guest account has its own persisted bootstrap grant.
			if row["attributes"].(map[string]interface{})["email"] != "guest@cms.go" {
				t.Fatalf("%s can list another account: %#v", check.name, row)
			}
		}
		if !foundSelf {
			t.Fatalf("%s cannot list their own account", check.name)
		}
		t.Logf("%s: own account listed; other account GET denied with HTTP 403", check.name)
	}
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/user_account/"+bobID, alice,
		accessGroupsE2ERecordPayload("user_account", bobID, map[string]interface{}{"name": "unauthorized edit"}), http.StatusForbidden)
	t.Log("Alice updating Bob: HTTP 403 (read permission did not grant update)")
	groupID := accessGroupsE2ECreateUsergroup(t, f.Client, f.URL, admin, "explicit-account-sharing")
	const joins = "user_account_user_account_id_has_usergroup_usergroup_id"
	accessGroupsE2ECreateJoin(t, f.Client, f.URL, admin, joins,
		map[string]interface{}{"user_account_id": aliceID, "usergroup_id": groupID}, 0)
	grantID := accessGroupsE2ECreateJoin(t, f.Client, f.URL, admin, joins,
		map[string]interface{}{"user_account_id": bobID, "usergroup_id": groupID}, int64(auth.GroupRead))
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodGet, f.URL+"/api/user_account/"+bobID, alice, nil, http.StatusOK)
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/"+joins+"/"+grantID, admin,
		accessGroupsE2ERecordPayload(joins, grantID, map[string]interface{}{"permission": 0}), http.StatusOK)
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodGet, f.URL+"/api/user_account/"+bobID, alice, nil, http.StatusForbidden)
	t.Log("Explicit group sharing grants read; revoking it immediately denies read")
}
