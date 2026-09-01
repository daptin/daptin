package resource

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/daptin/daptin/server/table_info"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestMeteringLifecycleIsAtomicGenericAndIdempotent(t *testing.T) {
	database := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 10, 0, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, database, now, `[{"metric":"requests","window":"minute","maximum":1,"mode":"hard"}]`)
	service := NewMeteringService(nil)
	service.now = func() time.Time { return now }
	user := &auth.SessionUser{UserId: 7}
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"}

	tx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	decision, err := service.Admit(MeteringContext{
		RequestID: "request-1", User: user, Endpoint: "/items", Method: "GET", RequestType: "crud", Metering: config,
	}, tx)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := service.Admit(MeteringContext{
		RequestID: "request-1", User: user, Endpoint: "/items", Method: "GET", RequestType: "crud", Metering: config,
	}, tx)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.ReservationToken != decision.ReservationToken {
		t.Fatalf("duplicate admission created a second reservation: %q != %q", duplicate.ReservationToken, decision.ReservationToken)
	}
	if _, err := service.Admit(MeteringContext{
		RequestID: "request-1", User: &auth.SessionUser{UserId: 8}, Metering: config,
	}, tx); err == nil {
		t.Fatal("another user reused an existing metering request_id")
	}
	if err := service.Complete(MeteringContext{
		User: &auth.SessionUser{UserId: 8}, Metering: config,
	}, decision, tx); err == nil {
		t.Fatal("another user terminalized a metering reservation")
	}
	assertMeteringBucket(t, tx, "requests", "", 1, 0)
	if err := service.Complete(MeteringContext{
		User: user, Endpoint: "/items", Method: "GET", RequestType: "crud", StatusCode: 200, Metering: config,
	}, decision, tx); err != nil {
		t.Fatal(err)
	}
	if err := service.Complete(MeteringContext{User: user, Metering: config}, decision, tx); err != nil {
		t.Fatalf("duplicate completion must be idempotent: %v", err)
	}
	assertMeteringBucket(t, tx, "requests", "", 0, 1)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	deniedTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Admit(MeteringContext{
		RequestID: "request-2", User: user, Endpoint: "/items", Method: "GET", RequestType: "crud", Metering: config,
	}, deniedTx)
	var limitError api2go.HTTPError
	if !errors.As(err, &limitError) || limitError.Status() != 402 {
		deniedTx.Rollback()
		t.Fatalf("hard limit did not fail closed: %v", err)
	}
	if err := deniedTx.Rollback(); err != nil {
		t.Fatal(err)
	}

	verifyTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer verifyTx.Rollback()
	assertMeteringBucket(t, verifyTx, "requests", "", 0, 1)
	usage, err := service.findUsageByRequestID("request-1", verifyTx)
	if err != nil {
		t.Fatal(err)
	}
	if state := StringOrEmpty(usage["state"]); state != meteringStateCompleted {
		t.Fatalf("usage state = %q, want completed", state)
	}
	if _, err := service.findUsageByRequestID("request-2", verifyTx); !errors.Is(err, errMeteringRowNotFound) {
		t.Fatalf("denied admission survived caller rollback: %v", err)
	}
}

