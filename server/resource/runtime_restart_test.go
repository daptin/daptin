package resource

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestRuntimeRestartRequiresCommittedRequestAndNewSnapshot(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	config, err := NewConfigStore(db)
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewRuntimeRestart(db, config)
	if err != nil {
		t.Fatal(err)
	}
	tx, _ := db.Beginx()
	_, missing := config.GetConfigValueFor(restartGenerationKey, "backend", tx)
	_ = tx.Rollback()
	if !errors.Is(missing, sql.ErrNoRows) {
		t.Fatalf("startup wrote a restart marker: %v", missing)
	}
	if err := first.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	tx, _ = db.Beginx()
	if err := first.Request(tx); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	if err := first.Check(context.Background()); err != nil {
		t.Fatal("rolled back restart became visible", err)
	}
	tx, _ = db.Beginx()
	if err := first.Request(tx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := first.Check(context.Background()); !errors.Is(err, ErrRestartPending) {
		t.Fatalf("old snapshot: %v", err)
	}
	next, err := NewRuntimeRestart(db, config)
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Check(context.Background()); err != nil {
		t.Fatal("new snapshot not ready", err)
	}
}
