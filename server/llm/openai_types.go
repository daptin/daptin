package llm

import jsoniter "github.com/json-iterator/go"

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// OpenAIChatRequest matches OpenAI's /v1/chat/completions request format.
type OpenAIChatRequest struct {
	Model            string                 `json:"model"`
	Messages         []OpenAIMessage        `json:"messages"`
	Stream           bool                   `json:"stream,omitempty"`
	MaxTokens        *int                   `json:"max_tokens,omitempty"`
	Temperature      *float64               `json:"temperature,omitempty"`
	TopP             *float64               `json:"top_p,omitempty"`
	N                *int                   `json:"n,omitempty"`
	Stop             interface{}            `json:"stop,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	User             string                 `json:"user,omitempty"`
	Tools            []OpenAITool           `json:"tools,omitempty"`
	ToolChoice       interface{}            `json:"tool_choice,omitempty"`
	ResponseFormat   *OpenAIResponseFormat  `json:"response_format,omitempty"`
	Seed             *int                   `json:"seed,omitempty"`
	StreamOptions    *StreamOptions         `json:"stream_options,omitempty"`
	ExtraParams      map[string]interface{} `json:"extra_params,omitempty"`
}

// OpenAIMessage represents a message in the conversation.
type OpenAIMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ContentPart represents a part of a multi-modal message content.
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL holds image data for vision requests.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// OpenAITool represents a tool definition.
type OpenAITool struct {
	Type     string             `json:"type"`
	Function OpenAIToolFunction `json:"function"`
}

// OpenAIToolFunction represents a function tool definition.
type OpenAIToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Strict      *bool       `json:"strict,omitempty"`
}

// ToolCall represents a tool call in the response.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction represents the function details of a tool call.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// OpenAIResponseFormat specifies the response format.
type OpenAIResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema interface{} `json:"json_schema,omitempty"`
}

// StreamOptions controls streaming behavior.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// OpenAIChatResponse matches OpenAI's non-streaming response format.
type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

// OpenAIChoice represents a choice in the response.
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIChatChunk is a streaming chunk in SSE format.
type OpenAIChatChunk struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []OpenAIChunkChoice `json:"choices"`
	Usage   *OpenAIUsage        `json:"usage,omitempty"`
}

// OpenAIChunkChoice represents a choice in a streaming chunk.
type OpenAIChunkChoice struct {
	Index        int            `json:"index"`
	Delta        OpenAIDelta    `json:"delta"`
	FinishReason *string        `json:"finish_reason"`
}

// OpenAIDelta represents the delta content in a streaming chunk.
type OpenAIDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// OpenAIUsage contains token usage information.
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
}

// OpenAIEmbeddingRequest matches OpenAI's /v1/embeddings request format.
type OpenAIEmbeddingRequest struct {
	Model          string      `json:"model"`
	Input          interface{} `json:"input"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
}

// OpenAIEmbeddingResponse matches OpenAI's embeddings response format.
type OpenAIEmbeddingResponse struct {
	Object string              `json:"object"`
	Data   []OpenAIEmbedding   `json:"data"`
	Model  string              `json:"model"`
	Usage  OpenAIEmbeddingUsage `json:"usage"`
}

// OpenAIEmbedding represents a single embedding result.
type OpenAIEmbedding struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// OpenAIEmbeddingUsage contains usage for embedding requests.
type OpenAIEmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// OpenAIModelList matches OpenAI's /v1/models response format.
type OpenAIModelList struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// OpenAIModel represents a model in the model list.
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}
