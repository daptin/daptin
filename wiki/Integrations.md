# Integrations

External API integration via OpenAPI specifications.

**Related**: [[Authentication|Authentication]] | [[Actions-Overview|Actions Overview]]

**Source of truth**: `server/resource/columns.go` (integration table), `server/actions/action_integration_*.go` (performers)

---

## Overview

The `integration` table stores OpenAPI specifications that define external APIs. When installed, each API operation becomes a callable Daptin action.

**Key features**:
- OpenAPI v2 (Swagger) and v3 support
- JSON or YAML specification format
- Multiple authentication methods
- REST, GraphQL-over-HTTP, unary gRPC, and short-lived WebSocket request/response transports
- Dynamic action creation from API operations
- Provider-scoped execution at `/integration/{provider_name}/{operation_id}`
- Provider-scoped discovery at `/integration/{provider_name}/operations`
- Scoped OpenAPI export at `/integration/{provider_name}/openapi.yaml`

---

## Which Endpoint Do I Call?

For new clients, call the provider-scoped integration endpoint:

```http
POST /integration/{provider_name}/{operation_id}
```

- `{provider_name}` is the value of `integration.name`, for example `airtable.com`, `asana.com`, or `github.com`.
- `{operation_id}` is the OpenAPI `operationId`, for example `airtableUpdateRecord`.
- Put operation inputs under `input`.
- For OAuth integrations, pass `oauth_token_id` at the top level.
- For custom credential integrations, pass `credential_id` at the top level.
- Do not put `oauth_token_id` or `credential_id` inside `input`; provider-scoped execution strips runtime fields from `input` and only honors top-level auth selectors.

Example:

```bash
curl -X POST "http://localhost:6336/integration/airtable.com/airtableUpdateRecord" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "oauth_token_id": "USER_OAUTH_TOKEN_REFERENCE_ID",
    "input": {
      "baseId": "appXXXXXXXXXXXXXX",
      "tableIdOrName": "tblXXXXXXXXXXXXXX",
      "recordId": "recXXXXXXXXXXXXXX",
      "fields": {
        "Status": "Done"
      }
    }
  }'
```

If you do not know the available operations, list them first:

```bash
curl "http://localhost:6336/integration/airtable.com/operations" \
  -H "Authorization: Bearer $TOKEN"
```

Then inspect one operation:

```bash
curl "http://localhost:6336/integration/airtable.com/operations/airtableUpdateRecord" \
  -H "Authorization: Bearer $TOKEN"
```

Older clients can still call generated action routes such as
`POST /action/integration/{operation_id}`, but the provider-scoped endpoint is
clearer because it keeps the provider name in the URL.

---

## Integration Table

| Column | Type | Description |
|--------|------|-------------|
| `name` | label | Unique integration name (becomes performer name) |
| `specification_language` | label | **Must be**: `openapiv2` or `openapiv3` |
| `specification_format` | label | **Must be**: `json` or `yaml` |
| `specification` | content | Full OpenAPI specification |
| `authentication_type` | label | Auth method: `oauth2` or `custom_credentials` |
| `authentication_specification` | encrypted | Auth metadata (JSON, encrypted); user secrets live in `oauth_token` or `credential` |
| `enable` | truefalse | Active/inactive (default: true) |

**Note**: This table has `DefaultGroups: adminsGroup` - only administrators can manage integrations.

---

## Specification Languages

| Value | Description |
|-------|-------------|
| `openapiv2` | OpenAPI 2.0 (Swagger) specification |
| `openapiv3` | OpenAPI 3.0 specification |

**Important**: Values must be lowercase: `openapiv2` or `openapiv3` (not "OpenAPI" or "swagger").

---

## Operation Transports

By default, Daptin executes OpenAPI operations as REST-style HTTP calls using
the method, path, parameters, request body, security schemes, and server URL
from the provider spec.

An operation can opt into another transport using OpenAPI extension fields on
that operation. These extensions are stored in `integration.specification`; user
tokens and API keys still come from `oauth_token` or `credential` at execution
time.

