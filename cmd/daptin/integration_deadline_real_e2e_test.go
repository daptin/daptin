package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

// This acceptance test deliberately crosses the real five-minute default lease.
// No server clock or reservation duration is shortened for the test.
func TestIntegrationReservationDeadlineRealE2E(t *testing.T) {
	requireRealE2E(t)
	if os.Getenv("DAPTIN_LONG_INTEGRATION_E2E") != "1" {
		t.Skip("set DAPTIN_LONG_INTEGRATION_E2E=1 for the >5 minute provider acceptance test")
	}
	for _, backend := range []struct{ name, driver, dsn string }{{"sqlite", "sqlite3", ""}, {"postgres", "postgres", os.Getenv("DAPTIN_TEST_POSTGRES_DSN")}} {
		t.Run(backend.name, func(t *testing.T) {
			if backend.driver == "postgres" && backend.dsn == "" {
				t.Skip("set DAPTIN_TEST_POSTGRES_DSN to an empty disposable database")
			}
			var calls, slowStarts, serviceCalls, cancellations atomic.Int64
			cancelStarted := make(chan struct{}, 1)
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch r.URL.Path {
				case "/slow":
					slowStarts.Add(1)
					if r.Header.Get("Authorization") == "Bearer service-token" {
						serviceCalls.Add(1)
					}
					timer := time.NewTimer(330 * time.Second)
					defer timer.Stop()
					select {
					case <-timer.C:
					case <-r.Context().Done():
						cancellations.Add(1)
						return
					}
				case "/timeout", "/cancel":
					if r.URL.Path == "/cancel" {
						cancelStarted <- struct{}{}
					}
					<-r.Context().Done()
					cancellations.Add(1)
					return
				}
				transportE2EWriteJSON(w, map[string]interface{}{"ok": true})
			}))
			t.Cleanup(provider.Close)
			schema := `EnableGraphQL: true
Tables:
  - TableName: integration
    Metering:
      on_actions:
        lease.example/slow: {enabled: true, meter_type: requests}
        lease.example/quick: {enabled: true, meter_type: requests}
        lease.example/timeout: {enabled: true, meter_type: requests}
        lease.example/cancel: {enabled: true, meter_type: requests}
        lease_workflow: {enabled: true, meter_type: requests}
`
			dsn := backend.dsn
			if dsn == "" {
				dsn = filepath.Join(t.TempDir(), "lease.db")
			}
			options := transportE2EDaptinOptions{databaseType: backend.driver, connectionString: dsn, schema: schema}
			fixture := startDaptinE2E(t, options)
			admin := fixture.SignupAdmin(t)
			customer := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "lease-customer")
			user := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "user_account", "email", "lease-customer@test.local")
			_ = accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "lease-service")
			serviceUser := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "user_account", "email", "lease-service@test.local")
			patch := func(kind, id string, attrs map[string]interface{}) {
				t.Helper()
				accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/"+kind+"/"+id, admin, accessGroupsE2ERecordPayload(kind, id, attrs), http.StatusOK)
			}
			credential := transportE2ECreateCredential(t, fixture.Client, fixture.URL, admin)
			patch("credential", credential, map[string]interface{}{"user_account_id": user})
			serviceCredential := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "credential", map[string]interface{}{"name": "lease-service-token", "content": `{"token":"service-token"}`})
			patch("credential", serviceCredential, map[string]interface{}{"user_account_id": serviceUser})
			paths := map[string]interface{}{}
			for _, op := range []string{"slow", "quick", "timeout", "cancel"} {
				timeout := 600000
				if op == "timeout" {
					timeout = 250
				}
				if op == "quick" {
					timeout = 10000
				}
				paths["/"+op] = map[string]interface{}{"get": map[string]interface{}{"operationId": op, "x-daptin-timeout-ms": timeout, "x-daptin-require-membership": true, "responses": transportE2EJSONResponses()}}
			}
			integration := transportE2ECreateIntegration(t, fixture.Client, fixture.URL, admin, "lease.example", transportE2EBaseSpec("Lease", provider.URL, paths))
			transportE2EInstallIntegration(t, fixture.Client, fixture.URL, admin, integration)
			world := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "world", "table_name", "integration")
			patch("world", world, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)})
			for _, op := range []string{"slow", "quick", "timeout", "cancel"} {
				action := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "action", "action_name", "lease.example/"+op)
				patch("action", action, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)})
			}
			plan := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "api_plan", map[string]interface{}{"name": "Lease plan", "limits": `[{"metric":"requests","window":"day","maximum":5,"mode":"hard"}]`})
			makeAction := func(name string, fields []map[string]interface{}, outcomes []map[string]interface{}) {
				t.Helper()
				definition, _ := json.Marshal(map[string]interface{}{"Name": name, "OnType": "integration", "InstanceOptional": true, "InFields": fields, "OutFields": outcomes})
				id := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "action", map[string]interface{}{"action_name": name, "label": name, "world_id": world, "instance_optional": true, "action_schema": string(definition)})
				patch("action", id, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)})
			}
			now := time.Now().UTC()
			makeAction("assign_lease_member", []map[string]interface{}{{"Name": "user_ref", "ColumnName": "user_ref", "ColumnType": "label"}}, []map[string]interface{}{
				{"Type": "__as_user", "Method": "SWITCH_USER", "Attributes": map[string]interface{}{"user_reference_id": "~user_ref"}},
				{"Type": "api_member", "Method": "POST", "Attributes": map[string]interface{}{"api_plan_id": plan, "status": "active", "period_start": now.Add(-time.Hour).Format(time.RFC3339), "period_end": now.Add(time.Hour).Format(time.RFC3339)}},
			})
			for _, ref := range []string{user, serviceUser} {
				accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/integration/assign_lease_member", admin, map[string]interface{}{"attributes": map[string]interface{}{"user_ref": ref}}, http.StatusOK)
			}
			makeAction("lease_workflow", nil, []map[string]interface{}{
				{"Type": "lease.example", "Method": "quick", "Attributes": map[string]interface{}{"credential_id": credential}},
				{"Type": "__as_user", "Method": "SWITCH_USER", "Attributes": map[string]interface{}{"user_reference_id": serviceUser}},
				{"Type": "lease.example", "Method": "slow", "Attributes": map[string]interface{}{"credential_id": serviceCredential}},
			})
			fixture.Process.stopProcess()
			fixture = startDaptinE2E(t, options)
			readUsage := func() []interface{} {
				t.Helper()
				return accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/api_usage?page[size]=1000", admin, nil, http.StatusOK))
			}
			quotas := func() (float64, float64) {
				t.Helper()
				var reserved, consumed float64
				for _, row := range accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/api_quota?page[size]=1000", admin, nil, http.StatusOK)) {
					a := row.(map[string]interface{})["attributes"].(map[string]interface{})
					reserved += a["reserved"].(float64)
					consumed += a["consumed"].(float64)
				}
				return reserved, consumed
			}
			// Timeout errors must still converge through the ordinary failure path.
			for _, route := range []string{"/integration/lease.example/timeout", "/action/integration/lease.example/timeout", "/graphql"} {
				payload := map[string]interface{}{"credential_id": credential, "input": map[string]interface{}{}}
				if strings.HasPrefix(route, "/action/") {
					payload = map[string]interface{}{"attributes": map[string]interface{}{"credential_id": credential}}
				}
				if route == "/graphql" {
					response := transportE2EPostJSON(t, fixture.Client, fixture.URL+route, customer, map[string]interface{}{"query": fmt.Sprintf(`mutation { executeTimeoutOnLeaseexample(credential_id: %q) { ResponseType } }`, credential)})
					if !transportE2EGraphQLHasErrors(response) {
						t.Fatal("timeout GraphQL succeeded")
					}
				} else {
					accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+route, customer, payload, http.StatusGatewayTimeout)
				}
			}
			// A client disconnect must stop external work and settle zero measures.
			ctx, cancel := context.WithCancel(context.Background())
			encoded, _ := json.Marshal(map[string]interface{}{"credential_id": credential, "input": map[string]interface{}{}})
			request, _ := http.NewRequestWithContext(ctx, http.MethodPost, fixture.URL+"/integration/lease.example/cancel", bytes.NewReader(encoded))
			request.Header.Set("Authorization", "Bearer "+customer)
			request.Header.Set("Content-Type", "application/json")
			cancelDone := make(chan error, 1)
			go func() {
				response, err := fixture.Client.Do(request)
				if response != nil {
					response.Body.Close()
				}
				cancelDone <- err
			}()
			select {
			case <-cancelStarted:
			case <-time.After(10 * time.Second):
				cancel()
				t.Fatal("cancel provider not reached")
			}
			cancel()
			select {
			case err := <-cancelDone:
				if err == nil {
					t.Fatal("client cancellation succeeded")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("client did not cancel")
			}
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				reserved, consumed := quotas()
				if reserved == 0 && consumed == 0 && cancellations.Load() == 4 {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if reserved, consumed := quotas(); reserved != 0 || consumed != 0 || cancellations.Load() != 4 {
				t.Fatalf("failure controls: quota=%v/%v cancellations=%d", reserved, consumed, cancellations.Load())
			}
			// Persist the task and restart to use the normal task loading/admission path.
			attrs, _ := json.Marshal(map[string]interface{}{"credential_id": credential})
			task := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "task", map[string]interface{}{"job_type": "action", "name": "lease-task", "entity_name": "integration", "action_name": "lease.example/slow", "schedule": "@every 10s", "active": true, "attributes": string(attrs), "as_user_id": user})
			fixture.Process.stopProcess()
			fixture = startDaptinE2E(t, options)
			type result struct {
				route  string
				status int
				body   []byte
				err    error
			}
			results := make(chan result, 4)
			longClient := &http.Client{Timeout: 11 * time.Minute}
			for _, route := range []string{"/integration/lease.example/slow", "/action/integration/lease.example/slow", "/graphql", "/action/integration/lease_workflow"} {
				payload := map[string]interface{}{"credential_id": credential, "input": map[string]interface{}{}}
				if strings.HasPrefix(route, "/action/") {
					payload = map[string]interface{}{"attributes": map[string]interface{}{"credential_id": credential}}
				}
				if route == "/graphql" {
					payload = map[string]interface{}{"query": fmt.Sprintf(`mutation { executeSlowOnLeaseexample(credential_id: %q) { ResponseType } }`, credential)}
				}
				body, _ := json.Marshal(payload)
				go func(route string, body []byte) {
					request, _ := http.NewRequest(http.MethodPost, fixture.URL+route, bytes.NewReader(body))
					request.Header.Set("Authorization", "Bearer "+customer)
					request.Header.Set("Content-Type", "application/json")
					response, err := longClient.Do(request)
					r := result{route: route, err: err}
					if response != nil {
						r.status = response.StatusCode
						r.body, r.err = io.ReadAll(response.Body)
						response.Body.Close()
					}
					results <- r
				}(route, body)
			}
			deadline = time.Now().Add(30 * time.Second)
			for slowStarts.Load() < 5 && time.Now().Before(deadline) {
				time.Sleep(100 * time.Millisecond)
			}
			patch("task", task, map[string]interface{}{"active": false})
			if slowStarts.Load() != 5 {
				t.Fatalf("slow calls=%d, want four transports/workflows plus one scheduled task", slowStarts.Load())
			}
			t.Log("five real provider calls running with 600-second deadlines; observing beyond five-minute reservation TTL")
			observationEnd := time.Now().Add(305 * time.Second)
			nextLog := time.Now().Add(50 * time.Second)
			for time.Now().Before(observationEnd) {
				if reserved, consumed := quotas(); reserved != 5 || consumed != 0 {
					t.Fatalf("live capacity released: reserved=%v consumed=%v", reserved, consumed)
				}
				if time.Now().After(nextLog) {
					t.Logf("live quota retained; %s until observation completes", time.Until(observationEnd).Round(time.Second))
					nextLog = time.Now().Add(50 * time.Second)
				}
				time.Sleep(2 * time.Second)
			}
			before := calls.Load()
			accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/integration/lease.example/quick", customer, map[string]interface{}{"credential_id": credential, "input": map[string]interface{}{}}, http.StatusPaymentRequired)
			if calls.Load() != before {
				t.Fatal("hard-limit denial reached provider")
			}
			for range 4 {
				select {
				case r := <-results:
					if r.err != nil || r.status != 200 {
						t.Fatalf("%s: HTTP %d err=%v body=%s", r.route, r.status, r.err, r.body)
					}
					if r.route == "/graphql" {
						var response interface{}
						if err := json.Unmarshal(r.body, &response); err != nil || transportE2EGraphQLHasErrors(response) {
							t.Fatalf("long GraphQL failed: %s", r.body)
						}
					}
				case <-time.After(time.Minute):
					t.Fatal("long action did not complete")
				}
			}
			deadline = time.Now().Add(time.Minute)
			for time.Now().Before(deadline) {
				reserved, consumed := quotas()
				if reserved == 0 && consumed == 5 {
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
			if reserved, consumed := quotas(); reserved != 0 || consumed != 5 {
				t.Fatalf("final quota=%v/%v", reserved, consumed)
			}
			completed, cancelled := 0, 0
			scheduled := false
			for _, row := range readUsage() {
				a := row.(map[string]interface{})["attributes"].(map[string]interface{})
				name, _ := a["action_name"].(string)
				if !strings.HasPrefix(name, "lease.example/") && name != "lease_workflow" {
					continue
				}
				if a["user_account_id"] != user {
					t.Fatalf("wrong admission owner: %#v", a)
				}
				switch a["state"] {
				case "completed":
					completed++
					if a["status_code"] != float64(200) {
						t.Fatalf("bad success status: %#v", a)
					}
					if a["method"] == "EXECUTE" && a["endpoint"] == "/action/integration" {
						scheduled = true
					}
				case "cancelled":
					cancelled++
					for _, value := range a["measures"].(map[string]interface{}) {
						if value != float64(0) {
							t.Fatalf("failed work consumed quota: %#v", a)
						}
					}
				default:
					t.Fatalf("nonterminal or expired work: %#v", a)
				}
			}
			if completed != 5 || cancelled != 4 || !scheduled || serviceCalls.Load() != 1 {
				t.Fatalf("completed=%d cancelled=%d scheduled=%v switched-provider=%d", completed, cancelled, scheduled, serviceCalls.Load())
			}
			t.Log("five completed customer-owned actions, four cancelled controls; quota reserved=0 consumed=5 after real >5 minute work")
		})
	}
}
