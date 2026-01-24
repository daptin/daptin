# Relationships

Define connections between tables to create linked data structures.

**Related**: [Core Concepts](Core-Concepts.md) | [Schema Definition](Schema-Definition.md) | [CRUD Operations](CRUD-Operations.md)

**Source of truth**: `server/resource/columns.go` (StandardRelations), `github.com/artpar/api2go/v2` (TableRelation)

---

## Relationship Types

| Type | Description | Creates | Example |
|------|-------------|---------|---------|
| `belongs_to` | Many-to-one | FK column on Subject | Order belongs_to Customer |
| `has_one` | One-to-one | FK column on Subject (nullable) | User has_one Profile |
| `has_many` | One-to-many | Join table | Customer has_many Orders |
| `has_many_and_belongs_to_many` | Many-to-many | Join table | Product has_many Categories |

---

## How Relationships Work

### belongs_to / has_one

Creates a foreign key column on the **Subject** table pointing to the **Object** table.

```
Subject Table                Object Table
┌─────────────────┐         ┌─────────────────┐
│ post            │         │ user_account    │
├─────────────────┤         ├─────────────────┤
│ id              │         │ id              │
│ title           │         │ name            │
│ user_account_id │────────>│ email           │
└─────────────────┘         └─────────────────┘
```

**Column naming**:
- Default: `{object}_id` (e.g., `user_account_id`)
- Custom: Use `ObjectName` to override (e.g., `author_id`)

**Nullability**:
- `belongs_to` → NOT NULL (required)
- `has_one` → NULL allowed (optional)
- Exception: `belongs_to user_account` or `usergroup` is always nullable

### has_many / has_many_and_belongs_to_many

Creates a **join table** to link both entities.

```
Subject Table         Join Table                    Object Table
┌──────────────┐     ┌──────────────────────┐     ┌──────────────┐
│ product      │     │ product_product_id   │     │ category     │
├──────────────┤     │ _has_category        │     ├──────────────┤
│ id           │<────│ _category_id         │     │ id           │
│ name         │     ├──────────────────────┤────>│ name         │
└──────────────┘     │ product_id           │     └──────────────┘
                     │ category_id          │
                     └──────────────────────┘
```

**Join table naming**: `{subject}_{subjectName}_has_{object}_{objectName}`

Example: `product_product_id_has_category_category_id`

---

## Defining Relationships

### Schema Definition

```yaml
Relations:
  # Basic belongs_to (creates order.customer_id)
  - Subject: order
    Object: customer
    Relation: belongs_to

  # belongs_to with custom column name (creates post.author_id instead of post.user_account_id)
  - Subject: post
    Object: user_account
    Relation: belongs_to
    ObjectName: author_id

  # Many-to-many (creates join table)
  - Subject: product
    Object: category
    Relation: has_many_and_belongs_to_many
```

### Relation Properties

| Property | Description | Default |
|----------|-------------|---------|
| `Subject` | Table that owns the relationship | Required |
| `Object` | Target table | Required |
| `Relation` | Relationship type | Required |
| `SubjectName` | Name used in join table for subject FK | `{subject}_id` |
| `ObjectName` | Name for the FK column (belongs_to/has_one) or join table object FK | `{object}_id` |

---

## Built-in Relationships (StandardRelations)

Daptin creates these relationships automatically between system tables:

| Subject | Relation | Object | Column Created |
|---------|----------|--------|----------------|
| `action` | belongs_to | `world` | `action.world_id` |
| `feed` | belongs_to | `stream` | `feed.stream_id` |
| `world` | has_many | `smd` | Join table |
| `oauth_token` | has_one | `oauth_connect` | `oauth_token.oauth_connect_id` |
| `data_exchange` | has_one | `oauth_token` | `data_exchange.oauth_token_id` |
| `data_exchange` | has_one | `user_account` | `data_exchange.as_user_id` |
| `timeline` | belongs_to | `world` | `timeline.world_id` |
| `cloud_store` | has_one | `credential` | `cloud_store.credential_id` |
| `site` | has_one | `cloud_store` | `site.cloud_store_id` |
| `mail_account` | belongs_to | `mail_server` | `mail_account.mail_server_id` |
| `mail_box` | belongs_to | `mail_account` | `mail_box.mail_account_id` |
| `mail` | belongs_to | `mail_box` | `mail.mail_box_id` |
| `task` | has_one | `user_account` | `task.as_user_id` |
| `calendar` | has_one | `collection` | `calendar.collection_id` |
| `user_otp_account` | belongs_to | `user_account` | `user_otp_account.otp_of_account` |

