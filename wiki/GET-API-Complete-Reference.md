# GET API Complete Reference

Comprehensive reference for Daptin's GET API based on systematic testing of `server/resource/resource_findallpaginated.go`.

**All features tested ✓** - January 25, 2026

---

## Quick Start

```bash
# Basic GET
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"

# With query filter
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"

# Full-featured query
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"more than","value":"100"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price&sort=-price&page%5Bsize%5D=20"
```

---

## URL Structure

```
http://localhost:6336/api/{table}?{parameters}
```

### Available Parameters

| Parameter | Purpose | Example |
|-----------|---------|---------|
| `query` | Filter records | `query=[{...}]` |
| `sort` | Order results | `sort=-created_at` |
| `page[number]` | Page number | `page[number]=2` |
| `page[size]` | Results per page | `page[size]=20` |
| `fields` | Select specific fields | `fields=name,email` |
| `included_relations` | Load related records | `included_relations=user_account_id` |

---

## Query Parameter

The `query` parameter filters records using JSON syntax.

### Query Structure

```json
[
  {
    "column": "field_name",
    "operator": "operator_name",
    "value": "filter_value",
    "logical_group": "optional_for_OR_logic"
  }
]
```

### URL Encoding

**ALWAYS use `curl --get --data-urlencode`** for reliable encoding:

```bash
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"is","value":"Product"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

---

## All Operators Reference

### String Operators

| Operator | Wildcards | Example Value | Matches |
|----------|-----------|---------------|---------|
| `is` | No | `"Admin"` | Exact: "Admin" |
| `eq` | No | `"Admin"` | Exact: "Admin" |
| `neq` / `is not` | No | `"Admin"` | Not "Admin" |
| `contains` | **YES** | `"%mail%"` | Substring: "email", "gmail" |
| `like` | **YES** | `"%user%"` | Pattern match |
| `ilike` | **YES** | `"%USER%"` | Case-insensitive |
| `begins with` | **YES** | `"user%"` | Starts with "user" |
| `ends with` | **YES** | `"%_account"` | Ends with "_account" |
| `not contains` | **YES** | `"%test%"` | Doesn't contain "test" |
| `not like` | **YES** | `"%temp%"` | NOT LIKE pattern |

**CRITICAL**: String pattern operators (`contains`, `begins with`, `ends with`) require manual `%` wildcards.

### Numeric Operators

| Operator | Description |
|----------|-------------|
| `more than` / `gt` / `after` | Greater than |
| `less than` / `lt` / `before` | Less than |

### Boolean Operators

| Operator | Matches | Value |
|----------|---------|-------|
| `is true` | Boolean true | Empty string or "1" |
| `is false` | Boolean false | Empty string or "0" |

### Null Operators

| Operator | Checks |
|----------|--------|
| `is empty` | IS NULL |
| `is not` | IS NOT NULL (when used with empty value) |

### Array Operators

| Operator | Status | Example |
|----------|--------|---------|
| `in` | ✓ Works | `"value":["A","B","C"]` |
| `any of` | ⚠️ Bug | Use `in` instead |
| `none of` | ⚠️ Bug | Not working |

### Fuzzy Search Operators

| Operator | Behavior |
|----------|----------|
| `fuzzy` | Single term fuzzy match |
| `fuzzy_any` | ANY keyword matches (OR) |
| `fuzzy_all` | ALL keywords match (AND) |

**Multi-column**: `"column":"field1,field2,field3"`

---

## Query Logic

### AND Logic (Default)

Queries without `logical_group` are ANDed:

```bash
# published=1 AND price>100
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"1"},{"column":"price","operator":"more than","value":"100"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### OR Logic (Same logical_group)

Queries with same `logical_group` are ORed:

```bash
# name contains 'Watch' OR name contains 'Phone'
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"contains","value":"%Watch%","logical_group":"g1"},{"column":"name","operator":"contains","value":"%Phone%","logical_group":"g1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Complex: (A OR B) AND C

```bash
# (published=0 OR price<140) AND name contains 'Test'
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"0","logical_group":"g1"},{"column":"price","operator":"less than","value":"140","logical_group":"g1"},{"column":"name","operator":"contains","value":"%Test%"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

---

## Sorting

Default: `-created_at` (newest first)

### Syntax

- No prefix or `+` = ascending
- `-` = descending
- Multiple: comma-separated

### Examples

```bash
# Single column ascending
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=name"

# Single column descending
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=-price"

# Multiple columns
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=published,-price,name"
```

**Note**: Invalid column names don't throw errors but sorting may not apply.

---

## Pagination

### Response Structure

Pagination metadata is in `.links`:

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
    "next_page_url": "//api/product?page[number]=2&page[size]=10",
    "last_page_url": "//api/product?page[number]=10&page[size]=10"
  }
}
```

### Offset Pagination

```bash
# Default (10 per page)
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"

