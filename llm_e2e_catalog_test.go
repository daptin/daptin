package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

type llmE2ECatalog struct {
	name           string
	upstreamURL    string
	apiKey         string
	upstreamModel  string
	operations     []string
	maxConcurrency int
}

func createLLME2ECatalog(t testing.TB, client *http.Client, baseURL string, token string, specification llmE2ECatalog) string {
	t.Helper()
	operations, err := json.Marshal(specification.operations)
	if err != nil {
		t.Fatal(err)
	}
	credentialContent, err := json.Marshal(map[string]string{"api_key": specification.apiKey})
	if err != nil {
		t.Fatal(err)
	}
	credential := transportE2EPostJSON(t, client, baseURL+"/api/credential", token, map[string]interface{}{
		"data": map[string]interface{}{"type": "credential", "attributes": map[string]interface{}{
			"name": specification.name, "content": string(credentialContent),
		}},
	})
	credentialReference := transportE2EReferenceID(t, credential)
	provider := transportE2EPostJSON(t, client, baseURL+"/api/llm_provider", token, map[string]interface{}{
		"data": map[string]interface{}{"type": "llm_provider", "attributes": map[string]interface{}{
			"name": specification.name, "provider_type": "openai-compatible", "base_url": specification.upstreamURL + "/v1",
			"provider_parameters": `{}`, "allow_insecure": true, "allow_private_network": true, "enable": true,
			"credential_id": credentialReference,
		}},
	})
	providerReference := transportE2EReferenceID(t, provider)
	model := transportE2EPostJSON(t, client, baseURL+"/api/llm_model", token, map[string]interface{}{
		"data": map[string]interface{}{"type": "llm_model", "attributes": map[string]interface{}{
			"name": specification.name, "operations": string(operations), "capabilities": `{}`,
			"routing_strategy": "priority_weighted", "fallback_models": `[]`, "default_parameters": `{}`,
			"unsupported_parameter_policy": "reject", "enable": true,
		}},
	})
	modelReference := transportE2EReferenceID(t, model)
	transportE2EPostJSON(t, client, baseURL+"/api/llm_deployment", token, map[string]interface{}{
		"data": map[string]interface{}{"type": "llm_deployment", "attributes": map[string]interface{}{
			"name": specification.name, "llm_model_id": modelReference, "llm_provider_id": providerReference,
			"upstream_model": specification.upstreamModel, "operations": string(operations), "priority": 0, "weight": 1,
			"request_timeout_ms": 5000, "connect_timeout_ms": 1000, "max_concurrency": specification.maxConcurrency, "rpm": -1, "tpm": -1,
			"pricing": `{}`, "parameters": `{}`, "health_check": `{}`, "enable": true,
		}},
	})
	return credentialReference
}
