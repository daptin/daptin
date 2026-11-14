package resource

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"strings"
	"testing"
	"time"
)

// This is a database-layer regression of the durable SMTP claim. No DNS, SMTP,
// mail account fixtures or external services are involved.
func TestMailOutboxClaimSurvivesLostLiveLease(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`CREATE TABLE outbox (id INTEGER PRIMARY KEY, sent BOOLEAN, retry_count INTEGER, next_retry_at DATETIME, last_error TEXT)`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, row := range []struct {
		id    int
		sent  bool
		retry int
		next  interface{}
	}{
		{1, false, 0, nil}, {2, true, 0, nil}, {3, false, 5, nil}, {4, false, 0, now.Add(time.Hour)},
	} {
		if _, err = db.Exec(`INSERT INTO outbox (id,sent,retry_count,next_retry_at) VALUES (?,?,?,?)`, row.id, row.sent, row.retry, row.next); err != nil {
			t.Fatal(err)
		}
	}
	crud := &DbResource{}
	claim := func(id int64, at time.Time) bool {
		t.Helper()
		tx, err := db.Beginx()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		claimed, err := crud.ClaimOutboxMail(id, at, tx)
		if err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		return claimed
	}
	if !claim(1, now) {
		t.Fatal("ready mail not claimed")
	}
	var marker string
	if err = db.Get(&marker, `SELECT last_error FROM outbox WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(marker, OutboxSMTPUncertainPrefix) {
		t.Fatalf("missing durable uncertainty: %q", marker)
	}
	// A new worker, even well after any in-memory/Olric lease expired, cannot send
	// this message again without an explicit operator reconciliation decision.
	if claim(1, now.Add(24*time.Hour)) {
		t.Fatal("crashed SMTP attempt automatically reclaimed")
	}
	for _, id := range []int64{2, 3, 4} {
		if claim(id, now) {
			t.Fatalf("claimed ineligible outbox row %d", id)
		}
	}
	if !claim(4, now.Add(2*time.Hour)) {
		t.Fatal("scheduled retry not eligible when due")
	}
}
