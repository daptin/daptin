# GET API Test Results

Complete testing results for all GET API features in Daptin.

**Server**: http://localhost:6336
**Test Date**: 2026-01-25
**Test Data**: Product table with 2 records

## Test Data Setup

```json
Products:
[
  {
    "id": "019bf540-7762-73b-a05a-04944227b8f0",
    "name": "Smart Watch Ultra",
    "price": 299.99,
    "published": 0
  },
  {
    "id": "019bf52a-4fc2-76f9-af2b-4793a379f306",
    "name": "Wireless Headphones Pro",
    "price": 149.99,
    "published": 1
  }
]
```

---

## Query Operators (OperatorMap - lines 1294-1320)

### 1. "is" operator (exact match)

**Test**: Find product with name "Smart Watch Ultra"

```bash
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"is","value":"Smart Watch Ultra"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

**Expected**: 1 result (Smart Watch Ultra)
**Result**: ✓ PASS - Found 1 result

---

## Operator Test Results Summary

### String Operators

| Operator | Works | Notes |
|----------|-------|-------|
| `is` | ✓ | Exact match |
| `eq` | ✓ | Alias for `is` |
| `neq` | ✓ | Not equals |
| `contains` | ✓ | Requires manual wildcards: `%value%` |
| `like` | ✓ | Explicit pattern with % wildcards |
| `ilike` | ? | Not tested (case-insensitive like) |
| `begins with` | ✓ | Requires manual wildcards: `value%` |
| `ends with` | ✓ | Requires manual wildcards: `%value` |
| `not contains` | ? | Not tested |
| `not like` | ? | Not tested |

**CRITICAL FINDING**: `contains`, `begins with`, `ends with` do NOT automatically add wildcards. You must add them manually:
- `contains`: Use `%value%`
- `begins with`: Use `value%`
- `ends with`: Use `%value`

### Numeric Operators

| Operator | Works | Notes |
|----------|-------|-------|
| `more than` / `gt` | ✓ | Greater than |
| `less than` / `lt` | ✓ | Less than |
| `after` | ✓ | Alias for `gt` |
| `before` | ✓ | Alias for `lt` |

### Boolean Operators

| Operator | Works | Notes |
|----------|-------|-------|
| `is true` | ✓ | Matches true/1 values |
| `is false` | ✓ | Matches false/0 values |

### Null Operators

| Operator | Works | Notes |
|----------|-------|-------|
| `is empty` | ✓ | Checks for NULL |
| `is not` | ✓ | Checks for NOT NULL |

### Array Operators

| Operator | Works | Notes |
|----------|-------|-------|
| `in` | ✓ | Value in array: `["val1", "val2"]` |
| `any of` | ✗ | Returned 0 results - needs investigation |
| `none of` | ✗ | Returned 0 results - needs investigation |

### Fuzzy Search Operators

| Operator | Works | Notes |
|----------|-------|-------|
| `fuzzy` | ✓ | Single term fuzzy match |
| `fuzzy_any` | ✓ | ANY keyword matches (OR) |
| `fuzzy_all` | ✓ | ALL keywords match (AND) |

**Supports multi-column search**: Use comma-separated column names: `"column":"name,description"`

---

## Complete Operator Examples

### Exact Match (is / eq)
```bash
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"is","value":"Smart Watch Ultra"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Not Equals (neq)
```bash
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"neq","value":"Smart Watch Ultra"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Contains (with wildcards)
```bash
# IMPORTANT: Must add % wildcards manually
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"contains","value":"%Watch%"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Begins With (with wildcards)
```bash
# IMPORTANT: Must add trailing % wildcard
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"begins with","value":"Wireless%"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Ends With (with wildcards)
```bash
# IMPORTANT: Must add leading % wildcard
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"ends with","value":"%Ultra"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Like Pattern
```bash
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"like","value":"%Headphones%"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Greater Than / More Than
```bash
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"more than","value":"200"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Less Than
```bash
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"less than","value":"200"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Is True
```bash
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is true","value":""}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Is False
```bash
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is false","value":""}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Is Empty (NULL check)
```bash
curl --get \
  --data-urlencode 'query=[{"column":"description","operator":"is empty","value":""}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Is Not (NOT NULL check)
```bash
curl --get \
  --data-urlencode 'query=[{"column":"description","operator":"is not","value":""}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### In Array
```bash
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"in","value":["Smart Watch Ultra", "Unknown Product"]}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Fuzzy Search (single term)
```bash
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"fuzzy","value":"Headphone"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
# Finds "Headphones" (fuzzy match)
```

