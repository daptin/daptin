package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/llm"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

// RegisterLLMEndpoints registers OpenAI-compatible API endpoints on the router.
// These endpoints provide a drop-in replacement for OpenAI's API so any tool/SDK
// can point at Daptin instead.
func RegisterLLMEndpoints(router *gin.Engine, goaiProvider *llm.GoAIProvider, cruds map[string]*resource.DbResource) {
	router.POST("/v1/chat/completions", createChatCompletionHandler(goaiProvider, cruds))
	router.POST("/v1/completions", createCompletionHandler(goaiProvider, cruds))
	router.POST("/v1/embeddings", createEmbeddingHandler(goaiProvider, cruds))
	router.GET("/v1/models", createModelsHandler(cruds))
	log.Infof("[llm] registered OpenAI-compatible endpoints: /v1/chat/completions, /v1/completions, /v1/embeddings, /v1/models")
}

func createChatCompletionHandler(goaiProvider *llm.GoAIProvider, cruds map[string]*resource.DbResource) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req llm.OpenAIChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Errorf("[llm] /v1/chat/completions: invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": fmt.Sprintf("invalid request body: %v", err),
					"type":    "invalid_request_error",
				},
			})
			return
		}

		if req.Model == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "model is required",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		transaction, err := cruds["world"].Connection().Beginx()
		if err != nil {
			log.Errorf("[llm] /v1/chat/completions: failed to begin transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"message": "internal error", "type": "server_error"},
			})
			return
		}
		defer transaction.Commit()

		// Resolve provider by model name
		llmProvider, err := cruds["world"].ResolveLLMProviderByModel(req.Model, transaction)
		if err != nil {
			log.Errorf("[llm] /v1/chat/completions: no provider for model=%s: %v", req.Model, err)
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"message": fmt.Sprintf("model '%s' not found in any configured provider", req.Model),
					"type":    "invalid_request_error",
				},
			})
			return
		}

		meteringService := resource.NewMeteringService(&cruds)
		meteringDecision, err := preflightLLMMetering(c, meteringService, cruds["world"].ConfigStore, req.Model, "llm_chat", transaction)
		if err != nil {
			writeLLMMeteringError(c, err)
			return
		}

		if req.Stream {
			// Streaming response via SSE
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("X-Accel-Buffering", "no")

			log.Infof("[llm] /v1/chat/completions: streaming provider=%s model=%s", llmProvider.Name, req.Model)

			c.Stream(func(w io.Writer) bool {
				var finalUsage *llm.OpenAIUsage
				finishReason := ""
				err := goaiProvider.ChatCompletionStream(c.Request.Context(), llmProvider, req, transaction, func(chunk llm.OpenAIChatChunk) {
					if chunk.Usage != nil {
						finalUsage = chunk.Usage
					}
					if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil {
						finishReason = *chunk.Choices[0].FinishReason
					}
					chunkBytes, marshalErr := json.Marshal(chunk)
					if marshalErr != nil {
						log.Errorf("[llm] stream: failed to marshal chunk: %v", marshalErr)
						return
					}
					fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
					c.Writer.Flush()
				})
				if err != nil {
					log.Errorf("[llm] stream error: provider=%s model=%s error=%v", llmProvider.Name, req.Model, err)
					errChunk := fmt.Sprintf(`{"error": {"message": "%s", "type": "server_error"}}`, err.Error())
					fmt.Fprintf(w, "data: %s\n\n", errChunk)
					c.Writer.Flush()
				}
				if finalUsage != nil {
					recordLLMMetering(c, meteringService, meteringDecision, cruds["world"].ConfigStore, transaction, req.Model, "llm_chat", finalUsage, finishReason, http.StatusOK)
				}
				fmt.Fprintf(w, "data: [DONE]\n\n")
				c.Writer.Flush()
				return false
			})
		} else {
			// Non-streaming response
			log.Infof("[llm] /v1/chat/completions: non-streaming provider=%s model=%s", llmProvider.Name, req.Model)

			response, err := goaiProvider.ChatCompletion(c.Request.Context(), llmProvider, req, transaction)
			if err != nil {
				log.Errorf("[llm] /v1/chat/completions: failed: provider=%s model=%s error=%v", llmProvider.Name, req.Model, err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"message": err.Error(),
						"type":    "server_error",
					},
				})
				return
			}

			finishReason := ""
			if len(response.Choices) > 0 {
				finishReason = response.Choices[0].FinishReason
			}
			recordLLMMetering(c, meteringService, meteringDecision, cruds["world"].ConfigStore, transaction, req.Model, "llm_chat", response.Usage, finishReason, http.StatusOK)
			c.JSON(http.StatusOK, response)
		}
	}
}

