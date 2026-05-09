package actions

import (
	"testing"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestCustomCredentialAuthUsesRequestSessionUser(t *testing.T) {
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
	ownerRef := daptinid.DaptinReferenceId(uuid.New())
	otherUserRef := daptinid.DaptinReferenceId(uuid.New())
	adminGroupRef := daptinid.DaptinReferenceId(uuid.New())
	encryptedContent, err := resource.Encrypt([]byte(secret), `{"token":"owner-token"}`)
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}

	if _, err := db.Exec(`insert into user_account (id, reference_id) values (?, ?)`, 42, ownerRef[:]); err != nil {
		t.Fatalf("insert owner user: %v", err)
	}
	if _, err := db.Exec(`insert into user_account (id, reference_id) values (?, ?)`, 77, otherUserRef[:]); err != nil {
		t.Fatalf("insert other user: %v", err)
	}
	if _, err := db.Exec(`insert into credential (id, name, content, user_account_id, reference_id, permission) values (?, ?, ?, ?, ?, ?)`, 20, "owner-cred", encryptedContent, 42, credentialRef[:], int64(auth.DEFAULT_PERMISSION)); err != nil {
		t.Fatalf("insert credential: %v", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	credentialCrud := &resource.DbResource{
		ConfigStore:          &resource.ConfigStore{},
		AdministratorGroupId: adminGroupRef,
	}

	performer := &integrationActionPerformer{
		cruds: map[string]*resource.DbResource{
			"credential": credentialCrud,
		},
	}

	elevatedSession := &auth.SessionUser{
		UserId:          77,
		UserReferenceId: otherUserRef,
		Groups: auth.GroupPermissionList{
			{GroupReferenceId: adminGroupRef},
		},
	}
	requestSession := &auth.SessionUser{
		UserId:          77,
		UserReferenceId: otherUserRef,
	}

	_, _, _, _, err = performer.customCredentialAuthArguments(
		map[string]interface{}{
			"credential_id":      credentialRef,
			"sessionUser":        elevatedSession,
			"requestSessionUser": requestSession,
		},
		map[string]interface{}{
			"scheme":      "bearer",
			"token_field": "token",
		},
		nil,
		tx,
		true,
	)
	if err == nil {
		t.Fatalf("request user should not inherit action-engine admin elevation for credential access")
	}
}
