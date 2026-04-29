# LLM Providers

Daptin integrates with LLM providers using the same pattern as cloud storage: the admin configures a **provider** record linked to a **credential**, and the system handles routing, authentication, and API translation automatically.

| Cloud Storage | LLM Providers |
|---|---|
| `cloud_store` table | `llm_provider` table |
| rclone SDK | GoAI SDK (24+ providers) |
| `credential` table | `credential` table (same) |

---

## Quick Start

### 1. Create a Credential

Store your API key in the credential table (encrypted at rest):

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# OpenAI
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "openai-key",
        "content": "{\"api_key\": \"sk-...\"}"
      }
    }
  }'
```

### 2. Create an LLM Provider

```bash
curl -X POST http://localhost:6336/api/llm_provider \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "llm_provider",
      "attributes": {
        "name": "my-openai",
        "provider_type": "openai",
        "models": "gpt-4o,gpt-4o-mini,gpt-3.5-turbo",
        "credential_name": "openai-key",
        "enable": true
      }
    }
  }'
```

### 3. Link Credential via Relationship

```bash
CRED_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/credential | jq -r '.data[] | select(.attributes.name=="openai-key") | .id')

PROVIDER_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/llm_provider | jq -r '.data[] | select(.attributes.name=="my-openai") | .id')

curl -X PATCH "http://localhost:6336/api/llm_provider/$PROVIDER_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{\"data\":{\"type\":\"llm_provider\",\"id\":\"$PROVIDER_ID\",\"relationships\":{\"credential_id\":{\"data\":{\"type\":\"credential\",\"id\":\"$CRED_ID\"}}}}}"
```

### 4. Restart Server

```bash
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
```

### 5. Use It

```bash
# Drop-in OpenAI-compatible endpoint
curl http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

---

## Supported Providers

| Provider | `provider_type` | Credential Fields | Notes |
|----------|----------------|-------------------|-------|
| OpenAI | `openai` | `{"api_key": "sk-..."}` | GPT-4, GPT-4o, o1, etc. |
| Anthropic | `anthropic` | `{"api_key": "sk-ant-..."}` | Claude Opus, Sonnet, Haiku |
| Google Gemini | `gemini` | `{"api_key": "AIza..."}` | Gemini Pro, Flash, etc. |
| Ollama | `ollama` | `{}` (no auth) | Local models. Set `base_url` |
| Groq | `groq` | `{"api_key": "gsk_..."}` | Fast inference |
| Azure OpenAI | `azure` | `{"api_key": "..."}` | Set `base_url` to your endpoint |
| AWS Bedrock | `bedrock` | `{"access_key_id": "...", "secret_access_key": "...", "region": "us-east-1"}` | Claude, Titan, etc. |
| DeepSeek | `deepseek` | `{"api_key": "..."}` | DeepSeek Chat, Coder |
| Mistral | `mistral` | `{"api_key": "..."}` | Mistral Large, Medium, etc. |
| OpenAI-compatible | Any other value | `{"api_key": "..."}` | Set `base_url`. Falls back to compat mode |

---

## Credential Formats

### OpenAI
```json
{"api_key": "sk-proj-..."}
```

### Anthropic
```json
{"api_key": "sk-ant-api03-..."}
```

### Google Gemini
```json
{"api_key": "AIzaSy..."}
```

### Ollama (local, no auth)
```json
{}
```
Set `base_url` to `http://localhost:11434` on the llm_provider record.

### Azure OpenAI
```json
{"api_key": "your-azure-key"}
```
Set `base_url` to `https://your-resource.openai.azure.com`.

### AWS Bedrock
```json
{
  "access_key_id": "AKIA...",
  "secret_access_key": "...",
  "region": "us-east-1"
}
```

### Any OpenAI-compatible endpoint
```json
{"api_key": "your-key"}
```
Set `base_url` to the endpoint URL (e.g. `https://api.together.xyz/v1`).

---

