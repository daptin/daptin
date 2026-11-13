# LLM Gateway

> **Configuration:** The gateway uses `llm_provider`, `llm_model`, and
> `llm_deployment` resources;
> provider rows alone do not expose models.
> Responses file inputs can be enabled with the `files` model capability in
> `llm_model`.

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
ledger. Catalog changes trigger debounced reloads through resource events. As recovery
for missed events, each instance reconciles roughly every 30 seconds, with jitter
(27–33 seconds). Unchanged checks read only active resource IDs and versions,
plus versions of referenced credentials. The full catalog reloads only when
those change. A missed event can delay a configuration change until the next
reconciliation; a server restart is not required.

## Supported provider adapters

The gateway uses the strict OpenAI-compatible adapter.

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

### Provider operation support

An adapter implements the request format; the selected upstream endpoint must
also support the operation. Declaring an operation in `llm_model.operations`
and `llm_deployment.operations` permits routing to it, but does not add that
operation to the provider. The adapter forwards chat to
`{base_url}/chat/completions` and Responses to `{base_url}/responses`; it does
not translate Responses into Chat Completions or Google's native API.

The following non-streaming calls were verified on November 12, 2026:

| Provider | `upstream_model` | Chat Completions | Responses |
|---|---|---|---|
| `google` | `gemini-2.5-flash` | Works | Unsupported at the tested default endpoint (404) |
| `google` | `gemini-2.5-flash-lite` | Works | Unsupported at the tested default endpoint (404) |
| `openrouter` | `google/gemini-2.5-flash` | Works | Works |
| `openrouter` | `openai/gpt-4o-mini` | Works | Works |

For the tested direct Google setup, set both operation lists to `["chat"]`.
Google's [OpenAI compatibility guide](https://aistudio.google.com/docs/openai)
documents Chat Completions at that base URL. For Gemini through Responses,
the verified route uses `provider_type: "openrouter"`,
`upstream_model: "google/gemini-2.5-flash"`, and operation lists containing
`"responses"`. Verify additional operations against the selected provider
before enabling them.

### Local Ollama through the OpenAI-compatible adapter

Ollama's OpenAI-compatible API was exercised with non-streaming chat, SSE
streaming, and model listing. Configure it as `openai-compatible`; it is not a
separate provider type:

```json
{
  "name": "local-ollama",
  "provider_type": "openai-compatible",
  "base_url": "http://127.0.0.1:11434/v1",
  "provider_parameters": "{}",
  "allow_insecure": true,
  "allow_private_network": true,
  "enable": true
}
```

Link a credential containing a non-empty placeholder API key, for example
`{"api_key":"ollama"}`, then create the normal `llm_model` and
`llm_deployment` relationships described below. Set the deployment's upstream
model name to one already pulled by Ollama.

> **Compatibility caveat:** OpenAI-compatible does not mean every request
> field has identical semantics. In the audit, `max_tokens: 32` was
> accepted but Ollama reported 124 output tokens. Treat adapter fields not
> explicitly verified by the upstream as unsupported/ignored, and test limits
> using the returned usage before relying on them for cost or safety. Durable
> Daptin metering remains separate from provider-side generation controls.

```bash
curl http://localhost:6336/v1/chat/completions \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/json' \
  --data-binary '{"model":"assistant","messages":[{"role":"user","content":"Reply briefly"}],"max_tokens":32,"stream":false}'
```

## Configure the gateway

Start with a running Daptin instance and a signed-in administrator. For Docker,
persistent storage, account creation, and claiming the first administrator, follow
[[Getting-Started-Guide]] and [[Installation]]. The examples below work with
`v0.13.46` and later and use `http://localhost:6336`.

Use ordinary `curl` commands below. Replace `YOUR_DAPTIN_TOKEN` with the
administrator's Daptin JWT. Copy each creation response's `data.id` into the
next command's ID placeholder; these are Daptin reference IDs. Your provider
API key is used only when creating the credential.

The walkthrough creates one public model, `assistant`, with GPT-4o mini as its
primary upstream and Gemini 2.5 Flash as its backup, both through OpenRouter.
Daptin chooses the deployment; your application always requests `assistant`.

