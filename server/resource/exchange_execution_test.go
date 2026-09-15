package resource

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestAfterExchangeFailureQueuesRetryWithoutFailingMutation(t *testing.T) {
	database, cruds, _ := newCanonicalMeteringDatabase(t)
	queue := cruds["data_exchange_execution"]
	if queue == nil {
		t.Fatal("canonical data_exchange_execution resource is unavailable")
	}
	configStore, err := NewConfigStore(database)
	if err != nil {
		t.Fatal(err)
	}
	secret := "0123456789abcdef0123456789abcdef"
	configTransaction := database.MustBegin()
	if err := configStore.SetConfigValueFor("encryption.secret", secret, "backend", configTransaction); err != nil {
		_ = configTransaction.Rollback()
		t.Fatal(err)
	}
	if err := configTransaction.Commit(); err != nil {
		t.Fatal(err)
	}
	queue.ConfigStore = configStore
	queue.EncryptionSecret = []byte(secret)

	config := &CmsConfig{ExchangeContracts: []ExchangeContract{{
		Name: "orders", SourceType: "self", TargetType: "invalid", AsUserId: 1,
		Options: map[string]interface{}{"on_error": exchangeOnErrorRetry},
		Attributes: map[string]interface{}{
			"name": "order", "hook": "after", "methods": []interface{}{"patch"},
		},
	}}}
	middleware := NewExchangeMiddleware(config, &cruds)
	transaction := database.MustBegin()
	request := apiRequestWithContext(context.Background(), http.MethodPatch)
	if _, err := middleware.InterceptAfter(nil, &request, []map[string]interface{}{{
		"__type": "order", "reference_id": "01994173-d4d0-7cc5-b168-a433df5f6944", "total": 42,
	}}, transaction); err != nil {
		_ = transaction.Rollback()
		t.Fatal(err)
	}
	var encryptedEnvelope string
	var asUserID int64
	var state, exchangeName string
	if err := transaction.QueryRow(`select envelope, user_account_id, state, exchange_name from data_exchange_execution`).
		Scan(&encryptedEnvelope, &asUserID, &state, &exchangeName); err != nil {
		_ = transaction.Rollback()
		t.Fatal(err)
	}
	if encryptedEnvelope == "" || encryptedEnvelope[0] == '{' {
		_ = transaction.Rollback()
		t.Fatal("exchange envelope was not encrypted by the resource lifecycle")
	}
	if asUserID != 1 {
		_ = transaction.Rollback()
		t.Fatalf("execution owner = %d, want configured exchange user 1", asUserID)
	}
	if state != exchangeExecutionPending || exchangeName != "orders" {
		_ = transaction.Rollback()
		t.Fatalf("queued execution = %s/%s, want orders/%s", exchangeName, state, exchangeExecutionPending)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := database.QueryRow(`select count(*) from data_exchange_execution`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rolled back source transaction retained %d exchange executions", count)
	}
}

func TestRetryQueueFailureAbortsSourceTransaction(t *testing.T) {
	database, cruds, _ := newCanonicalMeteringDatabase(t)
	queue := cruds["data_exchange_execution"]
	configStore, err := NewConfigStore(database)
	if err != nil {
		t.Fatal(err)
	}
	secret := "0123456789abcdef0123456789abcdef"
	configTransaction := database.MustBegin()
	if err := configStore.SetConfigValueFor("encryption.secret", secret, "backend", configTransaction); err != nil {
		_ = configTransaction.Rollback()
		t.Fatal(err)
	}
	if err := configTransaction.Commit(); err != nil {
		t.Fatal(err)
	}
	queue.ConfigStore = configStore
	queue.EncryptionSecret = []byte(secret)
	database.MustExec(`create table exchange_source_marker (value text not null)`)
	database.MustExec(`create trigger reject_exchange_execution before insert on data_exchange_execution
		begin select raise(abort, 'queue unavailable'); end`)

	config := &CmsConfig{ExchangeContracts: []ExchangeContract{{
		Name: "orders", SourceType: "self", TargetType: "invalid", AsUserId: 1,
		Options: map[string]interface{}{"on_error": exchangeOnErrorRetry},
		Attributes: map[string]interface{}{
			"name": "order", "hook": "after", "methods": []interface{}{"post"},
		},
	}}}
	middleware := NewExchangeMiddleware(config, &cruds)
	transaction := database.MustBegin()
	if _, err := transaction.Exec(`insert into exchange_source_marker (value) values ('accepted')`); err != nil {
		t.Fatal(err)
	}
	request := apiRequestWithContext(context.Background(), http.MethodPost)
	if _, err := middleware.InterceptAfter(nil, &request, []map[string]interface{}{{
		"__type": "order", "reference_id": "01994173-d4d0-7cc5-b168-a433df5f6944",
	}}, transaction); err == nil {
		_ = transaction.Rollback()
		t.Fatal("retry policy must fail when the durable execution cannot be created")
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatal(err)
	}

	var markerCount, executionCount int
	if err := database.QueryRow(`select count(*) from exchange_source_marker`).Scan(&markerCount); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`select count(*) from data_exchange_execution`).Scan(&executionCount); err != nil {
		t.Fatal(err)
	}
	if markerCount != 0 || executionCount != 0 {
		t.Fatalf("source markers = %d, executions = %d; want 0, 0", markerCount, executionCount)
	}
}

