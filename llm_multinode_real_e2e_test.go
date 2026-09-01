package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestLLMMultinodeCatalogConvergence(t *testing.T) {
	if os.Getenv("DAPTIN_LLM_MULTINODE_E2E") != "1" {
		t.Skip("set DAPTIN_LLM_MULTINODE_E2E=1 with DAPTIN_TEST_POSTGRES_DSN to run the LLM multi-node e2e")
	}
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DAPTIN_TEST_POSTGRES_DSN must identify a disposable PostgreSQL database")
	}

	var expectedKey atomic.Value
	expectedKey.Store("multinode-initial-key")
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			http.NotFound(response, request)
			return
		}
		if request.Header.Get("Authorization") != "Bearer "+expectedKey.Load().(string) {
			http.Error(response, "stale provider credential", http.StatusUnauthorized)
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["model"] != "multinode-upstream" {
			http.Error(response, "invalid upstream request", http.StatusBadRequest)
			return
		}
		transportE2EWriteJSON(response, map[string]interface{}{
			"id": "multinode-chat", "object": "chat.completion", "created": 1, "model": "multinode-upstream",
			"choices": []interface{}{map[string]interface{}{
				"index": 0, "message": map[string]interface{}{"role": "assistant", "content": "multinode-ok"}, "finish_reason": "stop",
			}},
			"usage": map[string]interface{}{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	defer upstream.Close()

	usedPorts := make(map[int]bool, 8)
	portA := freeLLMMultinodePort(t, usedPorts)
	httpsPortA := freeLLMMultinodePort(t, usedPorts)
	portB := freeLLMMultinodePort(t, usedPorts)
	httpsPortB := freeLLMMultinodePort(t, usedPorts)
	olricPortA := freeLLMMultinodePortPair(t, usedPorts)
	olricPortB := freeLLMMultinodePortPair(t, usedPorts)
	baseA := fmt.Sprintf("http://127.0.0.1:%d", portA)
	baseB := fmt.Sprintf("http://127.0.0.1:%d", portB)

	stopA := startTransportE2EDaptin(t, portA, httpsPortA, baseA, transportE2EDaptinOptions{
		databaseType: "postgres", connectionString: dsn, olricPort: olricPortA,
	})
	defer stopA()
	stopB := startTransportE2EDaptin(t, portB, httpsPortB, baseB, transportE2EDaptinOptions{
		databaseType: "postgres", connectionString: dsn, olricPort: olricPortB,
		olricPeers: net.JoinHostPort("127.0.0.1", fmt.Sprint(olricPortA+1)),
	})
	defer stopB()

	client := &http.Client{Timeout: 20 * time.Second}
	token, _ := transportE2ESignupSigninAdmin(t, client, baseA)
	credentialReference := createLLME2ECatalog(t, client, baseA, token, llmE2ECatalog{
		name: "llm-multinode-e2e", upstreamURL: upstream.URL, apiKey: "multinode-initial-key",
		upstreamModel: "multinode-upstream", operations: []string{"chat"}, maxConcurrency: 4,
	})
	waitForLLME2EModel(t, client, baseA, token, "llm-multinode-e2e")
	waitForLLME2EModel(t, client, baseB, token, "llm-multinode-e2e")

	assertTransportE2EString(t, invokeLLMMultinodeChat(t, client, baseA, token), "choices.0.message.content", "multinode-ok")
	assertTransportE2EString(t, invokeLLMMultinodeChat(t, client, baseB, token), "choices.0.message.content", "multinode-ok")

	rotatedKey := "multinode-rotated-key"
	patchBody, err := json.Marshal(map[string]interface{}{
		"data": map[string]interface{}{
			"type": "credential", "id": credentialReference,
			"attributes": map[string]interface{}{"content": `{"api_key":"` + rotatedKey + `"}`},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	patchRequest, err := http.NewRequest(http.MethodPatch, baseA+"/api/credential/"+credentialReference, bytes.NewReader(patchBody))
	if err != nil {
		t.Fatal(err)
	}
	patchRequest.Header.Set("Authorization", "Bearer "+token)
	patchRequest.Header.Set("Content-Type", "application/vnd.api+json")
	transportE2EDoJSON(t, client, patchRequest)
	expectedKey.Store(rotatedKey)

	deadline := time.Now().Add(5 * time.Second)
	for {
		response, status := tryLLMMultinodeChat(client, baseB, token)
		if status == http.StatusOK {
			assertTransportE2EString(t, response, "choices.0.message.content", "multinode-ok")
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("second Daptin process did not converge after credential rotation: status=%d response=%#v", status, response)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func waitForLLME2EModel(t testing.TB, client *http.Client, baseURL string, token string, modelName string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		models := transportE2EGetJSON(t, client, baseURL+"/v1/models", token)
		if data, ok := transportE2EPath(models, "data"); ok {
			if entries, valid := data.([]interface{}); valid {
				for _, entry := range entries {
					if id, present := transportE2EPath(entry, "id"); present && id == modelName {
						return
					}
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("Daptin process at %s did not converge on the LLM model: %#v", baseURL, models)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func invokeLLMMultinodeChat(t *testing.T, client *http.Client, baseURL string, token string) interface{} {
	t.Helper()
	response, status := tryLLMMultinodeChat(client, baseURL, token)
	if status != http.StatusOK {
		t.Fatalf("LLM chat at %s returned %d: %#v", baseURL, status, response)
	}
	return response
}

func tryLLMMultinodeChat(client *http.Client, baseURL string, token string) (interface{}, int) {
	body := bytes.NewBufferString(`{"model":"llm-multinode-e2e","messages":[{"role":"user","content":"hello"}]}`)
	request, err := http.NewRequest(http.MethodPost, baseURL+"/v1/chat/completions", body)
	if err != nil {
		return map[string]interface{}{"request_error": err.Error()}, 0
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return map[string]interface{}{"request_error": err.Error()}, 0
	}
	defer response.Body.Close()
	payload, _ := io.ReadAll(response.Body)
	var decoded interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		decoded = map[string]interface{}{"body": string(payload)}
	}
	return decoded, response.StatusCode
}

func freeLLMMultinodePort(t testing.TB, used map[int]bool) int {
	t.Helper()
	for attempt := 0; attempt < 100; attempt++ {
		listener, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()
		if !used[port] {
			used[port] = true
			return port
		}
	}
	t.Fatal("allocate distinct Daptin E2E port")
	return 0
}

func freeLLMMultinodePortPair(t testing.TB, used map[int]bool) int {
	t.Helper()
	for attempt := 0; attempt < 100; attempt++ {
		first, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			t.Fatal(err)
		}
		port := first.Addr().(*net.TCPAddr).Port
		if port == 65535 || used[port] || used[port+1] {
			_ = first.Close()
			continue
		}
		second, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", fmt.Sprint(port+1)))
		_ = first.Close()
		if err != nil {
			continue
		}
		_ = second.Close()
		used[port] = true
		used[port+1] = true
		return port
	}
	t.Fatal("allocate adjacent Olric ports")
	return 0
}
