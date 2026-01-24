# Integrations

External API integration via OpenAPI specifications.

**Related**: [Authentication](Authentication.md) | [Actions Overview](Actions-Overview.md)

**Source of truth**: `server/resource/columns.go` (integration table), `server/actions/action_integration_*.go` (performers)

---

## Overview

The `integration` table stores OpenAPI specifications that define external APIs. When installed, each API operation becomes a callable Daptin action.

**Key features**:
- OpenAPI v2 (Swagger) and v3 support
- JSON or YAML specification format
- Multiple authentication methods
- Dynamic action creation from API operations

---

## Integration Table

| Column | Type | Description |
|--------|------|-------------|
| `name` | label | Unique integration name (becomes performer name) |
| `specification_language` | label | **Must be**: `openapiv2` or `openapiv3` |
| `specification_format` | label | **Must be**: `json` or `yaml` |
| `specification` | content | Full OpenAPI specification |
| `authentication_type` | label | Auth method: `oauth2`, `http`, `apiKey` |
| `authentication_specification` | encrypted | Auth credentials (JSON, encrypted) |
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

## Authentication Types

Authentication is configured via `authentication_type` and `authentication_specification` (JSON).

### OAuth2

Uses a stored OAuth token from the `oauth_token` table.

```json
{
  "authentication_type": "oauth2",
  "authentication_specification": {
    "oauth_token_id": "OAUTH_TOKEN_REFERENCE_ID"
  }
}
```

**Prerequisites**: First configure OAuth provider via `oauth_connect`, complete OAuth flow to get token stored in `oauth_token`. See [Authentication](Authentication.md#oauth-authentication).

### HTTP Basic

```json
{
  "authentication_type": "http",
  "authentication_specification": {
    "scheme": "basic",
    "username": "your-username",
    "password": "your-password"
  }
}
```

### HTTP Bearer Token

```json
{
  "authentication_type": "http",
  "authentication_specification": {
    "scheme": "bearer",
    "token": "your-bearer-token"
  }
}
```

### API Key

API key in header, query parameter, or cookie.

**Header**:
```json
{
  "authentication_type": "apiKey",
  "authentication_specification": {
    "name": "X-API-Key",
    "in": "header",
    "X-API-Key": "your-actual-api-key"
  }
}
```

**Query parameter**:
```json
{
  "authentication_type": "apiKey",
  "authentication_specification": {
    "name": "api_key",
    "in": "query",
    "api_key": "your-actual-api-key"
  }
}
```

**Cookie**:
```json
{
  "authentication_type": "apiKey",
  "authentication_specification": {
    "name": "session",
    "in": "cookie",
    "session": "your-session-value"
  }
}
```

---

## Create Integration

**Admin required** - Only administrators can create integrations.

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
        "authentication_type": "apiKey",
        "authentication_specification": "{\"name\":\"api_key\",\"in\":\"header\",\"api_key\":\"special-key\"}",
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
**Requires instance**: Yes (integration reference_id)

```bash
curl -X POST "http://localhost:6336/action/integration/INTEGRATION_REF_ID/install_integration" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

**What happens**:
1. Parses the OpenAPI specification
2. Creates an action for each operation (identified by `operationId`)
3. Maps path/query/body parameters to action input fields
4. Registers the integration name as a performer

---

## Execute Integration Actions

After installation, each OpenAPI operation becomes a callable action.

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

**Note**: After updating specification or authentication, re-run `install_integration` to regenerate actions.

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

- Authentication credentials stored encrypted in `authentication_specification`
- OAuth2 tokens automatically refresh when expired
- Only administrators can create/modify integrations
- Disable integrations when not in use (`enable: false`)
- Rotate API keys periodically

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

1. For OAuth2: Verify `oauth_token_id` references a valid token
2. For HTTP: Check `scheme`, `username`, `password` or `token` are correct
3. For API Key: Ensure `name`, `in`, and the actual key value are all present

### Actions Not Created

After creating the integration, run `install_integration`:
```bash
curl -X POST "http://localhost:6336/action/integration/INTEGRATION_ID/install_integration" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{}'
```

---

## See Also

- [Authentication](Authentication.md) - OAuth for integration auth
- [Actions Overview](Actions-Overview.md) - How actions work
- [Data Exchange](Data-Exchange.md) - Import/export via integrations