| Resource | Purpose in this walkthrough |
|---|---|
| `llm_provider` | The OpenRouter account and its credential |
| `llm_model` | The public name `assistant`, allowed operations, capabilities, and execute permission |
| `llm_deployment` | One route from `assistant` to a particular upstream model, with priority and operational limits |

### 1. Create and link a credential

```bash
curl http://localhost:6336/api/credential \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "openrouter-key",
        "content": "{\"api_key\":\"YOUR_OPENROUTER_KEY\"}"
      }
    }
  }'
```

Copy `data.id` as `CREDENTIAL_ID`. `content` contains a JSON string, so the
escaped quotes in that one field are required.

```bash
curl http://localhost:6336/api/llm_provider \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_provider",
      "attributes": {
        "name": "openrouter",
        "provider_type": "openrouter",
        "credential_id": "CREDENTIAL_ID",
        "provider_parameters": "{}",
        "enable": true
      }
    }
  }'
```

Copy `data.id` as `PROVIDER_ID`. **Include `provider_parameters`: use `"{}"`
when no provider-specific settings are needed.** Omitting this required field
causes a 422 response on `v0.13.46`.

### 2. Create the public model and choose its access policy

```bash
curl http://localhost:6336/api/llm_model \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_model",
      "attributes": {
        "name": "assistant",
        "operations": "[\"chat\",\"responses\"]",
        "capabilities": "{}",
        "routing_strategy": "priority_weighted",
        "enable": true
      }
    }
  }'
```

Copy `data.id` as `MODEL_ID`. Clients send `"model": "assistant"`, not this ID
or the upstream model name. **Apply the access policy with a separate PATCH before
creating any deployment:**

```bash
curl -X PATCH http://localhost:6336/api/llm_model/MODEL_ID \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{"data":{"type":"llm_model","id":"MODEL_ID","attributes":{"permission":0}}}'
```

Resource creation uses the table's default record permission. Supplying
`"permission": 0` in the POST does not override that default on `v0.13.46`.
The PATCH is the step that applies the restriction; check its response's
`data.attributes.permission` is `0`.

