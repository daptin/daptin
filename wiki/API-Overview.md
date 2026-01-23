# API Overview

Daptin provides a JSON:API compliant REST API with automatic CRUD operations.

## Base URL

```
http://localhost:6336/api/{entity}
```

## JSON:API Compliance

All responses follow [JSON:API specification](https://jsonapi.org/):

```json
{
  "data": [...],
  "links": {...},
  "included": [...]
}
```

## Content Types

**Request:**
```
Content-Type: application/vnd.api+json
```

**Response:**
```
Content-Type: application/vnd.api+json
```

## Authentication

Include JWT token in Authorization header:

```bash
curl http://localhost:6336/api/todo \
  -H "Authorization: Bearer $TOKEN"
```

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/{entity}` | List records |
| GET | `/api/{entity}/{id}` | Get single record |
| POST | `/api/{entity}` | Create record |
| PATCH | `/api/{entity}/{id}` | Update record |
| DELETE | `/api/{entity}/{id}` | Delete record |
| GET | `/api/{entity}/{id}/{relation}` | Get related records |

## Response Structure

### List Response

```json
{
  "links": {
    "current_page": 1,
    "from": 0,
    "last_page": 5,
    "per_page": 10,
    "to": 10,
    "total": 47
  },
  "data": [
    {
      "type": "todo",
      "id": "abc123",
      "attributes": {
        "__type": "todo",
        "title": "Buy groceries",
        "completed": false,
        "created_at": "2024-01-15T10:30:00Z"
      },
      "relationships": {
        "user_account_id": {
          "data": {"type": "user_account", "id": "xyz789"}
        }
      }
    }
  ]
}
```

### Single Record Response

```json
{
  "data": {
    "type": "todo",
    "id": "abc123",
    "attributes": {...}
  },
  "included": [...]
}
```

## Error Responses

```json
{
  "errors": [
    {
      "status": "403",
      "title": "Forbidden",
      "detail": "Permission denied"
    }
  ]
}
```

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created |
| 204 | No Content (delete) |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 422 | Validation Error |
| 429 | Rate Limited |
| 500 | Server Error |

## Discovery Endpoints

| Endpoint | Description |
|----------|-------------|
| `/api/world` | List all entities |
| `/api/action` | List all actions |
| `/meta` | API metadata |
| `/openapi.yaml` | OpenAPI specification |
| `/health` | Health check |
| `/statistics` | System statistics |

## Meta Endpoint

```bash
curl http://localhost:6336/meta
```

Returns schema information for all entities.

## OpenAPI Documentation

```bash
curl http://localhost:6336/openapi.yaml
```

Full OpenAPI 3.0 specification for all endpoints.

## JavaScript Client Model

```bash
curl http://localhost:6336/jsmodel/todo
```

Returns JavaScript class definition for entity.

## Action Endpoint

Execute business logic:

```bash
curl -X POST http://localhost:6336/action/{entity}/{action_name} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {...}}'
```

### Action Response Types

| Type | Description |
|------|-------------|
| `client.notify` | Show notification |
| `client.redirect` | Redirect browser |
| `client.store.set` | Store value |
| `client.cookie.set` | Set cookie |
| `client.file.download` | Download file |

## Rate Limiting

Default: 500 requests/second per IP

When exceeded:
```
HTTP 429 Too Many Requests
```

Configure via:
```bash
curl -X POST http://localhost:6336/_config/backend/limit.rate \
  -H "Authorization: Bearer $TOKEN" -d '1000'
```

## CORS

All endpoints support CORS:
- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS`
- `Access-Control-Allow-Headers: Authorization, Content-Type`
- `Access-Control-Allow-Credentials: true`