func TestMeteringSettlesActualMeasuresAndReleasesUnusedReservation(t *testing.T) {
	database := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 11, 0, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, database, now, `[{"metric":"total_tokens","window":"month","maximum":100,"mode":"hard"}]`)
	service := NewMeteringService(nil)
	service.now = func() time.Time { return now }
	user := &auth.SessionUser{UserId: 7}
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "total_tokens", CostExpr: "1"}

	firstTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Admit(MeteringContext{
		RequestID: "tokens-1", User: user, Metering: config, EstimatedMeasures: map[string]int64{"total_tokens": 80},
	}, firstTx)
	if err != nil {
		firstTx.Rollback()
		t.Fatal(err)
	}
	assertMeteringBucket(t, firstTx, "total_tokens", "", 80, 0)
	if err := service.Complete(MeteringContext{
		User: user, Metering: config, Measures: map[string]int64{"total_tokens": 20},
	}, first, firstTx); err != nil {
		firstTx.Rollback()
		t.Fatal(err)
	}
	assertMeteringBucket(t, firstTx, "total_tokens", "", 0, 20)
	if err := firstTx.Commit(); err != nil {
		t.Fatal(err)
	}

	deniedTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Admit(MeteringContext{
		RequestID: "tokens-2", User: user, Metering: config, EstimatedMeasures: map[string]int64{"total_tokens": 90},
	}, deniedTx)
	var limitError api2go.HTTPError
	if !errors.As(err, &limitError) || limitError.Status() != 402 {
		deniedTx.Rollback()
		t.Fatalf("actual consumption was not enforced on the next admission: %v", err)
	}
	if err := deniedTx.Rollback(); err != nil {
		t.Fatal(err)
	}

	cancelTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := service.Admit(MeteringContext{
		RequestID: "tokens-3", User: user, Metering: config, EstimatedMeasures: map[string]int64{"total_tokens": 80},
	}, cancelTx)
	if err != nil {
		cancelTx.Rollback()
		t.Fatal(err)
	}
	if err := service.Cancel(MeteringContext{
		User: user, Metering: config, Measures: map[string]int64{"total_tokens": 5},
	}, cancelled, cancelTx); err != nil {
		cancelTx.Rollback()
		t.Fatal(err)
	}
	assertMeteringBucket(t, cancelTx, "total_tokens", "", 0, 25)
	if err := cancelTx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestMeteringConcurrentAdmissionCannotOversubscribe(t *testing.T) {
	database := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, database, now, `[{"metric":"requests","window":"minute","maximum":5,"mode":"hard"}]`)
	service := NewMeteringService(nil)
	service.now = func() time.Time { return now }
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"}
	start := make(chan struct{})
	results := make(chan bool, 16)
	var workers sync.WaitGroup
	for index := 0; index < 16; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			tx, err := database.Beginx()
			if err != nil {
				results <- false
				return
			}
			_, err = service.Admit(MeteringContext{
				RequestID: fmt.Sprintf("concurrent-%d", index), User: &auth.SessionUser{UserId: 7}, Metering: config,
			}, tx)
			if err != nil {
				tx.Rollback()
				results <- false
				return
			}
			if err := tx.Commit(); err != nil {
				results <- false
				return
			}
			results <- true
		}(index)
	}
	close(start)
	workers.Wait()
	close(results)
	accepted := int64(0)
	for admitted := range results {
		if admitted {
			accepted++
		}
	}
	if accepted < 1 || accepted > 5 {
		t.Fatalf("concurrent accepted count = %d, want 1..5", accepted)
	}
	verifyTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer verifyTx.Rollback()
	assertMeteringBucket(t, verifyTx, "requests", "", accepted, 0)
}

func TestMeteringExpiryReleasesReservedEstimateOnce(t *testing.T) {
	database := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 13, 0, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, database, now, `[{"metric":"requests","window":"minute","maximum":10,"mode":"hard"}]`)
	service := NewMeteringService(nil)
	service.now = func() time.Time { return now }
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"}

	admitTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	decision, err := service.Admit(MeteringContext{
		RequestID: "expires", User: &auth.SessionUser{UserId: 7}, Metering: config,
		EstimatedMeasures: map[string]int64{"requests": 3}, ReservationTTL: time.Minute,
	}, admitTx)
	if err != nil {
		admitTx.Rollback()
		t.Fatal(err)
	}
	if err := admitTx.Commit(); err != nil {
		t.Fatal(err)
	}

	service.now = func() time.Time { return now.Add(2 * time.Minute) }
	expireTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	expired, err := service.ExpireReservations(service.now(), 10, expireTx)
	if err != nil {
		expireTx.Rollback()
		t.Fatal(err)
	}
	if expired != 1 {
		expireTx.Rollback()
		t.Fatalf("expired reservations = %d, want 1", expired)
	}
	assertMeteringBucket(t, expireTx, "requests", "", 0, 0)
	usage, err := service.findUsageByToken(decision.ReservationToken, expireTx)
	if err != nil {
		expireTx.Rollback()
		t.Fatal(err)
	}
	if state := StringOrEmpty(usage["state"]); state != meteringStateExpired {
		expireTx.Rollback()
		t.Fatalf("expired usage state = %q", state)
	}
	if err := expireTx.Commit(); err != nil {
		t.Fatal(err)
	}

	idempotentTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer idempotentTx.Rollback()
	expired, err = service.ExpireReservations(service.now(), 10, idempotentTx)
	if err != nil || expired != 0 {
		t.Fatalf("second expiry pass = %d, %v; want 0, nil", expired, err)
	}
}

