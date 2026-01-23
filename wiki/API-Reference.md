# API Reference

Complete reference of all Daptin REST API endpoints.

## Base URL

Default: `http://localhost:6336`

## Authentication

Most endpoints require JWT authentication:

```bash
-H "Authorization: Bearer $TOKEN"
```

## JSON:API Endpoints

Daptin auto-generates CRUD endpoints for all entities following [JSON:API](https://jsonapi.org/) specification.

### List Resources

```
GET /api/{entity}
```

**Parameters:**

| Parameter | Description | Example |
|-----------|-------------|---------|
| query | Filter conditions (JSON) | `[{"column":"name","operator":"is","value":"test"}]` |
| page[number] | Page number (1-indexed) | `1` |
| page[size] | Items per page | `10` |
| sort | Sort field (prefix `-` for desc) | `-created_at` |
| include | Related resources to include | `user_account` |

**Example:**

```bash
curl 'http://localhost:6336/api/user_account?page[size]=10&page[number]=1&sort=-created_at' \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**

```json
{
  "data": [
    {
      "type": "user_account",
      "id": "abc123-ref-id",
      "attributes": {
        "name": "John Doe",
        "email": "john@example.com"
      },
      "relationships": {}
    }
  ],
  "links": {
    "current_page": 1,
    "from": 0,
    "last_page": 10,
    "per_page": 10,
    "to": 10,
    "total": 100
  }
}
```

### Get Single Resource

```
GET /api/{entity}/{reference_id}
```

**Example:**

```bash
curl http://localhost:6336/api/user_account/abc123-ref-id \
  -H "Authorization: Bearer $TOKEN"
```

### Create Resource

```
POST /api/{entity}
Content-Type: application/vnd.api+json
```

**Body:**

```json
{
  "data": {
    "type": "{entity}",
    "attributes": {
      "field1": "value1",
      "field2": "value2"
    }
  }
}
```

**Example:**

```bash
curl -X POST http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "attributes": {
        "name": "Jane Doe",
        "email": "jane@example.com"
      }
    }
  }'
```

### Update Resource

```
PATCH /api/{entity}/{reference_id}
Content-Type: application/vnd.api+json
```

**Body:**

```json
{
  "data": {
    "type": "{entity}",
    "id": "{reference_id}",
    "attributes": {
      "field1": "new_value"
    }
  }
}
```

### Delete Resource

```
DELETE /api/{entity}/{reference_id}
```

## Query Syntax

Filter resources using JSON query parameter:

```
query=[{"column":"field","operator":"op","value":"val"}]
```

### Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `is` | Exact match | `{"column":"status","operator":"is","value":"active"}` |
| `eq` | Equal | `{"column":"count","operator":"eq","value":10}` |
| `neq` | Not equal | `{"column":"status","operator":"neq","value":"deleted"}` |
| `contains` | LIKE %value% | `{"column":"name","operator":"contains","value":"john"}` |
| `like` | LIKE pattern | `{"column":"email","operator":"like","value":"%@gmail.com"}` |
| `ilike` | Case-insensitive LIKE | `{"column":"name","operator":"ilike","value":"john"}` |
| `in` | IN list | `{"column":"status","operator":"in","value":["a","b"]}` |
| `is not` | Not equal | `{"column":"type","operator":"is not","value":"admin"}` |
| `before` | Less than (dates) | `{"column":"created_at","operator":"before","value":"2024-01-01"}` |
| `after` | Greater than (dates) | `{"column":"created_at","operator":"after","value":"2024-01-01"}` |
| `more than` | Greater than | `{"column":"price","operator":"more than","value":100}` |
| `less than` | Less than | `{"column":"price","operator":"less than","value":100}` |
| `any of` | Any value matches | `{"column":"tags","operator":"any of","value":["a","b"]}` |
| `none of` | No value matches | `{"column":"tags","operator":"none of","value":["x","y"]}` |
| `is empty` | NULL or empty | `{"column":"notes","operator":"is empty","value":""}` |
| `is true` | Boolean true | `{"column":"active","operator":"is true","value":""}` |
| `is false` | Boolean false | `{"column":"active","operator":"is false","value":""}` |
| `fuzzy` | Fuzzy text search | `{"column":"name","operator":"fuzzy","value":"jon"}` |

### Multiple Conditions

```bash
curl 'http://localhost:6336/api/order?query=[{"column":"status","operator":"is","value":"pending"},{"column":"total","operator":"more than","value":100}]' \
  -H "Authorization: Bearer $TOKEN"
```

## Relationships

### Include Related Resources

```
GET /api/{entity}?include={relation}
```

```bash
curl 'http://localhost:6336/api/order?include=customer' \
  -H "Authorization: Bearer $TOKEN"