func apiRequestWithContext(ctx context.Context, method string) api2go.Request {
	return api2go.Request{PlainRequest: (&http.Request{Method: method}).WithContext(ctx)}
}

func TestExchangeExecutionClaimIsDatabaseAuthoritative(t *testing.T) {
	statementbuilder.InitialiseStatementBuilder("sqlite3")
	database := sqlx.MustOpen("sqlite3", "file:exchange-execution-claim?mode=memory&cache=shared")
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { database.Close() })
	createExchangeExecutionClaimTable(t, database, `integer primary key autoincrement`, `timestamp`)
	assertExchangeExecutionClaim(t, database)
}

func TestExchangeExecutionClaimIsDatabaseAuthoritativePostgres(t *testing.T) {
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to an empty disposable database")
	}
	statementbuilder.InitialiseStatementBuilder("postgres")
	database, err := sqlx.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := database.Ping(); err != nil {
		t.Fatal(err)
	}
	database.MustExec(`drop table if exists data_exchange_execution`)
	createExchangeExecutionClaimTable(t, database, `bigserial primary key`, `timestamp with time zone`)
	assertExchangeExecutionClaim(t, database)
}

func createExchangeExecutionClaimTable(t *testing.T, database *sqlx.DB, idType, timestampType string) {
	t.Helper()
	database.MustExec(`create table data_exchange_execution (
		id ` + idType + `,
		exchange_name text not null,
		envelope text not null,
		state text not null,
		attempt_count integer not null,
		user_account_id integer not null,
		next_attempt_at ` + timestampType + `,
		last_error text,
		completed_at ` + timestampType + `,
		created_at ` + timestampType + `,
		updated_at ` + timestampType + `
	)`)
}

func assertExchangeExecutionClaim(t *testing.T, database *sqlx.DB) {
	t.Helper()
	fixedNow := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	encryptionSecret := []byte("0123456789abcdef0123456789abcdef")
	encryptedEnvelope, err := Encrypt(encryptionSecret, `{"__type":"order","reference_id":"order-1"}`)
	if err != nil {
		t.Fatal(err)
	}
	insertSQL := database.Rebind(`insert into data_exchange_execution
		(exchange_name, envelope, state, attempt_count, user_account_id, next_attempt_at, created_at)
		values (?, ?, ?, ?, ?, ?, ?)`)
	database.MustExec(insertSQL, "broken", encryptedEnvelope,
		exchangeExecutionPending, 0, 7, fixedNow.Add(-time.Minute), fixedNow)

	cruds := map[string]*DbResource{"data_exchange_execution": {connection: database, EncryptionSecret: encryptionSecret}}
	service := NewExchangeExecutionService(&CmsConfig{ExchangeContracts: []ExchangeContract{{
		Name: "broken", TargetType: "invalid",
	}}}, &cruds)
	service.now = func() time.Time { return fixedNow }

	transaction := database.MustBegin()
	pendingSnapshot := map[string]interface{}{
		"id": int64(1), "exchange_name": "broken",
		"envelope": encryptedEnvelope, "attempt_count": int64(0), USER_ACCOUNT_ID_COLUMN: int64(7),
	}
	if err := service.ProcessPending(transaction); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatal(err)
	}
	// A second cluster worker can hold the same stale candidate snapshot, but
	// its conditional update must not claim the execution again.
	staleTransaction := database.MustBegin()
	if err := service.processOne(pendingSnapshot, staleTransaction, fixedNow); err != nil {
		t.Fatal(err)
	}
	if err := staleTransaction.Commit(); err != nil {
		t.Fatal(err)
	}

	var state string
	var attempts int64
	var lastError string
	if err := database.QueryRow(`select state, attempt_count, last_error from data_exchange_execution where id = 1`).
		Scan(&state, &attempts, &lastError); err != nil {
		t.Fatal(err)
	}
	if state != exchangeExecutionRetrying {
		t.Fatalf("state = %q, want %q", state, exchangeExecutionRetrying)
	}
	if attempts != 1 {
		t.Fatalf("attempt_count = %d, want exactly one claimed attempt", attempts)
	}
	if lastError == "" {
		t.Fatal("retry must retain a redacted diagnostic")
	}
}

func TestExchangeRetryBackoffIsBounded(t *testing.T) {
	for _, test := range []struct {
		attempts int64
		want     time.Duration
	}{
		{attempts: 1, want: 2 * time.Second},
		{attempts: 20, want: time.Hour},
	} {
		if delay := exchangeRetryDelay(test.attempts); delay != test.want {
			t.Fatalf("attempt %d delay = %s, want %s", test.attempts, delay, test.want)
		}
	}
}
