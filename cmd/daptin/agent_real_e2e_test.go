package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

const agentE2ESchema = `
EnableGraphQL: true
Tables:
  - TableName: agitem
    DefaultPermission: 16256
    AccessGroups:
      - Name: users
        Permission: 2080768
    Columns:
      - Name: name
        ColumnName: name
        ColumnType: label
        DataType: varchar(100)
Actions:
  - Name: verify
    Permission: 2097152
    OnType: agitem
    InstanceOptional: false
    OutFields:
      - Type: verified
        Method: ACTIONRESPONSE
        Attributes:
          message: verified
  - Name: effect
    OnType: agitem
    InstanceOptional: false
    OutFields:
      - Type: $network.request
        Method: EXECUTE
        Attributes:
          Url: AGENT_EFFECT_URL
          Method: GET
`

func TestAgentDurableInterfaceRealE2E(t *testing.T) {
	requireRealE2E(t)
	var inferenceCalls atomic.Int64
	var effects atomic.Int64
	var interruptedCalls atomic.Int64
	var retryCalls atomic.Int64
	modelStarted := make(chan struct{}, 1)
	effectStarted := make(chan struct{}, 1)
	releaseEffect := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path == "/effect" {
			effects.Add(1)
			select {
			case effectStarted <- struct{}{}:
			default:
			}
			<-releaseEffect
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sent":true}`))
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		inferenceCalls.Add(1)
		messages, _ := body["messages"].([]interface{})
		// Every synthetic tool call carries opaque continuation state. Checking
		// every replay also covers saved waits and replacement/cluster workers.
		for _, raw := range messages {
			message := raw.(map[string]interface{})
			calls, _ := message["tool_calls"].([]interface{})
			for _, rawCall := range calls {
				call := rawCall.(map[string]interface{})
				extra, _ := call["extra_content"].(map[string]interface{})
				google, _ := extra["google"].(map[string]interface{})
				if google["thought_signature"] != "opaque-state/"+fmt.Sprint(call["id"])+"/+==\n" {
					http.Error(w, "tool continuation metadata lost", http.StatusBadRequest)
					return
				}
			}
		}
		mode := "happy"
		var results []map[string]interface{}
		for _, raw := range messages {
			message := raw.(map[string]interface{})
			if message["role"] == "user" {
				if text, ok := message["content"].(string); ok {
					mode = text
				}
			}
			if message["role"] == "tool" {
				var result map[string]interface{}
				text, _ := message["content"].(string)
				_ = json.Unmarshal([]byte(text), &result)
				results = append(results, result)
			}
		}
		message := map[string]interface{}{"role": "assistant", "content": "done"}
		finish := "stop"
		tool := func(name string, args interface{}) {
			finish = "tool_calls"
			callID := fmt.Sprintf("call-%d", len(results))
			message = map[string]interface{}{"role": "assistant", "content": nil, "tool_calls": []interface{}{map[string]interface{}{"id": callID, "type": "function", "function": map[string]interface{}{"name": name, "arguments": string(mustAgentJSON(args))}, "extra_content": map[string]interface{}{"google": map[string]interface{}{"thought_signature": "opaque-state/" + callID + "/+==\n"}}}}}
		}
		effectReference := ""
		historyReference := ""
		if strings.HasPrefix(mode, "history:") {
			historyReference = strings.TrimPrefix(mode, "history:")
			mode = "history"
		}
		privilegeReference := ""
		if strings.HasPrefix(mode, "privilege:") {
			privilegeReference = strings.TrimPrefix(mode, "privilege:")
			mode = "privilege"
		}
		if strings.HasPrefix(mode, "effect:") {
			effectReference = strings.TrimPrefix(mode, "effect:")
			mode = "effect"
		}
		if strings.HasPrefix(mode, "ready:") {
			mode = "ready"
		}
		switch mode {
		case "ready":

		case "history":
			tool("daptin_read", map[string]interface{}{"entity": "agitem", "reference": historyReference})
			message["content"] = strings.Repeat("h", 600*1024)
		case "privilege":
			if len(results) == 0 {
				tool("daptin_action", map[string]interface{}{"entity": "agitem", "action": "verify", "attributes": map[string]interface{}{"agitem_id": privilegeReference}})
			}
			if len(results) == 1 {
				tool("daptin_create", map[string]interface{}{"entity": "credential", "attributes": map[string]interface{}{"name": "agent-must-not-create", "content": "{}"}})
			}
		case "retry":
			if retryCalls.Add(1) == 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(503)
				_, _ = w.Write([]byte(`{"error":{"message":"retry later","type":"server_error"}}`))
				return
			}
		case "heartbeat":
			select {
			case <-time.After(12 * time.Second):
			case <-r.Context().Done():
				return
			}
		case "interrupted":
			if interruptedCalls.Add(1) == 1 {
				modelStarted <- struct{}{}
				<-r.Context().Done()
				return
			}
		case "effect":
			if len(results) == 0 {
				tool("daptin_action", map[string]interface{}{"entity": "agitem", "action": "effect", "attributes": map[string]interface{}{"agitem_id": effectReference}})
			}

		case "crud":
			switch len(results) {
			case 0:
				tool("daptin_discover", map[string]interface{}{"entity": "agitem"})
			case 1:
				tool("daptin_create", map[string]interface{}{"entity": "agitem", "attributes": map[string]interface{}{"name": "temporary"}})
			case 2:
				tool("daptin_read", map[string]interface{}{"entity": "agitem", "reference": results[1]["reference_id"]})
			case 3:
				tool("daptin_update", map[string]interface{}{"entity": "agitem", "reference": results[1]["reference_id"], "attributes": map[string]interface{}{"name": "updated", "version": results[2]["version"]}})
			case 4:
				tool("daptin_relations", map[string]interface{}{"entity": "agitem", "reference": results[1]["reference_id"], "relation": "user_account_id"})
			case 5:
				tool("daptin_list", map[string]interface{}{"entity": "agitem", "query": map[string]interface{}{"page[size]": "10"}})
			case 6:
				tool("daptin_delete", map[string]interface{}{"entity": "agitem", "reference": results[1]["reference_id"]})
			}
		case "decision":
			if len(results) == 0 {
				tool("agent_decision", map[string]interface{}{"question": "Continue?"})
			}
		case "timer":
			if len(results) == 0 {
				tool("agent_wait", map[string]interface{}{"until": time.Now().UTC().Add(3 * time.Second).Format(time.RFC3339)})
			}
		case "invalid":
			tool("unknown_tool", map[string]interface{}{})
		case "denied":
			if len(results) == 0 {
				tool("daptin_create", map[string]interface{}{"entity": "credential", "attributes": map[string]interface{}{"name": "agent-must-not-create", "content": "{}"}})
			}
		default:
			switch len(results) {
			case 0:
				tool("daptin_create", map[string]interface{}{"entity": "agitem", "attributes": map[string]interface{}{"name": "agent-created"}})
			case 1:
				tool("daptin_read", map[string]interface{}{"entity": "agitem", "reference": results[0]["reference_id"]})
			case 2:
				tool("daptin_action", map[string]interface{}{"entity": "agitem", "action": "verify", "attributes": map[string]interface{}{"agitem_id": results[0]["reference_id"]}})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "agent-response", "object": "chat.completion", "created": time.Now().Unix(), "model": "agent-upstream", "choices": []interface{}{map[string]interface{}{"index": 0, "message": message, "finish_reason": finish}}, "usage": map[string]interface{}{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}})
	}))
	defer func() { close(releaseEffect); upstream.Close() }()
	database := transportE2EDaptinOptions{connectionString: filepath.Join(t.TempDir(), "agent.db"), schema: strings.ReplaceAll(agentE2ESchema, "AGENT_EFFECT_URL", upstream.URL+"/effect")}
	if os.Getenv("DAPTIN_E2E_MAIL_POSTGRES_BIN") != "" {
		database = mailProtocolDatabase(t)
		database.schema = strings.ReplaceAll(agentE2ESchema, "AGENT_EFFECT_URL", upstream.URL+"/effect")
	}
	used := map[int]bool{}
	var seed string
	nodes := map[string]*transportE2EDaptinProcess{}
	start := func(peer string) (string, *transportE2EDaptinProcess) {
		t.Helper()
		port := freeTransportE2EPort(t, used)
		https := freeTransportE2EPort(t, used)
		options := database
		options.olricPort = freeTransportE2EPortPair(t, used)
		options.olricPeers = peer
		if seed == "" {
			seed = fmt.Sprintf("127.0.0.1:%d", options.olricPort+1)
		}
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		process := startTransportE2EDaptin(t, port, https, base, options)
		nodes[base] = process
		return base, process
	}
	baseA, nodeA := start("")
	client := &http.Client{Timeout: 20 * time.Second}
	admin := transportE2ESignupSigninAdmin(t, client, baseA)
	var clusterB, clusterC string
	var peerB, peerC *transportE2EDaptinProcess
	if os.Getenv("DAPTIN_E2E_MAIL_POSTGRES_BIN") != "" {
		clusterB, peerB = start(seed)
		waitForLLME2ECluster(t, client, baseA, clusterB, admin)
		clusterC, peerC = start(seed)
		deadline := time.Now().Add(20 * time.Second)
		for {
			var members interface{}
			converged := true
			for _, base := range []string{baseA, clusterB, clusterC} {
				stats := transportE2EGetJSON(t, client, base+"/statistics", admin)
				count, _ := transportE2EPath(stats, "olric.member_count")
				roster, _ := transportE2EPath(stats, "olric.members")
				if count != float64(3) {
					converged = false
				}
				if members == nil {
					members = roster
				} else if !reflect.DeepEqual(members, roster) {
					converged = false
				}
			}
			if converged {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("three Olric nodes did not converge")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	createLLME2ECatalog(t, client, baseA, admin, llmE2ECatalog{name: "agent-model", upstreamURL: upstream.URL, apiKey: "agent-key", upstreamModel: "agent-upstream", operations: []string{"chat"}, maxConcurrency: 4, requestTimeoutMS: 60000})
	model := accessGroupsE2EFindResourceID(t, client, baseA, admin, "llm_model", "name", "agent-model")
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseA+"/api/llm_model/"+model, admin, accessGroupsE2ERecordPayload("llm_model", model, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute), "capabilities": `{"tools":true}`}), 200)
	waitForLLME2EModel(t, client, baseA, admin, "agent-model")
	ready := func(t *testing.T, base string) {
		t.Helper()
		var lastStatus int
		var lastBody []byte
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			status, body := postLLMActionForStatus(t, client, base+"/v1/chat/completions", admin, map[string]interface{}{"model": "agent-model", "max_tokens": 32, "messages": []interface{}{map[string]interface{}{"role": "user", "content": fmt.Sprintf("ready:%s:%d", base, time.Now().UnixNano())}}})
			lastStatus, lastBody = status, body
			if status == 200 {
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
		t.Fatalf("gateway coordination at %s did not become ready: %d %s", base, lastStatus, lastBody)
	}

	config := map[string]interface{}{"instructions": "Use the configured tools.", "model": "agent-model", "toolset": map[string]interface{}{"resources": []string{"*"}, "operations": []string{"list", "read", "create", "update", "delete", "relations"}, "actions": []string{"agitem.verify", "agitem.effect"}}, "limits": map[string]interface{}{"turns": 10, "tool_calls": 10, "seconds": 180, "output_tokens": 256}}
	definition := accessGroupsE2ECreateRecord(t, client, baseA, admin, "agent", map[string]interface{}{"name": "e2e-agent", "config": string(mustAgentJSON(config))})
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseA+"/api/agent/"+definition, admin, accessGroupsE2ERecordPayload("agent", definition, map[string]interface{}{"permission": int64(auth.GuestPeek | auth.GuestRead | auth.GuestRefer | auth.AuthenticatedExecute)}), 200)
	submit := func(t *testing.T, base, token, mode string) string {
		t.Helper()
		response := transportE2EPostJSON(t, client, base+"/action/agent/start", token, map[string]interface{}{"attributes": map[string]interface{}{"agent_id": definition, "messages": []interface{}{map[string]interface{}{"role": "user", "content": mode}}}})
		value, ok := transportE2EPath(response, "0.Attributes.reference_id")
		if !ok {
			t.Fatalf("no run reference: %#v", response)
		}
		return value.(string)
	}
	wait := func(t *testing.T, base, ref, status string) map[string]interface{} {
		t.Helper()
		deadline := time.Now().Add(40 * time.Second)
		var attrs map[string]interface{}
		for time.Now().Before(deadline) {
			response := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/agentrun/"+ref, admin, nil, 200)
			value, _ := transportE2EPath(response, "data.attributes")
			attrs = value.(map[string]interface{})
			if attrs["status"] == status {
				return attrs
			}
			if attrs["status"] == "failed" && status != "failed" {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatalf("run %s wanted %s got %#v\n%s", ref, status, attrs, nodes[base].logs.String())
		return nil
	}
	t.Run("CRUD and action results feed subsequent model turns", func(t *testing.T) {
		run := submit(t, baseA, admin, "happy")
		wait(t, baseA, run, "completed")
		rows := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseA+"/api/agitem", admin, nil, 200)
		data, _ := transportE2EPath(rows, "data")
		if len(data.([]interface{})) != 1 {
			t.Fatalf("expected exactly one create: %#v", rows)
		}
	})

	t.Run("discovery update relations list and delete reuse resources", func(t *testing.T) {
		run := submit(t, baseA, admin, "crud")
		wait(t, baseA, run, "completed")
		rows := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseA+"/api/agitem", admin, nil, 200)
		data, _ := transportE2EPath(rows, "data")
		if len(data.([]interface{})) != 1 {
			t.Fatalf("temporary record was not deleted: %#v", rows)
		}
	})

	t.Run("retryable inference is checkpointed and resumed", func(t *testing.T) {
		run := submit(t, baseA, admin, "retry")
		wait(t, baseA, run, "completed")
		if retryCalls.Load() != 2 {
			t.Fatalf("expected two attempts, got %d", retryCalls.Load())
		}
	})

	t.Run("history limit terminalizes without repeating completed work", func(t *testing.T) {
		record := accessGroupsE2EFindResourceID(t, client, baseA, admin, "agitem", "name", "agent-created")
		run := submit(t, baseA, admin, "history:"+record)
		attrs := wait(t, baseA, run, "failed")
		state := attrs["state"].(map[string]interface{})
		if state["error"] != "agent history limit exceeded" {
			t.Fatalf("unexpected limit outcome: %v", state["error"])
		}
	})
	t.Run("unknown tools fail before effects", func(t *testing.T) { run := submit(t, baseA, admin, "invalid"); wait(t, baseA, run, "failed") })

	t.Run("GraphQL uses the same start and cancel controls", func(t *testing.T) {
		query := fmt.Sprintf("mutation { executeStartOnAgent(agent_id: %q, messages: %q) { Attributes { value } } }", definition, string(mustAgentJSON([]interface{}{map[string]interface{}{"role": "user", "content": "decision"}})))
		response := transportE2EPostJSON(t, client, baseA+"/graphql", admin, map[string]interface{}{"query": query})
		value, ok := transportE2EPath(response, "data.executeStartOnAgent.0.Attributes.value")
		if !ok {
			t.Fatalf("GraphQL start: %#v", response)
		}
		run := value.(string)
		wait(t, baseA, run, "waiting_decision")
		query = fmt.Sprintf("mutation { executeCancelOnAgentrun(agentrun_id: %q) { Attributes { value } } }", run)
		response = transportE2EPostJSON(t, client, baseA+"/graphql", admin, map[string]interface{}{"query": query})
		if _, ok := response.(map[string]interface{})["errors"]; ok {
			t.Fatalf("GraphQL cancel: %#v", response)
		}
		wait(t, baseA, run, "cancelled")
	})
	t.Run("three nodes share ownership and cross-node controls", func(t *testing.T) {
		if os.Getenv("DAPTIN_E2E_MAIL_POSTGRES_BIN") == "" {
			t.Skip("set DAPTIN_E2E_MAIL_POSTGRES_BIN for the PostgreSQL/Olric cluster")
		}
		baseB, baseC := clusterB, clusterC
		ready(t, baseA)
		ready(t, baseB)
		ready(t, baseC)
		run := submit(t, baseA, admin, "happy")
		wait(t, baseB, run, "completed")
		t.Run("lease renewals persist during inference", func(t *testing.T) {
			before := inferenceCalls.Load()
			longRun := submit(t, baseB, admin, "heartbeat")
			first := wait(t, baseC, longRun, "running")
			time.Sleep(7 * time.Second)
			response := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseA+"/api/agentrun/"+longRun, admin, nil, 200)
			value, _ := transportE2EPath(response, "data.attributes.lease_until")
			if value == first["lease_until"] {
				t.Fatal("ownership heartbeat did not persist lease renewal")
			}
			wait(t, baseA, longRun, "completed")
			if inferenceCalls.Load() != before+1 {
				t.Fatal("concurrent wakes duplicated inference")
			}
		})

		rows := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseC+"/api/agitem", admin, nil, 200)
		data, _ := transportE2EPath(rows, "data")
		if len(data.([]interface{})) != 2 {
			t.Fatalf("concurrent wakes duplicated writes: %#v", rows)
		}
		decision := submit(t, baseB, admin, "decision")
		wait(t, baseC, decision, "waiting_decision")
		transportE2EPostJSON(t, client, baseC+"/action/agentrun/decide", admin, map[string]interface{}{"attributes": map[string]interface{}{"agentrun_id": decision, "decision": map[string]interface{}{"approved": true}}})
		wait(t, baseA, decision, "completed")
	})
	t.Run("waiting and controls survive a process replacement", func(t *testing.T) {
		run := submit(t, baseA, admin, "decision")
		wait(t, baseA, run, "waiting_decision")
		nodeA.stopProcess()
		if peerB != nil {
			peerB.stopProcess()
		}
		if peerC != nil {
			peerC.stopProcess()
		}
		baseB, nodeB := start("")
		defer nodeB.stopProcess()
		accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseB+"/api/agentrun/"+run, admin, nil, 200)
		transportE2EPostJSON(t, client, baseB+"/action/agentrun/decide", admin, map[string]interface{}{"attributes": map[string]interface{}{"agentrun_id": run, "decision": map[string]interface{}{"approved": true}}})
		wait(t, baseB, run, "completed")
		timer := submit(t, baseB, admin, "timer")
		wait(t, baseB, timer, "completed")
		cancelled := submit(t, baseB, admin, "decision")
		wait(t, baseB, cancelled, "waiting_decision")
		transportE2EPostJSON(t, client, baseB+"/action/agentrun/cancel", admin, map[string]interface{}{"attributes": map[string]interface{}{"agentrun_id": cancelled}})
		wait(t, baseB, cancelled, "cancelled")
		member := accessGroupsE2ESignupSigninUser(t, client, baseB, admin, "agent-member")
		denied := submit(t, baseB, member, "denied")
		deniedState := wait(t, baseB, denied, "unknown")
		memberReference := accessGroupsE2EFindResourceID(t, client, baseB, admin, "user_account", "email", "agent-member@test.local")
		if deniedState["user_account_id"] != memberReference {
			t.Fatal("run belongs to the wrong execution account")
		}
		assertLLMUsageOwnedBy(t, client, baseB, admin, memberReference, 1)
		attemptReference := deniedState["state"].(map[string]interface{})["active"].(string)
		attempt := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseB+"/api/agenttry/"+attemptReference, member, nil, 200)
		owner, _ := transportE2EPath(attempt, "data.attributes.user_account_id")
		if owner != memberReference {
			t.Fatal("tool attempt belongs to the wrong execution account")
		}
		t.Run("action privileges do not escape into independent CRUD", func(t *testing.T) {
			record := accessGroupsE2ECreateRecord(t, client, baseB, member, "agitem", map[string]interface{}{"name": "member-owned"})
			run := submit(t, baseB, member, "privilege:"+record)
			attrs := wait(t, baseB, run, "unknown")
			state := attrs["state"].(map[string]interface{})
			if state["calls"].(float64) != 2 {
				t.Fatalf("named action did not finish before CRUD denial: %#v", state)
			}
			assertLLMUsageOwnedBy(t, client, baseB, admin, memberReference, 3)
		})
		raw := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseB+"/api/credential", admin, nil, 200)
		if strings.Contains(string(mustAgentJSON(raw)), "agent-must-not-create") {
			t.Fatal("denied CRUD wrote a credential")
		}
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseB+"/api/agentrun/"+denied, member, accessGroupsE2ERecordPayload("agentrun", denied, map[string]interface{}{"status": "completed"}), 403)
		t.Run("revoked model permission prevents subsequent provider calls", func(t *testing.T) {
			run := submit(t, baseB, member, "decision")
			wait(t, baseB, run, "waiting_decision")
			before := inferenceCalls.Load()
			accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseB+"/api/llm_model/"+model, admin, accessGroupsE2ERecordPayload("llm_model", model, map[string]interface{}{"permission": 0}), 200)
			transportE2EPostJSON(t, client, baseB+"/action/agentrun/decide", member, map[string]interface{}{"attributes": map[string]interface{}{"agentrun_id": run, "decision": map[string]interface{}{"approved": true}}})
			wait(t, baseB, run, "failed")
			if inferenceCalls.Load() != before {
				t.Fatal("revoked model reached provider")
			}
			accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseB+"/api/llm_model/"+model, admin, accessGroupsE2ERecordPayload("llm_model", model, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)}), 200)
		})

		t.Run("run execute revocation stops background dispatch", func(t *testing.T) {
			run := submit(t, baseB, member, "decision")
			wait(t, baseB, run, "waiting_decision")
			before := inferenceCalls.Load()
			accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseB+"/api/agentrun/"+run, admin, accessGroupsE2ERecordPayload("agentrun", run, map[string]interface{}{"permission": int64(auth.UserRead | auth.UserPeek)}), 200)
			transportE2EPostJSON(t, client, baseB+"/action/agentrun/decide", admin, map[string]interface{}{"attributes": map[string]interface{}{"agentrun_id": run, "decision": map[string]interface{}{"approved": true}}})
			wait(t, baseB, run, "failed")
			if inferenceCalls.Load() != before {
				t.Fatal("revoked run reached provider")
			}
		})
		t.Run("authentication version revokes background delegation", func(t *testing.T) {
			run := submit(t, baseB, member, "decision")
			wait(t, baseB, run, "waiting_decision")
			before := inferenceCalls.Load()
			accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseB+"/api/user_account/"+memberReference, admin, accessGroupsE2ERecordPayload("user_account", memberReference, map[string]interface{}{"password": "changed-agent-password-123"}), 200)
			transportE2EPostJSON(t, client, baseB+"/action/agentrun/decide", admin, map[string]interface{}{"attributes": map[string]interface{}{"agentrun_id": run, "decision": map[string]interface{}{"approved": true}}})
			attrs := wait(t, baseB, run, "failed")
			state := attrs["state"].(map[string]interface{})
			if state["error"] != "background delegation revoked" {
				t.Fatalf("unexpected revocation: %#v", state)
			}
			if inferenceCalls.Load() != before {
				t.Fatal("revoked delegation reached provider")
			}
		})
		t.Run("interrupted inference resumes on another node", func(t *testing.T) {
			run := submit(t, baseB, admin, "interrupted")
			select {
			case <-modelStarted:
			case <-time.After(20 * time.Second):
				t.Fatal("inference never started")
			}
			_ = syscall.Kill(-nodeB.processGroupID, syscall.SIGKILL)
			nodeB.stopProcess()
			baseB, nodeB = start("")
			wait(t, baseB, run, "completed")
			if interruptedCalls.Load() != 2 {
				t.Fatalf("inference attempts = %d", interruptedCalls.Load())
			}
		})
		t.Run("effect before checkpoint is reconciled without redispatch", func(t *testing.T) {
			record := accessGroupsE2ECreateRecord(t, client, baseB, admin, "agitem", map[string]interface{}{"name": "effect-target"})
			run := submit(t, baseB, admin, "effect:"+record)
			select {
			case <-effectStarted:
			case <-time.After(20 * time.Second):
				t.Fatal("tool effect was never dispatched")
			}
			_ = syscall.Kill(-nodeB.processGroupID, syscall.SIGKILL)
			nodeB.stopProcess()
			baseC, nodeC := start("")
			defer nodeC.stopProcess()
			attrs := wait(t, baseC, run, "unknown")
			state := attrs["state"].(map[string]interface{})
			attempt := state["active"].(string)
			transportE2EPostJSON(t, client, baseC+"/action/agentrun/reconcile", admin, map[string]interface{}{"attributes": map[string]interface{}{"agentrun_id": run, "attempt": attempt, "result": map[string]interface{}{"sent": true}}})
			wait(t, baseC, run, "completed")
			if effects.Load() != 1 {
				t.Fatalf("effect repeated %d times", effects.Load())
			}
		})

	})
	if inferenceCalls.Load() < 8 {
		t.Fatalf("model loop did not run: %d", inferenceCalls.Load())
	}
}
func mustAgentJSON(value interface{}) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
