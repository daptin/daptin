package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

func TestLLMDeclarativeActionsRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run the declarative LLM action e2e")
	}

	upstream := startLLME2EUpstream(t, "declarative-action-key", "declarative-upstream", "declarative-chat-ok", nil)
	defer upstream.Close()

	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	daptinProcess := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{schema: llmE2EActionsSchema})
	defer daptinProcess.stopProcess()
	client := &http.Client{Timeout: 20 * time.Second}
	token := transportE2ESignupSigninAdmin(t, client, baseURL)
	createLLME2ECatalog(t, client, baseURL, token, llmE2ECatalog{
		name: "llm-declarative-e2e", upstreamURL: upstream.URL, apiKey: "declarative-action-key",
		upstreamModel: "declarative-upstream", operations: []string{"chat", "embeddings"}, maxConcurrency: 2,
	})
	waitForLLME2EModel(t, client, baseURL, token, "llm-declarative-e2e")

	chat := transportE2EPostJSON(t, client, baseURL+"/action/world/llm_e2e_chat", token, map[string]interface{}{
		"attributes": map[string]interface{}{"model": "llm-declarative-e2e", "prompt": "hello"},
	})
	assertTransportE2EString(t, chat, "0.Attributes.content", "declarative-chat-ok")
	assertLLMActionUsage(t, chat, "0.Attributes.usage.total_tokens", 5)

	embedding := transportE2EPostJSON(t, client, baseURL+"/action/world/llm_e2e_embedding", token, map[string]interface{}{
		"attributes": map[string]interface{}{"model": "llm-declarative-e2e", "input": "hello"},
	})
	assertLLMActionUsage(t, embedding, "0.Attributes.usage.total_tokens", 2)
	vectors, found := transportE2EPath(embedding, "0.Attributes.embeddings")
	vectorList, valid := vectors.([]interface{})
	if !found || !valid || len(vectorList) != 1 {
		t.Fatalf("declarative embedding response = %#v", embedding)
	}

	failureStatus, failureBody := postLLMActionForStatus(t, client, baseURL+"/action/world/llm_e2e_chat", token, map[string]interface{}{
		"attributes": map[string]interface{}{"model": "llm-declarative-e2e", "prompt": "trigger-provider-failure"},
	})
	if failureStatus < 400 || strings.Contains(string(failureBody), "provider-secret-marker") || !strings.Contains(string(failureBody), "provider_error") {
		t.Fatalf("provider failure was not safely normalized: status=%d body=%s", failureStatus, failureBody)
	}

	assertDeclarativeLLMUsage(t, client, baseURL, token)
}

func TestLLMModelAuthorizationAndMeteringRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run the LLM authorization e2e")
	}

	var upstreamRequests atomic.Int64
	upstream := startLLME2EUpstream(t, "authorization-key", "authorization-upstream", "authorized", func() bool {
		upstreamRequests.Add(1)
		return true
	})
	defer upstream.Close()

	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	daptinProcess := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{schema: llmE2EActionsSchema})
	defer daptinProcess.stopProcess()
	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	callerToken := accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "llm-caller")
	serviceToken := accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "llm-service")
	serviceReference := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "user_account", "email", "llm-service@test.local")

	modelName := "llm-authorization-e2e"
	createLLME2ECatalog(t, client, baseURL, adminToken, llmE2ECatalog{
		name: modelName, upstreamURL: upstream.URL, apiKey: "authorization-key",
		upstreamModel: "authorization-upstream", operations: []string{"chat"}, maxConcurrency: 2,
	})
	modelReference := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "llm_model", "name", modelName)
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/llm_model/"+modelReference, adminToken,
		accessGroupsE2ERecordPayload("llm_model", modelReference, map[string]interface{}{"permission": 0}), http.StatusOK)
	groupReference := accessGroupsE2ECreateUsergroup(t, client, baseURL, adminToken, "llm-service-model-access")
	accessGroupsE2ECreateJoin(t, client, baseURL, adminToken,
		"user_account_user_account_id_has_usergroup_usergroup_id",
		map[string]interface{}{"user_account_id": serviceReference, "usergroup_id": groupReference}, 0)
	accessGroupsE2ECreateJoin(t, client, baseURL, adminToken,
		"llm_model_llm_model_id_has_usergroup_usergroup_id",
		map[string]interface{}{"llm_model_id": modelReference, "usergroup_id": groupReference}, int64(auth.GroupExecute))
	plan := transportE2EPostJSON(t, client, baseURL+"/api/api_plan", adminToken, map[string]interface{}{
		"data": map[string]interface{}{"type": "api_plan", "attributes": map[string]interface{}{
			"name": "llm-service-two-requests", "limits": `[{"metric":"requests","window":"month","maximum":2,"mode":"hard"}]`,
		}},
	})
	planReference := transportE2EReferenceID(t, plan)
	transportE2EPostJSON(t, client, baseURL+"/action/world/llm_e2e_assign_plan", adminToken, map[string]interface{}{
		"attributes": map[string]interface{}{"user_reference_id": serviceReference, "api_plan_id": planReference},
	})
	waitForLLME2EModel(t, client, baseURL, adminToken, modelName)

	callerModels := transportE2EGetJSON(t, client, baseURL+"/v1/models", callerToken)
	if llmE2EModelListed(callerModels, modelName) {
		t.Fatal("model without a shared execute group was visible to the caller")
	}
	serviceModels := transportE2EGetJSON(t, client, baseURL+"/v1/models", serviceToken)
	if !llmE2EModelListed(serviceModels, modelName) {
		t.Fatal("model with a shared execute group was hidden from the service account")
	}

	directDenied, _ := postLLMActionForStatus(t, client, baseURL+"/v1/chat/completions", callerToken, map[string]interface{}{
		"model": modelName, "messages": []interface{}{map[string]interface{}{"role": "user", "content": "denied"}},
	})
	if directDenied != http.StatusForbidden || upstreamRequests.Load() != 0 {
		t.Fatalf("direct unauthorized request: status=%d upstream=%d", directDenied, upstreamRequests.Load())
	}
	actionDenied, _ := postLLMActionForStatus(t, client, baseURL+"/action/world/llm_e2e_chat", callerToken, map[string]interface{}{
		"attributes": map[string]interface{}{"model": modelName, "prompt": "denied"},
	})
	if actionDenied < http.StatusBadRequest || upstreamRequests.Load() != 0 {
		t.Fatalf("action synthetic administrator bypass: status=%d upstream=%d", actionDenied, upstreamRequests.Load())
	}

	for invocation := 0; invocation < 2; invocation++ {
		response := transportE2EPostJSON(t, client, baseURL+"/action/world/llm_e2e_switched_chat", callerToken, map[string]interface{}{
			"attributes": map[string]interface{}{"user_reference_id": serviceReference, "model": modelName, "prompt": "allowed"},
		})
		assertTransportE2EString(t, response, "0.Attributes.content", "authorized")
	}
	if upstreamRequests.Load() != 2 {
		t.Fatalf("authorized switched-user requests = %d, want 2", upstreamRequests.Load())
	}
	quotaStatus, _ := postLLMActionForStatus(t, client, baseURL+"/action/world/llm_e2e_switched_chat", callerToken, map[string]interface{}{
		"attributes": map[string]interface{}{"user_reference_id": serviceReference, "model": modelName, "prompt": "over quota"},
	})
	if quotaStatus < http.StatusBadRequest || upstreamRequests.Load() != 2 {
		t.Fatalf("quota denial reached provider: status=%d upstream=%d", quotaStatus, upstreamRequests.Load())
	}
	assertLLMUsageOwnedBy(t, client, baseURL, adminToken, serviceReference, 2)
}

