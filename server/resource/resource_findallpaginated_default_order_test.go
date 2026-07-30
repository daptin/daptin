package resource

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/table_info"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestAuthenticatedSQLiteReadsWithEmptyDefaultOrder(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	adminGroupRef := daptinid.DaptinReferenceId(uuid.New())
	oldUserAccountCrud := CRUD_MAP[USER_ACCOUNT_TABLE_NAME]
	CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = &DbResource{AdministratorGroupId: adminGroupRef}
	defer func() {
		if oldUserAccountCrud == nil {
			delete(CRUD_MAP, USER_ACCOUNT_TABLE_NAME)
			return
		}
		CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = oldUserAccountCrud
	}()

	for _, tableName := range []string{"llm_usage", "credential", "api_usage", "api_quota"} {
		t.Run(tableName, func(t *testing.T) {
			assertAuthenticatedSQLiteListWithEmptyDefaultOrder(t, db, tableName, adminGroupRef)
		})
	}
}

func assertAuthenticatedSQLiteListWithEmptyDefaultOrder(t *testing.T, db *sqlx.DB, tableName string, adminGroupRef daptinid.DaptinReferenceId) {
	t.Helper()

	if _, err := db.Exec(fmt.Sprintf(`create table %s (
		id integer primary key,
		name text,
		user_account_id integer,
		permission integer,
		reference_id blob not null unique,
		created_at timestamp
	)`, tableName)); err != nil {
		t.Fatalf("create %s: %v", tableName, err)
	}
	joinTable := fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id", tableName, tableName)
	if _, err := db.Exec(fmt.Sprintf(`create table %s (
		id integer primary key,
		%s_id integer,
		usergroup_id integer,
		permission integer,
		reference_id blob
	)`, joinTable, tableName)); err != nil {
		t.Fatalf("create %s: %v", joinTable, err)
	}

	columns := []api2go.ColumnInfo{
		{Name: "name", ColumnName: "name", ColumnType: "label"},
		{Name: USER_ACCOUNT_ID_COLUMN, ColumnName: USER_ACCOUNT_ID_COLUMN},
		{Name: "permission", ColumnName: "permission"},
		{Name: "reference_id", ColumnName: "reference_id"},
		{Name: "created_at", ColumnName: "created_at"},
	}
	model := api2go.NewApi2GoModel(tableName, columns, int64(auth.DEFAULT_PERMISSION), nil)
	crud := &DbResource{
		model:      model,
		connection: db,
		tableInfo: &table_info.TableInfo{
			TableName:         tableName,
			Columns:           columns,
			DefaultOrder:      "",
			DefaultPermission: auth.DEFAULT_PERMISSION,
		},
		ms: &MiddlewareSet{},
	}

	request, err := http.NewRequest(http.MethodGet, "/api/"+tableName+"?page[size]=1", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request = request.WithContext(context.WithValue(request.Context(), "user", &auth.SessionUser{
		UserReferenceId: daptinid.DaptinReferenceId(uuid.New()),
		Groups: auth.GroupPermissionList{
			{GroupReferenceId: adminGroupRef},
		},
	}))

	list := func(wantRows int) {
		t.Helper()
		tx, err := db.Beginx()
		if err != nil {
			t.Fatalf("begin transaction: %v", err)
		}
		defer tx.Rollback()

		rows, _, pagination, _, err := crud.PaginatedFindAllWithoutFilters(api2go.Request{
			PlainRequest: request,
			QueryParams:  url.Values{"page[size]": []string{"1"}},
		}, tx)
		if err != nil {
			t.Fatalf("authenticated list with empty default_order: %v", err)
		}
		if len(rows) != wantRows {
			t.Fatalf("rows = %d, want %d", len(rows), wantRows)
		}
		if pagination == nil || pagination.TotalCount != uint64(wantRows) {
			t.Fatalf("pagination = %#v, want total count %d", pagination, wantRows)
		}
	}

	list(0)
	referenceID := uuid.New()
	if _, err := db.Exec(
		fmt.Sprintf(`insert into %s (id, name, user_account_id, permission, reference_id, created_at) values (?, ?, ?, ?, ?, ?)`, tableName),
		1, "row", 1, int64(auth.ALLOW_ALL_PERMISSIONS), referenceID[:], time.Now(),
	); err != nil {
		t.Fatalf("insert %s: %v", tableName, err)
	}
	list(1)
}
