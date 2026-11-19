package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/daptin/daptin/server/auth"
)

const updateTrackingRealE2ESchema = `
Tables:
  - TableName: tracking_parent
    Columns:
      - {Name: title, ColumnType: label, DataType: varchar(100)}
  - TableName: tracking_child
    Columns:
      - {Name: title, ColumnType: label, DataType: varchar(100)}
  - TableName: tracking_import
    Columns:
      - {Name: external_key, ColumnType: label, DataType: varchar(100), IsUnique: true}
      - {Name: title, ColumnType: label, DataType: varchar(100)}
      - {Name: untouched, ColumnType: label, DataType: varchar(100)}
Relations:
  - Subject: tracking_child
    Object: tracking_parent
    Relation: belongs_to
`

func TestCSVImportUpdateTrackingRealE2E(t *testing.T) {
	requireRealE2E(t)
	database := filepath.Join(t.TempDir(), "import-tracking.db")
	options := transportE2EDaptinOptions{connectionString: database, schema: updateTrackingRealE2ESchema}
	fixture := startDaptinE2E(t, options)
	admin := fixture.SignupAdmin(t)
	id := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "tracking_import", map[string]interface{}{
		"external_key": "same-key", "title": "Original", "untouched": "Keep me",
	})
	before := transportE2EGetJSON(t, fixture.Client, fixture.URL+"/api/tracking_import/"+id, admin)
	version, ok := transportE2EPath(before, "data.attributes.version")
	if !ok {
		t.Fatal("resource version missing")
	}
	fixture.Process.stopProcess()
	source := filepath.Join(t.TempDir(), "updates.csv")
	if err := os.WriteFile(source, []byte("external_key,title\nsame-key,Imported change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	options.schema += fmt.Sprintf("\nImports:\n  - Entity: tracking_import\n    FileType: csv\n    FilePath: %q\n", source)
	restarted := startDaptinE2E(t, options)
	after := transportE2EGetJSON(t, restarted.Client, restarted.URL+"/api/tracking_import/"+id, admin)
	assertTransportE2EString(t, after, "data.attributes.title", "Imported change")
	assertTransportE2EString(t, after, "data.attributes.untouched", "Keep me")
	if next, _ := transportE2EPath(after, "data.attributes.version"); next != version.(float64)+1 {
		t.Fatalf("import did not advance version: before=%v after=%v", version, next)
	}
}

func TestReverseBelongsToUpdateTrackingRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: updateTrackingRealE2ESchema})
	admin := fixture.SignupAdmin(t)
	client, base := fixture.Client, fixture.URL
	original := accessGroupsE2ECreateRecord(t, client, base, admin, "tracking_parent", map[string]interface{}{"title": "Original parent"})
	for _, operation := range []string{"create", "update"} {
		t.Run(operation, func(t *testing.T) {
			child := accessGroupsE2ECreateRecord(t, client, base, admin, "tracking_child", map[string]interface{}{"title": "Keep child title", "tracking_parent_id": original})
			before := transportE2EGetJSON(t, client, base+"/api/tracking_child/"+child, admin)
			version, ok := transportE2EPath(before, "data.attributes.version")
			if !ok {
				t.Fatal("child version missing")
			}
			attrs := map[string]interface{}{"title": "New parent", "tracking_child_id": []interface{}{map[string]interface{}{"tracking_child_id": child}}}
			var parent string
			if operation == "create" {
				parent = accessGroupsE2ECreateRecord(t, client, base, admin, "tracking_parent", attrs)
			} else {
				parent = accessGroupsE2ECreateRecord(t, client, base, admin, "tracking_parent", map[string]interface{}{"title": "Before update"})
				accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/tracking_parent/"+parent, admin, accessGroupsE2ERecordPayload("tracking_parent", parent, attrs), http.StatusOK)
			}
			after := transportE2EGetJSON(t, client, base+"/api/tracking_child/"+child, admin)
			assertTransportE2EString(t, after, "data.relationships.tracking_parent_id.data.id", parent)
			assertTransportE2EString(t, after, "data.attributes.title", "Keep child title")
			if next, _ := transportE2EPath(after, "data.attributes.version"); next != version.(float64)+1 {
				t.Fatalf("reverse relation did not advance child version: before=%v after=%v", version, next)
			}
		})
	}
	t.Run("denied-child-update", func(t *testing.T) {
		caller := accessGroupsE2ESignupSigninUser(t, client, base, admin, "tracking-caller")
		callerRef := accessGroupsE2EFindResourceID(t, client, base, admin, "user_account", "email", "tracking-caller@test.local")
		patch := func(kind, id string, attrs map[string]interface{}) {
			t.Helper()
			accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/"+kind+"/"+id, admin, accessGroupsE2ERecordPayload(kind, id, attrs), http.StatusOK)
		}
		// Table access and access to the parent do not grant access to the child.
		for _, table := range []string{"tracking_parent", "tracking_child"} {
			world := accessGroupsE2EFindResourceID(t, client, base, admin, "world", "table_name", table)
			patch("world", world, map[string]interface{}{"permission": int64(auth.GuestCRUD)})
		}
		parent := accessGroupsE2ECreateRecord(t, client, base, admin, "tracking_parent", map[string]interface{}{"title": "Before denied update"})
		patch("tracking_parent", parent, map[string]interface{}{"user_account_id": callerRef, "permission": int64(auth.UserCRUD)})
		child := accessGroupsE2ECreateRecord(t, client, base, admin, "tracking_child", map[string]interface{}{"title": "Private child", "tracking_parent_id": original, "permission": int64(auth.UserCRUD)})
		before := transportE2EGetJSON(t, client, base+"/api/tracking_child/"+child, admin)
		version, _ := transportE2EPath(before, "data.attributes.version")
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/tracking_parent/"+parent, caller,
			accessGroupsE2ERecordPayload("tracking_parent", parent, map[string]interface{}{
				"title": "Must roll back", "tracking_child_id": []interface{}{map[string]interface{}{"tracking_child_id": child}},
			}), http.StatusForbidden)
		after := transportE2EGetJSON(t, client, base+"/api/tracking_child/"+child, admin)
		assertTransportE2EString(t, after, "data.relationships.tracking_parent_id.data.id", original)
		if next, _ := transportE2EPath(after, "data.attributes.version"); next != version {
			t.Fatalf("denied relation update changed child version: before=%v after=%v", version, next)
		}
		parentAfter := transportE2EGetJSON(t, client, base+"/api/tracking_parent/"+parent, admin)
		assertTransportE2EString(t, parentAfter, "data.attributes.title", "Before denied update")
	})

}

func TestResourceUpdateTrackingControlsRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: updateTrackingRealE2ESchema})
	token := fixture.SignupAdmin(t)
	client, base := fixture.Client, fixture.URL
	first := accessGroupsE2ECreateRecord(t, client, base, token, "tracking_parent", map[string]interface{}{"title": "First"})
	second := accessGroupsE2ECreateRecord(t, client, base, token, "tracking_parent", map[string]interface{}{"title": "Second"})
	created := accessGroupsE2ERequestJSON(t, client, http.MethodPost, base+"/api/tracking_child", token, map[string]interface{}{
		"data": map[string]interface{}{"type": "tracking_child", "attributes": map[string]interface{}{"title": "Original"}, "relationships": map[string]interface{}{"tracking_parent_id": map[string]interface{}{"data": map[string]interface{}{"type": "tracking_parent", "id": first}}}},
	}, http.StatusCreated)
	child := transportE2EReferenceID(t, created)
	original := transportE2EGetJSON(t, client, base+"/api/tracking_child/"+child, token)
	assertTransportE2EString(t, original, "data.relationships.tracking_parent_id.data.id", first)
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/tracking_child/"+child, token, map[string]interface{}{
		"data": map[string]interface{}{"type": "tracking_child", "id": child, "attributes": map[string]interface{}{"title": "Changed"}, "relationships": map[string]interface{}{"tracking_parent_id": map[string]interface{}{"data": map[string]interface{}{"type": "tracking_parent", "id": second}}}},
	}, http.StatusOK)
	after := transportE2EGetJSON(t, client, base+"/api/tracking_child/"+child, token)
	assertTransportE2EString(t, after, "data.relationships.tracking_parent_id.data.id", second)
	assertTransportE2EString(t, after, "data.attributes.title", "Changed")
	if version, _ := transportE2EPath(after, "data.attributes.version"); version != float64(2) {
		t.Fatalf("normal CRUD version=%v", version)
	}
}
