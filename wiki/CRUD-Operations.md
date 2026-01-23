# CRUD Operations

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
    "id": "01929123-abcd-7890-ef12-345678901234",
    "attributes": {
      "__type": "todo",
      "title": "Buy groceries",
      "completed": false,
      "reference_id": "01929123-abcd-7890-ef12-345678901234",
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z",
      "permission": 2097151,
      "version": 1
    }
  }
}
```

### Create with Relationships

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
        "category": {
          "data": {"type": "category", "id": "category-id"}
        }
      }
    }
  }'
```

## Read (GET)

### List All

```bash
curl http://localhost:6336/api/todo \
  -H "Authorization: Bearer $TOKEN"
```

### Get by ID

```bash
curl http://localhost:6336/api/todo/01929123-abcd-7890-ef12-345678901234 \
  -H "Authorization: Bearer $TOKEN"
```

### Include Relationships

```bash
curl "http://localhost:6336/api/todo/ID?include=category,user_account_id" \
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
      "category": {"data": {"type": "category", "id": "cat123"}}
    }
  },
  "included": [
    {
      "type": "category",
      "id": "cat123",
      "attributes": {"name": "Work"}
    }
  ]
}
```

## Update (PATCH)

```bash
curl -X PATCH http://localhost:6336/api/todo/01929123-abcd \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "01929123-abcd",
      "attributes": {
        "completed": true
      }
    }
  }'
```

### Partial Update

Only include fields you want to change:

```bash
curl -X PATCH http://localhost:6336/api/todo/ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "ID",
      "attributes": {
        "title": "Updated title"
      }
    }
  }'
```

### Update Relationships

```bash
curl -X PATCH http://localhost:6336/api/todo/ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "ID",
      "relationships": {
        "category": {
          "data": {"type": "category", "id": "new-category-id"}
        }
      }
    }
  }'
```

## Delete (DELETE)

```bash
curl -X DELETE http://localhost:6336/api/todo/01929123-abcd \
  -H "Authorization: Bearer $TOKEN"
```

**Response:** 204 No Content

### Cascade Delete

When deleting records with relationships, cascade behavior depends on relation configuration.

## Bulk Operations

### Bulk Create (via Import)

```bash
curl -X POST http://localhost:6336/action/todo/__data_import \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "table_name": "todo",
      "dump_file": [{
        "name": "todos.json",
        "file": "data:application/json;base64,W3sidGl0bGUiOiJUYXNrIDEifSx7InRpdGxlIjoiVGFzayAyIn1d"
      }]
    }
  }'
```

## Transactions

Wrap operations in transactions:

```bash
# Start transaction
curl -X POST http://localhost:6336/action/world/transaction \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes": {"action": "begin"}}'

# Perform operations...

# Commit
curl -X POST http://localhost:6336/action/world/transaction \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes": {"action": "commit"}}'

# Or rollback
curl -X POST http://localhost:6336/action/world/transaction \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes": {"action": "rollback"}}'
```

## Version Control

Each record has a `version` field that increments on update:

```json
{
  "attributes": {
    "version": 3
  }
}
```

## Timestamps

Automatic timestamps:
- `created_at` - Set on creation
- `updated_at` - Updated on each change

## Reference ID

Every record has a unique `reference_id` (UUID v7):
- Used as the `id` in API responses
- Globally unique across all tables
- Time-ordered for efficient indexing
