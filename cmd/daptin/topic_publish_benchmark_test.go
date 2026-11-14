package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	"golang.org/x/net/websocket"
)

// This opt-in measurement requires a disposable PostgreSQL database with
// pg_stat_statements enabled and otherwise identical baseline/fixed binaries.
// Product setup uses resource APIs; SQL reads only database execution statistics.
func TestTopicPublishThroughputPostgres(t *testing.T) {
	if os.Getenv("DAPTIN_TOPIC_BENCHMARK") != "1" {
		t.Skip("set DAPTIN_TOPIC_BENCHMARK=1 and DAPTIN_TOPIC_BENCH_CONFIG")
	}
	t.Setenv("DAPTIN_MAX_OPEN_CONNECTIONS", "50")
	t.Setenv("DAPTIN_MAX_IDLE_CONNECTIONS", "10")
	var config struct{ DSN, Baseline, Fixed, Results string }
	data, err := os.ReadFile(os.Getenv("DAPTIN_TOPIC_BENCH_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if config.DSN == "" || config.Baseline == "" || config.Fixed == "" || config.Results == "" {
		t.Fatal("benchmark config is incomplete")
	}
	database, err := sql.Open("postgres", config.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	options := transportE2EDaptinOptions{databaseType: "postgres", connectionString: config.DSN}
	t.Setenv("DAPTIN_E2E_BINARY", config.Fixed)
	fixture := startDaptinE2E(t, options)
	admin := accessGroupsE2ESignupSigninAdmin(t, fixture.Client, fixture.URL)
	ftpE2ESetConfig(t, fixture.Client, fixture.URL, admin, "limit.rate",
		`{"version":"1","limits":{"/action/world/publish_to_topic":1000000}}`)
	grantGroup := accessGroupsE2ECreateUsergroup(t, fixture.Client, fixture.URL, admin, "bench-publishers")
	worldID := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "world", "table_name", "world")
	accessGroupsE2ECreateJoin(t, fixture.Client, fixture.URL, admin, "world_world_id_has_usergroup_usergroup_id",
		map[string]interface{}{"world_id": worldID, "usergroup_id": grantGroup}, int64(auth.GroupCreate))
	actionID := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "action", "action_name", "publish_to_topic")
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/action/"+actionID, admin,
		accessGroupsE2ERecordPayload("action", actionID, map[string]interface{}{"permission": int64(auth.GuestExecute)}), http.StatusOK)
	type subject struct {
		Name, Token, ID string
		Groups          int
	}
	subjects := make([]subject, 2)
	for i, extra := range []int{0, 19} {
		name := fmt.Sprintf("bench-publisher-%d", extra)
		token := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, name)
		id := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "user_account", "email", name+"@test.local")
		accessGroupsE2ECreateJoin(t, fixture.Client, fixture.URL, admin, "user_account_user_account_id_has_usergroup_usergroup_id",
			map[string]interface{}{"user_account_id": id, "usergroup_id": grantGroup}, 0)
		for j := 0; j < extra; j++ {
			group := accessGroupsE2ECreateUsergroup(t, fixture.Client, fixture.URL, admin, fmt.Sprintf("bench-extra-%d", j))
			accessGroupsE2ECreateJoin(t, fixture.Client, fixture.URL, admin, "user_account_user_account_id_has_usergroup_usergroup_id",
				map[string]interface{}{"user_account_id": id, "usergroup_id": group}, 0)
		}
		cfg, err := websocket.NewConfig("ws"+strings.TrimPrefix(fixture.URL, "http")+"/live", fixture.URL)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Header.Set("Authorization", "Bearer "+token)
		conn, err := websocket.DialConfig(cfg)
		if err != nil {
			t.Fatal(err)
		}
		frame := recvJSON(t, conn, 5*time.Second)
		conn.Close()
		var session struct {
			Groups []string `json:"groups"`
		}
		if err := json.Unmarshal(frame.Data, &session); err != nil {
			t.Fatal(err)
		}
		subjects[i] = subject{Name: name, Token: token, ID: id, Groups: len(session.Groups)}
	}
	fixture.Process.stopProcess()

	type result struct {
		Variant                           string `json:"variant"`
		Trial, Groups, Concurrency        int
		Requests, Failures                int
		Seconds, RPS, P50ms, P95ms, P99ms float64
		Delivered, Duplicates             int
		MembershipQueries                 int64
		MembershipDBms                    float64
		MembershipRows                    int64
		SQLCalls                          int64
		PoolWaitCount                     int64
		PoolWaitMs                        float64
		MaxOpenConnections                int
		Error                             string `json:"error,omitempty"`
	}
	var results []result
	writeResults := func() {
		encoded, err := json.MarshalIndent(map[string]interface{}{"database": "PostgreSQL 17.5", "duration_seconds": 3, "pool_max_open": 50, "pool_max_idle": 10, "results": results}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(config.Results, encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Counterbalanced run order reduces systematic warm-cache and order bias.
	for run, variant := range []string{"baseline", "fixed", "fixed", "baseline", "baseline", "fixed"} {
		binary := config.Fixed
		if variant == "baseline" {
			binary = config.Baseline
		}
		t.Setenv("DAPTIN_E2E_BINARY", binary)
		fixture = startDaptinE2E(t, options)
		transport := &http.Transport{MaxIdleConns: 128, MaxIdleConnsPerHost: 128}
		client := &http.Client{Transport: transport, Timeout: 30 * time.Second}
		observer := websocketRowAuthDial(t, fixture.URL, admin, "world")
		if err := observer.SetReadDeadline(time.Time{}); err != nil {
			t.Fatal(err)
		}
		type deliveries struct {
			Seen              map[uint64]bool
			Count, Duplicates int
		}
		received := make(map[string]*deliveries)
		var receiveMu sync.Mutex
		readerDone := make(chan error, 1)
		go func() {
			for {
				var event struct {
					Type, Topic, Event, Source string
					Data                       []byte
				}
				if err := websocket.JSON.Receive(observer, &event); err != nil {
					readerDone <- err
					return
				}
				if event.Type != "event" || event.Topic != "world" || event.Event != "new-message" {
					continue
				}
				var message struct {
					Phase    string `json:"phase"`
					Sequence uint64 `json:"sequence"`
				}
				if err := json.Unmarshal(event.Data, &message); err != nil {
					readerDone <- err
					return
				}
				receiveMu.Lock()
				d := received[message.Phase]
				if d != nil {
					if d.Seen[message.Sequence] {
						d.Duplicates++
					} else {
						d.Seen[message.Sequence] = true
						d.Count++
					}
				}
				receiveMu.Unlock()
			}
		}()
		dbStats := func() sql.DBStats {
			response := accessGroupsE2ERequestJSON(t, client, http.MethodGet, fixture.URL+"/statistics", admin, nil, http.StatusOK)
			encoded, _ := json.Marshal(response)
			var stats struct {
				DB sql.DBStats `json:"db"`
			}
			if err := json.Unmarshal(encoded, &stats); err != nil {
				t.Fatal(err)
			}
			return stats.DB
		}
		statementStats := func() (int64, float64, int64, int64) {
			rows, err := database.Query(`SELECT query, calls, total_exec_time, rows FROM pg_stat_statements WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())`)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var membershipCalls, membershipRows, totalCalls int64
			var membershipMs float64
			for rows.Next() {
				var query string
				var calls, nrows int64
				var ms float64
				if err := rows.Scan(&query, &calls, &ms, &nrows); err != nil {
					t.Fatal(err)
				}
				totalCalls += calls
				lower := strings.ToLower(query)
				if strings.Contains(lower, `from "user_account"`) && strings.Contains(lower, `"groupreferenceid"`) && strings.Contains(lower, `"user_account"."reference_id"`) {
					membershipCalls += calls
					membershipRows += nrows
					membershipMs += ms
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			return membershipCalls, membershipMs, membershipRows, totalCalls
		}
		var sequence atomic.Uint64
		invoke := func(token, phase string) error {
			body, err := json.Marshal(map[string]interface{}{"attributes": map[string]interface{}{"topicName": "world", "message": map[string]interface{}{"phase": phase, "sequence": sequence.Add(1)}}})
			if err != nil {
				return err
			}
			req, err := http.NewRequest(http.MethodPost, fixture.URL+"/action/world/publish_to_topic", bytes.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			response, err := client.Do(req)
			if err != nil {
				return err
			}
			reply, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				return err
			}
			if response.StatusCode != http.StatusOK || !bytes.Contains(reply, []byte(`"publish_to_topic"`)) {
				return fmt.Errorf("HTTP %d: %.160s", response.StatusCode, reply)
			}
			return nil
		}
		for _, user := range subjects {
			for _, concurrency := range []int{1, 8, 32, 64} {
				var warm atomic.Int64
				var wg sync.WaitGroup
				for w := 0; w < concurrency; w++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for warm.Add(1) <= 100 {
							if err := invoke(user.Token, "warmup"); err != nil {
								t.Error(err)
								return
							}
						}
					}()
				}
				wg.Wait()
				if t.Failed() {
					t.FailNow()
				}
				phase := fmt.Sprintf("%s-%d-%d-%d", variant, run, user.Groups, concurrency)
				receiveMu.Lock()
				received[phase] = &deliveries{Seen: make(map[uint64]bool)}
				receiveMu.Unlock()
				beforePool := dbStats()
				beforeCalls, beforeMs, beforeRows, beforeSQL := statementStats()
				start := time.Now()
				deadline := start.Add(3 * time.Second)
				type workerResult struct {
					Latencies []float64
					Failures  int
					Error     string
				}
				samples := make(chan workerResult, concurrency)
				for w := 0; w < concurrency; w++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						sample := workerResult{}
						for time.Now().Before(deadline) {
							begin := time.Now()
							err := invoke(user.Token, phase)
							sample.Latencies = append(sample.Latencies, float64(time.Since(begin))/float64(time.Millisecond))
							if err != nil {
								sample.Failures++
								if sample.Error == "" {
									sample.Error = err.Error()
								}
							}
						}
						samples <- sample
					}()
				}
				wg.Wait()
				elapsed := time.Since(start).Seconds()
				close(samples)
				afterCalls, afterMs, afterRows, afterSQL := statementStats()
				afterPool := dbStats()
				var latencies []float64
				failures := 0
				firstError := ""
				for sample := range samples {
					latencies = append(latencies, sample.Latencies...)
					failures += sample.Failures
					if firstError == "" {
						firstError = sample.Error
					}
				}
				sort.Float64s(latencies)
				percentile := func(p float64) float64 {
					if len(latencies) == 0 {
						return 0
					}
					index := int(float64(len(latencies)-1) * p)
					return latencies[index]
				}
				deliveryDeadline := time.Now().Add(5 * time.Second)
				delivered, duplicates := 0, 0
				for {
					receiveMu.Lock()
					delivered = received[phase].Count
					duplicates = received[phase].Duplicates
					receiveMu.Unlock()
					if delivered >= len(latencies)-failures || time.Now().After(deliveryDeadline) {
						break
					}
					select {
					case err := <-readerDone:
						t.Fatalf("subscriber stopped: %v", err)
					default:
					}
					time.Sleep(5 * time.Millisecond)
				}
				row := result{Variant: variant, Trial: run/2 + 1, Groups: user.Groups, Concurrency: concurrency, Requests: len(latencies), Failures: failures, Seconds: elapsed, RPS: float64(len(latencies)-failures) / elapsed, P50ms: percentile(.50), P95ms: percentile(.95), P99ms: percentile(.99), Delivered: delivered, Duplicates: duplicates, MembershipQueries: afterCalls - beforeCalls, MembershipDBms: afterMs - beforeMs, MembershipRows: afterRows - beforeRows, SQLCalls: afterSQL - beforeSQL, PoolWaitCount: afterPool.WaitCount - beforePool.WaitCount, PoolWaitMs: float64(afterPool.WaitDuration-beforePool.WaitDuration) / float64(time.Millisecond), MaxOpenConnections: afterPool.MaxOpenConnections, Error: firstError}
				results = append(results, row)
				writeResults()
				t.Logf("%s trial=%d groups=%d c=%d requests=%d failed=%d delivered=%d rps=%.0f p95=%.2fms p99=%.2fms membership_queries=%d pool_waits=%d", variant, row.Trial, row.Groups, concurrency, row.Requests, failures, delivered, row.RPS, row.P95ms, row.P99ms, row.MembershipQueries, row.PoolWaitCount)
				if failures != 0 || delivered != len(latencies) || duplicates != 0 {
					t.Fatalf("benchmark publication failed: %+v", row)
				}
			}
		}
		observer.Close()
		<-readerDone
		transport.CloseIdleConnections()
		fixture.Process.stopProcess()
	}
}