| Extension | Applies To | Description |
|-----------|------------|-------------|
| `x-daptin-transport` | all non-REST transports | One of `rest`, `graphql`, `grpc`, or `websocket`. Defaults to `rest`. |
| `x-daptin-upstream-path` | GraphQL, WebSocket | Upstream path to call instead of the facade path in `paths`. Defaults to `/graphql` for GraphQL. |
| `x-daptin-timeout-ms` | GraphQL, WebSocket, gRPC | Per-operation timeout in milliseconds. Defaults to 10 seconds. |
| `x-daptin-graphql-document` | GraphQL | Required GraphQL query or mutation document. If this is present and `x-daptin-transport` is omitted, Daptin treats the operation as GraphQL. |
| `x-daptin-graphql-operation-name` | GraphQL | Optional GraphQL `operationName`. |
| `x-daptin-websocket-message-template` | WebSocket | Optional template used to build the outbound WebSocket message from operation input. Without it, Daptin sends the derived input object as JSON. |
| `x-daptin-websocket-response-selector` | WebSocket | Optional dot-path selector for selecting part of the JSON response. |
| `x-daptin-grpc-service` | gRPC | Required fully-qualified service name, for example `grpc.testing.SearchService`. |
| `x-daptin-grpc-method` | gRPC | Unary method name. Defaults to the OpenAPI `operationId`. |
| `x-daptin-grpc-descriptor-base64` | gRPC | Optional base64-encoded `FileDescriptorSet`. If omitted, Daptin uses gRPC server reflection. |

### REST

REST is the default transport. Parameters are mapped from OpenAPI path, query,
header, and body definitions. Daptin resolves OAuth or custom credential auth
first and protects generated auth headers/query parameters from user-supplied
operation input.

```json
{
  "paths": {
    "/tasks/{task_gid}": {
      "get": {
        "operationId": "getTask",
        "parameters": [
          {"name": "task_gid", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "opt_fields", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}
```

### GraphQL

GraphQL operations are represented as OpenAPI operations so they can use the
same install, discovery, auth, permission, and provider-scoped execution flow.
Daptin posts to the GraphQL upstream with:

```json
{
  "query": "...",
  "operationName": "optional",
  "variables": {
    "fieldFromInput": "value"
  }
}
```

Runtime fields such as `credential_id`, `oauth_token_id`, `sessionUser`, and
request metadata are not included in GraphQL variables.

```json
{
  "paths": {
    "/linear/listIssues": {
      "post": {
        "operationId": "listIssues",
        "x-daptin-transport": "graphql",
        "x-daptin-upstream-path": "/graphql",
        "x-daptin-graphql-operation-name": "ListIssues",
        "x-daptin-graphql-document": "query ListIssues($first: Int!, $after: String) { issues(first: $first, after: $after) { nodes { id title } } }",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "first": {"type": "integer"},
                  "after": {"type": "string"}
                }
              }
            }
          }
        },
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}
```

Discovery exposes sanitized transport metadata such as type, upstream path, and
operation name. It does not expose the GraphQL document.

### WebSocket

WebSocket transport is for short-lived request/response operations. Daptin opens
a WebSocket connection, sends one JSON message, reads one response message, and
closes the connection. `http` and `https` server URLs are converted to `ws` and
`wss` for dialing.

```json
{
  "paths": {
    "/ws/search": {
      "post": {
        "operationId": "wsSearch",
        "x-daptin-transport": "websocket",
        "x-daptin-upstream-path": "/ws",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "query": {"type": "string"}
                }
              }
            }
          }
        },
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}
```

### gRPC

gRPC transport currently supports unary calls. It does not support client,
server, or bidirectional streaming methods. Daptin builds the input protobuf
message from the OpenAPI request body schema and operation input, resolves auth,
and sends auth headers as outgoing gRPC metadata.

Daptin obtains protobuf descriptors in one of two ways:

1. Use `x-daptin-grpc-descriptor-base64` when the spec embeds a base64-encoded
   `FileDescriptorSet`.
2. Otherwise use gRPC server reflection on the configured upstream.

```json
{
  "paths": {
    "/grpc/search": {
      "post": {
        "operationId": "Search",
        "x-daptin-transport": "grpc",
        "x-daptin-grpc-service": "grpc.testing.SearchService",
        "x-daptin-grpc-method": "Search",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "query": {"type": "string"}
                }
              }
            }
          }
        },
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}
```

---

## Authentication Types

Authentication is configured via `authentication_type` and `authentication_specification` (JSON). Daptin supports two integration auth resolver families:

The integration record stores the provider-level auth wiring only. It must not store a particular user's token or API key. The user-specific secret is selected when the generated integration action is executed.

| `authentication_type` | Stored on `integration.authentication_specification` | Supplied on each action execution | Secret source |
|-----------------------|------------------------------------------------------|-----------------------------------|---------------|
| `oauth2` | `oauth_connect_id` | `oauth_token_id` | `oauth_token` |
| `custom_credentials` | Credential usage metadata (`scheme`, `token_field`, `name`, `in`, etc.) | `credential_id` | `credential.content` |

### OAuth2

OAuth integrations configure the provider/app connection with `oauth_connect_id`. The executing user supplies their own `oauth_token_id` when the integration action runs.

```json
{
  "authentication_type": "oauth2",
  "authentication_specification": {
    "oauth_connect_id": "OAUTH_CONNECT_REFERENCE_ID"
  }
}
```

**Prerequisites**: Configure the OAuth provider via `oauth_connect`, then have each user complete the existing OAuth flow to create their own `oauth_token`. See [OAuth Authentication](OAuth-Authentication.md).

Integrations must not store `oauth_token_id` directly in `authentication_specification`; token selection happens per execution so Daptin can validate token ownership against the current user.

At execution time Daptin verifies that the supplied token belongs to the authenticated request user and was created from the same `oauth_connect_id` configured on the integration.

### Custom Credentials

Custom credential integrations describe how to use a `credential.content` field. The executing user supplies `credential_id` when the integration action runs. The credential must be owned by the user, usable through group permission, or usable by an administrator.

**Bearer token**:

```json
{
  "authentication_type": "custom_credentials",
  "authentication_specification": {
    "scheme": "bearer",
    "token_field": "token"
  }
}
```

The credential row stores the actual user secret:
```json
{
  "token": "actual-user-token"
}
```

**API key in a header**:

```json
{
  "authentication_type": "custom_credentials",
  "authentication_specification": {
    "name": "X-API-Key",
    "in": "header",
    "value_field": "api_key"
  }
}
```

The credential row stores:
```json
{
  "api_key": "actual-user-api-key"
}
```

**Basic auth**:

```json
{
  "authentication_type": "custom_credentials",
  "authentication_specification": {
    "scheme": "basic",
    "username_field": "username",
    "password_field": "password"
  }
}
```

---

## Create Integration

**Admin required** - Only administrators can create integrations.

For large provider specs, prefer `daptin-cli integration import` instead of
hand-building JSON. The CLI can read specs from disk, URL, or stdin:

```bash
daptin-cli integration import \
  --provider asana.com \
  --spec-file ./asana_oas.yaml \
  --auth oauth2 \
  --oauth-connect asana.com

daptin-cli integration install asana.com
```

The same command supports URL and stdin inputs:

```bash
daptin-cli integration import \
  --provider example.com \
  --spec-url https://example.com/openapi.yaml \
  --auth custom_credentials \
  --auth-spec-file ./auth.json

curl -L https://example.com/openapi.yaml | daptin-cli integration import \
  --provider example.com \
  --spec-stdin \
  --auth custom_credentials \
  --auth-spec-json '{"name":"X-API-Key","in":"header","value_field":"api_key"}'
```

The raw JSON API remains available for automation that already prepares the
`integration` row:

```bash
curl -X POST http://localhost:6336/api/integration \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "integration",
      "attributes": {
        "name": "petstore",
        "specification_language": "openapiv3",
        "specification_format": "json",
        "specification": "{\"openapi\":\"3.0.0\",\"info\":{\"title\":\"Petstore\",\"version\":\"1.0\"},\"servers\":[{\"url\":\"https://petstore.swagger.io/v2\"}],\"paths\":{\"/pet/{petId}\":{\"get\":{\"operationId\":\"getPetById\",\"parameters\":[{\"name\":\"petId\",\"in\":\"path\",\"required\":true,\"schema\":{\"type\":\"integer\"}}],\"responses\":{\"200\":{\"description\":\"Success\"}}}}}}",
        "authentication_type": "custom_credentials",
        "authentication_specification": "{\"name\":\"X-API-Key\",\"in\":\"header\",\"value_field\":\"api_key\"}",
        "enable": true
      }
    }
  }'
```

