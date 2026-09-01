package resource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/daptin/daptin/server/table_info"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestMeteringLifecycleIsAtomicGenericAndIdempotent(t *testing.T) {
	database, cruds, user := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 10, 0, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, cruds, user, now, `[{"metric":"requests","window":"minute","maximum":1,"mode":"hard"}]`)
	service := NewMeteringService(&cruds)
	service.now = func() time.Time { return now }
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
	mismatched := *decision
	mismatched.RequestID = "another-request"
	if err := service.Complete(MeteringContext{User: user, Metering: config}, &mismatched, tx); err == nil {
		t.Fatal("mismatched request_id terminalized a metering reservation")
	}
	assertMeteringBucket(t, service, tx, "requests", "", 1, 0)
	if err := service.Complete(MeteringContext{
		User: user, Endpoint: "/items", Method: "GET", RequestType: "crud", StatusCode: 200, Metering: config,
	}, decision, tx); err != nil {
		t.Fatal(err)
	}
	if err := service.Complete(MeteringContext{User: user, Metering: config}, decision, tx); err != nil {
		t.Fatalf("duplicate completion must be idempotent: %v", err)
	}
	assertMeteringBucket(t, service, tx, "requests", "", 0, 1)
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
	assertMeteringBucket(t, service, verifyTx, "requests", "", 0, 1)
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
	database, cruds, user := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 11, 0, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, cruds, user, now, `[{"metric":"total_tokens","window":"month","maximum":100,"mode":"hard"}]`)
	service := NewMeteringService(&cruds)
	service.now = func() time.Time { return now }
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
	assertMeteringBucket(t, service, firstTx, "total_tokens", "", 80, 0)
	if err := service.Complete(MeteringContext{
		User: user, Metering: config, Measures: map[string]int64{"total_tokens": 20},
	}, first, firstTx); err != nil {
		firstTx.Rollback()
		t.Fatal(err)
	}
	assertMeteringBucket(t, service, firstTx, "total_tokens", "", 0, 20)
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
	assertMeteringBucket(t, service, cancelTx, "total_tokens", "", 0, 25)
	if err := cancelTx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestMeteringReconstructedDecisionUsesCompletionConfig(t *testing.T) {
	database, cruds, user := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 11, 30, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, cruds, user, now, `[{"metric":"compute_units","window":"month","maximum":100,"mode":"hard"}]`)
	service := NewMeteringService(&cruds)
	service.now = func() time.Time { return now }
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "compute_units", CostExpr: "response.units"}

	admitTransaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := service.Admit(MeteringContext{
		RequestID: "reconstructed-decision", User: user, Metering: config,
		EstimatedMeasures: map[string]int64{"compute_units": 10},
	}, admitTransaction)
	if err != nil {
		_ = admitTransaction.Rollback()
		t.Fatal(err)
	}
	if err := admitTransaction.Commit(); err != nil {
		t.Fatal(err)
	}

	terminalTransaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	reconstructed := &MeteringDecision{
		Enabled: true, RequestID: admitted.RequestID, ReservationToken: admitted.ReservationToken,
	}
	if err := service.Complete(MeteringContext{
		User: user, Metering: config, Response: map[string]interface{}{"units": 3},
	}, reconstructed, terminalTransaction); err != nil {
		_ = terminalTransaction.Rollback()
		t.Fatal(err)
	}
	assertMeteringBucket(t, service, terminalTransaction, "compute_units", "", 0, 3)
	if err := terminalTransaction.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestMeteringConcurrentAdmissionCannotOversubscribe(t *testing.T) {
	database, cruds, user := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, cruds, user, now, `[{"metric":"requests","window":"minute","maximum":5,"mode":"hard"}]`)
	service := NewMeteringService(&cruds)
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
				RequestID: fmt.Sprintf("concurrent-%d", index), User: user, Metering: config,
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
	assertMeteringBucket(t, service, verifyTx, "requests", "", accepted, 0)
}