```

### Create with Relationships

```json
{
  "data": {
    "type": "order",
    "attributes": {
      "total": 100
    },
    "relationships": {
      "customer_id": {
        "data": {"type": "customer", "id": "customer-ref-id"}
      }
    }
  }
}
```

## Action Endpoints

### Execute Action

```
POST /action/{entity}/{action_name}
POST /action/{entity}/{action_name}/{reference_id}
```

**Body:**

```json
{
  "attributes": {
    "param1": "value1",
    "param2": "value2"
  }
}
```

**Example:**

```bash
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "password": "password"
    }
  }'
```

### List Available Actions

```
GET /actions
```

Returns guest-accessible actions.

## Aggregation Endpoints

### Aggregate Query

```
GET /aggregate/{entity}
POST /aggregate/{entity}
```

**Parameters:**

| Parameter | Description |
|-----------|-------------|
| column | Column to aggregate |
| aggregator | Function (sum, count, avg, min, max) |
| group | Group by column |
| query | Filter conditions |

**Example:**

```bash
curl 'http://localhost:6336/aggregate/order?column=total&aggregator=sum&group=status' \
  -H "Authorization: Bearer $TOKEN"
```

## Asset Endpoints

### Get Asset

```
GET /asset/{entity}/{reference_id}/{column_name}
```

Returns file stored in asset column.

### Upload Asset

```
POST /asset/{entity}/{reference_id}/{column_name}/upload
```

Multipart file upload.

### Delete Asset

```
DELETE /asset/{entity}/{reference_id}/{column_name}/upload
```

## Configuration Endpoints

### Get Configuration

```
GET /_config
GET /_config/{section}/{key}
```

### Set Configuration

```
POST /_config/{section}/{key}
PATCH /_config/{section}/{key}
PUT /_config/{section}/{key}
```

**Example:**

```bash
curl -X POST 'http://localhost:6336/_config/backend/hostname' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer $TOKEN' \
  -d '"example.com"'
```

### Delete Configuration

```
DELETE /_config/{section}/{key}
```

## Feed Endpoints

### Get Feed

```
GET /feed/{feed_name}
```

Returns RSS/Atom feed for configured streams.

## State Machine Endpoints

### Start State Machine

```
POST /track/start/{state_machine_id}
```

### Trigger Event

```
POST /track/event/{entity}/{object_state_id}/{event_name}
```

## Metadata Endpoints

### Get Schema Metadata

```
GET /meta
```

Returns all table definitions and relationships.

### Get OpenAPI Specification

```
GET /openapi.yaml
```

Returns OpenAPI/Swagger specification.

### Get JS Model

```
GET /jsmodel/{entity}
```

Returns JavaScript model definition.

## Statistics Endpoint

```
GET /statistics
```

Returns server statistics including:
- Memory usage
- Goroutine count
- Request counts
- Database stats

## Health Check

```
GET /ping
```

Returns: `pong`

## WebSocket Endpoints

### Real-time Updates

```
WS /live
```

Subscribe to entity changes via WebSocket.

### YJS Collaboration

```
WS /yjs/{document_name}
```

Real-time document collaboration.

## GraphQL Endpoint

When enabled:

```
POST /graphql
GET /graphql
```

**Example:**

```bash
curl -X POST http://localhost:6336/graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "{ user_account { name email } }"
  }'
```

## Response Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 204 | No Content (delete success) |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 409 | Conflict |
| 500 | Internal Server Error |

## Error Response

```json
{
  "errors": [
    {
      "status": "400",
      "title": "Bad Request",
      "detail": "Invalid email format"
    }
  ]
}
```

## Rate Limiting Headers

When rate limiting is enabled:

| Header | Description |
|--------|-------------|
| X-RateLimit-Limit | Max requests per window |
| X-RateLimit-Remaining | Remaining requests |
| X-RateLimit-Reset | Window reset timestamp |

## Content Types

| Type | Usage |
|------|-------|
| `application/vnd.api+json` | JSON:API requests/responses |
| `application/json` | Action requests |
| `multipart/form-data` | File uploads |

## Pagination

JSON:API pagination in response `links`:

```json
{
  "links": {
    "current_page": 1,
    "from": 0,
    "last_page": 10,
    "per_page": 10,
    "to": 10,
    "total": 100
  }
}
```

## CORS

CORS is enabled by default. Configure allowed origins via:

```bash
curl -X POST 'http://localhost:6336/_config/backend/cors.allowed_origins' \
  -H 'Authorization: Bearer $TOKEN' \
  -d '"https://example.com"'
```
