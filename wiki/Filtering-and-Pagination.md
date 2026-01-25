# Filtering and Pagination

Complete reference for querying, filtering, sorting, and paginating API requests in Daptin.

**All examples tested ✓** - See [GET-API-Test-Results](GET-API-Test-Results.md) for full test documentation.

---

## Pagination

### Query Parameters

| Parameter | Default | Description | Status |
|-----------|---------|-------------|--------|
| `page[number]` | 1 | Page number (1-indexed) | ✓ Working |
| `page[size]` | 10 | Records per page | ✓ Working |
| `page[after]` | - | Cursor-based: records after UUID | ⚠️ Bug (see below) |
| `page[before]` | - | Cursor-based: records before UUID | ⚠️ Bug (see below) |

### Pagination Response Structure

**IMPORTANT**: Pagination data is in `.links` NOT `.meta`:

```json
{
  "data": [...],
  "links": {
    "current_page": 1,
    "from": 0,
    "to": 10,
    "per_page": 10,
    "total": 95,
    "last_page": 10,
    "next_page_url": "//api/world?page[number]=2&page[size]=10",
    "last_page_url": "//api/world?page[number]=10&page[size]=10"
  }
}
```

### Offset Pagination Examples **Tested ✓**

**Default pagination** (10 per page):
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

**Custom page size**:
```bash
# Single quotes (recommended - preserves brackets)
curl 'http://localhost:6336/api/product?page[number]=2&page[size]=20' \
  -H "Authorization: Bearer $TOKEN"

# URL-encoded brackets (alternative)
curl "http://localhost:6336/api/product?page%5Bnumber%5D=2&page%5Bsize%5D=20" \
  -H "Authorization: Bearer $TOKEN"
```

Where `%5B` = `[` and `%5D` = `]`

### Cursor-Based Pagination **⚠️ BUG**

**Status**: NOT WORKING - `page[after]` and `page[before]` parameters have a bug.

**Expected**: `page[after]=UUID` should return records after that UUID
**Actual**: Returns all records, ignoring cursor

**Bug Location**: `server/resource/resource_findallpaginated.go` lines 477-495

The filter is only applied when there's an error (backwards logic):
```go
if err != nil {  // Should be: if err == nil
    queryBuilder = queryBuilder.Where(...)
}
```

**Do NOT use** cursor pagination until this bug is fixed.

---

## Filtering with Query Parameter

Daptin uses JSON-based query syntax with the `query` parameter.

### Query Structure

```json
[
  {
    "column": "column_name",
    "operator": "operator_name",
    "value": "filter_value",
    "logical_group": "optional_group_name"
  }
]
```

### Best Practice: Use curl --data-urlencode

**Recommended method** (works reliably across all shells):

```bash
curl --get \
  --data-urlencode 'query=[{"column":"is_hidden","operator":"is","value":"0"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world"
```

**Why?**
- Automatically URL-encodes the JSON
- Works in all shells (bash, zsh, fish)
- Handles special characters correctly

---

## Supported Operators

All operators tested ✓

### String Operators

| Operator | Wildcards Required | Description | Example |
|----------|-------------------|-------------|---------|
| `is` | No | Exact match | `"value":"Admin"` |
| `eq` | No | Exact match (alias) | `"value":"Admin"` |
| `neq` | No | Not equal | `"value":"Admin"` |
| `is not` | No | Not equal | `"value":"Admin"` |
| `contains` | **YES** `%value%` | Contains substring | `"value":"%mail%"` |
| `like` | **YES** | SQL LIKE pattern | `"value":"%user%"` |
| `ilike` | **YES** | Case-insensitive LIKE | `"value":"%USER%"` |
| `begins with` | **YES** `value%` | Starts with | `"value":"user%"` |
| `ends with` | **YES** `%value` | Ends with | `"value":"%_account"` |
| `not contains` | **YES** `%value%` | Does not contain | `"value":"%test%"` |
| `not like` | **YES** | NOT LIKE pattern | `"value":"%temp%"` |