# Custom page size
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?page%5Bsize%5D=20"

# Specific page
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?page%5Bnumber%5D=2&page%5Bsize%5D=20"
```

**Notes**:
- `page[number]` is 1-indexed (first page = 1)
- `page[size]=0` falls back to 1 (safe default)
- Use single quotes or URL-encode brackets

### Cursor Pagination **⚠️ BUG**

**Status**: NOT WORKING

`page[after]` and `page[before]` have a bug in `resource_findallpaginated.go:477-495`. They return all records instead of filtering.

**Do not use** until bug is fixed.

---

## Field Selection

Request specific fields. System fields are always included.

### Always Included (System Fields)

- `__type` - Table name
- `permission` - Permission bits
- `reference_id` - Record UUID
- `user_account_id` - Owner ID

### Examples

```bash
# Single field
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name"

# Multiple fields
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price,published"
```

---

## Including Relationships

Use `included_relations` to load related records.

**Parameter name**: `included_relations` (NOT `include`)

### Examples

```bash
# Single relation
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?included_relations=user_account_id"

# Multiple relations
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?included_relations=user_account_id&included_relations=usergroup_id"
```

### Response

Adds `included` array to response:

```json
{
  "data": [{...}],
  "included": [
    {
      "type": "user_account",
      "id": "UUID",
      "attributes": {
        "name": "Admin",
        "email": "admin@admin.com",
        ...
      }
    }
  ],
  "links": {...}
}
```

---

## Complete Examples

### Basic Query
```bash
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Query + Sort
```bash
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"more than","value":"100"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=-price"
```

### Query + Pagination
```bash
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"0"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?page%5Bsize%5D=20&page%5Bnumber%5D=1"
```

### Query + Fields
```bash
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"less than","value":"200"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price"
```

### All Parameters Combined
```bash
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"more than","value":"100"},{"column":"name","operator":"contains","value":"%Watch%","logical_group":"g1"},{"column":"name","operator":"contains","value":"%Headphones%","logical_group":"g1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price,published&sort=-price&page%5Bsize%5D=10&included_relations=user_account_id"
```

---

## Common Patterns

### Find User by Email
```bash
curl --get \
  --data-urlencode 'query=[{"column":"email","operator":"is","value":"user@example.com"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/user_account"
```

### Find Records Modified Today
```bash
# Get today's date
TODAY=$(date +%Y-%m-%d)

curl --get \
  --data-urlencode "query=[{\"column\":\"updated_at\",\"operator\":\"begins with\",\"value\":\"${TODAY}%\"}]" \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Search Across Multiple Fields
```bash
# Fuzzy search in name and description
curl --get \
  --data-urlencode 'query=[{"column":"name,description","operator":"fuzzy","value":"wireless audio"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Get Top 10 by Price
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=-price&page%5Bsize%5D=10"
```

### Get All World Tables (not hidden)
```bash
curl --get \
  --data-urlencode 'query=[{"column":"is_hidden","operator":"is","value":"0"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world?page%5Bsize%5D=100"
```

---

## Error Handling

### Invalid Operator
```json
{
  "errors": [{
    "detail": "table [product] invalid column query [nonexistent]",
    "status": "400"
  }]
}
```

### Invalid Column Name in Query
Returns error with column name.

### Invalid Column Name in Sort
No error - query succeeds but sort may not apply.

### Invalid Cursor (page[after]/page[before])
Due to bug, returns all records instead of error.

---

## Performance Optimization

### 1. Use Specific Queries
```bash
# Bad: Get all then filter client-side
curl "/api/product?page%5Bsize%5D=1000"