## LLM Provider Table Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | varchar(100) | Yes | Unique identifier |
| `provider_type` | varchar(100) | Yes | Provider backend (openai, anthropic, etc.) |
| `base_url` | varchar(500) | No | Custom endpoint URL |
| `models` | text | Yes | Comma-separated model names for routing |
| `credential_name` | varchar(1000) | No | Name of credential record |
| `provider_parameters` | text (JSON) | No | Provider-specific defaults |
| `enable` | bool | Yes | Active/inactive toggle |

---

## OpenAI-Compatible Endpoints (Drop-in Replacement)

Daptin exposes standard OpenAI API endpoints. Any tool or SDK that works with OpenAI can point at Daptin instead.

### POST /v1/chat/completions

Standard chat completion with streaming support.

**Non-streaming:**
```bash
curl http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "system", "content": "You are helpful."},
      {"role": "user", "content": "What is 2+2?"}
    ],
    "temperature": 0.7,
    "max_tokens": 100
  }'
```

**Response:**
```json
{
  "id": "chatcmpl-daptin",
  "object": "chat.completion",
  "created": 1714400000,
  "model": "gpt-4o",
  "choices": [{
    "index": 0,
    "message": {"role": "assistant", "content": "2+2 equals 4."},
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 20,
    "completion_tokens": 8,
    "total_tokens": 28
  }
}
```

**Streaming:**
```bash
curl http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

**SSE Response:**
```
data: {"id":"chatcmpl-daptin-stream","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant"}}]}

data: {"id":"chatcmpl-daptin-stream","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"}}]}

data: {"id":"chatcmpl-daptin-stream","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":5,"total_tokens":14}}

data: [DONE]
```

### POST /v1/completions

Legacy completions endpoint. Maps `prompt` to a chat message internally.

```bash
curl http://localhost:6336/v1/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "prompt": "Once upon a time"}'
```

### POST /v1/embeddings

Generate vector embeddings.

```bash
curl http://localhost:6336/v1/embeddings \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "text-embedding-3-small",
    "input": ["Hello world", "Daptin is great"]
  }'
```

### GET /v1/models

List all models from active providers.

```bash
curl http://localhost:6336/v1/models \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "object": "list",
  "data": [
    {"id": "gpt-4o", "object": "model", "owned_by": "openai"},
    {"id": "claude-sonnet-4-20250514", "object": "model", "owned_by": "anthropic"}
  ]
}
```

---

## Using with Python OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:6336/v1",
    api_key="your-daptin-jwt-token"
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)

# Streaming
for chunk in client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Tell me a story"}],
    stream=True
):
    print(chunk.choices[0].delta.content or "", end="")
```

---

## Model Routing

When a request arrives at `/v1/chat/completions` with `"model": "gpt-4o"`, Daptin:

1. Scans all active `llm_provider` records
2. Finds the provider whose `models` field contains `gpt-4o`
3. Loads the linked credential (decrypts API key)
4. Routes the request to that provider via GoAI SDK
5. Translates the response back to OpenAI format

Multiple providers can coexist. Each handles its own set of models.

---

## Action Performers

Use LLM capabilities within Daptin action chains (YAML/JSON schemas).

### $llm.chat

Non-streaming chat completion for use in action OutFields.

```yaml
Actions:
  - Name: summarize_text
    Label: Summarize Text
    OnType: document
    InstanceOptional: true
    InFields:
      - ColumnName: text
        ColumnType: content
        IsNullable: false
    OutFields:
      - Type: $llm.chat
        Method: EXECUTE
        Reference: llm_result
        Attributes:
          model: "gpt-4o-mini"
          system_prompt: "Summarize the following text concisely."
          messages:
            - role: "user"
              content: "~text"
          max_tokens: 500
          temperature: 0.3
      - Type: client.notify
        Method: ACTIONRESPONSE
        Attributes:
          type: success
          title: Summary
          message: "!llm_result.content"
```

**Input Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | string | Yes | Model name (must match an active provider) |
| `messages` | array | Yes | `[{role, content}]` conversation |
| `system_prompt` | string | No | Convenience: prepended as system message |
| `max_tokens` | int | No | Max response tokens |
| `temperature` | float | No | Sampling temperature (0-2) |
| `top_p` | float | No | Nucleus sampling |
| `seed` | int | No | Deterministic generation |
| `stop` | string/array | No | Stop sequences |
| `tools` | array | No | Function/tool definitions |
| `tool_choice` | string/object | No | Tool selection control |
| `response_format` | object | No | JSON mode / structured output |
| `extra_params` | object | No | Provider-specific parameters |

