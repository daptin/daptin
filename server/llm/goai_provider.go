package llm

import (
	"context"
	stdjson "encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/zendev-sh/goai"
	"github.com/zendev-sh/goai/provider"
	"github.com/zendev-sh/goai/provider/anthropic"
	"github.com/zendev-sh/goai/provider/azure"
	"github.com/zendev-sh/goai/provider/bedrock"
	"github.com/zendev-sh/goai/provider/compat"
	"github.com/zendev-sh/goai/provider/google"
	"github.com/zendev-sh/goai/provider/ollama"
	"github.com/zendev-sh/goai/provider/vertex"
)

// knownBaseURLs maps provider types to their default API base URLs.
// All these providers use OpenAI-compatible APIs and go through the compat provider.
// Adding a new provider = adding one line here.
var knownBaseURLs = map[string]string{
	"openai":     "https://api.openai.com/v1",
	"cerebras":   "https://api.cerebras.ai/v1",
	"cloudflare": "https://api.cloudflare.com/client/v4",
	"cohere":     "https://api.cohere.com/v2",
	"deepinfra":  "https://api.deepinfra.com/v1/openai",
	"deepseek":   "https://api.deepseek.com",
	"fireworks":  "https://api.fireworks.ai/inference/v1",
	"fptcloud":   "https://api.fptcloud.com/v1",
	"groq":       "https://api.groq.com/openai/v1",
	"minimax":    "https://api.minimax.io/anthropic",
	"mistral":    "https://api.mistral.ai/v1",
	"nvidia":     "https://integrate.api.nvidia.com/v1",
	"openrouter": "https://openrouter.ai/api/v1",
	"perplexity": "https://api.perplexity.ai",
	"runpod":     "https://api.runpod.ai/v2",
	"together":   "https://api.together.xyz/v1",
	"vllm":       "http://localhost:8000/v1",
	"xai":        "https://api.x.ai/v1",
}

// GoAIProvider manages GoAI model instances and provides LLM operations.
// Parallel to rclone for cloud storage — GoAI abstracts all LLM providers.
type GoAIProvider struct {
	cruds map[string]*resource.DbResource
}

// NewGoAIProvider creates a new GoAI provider manager.
func NewGoAIProvider(cruds map[string]*resource.DbResource) *GoAIProvider {
	log.Infof("[llm] GoAI provider manager initialized with %d known provider base URLs", len(knownBaseURLs))
	return &GoAIProvider{
		cruds: cruds,
	}
}

// resolveCredential loads and decrypts the API key from the credential table.
func (p *GoAIProvider) resolveCredential(llmProvider rootpojo.LLMProvider, tx *sqlx.Tx) (map[string]interface{}, error) {
	if llmProvider.CredentialName == "" {
		return nil, nil
	}
	cred, err := p.cruds["credential"].GetCredentialByName(llmProvider.CredentialName, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential [%s]: %v", llmProvider.CredentialName, err)
	}
	return cred.DataMap, nil
}

