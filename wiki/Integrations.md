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
- Dynamic action creation from API operations

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

- Integration `authentication_specification` stores auth metadata only
- OAuth2 integrations store the provider `oauth_connect_id`; users pass their own `oauth_token_id` during execution
- Custom credential integrations describe how to use a credential; users pass their own `credential_id` during execution
- Daptin validates OAuth token ownership and provider match before using an `oauth_token`
- Daptin validates credential ownership/permission before decrypting `credential.content`
- Generated auth headers/query parameters are protected from user-supplied action attributes. For example, an action input named `Authorization` cannot override the OAuth or credential auth header Daptin resolved for the outbound request.
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
