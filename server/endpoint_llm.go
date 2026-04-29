package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/daptin/daptin/server/llm"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
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

		if req.Stream {
			// Streaming response via SSE
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("X-Accel-Buffering", "no")

			log.Infof("[llm] /v1/chat/completions: streaming provider=%s model=%s", llmProvider.Name, req.Model)

			c.Stream(func(w io.Writer) bool {
				err := goaiProvider.ChatCompletionStream(c.Request.Context(), llmProvider, req, transaction, func(chunk llm.OpenAIChatChunk) {
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

		response, err := goaiProvider.ChatCompletion(c.Request.Context(), llmProvider, req, transaction)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"message": err.Error(), "type": "server_error"},
			})
			return
		}

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

		response, err := goaiProvider.Embedding(c.Request.Context(), llmProvider, req, transaction)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"message": err.Error(), "type": "server_error"},
			})
			return
		}

		c.JSON(http.StatusOK, response)
	}
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
