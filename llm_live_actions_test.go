//go:build live

package main

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
)

type liveActionProvider struct {
	name           string
	apiVersion     string
	providerType   string
	apiKeyEnv      string
	chatModel      string
	embeddingModel string
}

func TestLiveLLMActions(t *testing.T) {
	providers := []liveActionProvider{
		{name: "google", apiVersion: "v1beta-openai", providerType: "google", apiKeyEnv: "GOOGLE_API_KEY", chatModel: "gemini-3.7-flash", embeddingModel: "gemini-embedding-001"},
		{name: "openrouter", apiVersion: "v1", providerType: "openrouter", apiKeyEnv: "OPENROUTER_API_KEY", chatModel: "openai/gpt-4o-mini", embeddingModel: "openai/text-embedding-3-small"},
		{name: "lilac", apiVersion: "v1", providerType: "lilac", apiKeyEnv: "LILAC_API_KEY", chatModel: "moonshotai/kimi-k2.6"},
	}
	for _, provider := range providers {
		if os.Getenv(provider.apiKeyEnv) == "" {
			t.Fatalf("%s is required for the live Daptin action gate", provider.apiKeyEnv)
		}
	}

	ensureServer()
	const baseURL = "http://localhost:6337"
	client := &http.Client{Timeout: 2 * time.Minute}
	token, email := transportE2ESignupSigninAdmin(t, client, baseURL)
	sessionUser := llmE2EActionUser(t, email)

	for _, provider := range providers {
		provider := provider
		t.Run(provider.name, func(t *testing.T) {
			t.Run("chat", func(t *testing.T) {
				runLiveChatActionCell(t, client, baseURL, token, sessionUser, provider, os.Getenv(provider.apiKeyEnv))
			})
			if provider.embeddingModel == "" {
				return
			}
			t.Run("embedding", func(t *testing.T) {
				runLiveEmbeddingActionCell(t, client, baseURL, token, sessionUser, provider, os.Getenv(provider.apiKeyEnv))
			})
		})
	}
}

func runLiveChatActionCell(t *testing.T, client *http.Client, baseURL, token string, sessionUser *auth.SessionUser, provider liveActionProvider, apiKey string) {
	t.Helper()
	modelName := "live-action-" + provider.name + "-chat"
	createLLME2ECatalog(t, client, baseURL, token, llmE2ECatalog{
		name: modelName, providerType: provider.providerType, apiKey: apiKey, upstreamModel: provider.chatModel,
		operations: []string{"chat"}, maxConcurrency: 2, requestTimeoutMS: 90_000, connectTimeoutMS: 10_000,
	})
	waitForLLME2EModel(t, client, baseURL, token, modelName)
	chat := invokeLLME2EAction(t, sessionUser, "$llm.chat", map[string]interface{}{
		"model":    modelName,
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "Reply with the single word pong."}},
	})
	content, contentFound := chat["content"].(string)
	totalTokens, usageFound := transportE2EPath(chat, "usage.total_tokens")
	totalTokenCount, usageErr := resource.ResourceRowInt64(totalTokens)
	if !contentFound || content == "" || !usageFound || usageErr != nil || totalTokenCount <= 0 {
		t.Fatal("live chat action returned an invalid normalized response")
	}
	t.Logf("certification entrypoint=$llm.chat provider=%s api_version=%s model=%s normalized_result=success usage_available=true skip_reason=none",
		provider.name, provider.apiVersion, provider.chatModel)
}

func runLiveEmbeddingActionCell(t *testing.T, client *http.Client, baseURL, token string, sessionUser *auth.SessionUser, provider liveActionProvider, apiKey string) {
	t.Helper()
	modelName := "live-action-" + provider.name + "-embedding"
	createLLME2ECatalog(t, client, baseURL, token, llmE2ECatalog{
		name: modelName, providerType: provider.providerType, apiKey: apiKey, upstreamModel: provider.embeddingModel,
		operations: []string{"embeddings"}, maxConcurrency: 2, requestTimeoutMS: 90_000, connectTimeoutMS: 10_000,
	})
	waitForLLME2EModel(t, client, baseURL, token, modelName)
	embedding := invokeLLME2EAction(t, sessionUser, "$llm.embedding", map[string]interface{}{
		"model": modelName, "input": "hello",
	})
	vectors, vectorsFound := embedding["embeddings"]
	totalTokens, usageFound := transportE2EPath(embedding, "usage.total_tokens")
	totalTokenCount, usageErr := resource.ResourceRowInt64(totalTokens)
	vectorList, validVectors := vectors.([]interface{})
	if !vectorsFound || !validVectors || len(vectorList) != 1 || !usageFound || usageErr != nil || totalTokenCount <= 0 {
		t.Fatal("live embedding action returned an invalid normalized response")
	}
	t.Logf("certification entrypoint=$llm.embedding provider=%s api_version=%s model=%s normalized_result=success usage_available=true skip_reason=none",
		provider.name, provider.apiVersion, provider.embeddingModel)
}