func createCompletionHandler(goaiProvider *llm.GoAIProvider, cruds map[string]*resource.DbResource) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Legacy /v1/completions — map to chat format
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{"message": "invalid request body", "type": "invalid_request_error"},
			})
			return
		}

		model, _ := body["model"].(string)
		prompt, _ := body["prompt"].(string)

		req := llm.OpenAIChatRequest{
			Model: model,
			Messages: []llm.OpenAIMessage{
				{Role: "user", Content: prompt},
			},
		}

		// Copy optional fields
		if v, ok := body["max_tokens"]; ok {
			if f, ok := v.(float64); ok {
				i := int(f)
				req.MaxTokens = &i
			}
		}
		if v, ok := body["temperature"]; ok {
			if f, ok := v.(float64); ok {
				req.Temperature = &f
			}
		}

		transaction, err := cruds["world"].Connection().Beginx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"message": "internal error", "type": "server_error"},
			})
			return
		}
		defer transaction.Commit()

		llmProvider, err := cruds["world"].ResolveLLMProviderByModel(req.Model, transaction)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"message": fmt.Sprintf("model '%s' not found", req.Model),
					"type":    "invalid_request_error",
				},
			})
			return
		}

		meteringService := resource.NewMeteringService(&cruds)
		meteringDecision, err := preflightLLMMetering(c, meteringService, cruds["world"].ConfigStore, req.Model, "llm_completion", transaction)
		if err != nil {
			writeLLMMeteringError(c, err)
			return
		}

		response, err := goaiProvider.ChatCompletion(c.Request.Context(), llmProvider, req, transaction)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"message": err.Error(), "type": "server_error"},
			})
			return
		}

		finishReason := ""
		if len(response.Choices) > 0 {
			finishReason = response.Choices[0].FinishReason
		}
		recordLLMMetering(c, meteringService, meteringDecision, cruds["world"].ConfigStore, transaction, req.Model, "llm_completion", response.Usage, finishReason, http.StatusOK)
		c.JSON(http.StatusOK, response)
	}
}

func createEmbeddingHandler(goaiProvider *llm.GoAIProvider, cruds map[string]*resource.DbResource) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req llm.OpenAIEmbeddingRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{"message": "invalid request body", "type": "invalid_request_error"},
			})
			return
		}

		transaction, err := cruds["world"].Connection().Beginx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"message": "internal error", "type": "server_error"},
			})
			return
		}
		defer transaction.Commit()

		llmProvider, err := cruds["world"].ResolveLLMProviderByModel(req.Model, transaction)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"message": fmt.Sprintf("model '%s' not found", req.Model),
					"type":    "invalid_request_error",
				},
			})
			return
		}

		meteringService := resource.NewMeteringService(&cruds)
		meteringDecision, err := preflightLLMMetering(c, meteringService, cruds["world"].ConfigStore, req.Model, "llm_embedding", transaction)
		if err != nil {
			writeLLMMeteringError(c, err)
			return
		}

		response, err := goaiProvider.Embedding(c.Request.Context(), llmProvider, req, transaction)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"message": err.Error(), "type": "server_error"},
			})
			return
		}

		recordLLMMetering(c, meteringService, meteringDecision, cruds["world"].ConfigStore, transaction, req.Model, "llm_embedding", &llm.OpenAIUsage{
			PromptTokens: response.Usage.PromptTokens,
			TotalTokens:  response.Usage.TotalTokens,
		}, "stop", http.StatusOK)
		c.JSON(http.StatusOK, response)
	}
}

func preflightLLMMetering(c *gin.Context, service *resource.MeteringService, configStore *resource.ConfigStore, model string, requestType string, tx *sqlx.Tx) (*resource.MeteringDecision, error) {
	user, _ := c.Request.Context().Value("user").(*auth.SessionUser)
	return service.Preflight(resource.MeteringContext{
		Request:     c.Request,
		User:        user,
		Endpoint:    c.Request.URL.Path,
		Method:      c.Request.Method,
		RequestType: requestType,
		Metering:    llmMeteringConfig(configStore, tx),
		Metadata: map[string]interface{}{
			"model":        model,
			"request_type": requestType,
		},
	}, tx)
}

