package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

type llmE2ECatalog struct {
	name             string
	upstreamURL      string
	providerType     string
	apiKey           string
	upstreamModel    string
	operations       []string
	maxConcurrency   int
	requestTimeoutMS int
	connectTimeoutMS int
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
	providerType := specification.providerType
	if providerType == "" {
		providerType = "openai-compatible"
	}
	providerBaseURL := ""
	allowInsecure, allowPrivateNetwork := false, false
	if specification.upstreamURL != "" {
		providerBaseURL = strings.TrimRight(specification.upstreamURL, "/") + "/v1"
		allowInsecure = strings.HasPrefix(providerBaseURL, "http://")
		allowPrivateNetwork = true
	}
	provider := transportE2EPostJSON(t, client, baseURL+"/api/llm_provider", token, map[string]interface{}{
		"data": map[string]interface{}{"type": "llm_provider", "attributes": map[string]interface{}{
			"name": specification.name, "provider_type": providerType, "base_url": providerBaseURL,
			"provider_parameters": `{}`, "allow_insecure": allowInsecure, "allow_private_network": allowPrivateNetwork, "enable": true,
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
	requestTimeout := specification.requestTimeoutMS
	if requestTimeout == 0 {
		requestTimeout = 5000
	}
	connectTimeout := specification.connectTimeoutMS
	if connectTimeout == 0 {
		connectTimeout = 1000
	}
	transportE2EPostJSON(t, client, baseURL+"/api/llm_deployment", token, map[string]interface{}{
		"data": map[string]interface{}{"type": "llm_deployment", "attributes": map[string]interface{}{
			"name": specification.name, "llm_model_id": modelReference, "llm_provider_id": providerReference,
			"upstream_model": specification.upstreamModel, "operations": string(operations), "priority": 0, "weight": 1,
			"request_timeout_ms": requestTimeout, "connect_timeout_ms": connectTimeout, "max_concurrency": specification.maxConcurrency, "rpm": -1, "tpm": -1,
			"pricing": `{}`, "parameters": `{}`, "health_check": `{}`, "enable": true,
		}},
	})
	return credentialReference
}

func llmE2EActionUser(t testing.TB, email string) *auth.SessionUser {
	t.Helper()
	world := resource.CRUD_MAP["world"]
	userResource := resource.CRUD_MAP[resource.USER_ACCOUNT_TABLE_NAME]
	if world == nil || userResource == nil {
		t.Fatal("runtime canonical resources are unavailable")
	}
	transaction, err := world.Connection().Beginx()
	if err != nil {
		t.Fatal(err)
	}
	users, _, loadErr := userResource.GetRowsByWhereClauseWithTransaction(
		resource.USER_ACCOUNT_TABLE_NAME, nil, transaction, goqu.Ex{"email": email},
	)
	_ = transaction.Rollback()
	if loadErr != nil || len(users) != 1 {
		t.Fatalf("load LLM action session user: rows=%d err=%v", len(users), loadErr)
	}
	userID, err := resource.ResourceRowInt64(users[0]["id"])
	if err != nil {
		t.Fatal(err)
	}
	return &auth.SessionUser{UserId: userID, UserReferenceId: daptinid.InterfaceToDIR(users[0]["reference_id"])}
}

func invokeLLME2EAction(t testing.TB, user *auth.SessionUser, actionName string, input map[string]interface{}, observers ...func(*sqlx.Tx)) map[string]interface{} {
	t.Helper()
	attributes, actionErrors := performLLME2EAction(t, user, actionName, input, observers...)
	if len(actionErrors) != 0 {
		t.Fatalf("%s failed: %v", actionName, actionErrors)
	}
	return attributes
}

func performLLME2EAction(t testing.TB, user *auth.SessionUser, actionName string, input map[string]interface{}, observers ...func(*sqlx.Tx)) (map[string]interface{}, []error) {
	t.Helper()
	if len(observers) > 1 {
		t.Fatal("LLM E2E action accepts at most one transaction observer")
	}
	world := resource.CRUD_MAP["world"]
	if world == nil {
		t.Fatal("runtime world resource is unavailable")
	}
	performer := world.GetActionHandler(actionName)
	if performer == nil {
		t.Fatalf("runtime did not register %s", actionName)
	}
	transaction, err := world.Connection().Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if len(observers) == 1 {
		observers[0](transaction)
	}
	input["requestSessionUser"] = user
	input["httpRequest"] = httptest.NewRequest(http.MethodPost, "/action/world/llm-e2e", nil)
	_, responses, actionErrors := performer.DoAction(actionresponse.Outcome{Type: "llm.e2e"}, input, transaction)
	_ = transaction.Rollback()
	if len(actionErrors) != 0 {
		return nil, actionErrors
	}
	if len(responses) != 1 {
		t.Fatalf("%s responses = %#v", actionName, responses)
	}
	attributes, ok := responses[0].Attributes.(map[string]interface{})
	if !ok {
		t.Fatalf("%s response attributes = %#v", actionName, responses[0].Attributes)
	}
	return attributes, nil
}

func llmE2ETransactionClosed(transaction *sqlx.Tx) bool {
	return errors.Is(transaction.Rollback(), sql.ErrTxDone)
}