### Fuzzy ANY (multiple keywords, OR logic)
```bash
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"fuzzy_any","value":"Watch Phone"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
# Finds products containing "Watch" OR "Phone"
```

### Fuzzy ALL (multiple keywords, AND logic)
```bash
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"fuzzy_all","value":"Wireless Pro"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
# Finds products containing BOTH "Wireless" AND "Pro"
```

### Multi-Column Fuzzy Search
```bash
curl --get \
  --data-urlencode 'query=[{"column":"name,description","operator":"fuzzy","value":"Watch"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
# Searches across both name and description columns
```

---

## Pagination Testing

### Pagination Response Structure

**IMPORTANT**: Pagination metadata is in `.links` NOT `.meta`:

```json
{
  "data": [...],
  "links": {
    "current_page": 1,
    "from": 0,
    "to": 10,
    "per_page": 10,
    "total": 2,
    "last_page": 1,
    "next_page_url": "//api/product?page[number]=2&page[size]=10",
    "last_page_url": "//api/product?page[number]=1&page[size]=10"
  }
}
```

### Offset Pagination Tests

| Test | Result | Notes |
|------|--------|-------|
| Default (no params) | ✓ | Default: page_size=10, page=1 |
| `page[size]=1` | ✓ | Returns 1 result per page |
| `page[number]=2` | ✓ | Returns second page (1-indexed) |
| `page[size]=0` | ✓ | Falls back to size=1 (safe default) |
| `page[size]=1000` | ✓ | Accepts large page sizes |

### Offset Pagination Examples

#### Default Pagination
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
# Default: 10 per page
```

#### Custom Page Size
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?page%5Bsize%5D=20"
```

#### Specific Page
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?page%5Bnumber%5D=2&page%5Bsize%5D=10"
```

### Cursor-Based Pagination

**Status**: ⚠️ NOT WORKING

`page[after]` and `page[before]` parameters do not filter results correctly. They return all results regardless of cursor value.

**Code Investigation** (lines 477-495 in resource_findallpaginated.go):
```go
if err != nil {  // BUG: Should be "if err == nil"
    queryBuilder = queryBuilder.Where(goqu.Ex{
        dbResource.TableInfo().TableName + ".id": goqu.Op{"gt": id},
    }).Limit(uint(pageSize))
}
```

The filter is only applied **when there's an error**, which is backwards logic.

**Expected behavior**:
- `page[after]=UUID` should return records **after** that UUID
- `page[before]=UUID` should return records **before** that UUID

**Actual behavior**: Returns all records, ignoring cursor

---

## Sorting Tests

### Sort Direction

| Syntax | Direction | Example |
|--------|-----------|---------|
| `sort=name` | Ascending | A to Z |
| `sort=-name` | Descending | Z to A |
| `sort=+name` | Ascending (explicit) | A to Z |

### Sorting Test Results

| Test | Result | Notes |
|------|--------|-------|
| Default (no sort) | ✓ | Default: `-created_at` (newest first) |
| `sort=name` | ✓ | Ascending alphabetical |
| `sort=-name` | ✓ | Descending alphabetical |
| `sort=+price` | ✓ | Ascending numeric |
| `sort=-price` | ✓ | Descending numeric |
| Multiple columns | ✓ | `sort=published,-price,name` |
| Invalid column | ⚠️ | No error, but sorting may not work |
| With query filter | ✓ | Combines correctly |

### Sorting Examples

#### Single Column Ascending
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=name"
```

#### Single Column Descending
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=-price"
```

#### Multiple Columns
```bash
# Sort by published (asc), then price (desc), then name (asc)
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=published,-price,name"
```

#### With Query Filter
```bash
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=-price"
```

### Sort Behavior Notes

1. **Default Sort**: If no `sort` parameter provided, defaults to `-created_at` (newest first)
2. **Multiple Columns**: Separate with commas, apply direction prefix to each: `sort=col1,-col2,+col3`
3. **Direction Prefixes**:
   - No prefix = ascending
   - `-` = descending
   - `+` = ascending (explicit)
4. **Invalid Columns**: Query succeeds but sorting may not apply (no error thrown)

---

## Field Selection Tests

### Field Selection Test Results

| Test | Result | Notes |
|------|--------|-------|
| Default (no fields) | ✓ | Returns all columns |
| `fields=name` | ✓ | Returns only name + system fields |
| `fields=name,price` | ✓ | Returns specified fields + system fields |
| Invalid field name | ✓ | Ignores invalid fields, no error |
| With query & sort | ✓ | Combines correctly |

### System Fields (Always Included)

These fields are **always** included regardless of `fields` parameter:
- `__type` - Table name
- `permission` - Record permission bits
- `reference_id` - Record UUID
- `user_account_id` - Owner user ID

### Field Selection Examples

#### Single Field
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name"
# Returns: __type, permission, reference_id, user_account_id, name
```

