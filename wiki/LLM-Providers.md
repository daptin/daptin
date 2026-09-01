# LLM Gateway

Daptin exposes one OpenAI-compatible gateway backed by the reusable
`github.com/daptin/llmgateway` engine. HTTP requests and the built-in LLM
actions share the same catalog, authorization, routing, provider adapters,
reliability controls, cost calculation, and generic metering lifecycle.

Configuration uses ordinary Daptin resources and relationships:

- `llm_provider` describes one provider account and links to a `credential`;
- `llm_model` defines a public model name and its allowed capabilities;
- `llm_deployment` maps a public model to an upstream provider/model;
- `api_plan`, `api_member`, `api_usage`, and `api_quota` provide generic
  metering for LLM and non-LLM workloads.

There is no comma-separated provider model list and no provider-specific usage
ledger. Catalog changes are reloaded after resource events, with periodic
fingerprint polling as recovery; a server restart is not required.

## Supported provider adapters

The first release uses the strict OpenAI-compatible adapter.

| `provider_type` | Default base URL |
|---|---|
| `openai` | `https://api.openai.com/v1` |
| `google` | `https://generativelanguage.googleapis.com/v1beta/openai` |
| `openrouter` | `https://openrouter.ai/api/v1` |
| `lilac` | `https://api.getlilac.com/v1` |
| `openai-compatible` | none; `base_url` is required |

Each provider credential must contain a non-empty `api_key`:

```json
{"api_key":"provider-secret"}
```

HTTPS is required by default. An HTTP endpoint requires
`allow_insecure: true`; loopback, link-local, or private-network access
separately requires `allow_private_network: true`. Redirects are not followed.

The OpenAI-compatible `provider_parameters` object accepts only:

```json
{
  "organization": "optional OpenAI organization",
  "project": "optional OpenAI project",
  "image_generation_path": "/images/generations"
}
```

`image_generation_path` may be `/images/generations` or `/images`.

## Configure the gateway

The examples use JSON:API resource reference IDs. Keep provider secrets out of
shell history and source control; the inline value below is only a placeholder.

### 1. Create and link a credential

```bash
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "gateway-provider-key",
        "content": "{\"api_key\":\"provider-secret\"}"
      }
    }
  }'
```

```bash
curl -X POST http://localhost:6336/api/llm_provider \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "llm_provider",
      "attributes": {
        "name": "primary-openai",
        "provider_type": "openai",
        "provider_parameters": "{}",
        "enable": true
      },
      "relationships": {
        "credential_id": {
          "data": {"type": "credential", "id": "CREDENTIAL_REFERENCE_ID"}
        }
      }
    }
  }'
```

### 2. Create a public model

Operations are `chat`, `text_completion`, `responses`, `response_compaction`,
`embeddings`, `image_generation`, `image_edit`, `image_variation`, `moderation`, `rerank`, `audio_speech`,
`audio_transcription`, `audio_translation`, `search`, and `ocr`. Capabilities
are explicit and may include `tools`, `vision`, `audio`, `files`, `streaming`,
`json_schema`, `logprobs`, `penalties`, `parallel_tools`, `reasoning`,
`dimensions`, `token_ids`, `exact_cache`, and `public_cache`.

```bash
curl -X POST http://localhost:6336/api/llm_model \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "llm_model",
      "attributes": {
        "name": "assistant",
        "operations": "[\"chat\",\"responses\"]",
        "capabilities": "{\"tools\":true,\"vision\":true,\"json_schema\":true}",
        "routing_strategy": "priority_weighted",
        "fallback_models": "[]",
        "default_parameters": "{\"chat\":{\"max_completion_tokens\":512}}",
        "unsupported_parameter_policy": "reject",
        "enable": true
      }
    }
  }'
```

`fallback_models` is an ordered JSON array of other public model names. Cycles
or missing models reject the candidate catalog. Supported parameter policies
are:

- `reject`: reject a request that asks for a capability the model does not
  declare;
- `drop`: remove only optional non-semantic controls; semantic media or tool
  history is never silently removed;
- `passthrough`: preserve typed fields when the selected adapter supports them.

Unknown wire fields are always rejected.

### 3. Create a deployment and its relationships

```bash
curl -X POST http://localhost:6336/api/llm_deployment \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "llm_deployment",
      "attributes": {
        "name": "assistant-primary",
        "upstream_model": "gpt-4o-mini",
        "operations": "[\"chat\",\"responses\"]",
        "priority": 0,
        "weight": 1,
        "request_timeout_ms": 90000,
        "connect_timeout_ms": 10000,
        "max_concurrency": 100,
        "rpm": -1,
        "tpm": -1,
        "pricing": "{\"input_tokens\":150000,\"output_tokens\":600000}",
        "parameters": "{}",
        "health_check": "{\"enabled\":true,\"interval_ms\":30000,\"timeout_ms\":5000,\"failure_threshold\":3}",
        "enable": true
      },
      "relationships": {
        "llm_model_id": {
          "data": {"type": "llm_model", "id": "MODEL_REFERENCE_ID"}
        },
        "llm_provider_id": {
          "data": {"type": "llm_provider", "id": "PROVIDER_REFERENCE_ID"}
        }
      }
    }
  }'
```

Deployments in the lowest eligible priority tier are selected by positive
weight without replacement. Retryable failures may move to another deployment
or an ordered fallback model, within the engine's bounded attempt and request
deadlines. Disabled, unhealthy, saturated, rate-protected, or
capability-incompatible deployments are excluded. A streaming request may
switch only before its first client-visible event.

