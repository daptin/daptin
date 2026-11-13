package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestAdminClaimRestartsAndInitializesSystemTasksRealE2E(t *testing.T) {
	f := startDaptinE2E(t, transportE2EDaptinOptions{})
	assertReady := func() {
		t.Helper()
		response, err := f.Client.Get(f.URL + "/ready")
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("readiness=%d, want 200", response.StatusCode)
		}
	}
	claimURL := f.URL + "/action/world/become_an_administrator"
	denied, err := f.Client.Post(claimURL, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	denied.Body.Close()
	if denied.StatusCode < 400 {
		t.Fatalf("anonymous administrator claim returned %d", denied.StatusCode)
	}
	assertReady()

	admin := accessGroupsE2ESignupSigninAdmin(t, f.Client, f.URL) // Waits for automatic restart.
	adminID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "admin@test.local")
	for _, name := range []string{"__daptin_outbox_process", "__daptin_calendar_delivery", "process-data-exchange-executions"} {
		id := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "task", "name", name)
		row := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/api/task/"+id, admin, nil, http.StatusOK)
		attrs := row.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["as_user_id"] != adminID {
			t.Fatalf("task %s identity=%v, want administrator %s", name, attrs["as_user_id"], adminID)
		}
	}

	// A second claim must neither grant access nor trigger another restart.
	response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost, claimURL, admin, map[string]interface{}{}, http.StatusInternalServerError)
	if message, _ := transportE2EFindString(response, "message"); message != "Unauthorized" {
		t.Fatalf("second administrator claim was not denied: %#v", response)
	}
	assertReady()
	accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "task", "name", "__daptin_outbox_process")
}