func TestMeteringExpiryReleasesReservedEstimateOnce(t *testing.T) {
	database, cruds, user := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 13, 0, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, cruds, user, now, `[{"metric":"requests","window":"minute","maximum":10,"mode":"hard"}]`)
	service := NewMeteringService(&cruds)
	service.now = func() time.Time { return now }
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"}

	admitTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	decision, err := service.Admit(MeteringContext{
		RequestID: "expires", User: user, Metering: config,
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
	assertMeteringBucket(t, service, expireTx, "requests", "", 0, 0)
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
	database, cruds, user := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 13, 30, 0, 0, time.UTC)
	service := NewMeteringService(&cruds)
	service.now = func() time.Time { return now }
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"}

	admitTransaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Admit(MeteringContext{
		RequestID: "scheduled-expiry", User: user, Metering: config,
		EstimatedMeasures: map[string]int64{"requests": 1}, ReservationTTL: time.Minute,
	}, admitTransaction); err != nil {
		_ = admitTransaction.Rollback()
		t.Fatal(err)
	}
	if err := admitTransaction.Commit(); err != nil {
		t.Fatal(err)
	}

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
	database, _, _ := newCanonicalMeteringDatabase(t)
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

func TestMeteringRequestIDBoundMatchesGatewayProtocol(t *testing.T) {
	database, cruds, user := newCanonicalMeteringDatabase(t)
	transaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback()
	service := NewMeteringService(&cruds)
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"}
	if _, err := service.Admit(MeteringContext{
		RequestID: strings.Repeat("a", 128), User: user, Metering: config,
	}, transaction); err != nil {
		t.Fatalf("128-character request_id: %v", err)
	}
	if _, err := service.Admit(MeteringContext{
		RequestID: strings.Repeat("b", 129), User: user, Metering: config,
	}, transaction); err == nil {
		t.Fatal("129-character request_id was accepted")
	}
}

func TestMeteringFailsClosedOnInvalidQuotaCounters(t *testing.T) {
	database, cruds, user := newCanonicalMeteringDatabase(t)
	now := time.Date(2026, time.September, 1, 14, 0, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, cruds, user, now, `[{"metric":"requests","window":"minute","maximum":10,"mode":"hard"}]`)
	service := NewMeteringService(&cruds)
	service.now = func() time.Time { return now }
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"}

	firstTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Admit(MeteringContext{
		RequestID: "valid-counter", User: user, Metering: config,
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
	usage, err := service.findUsageByRequestID("valid-counter", corruptTx)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := service.decisionFromUsage(usage, config, corruptTx)
	if err != nil {
		t.Fatal(err)
	}
	var bucketKey string
	for key := range decision.reservation {
		bucketKey = key
	}
	quota, err := service.findQuota(bucketKey, corruptTx)
	if err != nil {
		corruptTx.Rollback()
		t.Fatal(err)
	}
	quotaModel := api2go.NewApi2GoModelWithData("api_quota", cruds["api_quota"].TableInfo().Columns,
		int64(cruds["api_quota"].TableInfo().DefaultPermission), cruds["api_quota"].TableInfo().Relations, quota)
	quotaModel.SetAttributes(map[string]interface{}{"maximum": "invalid"})
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch,
		URL: &url.URL{Path: "/api_quota/" + daptinid.InterfaceToDIR(quota["reference_id"]).String()}}).
		WithContext(context.WithValue(context.Background(), "user", user))}
	if _, err := cruds["api_quota"].UpdateWithoutFilters(quotaModel, request, corruptTx); err != nil {
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
		RequestID: "invalid-counter", User: user, Metering: config,
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
	assertMeteringBucket(t, service, verifyTx, "requests", "", 1, 0)
	if _, err := service.findUsageByRequestID("invalid-counter", verifyTx); !errors.Is(err, errMeteringRowNotFound) {
		t.Fatalf("failed admission survived caller rollback: %v", err)
	}
}

func newCanonicalMeteringDatabase(t *testing.T) (*sqlx.DB, map[string]*DbResource, *auth.SessionUser) {
	t.Helper()
	database, err := sqlx.Open("sqlite3", fmt.Sprintf("file:metering-%s?mode=memory&cache=shared&_busy_timeout=10000", uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { database.Close() })
	config := initializeCanonicalMeteringSchema(t, database, "sqlite3", nil)
	cruds, user := newMeteringTestResources(t, database, &config)
	return database, cruds, user
}

func initializeCanonicalMeteringSchema(t *testing.T, database *sqlx.DB, dialect string, included map[string]bool) CmsConfig {
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
	return config
}

func newMeteringTestResources(t *testing.T, database *sqlx.DB, config *CmsConfig) (map[string]*DbResource, *auth.SessionUser) {
	t.Helper()
	bootstrap, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateWorldTable(config, bootstrap); err != nil {
		_ = bootstrap.Rollback()
		t.Fatalf("bootstrap canonical Daptin data: %v", err)
	}
	if err := bootstrap.Commit(); err != nil {
		t.Fatal(err)
	}
	references, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	adminReference, err := GetIdToReferenceIdWithTransaction("usergroup", 2, references)
	if err != nil {
		_ = references.Rollback()
		t.Fatalf("resolve canonical administrator group: %v", err)
	}
	bootstrapUserReference, err := GetIdToReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, 1, references)
	if err != nil {
		_ = references.Rollback()
		t.Fatalf("resolve canonical bootstrap user: %v", err)
	}
	if err := references.Commit(); err != nil {
		t.Fatal(err)
	}
	userReference := daptinid.DaptinReferenceId(uuid.New())
	otherReference := daptinid.DaptinReferenceId(uuid.New())
	cruds := make(map[string]*DbResource)
	previous := make(map[string]*DbResource)
	for index := range config.Tables {
		table := config.Tables[index]
		if table.TableName != USER_ACCOUNT_TABLE_NAME && table.TableName != "api_plan" && table.TableName != "api_member" &&
			table.TableName != "api_usage" && table.TableName != "api_quota" {
			continue
		}
		previous[table.TableName] = CRUD_MAP[table.TableName]
		crud := &DbResource{model: api2go.NewApi2GoModel(table.TableName, table.Columns, int64(table.DefaultPermission), table.Relations),
			connection: database, tableInfo: &table, Cruds: cruds, ms: &MiddlewareSet{}, AdministratorGroupId: adminReference}
		cruds[table.TableName] = crud
		CRUD_MAP[table.TableName] = crud
	}
	t.Cleanup(func() {
		for name, crud := range previous {
			if crud == nil {
				delete(CRUD_MAP, name)
			} else {
				CRUD_MAP[name] = crud
			}
		}
	})
	createTransaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer createTransaction.Rollback()
	administrator := &auth.SessionUser{UserId: 1, UserReferenceId: bootstrapUserReference,
		Groups: auth.GroupPermissionList{{GroupReferenceId: adminReference}}}
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/user_account"}}).
		WithContext(context.WithValue(context.Background(), "user", administrator))}
	for _, attributes := range []map[string]interface{}{
		{"name": "Metering Test", "email": "metering@example.test", "reference_id": userReference.String()},
		{"name": "Other Metering Test", "email": "other-metering@example.test", "reference_id": otherReference.String()},
	} {
		_, createErr := cruds[USER_ACCOUNT_TABLE_NAME].CreateWithoutFilter(
			api2go.NewApi2GoModelWithData(USER_ACCOUNT_TABLE_NAME, nil, 0, nil, attributes), request, createTransaction,
		)
		if createErr != nil {
			t.Fatalf("create metering user through canonical resource path: %v", createErr)
		}
	}
	userID, err := GetReferenceIdToIdWithTransaction(USER_ACCOUNT_TABLE_NAME, userReference, createTransaction)
	if err != nil {
		t.Fatalf("resolve created metering user: %v", err)
	}
	if err := createTransaction.Commit(); err != nil {
		t.Fatal(err)
	}
	return cruds, &auth.SessionUser{UserId: userID, UserReferenceId: userReference}
}

