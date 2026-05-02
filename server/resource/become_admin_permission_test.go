package resource

import (
	stdjson "encoding/json"
	"strconv"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestBecomeAdminOwnRowsTransitionsOnlyBootstrapPermission(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`create table category (
		id integer primary key,
		user_account_id integer,
		permission integer
	)`)
	if err != nil {
		t.Fatalf("create category: %v", err)
	}

	_, err = db.Exec(`insert into category (id, user_account_id, permission) values
		(1, 0, ?),
		(2, 0, ?)`,
		int64(auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN),
		int64(15811),
	)
	if err != nil {
		t.Fatalf("insert category rows: %v", err)
	}

	columns := []api2go.ColumnInfo{
		{ColumnName: USER_ACCOUNT_ID_COLUMN},
		{ColumnName: "permission"},
	}
	crud := &DbResource{
		model: api2go.NewApi2GoModel("category", columns, int64(auth.DEFAULT_PERMISSION), nil),
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := becomeAdminOwnRows(crud, 42, tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("become admin own rows: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	rows, err := db.Queryx(`select id, user_account_id, permission from category order by id`)
	if err != nil {
		t.Fatalf("select category rows: %v", err)
	}
	defer rows.Close()

	got := map[int]struct {
		userId     int64
		permission int64
	}{}
	for rows.Next() {
		var id int
		var userId int64
		var permission int64
		if err := rows.Scan(&id, &userId, &permission); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		got[id] = struct {
			userId     int64
			permission int64
		}{userId: userId, permission: permission}
	}

	if got[1].userId != 42 || got[1].permission != int64(auth.DEFAULT_PERMISSION) {
		t.Fatalf("expected bootstrap row to be owned by 42 with default permission, got %+v", got[1])
	}
	if got[2].userId != 42 || got[2].permission != 15811 {
		t.Fatalf("expected explicit row permission to be preserved, got %+v", got[2])
	}
}

func TestUpdateDefaultPermissionInSchemaJsonTransitionsOnlyBootstrapDefaults(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`create table world (
		id integer primary key,
		table_name text,
		world_schema_json text
	)`)
	if err != nil {
		t.Fatalf("create world: %v", err)
	}

	_, err = db.Exec(`insert into world (id, table_name, world_schema_json) values
		(1, 'private_table', ?),
		(2, 'public_table', ?),
		(3, 'legacy_table', ?),
		(4, 'private_table_audit', ?)`,
		`{"TableName":"private_table","DefaultPermission":`+intString(auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN)+`}`,
		`{"TableName":"public_table","DefaultPermission":15811}`,
		`{"TableName":"legacy_table"}`,
		`{"TableName":"private_table_audit","DefaultPermission":`+intString(auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN)+`}`,
	)
	if err != nil {
		t.Fatalf("insert world rows: %v", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	err = updateDefaultPermissionInSchemaJson(tx, auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN, auth.DEFAULT_PERMISSION, auth.UserRead|auth.GroupRead)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("update schema json: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	assertSchemaDefaultPermission(t, db, "private_table", int64(auth.DEFAULT_PERMISSION))
	assertSchemaDefaultPermission(t, db, "public_table", 15811)
	assertSchemaDefaultPermission(t, db, "legacy_table", int64(auth.DEFAULT_PERMISSION))
	assertSchemaDefaultPermission(t, db, "private_table_audit", int64(auth.UserRead|auth.GroupRead))
}

func TestNewImportAdminSessionUserIncludesAdminGroup(t *testing.T) {
	adminUserId := int64(12)
	adminUserRefId := daptinid.DaptinReferenceId(uuid.New())
	adminGroupId := daptinid.DaptinReferenceId(uuid.New())

	sessionUser := newImportAdminSessionUser(adminUserId, adminUserRefId, map[string]*DbResource{
		"usergroup": {
			AdministratorGroupId: adminGroupId,
		},
	})

	if sessionUser.UserId != adminUserId {
		t.Fatalf("expected user id %d, got %d", adminUserId, sessionUser.UserId)
	}
	if sessionUser.UserReferenceId != adminUserRefId {
		t.Fatalf("expected admin user reference id to be preserved")
	}
	if len(sessionUser.Groups) != 1 {
		t.Fatalf("expected one admin group, got %d", len(sessionUser.Groups))
	}
	if sessionUser.Groups[0].GroupReferenceId != adminGroupId {
		t.Fatalf("expected admin group reference id to be set")
	}
}

func assertSchemaDefaultPermission(t *testing.T, db *sqlx.DB, tableName string, expected int64) {
	t.Helper()
	var schemaJSON string
	if err := db.QueryRow(`select world_schema_json from world where table_name = ?`, tableName).Scan(&schemaJSON); err != nil {
		t.Fatalf("select schema json for %s: %v", tableName, err)
	}

	var schema map[string]interface{}
	if err := stdjson.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		t.Fatalf("unmarshal schema json for %s: %v", tableName, err)
	}

	got, ok := schema["DefaultPermission"].(float64)
	if !ok {
		t.Fatalf("schema json for %s has no numeric DefaultPermission: %#v", tableName, schema["DefaultPermission"])
	}
	if int64(got) != expected {
		t.Fatalf("expected DefaultPermission %d for %s, got %d", expected, tableName, int64(got))
	}
}

func intString(permission auth.AuthPermission) string {
	return strconv.FormatInt(int64(permission), 10)
}