Pricing is a single map from a usage measure to its fixed-point cost in micros
per one million units. Canonical token keys are `input_tokens`,
`output_tokens`, `cache_read_tokens`, `cache_write_tokens`, and
`reasoning_tokens`. Providers may also report named measures such as
`search_units`, `ocr_pages`, or `document_bytes`; those use the same map and
cost calculator.

## HTTP API

The implemented OpenAI-compatible routes are:

- `POST /v1/chat/completions`
- `POST /v1/completions`
- `POST /v1/messages`
- `POST /v1/responses` (stateless; `store` and `previous_response_id` are
  rejected)
- `POST /v1/responses/compact` (explicit input only;
  `previous_response_id` is rejected)
- `POST /v1/embeddings`
- `POST /v1/images/generations`
- `POST /v1/images/edits`
- `POST /v1/images/variations`
- `POST /v1/moderations`
- `POST /rerank`, `POST /v1/rerank`, and `POST /v2/rerank`
- `POST /v1/audio/speech`
- `POST /v1/audio/transcriptions`
- `POST /v1/audio/translations`
- `POST /v1/search` and `POST /v1/search/{tool}`
- `POST /v1/ocr` and `POST /ocr`
- file create/list/read/content/delete under `/v1/files`
- batch create/list/read/cancel under `/v1/batches`
- `GET /v1/models`
- `GET /v1/models/{id}`

Streaming chat, completion, Messages, and Responses requests emit SSE
keepalives while setup or upstream generation is idle. Responses events are
translated through typed contracts; unknown event names, conflicting event
types, missing coordinates, and non-increasing sequence numbers are rejected.
Provider-reported cached and reasoning token details are retained in compatible
chat/completion and Responses usage objects and in Daptin's generic metering
facts.

Text completions have a native canonical operation and are never converted to
chat. Files and batches are durable Daptin resources; their persistence,
permissions, relationships, and assets use the ordinary resource lifecycle.
The existing `document.document_content` asset column must be bound to a
configured Daptin cloud store. File creation fails with `service_unavailable`
before writing either resource when that canonical asset binding is absent.

All routes use Daptin authentication and model resource permissions. Model
listing hides models the authenticated principal cannot read.

```bash
curl http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "assistant",
    "messages": [{"role":"user","content":"Hello"}],
    "stream": false
  }'
```

OpenAI SDKs can use `http://localhost:6336/v1` as their base URL and a Daptin
bearer token as their API key.

## Daptin actions

`$llm.chat` and `$llm.embedding` use the same engine as HTTP. They apply the
same strict request decoder, model authorization, route plan, provider call,
costing, and generic metering. `$llm.chat` is non-streaming in action chains.

```yaml
OutFields:
  - Type: $llm.chat
    Method: EXECUTE
    Reference: llm_result
    Attributes:
      model: assistant
      messages:
        - role: user
          content: "~prompt"
      max_completion_tokens: 256
      temperature: 0.2
```

The chat result includes `content`, `role`, `model`, `finish_reason`, optional
`tool_calls`, and `usage`. The embedding result includes `embeddings`, `model`,
and `usage`. `usage` includes input/output/total/cache/reasoning tokens,
`cost_micros`, and whether usage is estimated. Arbitrary `extra_params` are
rejected; configure operation-scoped deployment parameters instead.

## Metering and cost

LLM metering composes with Daptin's generic metering resources; it is not an
LLM-owned quota system. Admission reserves named measures before provider I/O,
and completion/cancellation terminalizes the same `api_usage` record afterward.
No database transaction remains open during provider inference or streaming.

The emitted measures include `input_tokens`, `output_tokens`,
`cache_read_tokens`, `cache_write_tokens`, `reasoning_tokens`, `total_tokens`,
and `cost_micros`, plus operation-specific named measures. The same `api_plan`
limits can be applied to any other metered Daptin resource using its own named
measures.

LLM invocation uses the `llm_model` resource's ordinary `TableInfo.Metering`
configuration under the `invoke` action. The standard table enables request
metering there. Custom cost expressions, meter types, and post-metering actions
are configured through that same resource metering definition; there is no
separate LLM metering configuration path.

Hard limits use the database-backed generic quota state and fail closed when
that authority is unavailable. Olric counters protect deployments
(`max_concurrency`, RPM, TPM) and do not replace durable customer quotas.

## Health, reload, and shutdown

- `/llm/healthz` reports process liveness.
- `/llm/readyz` requires a valid active catalog and reports rejected reloads.
- Resource events trigger a debounced reload; a periodic content fingerprint
  recovers missed events and observes credential version changes.
- Invalid candidates never replace the last valid immutable snapshot.
- Shutdown stops new admission, waits for bounded in-flight requests, then
  closes adapter transports before Daptin closes shared database/Olric state.

## Troubleshooting

- **Model not found:** verify the public `llm_model.name`, its `enable` flag,
  operation list, model permission, and at least one enabled related deployment.
- **No healthy deployment:** verify provider/deployment enable flags,
  capabilities, operation lists, timeouts, health state, and protection limits.
- **Authentication error:** verify the provider's credential relationship and
  that decrypted credential content contains a non-empty `api_key`.
- **Catalog reload rejected:** inspect `/llm/readyz` and logs for the stable
  failure stage; the previous valid catalog remains active.
- **Quota denied:** inspect the user's active `api_member`, related `api_plan`,
  and the generic `api_quota` bucket for the named metric/window.