**CRITICAL**: `contains`, `begins with`, `ends with` do NOT automatically add wildcards. You MUST add `%` manually.

### Numeric Operators

| Operator | Description |
|----------|-------------|
| `more than` / `gt` | Greater than |
| `less than` / `lt` | Less than |
| `after` | Greater than (alias) |
| `before` | Less than (alias) |

### Boolean Operators

| Operator | Description |
|----------|-------------|
| `is true` | Boolean true (1) |
| `is false` | Boolean false (0) |

### Null Operators

| Operator | Description |
|----------|-------------|
| `is empty` | IS NULL |

### Array Operators

| Operator | Status | Description |
|----------|--------|-------------|
| `in` | ✓ | Value in array |
| `any of` | ⚠️ | NOT WORKING - use `in` instead |
| `none of` | ⚠️ | NOT WORKING |

### Fuzzy Search Operators **Tested ✓**

| Operator | Description |
|----------|-------------|
| `fuzzy` | Single term fuzzy match |
| `fuzzy_any` | ANY keyword matches (OR) |
| `fuzzy_all` | ALL keywords match (AND) |

**Multi-column search**: Use comma-separated columns: `"column":"name,description"`

---

## Query Examples

All examples tested ✓

### Exact Match
```bash
curl --get \
  --data-urlencode 'query=[{"column":"table_name","operator":"is","value":"user_account"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world"
```

### Contains (with wildcards)
```bash
# IMPORTANT: Must add % wildcards manually
curl --get \
  --data-urlencode 'query=[{"column":"table_name","operator":"contains","value":"%mail%"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world"
```

### Begins With (with wildcards)
```bash
# IMPORTANT: Must add trailing %
curl --get \
  --data-urlencode 'query=[{"column":"table_name","operator":"begins with","value":"user%"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world"
```

### Ends With (with wildcards)
```bash
# IMPORTANT: Must add leading %
curl --get \
  --data-urlencode 'query=[{"column":"table_name","operator":"ends with","value":"%_account"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world"
```

### Numeric Comparison
```bash
# Greater than
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"more than","value":"100"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"

# Less than
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"less than","value":"200"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Boolean
```bash
# Is true
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is true","value":""}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"

