package actions

import (
	"context"
	"fmt"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/llm"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type llmChatActionPerformer struct {
	cruds    map[string]*resource.DbResource
	provider *llm.GoAIProvider
}

func (d *llmChatActionPerformer) Name() string {
	return "$llm.chat"
}

func (d *llmChatActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	modelName, _ := inFieldMap["model"].(string)
	if modelName == "" {
		return nil, nil, []error{fmt.Errorf("model is required")}
	}

	// Resolve provider by model name or explicit credential
	llmProvider, err := d.cruds["world"].ResolveLLMProviderByModel(modelName, transaction)
	if err != nil {
		log.Errorf("[$llm.chat] failed to resolve provider for model=%s: %v", modelName, err)
		return nil, nil, []error{err}
	}

	// Build OpenAI-format request from inFieldMap
	req := llm.OpenAIChatRequest{
		Model: modelName,
	}

	// Messages
	if msgs, ok := inFieldMap["messages"]; ok {
		msgBytes, _ := json.Marshal(msgs)
		var messages []llm.OpenAIMessage
		err := json.Unmarshal(msgBytes, &messages)
		if err == nil {
			req.Messages = messages
		}
	}

	// System prompt convenience
	if systemPrompt, ok := inFieldMap["system_prompt"].(string); ok && systemPrompt != "" {
		req.Messages = append([]llm.OpenAIMessage{{Role: "system", Content: systemPrompt}}, req.Messages...)
	}

	// Optional parameters
	if v, ok := inFieldMap["max_tokens"]; ok {
		if maxTokens, ok := toInt(v); ok {
			req.MaxTokens = &maxTokens
		}
	}
	if v, ok := inFieldMap["temperature"]; ok {
		if temp, ok := toFloat64(v); ok {
			req.Temperature = &temp
		}
	}
	if v, ok := inFieldMap["top_p"]; ok {
		if topP, ok := toFloat64(v); ok {
			req.TopP = &topP
		}
	}
	if v, ok := inFieldMap["seed"]; ok {
		if seed, ok := toInt(v); ok {
			req.Seed = &seed
		}
	}
	if v, ok := inFieldMap["stop"]; ok {
		req.Stop = v
	}

	// Tools
	if tools, ok := inFieldMap["tools"]; ok {
		toolBytes, _ := json.Marshal(tools)
		var openAITools []llm.OpenAITool
		err := json.Unmarshal(toolBytes, &openAITools)
		if err == nil {
			req.Tools = openAITools
		}
	}

	// Tool choice
	if tc, ok := inFieldMap["tool_choice"]; ok {
		req.ToolChoice = tc
	}

	// Response format
	if rf, ok := inFieldMap["response_format"]; ok {
		rfBytes, _ := json.Marshal(rf)
		var responseFormat llm.OpenAIResponseFormat
		err := json.Unmarshal(rfBytes, &responseFormat)
		if err == nil {
			req.ResponseFormat = &responseFormat
		}
	}

	// Extra params for provider-specific features
	if ep, ok := inFieldMap["extra_params"].(map[string]interface{}); ok {
		req.ExtraParams = ep
	}

	log.Infof("[$llm.chat] provider=%s model=%s messages=%d", llmProvider.Name, modelName, len(req.Messages))

	// Execute non-streaming chat completion
	response, err := d.provider.ChatCompletion(context.Background(), llmProvider, req, transaction)
	if err != nil {
		log.Errorf("[$llm.chat] failed: provider=%s model=%s error=%v", llmProvider.Name, modelName, err)
		return nil, nil, []error{err}
	}

	// Build response map
	responseMap := make(map[string]interface{})
	if len(response.Choices) > 0 {
		choice := response.Choices[0]
		responseMap["content"] = choice.Message.Content
		responseMap["role"] = choice.Message.Role
		responseMap["finish_reason"] = choice.FinishReason
		if len(choice.Message.ToolCalls) > 0 {
			responseMap["tool_calls"] = choice.Message.ToolCalls
		}
	}
	responseMap["model"] = response.Model
	if response.Usage != nil {
		responseMap["usage"] = map[string]interface{}{
			"prompt_tokens":     response.Usage.PromptTokens,
			"completion_tokens": response.Usage.CompletionTokens,
			"total_tokens":      response.Usage.TotalTokens,
		}
	}
	responseMap["raw"] = response

	return api2go.Response{
			Res: api2go.NewApi2GoModelWithData("$llm.response", nil, 0, nil, responseMap),
		}, []actionresponse.ActionResponse{{
			ResponseType: request.Type,
			Attributes:   responseMap,
		}}, nil
}

func NewLLMChatPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, provider *llm.GoAIProvider) (actionresponse.ActionPerformerInterface, error) {
	handler := llmChatActionPerformer{
		cruds:    cruds,
		provider: provider,
	}
	log.Infof("[$llm.chat] performer registered")
	return &handler, nil
}

// toInt converts various numeric types to int.
func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	}
	return 0, false
}

// toFloat64 converts various numeric types to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	}
	return 0, false
}
