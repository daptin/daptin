package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
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
	stopDaptin := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{schema: llmE2EActionsSchema})
	defer stopDaptin()
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