**Response** includes the `reference_id` needed for installation:
```json
{
  "data": {
    "type": "integration",
    "id": "019bec12-3456-7890-abcd-ef1234567890",
    "attributes": {
      "name": "petstore",
      "reference_id": "019bec12-3456-7890-abcd-ef1234567890"
    }
  }
}
```

---

## Install Integration

After creating an integration record, install it to create actions for each API operation.

**Action**: `install_integration`
**OnType**: `integration`
**Requires instance**: Yes. Pass the integration `reference_id` as `attributes.integration_id`.

```bash
curl -X POST "http://localhost:6336/action/integration/install_integration" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "integration_id": "INTEGRATION_REF_ID"
    }
  }'
```

**What happens**:
1. Parses the OpenAPI specification
2. Creates an action for each operation (identified by `operationId`)
3. Adds the auth selector input for the integration type (`oauth_token_id` or `credential_id`)
4. Maps path/query/body parameters to action input fields
5. Registers the integration name as a performer
6. Refreshes provider-scoped operation mappings in memory without requiring a server restart

---

## Discover Integration Operations

After installation, clients can inspect one provider without downloading the
global `/openapi.yaml` document.

| Endpoint | Description |
|----------|-------------|
| `GET /integration/{provider_name}/operations` | Compact list of operation ids, provider methods, provider paths, summaries, descriptions, and auth selector metadata. |
| `GET /integration/{provider_name}/operations/{operation_id}` | One operation with auth selector metadata, provider method/path, inputs, request body hints, response hints, and derived schemas. |
| `GET /integration/{provider_name}/openapi.yaml` | Scoped OpenAPI document containing only Daptin execution endpoints for the selected provider. |

Use discovery when you are building a UI, SDK, or agent and need to know what to
send. The list endpoint answers "what operations exist?", the describe endpoint
answers "what fields does this operation need?", and the scoped OpenAPI endpoint
is for OpenAPI-aware tooling.

Example:

```bash
curl "http://localhost:6336/integration/asana.com/operations" \
  -H "Authorization: Bearer $TOKEN"

curl "http://localhost:6336/integration/asana.com/operations/getWorkspaces" \
  -H "Authorization: Bearer $TOKEN"

curl "http://localhost:6336/integration/asana.com/openapi.yaml" \
  -H "Authorization: Bearer $TOKEN"
```

Example operation detail shape:

```json
{
  "provider": "asana.com",
  "operation_id": "getWorkspaces",
  "method": "GET",
  "path": "/workspaces",
  "auth": {
    "type": "oauth2",
    "execution_field": "oauth_token_id",
    "required": true
  },
  "inputs": [
    {
      "name": "opt_fields",
      "in": "query",
      "required": false,
      "type": "string"
    }
  ],
  "input_schema": {
    "type": "object"
  },
  "response_schema": {
    "type": "object"
  }
}
```

Discovery is generated from the installed OpenAPI spec. Parameter, request body,
and response hints come from the provider spec. Daptin only falls back to a
free-form `input` object when the spec does not declare concrete inputs for the
operation.

Auth selectors are reported separately from provider operation inputs:

| `authentication_type` | Discovery auth selector | Execution body field |
|-----------------------|-------------------------|----------------------|
| `oauth2` | `oauth_token_id` | `oauth_token_id` |
| `custom_credentials` | `credential_id` | `credential_id` |

---

## Execute Integration Operations

After installation, each OpenAPI operation can be executed in two ways:

- Provider-scoped route: `POST /integration/{provider_name}/{operation_id}`
- Generated action route: `POST /action/integration/{operation_id}`

The provider-scoped route is preferred for new clients because the OpenAPI `operationId` stays inside the provider namespace. This avoids artificial provider prefixes in operation names and makes logs/audits read as `provider=asana.com operation=getWorkspaces`.

### Provider-scoped Route

Request body shape:

```json
{
  "oauth_token_id": "optional OAuth token reference id",
  "credential_id": "optional credential reference id",
  "input": {
    "pathOrQueryOrBodyField": "value"
  }
}
```

Use either `oauth_token_id` or `credential_id` depending on the integration auth
type. Do not put these auth selector fields inside `input`.

```bash
# Call the getPetById operation from petstore integration
curl -X POST "http://localhost:6336/integration/petstore/getPetById" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "input": {
      "petId": "123"
    }
  }'
```

