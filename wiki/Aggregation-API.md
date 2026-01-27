# Aggregation API

**Tested ✓** 2026-01-27 (POST method and HAVING clause fixed ✅)

SQL-like aggregation queries via REST API.

**Status**:
- ✅ Basic aggregations (count, sum, avg, min, max) - Working
- ✅ GROUP BY - Working
- ✅ Filters (eq, not, lt, lte, gt, gte, in) - Working
- ✅ ORDER BY - Working
- ✅ POST method - Fixed in commit 9e3b5650 ([Issue #174](https://github.com/daptin/daptin/issues/174))
- ✅ HAVING clause - Fixed in commit e1c3017c ([Issue #173](https://github.com/daptin/daptin/issues/173))

## Endpoint

```
GET /aggregate/{entity}    ✅ Working
POST /aggregate/{entity}   ✅ Working (fixed in commit 9e3b5650)
```

Both GET and POST methods work identically. Use POST for complex queries that would exceed URL length limits.

## Quick Start (Tested)

### Count all records

```bash
curl "http://localhost:6336/aggregate/product?column=count" \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "data": [
    {
      "type": "aggregate_product",
      "id": "019bfa88-8fc8-7242-bafc-5ca3da2a34fe",
      "attributes": {
        "__type": "aggregate_product",
        "count": 5
      }
    }
  ]
}
```

### Multiple aggregates at once

```bash
curl "http://localhost:6336/aggregate/product?\
column=count,sum(price),avg(price),min(price),max(price)" \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "data": [
    {
      "type": "aggregate_product",
      "attributes": {
        "count": 5,
        "sum(price)": 1559.95,
        "avg(price)": 311.99,
        "min(price)": 29.99,
        "max(price)": 999.99
      }
    }
  ]
}
```

### Group by with aggregates

```bash
curl "http://localhost:6336/aggregate/product?\
group=category&\
column=category,count,avg(price)" \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "data": [
    {
      "type": "aggregate_product",
      "attributes": {
        "category": "Electronics",
        "count": 3,
        "avg(price)": 369.99
      }
    },
    {
      "type": "aggregate_product",
      "attributes": {
        "category": "Furniture",
        "count": 2,
        "avg(price)": 224.99
      }
    }
  ]
}
```

## Aggregate Functions

| Function | Syntax | Description |
|----------|--------|-------------|
| Count | `count` | Count records |
| Sum | `sum(column)` | Sum of values |
| Average | `avg(column)` | Average value |
| Minimum | `min(column)` | Minimum value |
| Maximum | `max(column)` | Maximum value |
| First | `first(column)` | First value |
| Last | `last(column)` | Last value |

### Examples

```bash
# Count all
curl "http://localhost:6336/aggregate/product?column=count"

# Sum prices
curl "http://localhost:6336/aggregate/order?column=sum(total)"

# Multiple aggregates
curl "http://localhost:6336/aggregate/product?column=count,avg(price),min(price),max(price)"
```

## Group By

```bash
# Group by single column
curl "http://localhost:6336/aggregate/order?group=status&column=status,count,sum(total)"

# Group by multiple columns
curl "http://localhost:6336/aggregate/order?group=status,payment_method&column=status,payment_method,count"
```

**Response:**
```json
{
  "data": [
    {"type": "aggregate_order", "attributes": {"status": "pending", "count": 25, "sum_total": 5000}},
    {"type": "aggregate_order", "attributes": {"status": "completed", "count": 100, "sum_total": 25000}},
    {"type": "aggregate_order", "attributes": {"status": "cancelled", "count": 10, "sum_total": 2000}}
  ]
}
```

## Filter Functions

| Function | Description |
|----------|-------------|
| `eq(col,val)` | Equals |
| `not(col,val)` | Not equals |
| `lt(col,val)` | Less than |
| `lte(col,val)` | Less than or equal |
| `gt(col,val)` | Greater than |
| `gte(col,val)` | Greater than or equal |
| `in(col,v1,v2)` | In list |
| `notin(col,v1,v2)` | Not in list |
| `is(col,null)` | Is null |
| `not(col,null)` | Is not null |

### Filter Examples (Tested ✓)

```bash
# Equals - products in Electronics category
curl "http://localhost:6336/aggregate/product?filter=eq(category,Electronics)&column=count,sum(price)"
# Result: count=3, sum=1109.97

# Greater than - products with price > 100
curl "http://localhost:6336/aggregate/product?filter=gt(price,100)&column=count,avg(price)"
# Result: count=3, avg=483.32

# Greater than or equal
curl "http://localhost:6336/aggregate/product?filter=gte(price,50)&column=count"
# Result: count=4

# Less than
curl "http://localhost:6336/aggregate/product?filter=lt(price,100)&column=count"
# Result: count=2

# Less than or equal
curl "http://localhost:6336/aggregate/product?filter=lte(price,100)&column=count,avg(price)"
# Result: count=2, avg=54.99

# Not equals
curl "http://localhost:6336/aggregate/product?filter=not(category,Electronics)&column=count"
# Result: count=2 (Furniture products)

# Multiple filters (AND) - Electronics AND price >= 50
curl "http://localhost:6336/aggregate/product?filter=eq(category,Electronics)&filter=gte(price,50)&column=count"
# Result: count=2

# In list - orders with status in (completed, pending)
curl "http://localhost:6336/aggregate/sales_order?filter=in(status,completed,pending)&column=count,sum(total)"
# Result: count=4, sum=1929.94
```

## Having Clause

**✅ Working** (Fixed in commit e1c3017c): HAVING clause now properly filters aggregated results.

Filter on aggregated values:

```bash
# Filter groups with more than 5 orders
curl "http://localhost:6336/aggregate/order?group=customer_id&column=customer_id,count,sum(total)&having=gt(count,5)"

# Multiple HAVING conditions
curl "http://localhost:6336/aggregate/order?group=status&column=status,count,sum(total)&having=gte(count,10)&having=gt(sum(total),1000)"
```

### HAVING Examples (Tested ✓)

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Groups with more than 1 user
curl "http://localhost:6336/aggregate/user_account?group=permission&column=permission,count&having=gt(count,1)" \
  -H "Authorization: Bearer $TOKEN"

# Categories with total price over $500
curl "http://localhost:6336/aggregate/product?group=category&column=category,count,sum(price)&having=gt(sum(price),500)" \
  -H "Authorization: Bearer $TOKEN"

# Average order value greater than $100
curl "http://localhost:6336/aggregate/order?group=customer_id&column=customer_id,count,avg(total)&having=gt(avg(total),100)" \
  -H "Authorization: Bearer $TOKEN"
```

## POST Method

**✅ Working** (Fixed in commit 9e3b5650): POST method now properly handles JSON request bodies for complex aggregation queries.

**Use POST for**:
- Complex queries with many parameters
- GROUP BY with multiple columns
- Large filter conditions that exceed URL length limits
- Better readability of complex queries

### POST Examples (Now Working)

```bash
# Simple count via POST
curl -X POST http://localhost:6336/aggregate/order \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "column": ["count"]
  }'
```

```bash
# Complex query via POST
curl -X POST http://localhost:6336/aggregate/order \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "group": ["status", "payment_method"],
    "column": ["status", "payment_method", "count", "sum(total)", "avg(total)"],
    "filter": ["eq(status,completed)"]
  }'
```

**Response** (same as GET method):
```json
{
  "data": [
    {
      "type": "aggregate_order",
      "attributes": {
        "status": "completed",
        "payment_method": "credit_card",
        "count": 50,
        "sum(total)": 15000.00,
        "avg(total)": 300.00
      }
    }
  ]
}
```

### POST vs GET Comparison

| Aspect | POST | GET |
|--------|------|-----|
| Readability | Better for complex queries | Simple for basic queries |
| URL length limit | No limit | Limited by URL length |
| Request body | JSON object | Query parameters |
| Response | Identical | Identical |
| Use case | Complex multi-parameter queries | Simple single-condition queries |

## Time-Based Aggregation

### By Date

```bash
# Daily totals
curl "http://localhost:6336/aggregate/order?group=date(created_at)&column=date(created_at),count,sum(total)"

# Monthly totals
curl "http://localhost:6336/aggregate/order?group=month(created_at),year(created_at)&column=month(created_at),year(created_at),count"
```

## Complete Examples

### Sales Dashboard

```bash
# Total sales by status
curl "http://localhost:6336/aggregate/order?\
group=status&\
column=status,count,sum(total),avg(total)&\
filter=gte(created_at,2024-01-01)"

# Top customers by order count
curl "http://localhost:6336/aggregate/order?\
group=customer_id&\
column=customer_id,count,sum(total)&\
having=gte(count,5)&\
sort=-count&\
limit=10"
```

### Product Analytics

```bash
# Products by category with stats
curl "http://localhost:6336/aggregate/product?\
group=category_id&\
column=category_id,count,avg(price),min(price),max(price)"

# Low stock products
curl "http://localhost:6336/aggregate/product?\
filter=lt(stock,10)&\
column=count"
```

### User Metrics

```bash
# Users by registration month
curl "http://localhost:6336/aggregate/user_account?\
group=month(created_at),year(created_at)&\
column=month(created_at),year(created_at),count"
```

## Response Format

```json
{
  "links": {
    "current_page": 1,
    "total": 5
  },
  "data": [
    {
      "type": "aggregate_{entity}",
      "attributes": {
        "group_column": "value",
        "count": 100,
        "sum_total": 5000.00,
        "avg_total": 50.00
      }
    }
  ]
}
```

## JOIN Support

Aggregation queries support LEFT JOIN operations for cross-table analysis.

### Join Syntax

```
join=table@eq(local_column,remote_table.remote_column)
```

### Join Examples

```bash
# Join orders with customers
curl "http://localhost:6336/aggregate/order?\
join=customer@eq(customer_id,customer.id)&\
column=count,sum(total),customer.name&\
group=customer.name"

# Multiple join conditions (AND)
curl "http://localhost:6336/aggregate/order?\
join=customer@eq(customer_id,customer.id)&eq(region,customer.region)&\
column=count,customer.name"
```

### Join with Reference IDs

Reference entity values in join conditions using `entity@uuid` format:

```bash
curl "http://localhost:6336/aggregate/order?\
join=customer@eq(customer_id,customer@abc-123-uuid)&\
column=count"
```

## Time-Based Filtering

```bash
# Filter by time range
curl "http://localhost:6336/aggregate/order?\
filter=gte(created_at,2024-01-01)&\
filter=lt(created_at,2024-02-01)&\
column=count,sum(total)"
```

## Limitations

- Maximum 1000 result rows
- Use GraphQL for more complex nested queries
