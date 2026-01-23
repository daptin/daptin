# Filtering and Pagination

## Pagination

### Query Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `page[number]` | 1 | Page number (1-indexed) |
| `page[size]` | 10 | Records per page |
| `page[after]` | - | Cursor-based: records after UUID |
| `page[before]` | - | Cursor-based: records before UUID |

### Example

```bash
curl "http://localhost:6336/api/world?page[number]=2&page[size]=20" \
  -H "Authorization: Bearer $TOKEN"
```

### Response Links

```json
{
  "links": {
    "current_page": 2,
    "from": 20,
    "last_page": 5,
    "per_page": 20,
    "to": 40,
    "total": 95
  },
  "data": [...]
}
```

## Filtering with Query Parameter

Daptin uses JSON-based query syntax with the `query` parameter:

```bash
curl 'http://localhost:6336/api/world?query=[{"column":"is_hidden","operator":"is","value":"0"}]' \
  -H "Authorization: Bearer $TOKEN"
```

### Query Structure

```json
[
  {
    "column": "column_name",
    "operator": "operator_name",
    "value": "filter_value"
  }
]
```

### Supported Operators

From source code (`server/resource/resource_findallpaginated.go`):

| Operator | SQL Equivalent | Description |
|----------|----------------|-------------|
| `is` | `=` | Exact equality |
| `eq` | `=` | Exact equality |
| `is not` | `!=` | Not equal |
| `neq` | `!=` | Not equal |
| `contains` | `LIKE '%value%'` | Contains substring |
| `like` | `LIKE` | Pattern match |
| `ilike` | `ILIKE` | Case-insensitive pattern |
| `not contains` | `NOT LIKE` | Does not contain |
| `not like` | `NOT LIKE` | Pattern not match |
| `begins with` | `LIKE 'value%'` | Starts with |
| `ends with` | `LIKE '%value'` | Ends with |
| `before` | `<` | Less than |
| `less than` | `<` | Less than |
| `after` | `>` | Greater than |
| `more than` | `>` | Greater than |
| `in` | `IN` | Value in list |
| `any of` | `IN` | Value in list |
| `none of` | `NOT IN` | Value not in list |
| `is empty` | `IS NULL` | Null check |
| `is true` | `= true` | Boolean true |
| `is false` | `= false` | Boolean false |
| `fuzzy` | Fuzzy match | Single term fuzzy search |
| `fuzzy_any` | Fuzzy match | Any keyword matches |
| `fuzzy_all` | Fuzzy match | All keywords must match |

### Examples

**Filter by exact value:**
```bash
curl 'http://localhost:6336/api/world?query=[{"column":"table_name","operator":"is","value":"user_account"}]' \
  -H "Authorization: Bearer $TOKEN"
```

**Filter by multiple values (any of):**
```bash
curl 'http://localhost:6336/api/world?query=[{"column":"is_hidden","operator":"any of","value":"1,0"}]' \
  -H "Authorization: Bearer $TOKEN"
```

**Filter with contains:**
```bash
curl 'http://localhost:6336/api/world?query=[{"column":"table_name","operator":"contains","value":"mail"}]' \
  -H "Authorization: Bearer $TOKEN"
```

**Multiple conditions (AND):**
```bash
curl 'http://localhost:6336/api/world?query=[{"column":"is_hidden","operator":"is","value":"0"},{"column":"is_top_level","operator":"is","value":"1"}]' \
  -H "Authorization: Bearer $TOKEN"
```

**Fuzzy search:**
```bash
curl 'http://localhost:6336/api/document?query=[{"column":"document_name","operator":"fuzzy","value":"invoce"}]' \
  -H "Authorization: Bearer $TOKEN"
```

## Sorting

### Single Sort

```bash
# Ascending (default)
curl "http://localhost:6336/api/world?sort=table_name" \
  -H "Authorization: Bearer $TOKEN"

# Descending (prefix with -)
curl "http://localhost:6336/api/world?sort=-created_at" \
  -H "Authorization: Bearer $TOKEN"
```

### Multiple Sorts

```bash
curl "http://localhost:6336/api/world?sort=-created_at,table_name" \
  -H "Authorization: Bearer $TOKEN"
```

## Field Selection

Request specific fields with the `fields` parameter:

```bash
curl "http://localhost:6336/api/world?fields=table_name,icon" \
  -H "Authorization: Bearer $TOKEN"
```

## Including Relationships

Use `included_relations` parameter:

```bash
curl "http://localhost:6336/api/world?included_relations=action" \
  -H "Authorization: Bearer $TOKEN"
```

## Combined Example

```bash
curl 'http://localhost:6336/api/world?query=[{"column":"is_hidden","operator":"is","value":"0"}]&sort=-created_at&page[number]=1&page[size]=20&fields=table_name,icon' \
  -H "Authorization: Bearer $TOKEN"
```

## Default System Tables

Daptin creates these tables automatically:

| Table | Description |
|-------|-------------|
| `world` | Entity/table definitions |
| `action` | Available actions |
| `user_account` | User accounts |
| `usergroup` | User groups |
| `task` | Scheduled tasks |
| `timeline` | Audit/event log |
| `certificate` | TLS certificates |
| `document` | Documents |
| `calendar` | Calendar entries (iCal) |
| `credential` | Encrypted credentials |
| `feed` | RSS/Atom feed configs |
| `integration` | API integrations |
| `template` | Content templates |
| `collection` | Collections |
| `stream` | Data streams |
| `json_schema` | JSON schemas |

## Performance Tips

1. **Use pagination** - Don't load all records at once
2. **Select only needed fields** - Use `fields` parameter
3. **Filter early** - Apply query filters to reduce result set
4. **Index frequently filtered columns** - Set `IsIndexed: true` in schema
