# Relationships

## Relationship Types

| Type | Description | Example |
|------|-------------|---------|
| `belongs_to` | Many-to-one | Order belongs_to Customer |
| `has_one` | One-to-one | User has_one Profile |
| `has_many` | One-to-many | Customer has_many Orders |
| `has_many_and_belongs_to_many` | Many-to-many | Product has_many Categories |

## Defining Relationships

### Schema Definition

```yaml
Relations:
  - Subject: order
    Object: customer
    Relation: belongs_to
    SubjectName: orders
    ObjectName: customer

  - Subject: product
    Object: category
    Relation: has_many_and_belongs_to_many
```

### Relation Properties

| Property | Description |
|----------|-------------|
| `Subject` | Source entity |
| `Object` | Target entity |
| `Relation` | Relationship type |
| `SubjectName` | Name for reverse relation |
| `ObjectName` | Name for forward relation |

## belongs_to

Foreign key on the subject table.

```yaml
Relations:
  - Subject: post
    Object: user_account
    Relation: belongs_to
    ObjectName: author
```

Creates column `user_account_id` on `post` table.

### API Usage

```bash
# Create with relationship
curl -X POST http://localhost:6336/api/post \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "post",
      "attributes": {"title": "My Post"},
      "relationships": {
        "author": {"data": {"type": "user_account", "id": "USER_ID"}}
      }
    }
  }'

# Get post with author
curl "http://localhost:6336/api/post/POST_ID?include=author"
```

## has_one

One-to-one relationship.

```yaml
Relations:
  - Subject: user_account
    Object: profile
    Relation: has_one
```

### API Usage

```bash
# Get user with profile
curl "http://localhost:6336/api/user_account/USER_ID?include=profile"

# Get user's profile directly
curl "http://localhost:6336/api/user_account/USER_ID/profile"
```

## has_many

One-to-many relationship.

```yaml
Relations:
  - Subject: customer
    Object: order
    Relation: has_many
    SubjectName: customer
    ObjectName: orders
```

### API Usage

```bash
# Get customer with orders
curl "http://localhost:6336/api/customer/CUST_ID?include=orders"

# Get customer's orders directly
curl "http://localhost:6336/api/customer/CUST_ID/orders"

# With pagination
curl "http://localhost:6336/api/customer/CUST_ID/orders?page[size]=10"
```

## has_many_and_belongs_to_many

Many-to-many via junction table.

```yaml
Relations:
  - Subject: product
    Object: category
    Relation: has_many_and_belongs_to_many
```

Creates junction table: `product_{name}_has_category_{name}`

### API Usage

```bash
# Get product with categories
curl "http://localhost:6336/api/product/PROD_ID?include=category"

# Add category to product
curl -X POST http://localhost:6336/api/product_default_has_category_default \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product_default_has_category_default",
      "attributes": {
        "product_id": "PRODUCT_ID",
        "category_id": "CATEGORY_ID"
      }
    }
  }'

# Remove relationship
curl -X DELETE http://localhost:6336/api/product_default_has_category_default/JUNCTION_ID \
  -H "Authorization: Bearer $TOKEN"
```

## Loading Relationships

### Include Parameter

```bash
# Single include
curl "http://localhost:6336/api/order?include=customer"

# Multiple includes
curl "http://localhost:6336/api/order?include=customer,items"

# Nested includes
curl "http://localhost:6336/api/order?include=customer.address,items.product"
```

### Response Structure

```json
{
  "data": {
    "type": "order",
    "id": "order-123",
    "attributes": {...},
    "relationships": {
      "customer": {
        "data": {"type": "customer", "id": "cust-456"}
      }
    }
  },
  "included": [
    {
      "type": "customer",
      "id": "cust-456",
      "attributes": {
        "name": "John Doe",
        "email": "john@example.com"
      }
    }
  ]
}
```

## Relationship Endpoints

### Get Related Resources

```bash
# Get customer for order
curl "http://localhost:6336/api/order/ORDER_ID/customer"

# Get orders for customer
curl "http://localhost:6336/api/customer/CUST_ID/orders"
```

### Modify Relationships

```bash
# Update belongs_to relationship
curl -X PATCH http://localhost:6336/api/order/ORDER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "order",
      "id": "ORDER_ID",
      "relationships": {
        "customer": {"data": {"type": "customer", "id": "NEW_CUST_ID"}}
      }
    }
  }'

# Remove belongs_to (set to null)
curl -X PATCH http://localhost:6336/api/order/ORDER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "order",
      "id": "ORDER_ID",
      "relationships": {
        "customer": {"data": null}
      }
    }
  }'
```

## Self-Referential Relationships

```yaml
Relations:
  - Subject: category
    Object: category
    Relation: belongs_to
    ObjectName: parent
    SubjectName: children
```

```bash
# Get category with parent and children
curl "http://localhost:6336/api/category/CAT_ID?include=parent,children"
```

## Polymorphic Relationships

Not directly supported. Use separate relationships or a type column.

## Cascading Deletes

Configure cascade behavior in relation definition or handle via actions.
