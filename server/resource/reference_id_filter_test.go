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
	"github.com/daptin/daptin/server/table_info"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestGetReferenceIdListToIdListWithTransactionMissingReference(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`create table user_account (
		id integer primary key,
		reference_id blob not null unique
	)`); err != nil {
		t.Fatalf("create user_account: %v", err)
	}

	existingRef, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("create existing ref: %v", err)
	}
	if _, err := db.Exec(`insert into user_account (id, reference_id) values (?, ?)`, 42, existingRef[:]); err != nil {
		t.Fatalf("insert user_account: %v", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	existing, err := GetReferenceIdListToIdListWithTransaction("user_account", []daptinid.DaptinReferenceId{
		daptinid.DaptinReferenceId(existingRef),
	}, tx)
	if err != nil {
		t.Fatalf("existing reference should resolve without error: %v", err)
	}
	if existing[daptinid.DaptinReferenceId(existingRef)] != 42 {
		t.Fatalf("existing reference resolved to %v, want 42", existing[daptinid.DaptinReferenceId(existingRef)])
	}

	missingRef, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("create missing ref: %v", err)
	}
	missing, err := GetReferenceIdListToIdListWithTransaction("user_account", []daptinid.DaptinReferenceId{
		daptinid.DaptinReferenceId(missingRef),
	}, tx)
	if err != nil {
		t.Fatalf("missing reference should return empty map without error, got: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing reference returned %v, want empty map", missing)
	}
}

func TestPaginatedFindAllRelationFilterReferenceResolution(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	statements := []string{
		`create table user_account (
			id integer primary key,
			reference_id blob not null unique
		)`,
		`create table gig (
			id integer primary key,
			name text,
			user_account_id integer,
			permission integer,
			reference_id blob not null unique,
			created_at timestamp
		)`,
		`create table gig_gig_id_has_usergroup_usergroup_id (
			id integer primary key,
			gig_id integer,
			usergroup_id integer,
			permission integer,
			reference_id blob
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	userRef, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("create user ref: %v", err)
	}
	if _, err := db.Exec(`insert into user_account (id, reference_id) values (?, ?)`, 77, userRef[:]); err != nil {
		t.Fatalf("insert user_account: %v", err)
	}

	gigRef, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("create gig ref: %v", err)
	}
	if _, err := db.Exec(
		`insert into gig (id, name, user_account_id, permission, reference_id, created_at) values (?, ?, ?, ?, ?, ?)`,
		1, "existing gig", 77, int64(auth.ALLOW_ALL_PERMISSIONS), gigRef[:], time.Now(),
	); err != nil {
		t.Fatalf("insert gig: %v", err)
	}

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

	columns := []api2go.ColumnInfo{
		{Name: "name", ColumnName: "name", ColumnType: "label", IsIndexed: true},
		{Name: USER_ACCOUNT_ID_COLUMN, ColumnName: USER_ACCOUNT_ID_COLUMN},
		{Name: "permission", ColumnName: "permission"},
		{Name: "reference_id", ColumnName: "reference_id"},
		{Name: "created_at", ColumnName: "created_at"},
	}
	relations := []api2go.TableRelation{
		api2go.NewTableRelation("gig", "has_one", USER_ACCOUNT_TABLE_NAME),
	}
	model := api2go.NewApi2GoModel("gig", columns, int64(auth.DEFAULT_PERMISSION), relations)
	crud := &DbResource{
		model:      model,
		connection: db,
		tableInfo: &table_info.TableInfo{
			TableName:         "gig",
			Columns:           columns,
			Relations:         relations,
			DefaultPermission: auth.DEFAULT_PERMISSION,
		},
		ms: &MiddlewareSet{},
	}

	request, err := http.NewRequest(http.MethodGet, "/api/gig", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	sessionUser := &auth.SessionUser{
		UserReferenceId: daptinid.DaptinReferenceId(uuid.New()),
		Groups: auth.GroupPermissionList{
			{GroupReferenceId: adminGroupRef},
		},
	}
	request = request.WithContext(context.WithValue(request.Context(), "user", sessionUser))

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	results, includes, pagination, _, err := crud.PaginatedFindAllWithoutFilters(api2go.Request{
		PlainRequest: request,
		QueryParams: url.Values{
			"page[size]":      []string{"50"},
			"sort":            []string{"-created_at"},
			"user_account_id": []string{userRef.String()},
		},
	}, tx)
	if err != nil {
		t.Fatalf("existing relation filter should return matching result without error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("existing relation filter returned %d results, want 1: %v", len(results), results)
	}
	if results[0]["name"] != "existing gig" {
		t.Fatalf("existing relation filter returned row %v, want existing gig", results[0])
	}
	if len(includes) != 1 {
		t.Fatalf("existing relation filter returned %d include groups, want 1", len(includes))
	}
	if pagination == nil {
		t.Fatalf("pagination should be returned for existing relation filter")
	}
	if pagination.TotalCount != 1 {
		t.Fatalf("existing relation filter total count = %d, want 1", pagination.TotalCount)
	}

	missingRef, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("create missing ref: %v", err)
	}
	results, includes, pagination, _, err = crud.PaginatedFindAllWithoutFilters(api2go.Request{
		PlainRequest: request,
		QueryParams: url.Values{
			"page[size]":      []string{"50"},
			"sort":            []string{"-created_at"},
			"user_account_id": []string{missingRef.String()},
		},
	}, tx)
	if err != nil {
		t.Fatalf("missing relation filter should return empty result without error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("missing relation filter returned %d results, want 0: %v", len(results), results)
	}
	if len(includes) != 0 {
		t.Fatalf("missing relation filter returned %d include groups, want 0", len(includes))
	}
	if pagination == nil {
		t.Fatalf("pagination should be returned")
	}
	if pagination.TotalCount != 0 {
		t.Fatalf("missing relation filter total count = %d, want 0", pagination.TotalCount)
	}
}