For OAuth2 integrations, pass the current user's token reference at execution time:

```bash
curl -X POST "http://localhost:6336/integration/github.com/listRepos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "oauth_token_id": "USER_OAUTH_TOKEN_REFERENCE_ID",
    "input": {
      "owner": "daptin"
    }
  }'
```

For custom credential integrations, pass the credential reference at execution time:

```bash
curl -X POST "http://localhost:6336/integration/example.com/listUsers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "credential_id": "USER_CREDENTIAL_REFERENCE_ID",
    "input": {}
  }'
```

Daptin rejects the call if `oauth_token_id` belongs to another user or was issued for a different `oauth_connect` than the integration expects. Daptin rejects custom credential calls if `credential_id` is not readable by the current user, or if the decrypted credential content does not contain the fields named by `authentication_specification`.

### Generated Action Route

The generated action route is retained for existing clients.

**Action names**: Use the `operationId` from the OpenAPI spec
**OnType**: `integration`
**InstanceOptional**: true (no instance ID required)

```bash
# Call the getPetById operation from petstore integration
curl -X POST "http://localhost:6336/action/integration/getPetById" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "petId": "123"
    }
  }'
```

For OAuth2 integrations, pass the current user's token reference at execution time:

```bash
curl -X POST "http://localhost:6336/action/integration/listRepos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "oauth_token_id": "USER_OAUTH_TOKEN_REFERENCE_ID",
      "owner": "daptin"
    }
  }'
```

Daptin rejects the call if `oauth_token_id` belongs to another user or was issued for a different `oauth_connect` than the integration expects.

For custom credential integrations, pass the credential reference at execution time:

```bash
curl -X POST "http://localhost:6336/action/integration/listUsers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "credential_id": "USER_CREDENTIAL_REFERENCE_ID"
    }
  }'
```

Daptin rejects the call if `credential_id` is not readable by the current user, or if the decrypted credential content does not contain the fields named by `authentication_specification`.

**Response**:
```json
[
  {
    "ResponseType": "petstore.getPetById.response",
    "Attributes": {
      "id": 123,
      "name": "doggie",
      "status": "available"
    }
  },
  {
    "ResponseType": "petstore.getPetById.statusCode",
    "Attributes": 200
  }
]
```

### Parameter Mapping

| OpenAPI Location | Daptin Input |
|------------------|--------------|
| Path parameters | `attributes.{paramName}` |
| Query parameters | `attributes.{paramName}` |
| Header parameters | `attributes.{paramName}` |
| Request body fields | `attributes.{fieldName}` |

---

## Update Integration

```bash
curl -X PATCH http://localhost:6336/api/integration/INTEGRATION_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "integration",
      "id": "INTEGRATION_ID",
      "attributes": {
        "enable": false
      }
    }
  }'
```

**Note**: After updating specification or authentication, re-run `install_integration` to regenerate actions and refresh provider-scoped operation mappings.

---

## List Integrations

```bash
curl http://localhost:6336/api/integration \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## Get Integration

```bash
curl http://localhost:6336/api/integration/INTEGRATION_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## Delete Integration

```bash
curl -X DELETE http://localhost:6336/api/integration/INTEGRATION_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**Note**: This removes the integration record but does not automatically remove the installed actions.

---

## OpenAPI Example

### OpenAPI v3 (JSON)

```json
{
  "openapi": "3.0.0",
  "info": {
    "title": "External API",
    "version": "1.0.0"
  },
  "servers": [
    {
      "url": "https://api.example.com/v1"
    }
  ],
  "components": {
    "securitySchemes": {
      "ApiKeyAuth": {
        "type": "apiKey",
        "in": "header",
        "name": "X-API-Key"
      }
    }
  },
  "security": [
    {"ApiKeyAuth": []}
  ],
  "paths": {
    "/users": {
      "get": {
        "summary": "List users",
        "operationId": "listUsers",
        "responses": {
          "200": {
            "description": "Successful response"
          }
        }
      }
    },
    "/users/{userId}": {
      "get": {
        "summary": "Get user by ID",
        "operationId": "getUserById",
        "parameters": [
          {
            "name": "userId",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response"
          }
        }
      }
    }
  }
}
```

### OpenAPI v2 (Swagger, YAML)

```yaml
swagger: "2.0"
info:
  title: External API
  version: "1.0"
