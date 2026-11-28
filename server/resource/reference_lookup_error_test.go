package resource

import (
	"database/sql"
	"errors"
	"testing"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/jmoiron/sqlx"
)

func TestReferenceLookupPreservesDatabaseErrors(t *testing.T) {
	statementbuilder.InitialiseStatementBuilder("sqlite3")
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	crud := &DbResource{}
	if _, err := crud.GetReferenceIdToObjectWithTransaction("missing", daptinid.NullReferenceId, tx); err == nil {
		t.Fatal("lookup of missing table succeeded")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := crud.GetReferenceIdToObjectWithTransaction("missing", daptinid.NullReferenceId, tx); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("lookup after rollback: %v", err)
	}
}
