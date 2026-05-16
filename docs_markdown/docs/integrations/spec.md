# API Spec integrations

Import an OpenAPI v3 or v2 version specification in JSON or YAML format.

Integrations are **enabled** by default.

## Enabled Integrations

Operations defined inside the specification can be installed and invoked as
action outcomes. New clients should use provider-scoped execution:

```http
POST /integration/{provider_name}/{operation_id}
```

Request body:

```json
{
  "oauth_token_id": "optional OAuth token reference id",
  "credential_id": "optional credential reference id",
  "input": {
    "operationParam": "value"
  }
}
```

Auth selectors are top-level request fields. Runtime fields such as
`oauth_token_id`, `credential_id`, `sessionUser`, and request metadata are
stripped from `input` before execution.

## Transport extensions

OpenAPI operations use REST execution by default. Add operation-level extension
fields to opt into other transports:

| Extension | Applies To | Description |
|-----------|------------|-------------|
| `x-daptin-transport` | all non-REST transports | One of `rest`, `graphql`, `grpc`, or `websocket`. |
| `x-daptin-upstream-path` | GraphQL, WebSocket | Upstream path to call instead of the facade OpenAPI path. Defaults to `/graphql` for GraphQL. |
| `x-daptin-timeout-ms` | GraphQL, WebSocket, gRPC | Per-operation timeout in milliseconds. Defaults to 10 seconds. |
| `x-daptin-graphql-document` | GraphQL | Required GraphQL query or mutation document. |
| `x-daptin-graphql-operation-name` | GraphQL | Optional GraphQL `operationName`. |
| `x-daptin-websocket-message-template` | WebSocket | Optional template for the outbound WebSocket message. |
| `x-daptin-websocket-response-selector` | WebSocket | Optional dot-path selector for the JSON response. |
| `x-daptin-grpc-service` | gRPC | Required fully-qualified service name. |
| `x-daptin-grpc-method` | gRPC | Unary method name. Defaults to `operationId`. |
| `x-daptin-grpc-descriptor-base64` | gRPC | Optional base64-encoded protobuf `FileDescriptorSet`; otherwise Daptin uses gRPC reflection. |

GraphQL variables, WebSocket messages, and gRPC request messages are built from
the operation input schema and do not include runtime auth selectors. Operation
discovery exposes sanitized transport metadata, but does not expose GraphQL
documents, credential contents, OAuth tokens, or gRPC descriptor blobs.

## GraphQL example

```json
{
  "paths": {
    "/linear/listIssues": {
      "post": {
        "operationId": "listIssues",
        "x-daptin-transport": "graphql",
        "x-daptin-upstream-path": "/graphql",
        "x-daptin-graphql-operation-name": "ListIssues",
        "x-daptin-graphql-document": "query ListIssues($first: Int!) { issues(first: $first) { nodes { id title } } }",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "first": {"type": "integer"}
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

## WebSocket example

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

## gRPC example

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

gRPC transport currently supports unary calls only. Streaming gRPC methods are
not supported.
