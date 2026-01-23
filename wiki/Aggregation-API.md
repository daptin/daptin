# Aggregation API

SQL-like aggregation queries via REST API.

## Endpoint

```
GET /aggregate/{entity}
POST /aggregate/{entity}
```

## Basic Usage

```bash
curl "http://localhost:6336/aggregate/order?column=count" \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "data": [
    {
      "type": "aggregate_order",
      "attributes": {
        "count": 150
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

### Filter Examples

```bash
# Equals
curl "http://localhost:6336/aggregate/order?filter=eq(status,completed)&column=count"

# Greater than
curl "http://localhost:6336/aggregate/order?filter=gt(total,100)&column=count,sum(total)"

# Multiple filters (AND)
curl "http://localhost:6336/aggregate/order?filter=eq(status,completed)&filter=gte(total,50)&column=count"

# In list
curl "http://localhost:6336/aggregate/order?filter=in(status,pending,processing)&column=count"

# Date range
curl "http://localhost:6336/aggregate/order?filter=gte(created_at,2024-01-01)&filter=lt(created_at,2024-02-01)&column=count,sum(total)"
```

## Having Clause

Filter on aggregated values:

```bash
curl "http://localhost:6336/aggregate/order?group=customer_id&column=customer_id,count,sum(total)&having=gt(count,5)"
```

## POST Method

For complex queries, use POST:

```bash
curl -X POST http://localhost:6336/aggregate/order \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "group": ["status", "payment_method"],
    "column": ["status", "payment_method", "count", "sum(total)", "avg(total)"],
    "filter": [
      {"function": "gte", "column": "created_at", "value": "2024-01-01"},
      {"function": "lt", "column": "created_at", "value": "2024-02-01"}
    ],
    "having": [
      {"function": "gt", "column": "count", "value": 10}
    ]
  }'
```

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

## Limitations

- Maximum 1000 result rows
- Complex joins not supported
- Use GraphQL for more complex queries
