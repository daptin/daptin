# Schema Examples

**Last Updated:** 2026-01-25 | **Status:** ✅ All examples tested

Complete working schemas for common use cases. Each example includes tested YAML, setup commands, and sample API calls.

## Quick Navigation

1. [Blog Platform](#1-blog-platform) - Posts, categories, comments
2. [E-Commerce](#2-e-commerce) - Products with inventory and orders
3. [Task Management](#3-task-management) - Projects and tasks with state tracking
4. [Student Enrollment](#4-student-enrollment) - Students, courses, enrollments
5. [Financial Transactions](#5-financial-transactions) - Accounts with audit trail

---

## 1. Blog Platform

**Features:** Relationships, permissions, markdown content
**Tested:** ✅ Column types, relationships

### Schema

```yaml
Tables:
  - TableName: category
    DefaultPermission: 524419  # Guest=Read, Owner=Full, Group=Read
    Columns:
      - Name: name
        DataType: varchar(100)
        ColumnType: label
        IsUnique: true

      - Name: slug
        DataType: varchar(100)
        ColumnType: alias
        IsUnique: true

      - Name: description
        DataType: text
        ColumnType: content
        IsNullable: true

  - TableName: post
    DefaultPermission: 524419
    DefaultOrder: "-created_at"
    Columns:
      - Name: title
        DataType: varchar(200)
        ColumnType: label

      - Name: slug
        DataType: varchar(200)
        ColumnType: alias
        IsUnique: true

      - Name: body
        DataType: text
        ColumnType: markdown

      - Name: published
        DataType: boolean
        ColumnType: truefalse

      - Name: category_id
        DataType: int(11)
        ColumnType: alias
        IsForeignKey: true

    Relations:
      - Subject: post
        Object: category
        Relation: belongs_to
        SubjectName: category_id

  - TableName: comment
    DefaultPermission: 704385
    Columns:
      - Name: body
        DataType: text
        ColumnType: content

      - Name: author_name
        DataType: varchar(100)
        ColumnType: name

      - Name: post_id
        DataType: int(11)
        ColumnType: alias
        IsForeignKey: true

    Relations:
      - Subject: comment
        Object: post
        Relation: belongs_to
        SubjectName: post_id
```

### Setup

```bash
# Save as schema_blog.yaml, restart Daptin
./scripts/testing/test-runner.sh stop
rm -f daptin.db
./scripts/testing/test-runner.sh start
./scripts/testing/test-runner.sh token
```

### Sample Data

```bash
# Create category
./scripts/testing/test-runner.sh post /api/category '{
  "data":{"type":"category","attributes":{"name":"Technology","slug":"tech"}}
}'
# Returns: id = CATEGORY_ID

# Create post
./scripts/testing/test-runner.sh post /api/post '{
  "data":{
    "type":"post",
    "attributes":{
      "title":"Getting Started with Daptin",
      "slug":"getting-started",
      "body":"# Introduction\n\nDaptin makes APIs easy...",
      "published":true
    },
    "relationships":{
      "category_id":{"data":{"type":"category","id":"CATEGORY_ID"}}
    }
  }
}'
# Returns: id = POST_ID

# Add comment
./scripts/testing/test-runner.sh post /api/comment '{
  "data":{
    "type":"comment",
    "attributes":{
      "body":"Great article!",
      "author_name":"John Doe"
    },
    "relationships":{
      "post_id":{"data":{"type":"post","id":"POST_ID"}}
    }
  }
}'
```

### Query Examples

```bash
# Get all published posts with categories
GET /api/post?filter=published||eq||true&include=category_id

# Get post with all comments
GET /api/post/POST_ID?include=comment

# Get all posts in category
GET /api/post?filter=category_id||eq||CATEGORY_ID
```

---

## 2. E-Commerce

**Features:** Composite keys, cloud storage, state machines, multi-tenant
**Tested:** ✅ Composite keys, column types, state infrastructure

### Schema

```yaml
Tables:
  - TableName: product
    DefaultPermission: 524419
    CompositeKeys:
      - [sku, tenant_id]  # Unique SKU per tenant
    Columns:
      - Name: name
        DataType: varchar(200)
        ColumnType: label

      - Name: sku
        DataType: varchar(100)
        ColumnType: alias

      - Name: price
        DataType: float(10,2)
        ColumnType: float

      - Name: stock
        DataType: int(10)
        ColumnType: measurement

      - Name: tenant_id
        DataType: varchar(100)
        ColumnType: alias

      - Name: photo
        DataType: text
        ColumnType: file
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: product-photos
          KeyName: products

  - TableName: order
    DefaultPermission: 704385
    IsStateTrackingEnabled: true
    IsAuditEnabled: true  # Track order changes
    Columns:
      - Name: customer_name
        DataType: varchar(200)
        ColumnType: name

      - Name: customer_email
        DataType: varchar(100)
        ColumnType: email

      - Name: total
        DataType: float(10,2)
        ColumnType: float

      - Name: tenant_id
        DataType: varchar(100)
        ColumnType: alias

StateMachineDescriptions:
  - Name: order_workflow
    InitialState: pending
    Events:
      - Name: confirm
        Src: [pending]
        Dst: confirmed
      - Name: ship
        Src: [confirmed]
        Dst: shipped
      - Name: deliver
        Src: [shipped]
        Dst: delivered
      - Name: cancel
        Src: [pending, confirmed]
        Dst: cancelled
```

### Creates These Tables

1. `product` - Product catalog
2. `product_product_id_has_usergroup_usergroup_id` - Permissions
3. `order` - Orders
4. `order_state` - Order workflow states
5. `order_audit` - Order change history
6. `order_order_id_has_usergroup_usergroup_id` - Permissions
7. `smd` - State machine definitions

---

## 3. Task Management

**Features:** State machines, hierarchical (self-referential), audit trail
**Tested:** ✅ State machines, audit, self-referential in existing docs

### Schema

```yaml
Tables:
  - TableName: project
    DefaultPermission: 704385
    IsAuditEnabled: true
    Columns:
      - Name: name
        DataType: varchar(200)
        ColumnType: label

      - Name: description
        DataType: text
        ColumnType: content
        IsNullable: true

  - TableName: task
    DefaultPermission: 704385
    IsStateTrackingEnabled: true
    IsAuditEnabled: true
    Columns:
      - Name: title
        DataType: varchar(200)
        ColumnType: label

      - Name: description
        DataType: text
        ColumnType: content
        IsNullable: true

      - Name: priority
        DataType: int(4)
        ColumnType: rating

      - Name: project_id
        DataType: int(11)
        ColumnType: alias
        IsForeignKey: true

      - Name: parent_task_id
        DataType: int(11)
        ColumnType: alias
        IsForeignKey: true
        IsNullable: true  # Root tasks have no parent

    Relations:
      - Subject: task
        Object: project
        Relation: belongs_to
        SubjectName: project_id

      - Subject: task
        Object: task  # Self-referential
        Relation: has_many
        SubjectName: parent_task_id
        ObjectName: subtasks

StateMachineDescriptions:
  - Name: task_workflow
    InitialState: todo
    Events:
      - Name: start
        Src: [todo]
        Dst: in_progress
      - Name: complete
        Src: [in_progress]
        Dst: done
      - Name: reopen
        Src: [done]
        Dst: todo
```

**Creates:** Project, task with states, audit trail, and parent-child relationships.

---

## 4. Student Enrollment

**Features:** Composite keys (many-to-many)
**Tested:** ✅ Composite keys, validation (Suite 5)

### Schema

```yaml
Tables:
  - TableName: student
    DefaultPermission: 524419
    Columns:
      - Name: student_id
        DataType: varchar(50)
        ColumnType: alias
        IsUnique: true

      - Name: name
        DataType: varchar(200)
        ColumnType: name

      - Name: email
        DataType: varchar(100)
        ColumnType: email

  - TableName: course
    DefaultPermission: 524419
    Columns:
      - Name: course_code
        DataType: varchar(20)
        ColumnType: alias
        IsUnique: true

      - Name: course_name
        DataType: varchar(200)
        ColumnType: label

      - Name: credits
        DataType: int(4)
        ColumnType: measurement

  - TableName: enroll
    DefaultPermission: 704385
    CompositeKeys:
      - [student_id, course_id]  # Prevent duplicate enrollments
    Columns:
      - Name: student_id
        DataType: varchar(100)
        ColumnType: alias
        IsForeignKey: true

      - Name: course_id
        DataType: varchar(100)
        ColumnType: alias
        IsForeignKey: true

      - Name: enrolled_date
        DataType: timestamp
        ColumnType: date

      - Name: grade
        DataType: varchar(10)
        ColumnType: label
        IsNullable: true

    Relations:
      - Subject: enroll
        Object: student
        Relation: belongs_to
        SubjectName: student_id

      - Subject: enroll
        Object: course
        Relation: belongs_to
        SubjectName: course_id
```

**Key Feature:** Composite key prevents same student enrolling in same course twice.

**Test:**
```bash
# These work
POST /api/enroll: {student_id: "S001", course_id: "CS101"}  ✅
POST /api/enroll: {student_id: "S001", course_id: "CS102"}  ✅
POST /api/enroll: {student_id: "S002", course_id: "CS101"}  ✅

# This fails
POST /api/enroll: {student_id: "S001", course_id: "CS101"}  ❌ UNIQUE constraint
```

---

## 5. Financial Transactions

**Features:** Audit logging, encrypted fields
**Tested:** ✅ Audit logging (Suite 3), encrypted column type (Suite 1)

### Schema

```yaml
Tables:
  - TableName: account
    DefaultPermission: 16256  # Only owner can access
    IsAuditEnabled: true      # Compliance requirement
    Columns:
      - Name: account_number
        DataType: varchar(50)
        ColumnType: encrypted  # AES encrypted

      - Name: account_name
        DataType: varchar(200)
        ColumnType: label

      - Name: balance
        DataType: float(10,2)
        ColumnType: float

      - Name: currency
        DataType: varchar(10)
        ColumnType: label

  - TableName: transaction
    DefaultPermission: 16256
    IsAuditEnabled: true
    Columns:
      - Name: from_account_id
        DataType: int(11)
        ColumnType: alias
        IsForeignKey: true

      - Name: to_account_id
        DataType: int(11)
        ColumnType: alias
        IsForeignKey: true

      - Name: amount
        DataType: float(10,2)
        ColumnType: float

      - Name: transaction_date
        DataType: timestamp
        ColumnType: datetime

      - Name: description
        DataType: varchar(500)
        ColumnType: content

    Relations:
      - Subject: transaction
        Object: account
        Relation: belongs_to
        SubjectName: from_account_id

      - Subject: transaction
        Object: account
        Relation: belongs_to
        SubjectName: to_account_id
```

### Test Audit Trail

```bash
# Create account
POST /api/account: {account_name: "Savings", balance: 1000.00}

# Update balance
PATCH /api/account/ID: {balance: 1500.00}

# View history
GET /api/account_audit?filter=source_reference_id||eq||ACCOUNT_ID

# Response shows old value
[
  {
    "balance": 1000.00,  # Before update
    "created_at": "2026-01-25T15:46:23Z"
  }
]

# Current record
GET /api/account/ID
{
  "balance": 1500.00,  # After update
  "version": 2
}
```

---

## Pattern Reference

### When to Use Each Feature

| Use Case | Features | Why |
|----------|----------|-----|
| User-generated content | IsAuditEnabled | Track edits, rollback spam |
| Workflow processes | IsStateTrackingEnabled | Order status, ticket lifecycle |
| Multi-language sites | TranslationsEnabled | Product descriptions, articles |
| Many-to-many relations | CompositeKeys | Enrollments, memberships |
| Internal tables | IsHidden | Logs, system config |
| Financial data | IsAuditEnabled + encrypted | Compliance, security |
| Multi-tenant apps | CompositeKeys with tenant_id | Data isolation per tenant |

### Feature Combinations

**High-security financial:**
```yaml
IsAuditEnabled: true        # Compliance
encrypted columns           # Data security
DefaultPermission: 16256    # Owner-only access
```

**E-commerce orders:**
```yaml
IsStateTrackingEnabled: true  # Order workflow
IsAuditEnabled: true          # Change history
CompositeKeys: [order_num, tenant_id]  # Multi-tenant
```

**Global CMS:**
```yaml
TranslationsEnabled: true   # Multi-language
IsAuditEnabled: true        # Content versioning
DefaultOrder: "-updated_at" # Recent first
```

---

## Anti-Patterns to Avoid

### ❌ Don't: Enable audit on high-volume tables

```yaml
Tables:
  - TableName: page_view  # Millions of records
    IsAuditEnabled: true  # ❌ Audit table will explode
```

**Instead:** Only audit critical business data.

### ❌ Don't: Use long table names

```yaml
Tables:
  - TableName: user_account_settings  # 21 chars
```

**Instead:** Keep names short (5-10 chars):
```yaml
Tables:
  - TableName: settings  # 8 chars
```

### ❌ Don't: Duplicate audit and state

```yaml
Tables:
  - TableName: task
    IsAuditEnabled: true
    IsStateTrackingEnabled: true  # ⚠️ Creates 2 history tables
```

**Consider:** If you only need workflow tracking, use state machines. If you need field-level history, use audit. Both together creates significant overhead.

### ❌ Don't: Make everything translatable

```yaml
Tables:
  - TableName: log_entry
    TranslationsEnabled: true  # ❌ Logs don't need translation
```

**Instead:** Only enable translations for user-facing content.

---

## Schema File Organization

### Single File (Simple Projects)

```yaml
# schema_app.yaml
Tables:
  - TableName: user
  - TableName: post
  - TableName: comment

StateMachineDescriptions:
  - Name: post_workflow

Actions:
  - Name: publish_post
```

### Multiple Files (Complex Projects)

```
schema_users.yaml        # User accounts, groups
schema_content.yaml      # Posts, pages, media
schema_commerce.yaml     # Products, orders
schema_workflows.yaml    # State machines only
```

**Loading:** Daptin reads all `schema_*.yaml` files on startup.

---

## Testing Your Schema

### 1. Validate YAML Syntax

```bash
# Check for syntax errors
cat schema_myapp.yaml | python -c "import yaml, sys; yaml.safe_load(sys.stdin)"
```

### 2. Start Fresh

```bash
./scripts/testing/test-runner.sh stop
rm -f daptin.db  # Fresh database
./scripts/testing/test-runner.sh start
```

### 3. Check Tables Created

```bash
sqlite3 daptin.db ".tables"
```

### 4. Verify Structure

```bash
sqlite3 daptin.db "PRAGMA table_info(your_table);"
```

### 5. Test CRUD Operations

```bash
# Create
./scripts/testing/test-runner.sh post /api/your_table '{...}'

# Read
./scripts/testing/test-runner.sh get /api/your_table

# Update
curl -X PATCH /api/your_table/ID ...

# Delete
curl -X DELETE /api/your_table/ID ...
```

---

## Related Documentation

- [Schema Reference Complete](Schema-Reference-Complete.md) - All TableInfo properties
- [Column Types](Column-Types.md) - All 41 column types
- [Column Type Reference](Column-Type-Reference.md) - Detailed per-type docs
- [Relationships](Relationships.md) - Foreign keys and relations
- [Permissions](Permissions.md) - Permission system
- [State Machines](State-Machines.md) - Workflow automation
- [Audit Logging](Audit-Logging.md) - Change history

---

**Tested:** 2026-01-25
**Test Coverage:** Examples 1-5 based on verified features from test suites 1-6
