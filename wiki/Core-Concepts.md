# Core Concepts

Daptin is a headless CMS that turns your data schema into a full-featured API. Understanding these foundational concepts is essential.

---

## The Entity Model

Every piece of data in Daptin lives in an **entity** (table). When you define a schema, Daptin automatically:

1. Creates the database table with [Standard Columns](#standard-columns)
2. Generates REST API endpoints (see [API Overview](API-Overview.md))
3. Generates GraphQL schema (see [GraphQL API](GraphQL-API.md))
4. Applies the [Permission Model](Permissions.md)
5. Tracks ownership and audit data

---

## Standard Columns

Every table automatically includes these columns - you don't define them:

| Column | Type | Description | API Visible |
|--------|------|-------------|-------------|
| `id` | INTEGER | Internal primary key, auto-increment | No |
| `reference_id` | BLOB (16 bytes) | External UUID identifier | Yes (as `id` in JSON:API) |
| `version` | INTEGER | Modification counter for optimistic locking | No |
| `created_at` | TIMESTAMP | Record creation time | Yes |
| `updated_at` | TIMESTAMP | Last modification time | Yes |
| `permission` | INT | Row-level permission bitmask | No |

**Key insight**: The `reference_id` (UUID) is what you use in API calls, not the internal `id`.

```bash
# API returns reference_id as "id"
curl http://localhost:6336/api/product/019BEBDB52B673EF8D1A46F9511858B2
```

See: [Permissions](Permissions.md) for permission bitmask details.

---

## Column Types

Daptin supports **41 built-in column types** that provide:
- **Validation** - Automatic input validation
- **Storage** - Appropriate SQL data types
- **GraphQL** - Correct type mapping
- **Fake data** - Test data generation

Full reference: [Column Type Reference](Column-Type-Reference.md)

### Quick Reference

| Category | Types |
|----------|-------|
| **Text** | `label`, `content`, `html`, `markdown`, `name`, `alias`, `hidden` |
| **Numeric** | `measurement`, `float`, `rating`, `value` |
| **Boolean** | `truefalse` |
| **Date/Time** | `datetime`, `date`, `time`, `timestamp`, `year`, `month`, `day`, `hour`, `minute` |
| **Identity** | `id`, `email`, `url`, `namespace` |
| **Security** | `password`, `bcrypt`, `md5`, `md5-bcrypt`, `encrypted` |
| **Location** | `location`, `location.latitude`, `location.longitude`, `location.altitude` |
| **Files** | `file`, `image`, `video`, `gzip` |
| **Data** | `json`, `enum`, `color` |

### Common Patterns

```yaml
# Text field
- Name: title
  DataType: varchar(100)
  ColumnType: label

# Numeric field
- Name: price
  DataType: int(10)
  ColumnType: measurement

# Boolean field
- Name: active
  DataType: boolean
  ColumnType: truefalse

# Email with validation
- Name: email
  DataType: varchar(100)
  ColumnType: email

# Encrypted sensitive data
- Name: api_secret
  DataType: text
  ColumnType: encrypted

# File storage
- Name: document
  DataType: blob
  ColumnType: file
```

See: [Column Types](Column-Types.md) for detailed usage.

---

## Schema Definition

Define your data model in JSON, YAML, or TOML:

```yaml
Tables:
  - TableName: product
    Icon: shopping-cart
    Columns:
      - Name: name
        DataType: varchar(100)
        ColumnType: label
        IsIndexed: true
      - Name: price
        DataType: int(10)
        ColumnType: measurement
```

### Column Properties

| Property | Type | Description |
|----------|------|-------------|
| `Name` | string | Column name (snake_case recommended) |
| `DataType` | string | SQL data type |
| `ColumnType` | string | Daptin column type (see above) |
| `IsNullable` | bool | Allow NULL values |
| `IsIndexed` | bool | Create database index |
| `IsUnique` | bool | Unique constraint |
| `DefaultValue` | string | Default value expression |
| `IsForeignKey` | bool | Foreign key column |

Full reference: [Schema Definition](Schema-Definition.md)

---

## Relationships

Connect entities with relationships:

```yaml
Relations:
  - Subject: order
    Object: user_account
    Relation: belongs_to

  - Subject: order
    Object: product
    Relation: has_many
```

| Type | Description | Creates |
|------|-------------|---------|
| `belongs_to` | Many-to-one | FK column on subject |
| `has_one` | One-to-one | FK column on subject |
| `has_many` | One-to-many | FK column on object |
| `has_many_and_belongs_to_many` | Many-to-many | Join table |

**Note**: Relationships automatically add `{relation}_id` columns referencing the target's `reference_id`.

See: [Relationships](Relationships.md)

---

## System Tables

Daptin includes built-in tables you can extend but not delete:

| Table | Purpose | Documentation |
|-------|---------|---------------|
| `user_account` | User authentication | [Users and Groups](Users-and-Groups.md) |
| `usergroup` | User groups | [Users and Groups](Users-and-Groups.md) |
| `world` | Table metadata | Internal |
| `action` | Action definitions | [Actions Overview](Actions-Overview.md) |
| `certificate` | TLS certificates | [TLS Certificates](TLS-Certificates.md) |
| `credential` | Stored credentials | [Cloud Storage](Cloud-Storage.md) |
| `cloud_store` | Cloud storage connections | [Cloud Storage](Cloud-Storage.md) |
| `site` | Static sites | [Subsites](Subsites.md) |
| `mail_server` | Mail servers | [SMTP Server](SMTP-Server.md) |
| `mail_account` | Mail accounts | [SMTP Server](SMTP-Server.md) |
| `mail_box` | Mailboxes | [IMAP Support](IMAP-Support.md) |
| `mail` | Emails | [IMAP Support](IMAP-Support.md) |
| `task` | Scheduled tasks | [Task Scheduling](Task-Scheduling.md) |
| `integration` | External integrations | [Integrations](Integrations.md) |
| `oauth_connect` | OAuth providers | [Authentication](Authentication.md) |

---

## Actions

Actions are operations on entities beyond CRUD:

```bash
# Call an action
curl -X POST http://localhost:6336/action/{entity}/{action_name} \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes": {...}}'
```

Actions can:
- Require authentication
- Operate on specific instances or globally
- Chain multiple operations
- Return various response types

See: [Actions Overview](Actions-Overview.md), [User Actions](User-Actions.md)

---

## Where to Go Next

1. **Setup** → [Installation](Installation.md), [Configuration](Configuration.md)
2. **Define Data** → [Schema Definition](Schema-Definition.md), [Column Types](Column-Types.md)
3. **Use API** → [CRUD Operations](CRUD-Operations.md), [Filtering and Pagination](Filtering-and-Pagination.md)
4. **Secure** → [Authentication](Authentication.md), [Permissions](Permissions.md)
5. **Extend** → [Actions Overview](Actions-Overview.md), [Custom Actions](Custom-Actions.md)

---

## Quick Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Your Application                          │
└─────────────────────────────────────────────────────────────┘
                              │
                    REST / GraphQL / WebSocket
                              │
┌─────────────────────────────────────────────────────────────┐
│                         Daptin                               │
│  ┌─────────────┬─────────────┬─────────────┬──────────────┐ │
│  │   Schema    │   Actions   │    Auth     │   Storage    │ │
│  │  (Tables,   │  (Performers)│  (JWT,      │  (Files,     │ │
│  │   Columns,  │             │   OAuth)    │   Cloud)     │ │
│  │   Relations)│             │             │              │ │
│  └─────────────┴─────────────┴─────────────┴──────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
                          Database
                    (SQLite / MySQL / PostgreSQL)
```