func TestMeteringScheduledRecoveryUsesCanonicalTransaction(t *testing.T) {
	database := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 13, 30, 0, 0, time.UTC)
	service := NewMeteringService(nil)
	service.now = func() time.Time { return now }
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"}

	admitTransaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Admit(MeteringContext{
		RequestID: "scheduled-expiry", User: &auth.SessionUser{UserId: 7}, Metering: config,
		EstimatedMeasures: map[string]int64{"requests": 1}, ReservationTTL: time.Minute,
	}, admitTransaction); err != nil {
		_ = admitTransaction.Rollback()
		t.Fatal(err)
	}
	if err := admitTransaction.Commit(); err != nil {
		t.Fatal(err)
	}

	cruds := map[string]*DbResource{"api_usage": {connection: database}}
	service = NewMeteringService(&cruds)
	expired, err := service.recoverExpiredReservations(now.Add(2*time.Minute), 100)
	if err != nil || expired != 1 {
		t.Fatalf("scheduled recovery = %d, %v; want 1, nil", expired, err)
	}
	verifyTransaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer verifyTransaction.Rollback()
	usage, err := service.findUsageByRequestID("scheduled-expiry", verifyTransaction)
	if err != nil {
		t.Fatal(err)
	}
	if state := StringOrEmpty(usage["state"]); state != meteringStateExpired {
		t.Fatalf("scheduled recovery state = %q, want expired", state)
	}
}

func TestMeteringFailsClosedWhenPersistenceIsUnavailable(t *testing.T) {
	database := newCanonicalMeteringDatabase(t)
	tx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	service := NewMeteringService(nil)
	_, err = service.Admit(MeteringContext{
		RequestID: "persistence-unavailable", User: &auth.SessionUser{UserId: 7},
		Metering: &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"},
	}, tx)
	if err == nil {
		t.Fatal("metering admission succeeded with an unusable transaction")
	}
}

func TestMeteringFailsClosedOnInvalidQuotaCounters(t *testing.T) {
	database := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 14, 0, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, database, now, `[{"metric":"requests","window":"minute","maximum":10,"mode":"hard"}]`)
	service := NewMeteringService(nil)
	service.now = func() time.Time { return now }
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"}

	firstTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Admit(MeteringContext{
		RequestID: "valid-counter", User: &auth.SessionUser{UserId: 7}, Metering: config,
	}, firstTx); err != nil {
		firstTx.Rollback()
		t.Fatal(err)
	}
	if err := firstTx.Commit(); err != nil {
		t.Fatal(err)
	}

	corruptTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	query, arguments, err := statementbuilder.Squirrel.Update("api_quota").Prepared(true).
		Set(goqu.Record{"maximum": "invalid"}).Where(goqu.Ex{"metric": "requests"}).ToSQL()
	if err != nil {
		corruptTx.Rollback()
		t.Fatal(err)
	}
	if _, err := corruptTx.Exec(query, arguments...); err != nil {
		corruptTx.Rollback()
		t.Fatal(err)
	}
	if err := corruptTx.Commit(); err != nil {
		t.Fatal(err)
	}

	admitTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Admit(MeteringContext{
		RequestID: "invalid-counter", User: &auth.SessionUser{UserId: 7}, Metering: config,
	}, admitTx)
	if err == nil || !strings.Contains(err.Error(), "invalid api_quota maximum") {
		admitTx.Rollback()
		t.Fatalf("admission with an invalid quota counter did not fail closed: %v", err)
	}
	if err := admitTx.Rollback(); err != nil {
		t.Fatal(err)
	}

	verifyTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer verifyTx.Rollback()
	assertMeteringBucket(t, verifyTx, "requests", "", 1, 0)
	if _, err := service.findUsageByRequestID("invalid-counter", verifyTx); !errors.Is(err, errMeteringRowNotFound) {
		t.Fatalf("failed admission survived caller rollback: %v", err)
	}
}

