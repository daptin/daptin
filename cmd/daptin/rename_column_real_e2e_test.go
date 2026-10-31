package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRenameColumnSQLiteRealE2E(t *testing.T) {
	options := transportE2EDaptinOptions{
		connectionString: filepath.Join(t.TempDir(), "rename.db"),
		schema: `Tables:
  - TableName: rename_column_probe
    IsTopLevel: true
    Columns:
      - Name: value
        ColumnType: label
        DataType: varchar(100)
      - Name: other
        ColumnType: label
        DataType: varchar(100)
`,
	}
	fixture := startDaptinE2E(t, options)
	token := fixture.SignupAdmin(t)
	worldID := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, token, "world", "table_name", "rename_column_probe")
	accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, token, "rename_column_probe", map[string]interface{}{"value": "preserved", "other": "untouched"})
	world := func() map[string]interface{} {
		return accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/world/"+worldID, token, nil, http.StatusOK).(map[string]interface{})
	}
	before := world()
	for _, name := range []interface{}{
		"renamed;\nDROP\nTABLE\nrename_column_probe;\n--", "renamed`", "renamed\tname", "SELECT", "other", "OTHER", "", nil, 42,
	} {
		status, _ := postLLMActionForStatus(t, fixture.Client, fixture.URL+"/action/world/rename_column", token, map[string]interface{}{
			"attributes": map[string]interface{}{"table_name": "rename_column_probe", "column_name": "value", "new_column_name": name},
		})
		if status < http.StatusBadRequest {
			t.Fatalf("invalid rename %q returned status %d", name, status)
		}
		if !reflect.DeepEqual(world(), before) {
			t.Fatalf("invalid rename %q changed world metadata", name)
		}
	}
	response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/world/rename_column", token,
		map[string]interface{}{"attributes": map[string]interface{}{
			"table_name": "rename_column_probe", "column_name": "value", "new_column_name": "renamed value",
		}}, http.StatusOK)
	message, _ := transportE2EFindString(response, "message")
	if message != "Column renamed" {
		t.Fatalf("rename failed: %#v", response)
	}
	// The action persists schema metadata immediately; runtime resources reload on restart.
	data := world()["data"].(map[string]interface{})
	schema := data["attributes"].(map[string]interface{})["world_schema_json"]
	var table struct {
		Columns []struct{ Name, ColumnName string }
	}
	bytes, err := json.Marshal(schema)
	if text, ok := schema.(string); ok {
		bytes = []byte(text)
	}
	if err != nil || json.Unmarshal(bytes, &table) != nil {
		t.Fatalf("invalid persisted schema: %#v", schema)
	}
	found := false
	for _, col := range table.Columns {
		if col.Name == "renamed_value" && col.ColumnName == "renamed_value" {
			found = true
		}
		if col.Name == "value" || col.ColumnName == "value" {
			t.Fatal("old column remains in schema")
		}
	}
	if !found {
		t.Fatal("renamed column missing from schema")
	}
	fixture.Process.stopProcess()
	options.schema = ""
	fixture = startDaptinE2E(t, options)
	rows := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/rename_column_probe", token, nil, http.StatusOK)
	items := accessGroupsE2EDataArray(t, rows)
	if len(items) != 1 || items[0].(map[string]interface{})["attributes"].(map[string]interface{})["renamed_value"] != "preserved" {
		t.Fatalf("rename lost stored data: %#v", rows)
	}
}