See: [SMTP Server](SMTP-Server.md) (mail relationships), [Cloud Storage](Cloud-Storage.md), [Two-Factor Auth](Two-Factor-Auth.md)

---

## belongs_to

Foreign key on the subject table. Required unless pointing to `user_account` or `usergroup`.

```yaml
Relations:
  - Subject: post
    Object: user_account
    Relation: belongs_to
    ObjectName: author_id  # Creates post.author_id (not user_account_id)
```

Creates column `author_id` on `post` table referencing `user_account.id`.

### API Usage

```bash
# Create with relationship (use FK column name as key)
curl -X POST http://localhost:6336/api/post \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "post",
      "attributes": {"title": "My Post"},
      "relationships": {
        "author_id": {"data": {"type": "user_account", "id": "USER_UUID"}}
      }
    }
  }'

# Get post with author included (use FK column name)
curl "http://localhost:6336/api/post/POST_ID?include=author_id"

# Get author via relationship endpoint
curl "http://localhost:6336/api/post/POST_ID/author_id"
```

---

## has_one

One-to-one relationship. FK column is nullable.

```yaml
Relations:
  - Subject: user_account
    Object: profile
    Relation: has_one
```

Creates column `profile_id` on `user_account` table.

### API Usage

```bash
# Get user with profile included (use FK column name)
curl "http://localhost:6336/api/user_account/USER_ID?include=profile_id"

# Get user's profile via relationship endpoint
curl "http://localhost:6336/api/user_account/USER_ID/profile_id"
```

---

## has_many

One-to-many via join table.

```yaml
Relations:
  - Subject: customer
    Object: order
    Relation: has_many
```

Creates join table: `customer_customer_id_has_order_order_id`

**Note**: Join tables are NOT directly accessible. Use relationship endpoints.

### API Usage

```bash
# Get customer's orders via relationship endpoint (use FK column name)
curl "http://localhost:6336/api/customer/CUST_ID/order_id"

# With pagination
curl "http://localhost:6336/api/customer/CUST_ID/order_id?page[size]=10"
```

---

## has_many_and_belongs_to_many

Many-to-many via join table.

```yaml
Relations:
  - Subject: product
    Object: category
    Relation: has_many_and_belongs_to_many
```

Creates join table: `product_product_id_has_category_category_id`

**Note**: Join tables are NOT directly accessible via REST API. Access relationships through parent entities.

### API Usage

```bash
# Get product's categories via relationship endpoint
curl "http://localhost:6336/api/product/PROD_ID/category_id"

# Add category to product via PATCH (adds to relationship array)
curl -X PATCH "http://localhost:6336/api/product/PROD_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "id": "PROD_ID",
      "relationships": {
        "category_id": {
          "data": [{"type": "category", "id": "CATEGORY_UUID"}]
        }
      }
    }
  }'

# To remove a relationship, you need to update with the remaining relationships
# or use the inverse relationship from the other entity
```

---

## Loading Relationships

### Include Parameter

Use `?include=` with the **FK column name** (not table name) to fetch related data:

```bash
# CORRECT: Use FK column name (mail_server_id, not mail_server)
curl "http://localhost:6336/api/mail_account?include=mail_server_id"

# Multiple includes (comma-separated)
curl "http://localhost:6336/api/mail_account?include=mail_server_id,user_account_id"
```

**Important**: The include parameter populates the `relationships` field in the response but does NOT populate a separate `included` array. Use the relationship endpoint to get full object data.

### Response Structure (JSON:API)

```json
{
  "data": {
    "type": "mail_account",
    "id": "019bec3e-0bcc-751c-80dd-74bddc39a218",
    "attributes": {
      "username": "test@example.com",
      "__type": "mail_account"
    },
    "relationships": {
      "mail_server_id": {
        "data": {"type": "mail_server", "id": "019bec3d-980c-7b13-886a-2d9f0dc73bc4"}
      },
      "user_account_id": {
        "data": {"type": "user_account", "id": "019bec91-c7ed-77fa-96b9-43562bcbad5c"}
      },
      "usergroup_id": {
        "data": []
      }
    }
  }
}
```

**Notes**:
- IDs are always `reference_id` UUIDs, not internal numeric IDs
- Relationship keys use the FK column name (e.g., `mail_server_id`)
- Empty relationships return `"data": []`
- See [Core Concepts](Core-Concepts.md) for more on reference IDs

---

## Relationship Endpoints

### Get Related Resources

Use the FK column name in the URL path:

```bash
# Get related object via belongs_to (use FK column name)
curl "http://localhost:6336/api/mail_account/ACCOUNT_UUID/mail_server_id"

# Get related objects via has_many (use FK column name)
curl "http://localhost:6336/api/user_account/USER_UUID/usergroup_id"

# With pagination
curl "http://localhost:6336/api/user_account/USER_UUID/usergroup_id?page[size]=10&page[number]=1"
```