func newCanonicalMeteringDatabase(t *testing.T) *sqlx.DB {
	t.Helper()
	database, err := sqlx.Open("sqlite3", fmt.Sprintf("file:metering-%s?mode=memory&cache=shared&_busy_timeout=10000", uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { database.Close() })
	initializeCanonicalMeteringSchema(t, database, "sqlite3", nil)
	return database
}

func initializeCanonicalMeteringSchema(t *testing.T, database *sqlx.DB, dialect string, included map[string]bool) {
	t.Helper()
	statementbuilder.InitialiseStatementBuilder(dialect)
	config := CmsConfig{Tables: standardTablesForTest(nil)}
	CheckRelations(&config)
	if included != nil {
		tables := make([]table_info.TableInfo, 0, len(included))
		for _, table := range config.Tables {
			if included[table.TableName] {
				tables = append(tables, table)
			}
		}
		config.Tables = tables
	}
	CheckAllTableStatus(&config, database)
	constraintTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	CreateUniqueConstraints(&config, constraintTx)
	if err := constraintTx.Commit(); err != nil {
		t.Fatal(err)
	}
	CreateIndexes(&config, database)
}

func insertMeteringPlanAndMember(t *testing.T, database *sqlx.DB, now time.Time, limits string) {
	t.Helper()
	tx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	planReference := uuid.Must(uuid.NewV7())
	planName := "test-plan-" + uuid.NewString()
	query, arguments, err := statementbuilder.Squirrel.Insert("api_plan").Prepared(true).Rows(goqu.Record{
		"name": planName, "limits": limits, "user_account_id": 7,
		"reference_id": planReference[:], "permission": auth.DEFAULT_PERMISSION, "created_at": now, "updated_at": now,
	}).ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(query, arguments...); err != nil {
		t.Fatal(err)
	}
	var planID int64
	query, arguments, err = statementbuilder.Squirrel.Select("id").Prepared(true).From("api_plan").Where(goqu.Ex{"name": planName}).ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Get(&planID, query, arguments...); err != nil {
		t.Fatal(err)
	}
	memberReference := uuid.Must(uuid.NewV7())
	query, arguments, err = statementbuilder.Squirrel.Insert("api_member").Prepared(true).Rows(goqu.Record{
		"status": "active", "period_start": now, "period_end": now.AddDate(0, 1, 0),
		"api_plan_id": planID, "user_account_id": 7, "reference_id": memberReference[:],
		"permission": auth.DEFAULT_PERMISSION, "created_at": now, "updated_at": now,
	}).ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(query, arguments...); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func assertMeteringBucket(t *testing.T, tx *sqlx.Tx, metric, bucketKey string, reserved, consumed int64) {
	t.Helper()
	selectBucket := statementbuilder.Squirrel.Select("reserved", "consumed").Prepared(true).
		From("api_quota").Where(goqu.Ex{"metric": metric})
	if bucketKey != "" {
		selectBucket = selectBucket.Where(goqu.Ex{"bucket_key": bucketKey})
	}
	query, arguments, err := selectBucket.ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	var actual struct {
		Reserved int64 `db:"reserved"`
		Consumed int64 `db:"consumed"`
	}
	if err := tx.Get(&actual, query, arguments...); err != nil {
		t.Fatal(err)
	}
	if actual.Reserved != reserved || actual.Consumed != consumed {
		t.Fatalf("quota bucket = reserved %d consumed %d, want %d/%d", actual.Reserved, actual.Consumed, reserved, consumed)
	}
}