host: api.example.com
basePath: /v1
schemes:
  - https
securityDefinitions:
  ApiKeyAuth:
    type: apiKey
    in: header
    name: X-API-Key
security:
  - ApiKeyAuth: []
paths:
  /users:
    get:
      operationId: listUsers
      responses:
        200:
          description: Success
```

---

## Security Notes

- Integration `authentication_specification` stores auth metadata only
- OAuth2 integrations store the provider `oauth_connect_id`; users pass their own `oauth_token_id` during execution
- Custom credential integrations describe how to use a credential; users pass their own `credential_id` during execution
- Daptin validates OAuth token ownership and provider match before using an `oauth_token`
- Daptin validates credential ownership/permission before decrypting `credential.content`
- Generated auth headers/query parameters are protected from user-supplied action attributes. For example, an action input named `Authorization` cannot override the OAuth or credential auth header Daptin resolved for the outbound request.
- Provider-scoped execution ignores runtime auth selector fields inside `input`; `oauth_token_id` and `credential_id` must be top-level request fields
- GraphQL variables, WebSocket messages, and gRPC request messages exclude runtime fields such as `credential_id`, `oauth_token_id`, `sessionUser`, and request metadata
- Operation discovery exposes sanitized transport metadata but does not expose GraphQL documents, credential contents, OAuth tokens, or gRPC descriptor blobs
- Only administrators can create/modify integrations
- Disable integrations when not in use (`enable: false`)
- Rotate credentials periodically

---

## Troubleshooting

### Integration Not Working

1. Verify `enable` is true
2. Check `specification_language` is exactly `openapiv2` or `openapiv3`
3. Check `specification_format` is exactly `json` or `yaml`
4. Ensure specification has `servers` array (v3) or `host` (v2)
5. Verify each operation has `operationId`

### "No servers found" Error

The OpenAPI spec must include server information:

**v3**: `"servers": [{"url": "https://api.example.com"}]`
**v2**: `"host": "api.example.com"` with `"schemes": ["https"]`

### "no such method" Error

The operation ID doesn't exist in the specification. Check:
1. The `operationId` spelling matches exactly
2. The integration was installed after the spec was updated
3. The operation exists in the spec's `paths`

### Authentication Errors

1. For OAuth2: Verify execution attributes include `oauth_token_id`, the token belongs to the current user, and its `oauth_connect_id` matches the integration
2. For custom credentials: Verify execution attributes include `credential_id`, the credential is usable by the current user, and `credential.content` contains the fields named by `authentication_specification`
3. For header/query auth: Verify the OpenAPI security scheme matches `authentication_type`; Daptin intentionally ignores action attributes that try to overwrite protected auth fields
4. For provider-scoped execution: Verify `oauth_token_id` or `credential_id` is top-level in the JSON body, not inside `input`

### GraphQL Transport Errors

1. `x-daptin-graphql-document is required`: Add `x-daptin-graphql-document` to the operation or use REST transport
2. Wrong upstream path: Set `x-daptin-upstream-path` or rely on the GraphQL default `/graphql`
3. Missing variables: Ensure the OpenAPI request body schema declares the fields Daptin should map into GraphQL variables

### WebSocket Transport Errors

1. Dial failure: Ensure the integration server URL uses `http`, `https`, `ws`, or `wss`
2. Read timeout: WebSocket transport expects one response message before the operation timeout
3. Selector failure: Check `x-daptin-websocket-response-selector` against the JSON response shape

### gRPC Transport Errors

1. Reflection failure: Enable gRPC server reflection or provide `x-daptin-grpc-descriptor-base64`
2. Method not found: Check `x-daptin-grpc-service` and `x-daptin-grpc-method`
3. Streaming method unsupported: Use a unary method; streaming gRPC is not currently supported

### Actions Not Created

After creating the integration, run `install_integration`:
```bash
curl -X POST "http://localhost:6336/action/integration/install_integration" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "integration_id": "INTEGRATION_ID"
    }
  }'
```

---

## See Also

- [[Authentication|Authentication]] - OAuth for integration auth
- [[Actions-Overview|Actions Overview]] - How actions work
- [[Data-Exchange|Data Exchange]] - Import/export via integrations
