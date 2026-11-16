package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

// Uses the real resource/action lifecycle, a durable database and an actual
// provider listener. No fixture SQL or fake action dispatch is involved.
func TestIntegrationMembershipPreflightRealE2E(t *testing.T) {
	requireRealE2E(t)
	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		transportE2EWriteJSON(w, map[string]interface{}{"ok": true})
	}))
	defer provider.Close()
	used := map[int]bool{}
	port, httpsPort := freeTransportE2EPort(t, used), freeTransportE2EPort(t, used)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	options := transportE2EDaptinOptions{databaseType: "sqlite3", connectionString: filepath.Join(t.TempDir(), "membership.db"), olricPort: freeTransportE2EPortPair(t, used), schema: "EnableGraphQL: true\n"}
	process := startTransportE2EDaptin(t, port, httpsPort, base, options)
	defer func() { process.stopProcess() }()
	client := &http.Client{Timeout: 20 * time.Second}
	admin := accessGroupsE2ESignupSigninAdmin(t, client, base)
	customer := accessGroupsE2ESignupSigninUser(t, client, base, admin, "membership-customer")
	user := accessGroupsE2EFindResourceID(t, client, base, admin, "user_account", "email", "membership-customer@test.local")
	adminUser := accessGroupsE2EFindResourceID(t, client, base, admin, "user_account", "email", "admin@test.local")
	credential := transportE2ECreateCredential(t, client, base, admin)
	patch := func(kind, id string, attrs map[string]interface{}) {
		t.Helper()
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/"+kind+"/"+id, admin, accessGroupsE2ERecordPayload(kind, id, attrs), http.StatusOK)
	}
	patch("credential", credential, map[string]interface{}{"user_account_id": user})
	integration := transportE2ECreateIntegration(t, client, base, admin, "membership.example", transportE2EBaseSpec("Membership", provider.URL, map[string]interface{}{
		"/guarded": map[string]interface{}{"get": map[string]interface{}{"operationId": "invoke", "x-daptin-require-membership": true, "responses": transportE2EJSONResponses()}},
		"/free":    map[string]interface{}{"get": map[string]interface{}{"operationId": "free", "responses": transportE2EJSONResponses()}},
	}))
	transportE2EInstallIntegration(t, client, base, admin, integration)
	world := accessGroupsE2EFindResourceID(t, client, base, admin, "world", "table_name", "integration")
	patch("world", world, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)})
	for _, name := range []string{"invoke", "free"} {
		id := accessGroupsE2EFindResourceID(t, client, base, admin, "action", "action_name", "membership.example/"+name)
		patch("action", id, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)})
	}
	// Configure metering only for the guarded operation. Membership provisioning
	// uses SWITCH_USER so ownership comes from the active Daptin account.
	options.schema = fmt.Sprintf(`EnableGraphQL: true
Tables:
  - TableName: integration
    Metering:
      on_actions:
        membership.example/invoke:
          enabled: true
          meter_type: requests
          cost_expr: "1"
Actions:
  - Name: assign_membership
    OnType: integration
    InstanceOptional: true
    InFields:
      - Name: status
        ColumnName: status
        ColumnType: label
      - Name: period_start
        ColumnName: period_start
        ColumnType: label
      - Name: period_end
        ColumnName: period_end
        ColumnType: label
      - Name: api_plan_id
        ColumnName: api_plan_id
        ColumnType: label
    OutFields:
      - Type: __as_user
        Method: SWITCH_USER
        Attributes:
          user_reference_id: %q
      - Type: api_member
        Method: POST
        Attributes:
          status: "~status"
          period_start: "~period_start"
          period_end: "~period_end"
          api_plan_id: "~api_plan_id"
`, user)
	// Fixed backend workflows test active identity after SWITCH_USER in both
	// directions. The administrator's transient groups cannot satisfy the gate.
	for _, wrapper := range []struct{ name, target string }{{"switch_to_member", user}, {"switch_to_nonmember", adminUser}} {
		options.schema += fmt.Sprintf(`
  - Name: %s
    OnType: integration
    InstanceOptional: true
    Permission: %d
    InFields: []
    OutFields:
      - Type: __as_user
        Method: SWITCH_USER
        Attributes:
          user_reference_id: %q
      - Type: api_member.require_current
        Method: EXECUTE
        SkipInResponse: true
      - Type: membership.example
        Method: invoke
        Attributes:
          credential_id: %q
`, wrapper.name, auth.AuthenticatedExecute, wrapper.target, credential)
	}
	process.stopProcess()
	process = startTransportE2EDaptin(t, port, httpsPort, base, options)
	invoke := func(token, operation string, route int) bool {
		t.Helper()
		if route == 2 {
			response := transportE2EPostJSON(t, client, base+"/graphql", token, map[string]interface{}{"query": fmt.Sprintf(`mutation { execute%sOnMembershipexample(credential_id: %q) { ResponseType } }`, map[string]string{"invoke": "Invoke", "free": "Free"}[operation], credential)})
			return !transportE2EGraphQLHasErrors(response)
		}
		payload := map[string]interface{}{"credential_id": credential, "input": map[string]interface{}{"sessionUser": map[string]interface{}{"UserReferenceId": user}, "require_membership": false}}
		path := "/integration/"
		if route == 1 {
			path = "/action/integration/"
			payload = map[string]interface{}{"attributes": map[string]interface{}{"credential_id": credential, "user_reference_id": user, "require_membership": false}}
		}
		status, body := postLLMActionForStatus(t, client, base+path+"membership.example/"+operation, token, payload)
		if status != http.StatusOK && status != http.StatusForbidden {
			t.Fatalf("unexpected status %d: %s", status, body)
		}
		return status == http.StatusOK
	}
	assertRoutes := func(label string, want bool) {
		t.Helper()
		before := calls.Load()
		for route := 0; route < 3; route++ {
			if got := invoke(customer, "invoke", route); got != want {
				t.Fatalf("%s route %d allowed=%v, want %v", label, route, got, want)
			}
		}
		delta := int64(0)
		if want {
			delta = 3
		}
		if calls.Load() != before+delta {
			t.Fatalf("%s provider calls=%d, want %d", label, calls.Load(), before+delta)
		}
	}
	assertRoutes("missing", false)
	for route := 0; route < 3; route++ {
		if !invoke(customer, "free", route) {
			t.Fatal("ungated operation rejected")
		}
	}
	create := func(kind string, attrs map[string]interface{}) string {
		t.Helper()
		response := accessGroupsE2ERequestJSON(t, client, http.MethodPost, base+"/api/"+kind, admin, accessGroupsE2ERecordPayload(kind, "", attrs), http.StatusCreated)
		return response.(map[string]interface{})["data"].(map[string]interface{})["id"].(string)
	}
	plan := create("api_plan", map[string]interface{}{"name": "Preflight plan", "limits": `[{"metric":"requests","window":"day","maximum":100,"mode":"hard"}]`})
	now := time.Now().UTC()
	stamp := func(hours int) string { return now.Add(time.Duration(hours) * time.Hour).Format(time.RFC3339) }
	accessGroupsE2EAssertStatus(t, client, http.MethodPost, base+"/action/integration/assign_membership", admin, map[string]interface{}{"attributes": map[string]interface{}{"status": "cancelled", "period_start": stamp(-2), "period_end": stamp(2), "api_plan_id": plan}}, http.StatusOK)
	member := accessGroupsE2EFindResourceID(t, client, base, admin, "api_member", "api_plan_id", plan)
	for _, test := range []struct {
		name, status string
		start, end   int
		allow        bool
	}{{"cancelled", "cancelled", -2, 2, false}, {"expired", "active", -2, -1, false}, {"future", "active", 1, 2, false}, {"current", "active", -1, 2, true}} {
		patch("api_member", member, map[string]interface{}{"status": test.status, "period_start": stamp(test.start), "period_end": stamp(test.end)})
		assertRoutes(test.name, test.allow)
	}
	before := calls.Load()
	status, body := postLLMActionForStatus(t, client, base+"/action/integration/switch_to_member", admin, map[string]interface{}{"attributes": map[string]interface{}{"user_reference_id": adminUser}})
	if status != 200 || calls.Load() != before+1 {
		t.Fatalf("switch to member: HTTP %d %s; provider=%d", status, body, calls.Load())
	}
	status, body = postLLMActionForStatus(t, client, base+"/action/integration/switch_to_nonmember", customer, map[string]interface{}{"attributes": map[string]interface{}{"user_reference_id": user}})
	if status != 403 || calls.Load() != before+1 {
		t.Fatalf("switch to nonmember: HTTP %d %s; provider=%d", status, body, calls.Load())
	}
	patch("api_member", member, map[string]interface{}{"period_end": nil})
	assertRoutes("open ended", true)
	patch("api_member", member, map[string]interface{}{"status": "cancelled"})
	assertRoutes("revoked", false)
	// Reinstall regenerates the same gate from the persisted backend spec.
	transportE2EInstallIntegration(t, client, base, admin, integration)
	assertRoutes("reinstalled", false)
	query := url.QueryEscape(`[{"column":"action_name","operator":"is","value":"membership.example/invoke"}]`)
	ledger := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/api_usage?query="+query, admin, nil, http.StatusOK)
	rows := accessGroupsE2EDataArray(t, ledger)
	if len(rows) != 6 {
		encoded, _ := json.Marshal(ledger)
		t.Fatalf("usage rows=%d, want 6 completed only: %s", len(rows), encoded)
	}
	for _, row := range rows {
		attrs := row.(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["state"] != "completed" || attrs["user_account_id"] != user {
			t.Fatalf("incorrect usage ownership/state: %#v", attrs)
		}
	}
	quotas := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/api_quota", admin, nil, http.StatusOK))
	consumed := float64(0)
	for _, row := range quotas {
		attrs := row.(map[string]interface{})["attributes"].(map[string]interface{})
		consumed += attrs["consumed"].(float64)
		if attrs["reserved"].(float64) != 0 {
			t.Fatalf("denied request leaked quota reservation: %#v", attrs)
		}
	}
	if consumed != 6 {
		t.Fatalf("consumed quota=%v, want 6", consumed)
	}
	t.Logf("Real provider calls=%d; all three transports denied missing/cancelled/expired/future/revoked memberships; 6 completed customer usage rows and quota units", calls.Load())
}
