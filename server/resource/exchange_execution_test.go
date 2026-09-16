package resource

import (
	"testing"
	"time"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestExchangeExecutionClaimIsDatabaseAuthoritative(t *testing.T) {
	statementbuilder.InitialiseStatementBuilder("sqlite3")
	database := sqlx.MustOpen("sqlite3", "file:exchange-execution-claim?mode=memory&cache=shared")
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	database.MustExec(`create table data_exchange_execution (
		id integer primary key autoincrement,
		state text not null,
		attempt_count integer not null,
		max_attempts integer not null,
		next_attempt_at timestamp,
		lease_token text,
		lease_expires_at timestamp,
		created_at timestamp,
		updated_at timestamp
	)`)

	fixedNow := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	database.MustExec(`insert into data_exchange_execution
		(state, attempt_count, max_attempts, next_attempt_at, created_at)
		values (?, ?, ?, ?, ?)`, exchangeExecutionPending, 0, exchangeExecutionMaxAttempts,
		fixedNow.Add(-time.Minute), fixedNow)
	service := NewExchangeExecutionService(nil, &map[string]*DbResource{
		"data_exchange_execution": {connection: database},
	})

	first := database.MustBegin()
	claim, err := service.claimNext(first, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if claim == nil || claim.attempts != 1 || claim.leaseToken == "" {
		t.Fatalf("invalid first claim: %#v", claim)
	}
	if err := first.Commit(); err != nil {
		t.Fatal(err)
	}

	second := database.MustBegin()
	staleClaim, err := service.claimNext(second, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if staleClaim != nil {
		t.Fatalf("unexpired running execution was claimed twice: %#v", staleClaim)
	}
	_ = second.Rollback()
}

func TestExchangeExecutionExpiredLeaseCanBeReclaimed(t *testing.T) {
	statementbuilder.InitialiseStatementBuilder("sqlite3")
	database := sqlx.MustOpen("sqlite3", "file:exchange-execution-reclaim?mode=memory&cache=shared")
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	database.MustExec(`create table data_exchange_execution (
		id integer primary key autoincrement,
		state text not null,
		attempt_count integer not null,
		max_attempts integer not null,
		next_attempt_at timestamp,
		lease_token text,
		lease_expires_at timestamp,
		created_at timestamp,
		updated_at timestamp
	)`)
	now := time.Now().UTC()
	database.MustExec(`insert into data_exchange_execution
		(state, attempt_count, max_attempts, lease_token, lease_expires_at, created_at)
		values (?, ?, ?, ?, ?, ?)`, exchangeExecutionRunning, 1, exchangeExecutionMaxAttempts,
		"expired", now.Add(-time.Minute), now.Add(-time.Hour))
	service := NewExchangeExecutionService(nil, &map[string]*DbResource{
		"data_exchange_execution": {connection: database},
	})
	tx := database.MustBegin()
	claim, err := service.claimNext(tx, now)
	if err != nil {
		t.Fatal(err)
	}
	if claim == nil || claim.attempts != 2 || claim.leaseToken == "expired" {
		t.Fatalf("expired execution was not reclaimed: %#v", claim)
	}
	_ = tx.Rollback()
}

func TestExchangeRetryBackoffIsBounded(t *testing.T) {
	if delay := exchangeRetryDelay(1); delay != 2*time.Second {
		t.Fatalf("first retry delay = %s", delay)
	}
	if delay := exchangeRetryDelay(20); delay != time.Hour {
		t.Fatalf("maximum retry delay = %s", delay)
	}
}

func TestExchangeSourceVersionAcceptsResourceRepresentations(t *testing.T) {
	for _, value := range []interface{}{int64(2), 2, float64(2), float32(2), "2", []byte("2")} {
		version, err := exchangeSourceVersion(value)
		if err != nil || version != 2 {
			t.Fatalf("version %T(%v) = %d, %v; want 2", value, value, version, err)
		}
	}
	if _, err := exchangeSourceVersion(2.5); err == nil {
		t.Fatal("fractional source version must be rejected")
	}
}

func TestDataExchangeExecutionSchemaStoresIdentityNotPayload(t *testing.T) {
	var columns map[string]bool
	for _, table := range StandardTables {
		if table.TableName != "data_exchange_execution" {
			continue
		}
		columns = make(map[string]bool, len(table.Columns))
		for _, column := range table.Columns {
			columns[column.ColumnName] = true
		}
		if table.DefaultPermission != 0 {
			t.Fatalf("execution resource permission = %d, want no implicit row access", table.DefaultPermission)
		}
	}
	if columns == nil || !columns["source_reference_id"] || !columns["source_version"] {
		t.Fatalf("execution identity columns are missing: %#v", columns)
	}
	for _, forbidden := range []string{"envelope", "payload", "exchange_name"} {
		if columns[forbidden] {
			t.Fatalf("execution resource stores forbidden duplicate data column %q", forbidden)
		}
	}

	relations := map[string]bool{}
	for _, relation := range StandardRelations {
		if relation.GetSubject() == "data_exchange_execution" {
			relations[relation.GetObjectName()] = true
		}
	}
	if !relations["data_exchange_id"] || !relations["as_user_id"] {
		t.Fatalf("execution authority relations are missing: %#v", relations)
	}
}

func TestRetryDataExchangeExecutionUsesTheActionSubject(t *testing.T) {
	for _, action := range SystemActions {
		if action.Name != "retry_data_exchange_execution" || action.OnType != "data_exchange_execution" {
			continue
		}
		if len(action.OutFields) != 1 || action.OutFields[0].Attributes["subject"] != "~subject" {
			t.Fatalf("retry outcome does not reuse the action subject: %#v", action.OutFields)
		}
		return
	}
	t.Fatal("retry_data_exchange_execution action is missing")
}

func TestManualRetryReturnsTerminalExecutionToScheduledPath(t *testing.T) {
	statementbuilder.InitialiseStatementBuilder("sqlite3")
	database := sqlx.MustOpen("sqlite3", "file:exchange-execution-retry?mode=memory&cache=shared")
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	database.MustExec(`create table data_exchange_execution (
		id integer primary key autoincrement,
		reference_id blob not null unique,
		state text not null,
		attempt_count integer not null,
		max_attempts integer not null,
		next_attempt_at timestamp,
		lease_token text,
		lease_expires_at timestamp,
		last_error_code text,
		last_error_summary text,
		completed_at timestamp,
		updated_at timestamp
	)`)
	referenceID := daptinid.DaptinReferenceId(uuid.New())
	database.MustExec(`insert into data_exchange_execution
		(reference_id, state, attempt_count, max_attempts, completed_at)
		values (?, ?, ?, ?, ?)`, referenceID[:], exchangeExecutionTerminalFailed, 5, 5, time.Now())
	service := NewExchangeExecutionService(nil, &map[string]*DbResource{
		"data_exchange_execution": {connection: database},
	})
	tx := database.MustBegin()
	if err := service.Retry(referenceID, tx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var state string
	var attempts, maxAttempts int64
	if err := database.QueryRow(`select state, attempt_count, max_attempts from data_exchange_execution`).
		Scan(&state, &attempts, &maxAttempts); err != nil {
		t.Fatal(err)
	}
	if state != exchangeExecutionPending || attempts != 5 || maxAttempts != 10 {
		t.Fatalf("retry state = %s/%d/%d, want pending/5/10", state, attempts, maxAttempts)
	}
}

func TestExpiredFinalLeaseBecomesTerminal(t *testing.T) {
	statementbuilder.InitialiseStatementBuilder("sqlite3")
	database := sqlx.MustOpen("sqlite3", "file:exchange-execution-terminal?mode=memory&cache=shared")
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	database.MustExec(`create table data_exchange_execution (
		id integer primary key autoincrement,
		state text not null,
		attempt_count integer not null,
		max_attempts integer not null,
		lease_token text,
		lease_expires_at timestamp,
		last_error_code text,
		last_error_summary text,
		completed_at timestamp,
		updated_at timestamp
	)`)
	now := time.Now().UTC()
	database.MustExec(`insert into data_exchange_execution
		(state, attempt_count, max_attempts, lease_token, lease_expires_at)
		values (?, ?, ?, ?, ?)`, exchangeExecutionRunning, 5, 5, "abandoned", now.Add(-time.Minute))
	service := NewExchangeExecutionService(nil, &map[string]*DbResource{
		"data_exchange_execution": {connection: database},
	})
	tx := database.MustBegin()
	if err := service.terminalizeExhaustedLeases(tx, now); err != nil {
		t.Fatal(err)
	}
	var state, code string
	if err := tx.QueryRow(`select state, last_error_code from data_exchange_execution`).Scan(&state, &code); err != nil {
		t.Fatal(err)
	}
	if state != exchangeExecutionTerminalFailed || code != "lease_expired" {
		t.Fatalf("exhausted lease = %s/%s, want terminal_failed/lease_expired", state, code)
	}
	_ = tx.Rollback()
}
