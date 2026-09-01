package actions

import (
	"errors"
	"fmt"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/llm"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/contract"
	"github.com/daptin/llmgateway/protocol/openai"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type llmEmbeddingActionPerformer struct {
	gateway *llm.Gateway
	cruds   map[string]*resource.DbResource
}

func (performer *llmEmbeddingActionPerformer) Name() string { return "$llm.embedding" }

func (performer *llmEmbeddingActionPerformer) DoAction(outcome actionresponse.Outcome, input map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	model, _ := input["model"].(string)
	if model == "" {
		return nil, nil, []error{errors.New("model is required")}
	}
	body := map[string]interface{}{"model": model, "input": input["input"]}
	for _, field := range []string{"dimensions", "encoding_format", "user"} {
		if value, exists := input[field]; exists {
			body[field] = value
		}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, nil, []error{fmt.Errorf("encode LLM embedding action: %w", err)}
	}
	canonical, err := openai.DecodeEmbeddingsRequest(contract.ID(uuid.Must(uuid.NewV7()).String()), payload)
	if err != nil {
		return nil, nil, []error{err}
	}
	response, err := invokeLLMAction(performer.gateway, performer.cruds, input, transaction, canonical)
	if err != nil {
		return nil, nil, []error{err}
	}
	if response.Embeddings == nil {
		return nil, nil, []error{errors.New("LLM gateway returned no embeddings response")}
	}
	embeddings := make([]interface{}, 0, len(response.Embeddings.Data))
	for _, embedding := range response.Embeddings.Data {
		if embedding.Base64 != "" {
			embeddings = append(embeddings, embedding.Base64)
		} else {
			embeddings = append(embeddings, embedding.Vector)
		}
	}
	responseMap := map[string]interface{}{
		"embeddings": embeddings, "model": response.Model, "usage": gatewayUsageResponse(response.Usage),
	}
	return api2go.Response{
		Res: api2go.NewApi2GoModelWithData("$llm.embedding.response", nil, 0, nil, responseMap),
	}, []actionresponse.ActionResponse{{ResponseType: outcome.Type, Attributes: responseMap}}, nil
}

func NewLLMEmbeddingPerformer(cruds map[string]*resource.DbResource, gateway *llm.Gateway) (actionresponse.ActionPerformerInterface, error) {
	if gateway == nil {
		return nil, errors.New("LLM gateway is required")
	}
	log.Infof("[$llm.embedding] performer registered")
	return &llmEmbeddingActionPerformer{gateway: gateway, cruds: cruds}, nil
}
