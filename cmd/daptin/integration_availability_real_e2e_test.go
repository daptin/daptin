package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

func TestIntegrationAvailabilityRealE2E(t *testing.T) {
	requireRealE2E(t)
	for _, backend := range []struct{ name, driver, dsn string }{{"sqlite", "sqlite3", ""}, {"postgres", "postgres", os.Getenv("DAPTIN_TEST_POSTGRES_DSN")}} {
		t.Run(backend.name, func(t *testing.T) {
			if backend.driver == "postgres" && backend.dsn == "" {
				t.Skip("set DAPTIN_TEST_POSTGRES_DSN to an empty disposable database")
			}
			var calls atomic.Int64
			var mode atomic.Int32
			var followups atomic.Int64
			followup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				followups.Add(1)
				transportE2EWriteJSON(w, map[string]interface{}{"ok": true})
			}))
			defer followup.Close()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch mode.Load() {
				case 1:
					<-r.Context().Done()
					return
				case 2:
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"fixture-provider-secret":`)
					return
				case 4:
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"ok":`)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				case 5, 6, 7, 8, 9:
					status := map[int32]int{5: 201, 6: 202, 7: 400, 8: 422, 9: 500}[mode.Load()]
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					if status >= 400 {
						fmt.Fprint(w, `{"detail":"provider rejected input"}`)
					} else {
						fmt.Fprint(w, `{"ok":true}`)
					}
					return
				case 3:
					w.WriteHeader(http.StatusServiceUnavailable)
					fmt.Fprint(w, `{"error":"provider-maintenance"}`)
					return
				}
				transportE2EWriteJSON(w, map[string]interface{}{"ok": true})
			})
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			providerAddress := listener.Addr().String()
			provider := &http.Server{Handler: handler}
			go provider.Serve(listener)
			defer func() { provider.Close() }()
			restartProvider := func() {
				t.Helper()
				listener, err := net.Listen("tcp", providerAddress)
				if err != nil {
					t.Fatal(err)
				}
				provider = &http.Server{Handler: handler}
				go provider.Serve(listener)
			}
			used := map[int]bool{}
			port, httpsPort := freeTransportE2EPort(t, used), freeTransportE2EPort(t, used)
			base := fmt.Sprintf("http://127.0.0.1:%d", port)
			dsn := backend.dsn
			if dsn == "" {
				dsn = filepath.Join(t.TempDir(), "availability.db")
			}
			options := transportE2EDaptinOptions{databaseType: backend.driver, connectionString: dsn, olricPort: freeTransportE2EPortPair(t, used), schema: "EnableGraphQL: true\n"}
			process := startTransportE2EDaptin(t, port, httpsPort, base, options)
			defer func() { process.stopProcess() }()
			client := &http.Client{Timeout: 20 * time.Second}
			admin := accessGroupsE2ESignupSigninAdmin(t, client, base)
			customer := accessGroupsE2ESignupSigninUser(t, client, base, admin, "availability-customer")
			user := accessGroupsE2EFindResourceID(t, client, base, admin, "user_account", "email", "availability-customer@test.local")
			_ = accessGroupsE2ESignupSigninUser(t, client, base, admin, "availability-service")
			serviceUser := accessGroupsE2EFindResourceID(t, client, base, admin, "user_account", "email", "availability-service@test.local")
			patch := func(kind, id string, attrs map[string]interface{}) {
				t.Helper()
				accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/"+kind+"/"+id, admin, accessGroupsE2ERecordPayload(kind, id, attrs), http.StatusOK)
			}
			credential := transportE2ECreateCredential(t, client, base, admin)
			patch("credential", credential, map[string]interface{}{"user_account_id": user})
			serviceCredential := accessGroupsE2ECreateRecord(t, client, base, admin, "credential", map[string]interface{}{"name": "availability-service-token", "content": `{"token":"service-token"}`})
			patch("credential", serviceCredential, map[string]interface{}{"user_account_id": serviceUser})
			integration := transportE2ECreateIntegration(t, client, base, admin, "availability.example", transportE2EBaseSpec("Availability", "http://"+providerAddress, map[string]interface{}{
				"/invoke/{record_id}": map[string]interface{}{"get": map[string]interface{}{
					"operationId": "invoke", "x-daptin-timeout-ms": 250,
					"parameters": []interface{}{map[string]interface{}{"name": "record_id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}}, map[string]interface{}{"name": "probe", "in": "query", "schema": map[string]interface{}{"type": "string"}}},
					"responses":  transportE2EJSONResponses(),
				}},
			}))
			transportE2EInstallIntegration(t, client, base, admin, integration)
			world := accessGroupsE2EFindResourceID(t, client, base, admin, "world", "table_name", "integration")
			action := accessGroupsE2EFindResourceID(t, client, base, admin, "action", "action_name", "availability.example/invoke")
			patch("world", world, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)})
			patch("action", action, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)})
			options.schema = fmt.Sprintf(`EnableGraphQL: true
Tables:
  - TableName: integration
    Metering:
      on_actions:
        availability.example/invoke: {enabled: true, meter_type: requests, cost_expr: "1"}
        availability_workflow: {enabled: true, meter_type: successful_generations, cost_expr: "request.status_code === 201 ? 2 : 1"}
Actions:
  - Name: assign_availability_member
    OnType: integration
    InstanceOptional: true
    InFields:
      - {Name: api_plan_id, ColumnName: api_plan_id, ColumnType: label}
    OutFields:
      - Type: __as_user
        Method: SWITCH_USER
        Attributes: {user_reference_id: %q}
      - Type: api_member
        Method: POST
        Attributes:
          status: active
          api_plan_id: "~api_plan_id"
          period_start: %q
          period_end: %q
  - Name: availability_workflow
    OnType: integration
    InstanceOptional: true
    Permission: %d
    InFields: []
    OutFields:
      - Type: __as_user
        Method: SWITCH_USER
        Attributes: {user_reference_id: %q}
      - Type: availability.example
        Method: invoke
        Attributes:
          credential_id: %q
          record_id: record
          probe: fixture-provider-secret
      - Type: $network.request
        Method: EXECUTE
        Attributes:
          Url: %q
          Method: POST
`, user, time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), time.Now().UTC().Add(time.Hour).Format(time.RFC3339), auth.AuthenticatedExecute, serviceUser, serviceCredential, followup.URL)
			process.stopProcess()
			process = startTransportE2EDaptin(t, port, httpsPort, base, options)
			plan := accessGroupsE2ECreateRecord(t, client, base, admin, "api_plan", map[string]interface{}{"name": "Availability plan", "limits": `[{"metric":"requests","window":"day","maximum":100,"mode":"hard"},{"metric":"successful_generations","window":"day","maximum":100,"mode":"hard"}]`})
			accessGroupsE2EAssertStatus(t, client, http.MethodPost, base+"/action/integration/assign_availability_member", admin, map[string]interface{}{"attributes": map[string]interface{}{"api_plan_id": plan}}, http.StatusOK)
			attributes := map[string]interface{}{"credential_id": credential, "record_id": "record", "probe": "fixture-provider-secret"}
			invoke := func(route int, want int) {
				t.Helper()
				path := "/integration/availability.example/invoke"
				payload := map[string]interface{}{"credential_id": credential, "input": attributes}
				if route == 1 {
					path = "/action/integration/availability.example/invoke"
					payload = map[string]interface{}{"attributes": attributes}
				}
				if route == 2 {
					response := transportE2EPostJSON(t, client, base+"/graphql", customer, map[string]interface{}{"query": fmt.Sprintf(`mutation { executeInvokeOnAvailabilityexample(credential_id: %q, record_id: "record", record_id_2: "record", probe: "fixture-provider-secret") { ResponseType } }`, credential)})
					failed := want >= 400
					if transportE2EGraphQLHasErrors(response) != failed {
						t.Fatalf("GraphQL failed=%v, want %v: %#v", transportE2EGraphQLHasErrors(response), failed, response)
					}
					if failed {
						encoded, _ := json.Marshal(response)
						if strings.Contains(string(encoded), providerAddress) || strings.Contains(string(encoded), "fixture-provider-secret") || strings.Contains(string(encoded), "dial tcp") {
							t.Fatalf("GraphQL leaked upstream details: %s", encoded)
						}
					}
					return
				}
				status, body := postLLMActionForStatus(t, client, base+path, customer, payload)
				expected := want
				if route == 1 && want < 400 {
					expected = http.StatusOK
				}
				if status != expected {
					t.Fatalf("route=%d HTTP %d, want %d: %s", route, status, expected, body)
				}
				if route == 0 && (want == 400 || want == 422 || want == 500 || want == 503) {
					var preserved map[string]interface{}
					if err := json.Unmarshal(body, &preserved); err != nil {
						t.Fatalf("provider rejection lost JSON: %s", body)
					}
					if want == 503 {
						if preserved["error"] != "provider-maintenance" || len(preserved) != 1 {
							t.Fatalf("provider rejection changed: %s", body)
						}
					} else if preserved["detail"] != "provider rejected input" || len(preserved) != 1 {
						t.Fatalf("provider rejection changed: %s", body)
					}
				}
				if want == 502 || want == 504 {
					text := string(body)
					if strings.Contains(text, providerAddress) || strings.Contains(text, "fixture-provider-secret") || strings.Contains(text, "dial tcp") {
						t.Fatalf("upstream details leaked: %s", body)
					}
				}
			}
			allRoutes := func(label string, want int, delta int64) {
				t.Helper()
				before := calls.Load()
				for route := 0; route < 3; route++ {
					invoke(route, want)
				}
				if calls.Load() != before+delta {
					t.Fatalf("%s provider calls=%d, want %d", label, calls.Load(), before+delta)
				}
				t.Logf("%s: all routes verified, provider calls +%d", label, delta)
			}
			allRoutes("healthy", 200, 3)
			provider.Close()
			allRoutes("provider stopped", 502, 0)
			// A wrapper meters its original caller even after a trusted SWITCH_USER.
			status, body := postLLMActionForStatus(t, client, base+"/action/integration/availability_workflow", customer, map[string]interface{}{"attributes": map[string]interface{}{"user_reference_id": user}})
			if status != 502 {
				t.Fatalf("wrapper HTTP %d: %s", status, body)
			}
			restartProvider()
			allRoutes("provider restarted", 200, 3)
			mode.Store(1)
			allRoutes("timeout", 504, 3)
			mode.Store(4)
			allRoutes("response body timeout", 504, 3)
			mode.Store(2)
			allRoutes("malformed upstream response", 502, 3)
			for _, test := range []struct {
				mode   int32
				status int
			}{{5, 201}, {6, 202}, {7, 400}, {8, 422}, {9, 500}, {3, 503}} {
				mode.Store(test.mode)
				allRoutes(fmt.Sprintf("provider HTTP %d", test.status), test.status, 3)
			}
			mode.Store(8)
			status, body = postLLMActionForStatus(t, client, base+"/action/integration/availability_workflow", customer, map[string]interface{}{"attributes": map[string]interface{}{}})
			if status != 422 || followups.Load() != 0 {
				t.Fatalf("rejected workflow ran follow-up: HTTP %d calls=%d body=%s", status, followups.Load(), body)
			}
			mode.Store(5)
			status, body = postLLMActionForStatus(t, client, base+"/action/integration/availability_workflow", customer, map[string]interface{}{"attributes": map[string]interface{}{}})
			if status != 200 || followups.Load() != 1 {
				t.Fatalf("healthy switched workflow: HTTP %d calls=%d body=%s", status, followups.Load(), body)
			}
			mode.Store(0)
			before := calls.Load()
			accessGroupsE2EAssertStatus(t, client, http.MethodPost, base+"/integration/availability.example/invoke", "", map[string]interface{}{"credential_id": credential, "input": attributes}, http.StatusForbidden)
			for _, route := range []string{"/integration/availability.example/invoke", "/action/integration/availability.example/invoke"} {
				payload := map[string]interface{}{"credential_id": credential, "input": map[string]interface{}{"record_id": 123}}
				if strings.HasPrefix(route, "/action/") {
					payload = map[string]interface{}{"attributes": map[string]interface{}{"credential_id": credential, "record_id": 123}}
				}
				accessGroupsE2EAssertStatus(t, client, http.MethodPost, base+route, customer, payload, http.StatusBadRequest)
			}
			if calls.Load() != before {
				t.Fatal("invalid/denied requests reached provider")
			}
			// Persist a scheduled action through the API, then restart so the normal
			// scheduler loads it. Failure must not depend on HTTP payload middleware.
			provider.Close()
			taskAttrs, _ := json.Marshal(attributes)
			task := accessGroupsE2ECreateRecord(t, client, base, admin, "task", map[string]interface{}{"job_type": "action", "name": "availability-task", "entity_name": "integration", "action_name": "availability.example/invoke", "schedule": "@every 1s", "active": true, "attributes": string(taskAttrs), "as_user_id": user})
			process.stopProcess()
			process = startTransportE2EDaptin(t, port, httpsPort, base, options)
			readUsage := func() []interface{} {
				t.Helper()
				query := url.QueryEscape(`[{"column":"entity_type","operator":"is","value":"integration"}]`)
				return accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/api_usage?page[size]=1000&query="+query, admin, nil, http.StatusOK))
			}
			deadline := time.Now().Add(15 * time.Second)
			scheduled := false
			for time.Now().Before(deadline) {
				for _, row := range readUsage() {
					a := row.(map[string]interface{})["attributes"].(map[string]interface{})
					if a["method"] == "EXECUTE" && a["endpoint"] == "/action/integration" && a["state"] == "cancelled" {
						scheduled = true
					}
				}
				if scheduled {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if !scheduled {
				t.Fatal("scheduled action did not terminalize transport failure")
			}
			mode.Store(8)
			restartProvider()
			deadline = time.Now().Add(15 * time.Second)
			scheduled = false
			for time.Now().Before(deadline) {
				for _, row := range readUsage() {
					a := row.(map[string]interface{})["attributes"].(map[string]interface{})
					if a["method"] == "EXECUTE" && a["endpoint"] == "/action/integration" && a["state"] == "cancelled" && a["status_code"] == float64(422) {
						scheduled = true
					}
				}
				if scheduled {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			patch("task", task, map[string]interface{}{"active": false})
			if !scheduled {
				t.Fatal("scheduled action did not terminalize HTTP 422")
			}
			// Let an already claimed task finish before examining durable accounting.
			time.Sleep(300 * time.Millisecond)
			completed, cancelled := 0, 0
			completedStatuses := map[int]int{}
			cancelledStatuses := map[int]int{}
			for _, row := range readUsage() {
				a := row.(map[string]interface{})["attributes"].(map[string]interface{})
				if a["action_name"] != "availability.example/invoke" && a["action_name"] != "availability_workflow" {
					continue
				}
				if a["user_account_id"] != user {
					t.Fatalf("wrong reservation owner after SWITCH_USER: %#v", a)
				}
				switch a["state"] {
				case "completed":
					completed++
					completedStatuses[int(a["status_code"].(float64))]++
				case "cancelled":
					cancelled++
					cancelledStatuses[int(a["status_code"].(float64))]++
					measures := a["measures"].(map[string]interface{})
					for metric, value := range measures {
						if value != float64(0) {
							t.Fatalf("rejected work consumed %s=%v", metric, value)
						}
					}
					if a["status_code"].(float64) < 400 {
						t.Fatalf("wrong failure status: %#v", a)
					}
				default:
					t.Fatalf("reservation not terminal: %#v", a)
				}
			}
			if completed != 13 || cancelled < 28 {
				t.Fatalf("completed/cancelled=%d/%d, want 13/at least 28", completed, cancelled)
			}
			if completedStatuses[200] != 6 || completedStatuses[201] != 4 || completedStatuses[202] != 3 {
				t.Fatalf("successful provider statuses lost: %v", completedStatuses)
			}
			for status, count := range map[int]int{400: 3, 500: 3, 503: 3, 504: 6} {
				if cancelledStatuses[status] != count {
					t.Fatalf("cancelled status %d count=%d, want %d", status, cancelledStatuses[status], count)
				}
			}
			if cancelledStatuses[422] < 5 || cancelledStatuses[502] < 8 {
				t.Fatalf("scheduled failures lost: %v", cancelledStatuses)
			}
			quotas := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/api_quota", admin, nil, http.StatusOK))
			consumed := map[string]float64{}
			for _, row := range quotas {
				a := row.(map[string]interface{})["attributes"].(map[string]interface{})
				if a["reserved"] != float64(0) {
					t.Fatalf("held quota after failed requests: %#v", a)
				}
				consumed[a["metric"].(string)] += a["consumed"].(float64)
			}
			if consumed["requests"] != 13 || consumed["successful_generations"] != 2 {
				t.Fatalf("quota consumed=%v, want requests=13 and successful_generations=2", consumed)
			}
			t.Logf("%s: %d completed and %d cancelled customer-owned usage rows; quota reserved=0 consumed=%v", backend.name, completed, cancelled, consumed)
		})
	}
}
