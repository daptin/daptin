package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/daptin/daptin/server/auth"
)

func TestActionPatchVersionRealE2E(t *testing.T) {
	const entity = "patch_version_probe"
	patch := func(reference, phase string) map[string]interface{} {
		return map[string]interface{}{"Type": entity, "Method": "PATCH", "Attributes": map[string]interface{}{"reference_id": reference, "phase": phase}}
	}
	fields := []map[string]interface{}{}
	for name, kind := range map[string]string{"patch_version_probe_id": "label", "phase": "label", "expected_version": "measurement", "marker_id": "label", "updates": "json"} {
		fields = append(fields, map[string]interface{}{"Name": name, "ColumnName": name, "ColumnType": kind, "IsNullable": true})
	}
	actions := []map[string]interface{}{}
	for _, name := range []string{"with_version", "without_version", "foreach_version"} {
		target := patch("~patch_version_probe_id", "~phase")
		if name != "without_version" {
			target["Attributes"].(map[string]interface{})["version"] = "~expected_version"
		}
		if name == "foreach_version" {
			target = patch("~item.reference_id", "~item.phase")
			target["Attributes"].(map[string]interface{})["version"] = "~item.version"
			target["ForEach"] = "~updates"
			target["MaxItems"] = 3
		}
		actions = append(actions, map[string]interface{}{
			"Name": name, "OnType": entity, "InstanceOptional": false, "Permission": 0,
			"AccessGroups": []map[string]interface{}{{"Name": "users", "Permission": auth.GroupExecute}},
			"InFields":     fields,
			"OutFields": []map[string]interface{}{
				patch("~marker_id", "before"), target, patch("~marker_id", "after"),
				{"Type": "client.notify", "Method": "ACTIONRESPONSE", "Attributes": map[string]interface{}{"message": "completed"}},
			},
		})
	}
	schema, err := json.Marshal(map[string]interface{}{
		"EnableGraphQL": true,
		"Tables": []map[string]interface{}{{
			"TableName": entity, "Permission": auth.GroupCRUD | auth.GroupExecute,
			"DefaultPermission": auth.UserCRUD | auth.UserExecute,
			"AccessGroups":      []map[string]interface{}{{"Name": "users", "Permission": auth.GroupCRUD | auth.GroupExecute}},
			"Columns":           []map[string]interface{}{{"Name": "phase", "ColumnName": "phase", "ColumnType": "label", "DataType": "varchar(255)"}},
		}}, "Actions": actions,
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: string(schema)})
	admin := fixture.SignupAdmin(t)
	user := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "patch-version-owner")
	other := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "patch-version-other")
	create := func(t *testing.T) string {
		return accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, user, entity, map[string]interface{}{"phase": "queued"})
	}
	recordURL := func(ref string) string { return fixture.URL + "/api/" + entity + "/" + ref }
	read := func(t *testing.T, ref, phase string, version float64) {
		t.Helper()
		response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, recordURL(ref), admin, nil, http.StatusOK)
		attrs := response.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["phase"] != phase || attrs["version"] != version {
			t.Fatalf("record %s attributes = %v, want phase=%s version=%v", ref, attrs, phase, version)
		}
	}
	invoke := func(t *testing.T, token, name, ref, marker string, version interface{}, status int, extra map[string]interface{}) interface{} {
		t.Helper()
		attrs := map[string]interface{}{entity + "_id": ref, "marker_id": marker, "phase": "running"}
		if name != "without_version" && name != "foreach_version" {
			attrs["expected_version"] = version
		}
		for k, v := range extra {
			attrs[k] = v
		}
		return accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/"+entity+"/"+name, token, map[string]interface{}{"attributes": attrs}, status)
	}
	t.Run("current version persists and later outcomes run", func(t *testing.T) {
		ref, marker := create(t), create(t)
		response := invoke(t, user, "with_version", ref, marker, 1, http.StatusOK, nil)
		if message, ok := accessGroupsE2EFindString(response, "message"); !ok || message != "completed" {
			t.Fatalf("missing final outcome: %v", response)
		}
		read(t, ref, "running", 2)
		read(t, marker, "after", 3)
		// A repeated same-value update must still reject a stale precondition.
		invoke(t, user, "with_version", ref, marker, 1, http.StatusConflict, nil)
		read(t, ref, "running", 2)
		read(t, marker, "after", 3)
	})
	t.Run("stale version rolls back earlier outcomes", func(t *testing.T) {
		ref, marker := create(t), create(t)
		invoke(t, user, "with_version", ref, marker, 99, http.StatusConflict, nil)
		read(t, ref, "queued", 1)
		read(t, marker, "queued", 1)
	})
	t.Run("invalid versions are rejected", func(t *testing.T) {
		for _, version := range []interface{}{0, -1, 1.5} {
			ref, marker := create(t), create(t)
			invoke(t, user, "with_version", ref, marker, version, http.StatusUnprocessableEntity, nil)
			read(t, ref, "queued", 1)
			read(t, marker, "queued", 1)
		}
	})
	t.Run("unversioned action still persists", func(t *testing.T) {
		ref, marker := create(t), create(t)
		invoke(t, user, "without_version", ref, marker, nil, http.StatusOK, nil)
		read(t, ref, "running", 2)
		read(t, marker, "after", 3)
	})
	t.Run("foreach shares version checks and transaction", func(t *testing.T) {
		first, second, marker := create(t), create(t), create(t)
		updates := []map[string]interface{}{{"reference_id": first, "phase": "first", "version": 1}, {"reference_id": second, "phase": "second", "version": 99}}
		extra := map[string]interface{}{"updates": updates}
		invoke(t, user, "foreach_version", first, marker, nil, http.StatusConflict, extra)
		read(t, first, "queued", 1)
		read(t, second, "queued", 1)
		read(t, marker, "queued", 1)
		updates[1]["version"] = 1
		invoke(t, user, "foreach_version", first, marker, nil, http.StatusOK, extra)
		read(t, first, "first", 2)
		read(t, second, "second", 2)
		read(t, marker, "after", 3)
	})
	t.Run("GraphQL action shares persistence and conflict rollback", func(t *testing.T) {
		ref, marker := create(t), create(t)
		query := fmt.Sprintf(`mutation { executeWithVersionOnPatchVersionProbe(patch_version_probe_id: %q, marker_id: %q, phase: "running", expected_version: 1) { ResponseType } }`, ref, marker)
		response := transportE2EPostJSON(t, fixture.Client, fixture.URL+"/graphql", user, map[string]interface{}{"query": query})
		if transportE2EGraphQLHasErrors(response) {
			t.Fatalf("GraphQL action failed: %v", response)
		}
		read(t, ref, "running", 2)
		read(t, marker, "after", 3)
		response = transportE2EPostJSON(t, fixture.Client, fixture.URL+"/graphql", user, map[string]interface{}{"query": query})
		if !transportE2EGraphQLHasErrors(response) {
			t.Fatalf("stale GraphQL action succeeded: %v", response)
		}
		read(t, ref, "running", 2)
		read(t, marker, "after", 3)
	})
	t.Run("REST version checking remains intact", func(t *testing.T) {
		ref := create(t)
		payload := accessGroupsE2ERecordPayload(entity, ref, map[string]interface{}{"phase": "rest", "version": 1})
		accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPatch, recordURL(ref), user, payload, http.StatusOK)
		read(t, ref, "rest", 2)
		accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPatch, recordURL(ref), user, payload, http.StatusConflict)
		read(t, ref, "rest", 2)
	})
	t.Run("unreadable subject and revoked execute cannot update", func(t *testing.T) {
		ref, marker := create(t), create(t)
		invoke(t, other, "with_version", ref, marker, 1, http.StatusBadRequest, nil)
		read(t, ref, "queued", 1)
		read(t, marker, "queued", 1)
		payload := accessGroupsE2ERecordPayload(entity, ref, map[string]interface{}{"permission": auth.UserCRUD})
		accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPatch, recordURL(ref), admin, payload, http.StatusOK)
		invoke(t, user, "with_version", ref, marker, 2, http.StatusForbidden, nil)
		read(t, ref, "queued", 2)
		read(t, marker, "queued", 1)
	})
}