**After this PATCH, invocation is restricted to administrators.** `permission: 0`
grants no row-level execution rights to the owner, ordinary signed-in users, or
guests; Daptin's administrator-group rules still allow administrators. Creating
an account or knowing the model name does not grant access. Later in this guide,
[grant a usergroup access](#5-allow-an-application-user-to-invoke-the-model) before
using an ordinary application user's token. Existing explicit group grants are
separate permissions; changing the row permission does not revoke those grants.

Start with text requests. `capabilities: "{}"` does not enable images, files, or
tools. The [media section](#images-files-and-audio) explains how to enable them
when your upstreams support them. Supported operations and capabilities are
listed in [Operations and capabilities](#operations-and-capabilities).

### 3. Create the primary deployment

```bash
curl http://localhost:6336/api/llm_deployment \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_deployment",
      "attributes": {
        "name": "assistant-primary",
        "llm_model_id": "MODEL_ID",
        "llm_provider_id": "PROVIDER_ID",
        "upstream_model": "openai/gpt-4o-mini",
        "operations": "[\"chat\",\"responses\"]",
        "priority": 0,
        "weight": 1,
        "enable": true
      }
    }
  }'
```

Copy `data.id` as `PRIMARY_DEPLOYMENT_ID`. A deployment belongs to one public
model and one provider account. To use a different provider account, create its
credential and provider first, then reference that provider ID in the deployment.
The upstream model must support each declared operation.

Catalog changes reload automatically. Allow a few seconds after a change and
check model discovery. See [Health, reload, and shutdown](#health-reload-and-shutdown)
for missed-event recovery and rejected reloads.

### 4. Verify a non-streaming request as administrator

```bash
curl http://localhost:6336/v1/models \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN'
```

The response should list `assistant` for your administrator token.

```bash
curl http://localhost:6336/v1/chat/completions \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "messages": [{"role": "user", "content": "Reply with a short greeting."}],
    "max_tokens": 128,
    "stream": false
  }'
```

Expect one JSON response containing `choices` and `usage`. No response ID from
Chat Completions can be used as a Daptin `previous_response_id`. To continue a
Chat conversation, send the earlier messages in the next `messages` array.
For Daptin-owned history, use [Stored Responses](#stored-responses-and-conversation-history).

Before granting anyone access, verify that the configured model rejects guest
requests:

```bash
curl http://localhost:6336/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"assistant","messages":[{"role":"user","content":"Hello"}]}'
```

Expect HTTP 403. Repeat with `-H 'Authorization: Bearer ooga'`; a made-up token
must not give access to this restricted model either. On `v0.13.46`, an invalid
bearer token can be evaluated as a guest, so an open model can still answer it.
That does not mean the token authenticated successfully. If either request
succeeds, verify that the separate permission PATCH in step 2 was applied and
inspect any existing group grants. The model creation POST alone does not apply
the administrator-only policy.

<a id="grant-a-usergroup-permission-to-invoke-the-model"></a>

### 5. Allow an application user to invoke the model

Use the administrator token to create the user's account, if it does not
already exist:

```bash
curl http://localhost:6336/action/user_account/signup \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "attributes": {
      "name": "Router application",
      "email": "app@example.com",
      "password": "YOUR_APP_PASSWORD",
      "passwordConfirm": "YOUR_APP_PASSWORD"
    }
  }'
```

As administrator, list accounts and copy the `data.id` of `app@example.com` as
`USER_ID`:

```bash
curl http://localhost:6336/api/user_account \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN'
```

Create the access group and copy its `data.id` as `GROUP_ID`:

```bash
curl http://localhost:6336/api/usergroup \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{"data":{"type":"usergroup","attributes":{"name":"assistant-users"}}}'
```

Add the user to the group:

```bash
curl http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "user_account_user_account_id_has_usergroup_usergroup_id",
      "attributes": {"user_account_id": "USER_ID", "usergroup_id": "GROUP_ID"}
    }
  }'
```

Relate the model to the group:

```bash
curl http://localhost:6336/api/llm_model_llm_model_id_has_usergroup_usergroup_id \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_model_llm_model_id_has_usergroup_usergroup_id",
      "attributes": {"llm_model_id": "MODEL_ID", "usergroup_id": "GROUP_ID"}
    }
  }'
```

Copy this relationship's `data.id` as `MODEL_GROUP_RELATION_ID`, then grant
`GroupExecute` on that relationship:

```bash
curl -X PATCH http://localhost:6336/api/llm_model_llm_model_id_has_usergroup_usergroup_id/MODEL_GROUP_RELATION_ID \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_model_llm_model_id_has_usergroup_usergroup_id",
      "id": "MODEL_GROUP_RELATION_ID",
      "attributes": {"permission": 524288}
    }
  }'
```

`524288` is `GroupExecute`. Keep the model row's `permission` at `0`; the
persisted group relationship is the access grant. To revoke this grant, PATCH
this same relationship's permission to `0`. Other explicit grants, if present,
remain independent.

Sign in as the application user:

```bash
curl http://localhost:6336/action/user_account/signin \
  -H 'Content-Type: application/json' \
  -d '{"attributes":{"email":"app@example.com","password":"YOUR_APP_PASSWORD"}}'
```

Copy the `client.store.set` response item whose `Attributes.key` is `token`;
its `Attributes.value` is `APP_TOKEN`. Use that token for discovery and inference:

```bash
curl http://localhost:6336/v1/models \
  -H 'Authorization: Bearer APP_TOKEN'

curl http://localhost:6336/v1/chat/completions \
  -H 'Authorization: Bearer APP_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"model":"assistant","messages":[{"role":"user","content":"Hello"}],"stream":false}'
```

After the grant, discovery lists `assistant` and inference is allowed. Without a
grant, discovery hides the model and inference is denied. An action wrapping
`$llm.chat` has its own execute permission, independent of the model grant.
Application users do not need the provider key or access to credential records.
Their usage and saved Responses belong to their own active Daptin account.

### 6. Add Gemini as a fallback and understand when it is used

```bash
curl http://localhost:6336/api/llm_deployment \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_deployment",
      "attributes": {
        "name": "assistant-fallback",
        "llm_model_id": "MODEL_ID",
        "llm_provider_id": "PROVIDER_ID",
        "upstream_model": "google/gemini-2.5-flash",
        "operations": "[\"chat\",\"responses\"]",
        "priority": 1,
        "weight": 1,
        "enable": true
      }
    }
  }'
```

Copy `data.id` as `FALLBACK_DEPLOYMENT_ID`. Your application continues to send
`"model": "assistant"`; it does not choose a deployment ID.

| Situation | What Daptin does |
|---|---|
| Primary is eligible and succeeds | Returns the GPT result; Gemini is not called |
| Primary is disabled, unhealthy, or excluded by a deployment protection limit | Can use the eligible Gemini deployment |
| Primary encounters a retryable failure | Can try Gemini within the attempt and request deadlines |
| Request fails Daptin model permission, validation, or quota admission | Rejects it before inference; a fallback does not bypass that decision |
| Upstream rejects with 400/401/402/403/404/422 | Returns the non-retryable error; adding Gemini does not repair the request or credentials |
| A stream has already emitted its first client-visible event | Keeps that deployment; a later failure ends the stream rather than mixing upstreams |

Retryable upstream errors include 408, 409, 429, and 5xx, plus connection failures
and timeouts. Another attempt still needs an eligible route and remaining time;
fallback is not a promise that every request succeeds. The default engine budget
is at most three deployment attempts across the route plan.

Priorities are ordered tiers: `0` is tried before `1`. Weights choose the order
among deployments in the same priority tier. Giving both deployments priority
`0` would make them weighted alternatives, rather than a primary and a standby.
This example uses one provider account, so an OpenRouter account-wide problem
can affect both routes. Separate provider accounts require separate provider
rows and credentials.

#### Test the backup without waiting for an outage

As administrator, temporarily disable the primary:

```bash
curl -X PATCH http://localhost:6336/api/llm_deployment/PRIMARY_DEPLOYMENT_ID \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_deployment",
      "id": "PRIMARY_DEPLOYMENT_ID",
      "attributes": {"enable": false}
    }
  }'
```

After the catalog reloads, repeat the same chat request using `APP_TOKEN` and
`"model": "assistant"`. Gemini should handle it. The response's public model
name remains `assistant`; inspect the usage record to identify the actual route:

```bash
curl 'http://localhost:6336/api/api_usage?page[size]=10&sort=-created_at' \
  -g -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN'
```

In the relevant row's `attributes.metadata.attempts`, compare `DeploymentID`
with `FALLBACK_DEPLOYMENT_ID`. `Outcome` and `HTTPStatus` describe each attempt.
A request rejected before admission has no new admitted usage row.

Restore the primary by repeating the PATCH with `"enable": true`. Wait for the
catalog reload before expecting GPT to be preferred again. Disabling a route
tests eligibility switching; it does not simulate an upstream retryable error.

### Optional: allow guest invocation

To make `assistant` callable without a signed-in account, explicitly grant
`GuestExecute` (`32`) on the model row:

```bash
curl -X PATCH http://localhost:6336/api/llm_model/MODEL_ID \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{"data":{"type":"llm_model","id":"MODEL_ID","attributes":{"permission":32}}}'

curl http://localhost:6336/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"assistant","messages":[{"role":"user","content":"Hello"}]}'
```

Guest calls consume provider work and are metered to Daptin's persisted guest
account. Stored Responses still require a signed-in account. Revert the row's
permission to `0` to remove this row-level guest grant; existing usergroup grants
remain separate. See [[Permissions]] for combined permission bitsets.

## Streaming and non-streaming requests

Use `APP_TOKEN` below after granting the application user access. Administrator
tokens also work. The request's `stream` flag controls delivery; it does not
change model permissions or make conversation history persistent.

| Request | Delivery | Conversation handling |
|---|---|---|
| Chat with `stream: false` | One JSON response | Send previous messages yourself on the next call |
| Chat with `stream: true` | Server-sent events (SSE) | Same caller-managed messages |
| Responses with `store: true` | One JSON response by default | Continue with the returned response `id` |
| Responses with both `stream: true` and `store: true` | SSE events | Continue with the saved response `id` once the turn completes |

For a streaming chat request, `curl -N` displays events as they arrive:

```bash
curl -N http://localhost:6336/v1/chat/completions \
  -H 'Authorization: Bearer APP_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "messages": [{"role": "user", "content": "Write a short greeting."}],
    "max_tokens": 128,
    "stream": true
  }'
```

Expect `data:` events and a final `[DONE]` marker. Non-streaming chat places the
finished text in `choices[].message.content`; streaming chat sends incremental
content in `choices[].delta`. Requests use the same routing policy, but fallback
can occur only before the first client-visible event. If a stream fails after
that point, handle the error in the client; do not treat partial output as a
completed saved turn.

Use [Stored Responses](#stored-responses-and-conversation-history) for saving,
continuation, retrieval, deletion, and streaming with retained history.

## Images, files, and audio

Media support requires both Daptin configuration and upstream support. Declaring
a capability permits the request; it does not teach an upstream model to process
that media. Daptin checks the public model and adapter capabilities, but does not
discover every upstream model's abilities. Every route you intend to use for that
request must support its input and output formats, including backup routes.

| Feature | Operation in both model and deployment | Additional public-model capability |
|---|---|---|
| Image input in chat or Responses | `chat` or `responses` | `vision` |
| File input in Responses | `responses` | `files` |
| Inline audio input in chat | `chat` | `audio` |
| Speech-to-text file upload | `audio_transcription` | No extra capability required for that dedicated operation |
| Text-to-speech | `audio_speech` | No extra capability required for that dedicated operation |
| Image generation | `image_generation` | No extra capability required for that dedicated operation |

### Image and file inputs

After verifying that the upstream routes support them, enable `vision` and
`files` on `assistant` as administrator:

```bash
curl -X PATCH http://localhost:6336/api/llm_model/MODEL_ID \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_model",
      "id": "MODEL_ID",
      "attributes": {"capabilities": "{\"vision\":true,\"files\":true}"}
    }
  }'
```

This replaces the capability object; include any other capabilities you already
use. It leaves the model permission unchanged.

Send an image using a publicly reachable image URL, replacing the example URL
with your actual image:

```bash
curl http://localhost:6336/v1/chat/completions \
  -H 'Authorization: Bearer APP_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "Describe this image."},
        {"type": "image_url", "image_url": {"url": "https://YOUR_PUBLIC_HOST/photo.jpg"}}
      ]
    }],
    "max_tokens": 128,
    "stream": false
  }'
```

The image URL is fetched by the upstream provider. A local path such as
`/home/me/photo.jpg` is not an image URL. A `data:image/png;base64,...` value is
another supported request form when the provider accepts it. For image input in
Responses, use a content part with `"type": "input_image"` and `"image_url"` set
to the URL string. Neither image input nor file input implies image generation.

For inline files, file URLs, and Daptin file IDs, continue with
[Responses files and web search](#responses-files-and-web-search). In particular,
Daptin file IDs require an asset-backed file resource and read permission; a model
execute grant does not grant file access. File formats and media streaming
support depend on the selected upstream.

### Dedicated audio models

The GPT/Gemini text deployments above do not automatically become speech
services. Repeat the model and deployment creation steps with upstream models
that support the dedicated operation:

| Example public name | `operations` JSON string on model and deployment |
|---|---|
| `transcriber` | `["audio_transcription"]` |
| `speaker` | `["audio_speech"]` |

Grant each new public model its own execute permission. Once `transcriber` is
configured and granted to the application user, upload a real audio file:

```bash
curl http://localhost:6336/v1/audio/transcriptions \
  -H 'Authorization: Bearer APP_TOKEN' \
  -F 'model=transcriber' \
  -F 'file=@recording.wav' \
  -F 'response_format=json'
```

Do not set a JSON Content-Type on this multipart upload. Once `speaker` is
configured, request audio using a voice supported by its upstream provider:

```bash
curl http://localhost:6336/v1/audio/speech \
  -H 'Authorization: Bearer APP_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"model":"speaker","input":"Hello from Daptin.","voice":"YOUR_UPSTREAM_VOICE","response_format":"mp3"}' \
  -o greeting.mp3
```

These audio examples describe request formats after configuring compatible
upstreams; they do not claim that the OpenRouter GPT/Gemini routes in this
walkthrough implement those audio operations.

## Operations and capabilities

Operations are `chat`, `text_completion`, `responses`, `response_compaction`,
`embeddings`, `image_generation`, `image_edit`, `image_variation`, `moderation`,
`rerank`, `audio_speech`, `audio_transcription`, `audio_translation`, `search`,
and `ocr`. Capabilities may include `tools`, `vision`, `audio`, `files`,
`json_schema`, `logprobs`, `penalties`, `parallel_tools`, `reasoning`,
`dimensions`, `token_ids`, `exact_cache`, and `public_cache`.

`fallback_models` is an ordered JSON array of other public model names. It is
separate from adding a second deployment to the same public model. Cycles or
missing models reject the candidate catalog. Supported parameter policies are:

- `reject`: reject a request that asks for a capability the model does not declare;
- `drop`: remove only optional non-semantic controls; semantic media or tool history is never silently removed;
- `passthrough`: preserve typed fields when the selected adapter supports them.

Unknown wire fields are always rejected. Deployment `pricing` maps usage measures
to fixed-point cost in micros per one million units, for example `input_tokens`,
`output_tokens`, `cache_read_tokens`, `cache_write_tokens`, and `reasoning_tokens`.
Empty pricing records quantities without adding configured token cost. See
[[API-Metering]] for quota and billing setup.

## HTTP API

The implemented OpenAI-compatible routes are:

- `POST /v1/chat/completions`
- `POST /v1/completions`
- `POST /v1/messages`
- `POST /v1/responses` (stateless or stored conversation history;
  `background: true` is rejected)
- `GET /v1/responses/{id}`
- `DELETE /v1/responses/{id}`
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
Their CRUD permissions are independent of `llm_model` execute permission.
The existing `document.document_content` asset column must be bound to a
configured Daptin cloud store. File creation fails with `service_unavailable`
before writing either resource when that canonical asset binding is absent.
Responses `input_file` content accepts exactly one of inline `file_data`, an
HTTP(S) `file_url`, or a Daptin `/v1/files` `file_id`. A Daptin file ID is read
through the same permissioned file resource and normalized to inline content
before model authorization, routing, metering, and provider invocation.

### Stored Responses and conversation history

Set `store: true` to retain the turn’s existing `api_usage` capture for conversation history. No separate response payload resource is created.
Daptin returns that resource's reference ID as the response `id`. Send it as
`previous_response_id` on the next turn; Daptin reads the saved input and output,
appends the new input, and invokes the shared gateway with the complete history.
Stored history survives restarts and can be routed to another eligible provider.
Providers still receive `store: false`, without `previous_response_id`: Daptin
owns this conversation state, independently of provider-side storage.

Create a stored turn with the application user's token:

```bash
curl http://localhost:6336/v1/responses \
  -H 'Authorization: Bearer APP_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "input": "Remember that my favorite color is blue.",
    "store": true,
    "stream": false,
    "max_output_tokens": 128
  }'
```

Copy the top-level `id` from this response as `RESPONSE_ID`, then continue:

```bash
curl http://localhost:6336/v1/responses \
  -H 'Authorization: Bearer APP_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "previous_response_id": "RESPONSE_ID",
    "input": "What is my favorite color?",
    "store": true,
    "max_output_tokens": 128
  }'
```

Expect the next response to recall blue. For another turn, use the newest
completed response's `id`. Each saved turn already contains the complete replayed
context; do not also resend the earlier input when using `previous_response_id`.

To verify cross-deployment continuation, save the first turn while the primary
is enabled, disable the primary with the [fallback test](#test-the-backup-without-waiting-for-an-outage),
and send the continuation after the catalog reloads. Gemini receives Daptin's
saved context. Inspect `api_usage` to confirm the deployment used, then restore
the primary.

To stream a saved continuation, add `stream: true` and use `curl -N`:

```bash
curl -N http://localhost:6336/v1/responses \
  -H 'Authorization: Bearer APP_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "previous_response_id": "RESPONSE_ID",
    "input": "Write a short poem about that color.",
    "store": true,
    "stream": true,
    "max_output_tokens": 256
  }'
```

Use the latest undeleted completed response ID for this request. Responses streaming
uses named events such as `response.created`, `response.output_text.delta`, and
`response.completed`, rather than Chat Completions' `choices[].delta` format.
Copy the completed response's `id` from its event for the next continuation.
An incomplete terminal response may also be continued; a failed, cancelled, or
still-running response cannot.

Retrieve or delete a saved turn as its owner:

```bash
curl http://localhost:6336/v1/responses/RESPONSE_ID \
  -H 'Authorization: Bearer APP_TOKEN'

curl -X DELETE http://localhost:6336/v1/responses/RESPONSE_ID \
  -H 'Authorization: Bearer APP_TOKEN'
```

Retrieval returns the saved response. After deletion, retrieval and continuation
using that ID fail; the accounting facts remain. Deleting a parent does not
delete a descendant's already-captured context.


Stored responses require a signed-in account and belong to the active account.
Rows are private to their owner by default; administrators and explicitly
permissioned usergroups use the ordinary resource permissions. Reading, referring to, and deleting retained payloads use the usage row’s respective resource permissions. Usage facts remain backend managed. A continuation independently requires read access to the saved
response and execute permission on the requested public model. Denial happens
before provider invocation. Retrieval and deletion do not require model execute
permission.

Each saved row contains the complete input used for that turn and its output,
including tool calls, reasoning items, and resolved file content. Subsequent
turns recheck the selected model's capabilities against that history and reserve
usage for the replayed context through generic metering. The selected provider
must support the replayed item types. Top-level `instructions` and other request
controls belong to the current turn; Daptin does not copy them from an earlier
request.

Omitting `store` or setting it to false leaves the new response unstored. A
request may read a `previous_response_id` with `store: false`, but its new result
cannot be used as a stored response ID. Use `store: true` on each turn to retain
an ongoing chain. In-progress, failed, cancelled, missing, and inaccessible
responses cannot be continued. Streaming stored responses use the same Daptin
reference ID throughout their events and are saved before the terminal event is
returned.

Stored snapshots and replayed input are limited to 16 MiB each. Oversized
history is rejected before provider invocation. A response whose output makes
the stored snapshot exceed the limit fails storage after inference; admitted
provider work is still metered. History remains in the usage record until its captured payload is deleted. `DELETE /v1/responses/{id}` clears retained request and response bodies and prevents continuation; it preserves usage quantities, quota accounting, and terminal status. Descendant turns retain their own complete context.

The `llm_model` backend `invoke` configuration enables payload capture up to 16 MiB by default. Capture can remain enabled with metering disabled. Stateful storage requires that full-history capture policy; disabling capture or lowering its bound rejects `store: true` before inference. Ordinary non-retained invocations may still be captured according to policy, but their IDs cannot be used for continuation. See [[API-Metering]] for capture policy and resource access.

### Responses files and web search

Declare `files` on the public model before accepting any `input_file`. The
capability check is shared by direct HTTP, actions, and other callers of the
gateway engine. A request that uses files against a model without that
capability is denied before provider invocation.

This example sends inline content:

```bash
curl http://localhost:6336/v1/responses \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H "Content-Type: application/json" \
  --data-binary '{
    "model": "assistant",
    "input": [{
      "type": "message",
      "role": "user",
      "content": [
        {
          "type": "input_file",
          "file_data": "data:text/plain;base64,SGVsbG8gZnJvbSBEYXB0aW4=",
          "filename": "note.txt"
        },
        {"type": "input_text", "text": "Summarize the attached note."}
      ]
    }]
  }'
```

Replace `file_data` with exactly one of these forms when appropriate:

```json
{"type":"input_file","file_url":"https://example.com/report.pdf","filename":"report.pdf"}
```

```json
{"type":"input_file","file_id":"DAPTIN_FILE_REFERENCE_ID"}
```

`file_url` must be an HTTP(S) URL without embedded credentials and is forwarded
to the selected provider. Provider support for fetching that URL remains a
provider capability. `file_id` is a Daptin reference ID returned by
`POST /v1/files`; Daptin reads it as the active account through the existing
file resource permission path. An inaccessible or missing file fails before
the model provider is called. The resolved content and the original request
must fit within the gateway request-size limit.

Web search uses the existing `tools` capability; it does not have a separate
model capability:

```bash
curl http://localhost:6336/v1/responses \
  -H 'Authorization: Bearer YOUR_DAPTIN_TOKEN' \
  -H "Content-Type: application/json" \
  --data-binary '{
    "model": "assistant",
    "input": "Find the Daptin documentation and cite one source.",
    "tools": [{"type": "web_search"}]
  }'
```

For non-streaming calls, conforming providers return `web_search_call` output
items. Streaming providers may emit
`response.web_search_call.in_progress`,
`response.web_search_call.searching`, and
`response.web_search_call.completed`. The adapter is deliberately strict:
vendor-namespaced output item types such as `openrouter:web_search` are rejected
as provider errors rather than silently reinterpreted. Enable `tools` only for
deployments verified to accept the request and return the canonical Responses
shape.

File access, model execution, and metering remain independent decisions.
Permission to execute the model does not grant access to a referenced Daptin
file, and permission to read a file does not grant model execution. Successful
and provider-failed admitted calls terminalize through the same `api_usage`
lifecycle.

OpenAI SDKs can use `http://localhost:6336/v1` as their base URL and a Daptin
bearer token as their API key.

### Codex CLI

Daptin accepts the documented Codex Responses subset, including
reasoning includes, input-item IDs, function tool namespaces, and web search
tools. `include` and item IDs are preserved upstream. Unsupported fields,
including `client_metadata`, are rejected instead of being silently ignored.

Configure Codex with a Daptin bearer token in `DAPTIN_API_KEY`:

```toml
model = "assistant"
model_provider = "daptin"

[model_providers.daptin]
name = "Daptin"
base_url = "http://localhost:6336/v1"
env_key = "DAPTIN_API_KEY"
wire_api = "responses"
requires_openai_auth = false
```

Codex sends tool definitions, reasoning controls, and
`parallel_tool_calls` when those features are enabled. Declare `tools`,
`parallel_tools`, and `reasoning` in the `llm_model.capabilities` JSON only when
the selected upstream deployment supports them. A strict model rejects a
requested capability that it does not declare.
The `tools` capability covers function tools, namespaces, and web search; there
is no second web-search policy flag.

## Daptin actions

`$llm.chat` and `$llm.embedding` use the same engine as HTTP. They apply the
same strict request decoder, model authorization, route plan, provider call,
costing, and generic metering. `$llm.chat` is non-streaming in action chains.
After Daptin authorizes the containing action, its server-defined outcomes use
the normal trusted action context. `SWITCH_USER` changes the active account for
subsequent outcomes and their metering.

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

LLM invocation is metered through the `invoke` setting on the `llm_model`
resource. It is enabled by default. To change the cost expression, measured
quantity, or billing action, edit that resource's `metering.on_actions.invoke`
setting in the world schema. See [[API-Metering]] for the complete setup.

Hard limits use the database-backed generic quota state and fail closed when
that authority is unavailable. Olric counters protect deployments
(`max_concurrency`, RPM, TPM) and do not replace durable customer quotas.

The active account determines metering ownership. Guest invocation uses the
persisted guest account for usage and any configured plan limits while remaining
unauthenticated for model permissions. Without `SWITCH_USER`, a signed-in caller
is the metered account. After a trusted action performs `SWITCH_USER`, the
selected account's `api_member`, plan, usage, and quota are used.
If the wrapper action is itself metered, its admission occurred before the
switch and remains charged to the original caller.

An account without an active `api_member` still produces `api_usage`, but it has
no plan-defined maximum. Assign an active membership whenever a hard or soft
limit is required. See [[API-Metering]] for the shared ledger lifecycle.

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
  operation list, the active account's execute permission (or `GuestExecute`
  for a request without an account), and at least one enabled deployment.
- **Upstream endpoint or model not found:** an upstream HTTP 404 does not
  distinguish a missing operation endpoint from a missing model. Verify the
  provider's base URL, its support for the requested operation, and the
  deployment's `upstream_model`. See [Provider operation support](#provider-operation-support).
- **No healthy deployment:** verify provider/deployment enable flags,
  capabilities, operation lists, timeouts, health state, and protection limits.
- **Provider authentication error:** verify the provider's credential relationship and
  that decrypted credential content contains a non-empty `api_key`.
- **Responses file rejected:** verify that the model declares `files`, that
  exactly one of `file_data`, `file_url`, or `file_id` is present, and that the
  active account can read a referenced Daptin file. An upstream error after
  admission means the selected provider/model rejected the file form.
- **Web search provider error:** verify that the model declares `tools` and that
  the selected provider returns canonical `web_search_call` items rather than a
  vendor-namespaced output type.
- **Catalog reload rejected:** inspect `/llm/readyz` and logs for the stable
  failure stage; the previous valid catalog remains active.
- **Quota denied:** inspect the user's active `api_member`, related `api_plan`,
  and the generic `api_quota` bucket for the named metric/window.
