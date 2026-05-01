package resource

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/table_info"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func testUserAccountResource(t *testing.T, db *sqlx.DB) (*DbResource, uuid.UUID) {
	t.Helper()

	refId, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("create ref id: %v", err)
	}
	passwordHash, err := BcryptHashString("OldPass123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	statements := []string{
		`create table user_account (
			id integer primary key autoincrement,
			name text,
			email text,
			password text,
			confirmed bool default false,
			auth_version integer not null default 1,
			version integer not null default 1,
			created_at timestamp,
			updated_at timestamp,
			reference_id blob not null unique,
			permission integer
		)`,
		`create table usergroup (id integer primary key, name text, reference_id blob)`,
		`create table user_account_user_account_id_has_usergroup_usergroup_id (
			id integer primary key,
			user_account_id integer,
			usergroup_id integer,
			created_at timestamp
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	if _, err := db.Exec(
		`insert into user_account
			(name, email, password, confirmed, auth_version, version, reference_id, permission)
			values (?, ?, ?, ?, ?, ?, ?, ?)`,
		"Test User", "test@example.com", passwordHash, false, 1, 1, refId[:], 0,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	columns := []api2go.ColumnInfo{
		{Name: "name", ColumnName: "name", DataType: "varchar(80)", ColumnType: "label"},
		{Name: "email", ColumnName: "email", DataType: "varchar(80)", ColumnType: "email"},
		{Name: "password", ColumnName: "password", DataType: "varchar(100)", ColumnType: "password", IsNullable: true},
		{Name: "confirmed", ColumnName: "confirmed", DataType: "bool", ColumnType: "truefalse", DefaultValue: "false"},
		{Name: auth.AuthVersionColumn, ColumnName: auth.AuthVersionColumn, DataType: "INTEGER", ColumnType: "measurement", DefaultValue: "1", ExcludeFromApi: true},
	}
	columns = append(columns, StandardColumns...)
	tableInfo := table_info.TableInfo{
		TableName: USER_ACCOUNT_TABLE_NAME,
		Columns:   columns,
	}
	model := api2go.NewApi2GoModel(USER_ACCOUNT_TABLE_NAME, columns, int64(auth.DEFAULT_PERMISSION), nil)
	dbResource := &DbResource{
		model:      model,
		db:         db,
		connection: db,
		tableInfo:  &tableInfo,
		Cruds:      map[string]*DbResource{},
	}
	oldCrud := CRUD_MAP[USER_ACCOUNT_TABLE_NAME]
	CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = dbResource
	t.Cleanup(func() {
		if oldCrud == nil {
			delete(CRUD_MAP, USER_ACCOUNT_TABLE_NAME)
		} else {
			CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = oldCrud
		}
	})

	return dbResource, refId
}

func updateUserForAuthLifecycleTest(t *testing.T, dbResource *DbResource, refId uuid.UUID, attrs map[string]interface{}, version int64, authVersion int64) {
	t.Helper()
	model := api2go.NewApi2GoModelWithData(USER_ACCOUNT_TABLE_NAME, dbResource.model.GetColumns(), int64(auth.DEFAULT_PERMISSION), nil, map[string]interface{}{
		"id":           int64(1),
		"name":         "Test User",
		"email":        "test@example.com",
		"password":     "$2a$11$old",
		"confirmed":    false,
		"auth_version": authVersion,
		"version":      version,
		"reference_id": refId,
		"permission":   int64(0),
	})
	model.SetAttributes(attrs)

	u, _ := url.Parse("/user_account")
	req := api2go.Request{
		PlainRequest: (&http.Request{Method: "PATCH", URL: u}).WithContext(context.Background()),
	}

	tx, err := dbResource.connection.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = dbResource.UpdateWithoutFilters(model, req, tx)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("update user: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
}

func TestUserAccountPasswordUpdateIncrementsAuthVersion(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	dbResource, refId := testUserAccountResource(t, db)
	updateUserForAuthLifecycleTest(t, dbResource, refId, map[string]interface{}{
		"password": "NewPass456!",
	}, 1, 1)

	var authVersion int64
	var version int64
	var passwordHash string
	if err := db.QueryRow("select auth_version, version, password from user_account where id = 1").Scan(&authVersion, &version, &passwordHash); err != nil {
		t.Fatalf("select updated user: %v", err)
	}
	if authVersion != 2 {
		t.Fatalf("expected auth_version 2 after password change, got %d", authVersion)
	}
	if version != 2 {
		t.Fatalf("expected row version 2 after update, got %d", version)
	}
	if !BcryptCheckStringHash("NewPass456!", passwordHash) {
		t.Fatal("expected password to be re-hashed to the new value")
	}
}

func TestUserAccountNonPasswordUpdateDoesNotIncrementAuthVersion(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	dbResource, refId := testUserAccountResource(t, db)
	updateUserForAuthLifecycleTest(t, dbResource, refId, map[string]interface{}{
		"name": "Renamed User",
	}, 1, 1)

	var authVersion int64
	var version int64
	if err := db.QueryRow("select auth_version, version from user_account where id = 1").Scan(&authVersion, &version); err != nil {
		t.Fatalf("select updated user: %v", err)
	}
	if authVersion != 1 {
		t.Fatalf("expected auth_version to remain 1 after non-password update, got %d", authVersion)
	}
	if version != 2 {
		t.Fatalf("expected row version 2 after update, got %d", version)
	}
}