func llmE2EModelListed(response interface{}, modelName string) bool {
	models, found := transportE2EPath(response, "data")
	if !found {
		return false
	}
	for _, item := range models.([]interface{}) {
		if id, _ := transportE2EPath(item, "id"); id == modelName {
			return true
		}
	}
	return false
}

func assertLLMUsageOwnedBy(t testing.TB, client *http.Client, baseURL, token, userReference string, expected int) {
	t.Helper()
	response := transportE2EGetJSON(t, client, baseURL+"/api/api_usage?page%5Bsize%5D=100", token)
	rows, _ := transportE2EPath(response, "data")
	completed := 0
	total := 0
	for _, item := range rows.([]interface{}) {
		entityType, _ := transportE2EPath(item, "attributes.entity_type")
		if entityType != "llm_model" {
			continue
		}
		total++
		owner, _ := transportE2EPath(item, "attributes.user_account_id")
		if owner != userReference {
			t.Fatalf("LLM usage was charged to %v, want %s: %#v", owner, userReference, response)
		}
		state, _ := transportE2EPath(item, "attributes.state")
		if state == "completed" {
			completed++
		}
	}
	if completed != expected || total != expected {
		t.Fatalf("LLM usage owned by %s: completed=%d total=%d, want %d: %#v", userReference, completed, total, expected, response)
	}
}

func assertLLMActionUsage(t testing.TB, response interface{}, path string, expected int64) {
	t.Helper()
	value, found := transportE2EPath(response, path)
	actual, valid := value.(float64)
	if !found || !valid || actual != float64(expected) {
		t.Fatalf("usage at %s = %#v, want %d", path, value, expected)
	}
}

func postLLMActionForStatus(t testing.TB, client *http.Client, url string, token string, payload interface{}) (int, []byte) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, responseBody
}

func assertDeclarativeLLMUsage(t testing.TB, client *http.Client, baseURL string, token string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		response := transportE2EGetJSON(t, client, baseURL+"/api/api_usage", token)
		rows, _ := transportE2EPath(response, "data")
		entries, _ := rows.([]interface{})
		completed, failedChat := 0, 0
		requestTypes := make(map[string]bool, 2)
		for _, entry := range entries {
			entityType, _ := transportE2EPath(entry, "attributes.entity_type")
			state, _ := transportE2EPath(entry, "attributes.state")
			if entityType != "llm_model" || state != "completed" {
				continue
			}
			completed++
			requestType, _ := transportE2EPath(entry, "attributes.request_type")
			requestTypeString, _ := requestType.(string)
			requestTypes[requestTypeString] = true
			statusCode, _ := transportE2EPath(entry, "attributes.status_code")
			status, validStatus := statusCode.(float64)
			if requestTypeString == "llm_chat" && validStatus && status == http.StatusServiceUnavailable {
				failedChat++
			}
		}
		if completed == 3 && requestTypes["llm_chat"] && requestTypes["llm_embeddings"] && failedChat == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("declarative LLM actions did not terminalize through api_usage: %#v", response)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
