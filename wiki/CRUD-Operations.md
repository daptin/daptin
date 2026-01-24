# CRUD Operations

Create, Read, Update, and Delete records via the REST API.

Daptin uses [JSON:API](https://jsonapi.org/) format for all requests and responses.

---

## Create (POST)

```bash
curl -X POST http://localhost:6336/api/todo \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "attributes": {
        "title": "Buy groceries",
        "completed": false
      }
    }
  }'
```

**Response (201 Created):**
```json
{
  "data": {
    "type": "todo",
    "id": "abc123-def456-...",
    "attributes": {
      "title": "Buy groceries",
      "completed": false,
      "reference_id": "abc123-def456-...",
      "created_at": "2026-01-24T10:30:00Z",
      "updated_at": "2026-01-24T10:30:00Z",
      "permission": 2097151
    }
  }
}
```

### Create with Relationship

Link to an existing record:

```bash
curl -X POST http://localhost:6336/api/todo \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "attributes": {
        "title": "Team task"
      },
      "relationships": {
        "project_id": {
          "data": {"type": "project", "id": "PROJECT_REFERENCE_ID"}
        }
      }
    }
  }'
```

---

## Read (GET)

### List All Records

```bash
curl http://localhost:6336/api/todo \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "links": {
    "current_page": 1,
    "from": 0,
    "per_page": 10,
    "total": 25
  },
  "data": [
    {"type": "todo", "id": "...", "attributes": {...}},
    {"type": "todo", "id": "...", "attributes": {...}}
  ]
}
```

### Get Single Record

```bash
curl http://localhost:6336/api/todo/REFERENCE_ID \
  -H "Authorization: Bearer $TOKEN"
```

### Include Related Records

```bash
curl "http://localhost:6336/api/todo/REFERENCE_ID?include=project_id" \
  -H "Authorization: Bearer $TOKEN"
```

**Response with included:**
```json
{
  "data": {
    "type": "todo",
    "id": "...",
    "attributes": {...},
    "relationships": {
      "project_id": {"data": {"type": "project", "id": "proj123"}}
    }
  },
  "included": [
    {
      "type": "project",
      "id": "proj123",
      "attributes": {"name": "My Project"}
    }
  ]
}
```

### Filter Records

```bash
curl 'http://localhost:6336/api/todo?query=[{"column":"completed","operator":"is","value":"false"}]' \
  -H "Authorization: Bearer $TOKEN"
```

See [Filtering and Pagination](Filtering-and-Pagination.md) for all filter options.

### Sort Records

```bash
# Ascending
curl "http://localhost:6336/api/todo?sort=created_at" \
  -H "Authorization: Bearer $TOKEN"

# Descending (prefix with -)
curl "http://localhost:6336/api/todo?sort=-created_at" \
  -H "Authorization: Bearer $TOKEN"
```

### Paginate

```bash
curl "http://localhost:6336/api/todo?page[number]=2&page[size]=20" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Update (PATCH)

Update only the fields you want to change:

```bash
curl -X PATCH http://localhost:6336/api/todo/REFERENCE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "REFERENCE_ID",
      "attributes": {
        "completed": true
      }
    }
  }'
```

### Update Relationship

```bash
curl -X PATCH http://localhost:6336/api/todo/REFERENCE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "REFERENCE_ID",
      "relationships": {
        "project_id": {
          "data": {"type": "project", "id": "NEW_PROJECT_ID"}
        }
      }
    }
  }'
```

---

## Delete (DELETE)

```bash
curl -X DELETE http://localhost:6336/api/todo/REFERENCE_ID \
  -H "Authorization: Bearer $TOKEN"
```

**Response:** 200 OK with empty data, or 204 No Content

---

## Standard Fields

Every record automatically has these fields:

| Field | Description |
|-------|-------------|
| `reference_id` | Unique UUID (used as `id` in API) |
| `created_at` | When the record was created |
| `updated_at` | When the record was last modified |
| `permission` | Access control value |

---

## Common Errors

| Status | Meaning |
|--------|---------|
| 400 | Invalid JSON or missing required field |
| 401 | Not authenticated (missing or invalid token) |
| 403 | No permission for this operation |
| 404 | Record or table not found |
| 422 | Validation failed |

### Example Error Response

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

---

## Tips

1. **Always include the `type`** in your request body - it must match the table name
2. **Use `reference_id`** (the UUID) as the `id` in URLs and request bodies
3. **Set Content-Type header** to `application/vnd.api+json` for POST/PATCH
4. **Include Authorization header** for any non-public data

---

## See Also

- [Filtering and Pagination](Filtering-and-Pagination.md) - Query options
- [Relationships](Relationships.md) - Linking tables
- [Permissions](Permissions.md) - Access control