# Good: Filter server-side
curl --get --data-urlencode 'query=[...]' "/api/product"
```

### 2. Select Only Needed Fields
```bash
# Bad: Get all fields
curl "/api/product"

# Good: Select specific fields
curl "/api/product?fields=name,price"
```

### 3. Use Pagination
```bash
# Bad: Load 1000 records
curl "/api/product?page%5Bsize%5D=1000"

# Good: Paginate
curl "/api/product?page%5Bsize%5D=20"
```

### 4. Index Filtered Columns

In your schema YAML:
```yaml
Columns:
  - Name: email
    ColumnType: email
    IsIndexed: true  # Add index for filter performance
```

### 5. Avoid Fuzzy Search When Possible
```bash
# Faster: Exact match
query=[{"column":"name","operator":"is","value":"Product"}]

# Slower: Fuzzy search
query=[{"column":"name","operator":"fuzzy","value":"Product"}]
```

---

## Troubleshooting

### Query Returns Empty Even Though Data Exists

**Problem**: `query=[{"column":"table_name","operator":"is","value":"product"}]` returns no results

**Solutions**:
1. Check pagination - default is 10 records, use `page[size]=100` if you have more
2. Verify column name is correct (case-sensitive)
3. Check value is exact match (or use `contains` with wildcards)
4. Verify you have read permission on the records

### Contains / Begins With / Ends With Don't Work

**Problem**: `operator":"contains","value":"user"` returns no results

**Solution**: Add wildcards manually:
- `contains`: `"value":"%user%"`
- `begins with`: `"value":"user%"`
- `ends with`: `"value":"%account"`

### Cursor Pagination Not Working

**Problem**: `page[after]` returns all records

**Solution**: Use offset pagination instead:
```bash
# Instead of: page[after]=UUID
# Use: page[number]=2&page[size]=20
```

### "any of" Operator Returns Empty

**Problem**: `operator":"any of"` not working

**Solution**: Use `in` operator instead:
```bash
# Instead of: "operator":"any of"
# Use: "operator":"in","value":["A","B","C"]
```

---

## Real-World Examples

### E-commerce Product Search

```bash
# Search: (published=true) AND (price < 200) AND (name contains "wireless")
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is true","value":""},{"column":"price","operator":"less than","value":"200"},{"column":"name","operator":"contains","value":"%wireless%"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=-price&page%5Bsize%5D=20"
```

### User Management

```bash
# Find active users in specific groups
curl --get \
  --data-urlencode 'query=[{"column":"confirmed","operator":"is true","value":""},{"column":"email","operator":"not contains","value":"%test%"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/user_account?fields=name,email&sort=name"
```

### Admin Panel - Table List

```bash
# Get all visible tables, sorted by name
curl --get \
  --data-urlencode 'query=[{"column":"is_hidden","operator":"is","value":"0"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world?fields=table_name,icon&sort=table_name&page%5Bsize%5D=100"
```

---

## Code Reference

Implementation: `server/resource/resource_findallpaginated.go`

| Feature | Lines | Function |
|---------|-------|----------|
| Query parsing | 233-249 | Parse JSON query array |
| Filter fallback | 316-332 | Legacy filter support |
| Pagination | 225-230, 277-282 | Page number/size parsing |
| Cursor pagination | 477-495 | page[after]/page[before] (buggy) |
| Sorting | 301-312, 909-937 | Sort parameter parsing |
| Field selection | 264-274, 352-372 | Select specific columns |
| Included relations | 284-294 | Load related records |
| Operator map | 1294-1320 | All supported operators |
| Query filter processing | 1342-1467 | Apply filters to SQL |
| Fuzzy search | 1470-1740 | Fuzzy search implementation |
| Logical groups | 1751-1806 | OR logic via groups |

---

## Known Bugs

1. **Cursor Pagination** (lines 477-495): `if err != nil` should be `if err == nil`
2. **any of / none of**: Not implemented correctly - use `in` instead
3. **Wildcard Auto-Add**: `contains`, `begins with`, `ends with` don't add wildcards automatically

---

## See Also

- [GET API Test Results](GET-API-Test-Results.md) - Detailed test log with all operators
- [Filtering and Pagination](Filtering-and-Pagination.md) - User guide
- [CRUD Operations](CRUD-Operations.md) - Full CRUD API reference
- [Relationships](Relationships.md) - Working with related data
