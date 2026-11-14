package main

import (
	"net/http"
	"testing"

	"github.com/daptin/daptin/server/auth"
)

// Reproduces GHSA-mw3m-ppxq-mxhc using the shipped policy, then checks
// whether revoking the persisted sharing grant actually removes access.
func TestUserDirectoryDefaultSharingRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{})
	baseURL, client := fixture.URL, fixture.Client
	admin := fixture.SignupAdmin(t)
	observer := accessGroupsE2ESignupSigninUser(t, client, baseURL, admin, "directory-observer")
	target := accessGroupsE2ESignupSigninUser(t, client, baseURL, admin, "directory-target")
	targetID := accessGroupsE2EFindResourceID(t, client, baseURL, admin, "user_account", "email", "directory-target@test.local")
	observerID := accessGroupsE2EFindResourceID(t, client, baseURL, admin, "user_account", "email", "directory-observer@test.local")

	assertVisible := func(token string, id string, want bool) {
		t.Helper()
		response := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
			baseURL+"/api/user_account?page%5Bsize%5D=100", token, nil, http.StatusOK)
		found := false
		for _, item := range accessGroupsE2EDataArray(t, response) {
			row := item.(map[string]interface{})
			if row["id"] == id {
				found = true
				attributes := row["attributes"].(map[string]interface{})
				if attributes["email"] == nil || attributes["name"] == nil || attributes["confirmed"] == nil {
					t.Fatalf("account missing reported fields: %#v", attributes)
				}
			}
		}
		if found != want {
			t.Fatalf("account %s visible=%v, want %v: %#v", id, found, want, response)
		}
	}

	assertVisible(observer, targetID, true)
	assertVisible(observer, observerID, true)
	accessGroupsE2EAssertStatus(t, client, http.MethodGet, baseURL+"/api/user_account/"+targetID, observer, nil, http.StatusOK)
	t.Log("Reproduced: a non-admin reads another account's email, name and confirmed status with unchanged defaults")

	// Row permission and relationship permission are separate authorities.
	// Removing GroupRead from the row alone does not revoke a group grant.
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/user_account/"+targetID, admin,
		accessGroupsE2ERecordPayload("user_account", targetID, map[string]interface{}{
			"permission": int64(auth.UserRead | auth.UserExecute),
		}), http.StatusOK)
	assertVisible(observer, targetID, true)
	t.Log("Removing GroupRead from the account row alone leaves relationship-based read access intact")

	const joins = "user_account_user_account_id_has_usergroup_usergroup_id"
	usersID := accessGroupsE2EFindResourceID(t, client, baseURL, admin, "usergroup", "name", "users")
	response := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		baseURL+"/api/"+joins+"?page%5Bsize%5D=100", admin, nil, http.StatusOK)
	var joinID string
	for _, item := range accessGroupsE2EDataArray(t, response) {
		row := item.(map[string]interface{})
		attributes := row["attributes"].(map[string]interface{})
		if attributes["user_account_id"] == targetID && attributes["usergroup_id"] == usersID {
			joinID = row["id"].(string)
			t.Logf("Target's shared users-group relationship permission: %v", attributes["permission"])
		}
	}
	if joinID == "" {
		t.Fatal("target has no default users-group relationship")
	}
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/"+joins+"/"+joinID, admin,
		accessGroupsE2ERecordPayload(joins, joinID, map[string]interface{}{"permission": int64(auth.None)}), http.StatusOK)
	assertVisible(observer, targetID, false)
	assertVisible(observer, observerID, true)
	assertVisible(target, targetID, true)
	assertVisible(admin, targetID, true)
	t.Log("Revoking the users-group relationship grant hides the target from the observer; self and admin access remain")
}
