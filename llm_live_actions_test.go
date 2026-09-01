//go:build live

package main

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
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
	availableKeys := 0
	for _, provider := range providers {
		if os.Getenv(provider.apiKeyEnv) != "" {
			availableKeys++
		}
	}
	if availableKeys == 0 {
		t.Fatal("live Daptin action gate requires at least one configured provider key")
	}
	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	stopDaptin := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{schema: llmE2EActionsSchema})
	defer stopDaptin()
	client := &http.Client{Timeout: 2 * time.Minute}
	token := transportE2ESignupSigninAdmin(t, client, baseURL)

	for _, provider := range providers {
		provider := provider
		t.Run(provider.name, func(t *testing.T) {
			apiKey := os.Getenv(provider.apiKeyEnv)
			if apiKey == "" {
				t.Fatalf("%s is required for the live Daptin action gate", provider.apiKeyEnv)
			}
			t.Run("chat", func(t *testing.T) {
				runLiveChatActionCell(t, client, baseURL, token, provider, apiKey)
			})
			if provider.embeddingModel == "" {
				return
			}
			t.Run("embedding", func(t *testing.T) {
				runLiveEmbeddingActionCell(t, client, baseURL, token, provider, apiKey)
			})
		})
	}
}

func runLiveChatActionCell(t *testing.T, client *http.Client, baseURL, token string, provider liveActionProvider, apiKey string) {
	t.Helper()
	modelName := "live-action-" + provider.name + "-chat"
	createLLME2ECatalog(t, client, baseURL, token, llmE2ECatalog{
		name: modelName, providerType: provider.providerType, apiKey: apiKey, upstreamModel: provider.chatModel,
		operations: []string{"chat"}, maxConcurrency: 2, requestTimeoutMS: 90_000, connectTimeoutMS: 10_000,
	})
	waitForLLME2EModel(t, client, baseURL, token, modelName)
	chat := transportE2EPostJSON(t, client, baseURL+"/action/world/llm_e2e_chat", token, map[string]interface{}{
		"attributes": map[string]interface{}{"model": modelName, "prompt": "Reply with the single word pong."},
	})
	content, contentFound := transportE2EPath(chat, "0.Attributes.content")
	contentText, validContent := content.(string)
	totalTokens, usageFound := transportE2EPath(chat, "0.Attributes.usage.total_tokens")
	totalTokenCount, validUsage := totalTokens.(float64)
	if !contentFound || !validContent || contentText == "" || !usageFound || !validUsage || totalTokenCount <= 0 {
		t.Fatal("live chat action returned an invalid normalized response")
	}
	t.Logf("certification entrypoint=$llm.chat provider=%s api_version=%s model=%s normalized_result=success usage_available=true skip_reason=none",
		provider.name, provider.apiVersion, provider.chatModel)
}

func runLiveEmbeddingActionCell(t *testing.T, client *http.Client, baseURL, token string, provider liveActionProvider, apiKey string) {
	t.Helper()
	modelName := "live-action-" + provider.name + "-embedding"
	createLLME2ECatalog(t, client, baseURL, token, llmE2ECatalog{
		name: modelName, providerType: provider.providerType, apiKey: apiKey, upstreamModel: provider.embeddingModel,
		operations: []string{"embeddings"}, maxConcurrency: 2, requestTimeoutMS: 90_000, connectTimeoutMS: 10_000,
	})
	waitForLLME2EModel(t, client, baseURL, token, modelName)
	embedding := transportE2EPostJSON(t, client, baseURL+"/action/world/llm_e2e_embedding", token, map[string]interface{}{
		"attributes": map[string]interface{}{"model": modelName, "input": "hello"},
	})
	vectors, vectorsFound := transportE2EPath(embedding, "0.Attributes.embeddings")
	totalTokens, usageFound := transportE2EPath(embedding, "0.Attributes.usage.total_tokens")
	totalTokenCount, validUsage := totalTokens.(float64)
	vectorList, validVectors := vectors.([]interface{})
	if !vectorsFound || !validVectors || len(vectorList) != 1 || !usageFound || !validUsage || totalTokenCount <= 0 {
		t.Fatal("live embedding action returned an invalid normalized response")
	}
	t.Logf("certification entrypoint=$llm.embedding provider=%s api_version=%s model=%s normalized_result=success usage_available=true skip_reason=none",
		provider.name, provider.apiVersion, provider.embeddingModel)
}
