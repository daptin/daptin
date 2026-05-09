package resource

import (
	"testing"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestValidateOAuthTokenForIntegrationExecution(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	statements := []string{
		`create table oauth_connect (
			id integer primary key,
			reference_id blob not null unique
		)`,
		`create table oauth_token (
			id integer primary key,
			reference_id blob not null unique,
			oauth_connect_id integer not null,
			user_account_id integer not null
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	providerRef := daptinid.DaptinReferenceId(uuid.New())
	otherProviderRef := daptinid.DaptinReferenceId(uuid.New())
	tokenRef := daptinid.DaptinReferenceId(uuid.New())
	otherUserTokenRef := daptinid.DaptinReferenceId(uuid.New())
	otherProviderTokenRef := daptinid.DaptinReferenceId(uuid.New())

	if _, err := db.Exec(`insert into oauth_connect (id, reference_id) values (?, ?)`, 10, providerRef[:]); err != nil {
		t.Fatalf("insert provider: %v", err)
	}
	if _, err := db.Exec(`insert into oauth_connect (id, reference_id) values (?, ?)`, 11, otherProviderRef[:]); err != nil {
		t.Fatalf("insert other provider: %v", err)
	}
	if _, err := db.Exec(`insert into oauth_token (id, reference_id, oauth_connect_id, user_account_id) values (?, ?, ?, ?)`, 20, tokenRef[:], 10, 42); err != nil {
		t.Fatalf("insert token: %v", err)
	}
	if _, err := db.Exec(`insert into oauth_token (id, reference_id, oauth_connect_id, user_account_id) values (?, ?, ?, ?)`, 21, otherUserTokenRef[:], 10, 77); err != nil {
		t.Fatalf("insert other user token: %v", err)
	}
	if _, err := db.Exec(`insert into oauth_token (id, reference_id, oauth_connect_id, user_account_id) values (?, ?, ?, ?)`, 22, otherProviderTokenRef[:], 11, 42); err != nil {
		t.Fatalf("insert other provider token: %v", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	crud := &DbResource{}
	if err := crud.ValidateOAuthTokenForIntegrationExecution(tokenRef, 42, providerRef, tx); err != nil {
		t.Fatalf("valid token should pass: %v", err)
	}
	if err := crud.ValidateOAuthTokenForIntegrationExecution(otherUserTokenRef, 42, providerRef, tx); err == nil {
		t.Fatalf("other user's token should fail")
	}
	if err := crud.ValidateOAuthTokenForIntegrationExecution(otherProviderTokenRef, 42, providerRef, tx); err == nil {
		t.Fatalf("wrong provider token should fail")
	}
	if err := crud.ValidateOAuthTokenForIntegrationExecution(daptinid.NullReferenceId, 42, providerRef, tx); err == nil {
		t.Fatalf("missing token should fail")
	}
	if err := crud.ValidateOAuthTokenForIntegrationExecution(tokenRef, 0, providerRef, tx); err == nil {
		t.Fatalf("missing user should fail")
	}
	if err := crud.ValidateOAuthTokenForIntegrationExecution(tokenRef, 42, daptinid.NullReferenceId, tx); err == nil {
		t.Fatalf("missing provider should fail")
	}
}
