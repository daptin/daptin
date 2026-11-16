package main

import (
	"fmt"
	"net/http"
	"os"
	"testing"
)

func TestJoinTableRoutesRealE2E(t *testing.T) {
	requireRealE2E(t)
	for _, backend := range []struct{ name, driver, dsn string }{
		{"sqlite", "sqlite3", ""},
		{"postgres", "postgres", os.Getenv("DAPTIN_TEST_POSTGRES_DSN")},
	} {
		t.Run(backend.name, func(t *testing.T) {
			if backend.driver == "postgres" && backend.dsn == "" {
				t.Skip("set DAPTIN_TEST_POSTGRES_DSN to an empty disposable database")
			}
			fixture := startDaptinE2E(t, transportE2EDaptinOptions{databaseType: backend.driver, connectionString: backend.dsn})
			admin := fixture.SignupAdmin(t)
			_ = accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "join-route-customer")
			account := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "user_account", "email", "join-route-customer@test.local")
			group := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "usergroup", map[string]interface{}{"name": "Join route fixture"})
			parent := fixture.URL + "/api/user_account/" + account
			related := parent + "/usergroup_id"
			linkage := parent + "/relationships/usergroup_id"
			readRelated := func() []interface{} {
				t.Helper()
				return accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, related, admin, nil, http.StatusOK))
			}
			rows := readRelated()
			retained := make([]map[string]interface{}, 0, len(rows)+1)
			for _, row := range rows {
				attrs := row.(map[string]interface{})["attributes"].(map[string]interface{})
				retained = append(retained, map[string]interface{}{"type": "usergroup", "id": attrs["relation_reference_id"]})
			}
			retained = append(retained, map[string]interface{}{"type": "usergroup", "id": group})
			accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPatch, linkage, admin, map[string]interface{}{"data": retained}, http.StatusNoContent)
			var joinID string
			for _, row := range readRelated() {
				record := row.(map[string]interface{})
				if record["attributes"].(map[string]interface{})["relation_reference_id"] == group {
					joinID = record["id"].(string)
				}
			}
			if joinID == "" {
				t.Fatal("parent relationship did not persist the group link")
			}
			// Existing row IDs must not make internal storage routes accessible either.
			for _, table := range []string{"user_account_user_account_id_has_usergroup_usergroup_id", "world_world_id_has_usergroup_usergroup_id", "collection_collection_id_has_usergroup_usergroup_id"} {
				_ = accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "world", "table_name", table)
				for _, token := range []string{"", admin} {
					for _, suffix := range []string{"", "/" + joinID, "/" + joinID + "/relationships/usergroup_id"} {
						for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete, http.MethodOptions} {
							response := davE2ERequest(fixture.Client, method, fixture.URL+"/api/"+table+suffix, token, "application/json", `{"data":{}}`, nil)
							if response.err != nil || response.status != http.StatusNotFound {
								t.Fatalf("internal route %s %s%s: %+v", method, table, suffix, response)
							}
						}
					}
				}
			}
			for _, path := range []string{related, linkage} {
				response := davE2ERequest(fixture.Client, http.MethodGet, path, "", "", "", nil)
				if response.err != nil || response.status != http.StatusForbidden {
					t.Fatalf("guest parent relationship: %+v", response)
				}
			}
			linked := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, linkage, admin, nil, http.StatusOK))
			found := false
			for _, row := range linked {
				if row.(map[string]interface{})["id"] == group {
					found = true
				}
			}
			if !found {
				t.Fatal("linkage did not expose the related group reference")
			}
			for _, method := range []string{http.MethodPatch, http.MethodDelete} {
				response := davE2ERequest(fixture.Client, method, linkage, "", "application/json", fmt.Sprintf(`{"data":[{"type":"usergroup","id":%q}]}`, group), nil)
				if response.err != nil || response.status != http.StatusForbidden {
					t.Fatalf("guest relationship mutation: %+v", response)
				}
			}
			accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodDelete, linkage, admin, map[string]interface{}{"data": []map[string]interface{}{{"type": "usergroup", "id": group}}}, http.StatusNoContent)
			for _, row := range readRelated() {
				if row.(map[string]interface{})["attributes"].(map[string]interface{})["relation_reference_id"] == group {
					t.Fatal("removed relationship remained visible")
				}
			}
			t.Log("Internal collection, record and nested routes return 404 for all methods; parent reads, linking, unlinking, and guest denial verified")
		})
	}
}
