package resource

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func TestMeteringDeadlinePostgresLockRaces(t *testing.T) {
	dsn := os.Getenv("DAPTIN_TEST_METERING_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_METERING_POSTGRES_DSN to an empty disposable PostgreSQL database")
	}
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { statementbuilder.InitialiseStatementBuilder("sqlite3") })
	schema := initializeCanonicalMeteringSchema(t, db, "postgres", nil)
	cruds, user := newMeteringTestResources(t, db, &schema)
	now := time.Now().UTC()
	insertMeteringPlanAndMember(t, cruds, user, now, `[{"metric":"requests","window":"day","maximum":10,"mode":"hard"}]`)
	service := NewMeteringService(&cruds)
	service.now = func() time.Time { return now }
	config := &table_info.MeteringConfig{Enabled: true}
	admit := func(id string) *MeteringDecision {
		t.Helper()
		tx, err := db.Beginx()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		decision, err := service.Admit(MeteringContext{User: user, RequestID: id, Metering: config}, tx)
		if err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		return decision
	}
	begin := func() *sqlx.Tx {
		t.Helper()
		tx, err := db.Beginx()
		if err != nil {
			t.Fatal(err)
		}
		return tx
	}
	// Observe PostgreSQL's actual lock wait, rather than assuming goroutine order
	// or adding synchronization hooks to production code.
	waitForLock := func(pid int) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			var waiting bool
			err := db.QueryRowx("SELECT COALESCE(wait_event_type = 'Lock', false) FROM pg_stat_activity WHERE pid = $1", pid).Scan(&waiting)
			if err != nil {
				t.Fatal(err)
			}
			if waiting {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("backend %d did not wait on the usage row lock", pid)
	}
	decision := admit("renew-first")
	renewal := begin()
	defer renewal.Rollback()
	if err = service.EnsureReservationDeadline(MeteringContext{User: user}, decision, now.Add(10*time.Minute), renewal); err != nil {
		t.Fatal(err)
	}
	recovery := begin()
	defer recovery.Rollback()
	var pid int
	if err = recovery.QueryRowx("SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatal(err)
	}
	type expiredResult struct {
		count int
		err   error
	}
	expiredDone := make(chan expiredResult, 1)
	go func() {
		count, err := service.ExpireReservations(now.Add(6*time.Minute), 100, recovery)
		expiredDone <- expiredResult{count, err}
	}()
	waitForLock(pid)
	if err = renewal.Commit(); err != nil {
		t.Fatal(err)
	}
	result := <-expiredDone
	if result.err != nil || result.count != 0 {
		t.Fatalf("stale recovery expired renewed work: %+v", result)
	}
	assertMeteringBucket(t, service, recovery, "requests", "", 1, 0)
	if err = recovery.Commit(); err != nil {
		t.Fatal(err)
	}
	// Expiry wins the opposite ordering and a waiting extension must not revive it.
	expiry := begin()
	defer expiry.Rollback()
	if count, err := service.ExpireReservations(now.Add(16*time.Minute), 100, expiry); err != nil || count != 1 {
		t.Fatalf("expiry=%d %v", count, err)
	}
	waiting := begin()
	defer waiting.Rollback()
	if err = waiting.QueryRowx("SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatal(err)
	}
	renewed := make(chan error, 1)
	go func() {
		renewed <- service.EnsureReservationDeadline(MeteringContext{User: user}, decision, now.Add(20*time.Minute), waiting)
	}()
	waitForLock(pid)
	if err = expiry.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-renewed; !errors.Is(err, errMeteringRequestTerminal) {
		t.Fatalf("expiry winner revived: %v", err)
	}
	if err = waiting.Rollback(); err != nil {
		t.Fatal(err)
	}
	// Completion/cancellation and extension use exactly the same row lock.
	for _, cancelled := range []bool{false, true} {
		id := "complete-first"
		if cancelled {
			id = "cancel-first"
		}
		decision = admit(id)
		terminal := begin()
		defer terminal.Rollback()
		if cancelled {
			err = service.Cancel(MeteringContext{User: user, Measures: map[string]int64{"requests": 0, "bytes": 0}}, decision, terminal)
		} else {
			err = service.Complete(MeteringContext{User: user, Metering: config, StatusCode: 200}, decision, terminal)
		}
		if err != nil {
			t.Fatal(err)
		}
		competing := begin()
		if err = competing.QueryRowx("SELECT pg_backend_pid()").Scan(&pid); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			done <- service.EnsureReservationDeadline(MeteringContext{User: user}, decision, now.Add(time.Minute), competing)
		}()
		waitForLock(pid)
		if err = terminal.Commit(); err != nil {
			t.Fatal(err)
		}
		if err = <-done; !errors.Is(err, errMeteringRequestTerminal) {
			t.Fatalf("terminal winner revived: %v", err)
		}
		assertMeteringBucket(t, service, competing, "requests", "", 0, 1)
		if err = competing.Rollback(); err != nil {
			t.Fatal(err)
		}
	}
}