# Is false
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is false","value":""}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Null Check
```bash
# Is NULL
curl --get \
  --data-urlencode 'query=[{"column":"description","operator":"is empty","value":""}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"

# Is NOT NULL
curl --get \
  --data-urlencode 'query=[{"column":"description","operator":"is not","value":""}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### In Array
```bash
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"in","value":["Product A", "Product B"]}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Fuzzy Search
```bash
# Single term
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"fuzzy","value":"Headphone"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
# Finds "Headphones" (fuzzy match)

# ANY keyword (OR)
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"fuzzy_any","value":"Watch Phone"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
# Finds products containing "Watch" OR "Phone"

# ALL keywords (AND)
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"fuzzy_all","value":"Wireless Pro"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
# Finds products containing BOTH "Wireless" AND "Pro"

# Multi-column fuzzy search
curl --get \
  --data-urlencode 'query=[{"column":"name,description","operator":"fuzzy","value":"Watch"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

---

## Multiple Conditions

### AND Logic (Default) **Tested ✓**

Multiple queries without `logical_group` are combined with AND:

```bash
# Find products where published=1 AND price>100
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"1"},{"column":"price","operator":"more than","value":"100"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### OR Logic (Using logical_group) **Tested ✓**

Queries with the **same** `logical_group` are combined with OR:

```bash
# Find products where name contains 'Smart' OR name contains 'Watch'
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"contains","value":"%Smart%","logical_group":"group1"},{"column":"name","operator":"contains","value":"%Watch%","logical_group":"group1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

**Rules**:
- Same `logical_group` value = OR
- Different `logical_group` values = separate OR groups, ANDed together
- No `logical_group` = AND with everything

### Mixed AND/OR Logic **Tested ✓**

```bash
# (published=0 OR price<140) AND name contains 'Test'
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"0","logical_group":"g1"},{"column":"price","operator":"less than","value":"140","logical_group":"g1"},{"column":"name","operator":"contains","value":"%Test%"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

**Logic**: `(g1_condition1 OR g1_condition2) AND ungrouped_condition`

---

## Sorting

Default sort: `-created_at` (newest first)

### Single Column **Tested ✓**

```bash
# Ascending (default)
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=name"

# Descending (prefix with -)
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=-price"

# Ascending (explicit + prefix)
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=+created_at"
```

### Multiple Columns **Tested ✓**

```bash
# Sort by published (asc), then price (desc), then name (asc)
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=published,-price,name"
```

**Direction prefixes**:
- No prefix or `+` = ascending
- `-` = descending

---

## Field Selection

Request only specific fields. System fields (`__type`, `permission`, `reference_id`, `user_account_id`) are always included.

### Examples **Tested ✓**

```bash
# Single field
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name"

# Multiple fields
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price,published"
```

**Performance Note**: Reduces response size but doesn't reduce database query.

---

## Including Relationships

**IMPORTANT**: Use `included_relations` NOT `include`

### Examples **Tested ✓**

```bash
# Single relation
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?included_relations=user_account_id"

# Multiple relations (use multiple parameters)
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?included_relations=user_account_id&included_relations=usergroup_id"
```

**Response structure**:
```json
{
  "data": [{...}],
  "included": [
    {
      "type": "user_account",
      "id": "UUID",
      "attributes": {...}
    }
  ],
  "links": {...}
}
```

**Notes**:
- Shared relations appear once (deduplication)
- Included records contain all fields
- Makes additional database queries

---

## Combined Examples

All combinations tested ✓

### Query + Sort + Pagination
```bash
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"more than","value":"100"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=-price&page%5Bsize%5D=10"
```

### Query + Fields + Sort
```bash
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price&sort=-price"
```

### All Features Combined
```bash
# Query (AND + OR) + Sort + Fields + Pagination + Included Relations
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"more than","value":"100"},{"column":"name","operator":"contains","value":"%Watch%","logical_group":"g1"},{"column":"name","operator":"contains","value":"%Headphones%","logical_group":"g1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price,published&sort=-price&page%5Bsize%5D=10&included_relations=user_account_id"
```

**This query**:
- Filters: `price > 100 AND (name contains Watch OR name contains Headphones)`
- Selects: Only name, price, published fields
- Sorts: By price descending
- Paginates: 10 per page
- Includes: User account data

---

## Performance Tips

1. **Use pagination** - Don't load all records at once (default: 10 per page)
2. **Select only needed fields** - Use `fields` parameter to reduce response size
3. **Filter early** - Apply query filters to reduce result set
4. **Index frequently filtered columns** - Set `IsIndexed: true` in schema
5. **Avoid cursor pagination** - Use offset pagination until cursor bug is fixed
6. **Use fuzzy search carefully** - Fuzzy search is slower than exact match

---

## Known Issues

1. **Cursor Pagination Bug**: `page[after]` and `page[before]` don't filter correctly (lines 477-495 in `resource_findallpaginated.go`)
2. **Operators `any of` and `none of`**: Not working - use `in` instead
3. **Invalid sort columns**: No error thrown, query succeeds but sorting may not apply
4. **Wildcard requirement**: `contains`, `begins with`, `ends with` require manual `%` wildcards

---

## See Also

- [GET API Test Results](GET-API-Test-Results.md) - Complete test documentation with all operators
- [CRUD Operations](CRUD-Operations.md) - Full API reference
- [Relationships](Relationships.md) - Loading related data
- [Core Concepts](Core-Concepts.md) - Understanding IDs and permissions
