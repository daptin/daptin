# Integrations

API specifications for external service integration.

## Overview

The `integration` table stores API specifications that define:
- External API endpoints
- Authentication methods
- Request/response formats
- Connection parameters

## Integration Table

| Column | Type | Description |
|--------|------|-------------|
| `name` | label | Unique integration name |
| `specification_language` | label | API language (OpenAPI, GraphQL, WSDL) |
| `specification_format` | label | Format (json, yaml, xml) |
| `specification` | content | Full API specification |
| `authentication_type` | label | Auth method |
| `authentication_specification` | encrypted | Auth credentials |
| `enable` | bool | Active/inactive |

## Specification Languages

| Language | Description |
|----------|-------------|
| OpenAPI | REST API specification (Swagger) |
| GraphQL | GraphQL schema |
| WSDL | SOAP web services |
| Custom | Custom format |

## Authentication Types

| Type | Description |
|------|-------------|
| API Key | Header or query parameter API key |
| OAuth2 | OAuth 2.0 flow |
| Basic Auth | HTTP Basic Authentication |
| JWT | JSON Web Token |
| None | Public API |

## Create Integration

### OpenAPI Integration

```bash
curl -X POST http://localhost:6336/api/integration \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "integration",
      "attributes": {
        "name": "payment-gateway",
        "specification_language": "OpenAPI",
        "specification_format": "json",
        "specification": "{\"openapi\": \"3.0.0\", \"info\": {\"title\": \"Payment API\", \"version\": \"1.0\"}, \"servers\": [{\"url\": \"https://api.payment.com\"}], \"paths\": {...}}",
        "authentication_type": "API Key",
        "authentication_specification": "{\"header\": \"X-API-Key\", \"value\": \"your-api-key\"}",
        "enable": true
      }
    }
  }'
```

### API Key Authentication

```json
{
  "authentication_type": "API Key",
  "authentication_specification": {
    "header": "X-API-Key",
    "value": "your-secret-key"
  }
}
```

### OAuth2 Authentication

```json
{
  "authentication_type": "OAuth2",
  "authentication_specification": {
    "client_id": "your-client-id",
    "client_secret": "your-client-secret",
    "token_url": "https://auth.provider.com/token",
    "scopes": ["read", "write"]
  }
}
```

### Basic Auth

```json
{
  "authentication_type": "Basic Auth",
  "authentication_specification": {
    "username": "user",
    "password": "pass"
  }
}
```

## Update Integration

```bash
curl -X PATCH http://localhost:6336/api/integration/INTEGRATION_ID \
  -H "Authorization: Bearer $TOKEN" \
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

## List Integrations

```bash
curl http://localhost:6336/api/integration \
  -H "Authorization: Bearer $TOKEN"
```

## Get Integration

```bash
curl http://localhost:6336/api/integration/INTEGRATION_ID \
  -H "Authorization: Bearer $TOKEN"
```

## Delete Integration

```bash
curl -X DELETE http://localhost:6336/api/integration/INTEGRATION_ID \
  -H "Authorization: Bearer $TOKEN"
```

## OpenAPI Example

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
    }
  }
}
```

## Security Notes

- Authentication credentials stored encrypted
- Use environment variables for sensitive values
- Rotate API keys periodically
- Enable only necessary integrations

## Troubleshooting

### Integration Not Working

1. Check `enable` is true
2. Verify specification format is valid
3. Test authentication credentials
4. Check API endpoint is reachable

### Authentication Errors

1. Verify credentials are correct
2. Check token expiration
3. Ensure proper authentication type selected