#### Multiple Fields
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price,published"
```

#### With Query and Sort
```bash
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price&sort=-price"
```

### Field Selection Behavior

1. **Comma-separated**: Multiple fields: `fields=field1,field2,field3`
2. **System Fields**: Always included for API functionality
3. **Invalid Fields**: Silently ignored (no error)
4. **Performance**: Reduces response size but doesn't reduce database query (still selects all columns)

---

## Included Relations Tests

### Parameter Name

**IMPORTANT**: Use `included_relations` NOT `include`
- ✓ `included_relations=user_account_id` - Works
- ✗ `include=user_account_id` - Does NOT work (JSON:API standard not supported)

### Included Relations Test Results

| Test | Result | Notes |
|------|--------|-------|
| No parameter | ✓ | Response has `data` and `links` only |
| `included_relations=user_account_id` | ✓ | Adds `included` array with user data |
| Multiple relations | ✓ | Use multiple `included_relations` params |
| `include=` (JSON:API) | ✗ | Not supported |

### Response Structure

With `included_relations`:
```json
{
  "data": [
    {
      "type": "product",
      "id": "UUID",
      "attributes": {...}
    }
  ],
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

### Included Relations Examples

#### Single Relation
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?included_relations=user_account_id"
```

#### Multiple Relations
```bash
# Use multiple included_relations parameters
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?included_relations=user_account_id&included_relations=usergroup_id"
```

#### With Query, Sort, and Fields
```bash
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price&sort=-price&included_relations=user_account_id"
```

### Included Relations Behavior

1. **Deduplication**: Shared related records appear once in `included` array
2. **Full Records**: Included records contain all fields (not affected by `fields` parameter)
3. **Performance**: Makes additional database queries for related records

---

## Complex Query Combinations

### AND Logic (Default)

Multiple query conditions without `logical_group` are combined with AND:

```bash
# Find products where published=1 AND price>100
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"1"},{"column":"price","operator":"more than","value":"100"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### OR Logic (Using logical_group)

Queries with the **same** `logical_group` are combined with OR:

```bash
# Find products where name contains 'Smart' OR name contains 'Test'
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"contains","value":"%Smart%","logical_group":"group1"},{"column":"name","operator":"contains","value":"%Test%","logical_group":"group1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

### Mixed AND/OR Logic

Combine multiple logical groups with ungrouped conditions:

```bash
# (published=0 OR price<140) AND name contains 'Test'
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"0","logical_group":"g1"},{"column":"price","operator":"less than","value":"140","logical_group":"g1"},{"column":"name","operator":"contains","value":"%Test%"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

**Logic**: `(g1_cond1 OR g1_cond2) AND ungrouped_cond`

### Query + Sort + Pagination

```bash
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"more than","value":"100"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?sort=-price&page%5Bsize%5D=2"
```

### Query + Fields + Sort

```bash
curl --get \
  --data-urlencode 'query=[{"column":"published","operator":"is","value":"0"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price&sort=-price"
```

### All Features Combined

```bash
curl --get \
  --data-urlencode 'query=[{"column":"price","operator":"more than","value":"100"},{"column":"name","operator":"contains","value":"%Watch%","logical_group":"g1"},{"column":"name","operator":"contains","value":"%Headphones%","logical_group":"g1"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product?fields=name,price,published&sort=-price&page%5Bsize%5D=3&included_relations=user_account_id"
```

**This combines**:
- Query with AND logic: `price > 100 AND (name contains Watch OR name contains Headphones)`
- Field selection: Only name, price, published
- Sort: Descending by price
- Pagination: 3 per page
- Included relations: Load user_account data

### Complex Query Test Results

| Combination | Result | Notes |
|-------------|--------|-------|
| Multiple queries (AND) | ✓ | All conditions must match |
| logical_group (OR) | ✓ | Same group = OR, different groups = AND |
| Query + Sort | ✓ | Filters then sorts |
| Query + Pagination | ✓ | Filters then paginates |
| Query + Fields | ✓ | Filters then selects fields |
| Query + Included | ✓ | Filters then loads relations |
| ALL combined | ✓ | All features work together |
| Fuzzy + Filters | ✓ | Fuzzy search with other conditions |