func recordLLMMetering(c *gin.Context, service *resource.MeteringService, decision *resource.MeteringDecision, configStore *resource.ConfigStore, tx *sqlx.Tx, model string, requestType string, usage *llm.OpenAIUsage, finishReason string, statusCode int) {
	if usage == nil {
		return
	}
	user, _ := c.Request.Context().Value("user").(*auth.SessionUser)
	metadata := map[string]interface{}{
		"model":              model,
		"request_type":       requestType,
		"prompt_tokens":      usage.PromptTokens,
		"completion_tokens":  usage.CompletionTokens,
		"total_tokens":       usage.TotalTokens,
		"reasoning_tokens":   usage.ReasoningTokens,
		"cache_read_tokens":  usage.CacheReadTokens,
		"cache_write_tokens": usage.CacheWriteTokens,
		"finish_reason":      finishReason,
	}
	err := service.Record(resource.MeteringContext{
		Request:       c.Request,
		User:          user,
		Endpoint:      c.Request.URL.Path,
		Method:        c.Request.Method,
		RequestType:   requestType,
		StatusCode:    statusCode,
		RequestBytes:  resourceRequestContentLength(c.Request.ContentLength),
		ResponseBytes: len(resource.ToJson(metadata)),
		Metering:      llmMeteringConfig(configStore, tx),
		Metadata:      metadata,
		Response: map[string]interface{}{
			"usage": metadata,
		},
	}, decision, tx)
	if err != nil {
		log.Errorf("[metering] failed to record LLM usage: %v", err)
	}
}

func llmMeteringConfig(configStore *resource.ConfigStore, tx *sqlx.Tx) *table_info.MeteringConfig {
	cfg := &table_info.MeteringConfig{
		Enabled:   true,
		CostExpr:  "response.usage.total_tokens",
		MeterType: "compute_units",
	}
	if configStore == nil || tx == nil {
		return cfg
	}
	if enabled, err := configStore.GetConfigValueFor("metering.llm.enabled", "backend", tx); err == nil && strings.EqualFold(enabled, "false") {
		cfg.Enabled = false
	}
	if costExpr, err := configStore.GetConfigValueFor("metering.llm.cost_expr", "backend", tx); err == nil && costExpr != "" {
		cfg.CostExpr = costExpr
	}
	if meterType, err := configStore.GetConfigValueFor("metering.llm.meter_type", "backend", tx); err == nil && meterType != "" {
		cfg.MeterType = meterType
	}
	if enforceMode, err := configStore.GetConfigValueFor("metering.llm.enforce_mode", "backend", tx); err == nil && enforceMode != "" {
		cfg.EnforceMode = enforceMode
	}
	if postAction, err := configStore.GetConfigValueFor("metering.llm.post_metering_action", "backend", tx); err == nil && postAction != "" {
		cfg.PostMeteringAction = postAction
	}
	return cfg
}

func resourceRequestContentLength(contentLength int64) int {
	if contentLength < 0 {
		return 0
	}
	return int(contentLength)
}

func writeLLMMeteringError(c *gin.Context, err error) {
	status := http.StatusPaymentRequired
	errorType := "insufficient_quota"
	if httpErr, ok := err.(api2go.HTTPError); ok {
		status = httpErr.Status()
		if status == http.StatusTooManyRequests {
			errorType = "rate_limit_exceeded"
		}
	}
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": err.Error(),
			"type":    errorType,
		},
	})
}

func createModelsHandler(cruds map[string]*resource.DbResource) gin.HandlerFunc {
	return func(c *gin.Context) {
		transaction, err := cruds["world"].Connection().Beginx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"message": "internal error", "type": "server_error"},
			})
			return
		}
		defer transaction.Commit()

		providers, err := cruds["world"].GetActiveLLMProviders(transaction)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"message": "failed to get providers", "type": "server_error"},
			})
			return
		}

		models := make([]llm.OpenAIModel, 0)
		for _, p := range providers {
			modelNames := strings.Split(p.Models, ",")
			for _, m := range modelNames {
				m = strings.TrimSpace(m)
				if m == "" {
					continue
				}
				models = append(models, llm.OpenAIModel{
					ID:      m,
					Object:  "model",
					Created: time.Now().Unix(),
					OwnedBy: p.ProviderType,
				})
			}
		}

		c.JSON(http.StatusOK, llm.OpenAIModelList{
			Object: "list",
			Data:   models,
		})
	}
}
