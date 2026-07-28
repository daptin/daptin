package resource

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/table_info"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestGetObjectPermissionByReferenceIdWithTransactionIgnoresStaleCache(t *testing.T) {
	dm, cleanup := testOlric(t, "test-tx-permission-read")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	for _, statement := range []string{
		`create table user_account (
			id integer primary key,
			reference_id blob not null unique,
			user_account_id integer,
			permission integer
		)`,
		`create table usergroup (
			id integer primary key,
			reference_id blob not null unique,
			permission integer
		)`,
		`create table user_account_user_account_id_has_usergroup_usergroup_id (
			id integer primary key,
			user_account_id integer,
			usergroup_id integer,
			reference_id blob,
			permission integer
		)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	userRef := daptinid.DaptinReferenceId(uuid.New())
	if _, err := db.Exec(
		`insert into user_account (id, reference_id, user_account_id, permission) values (?, ?, ?, ?)`,
		1, userRef[:], 1, int64(auth.UserRefer),
	); err != nil {
		t.Fatalf("seed user account: %v", err)
	}

	cacheKey := "object-permission-user_account-" + userRef.String()
	if err := dm.Put(context.Background(), cacheKey, permission.PermissionInstance{
		Permission: auth.DEFAULT_PERMISSION,
	}); err != nil {
		t.Fatalf("seed stale permission cache: %v", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	perm := GetObjectPermissionByReferenceIdWithTransaction("user_account", userRef, tx)
	if perm.Permission != auth.UserRefer {
		t.Fatalf("permission came from stale cache: got %d, want %d", perm.Permission, auth.UserRefer)
	}
	if perm.UserId != userRef {
		t.Fatalf("owner came from stale cache: got %s, want %s", perm.UserId, userRef)
	}
}

func TestUpdateWithoutFiltersReturnsErrorWhenVersionMatchesNoRows(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`create table widget (
		id integer primary key,
		reference_id blob not null unique,
		name text,
		permission integer,
		version integer,
		updated_at timestamp
	)`); err != nil {
		t.Fatalf("create widget table: %v", err)
	}

	ref := daptinid.DaptinReferenceId(uuid.New())
	if _, err := db.Exec(
		`insert into widget (id, reference_id, name, permission, version, updated_at) values (?, ?, ?, ?, ?, ?)`,
		1, ref[:], "old", int64(auth.DEFAULT_PERMISSION), 1, time.Now(),
	); err != nil {
		t.Fatalf("seed widget: %v", err)
	}

	columns := []api2go.ColumnInfo{
		{Name: "name", ColumnName: "name", ColumnType: "label"},
		{Name: "permission", ColumnName: "permission"},
		{Name: "reference_id", ColumnName: "reference_id"},
		{Name: "version", ColumnName: "version"},
		{Name: "updated_at", ColumnName: "updated_at"},
	}
	crud := &DbResource{
		model: api2go.NewApi2GoModel("widget", columns, int64(auth.DEFAULT_PERMISSION), nil),
		tableInfo: &table_info.TableInfo{
			TableName:         "widget",
			Columns:           columns,
			DefaultPermission: auth.DEFAULT_PERMISSION,
		},
		connection: db,
		ms:         &MiddlewareSet{},
		Cruds:      map[string]*DbResource{},
	}
	oldUserAccountCrud := CRUD_MAP[USER_ACCOUNT_TABLE_NAME]
	CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = &DbResource{AdministratorGroupId: daptinid.DaptinReferenceId(uuid.New())}
	defer func() {
		if oldUserAccountCrud == nil {
			delete(CRUD_MAP, USER_ACCOUNT_TABLE_NAME)
			return
		}
		CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = oldUserAccountCrud
	}()

	model := api2go.NewApi2GoModelWithData("widget", columns, int64(auth.DEFAULT_PERMISSION), nil, map[string]interface{}{
		"id":           int64(1),
		"reference_id": uuid.UUID(ref),
		"name":         "old",
		"permission":   int64(auth.DEFAULT_PERMISSION),
		"version":      int64(99),
	})
	model.SetAttributes(map[string]interface{}{
		"reference_id": ref.String(),
		"name":         "new",
		"permission":   int64(auth.DEFAULT_PERMISSION),
		"version":      int64(99),
	})

	req := api2go.Request{
		PlainRequest: (&http.Request{
			Method: http.MethodPatch,
			URL:    &url.URL{Path: "/widget"},
		}).WithContext(context.Background()),
	}
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	_, err = crud.UpdateWithoutFilters(model, req, tx)
	if err == nil {
		t.Fatalf("expected stale version update to fail")
	}
}