func insertMeteringPlanAndMember(t *testing.T, cruds map[string]*DbResource, user *auth.SessionUser, now time.Time, limits string) {
	t.Helper()
	tx, err := cruds["api_plan"].Connection().Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	planName := "test-plan-" + uuid.NewString()
	administrator := *user
	administrator.Groups = auth.GroupPermissionList{{GroupReferenceId: cruds["api_plan"].AdministratorGroupId}}
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/api_plan"}}).
		WithContext(context.WithValue(context.Background(), "user", &administrator))}
	plan, err := cruds["api_plan"].CreateWithoutFilter(api2go.NewApi2GoModelWithData("api_plan", nil, 0, nil,
		map[string]interface{}{"name": planName, "limits": limits}), request, tx)
	if err != nil {
		t.Fatal(err)
	}
	request.PlainRequest.URL.Path = "/api_member"
	if _, err := cruds["api_member"].CreateWithoutFilter(api2go.NewApi2GoModelWithData("api_member", nil, 0, nil,
		map[string]interface{}{"status": "active", "period_start": now, "period_end": now.AddDate(0, 1, 0),
			"api_plan_id": daptinid.InterfaceToDIR(plan["reference_id"]).String()}), request, tx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func assertMeteringBucket(t *testing.T, service *MeteringService, tx *sqlx.Tx, metric, bucketKey string, reserved, consumed int64) {
	t.Helper()
	var row map[string]interface{}
	var err error
	if bucketKey != "" {
		row, err = service.findQuota(bucketKey, tx)
	} else {
		var rows []map[string]interface{}
		rows, _, err = (*service.cruds)["api_quota"].GetRowsByWhereClauseWithTransaction("api_quota", nil, tx, goqu.Ex{"metric": metric})
		if err == nil && len(rows) != 1 {
			t.Fatalf("quota rows for %s = %d, want 1", metric, len(rows))
		}
		if len(rows) == 1 {
			row = rows[0]
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	actualReserved, reservedErr := ResourceRowInt64(row["reserved"])
	actualConsumed, consumedErr := ResourceRowInt64(row["consumed"])
	if reservedErr != nil || consumedErr != nil {
		t.Fatalf("invalid quota counters: %v, %v", reservedErr, consumedErr)
	}
	if actualReserved != reserved || actualConsumed != consumed {
		t.Fatalf("quota bucket = reserved %d consumed %d, want %d/%d", actualReserved, actualConsumed, reserved, consumed)
	}
}
