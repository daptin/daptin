package resource

import (
	"context"
	"net/http"
	"net/url"
	"strings"
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

func TestOutboxMailServerReferRequiresAdminContext(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	adminGroupRef := daptinid.DaptinReferenceId(uuid.New())
	adminUserRef := daptinid.DaptinReferenceId(uuid.New())
	mailServerRef := daptinid.DaptinReferenceId(uuid.New())

	for _, statement := range []string{
		`create table usergroup (id integer primary key, name text, reference_id blob, permission integer)`,
		`create table user_account (id integer primary key, reference_id blob, user_account_id integer, permission integer)`,
		`create table user_account_user_account_id_has_usergroup_usergroup_id (
			id integer primary key,
			user_account_id integer,
			usergroup_id integer,
			reference_id blob,
			permission integer,
			created_at timestamp
		)`,
		`create table mail_server (
			id integer primary key,
			hostname text,
			reference_id blob not null unique,
			user_account_id integer,
			permission integer
		)`,
		`create table mail_server_mail_server_id_has_usergroup_usergroup_id (
			id integer primary key,
			mail_server_id integer,
			usergroup_id integer,
			reference_id blob,
			permission integer
		)`,
		`create table outbox (
			id integer primary key,
			from_address text,
			to_address text,
			to_host text,
			mail_server_id integer not null,
			mail text,
			sent bool,
			retry_count integer,
			next_retry_at timestamp,
			reference_id blob,
			permission integer,
			created_at timestamp,
			updated_at timestamp
		)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	if _, err := db.Exec(`insert into usergroup (id, name, reference_id, permission) values (?, ?, ?, ?)`, 2, "administrators", adminGroupRef[:], int64(auth.DEFAULT_PERMISSION)); err != nil {
		t.Fatalf("insert admin group: %v", err)
	}
	if _, err := db.Exec(`insert into user_account (id, reference_id, permission) values (?, ?, ?)`, 1, adminUserRef[:], int64(auth.DEFAULT_PERMISSION)); err != nil {
		t.Fatalf("insert admin user: %v", err)
	}
	adminMembershipRef := uuid.New()
	if _, err := db.Exec(`insert into user_account_user_account_id_has_usergroup_usergroup_id (user_account_id, usergroup_id, reference_id, permission, created_at) values (?, ?, ?, ?, ?)`,
		1, 2, adminMembershipRef[:], int64(auth.DEFAULT_PERMISSION), time.Now()); err != nil {
		t.Fatalf("insert admin membership: %v", err)
	}
	if _, err := db.Exec(`insert into mail_server (id, hostname, reference_id, user_account_id, permission) values (?, ?, ?, ?, ?)`,
		9, "mail.example.test", mailServerRef[:], 1, int64(auth.DEFAULT_PERMISSION)); err != nil {
		t.Fatalf("insert mail_server: %v", err)
	}

	outboxCrud := testOutboxCrud(db, adminGroupRef)
	oldUserAccountCrud := CRUD_MAP[USER_ACCOUNT_TABLE_NAME]
	CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = &DbResource{AdministratorGroupId: adminGroupRef}
	defer func() {
		if oldUserAccountCrud == nil {
			delete(CRUD_MAP, USER_ACCOUNT_TABLE_NAME)
			return
		}
		CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = oldUserAccountCrud
	}()

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	_, err = outboxCrud.CreateWithoutFilter(testOutboxModel(mailServerRef), testOutboxRequest(&auth.SessionUser{
		UserReferenceId: daptinid.DaptinReferenceId(uuid.New()),
	}), tx)
	if err == nil || !strings.Contains(err.Error(), "refer object not allowed [mail_server]") {
		t.Fatalf("expected normal user refer failure, got %v", err)
	}

	_, err = outboxCrud.CreateWithoutFilter(testOutboxModel(mailServerRef), testOutboxRequest(&auth.SessionUser{
		UserId:          1,
		UserReferenceId: adminUserRef,
		Groups: auth.GroupPermissionList{
			{GroupReferenceId: adminGroupRef},
		},
	}), tx)
	if err != nil {
		t.Fatalf("expected admin/internal outbox create to succeed: %v", err)
	}
}

func testOutboxCrud(db *sqlx.DB, adminGroupRef daptinid.DaptinReferenceId) *DbResource {
	columns := []api2go.ColumnInfo{
		{Name: "id", ColumnName: "id", DataType: "INTEGER", ColumnType: "id", IsAutoIncrement: true},
		{Name: "from_address", ColumnName: "from_address", DataType: "varchar(200)", ColumnType: "label"},
		{Name: "to_address", ColumnName: "to_address", DataType: "varchar(200)", ColumnType: "label"},
		{Name: "to_host", ColumnName: "to_host", DataType: "varchar(200)", ColumnType: "label"},
		{
			Name:         "mail_server",
			ColumnName:   "mail_server_id",
			DataType:     "int(11)",
			ColumnType:   "alias",
			IsForeignKey: true,
			ForeignKeyData: api2go.ForeignKeyData{
				Namespace:  "mail_server",
				KeyName:    "id",
				DataSource: "self",
			},
		},
		{Name: "mail", ColumnName: "mail", DataType: "blob", ColumnType: "gzip"},
		{Name: "sent", ColumnName: "sent", DataType: "bool", ColumnType: "truefalse"},
		{Name: "retry_count", ColumnName: "retry_count", DataType: "int(11)", ColumnType: "value"},
		{Name: "next_retry_at", ColumnName: "next_retry_at", DataType: "timestamp", ColumnType: "datetime", IsNullable: true},
		{Name: "reference_id", ColumnName: "reference_id", DataType: "blob", ColumnType: "alias"},
		{Name: "permission", ColumnName: "permission", DataType: "integer"},
		{Name: "created_at", ColumnName: "created_at", DataType: "timestamp", ColumnType: "datetime"},
		{Name: "updated_at", ColumnName: "updated_at", DataType: "timestamp", ColumnType: "datetime", IsNullable: true},
	}
	crud := &DbResource{
		model: api2go.NewApi2GoModel("outbox", columns, int64(auth.DEFAULT_PERMISSION), nil),
		tableInfo: &table_info.TableInfo{
			TableName:         "outbox",
			Columns:           columns,
			DefaultPermission: auth.DEFAULT_PERMISSION,
		},
		connection:           db,
		AdministratorGroupId: adminGroupRef,
		Cruds:                map[string]*DbResource{},
	}
	crud.Cruds["outbox"] = crud
	return crud
}

func testOutboxModel(mailServerRef daptinid.DaptinReferenceId) api2go.Api2GoModel {
	return api2go.NewApi2GoModelWithData("outbox", nil, 0, nil, map[string]interface{}{
		"from_address":   "login@example.test",
		"to_address":     "user@example.test",
		"to_host":        "example.test",
		"mail_server_id": mailServerRef,
		"mail":           "bWVzc2FnZQ==",
		"sent":           false,
		"retry_count":    0,
		"next_retry_at":  time.Now(),
	})
}

func testOutboxRequest(sessionUser *auth.SessionUser) api2go.Request {
	outboxURL, _ := url.Parse("/api/outbox")
	request := &http.Request{
		Method: http.MethodPost,
		URL:    outboxURL,
	}
	request = request.WithContext(context.WithValue(context.Background(), "user", sessionUser))
	return api2go.Request{PlainRequest: request}
}
