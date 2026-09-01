package resource

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/table_info"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func TestMeteringDatabaseMatrix(t *testing.T) {
	tests := []struct {
		name    string
		dialect string
		driver  string
		env     string
	}{
		{name: "postgres", dialect: "postgres", driver: "postgres", env: "DAPTIN_TEST_POSTGRES_DSN"},
		{name: "mysql", dialect: "mysql", driver: "mysql", env: "DAPTIN_TEST_MYSQL_DSN"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := os.Getenv(test.env)
			if dsn == "" {
				t.Skipf("set %s to an empty disposable database to run the metering matrix", test.env)
			}
			database, err := sqlx.Open(test.driver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			database.SetMaxOpenConns(16)
			if err := database.Ping(); err != nil {
				t.Fatal(err)
			}
			config := initializeCanonicalMeteringSchema(t, database, test.dialect, map[string]bool{
				"user_account": true,
				"usergroup":    true,
				"api_plan":     true,
				"api_member":   true,
				"api_usage":    true,
				"api_quota":    true,
			})
			cruds, user := newMeteringTestResources(t, database, &config)
			runMeteringDatabaseContract(t, database, cruds, user)
		})
	}
}

func runMeteringDatabaseContract(t *testing.T, database *sqlx.DB, cruds map[string]*DbResource, user *auth.SessionUser) {
	t.Helper()
	now := time.Date(2026, time.September, 1, 15, 0, 0, 0, time.UTC)
	insertMeteringPlanAndMember(t, cruds, user, now, `[{"metric":"requests","window":"minute","maximum":5,"mode":"hard"}]`)
	services := []*MeteringService{NewMeteringService(&cruds), NewMeteringService(&cruds)}
	for _, service := range services {
		service.now = func() time.Time { return now }
	}
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"}
	requestPrefix := uuid.NewString()

	start := make(chan struct{})
	type admissionResult struct {
		admitted bool
		decision *MeteringDecision
		err      error
	}
	results := make(chan admissionResult, 12)
	var workers sync.WaitGroup
	for index := 0; index < 12; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			service := services[index%len(services)]
			tx, err := database.Beginx()
			if err != nil {
				results <- admissionResult{err: fmt.Errorf("begin admission %d: %w", index, err)}
				return
			}
			decision, err := service.Admit(MeteringContext{
				RequestID: fmt.Sprintf("matrix-%s-%d", requestPrefix, index), User: user, Metering: config,
			}, tx)
			if err != nil {
				_ = tx.Rollback()
				results <- admissionResult{err: err}
				return
			}
			if err := tx.Commit(); err != nil {
				results <- admissionResult{err: fmt.Errorf("commit admission %d: %w", index, err)}
				return
			}
			results <- admissionResult{admitted: true, decision: decision}
		}(index)
	}
	close(start)
	workers.Wait()
	close(results)
	accepted := int64(0)
	acceptedDecisions := make([]*MeteringDecision, 0, 5)
	for result := range results {
		if result.admitted {
			accepted++
			acceptedDecisions = append(acceptedDecisions, result.decision)
			continue
		}
		if result.err == nil || !strings.Contains(result.err.Error(), "insufficient_quota") {
			t.Errorf("unexpected admission failure: %v", result.err)
		}
	}
	if accepted != 5 {
		t.Fatalf("accepted %d requests, want exactly 5", accepted)
	}
	completionResults := make(chan error, 2)
	for index := 0; index < 2; index++ {
		go func(service *MeteringService) {
			tx, err := database.Beginx()
			if err == nil {
				err = service.Complete(MeteringContext{User: user, Metering: config}, acceptedDecisions[0], tx)
			}
			if err == nil {
				err = tx.Commit()
			} else if tx != nil {
				_ = tx.Rollback()
			}
			completionResults <- err
		}(services[index])
	}
	for index := 0; index < 2; index++ {
		if err := <-completionResults; err != nil {
			t.Fatalf("concurrent duplicate completion %d: %v", index, err)
		}
	}
	verify, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer verify.Rollback()
	var bucketKey string
	for key := range acceptedDecisions[0].reservation {
		bucketKey = key
	}
	assertMeteringBucket(t, services[0], verify, "requests", bucketKey, accepted-1, 1)
}
