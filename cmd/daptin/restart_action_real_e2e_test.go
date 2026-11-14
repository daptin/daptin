package main

import (
	"io"
	"net/http"
	"testing"
)

func TestRestartActionAuthorizationAndConfigurationReloadRealE2E(t *testing.T) {
	f := startDaptinE2E(t, transportE2EDaptinOptions{})
	endpoint := f.URL + "/action/world/restart_daptin"
	payload := map[string]interface{}{"attributes": map[string]interface{}{}}
	// This maintenance action must be private even during bootstrap.
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, endpoint, "", payload, http.StatusForbidden)
	member := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, "", "restart-member")
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, endpoint, member, payload, http.StatusForbidden)
	admin := accessGroupsE2ESignupSigninAdmin(t, f.Client, f.URL)
	for _, caller := range []string{"", member} {
		accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, endpoint, caller, payload, http.StatusForbidden)
	}

	readLimit := func(want string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, f.URL+"/api/world", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+admin)
		response, err := f.Client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		if response.StatusCode != http.StatusOK || response.Header.Get("X-RateLimit-Limit") != want {
			t.Fatalf("world status=%d limit=%q, want 200 and %s", response.StatusCode, response.Header.Get("X-RateLimit-Limit"), want)
		}
	}
	readLimit("500")
	ftpE2ESetConfig(t, f.Client, f.URL, admin, "limit.rate", `{"version":"1","limits":{"/api/world":17}}`)
	readLimit("500") // Persisting startup configuration alone does not reload it.

	for attempt := 0; attempt < 2; attempt++ {
		result := accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost, endpoint, admin, payload, http.StatusOK)
		if responseType, _ := accessGroupsE2EFindString(result, "ResponseType"); responseType != "Restart" {
			t.Fatalf("restart response=%#v", result)
		}
		waitForAdminRestart(t, f.Client, f.URL)
		readLimit("17") // The same JWT still works, and startup read the new config.
		// The restored action survives schema initialization on every restart.
		accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "action", "action_name", "restart_daptin")
		for _, caller := range []string{"", member} {
			accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, endpoint, caller, payload, http.StatusForbidden)
		}
	}

	// Restart access follows persisted administrator membership, including revocation.
	memberID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "restart-member@test.local")
	groupID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "usergroup", "name", "administrators")
	link := map[string]interface{}{"data": []interface{}{map[string]interface{}{"type": "usergroup", "id": groupID}}}
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/user_account/"+memberID, admin, map[string]interface{}{
		"data": map[string]interface{}{"type": "user_account", "id": memberID,
			"relationships": map[string]interface{}{"usergroup_id": link}},
	}, http.StatusOK)
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, endpoint, member, payload, http.StatusOK)
	waitForAdminRestart(t, f.Client, f.URL)
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodDelete, f.URL+"/api/user_account/"+memberID+"/relationships/usergroup_id", admin, link, http.StatusNoContent)
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, endpoint, member, payload, http.StatusForbidden)
	readLimit("17")
}
