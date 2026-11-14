package main

import (
	"io"
	"net/http"
	"testing"
)

func TestStatisticsAdministratorAccessRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{})
	base, client := fixture.URL, fixture.Client
	assertDenied := func(token string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, base+"/statistics", nil)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusForbidden || len(body) != 0 {
			t.Fatalf("denied statistics: status=%d body=%s", resp.StatusCode, body)
		}
	}
	assertAllowed := func(token string) {
		t.Helper()
		stats := transportE2EGetJSON(t, client, base+"/statistics", token)
		for _, path := range []string{"host.info.hostname", "process.count", "db.OpenConnections", "olric.members"} {
			if _, ok := transportE2EPath(stats, path); !ok {
				t.Fatalf("administrator statistics missing %s: %#v", path, stats)
			}
		}
	}

	assertDenied("")
	assertDenied("invalid-token")
	admin := fixture.SignupAdmin(t)
	assertAllowed(admin)
	user := accessGroupsE2ESignupSigninUser(t, client, base, admin, "statistics-user")
	assertDenied(user)

	userID := accessGroupsE2EFindResourceID(t, client, base, admin, "user_account", "email", "statistics-user@test.local")
	groupID := accessGroupsE2EFindResourceID(t, client, base, admin, "usergroup", "name", "administrators")
	const membership = "user_account_user_account_id_has_usergroup_usergroup_id"
	joinID := accessGroupsE2ECreateJoin(t, client, base, admin, membership, map[string]interface{}{
		"user_account_id": userID, "usergroup_id": groupID,
	}, 0)
	assertAllowed(user)
	accessGroupsE2EAssertStatus(t, client, http.MethodDelete, base+"/api/"+membership+"/"+joinID, admin, nil, http.StatusOK)
	assertDenied(user)
	assertAllowed(admin)
}
