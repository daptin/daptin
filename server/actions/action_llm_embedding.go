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

type llmEmbeddingActionPerformer struct {
	cruds    map[string]*resource.DbResource
	provider *llm.GoAIProvider
}

func (d *llmEmbeddingActionPerformer) Name() string {
	return "$llm.embedding"
}

func (d *llmEmbeddingActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	modelName, _ := inFieldMap["model"].(string)
	if modelName == "" {
		return nil, nil, []error{fmt.Errorf("model is required")}
	}

	llmProvider, err := d.cruds["world"].ResolveLLMProviderByModel(modelName, transaction)
	if err != nil {
		log.Errorf("[$llm.embedding] failed to resolve provider for model=%s: %v", modelName, err)
		return nil, nil, []error{err}
	}

	req := llm.OpenAIEmbeddingRequest{
		Model: modelName,
		Input: inFieldMap["input"],
	}

	log.Infof("[$llm.embedding] provider=%s model=%s", llmProvider.Name, modelName)

	response, err := d.provider.Embedding(context.Background(), llmProvider, req, transaction)
	if err != nil {
		log.Errorf("[$llm.embedding] failed: provider=%s model=%s error=%v", llmProvider.Name, modelName, err)
		return nil, nil, []error{err}
	}

	responseMap := make(map[string]interface{})
	embeddings := make([]interface{}, 0, len(response.Data))
	for _, emb := range response.Data {
		embeddings = append(embeddings, emb.Embedding)
	}
	responseMap["embeddings"] = embeddings
	responseMap["model"] = response.Model
	responseMap["usage"] = map[string]interface{}{
		"prompt_tokens": response.Usage.PromptTokens,
		"total_tokens":  response.Usage.TotalTokens,
	}

	return api2go.Response{
			Res: api2go.NewApi2GoModelWithData("$llm.embedding.response", nil, 0, nil, responseMap),
		}, []actionresponse.ActionResponse{{
			ResponseType: request.Type,
			Attributes:   responseMap,
		}}, nil
}

func NewLLMEmbeddingPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, provider *llm.GoAIProvider) (actionresponse.ActionPerformerInterface, error) {
	handler := llmEmbeddingActionPerformer{
		cruds:    cruds,
		provider: provider,
	}
	log.Infof("[$llm.embedding] performer registered")
	return &handler, nil
}
