# LLM gateway deployment example

This example starts a clean Daptin `v0.13.0` deployment with PostgreSQL and
idempotently creates the complete gateway catalog:

```text
credential -> llm_provider <- llm_deployment -> llm_model
```

Copy `.env.example` to `.env`, replace both secrets, and select the provider and
model. Then start the stack:

```sh
docker compose up --wait
docker compose logs bootstrap
```

The bootstrap container may be run again safely:

```sh
docker compose run --rm bootstrap
```

Verify the configured model:

```sh
TOKEN="$(curl --fail --silent http://localhost:6336/action/user_account/signin \
  -H 'Content-Type: application/vnd.api+json' \
  --data-binary "{\"attributes\":{\"email\":\"$DAPTIN_ADMIN_EMAIL\",\"password\":\"$DAPTIN_ADMIN_PASSWORD\"}}" \
  | jq -r '.[]? | select(.Attributes.key == "token") | .Attributes.value')"

curl --fail http://localhost:6336/v1/models \
  -H "Authorization: Bearer $TOKEN"

curl --fail http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary "{\"model\":\"$LLM_PUBLIC_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Reply with pong\"}]}"
```

Provider presets with default base URLs are `openai`, `google`, `openrouter`,
and `lilac`. Set `LLM_PROVIDER_TYPE=openai-compatible` and `LLM_BASE_URL` for a
custom endpoint. Plain HTTP and private-network URLs are explicitly opted into
by the bootstrapper when their URL requires it.

Operations and capabilities are policy, not provider discovery. Set
`LLM_OPERATIONS` and `LLM_CAPABILITIES` to only the behavior verified for the
selected upstream model. Streaming is a request mode rather than a model
capability; valid capability keys are listed in the gateway guide. Provider
credentials are stored in Daptin's encrypted
credential resource and are never written to the catalog records.

For production, replace the development PostgreSQL password, keep `.env` out of
source control, pin the Daptin image by digest, configure backups, and apply the
resource permissions described in the [LLM gateway guide](../../wiki/LLM-Providers.md).
