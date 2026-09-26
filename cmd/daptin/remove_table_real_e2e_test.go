package main

import (
	"net/http"
	"os"
	"testing"
)

const removeTableRealE2ESchema = `
Tables:
  - TableName: remove_table_probe
    IsTopLevel: true
    Permission: 2097151
    DefaultPermission: 2097151
    Columns:
      - Name: value
        DataType: varchar(100)
        ColumnType: label
Relations:
  - Subject: remove_table_probe
    Object: user_account
    Relation: belongs_to
    ObjectName: user_account_id
  - Subject: remove_table_probe
    Object: usergroup
    Relation: has_many
`

func TestRemoveColumnThenTablePostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	testRemoveColumnThenTableRealE2E(t, transportE2EDaptinOptions{
		databaseType: "postgres", connectionString: dsn, schema: removeTableRealE2ESchema,
	})
}

func TestRemoveColumnThenTableSQLiteRealE2E(t *testing.T) {
	testRemoveColumnThenTableRealE2E(t, transportE2EDaptinOptions{schema: removeTableRealE2ESchema})
}

func testRemoveColumnThenTableRealE2E(t *testing.T, options transportE2EDaptinOptions) {
	t.Helper()
	fixture := startDaptinE2E(t, options)
	token := fixture.SignupAdmin(t)
	worldID := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, token, "world", "table_name", "remove_table_probe")
	accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, token, "remove_table_probe", map[string]interface{}{"value": "marker"})

	for _, action := range []struct {
		name       string
		attributes map[string]interface{}
		message    string
	}{
		{"remove_column", map[string]interface{}{"world_id": worldID, "column_name": "value"}, "Column deleted"},
		{"remove_table", map[string]interface{}{"world_id": worldID}, "Table deleted"},
	} {
		response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost,
			fixture.URL+"/action/world/"+action.name, token,
			map[string]interface{}{"attributes": action.attributes}, http.StatusOK)
		message, _ := transportE2EFindString(response, "message")
		if message != action.message {
			t.Fatalf("%s returned %q, want %q: %#v", action.name, message, action.message, response)
		}
	}
	worlds := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/world?page[size]=200", token, nil, http.StatusOK)
	for _, item := range accessGroupsE2EDataArray(t, worlds) {
		world, _ := item.(map[string]interface{})
		if world["id"] == worldID {
			t.Fatalf("deleted world %s remains in resource listing", worldID)
		}
	}
}