// ResolveChatModel creates a GoAI LanguageModel from an LLMProvider + Credential.
// Only providers with unique (non-OpenAI) APIs get explicit cases.
// All OpenAI-compatible providers (19+) go through the compat provider with knownBaseURLs.
func (p *GoAIProvider) ResolveChatModel(llmProvider rootpojo.LLMProvider, modelName string, tx *sqlx.Tx) (provider.LanguageModel, error) {
	credData, err := p.resolveCredential(llmProvider, tx)
	if err != nil {
		return nil, err
	}
	apiKey, _ := credData["api_key"].(string)

	log.Debugf("[llm] resolving chat model: provider=%s model=%s base_url=%s", llmProvider.ProviderType, modelName, llmProvider.BaseUrl)

	switch strings.ToLower(llmProvider.ProviderType) {

	// --- Providers with unique (non-OpenAI) APIs ---

	case "anthropic":
		opts := []anthropic.Option{}
		if apiKey != "" {
			opts = append(opts, anthropic.WithAPIKey(apiKey))
		}
		if llmProvider.BaseUrl != "" {
			opts = append(opts, anthropic.WithBaseURL(llmProvider.BaseUrl))
		}
		return anthropic.Chat(modelName, opts...), nil

	case "gemini", "google":
		opts := []google.Option{}
		if apiKey != "" {
			opts = append(opts, google.WithAPIKey(apiKey))
		}
		if llmProvider.BaseUrl != "" {
			opts = append(opts, google.WithBaseURL(llmProvider.BaseUrl))
		}
		return google.Chat(modelName, opts...), nil

	case "vertex":
		opts := []vertex.Option{}
		if apiKey != "" {
			opts = append(opts, vertex.WithAPIKey(apiKey))
		}
		// Project and location from provider_parameters; ADC auto-resolves from env
		if project, ok := llmProvider.ProviderParameters["project"].(string); ok {
			opts = append(opts, vertex.WithProject(project))
		}
		if location, ok := llmProvider.ProviderParameters["location"].(string); ok {
			opts = append(opts, vertex.WithLocation(location))
		}
		return vertex.Chat(modelName, opts...), nil

	case "azure":
		opts := []azure.Option{}
		if apiKey != "" {
			opts = append(opts, azure.WithAPIKey(apiKey))
		}
		if llmProvider.BaseUrl != "" {
			opts = append(opts, azure.WithEndpoint(llmProvider.BaseUrl))
		}
		if apiVersion, ok := llmProvider.ProviderParameters["api_version"].(string); ok {
			opts = append(opts, azure.WithAPIVersion(apiVersion))
		}
		return azure.Chat(modelName, opts...), nil

	case "bedrock":
		opts := []bedrock.Option{}
		// Bedrock uses AWS credentials from credential DataMap
		if accessKey, ok := credData["access_key_id"].(string); ok {
			opts = append(opts, bedrock.WithAccessKey(accessKey))
		}
		if secretKey, ok := credData["secret_access_key"].(string); ok {
			opts = append(opts, bedrock.WithSecretKey(secretKey))
		}
		if sessionToken, ok := credData["session_token"].(string); ok {
			opts = append(opts, bedrock.WithSessionToken(sessionToken))
		}
		if region, ok := credData["region"].(string); ok {
			opts = append(opts, bedrock.WithRegion(region))
		}
		if llmProvider.BaseUrl != "" {
			opts = append(opts, bedrock.WithBaseURL(llmProvider.BaseUrl))
		}
		return bedrock.Chat(modelName, opts...), nil

	case "ollama":
		opts := []ollama.Option{}
		if llmProvider.BaseUrl != "" {
			opts = append(opts, ollama.WithBaseURL(llmProvider.BaseUrl))
		}
		return ollama.Chat(modelName, opts...), nil

	// --- All OpenAI-compatible providers go through compat ---
	default:
		return p.resolveCompatChat(llmProvider.ProviderType, modelName, apiKey, llmProvider.BaseUrl), nil
	}
}

// resolveCompatChat creates a compat (OpenAI-compatible) chat model.
// Used for openai, groq, deepseek, mistral, cerebras, nvidia, xai, together,
// fireworks, deepinfra, openrouter, perplexity, cohere, minimax, cloudflare,
// fptcloud, runpod, vllm, and any unknown provider with a base_url.
func (p *GoAIProvider) resolveCompatChat(providerType, modelName, apiKey, baseURL string) provider.LanguageModel {
	opts := []compat.Option{compat.WithProviderID(providerType)}
	if apiKey != "" {
		opts = append(opts, compat.WithAPIKey(apiKey))
	}
	resolvedURL := baseURL
	if resolvedURL == "" {
		resolvedURL = knownBaseURLs[strings.ToLower(providerType)]
	}
	if resolvedURL != "" {
		opts = append(opts, compat.WithBaseURL(resolvedURL))
	}
	log.Debugf("[llm] compat provider: type=%s model=%s base_url=%s", providerType, modelName, resolvedURL)
	return compat.Chat(modelName, opts...)
}

