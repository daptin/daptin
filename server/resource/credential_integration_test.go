package resource

import (
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestGetCredentialByReferenceIdForIntegrationExecution(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	statements := []string{
		`create table _config (
			id integer primary key,
			name text,
			configtype text,
			configstate text,
			configenv text,
			value text
		)`,
		`create table credential (
			id integer primary key,
			name text not null,
			content text not null,
			user_account_id integer,
			reference_id blob not null unique,
			permission integer not null
		)`,
		`create table user_account (
			id integer primary key,
			reference_id blob not null unique
		)`,
		`create table usergroup (
			id integer primary key,
			reference_id blob not null unique
		)`,
		`create table credential_credential_id_has_usergroup_usergroup_id (
			id integer primary key,
			credential_id integer,
			usergroup_id integer,
			reference_id blob not null unique,
			permission integer not null
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	secret := "0123456789abcdef0123456789abcdef"
	if _, err := db.Exec(`insert into _config (name, configtype, configstate, configenv, value) values (?, ?, ?, ?, ?)`, "encryption.secret", "backend", "enabled", "", secret); err != nil {
		t.Fatalf("insert config: %v", err)
	}

	credentialRef := daptinid.DaptinReferenceId(uuid.New())
	groupSharedCredentialRef := daptinid.DaptinReferenceId(uuid.New())
	executeOnlyCredentialRef := daptinid.DaptinReferenceId(uuid.New())
	ownerRef := daptinid.DaptinReferenceId(uuid.New())
	otherUserRef := daptinid.DaptinReferenceId(uuid.New())
	adminGroupRef := daptinid.DaptinReferenceId(uuid.New())
	sharedGroupRef := daptinid.DaptinReferenceId(uuid.New())
	relationRef := daptinid.DaptinReferenceId(uuid.New())

	encryptedContent, err := Encrypt([]byte(secret), `{"token":"owner-token","api_key":"owner-api-key"}`)
	if err != nil {
		t.Fatalf("encrypt owner credential: %v", err)
	}
	groupEncryptedContent, err := Encrypt([]byte(secret), `{"token":"group-token"}`)
	if err != nil {
		t.Fatalf("encrypt group credential: %v", err)
	}
	executeOnlyEncryptedContent, err := Encrypt([]byte(secret), `{"token":"execute-only-token"}`)
	if err != nil {
		t.Fatalf("encrypt execute-only credential: %v", err)
	}

	if _, err := db.Exec(`insert into user_account (id, reference_id) values (?, ?)`, 42, ownerRef[:]); err != nil {
		t.Fatalf("insert owner user: %v", err)
	}
	if _, err := db.Exec(`insert into user_account (id, reference_id) values (?, ?)`, 77, otherUserRef[:]); err != nil {
		t.Fatalf("insert other user: %v", err)
	}
	if _, err := db.Exec(`insert into usergroup (id, reference_id) values (?, ?)`, 10, sharedGroupRef[:]); err != nil {
		t.Fatalf("insert shared group: %v", err)
	}
	if _, err := db.Exec(`insert into credential (id, name, content, user_account_id, reference_id, permission) values (?, ?, ?, ?, ?, ?)`, 20, "owner-cred", encryptedContent, 42, credentialRef[:], int64(auth.DEFAULT_PERMISSION)); err != nil {
		t.Fatalf("insert owner credential: %v", err)
	}
	if _, err := db.Exec(`insert into credential (id, name, content, user_account_id, reference_id, permission) values (?, ?, ?, ?, ?, ?)`, 21, "group-cred", groupEncryptedContent, 42, groupSharedCredentialRef[:], int64(auth.DEFAULT_PERMISSION)); err != nil {
		t.Fatalf("insert group credential: %v", err)
	}
	if _, err := db.Exec(`insert into credential (id, name, content, user_account_id, reference_id, permission) values (?, ?, ?, ?, ?, ?)`, 22, "execute-only-cred", executeOnlyEncryptedContent, 42, executeOnlyCredentialRef[:], int64(auth.UserExecute|auth.GuestExecute)); err != nil {
		t.Fatalf("insert execute-only credential: %v", err)
	}
	if _, err := db.Exec(`insert into credential_credential_id_has_usergroup_usergroup_id (id, credential_id, usergroup_id, reference_id, permission) values (?, ?, ?, ?, ?)`, 30, 21, 10, relationRef[:], int64(auth.GroupRead)); err != nil {
		t.Fatalf("insert credential group relation: %v", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	columns := []api2go.ColumnInfo{
		{Name: "id", ColumnName: "id"},
		{Name: "name", ColumnName: "name"},
		{Name: "content", ColumnName: "content"},
		{Name: "user_account_id", ColumnName: "user_account_id"},
		{Name: "reference_id", ColumnName: "reference_id"},
		{Name: "permission", ColumnName: "permission"},
	}
	credentialCrud := &DbResource{
		model:                api2go.NewApi2GoModel("credential", columns, int64(auth.DEFAULT_PERMISSION), nil),
		ConfigStore:          &ConfigStore{},
		AdministratorGroupId: adminGroupRef,
	}
	credentialCrud.Cruds = map[string]*DbResource{"credential": credentialCrud}

	ownerSession := &auth.SessionUser{UserId: 42, UserReferenceId: ownerRef}
	credential, err := credentialCrud.GetCredentialByReferenceIdForIntegrationExecution(credentialRef, ownerSession, tx)
	if err != nil {
		t.Fatalf("owner credential should pass: %v", err)
	}
	if credential.DataMap["token"] != "owner-token" {
		t.Fatalf("unexpected decrypted token: %v", credential.DataMap["token"])
	}

	otherSession := &auth.SessionUser{UserId: 77, UserReferenceId: otherUserRef}
	if _, err := credentialCrud.GetCredentialByReferenceIdForIntegrationExecution(credentialRef, otherSession, tx); err == nil {
		t.Fatalf("other user's unshared credential should fail")
	}
	if _, err := credentialCrud.GetCredentialByReferenceIdForIntegrationExecution(executeOnlyCredentialRef, ownerSession, tx); err == nil {
		t.Fatalf("execute-only credential should not expose credential content")
	}

	groupSession := &auth.SessionUser{
		UserId:          77,
		UserReferenceId: otherUserRef,
		Groups: auth.GroupPermissionList{
			{GroupReferenceId: sharedGroupRef},
		},
	}
	if _, err := credentialCrud.GetCredentialByReferenceIdForIntegrationExecution(groupSharedCredentialRef, groupSession, tx); err != nil {
		t.Fatalf("group shared credential should pass: %v", err)
	}

	adminSession := &auth.SessionUser{
		UserId:          77,
		UserReferenceId: otherUserRef,
		Groups: auth.GroupPermissionList{
			{GroupReferenceId: adminGroupRef},
		},
	}
	if _, err := credentialCrud.GetCredentialByReferenceIdForIntegrationExecution(credentialRef, adminSession, tx); err != nil {
		t.Fatalf("admin credential access should pass: %v", err)
	}

	if _, err := credentialCrud.GetCredentialByReferenceIdForIntegrationExecution(daptinid.NullReferenceId, ownerSession, tx); err == nil {
		t.Fatalf("missing credential id should fail")
	}
	if _, err := credentialCrud.GetCredentialByReferenceIdForIntegrationExecution(credentialRef, nil, tx); err == nil {
		t.Fatalf("missing session user should fail")
	}
}
