# Integrations overview

Daptin can import OpenAPI v2/v3 specifications and expose each operation as an installed integration action.

Installed integrations can be executed through provider-scoped routes:

```http
POST /integration/{provider_name}/{operation_id}
GET /integration/{provider_name}/operations
GET /integration/{provider_name}/operations/{operation_id}
GET /integration/{provider_name}/openapi.yaml
```

Use the provider-scoped execution route for new clients. Put provider operation
fields under `input`. OAuth and credential selectors are runtime fields and
must be top-level fields in the request body:

```json
{
  "oauth_token_id": "USER_OAUTH_TOKEN_REFERENCE_ID",
  "credential_id": "USER_CREDENTIAL_REFERENCE_ID",
  "input": {
    "operationParam": "value"
  }
}
```

Use `oauth_token_id` for OAuth integrations and `credential_id` for custom
credential integrations. Daptin validates token ownership/provider match and
credential ownership/permission before sending the upstream request.

OpenAPI operations execute as REST by default. Operations can also opt into
GraphQL-over-HTTP, short-lived WebSocket request/response, or unary gRPC
transports with `x-daptin-*` OpenAPI extensions. See the integration spec page
for the extension fields.

Examples to add:

- Accepting payments with Stripe
- 2FA OTP
- GraphQL provider operation
- WebSocket request/response provider operation
- Unary gRPC provider operation