// ResolveEmbeddingModel creates a GoAI EmbeddingModel from an LLMProvider + Credential.
// Same pattern as ResolveChatModel: unique cases for non-OpenAI APIs, compat for the rest.
func (p *GoAIProvider) ResolveEmbeddingModel(llmProvider rootpojo.LLMProvider, modelName string, tx *sqlx.Tx) (provider.EmbeddingModel, error) {
	credData, err := p.resolveCredential(llmProvider, tx)
	if err != nil {
		return nil, err
	}
	apiKey, _ := credData["api_key"].(string)

	log.Debugf("[llm] resolving embedding model: provider=%s model=%s", llmProvider.ProviderType, modelName)

	switch strings.ToLower(llmProvider.ProviderType) {
	case "gemini", "google":
		opts := []google.Option{}
		if apiKey != "" {
			opts = append(opts, google.WithAPIKey(apiKey))
		}
		if llmProvider.BaseUrl != "" {
			opts = append(opts, google.WithBaseURL(llmProvider.BaseUrl))
		}
		return google.Embedding(modelName, opts...), nil

	case "vertex":
		opts := []vertex.Option{}
		if apiKey != "" {
			opts = append(opts, vertex.WithAPIKey(apiKey))
		}
		if project, ok := llmProvider.ProviderParameters["project"].(string); ok {
			opts = append(opts, vertex.WithProject(project))
		}
		if location, ok := llmProvider.ProviderParameters["location"].(string); ok {
			opts = append(opts, vertex.WithLocation(location))
		}
		return vertex.Embedding(modelName, opts...), nil

	case "bedrock":
		opts := []bedrock.Option{}
		if accessKey, ok := credData["access_key_id"].(string); ok {
			opts = append(opts, bedrock.WithAccessKey(accessKey))
		}
		if secretKey, ok := credData["secret_access_key"].(string); ok {
			opts = append(opts, bedrock.WithSecretKey(secretKey))
		}
		if region, ok := credData["region"].(string); ok {
			opts = append(opts, bedrock.WithRegion(region))
		}
		return bedrock.Embedding(modelName, opts...), nil

	case "ollama":
		opts := []ollama.Option{}
		if llmProvider.BaseUrl != "" {
			opts = append(opts, ollama.WithBaseURL(llmProvider.BaseUrl))
		}
		return ollama.Embedding(modelName, opts...), nil

	default:
		// All OpenAI-compatible providers (openai, nvidia, cohere, cloudflare, fptcloud, vllm, etc.)
		opts := []compat.Option{compat.WithProviderID(llmProvider.ProviderType)}
		if apiKey != "" {
			opts = append(opts, compat.WithAPIKey(apiKey))
		}
		resolvedURL := llmProvider.BaseUrl
		if resolvedURL == "" {
			resolvedURL = knownBaseURLs[strings.ToLower(llmProvider.ProviderType)]
		}
		if resolvedURL != "" {
			opts = append(opts, compat.WithBaseURL(resolvedURL))
		}
		return compat.Embedding(modelName, opts...), nil
	}
}

// buildGoAIOptions converts OpenAI-format request fields to GoAI options.
func buildGoAIOptions(req OpenAIChatRequest, providerParams map[string]interface{}) []goai.Option {
	opts := []goai.Option{}

	// Apply provider-level parameters as GoAI provider options
	if len(providerParams) > 0 {
		provOpts := make(map[string]any)
		for k, v := range providerParams {
			provOpts[k] = v
		}
		opts = append(opts, goai.WithProviderOptions(provOpts))
	}

	// Extract system message and user messages
	var systemPrompt string
	var messages []provider.Message
	for _, msg := range req.Messages {
		contentStr := extractContentString(msg.Content)
		switch msg.Role {
		case "system":
			systemPrompt = contentStr
		case "user":
			messages = append(messages, provider.Message{
				Role:    provider.RoleUser,
				Content: []provider.Part{{Type: provider.PartText, Text: contentStr}},
			})
		case "assistant":
			parts := []provider.Part{{Type: provider.PartText, Text: contentStr}}
			for _, tc := range msg.ToolCalls {
				parts = append(parts, provider.Part{
					Type:       provider.PartToolCall,
					ToolCallID: tc.ID,
					ToolName:   tc.Function.Name,
					ToolInput:  stdjson.RawMessage(tc.Function.Arguments),
				})
			}
			messages = append(messages, provider.Message{
				Role:    provider.RoleAssistant,
				Content: parts,
			})
		case "tool":
			messages = append(messages, provider.Message{
				Role: provider.RoleTool,
				Content: []provider.Part{{
					Type:       provider.PartToolResult,
					ToolCallID: msg.ToolCallID,
					ToolOutput: contentStr,
				}},
			})
		}
	}

	if systemPrompt != "" {
		opts = append(opts, goai.WithSystem(systemPrompt))
	}
	if len(messages) > 0 {
		opts = append(opts, goai.WithMessages(messages...))
	}
	if req.MaxTokens != nil {
		opts = append(opts, goai.WithMaxOutputTokens(*req.MaxTokens))
	}
	if req.Temperature != nil {
		opts = append(opts, goai.WithTemperature(*req.Temperature))
	}
	if req.TopP != nil {
		opts = append(opts, goai.WithTopP(*req.TopP))
	}
	if req.Seed != nil {
		opts = append(opts, goai.WithSeed(*req.Seed))
	}
	if req.FrequencyPenalty != nil {
		opts = append(opts, goai.WithFrequencyPenalty(*req.FrequencyPenalty))
	}
	if req.PresencePenalty != nil {
		opts = append(opts, goai.WithPresencePenalty(*req.PresencePenalty))
	}

	// Stop sequences
	if req.Stop != nil {
		switch v := req.Stop.(type) {
		case string:
			opts = append(opts, goai.WithStopSequences(v))
		case []interface{}:
			stops := make([]string, 0, len(v))
			for _, s := range v {
				if str, ok := s.(string); ok {
					stops = append(stops, str)
				}
			}
			opts = append(opts, goai.WithStopSequences(stops...))
		}
	}

	// Tools
	if len(req.Tools) > 0 {
		goaiTools := make([]goai.Tool, 0, len(req.Tools))
		for _, t := range req.Tools {
			if t.Type == "function" {
				paramsJSON, _ := json.Marshal(t.Function.Parameters)
				goaiTools = append(goaiTools, goai.Tool{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					InputSchema: paramsJSON,
				})
			}
		}
		if len(goaiTools) > 0 {
			opts = append(opts, goai.WithTools(goaiTools...))
		}
	}

	// Tool choice
	if req.ToolChoice != nil {
		switch v := req.ToolChoice.(type) {
		case string:
			opts = append(opts, goai.WithToolChoice(v))
		case map[string]interface{}:
			if fn, ok := v["function"].(map[string]interface{}); ok {
				if name, ok := fn["name"].(string); ok {
					opts = append(opts, goai.WithToolChoice(name))
				}
			}
		}
	}

	// Provider-specific options passthrough (request-level overrides provider defaults)
	if len(req.ExtraParams) > 0 {
		opts = append(opts, goai.WithProviderOptions(req.ExtraParams))
	}

	return opts
}