### Understanding IDs in Relationship Responses

**Important**: When accessing via relationship endpoints, the IDs have special meaning:

```json
{
  "data": [{
    "type": "usergroup",
    "id": "019bec98-b05f-793b-9372-6db1543b301f",
    "attributes": {
      "name": "administrators",
      "reference_id": "019bec98-b05f-793b-9372-6db1543b301f",
      "relation_reference_id": "019bec3d-0d7a-79b4-87b6-fc5af7e147dd",
      "relation_created_at": "2026-01-23T20:38:29Z"
    }
  }]
}
```

| Field | Contains |
|-------|----------|
| `id` | Join table record's reference_id |
| `reference_id` | Same as `id` (join table record) |
| `relation_reference_id` | **Actual entity's reference_id** |
| `relation_created_at` | When the relationship was created |

**Use `relation_reference_id`** when you need the actual entity's ID for further operations.

### Modify Relationships

```bash
# Update belongs_to relationship (use FK column name)
curl -X PATCH http://localhost:6336/api/order/ORDER_UUID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "order",
      "id": "ORDER_UUID",
      "relationships": {
        "customer_id": {"data": {"type": "customer", "id": "NEW_CUST_UUID"}}
      }
    }
  }'

# Add to has_many relationship (array of objects)
curl -X PATCH http://localhost:6336/api/user_account/USER_UUID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "id": "USER_UUID",
      "relationships": {
        "usergroup_id": {
          "data": [{"type": "usergroup", "id": "GROUP_UUID"}]
        }
      }
    }
  }'

# Remove belongs_to (set to null) - only works if column is nullable
curl -X PATCH http://localhost:6336/api/order/ORDER_UUID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "order",
      "id": "ORDER_UUID",
      "relationships": {
        "customer_id": {"data": null}
      }
    }
  }'
```

---

## Self-Referential Relationships

A table can reference itself for hierarchical data:

```yaml
Relations:
  - Subject: category
    Object: category
    Relation: belongs_to
    ObjectName: parent_id
```

Creates `category.parent_id` → `category.id`

```bash
# Get category with parent
curl "http://localhost:6336/api/category/CAT_UUID?include=category"

# Find root categories (no parent)
curl "http://localhost:6336/api/category?filter[parent_id]=null"

# Find children of a category
curl "http://localhost:6336/api/category?filter[parent_id]=PARENT_UUID"
```

---

## Polymorphic Relationships

Not directly supported. Use one of these patterns:

**Option 1: Type column**
```yaml
Columns:
  - Name: commentable_type
    ColumnType: label
  - Name: commentable_id
    ColumnType: alias
```

**Option 2: Separate relationships**
```yaml
Relations:
  - Subject: comment
    Object: post
    Relation: belongs_to
  - Subject: comment
    Object: photo
    Relation: belongs_to
```

---

## Cascade Behavior

The `table_info.TableRelation` struct supports `OnDelete`:

| OnDelete | Behavior |
|----------|----------|
| `cascade` | Delete related records |
| `restrict` | Prevent delete if related records exist |
| `set_null` | Set FK to NULL |
| `set_default` | Set FK to default value |
| `no_action` | Database default |

**Note**: SQLite does not enforce foreign key constraints by default.

---

## Complete Example

```yaml
Tables:
  - TableName: blog_post
    Columns:
      - Name: title
        ColumnType: label
      - Name: content
        ColumnType: content

  - TableName: comment
    Columns:
      - Name: body
        ColumnType: content

  - TableName: tag
    Columns:
      - Name: name
        ColumnType: label
        IsUnique: true

Relations:
  # Each post belongs to an author
  - Subject: blog_post
    Object: user_account
    Relation: belongs_to
    ObjectName: author_id

  # Each comment belongs to a post
  - Subject: comment
    Object: blog_post
    Relation: belongs_to

  # Posts can have many tags (many-to-many)
  - Subject: blog_post
    Object: tag
    Relation: has_many_and_belongs_to_many
```

This creates:
- `blog_post.author_id` → `user_account.id`
- `comment.blog_post_id` → `blog_post.id`
- `blog_post_blog_post_id_has_tag_tag_id` join table

---

## See Also

- [Core Concepts](Core-Concepts.md) - Standard columns and entity model
- [Schema Definition](Schema-Definition.md) - Full schema syntax
- [CRUD Operations](CRUD-Operations.md) - API usage
- [Filtering and Pagination](Filtering-and-Pagination.md) - Query related data