**Output Fields (in ActionResponse):**

| Field | Description |
|-------|-------------|
| `content` | Generated text |
| `role` | `assistant` |
| `model` | Model used |
| `finish_reason` | `stop`, `length`, `tool_calls` |
| `tool_calls` | Array of tool calls (if tools used) |
| `usage` | `{prompt_tokens, completion_tokens, total_tokens}` |
| `raw` | Full response object |

### $llm.embedding

Generate embeddings for use in action chains.

```yaml
OutFields:
  - Type: $llm.embedding
    Method: EXECUTE
    Reference: embed_result
    Attributes:
      model: "text-embedding-3-small"
      input: "~text_to_embed"
```

**Output:** `embeddings` (array of float arrays), `model`, `usage`

---

## Tool Calling

```yaml
OutFields:
  - Type: $llm.chat
    Method: EXECUTE
    Attributes:
      model: "gpt-4o"
      messages:
        - role: "user"
          content: "What's the weather in Tokyo?"
      tools:
        - type: "function"
          function:
            name: "get_weather"
            description: "Get current weather for a location"
            parameters:
              type: "object"
              properties:
                location:
                  type: "string"
                  description: "City name"
              required: ["location"]
```

---

## Provider-Specific Features

Pass provider-specific options via `extra_params` or `provider_parameters`:

| Provider | Feature | How |
|----------|---------|-----|
| Anthropic | Extended thinking | `extra_params: {thinking: {type: "enabled", budget_tokens: 10000}}` |
| Anthropic | Prompt caching | Automatic via GoAI SDK |
| OpenAI | Structured output | `response_format: {type: "json_schema", json_schema: {...}}` |
| OpenAI | Reproducibility | `seed: 42` |
| Gemini | Google Search grounding | `extra_params: {grounding: {google_search: true}}` |
| Gemini | Safety settings | `extra_params: {safety_settings: [...]}` |

---

## Multiple Providers Example

Run OpenAI, Anthropic, and local Ollama side by side:

```bash
# 1. Create credentials
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"credential","attributes":{"name":"openai-key","content":"{\"api_key\":\"sk-...\"}"}}}'

curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"credential","attributes":{"name":"anthropic-key","content":"{\"api_key\":\"sk-ant-...\"}"}}}'

# 2. Create providers
curl -X POST http://localhost:6336/api/llm_provider \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"llm_provider","attributes":{"name":"openai","provider_type":"openai","models":"gpt-4o,gpt-4o-mini","credential_name":"openai-key","enable":true}}}'

curl -X POST http://localhost:6336/api/llm_provider \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"llm_provider","attributes":{"name":"anthropic","provider_type":"anthropic","models":"claude-sonnet-4-20250514,claude-haiku-4-5-20251001","credential_name":"anthropic-key","enable":true}}}'

curl -X POST http://localhost:6336/api/llm_provider \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"llm_provider","attributes":{"name":"local-ollama","provider_type":"ollama","base_url":"http://localhost:11434","models":"llama3,mistral,codellama","enable":true}}}'

# 3. Restart, then use any model:
curl http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello from Anthropic!"}]}'
```

---

## Troubleshooting

### "model 'xxx' not found in any configured provider"

- Check that an `llm_provider` with `enable: true` lists this model in its `models` field
- Models must match exactly (case-sensitive)
- Restart server after creating providers

### "failed to get credential"

- Verify `credential_name` in llm_provider matches a credential record's `name` field
- Link credential via relationship PATCH (see Quick Start step 3)

### Provider returns authentication error

- Check the credential `content` JSON has the correct `api_key` field
- Verify the API key is valid with the provider directly

### Streaming not working

- Ensure `"stream": true` is in the request body
- Check that the client supports SSE (Server-Sent Events)
- Response Content-Type must be `text/event-stream`

### Server restart required

After creating or modifying `llm_provider` or `credential` records, restart the server for changes to take effect in the Olric cache.