// ChatCompletion performs a non-streaming chat completion.
func (p *GoAIProvider) ChatCompletion(ctx context.Context, llmProvider rootpojo.LLMProvider, req OpenAIChatRequest, tx *sqlx.Tx) (*OpenAIChatResponse, error) {
	model, err := p.ResolveChatModel(llmProvider, req.Model, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve model: %v", err)
	}

	log.Infof("[llm] chat request: provider=%s model=%s messages=%d stream=false", llmProvider.Name, req.Model, len(req.Messages))

	opts := buildGoAIOptions(req, llmProvider.ProviderParameters)
	result, err := goai.GenerateText(ctx, model, opts...)
	if err != nil {
		log.Errorf("[llm] chat failed: provider=%s model=%s error=%v", llmProvider.Name, req.Model, err)
		return nil, err
	}

	responseID := "chatcmpl-daptin"
	if result.Response.ID != "" {
		responseID = result.Response.ID
	}

	respMessage := OpenAIMessage{
		Role:    "assistant",
		Content: result.Text,
	}

	if len(result.ToolCalls) > 0 {
		toolCalls := make([]ToolCall, 0, len(result.ToolCalls))
		for _, tc := range result.ToolCalls {
			toolCalls = append(toolCalls, ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      tc.Name,
					Arguments: string(tc.Input),
				},
			})
		}
		respMessage.ToolCalls = toolCalls
	}

	finishReason := mapFinishReason(result.FinishReason)

	response := &OpenAIChatResponse{
		ID:      responseID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []OpenAIChoice{
			{
				Index:        0,
				Message:      respMessage,
				FinishReason: finishReason,
			},
		},
		Usage: &OpenAIUsage{
			PromptTokens:     result.TotalUsage.InputTokens,
			CompletionTokens: result.TotalUsage.OutputTokens,
			TotalTokens:      result.TotalUsage.InputTokens + result.TotalUsage.OutputTokens,
		},
	}

	log.Infof("[llm] chat response: model=%s finish=%s tokens_in=%d tokens_out=%d",
		req.Model, finishReason, result.TotalUsage.InputTokens, result.TotalUsage.OutputTokens)

	return response, nil
}

