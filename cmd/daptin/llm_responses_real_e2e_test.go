package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/daptin/daptin/server/auth"
)

func TestLLMStoredResponsesRealE2E(t *testing.T) {
	requireRealE2E(t)
	var mu sync.Mutex
	var received []map[string]interface{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("wrong provider credential")
			http.Error(w, "unauthorized", 401)
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			http.Error(w, "invalid", 400)
			return
		}
		mu.Lock()
		received = append(received, body)
		number := len(received)
		mu.Unlock()
		if body["store"] != false || body["previous_response_id"] != nil {
			t.Errorf("provider-owned state leaked upstream: %#v", body)
		}
		snapshot := map[string]interface{}{"id": fmt.Sprintf("upstream-%d", number), "object": "response", "status": "completed", "model": "upstream", "output": []interface{}{map[string]interface{}{"id": fmt.Sprintf("message-%d", number), "type": "message", "role": "assistant", "status": "completed", "content": []interface{}{map[string]interface{}{"type": "output_text", "text": "I remember blue.", "annotations": []interface{}{}}}}}, "usage": map[string]interface{}{"input_tokens": 10, "output_tokens": 3, "total_tokens": 13}}
		if body["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			payload, _ := json.Marshal(map[string]interface{}{"type": "response.completed", "sequence_number": 1, "response": snapshot})
			fmt.Fprintf(w, "event: response.completed\ndata: %s\n\n", payload)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snapshot)
	}))
	defer upstream.Close()
	options := transportE2EDaptinOptions{schema: llmE2EActionsSchema, connectionString: filepath.Join(t.TempDir(), "responses.db")}
	f := startDaptinE2E(t, options)
	t.Cleanup(func() {
		if t.Failed() {
			t.Log(f.Process.logs.String())
		}
	})
	admin := f.SignupAdmin(t)
	member := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "stateful-owner")
	other := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "stateful-other")
	owner := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "stateful-owner@test.local")
	credential := createLLME2ECatalog(t, f.Client, f.URL, admin, llmE2ECatalog{name: "history-model", upstreamURL: upstream.URL, apiKey: "test-key", upstreamModel: "upstream", operations: []string{"responses"}, maxConcurrency: 4})
	model := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "llm_model", "name", "history-model")
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/llm_model/"+model, admin, accessGroupsE2ERecordPayload("llm_model", model, map[string]interface{}{"permission": 32}), http.StatusOK)
	waitForLLME2EModel(t, f.Client, f.URL, member, "history-model")
	call := func(token string, attrs map[string]interface{}, want int) interface{} {
		t.Helper()
		return accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost, f.URL+"/v1/responses", token, attrs, want)
	}
	accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost, f.URL+"/api/api_usage", "", accessGroupsE2ERecordPayload("api_usage", "", map[string]interface{}{"status": "in_progress", "input": "[]", "response": "{}"}), 403)
	first := call(member, map[string]interface{}{"model": "history-model", "input": "Remember that my color is blue.", "instructions": "First turn only.", "store": true, "max_output_tokens": 16}, 200)
	id, ok := transportE2EPath(first, "id")
	if !ok {
		t.Fatalf("no response id: %#v", first)
	}
	reference := id.(string)
	if strings.HasPrefix(reference, "upstream-") {
		t.Fatal("stored response exposed provider ID")
	}
	retrieved := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/v1/responses/"+reference, member, nil, 200)
	assertTransportE2EString(t, retrieved, "output.0.content.0.text", "I remember blue.")
	rows := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/api/api_usage/"+reference, admin, nil, 200)
	assertTransportE2EString(t, rows, "data.attributes.user_account_id", owner)
	mu.Lock()
	before := len(received)
	mu.Unlock()
	accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/v1/responses/"+reference, other, nil, 404)
	accessGroupsE2ERequestJSON(t, f.Client, http.MethodDelete, f.URL+"/v1/responses/"+reference, other, nil, 404)
	call(other, map[string]interface{}{"model": "history-model", "input": "What color?", "previous_response_id": reference, "store": true}, 404)
	call("", map[string]interface{}{"model": "history-model", "input": "Guest store", "store": true}, 401)
	mu.Lock()
	if len(received) != before {
		t.Error("denied state access reached provider")
	}
	mu.Unlock()
	// Sharing is governed by persisted resource/group relations, independently of model execution.
	otherReference := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "stateful-other@test.local")
	group := accessGroupsE2ECreateUsergroup(t, f.Client, f.URL, admin, "history-readers")
	accessGroupsE2ECreateJoin(t, f.Client, f.URL, admin, "user_account_user_account_id_has_usergroup_usergroup_id", map[string]interface{}{"user_account_id": otherReference, "usergroup_id": group}, 0)
	relation := accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, "api_usage_api_usage_id_has_usergroup_usergroup_id", map[string]interface{}{"api_usage_id": reference, "usergroup_id": group})
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/api_usage_api_usage_id_has_usergroup_usergroup_id/"+relation, admin, accessGroupsE2ERecordPayload("api_usage_api_usage_id_has_usergroup_usergroup_id", relation, map[string]interface{}{"permission": int64(auth.GroupRead | auth.GroupPeek)}), 200)
	accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/v1/responses/"+reference, other, nil, 200)
	mu.Lock()
	before = len(received)
	mu.Unlock()
	call(other, map[string]interface{}{"model": "history-model", "input": "cannot refer yet", "previous_response_id": reference, "store": true}, 403)
	mu.Lock()
	if len(received) != before {
		t.Error("relation permission denial reached provider")
	}
	mu.Unlock()
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/api_usage_api_usage_id_has_usergroup_usergroup_id/"+relation, admin, accessGroupsE2ERecordPayload("api_usage_api_usage_id_has_usergroup_usergroup_id", relation, map[string]interface{}{"permission": int64(auth.GroupRead | auth.GroupPeek | auth.GroupRefer)}), 200)
	fork := call(other, map[string]interface{}{"model": "history-model", "input": "shared context", "previous_response_id": reference, "store": true, "max_output_tokens": 16}, 200)
	forkID, _ := transportE2EPath(fork, "id")
	forkRow := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/api/api_usage/"+forkID.(string), admin, nil, 200)
	assertTransportE2EString(t, forkRow, "data.attributes.user_account_id", otherReference)
	accessGroupsE2ERequestJSON(t, f.Client, http.MethodDelete, f.URL+"/v1/responses/"+forkID.(string), other, nil, 200)
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/api_usage_api_usage_id_has_usergroup_usergroup_id/"+relation, admin, accessGroupsE2ERecordPayload("api_usage_api_usage_id_has_usergroup_usergroup_id", relation, map[string]interface{}{"permission": 0}), 200)
	call(other, map[string]interface{}{"model": "history-model", "input": "share revoked", "previous_response_id": reference}, 404)
	// Restart with a different provider endpoint; history must come from SQL resources.
	var secondProviderCalls atomic.Int64
	secondProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondProviderCalls.Add(1)
		upstream.Config.Handler.ServeHTTP(w, r)
	}))
	defer secondProvider.Close()
	provider := accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, "llm_provider", map[string]interface{}{"name": "replacement", "provider_type": "openai-compatible", "base_url": secondProvider.URL + "/v1", "credential_id": credential, "provider_parameters": "{}", "allow_insecure": true, "allow_private_network": true, "enable": true})
	deployment := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "llm_deployment", "name", "history-model")
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/llm_deployment/"+deployment, admin, accessGroupsE2ERecordPayload("llm_deployment", deployment, map[string]interface{}{"llm_provider_id": provider}), 200)
	f.Process.stopProcess()
	f = startDaptinE2E(t, options)
	accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/v1/responses/"+reference, member, nil, 200)
	second := call(member, map[string]interface{}{"model": "history-model", "input": "What color?", "previous_response_id": reference, "store": true, "max_output_tokens": 16}, 200)
	secondID, _ := transportE2EPath(second, "id")
	if secondProviderCalls.Load() != 1 {
		t.Fatalf("continuation did not use replacement provider: %d", secondProviderCalls.Load())
	}
	mu.Lock()
	replay := received[len(received)-1]["input"].([]interface{})
	mu.Unlock()
	if len(replay) != 3 {
		t.Fatalf("expected prior input, prior output and new input: %#v", replay)
	}
	assertTransportE2EString(t, replay, "0.content.0.text", "Remember that my color is blue.")
	assertTransportE2EString(t, replay, "1.content.0.text", "I remember blue.")
	assertTransportE2EString(t, replay, "2.content.0.text", "What color?")
	// New instructions belong to the current turn; old top-level instructions are not replayed.
	status, streamBody := postLLMActionForStatus(t, f.Client, f.URL+"/v1/responses", member, map[string]interface{}{"model": "history-model", "input": "Again?", "previous_response_id": secondID, "store": true, "stream": true, "max_output_tokens": 16})
	if status != 200 || !strings.Contains(string(streamBody), "response.completed") || strings.Contains(string(streamBody), "upstream-") {
		t.Fatalf("stored SSE: status=%d body=%s", status, streamBody)
	}
	assertLLMUsageOwnedBy(t, f.Client, f.URL, admin, owner, 3)
	mu.Lock()
	if received[len(received)-1]["instructions"] != nil {
		t.Error("previous instructions were replayed")
	}
	mu.Unlock()
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/llm_model/"+model, admin, accessGroupsE2ERecordPayload("llm_model", model, map[string]interface{}{"permission": 0}), 200)
	call(member, map[string]interface{}{"model": "history-model", "input": "model denied", "previous_response_id": reference}, 403)
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/llm_model/"+model, admin, accessGroupsE2ERecordPayload("llm_model", model, map[string]interface{}{"permission": 32}), 200)
	plan := accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, "api_plan", map[string]interface{}{"name": "one more turn", "limits": `[{"metric":"requests","window":"month","maximum":1,"mode":"hard"}]`})
	accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost, f.URL+"/action/world/llm_e2e_assign_plan", admin, map[string]interface{}{"attributes": map[string]interface{}{"user_reference_id": owner, "api_plan_id": plan}}, 200)
	call(member, map[string]interface{}{"model": "history-model", "input": "one allowed turn", "previous_response_id": secondID}, 200)
	providerBeforeQuota := secondProviderCalls.Load()
	call(member, map[string]interface{}{"model": "history-model", "input": "over quota", "previous_response_id": secondID, "store": true}, 402)
	records := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/api/api_usage?page%5Bsize%5D=200", admin, nil, 200)
	retained := 0
	for _, row := range accessGroupsE2EDataArray(t, records) {
		record, _ := row.(map[string]interface{})
		attrs, _ := record["attributes"].(map[string]interface{})
		metadata, _ := attrs["metadata"].(map[string]interface{})
		if metadata["payload_retained"] == true {
			retained++
		}
	}
	if retained != 3 {
		t.Fatalf("retained captures=%d, want 3", retained)
	}
	if secondProviderCalls.Load() != providerBeforeQuota {
		t.Fatal("quota denial reached replacement provider")
	}
	assertLLMUsageOwnedBy(t, f.Client, f.URL, admin, owner, 4)
	// Revocation applies to subsequent continuations, even when the model remains executable.
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/api_usage/"+reference, admin, accessGroupsE2ERecordPayload("api_usage", reference, map[string]interface{}{"permission": 0}), 200)
	call(member, map[string]interface{}{"model": "history-model", "input": "revoked", "previous_response_id": reference}, 404)
	accessGroupsE2ERequestJSON(t, f.Client, http.MethodDelete, f.URL+"/v1/responses/"+secondID.(string), member, nil, 200)
	call(member, map[string]interface{}{"model": "history-model", "input": "deleted", "previous_response_id": secondID}, 404)
	// Retention deletion preserves the ledger row and its consumption facts.
	ledger := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/api/api_usage/"+secondID.(string), admin, nil, 200)
	assertTransportE2EString(t, ledger, "data.attributes.state", "completed")
	for _, field := range []string{"request_body", "response_body"} {
		value, _ := transportE2EPath(ledger, "data.attributes."+field)
		if value != nil && value != "" {
			t.Fatalf("deleted history still has %s", field)
		}
	}
	total, _ := transportE2EPath(ledger, "data.attributes.measures.total_tokens")
	if total != float64(13) {
		t.Fatalf("history deletion changed consumption: %v", total)
	}
	for _, line := range strings.Split(string(streamBody), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event map[string]interface{}
		if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event) != nil {
			continue
		}
		response, _ := event["response"].(map[string]interface{})
		childID, _ := response["id"].(string)
		if childID != "" {
			accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/v1/responses/"+childID, member, nil, 200)
		}
	}

}
