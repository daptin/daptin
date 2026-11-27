package main

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/daptin/daptin/server/auth"
)

func TestRelationshipQueryPermissionRealE2E(t *testing.T) {
	// The first table isolates table denial; the second admits table mutations
	// so that PATCH and DELETE must be denied by the row permission checker.
	var schema string
	for _, table := range []string{"table_permission_probe", "row_permission_probe"} {
		tableGrant := auth.GroupPeek | auth.GroupRead | auth.GroupRefer
		if table == "row_permission_probe" {
			tableGrant = auth.GroupCRUD
		}
		schema += fmt.Sprintf(`
  - TableName: %s
    Permission: %d
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
    DefaultGroups:
      - Name: users
        Permission: %d
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label
`, table, auth.UserCRUD, auth.UserCRUD, tableGrant, auth.GroupPeek|auth.GroupRead|auth.GroupRefer)
	}
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: "Tables:\n" + schema})
	admin := fixture.SignupAdmin(t)
	user := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "relationship-reader")

	for _, table := range []string{"table_permission_probe", "row_permission_probe"} {
		t.Run(table, func(t *testing.T) {
			collection := fixture.URL + "/api/" + table
			id := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, table, map[string]interface{}{"title": "unchanged"})
			endpoint := collection + "/" + id
			read := func() interface{} {
				t.Helper()
				return accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, endpoint, admin, nil, http.StatusOK).(map[string]interface{})["data"]
			}
			original := read()
			// Confirm the attacker can read the row, rather than being stopped
			// before the mutation permission check by the router's initial GET.
			accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, endpoint, user, nil, http.StatusOK)
			for _, suffix := range []string{"", "?x=/relationships/", "?/relationships/", "?x=%2Frelationships%2F"} {
				t.Run("query="+suffix, func(t *testing.T) {
					accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPatch, endpoint+suffix, user,
						accessGroupsE2ERecordPayload(table, id, map[string]interface{}{"title": "unauthorized", "permission": int64(auth.ALLOW_ALL_PERMISSIONS)}), http.StatusForbidden)
					accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodDelete, endpoint+suffix, user, nil, http.StatusForbidden)
					if table == "table_permission_probe" {
						accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, collection+suffix, user,
							accessGroupsE2ERecordPayload(table, "", map[string]interface{}{"title": "unauthorized"}), http.StatusForbidden)
					}
					if current := read(); !reflect.DeepEqual(current, original) {
						t.Fatalf("denied request changed the record: before=%#v after=%#v", original, current)
					}
					rows := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, collection, admin, nil, http.StatusOK))
					if len(rows) != 1 {
						t.Fatalf("denied request changed collection size: %#v", rows)
					}
				})
			}

			// Genuine relationship PATCH and DELETE routes must still change links.
			group := accessGroupsE2ECreateUsergroup(t, fixture.Client, fixture.URL, admin, table+"-group")
			linkage := map[string]interface{}{"data": []map[string]interface{}{{"type": "usergroup", "id": group}}}
			relationshipURL := endpoint + "/relationships/usergroup_id?x=/relationships/"
			// DefaultGroups preserves the users grant on this resource.
			defaultLinks := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
				endpoint+"/relationships/usergroup_id", admin, nil, http.StatusOK))
			for _, method := range []string{http.MethodPatch, http.MethodDelete} {
				accessGroupsE2ERequestJSON(t, fixture.Client, method, relationshipURL, admin, linkage, http.StatusNoContent)
				linked := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
					endpoint+"/relationships/usergroup_id", admin, nil, http.StatusOK))
				found := false
				for _, link := range linked {
					found = found || link.(map[string]interface{})["id"] == group
				}
				want := len(defaultLinks)
				if method != http.MethodDelete {
					want++
				}
				if len(linked) != want || found != (method != http.MethodDelete) {
					t.Fatalf("%s relationship result = %#v", method, linked)
				}
			}

			// Table mutation permission and owner permission together still allow
			// ordinary CRUD, even with the previously malicious query attached.
			writer := admin
			if table == "row_permission_probe" {
				writer = user
			}
			owned := accessGroupsE2EResourceID(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, collection+"?x=/relationships/", writer,
				accessGroupsE2ERecordPayload(table, "", map[string]interface{}{"title": "owned"}), http.StatusCreated))
			ownedURL := collection + "/" + owned + "?x=/relationships/"
			accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPatch, ownedURL, writer,
				accessGroupsE2ERecordPayload(table, owned, map[string]interface{}{"title": "authorized"}), http.StatusOK)
			updated := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, ownedURL, admin, nil, http.StatusOK)
			if attrs := updated.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{}); attrs["title"] != "authorized" {
				t.Fatalf("authorized update was not persisted: %#v", updated)
			}
			accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodDelete, ownedURL, writer, nil, http.StatusOK)
			accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, collection+"/"+owned, admin, nil, http.StatusNotFound)
		})
	}
}