// ChatCompletionStream performs a streaming chat completion, calling flush for each chunk.
func (p *GoAIProvider) ChatCompletionStream(ctx context.Context, llmProvider rootpojo.LLMProvider, req OpenAIChatRequest, tx *sqlx.Tx, flush func(chunk OpenAIChatChunk)) error {
	model, err := p.ResolveChatModel(llmProvider, req.Model, tx)
	if err != nil {
		return fmt.Errorf("failed to resolve model: %v", err)
	}

	log.Infof("[llm] chat request: provider=%s model=%s messages=%d stream=true", llmProvider.Name, req.Model, len(req.Messages))

	opts := buildGoAIOptions(req, llmProvider.ProviderParameters)
	stream, err := goai.StreamText(ctx, model, opts...)
	if err != nil {
		log.Errorf("[llm] stream failed: provider=%s model=%s error=%v", llmProvider.Name, req.Model, err)
		return err
	}

	responseID := "chatcmpl-daptin-stream"
	created := time.Now().Unix()
	chunkCount := 0

	flush(OpenAIChatChunk{
		ID: responseID, Object: "chat.completion.chunk", Created: created, Model: req.Model,
		Choices: []OpenAIChunkChoice{{Index: 0, Delta: OpenAIDelta{Role: "assistant"}}},
	})
	chunkCount++

	for text := range stream.TextStream() {
		flush(OpenAIChatChunk{
			ID: responseID, Object: "chat.completion.chunk", Created: created, Model: req.Model,
			Choices: []OpenAIChunkChoice{{Index: 0, Delta: OpenAIDelta{Content: text}}},
		})
		chunkCount++
	}

	if err := stream.Err(); err != nil {
		log.Errorf("[llm] stream error: provider=%s model=%s error=%v", llmProvider.Name, req.Model, err)
		return err
	}

	result := stream.Result()
	finishReason := mapFinishReason(result.FinishReason)

	flush(OpenAIChatChunk{
		ID: responseID, Object: "chat.completion.chunk", Created: created, Model: req.Model,
		Choices: []OpenAIChunkChoice{{Index: 0, Delta: OpenAIDelta{}, FinishReason: &finishReason}},
		Usage: &OpenAIUsage{
			PromptTokens: result.TotalUsage.InputTokens, CompletionTokens: result.TotalUsage.OutputTokens,
			TotalTokens: result.TotalUsage.InputTokens + result.TotalUsage.OutputTokens,
		},
	})

	log.Infof("[llm] stream complete: model=%s chunks=%d tokens_in=%d tokens_out=%d",
		req.Model, chunkCount, result.TotalUsage.InputTokens, result.TotalUsage.OutputTokens)
	return nil
}

// Embedding generates embeddings for the given input.
func (p *GoAIProvider) Embedding(ctx context.Context, llmProvider rootpojo.LLMProvider, req OpenAIEmbeddingRequest, tx *sqlx.Tx) (*OpenAIEmbeddingResponse, error) {
	model, err := p.ResolveEmbeddingModel(llmProvider, req.Model, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve embedding model: %v", err)
	}

	var inputs []string
	switch v := req.Input.(type) {
	case string:
		inputs = []string{v}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				inputs = append(inputs, s)
			}
		}
	}

	log.Infof("[llm] embedding request: provider=%s model=%s inputs=%d", llmProvider.Name, req.Model, len(inputs))

	result, err := goai.EmbedMany(ctx, model, inputs)
	if err != nil {
		log.Errorf("[llm] embedding failed: provider=%s model=%s error=%v", llmProvider.Name, req.Model, err)
		return nil, err
	}

	data := make([]OpenAIEmbedding, 0, len(result.Embeddings))
	for i, emb := range result.Embeddings {
		data = append(data, OpenAIEmbedding{Object: "embedding", Embedding: emb, Index: i})
	}

	log.Infof("[llm] embedding response: model=%s embeddings=%d", req.Model, len(data))
	return &OpenAIEmbeddingResponse{
		Object: "list", Data: data, Model: req.Model,
		Usage: OpenAIEmbeddingUsage{PromptTokens: result.Usage.InputTokens, TotalTokens: result.Usage.InputTokens},
	}, nil
}

func extractContentString(content interface{}) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, part := range v {
			if m, ok := part.(map[string]interface{}); ok {
				if t, ok := m["type"].(string); ok && t == "text" {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func mapFinishReason(reason provider.FinishReason) string {
	switch reason {
	case provider.FinishStop:
		return "stop"
	case provider.FinishToolCalls:
		return "tool_calls"
	case provider.FinishLength:
		return "length"
	case provider.FinishContentFilter:
		return "content_filter"
	default:
		return "stop"
	}
}
