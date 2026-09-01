package actions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/llm"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/contract"
	"github.com/daptin/llmgateway/protocol/openai"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type llmChatActionPerformer struct {
	gateway *llm.Gateway
	cruds   map[string]*resource.DbResource
}

func (performer *llmChatActionPerformer) Name() string { return "$llm.chat" }

func (performer *llmChatActionPerformer) DoAction(outcome actionresponse.Outcome, input map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	model, _ := input["model"].(string)
	if model == "" {
		return nil, nil, []error{errors.New("model is required")}
	}
	if stream, _ := input["stream"].(bool); stream {
		return nil, nil, []error{errors.New("$llm.chat does not support streaming")}
	}
	if _, exists := input["extra_params"]; exists {
		return nil, nil, []error{errors.New("extra_params is not supported; use a declared model or deployment parameter")}
	}
	body := map[string]interface{}{"model": model}
	for _, field := range []string{
		"messages", "tools", "tool_choice", "response_format", "n", "temperature", "top_p", "frequency_penalty",
		"presence_penalty", "max_tokens", "max_completion_tokens", "stop", "user", "seed", "logprobs", "top_logprobs",
		"parallel_tool_calls", "reasoning_effort",
	} {
		if value, exists := input[field]; exists {
			body[field] = value
		}
	}
	if systemPrompt, _ := input["system_prompt"].(string); systemPrompt != "" {
		encoded, err := json.Marshal(body["messages"])
		if err != nil {
			return nil, nil, []error{fmt.Errorf("encode messages: %w", err)}
		}
		var messages []interface{}
		if err := json.Unmarshal(encoded, &messages); err != nil {
			return nil, nil, []error{fmt.Errorf("decode messages: %w", err)}
		}
		body["messages"] = append([]interface{}{map[string]interface{}{"role": "system", "content": systemPrompt}}, messages...)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, nil, []error{fmt.Errorf("encode LLM chat action: %w", err)}
	}
	canonical, err := openai.DecodeChatRequest(contract.ID(uuid.Must(uuid.NewV7()).String()), payload)
	if err != nil {
		return nil, nil, []error{err}
	}
	if canonical.Chat.N > 1 {
		return nil, nil, []error{errors.New("$llm.chat supports exactly one choice")}
	}
	response, err := invokeLLMAction(performer.gateway, performer.cruds, input, transaction, canonical)
	if err != nil {
		return nil, nil, []error{err}
	}
	if response.Chat == nil {
		return nil, nil, []error{errors.New("LLM gateway returned no chat response")}
	}

	responseMap := map[string]interface{}{"model": response.Model}
	if len(response.Chat.Choices) > 0 {
		choice := response.Chat.Choices[0]
		responseMap["role"] = choice.Message.Role
		responseMap["finish_reason"] = choice.FinishReason
		var content strings.Builder
		for _, part := range choice.Message.Content {
			if part.Type == "text" || part.Type == "output_text" {
				content.WriteString(part.Text)
			}
		}
		responseMap["content"] = content.String()
		if len(choice.Message.ToolCalls) > 0 {
			responseMap["tool_calls"] = choice.Message.ToolCalls
		}
	}
	responseMap["usage"] = gatewayUsageResponse(response.Usage)
	return api2go.Response{
		Res: api2go.NewApi2GoModelWithData("$llm.response", nil, 0, nil, responseMap),
	}, []actionresponse.ActionResponse{{ResponseType: outcome.Type, Attributes: responseMap}}, nil
}

func NewLLMChatPerformer(cruds map[string]*resource.DbResource, gateway *llm.Gateway) (actionresponse.ActionPerformerInterface, error) {
	if gateway == nil {
		return nil, errors.New("LLM gateway is required")
	}
	log.Infof("[$llm.chat] performer registered")
	return &llmChatActionPerformer{gateway: gateway, cruds: cruds}, nil
}

func invokeLLMAction(gateway *llm.Gateway, cruds map[string]*resource.DbResource, input map[string]interface{}, transaction *sqlx.Tx, request contract.Request) (contract.Response, error) {
	user, _ := input["requestSessionUser"].(*auth.SessionUser)
	if user == nil || user.UserId == 0 {
		return contract.Response{}, errors.New("LLM action requires an authenticated user")
	}
	ctx := context.Background()
	if httpRequest, _ := input["httpRequest"].(*http.Request); httpRequest != nil {
		ctx = httpRequest.Context()
	}
	if transaction != nil {
		if err := transaction.Commit(); err != nil {
			return contract.Response{}, err
		}
	}
	response, invokeErr := gateway.Invoke(ctx, user, request)
	if transaction != nil {
		newTransaction, err := cruds["world"].Connection().Beginx()
		if err != nil {
			return contract.Response{}, err
		}
		*transaction = *newTransaction
	}
	return response, invokeErr
}

func gatewayUsageResponse(usage contract.Usage) map[string]interface{} {
	return map[string]interface{}{
		"prompt_tokens": usage.InputTokens, "completion_tokens": usage.OutputTokens, "total_tokens": usage.TotalTokens,
		"cache_read_tokens": usage.CacheReadTokens, "cache_write_tokens": usage.CacheWriteTokens,
		"reasoning_tokens": usage.ReasoningTokens, "cost_micros": usage.CostMicros, "estimated": usage.Estimated,
	}
}
