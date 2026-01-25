# Schema Definition - Getting Started

**Quick Start:** Define your database tables using YAML files. Daptin reads them on startup and creates everything automatically.

**Related:** [Complete Reference](Schema-Reference-Complete.md) | [Examples](Schema-Examples.md) | [Column Types](Column-Types.md) | [Relationships](Relationships.md)

---

## Your First Table (5 minutes)

Create `schema_product.yaml`:

```yaml
Tables:
  - TableName: product
    Columns:
      - Name: name
        DataType: varchar(500)
        ColumnType: label
        IsNullable: false

      - Name: price
        DataType: float(7,2)
        ColumnType: float
```

**Start Daptin:**
```bash
./daptin
```

**That's it!** You now have:
- ✅ `product` table in database
- ✅ Full CRUD API at `/api/product`
- ✅ Built-in permissions and versioning
- ✅ Auto-generated admin UI

**Test it:**
```bash
curl -X POST http://localhost:6336/api/product \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"product","attributes":{"name":"Widget","price":19.99}}}'
```

---

## What Gets Auto-Created

Every table automatically includes these system columns:

| Column | Type | Purpose |
|--------|------|---------|
| `id` | INTEGER | Primary key (auto-increment) |
| `reference_id` | BLOB | UUID for API references |
| `version` | INTEGER | Optimistic locking (starts at 1) |
| `created_at` | timestamp | Creation time |
| `updated_at` | timestamp | Last modification time |
| `permission` | int(11) | Row-level access control |
| `user_account_id` | int(11) | Record owner (FK to user_account) |

**You never define these** - Daptin adds them automatically.

---

## Essential Column Properties

Only 4 properties needed for basic columns:

```yaml
Columns:
  - Name: title              # Display name
    DataType: varchar(200)   # SQL type
    ColumnType: label        # Daptin type (see Column Types)
    IsNullable: false        # Required or optional
```

**That's it for 90% of columns!**

### When You Need More

```yaml
  - Name: email
    DataType: varchar(100)
    ColumnType: email
    IsNullable: false
    IsUnique: true           # Prevent duplicates
    IsIndexed: true          # Faster lookups
```

See [Column Properties Guide](Column-Properties-Guide.md) for all options.

---

## Next Steps

### ✅ Learn by Example

[Schema Examples](Schema-Examples.md) has 5 complete use cases:
1. Blog Platform - Posts, categories, comments
2. E-Commerce - Products, orders, workflow
3. Task Management - Projects with state tracking
4. Student Enrollment - Many-to-many with composite keys
5. Financial - Accounts with audit trail

### ✅ Browse Column Types

[Column Types](Column-Types.md) lists all 41 types:
- `label` - Short text (product name)
- `content` - Long text (blog post)
- `email` - Email with validation
- `float` - Decimal numbers (price)
- `datetime` - Timestamps
- And 36 more...

### ✅ Add Relationships

[Relationships](Relationships.md) shows how to link tables:
```yaml
Relations:
  - Subject: comment
    Object: post
    Relation: belongs_to
```

### ✅ Complete Reference

[Schema Reference Complete](Schema-Reference-Complete.md) documents all 18 TableInfo properties with test status.

### ✅ Full Walkthrough

`docs_source/walkthrough-product-catalog-with-permissions.md` is a complete tested example covering:
- Cloud storage
- Permissions
- User groups
- Custom actions

---

## Common Patterns

### Pattern 1: Basic CRUD Table

```yaml
Tables:
  - TableName: task
    Columns:
      - Name: title
        DataType: varchar(200)
        ColumnType: label
```

**Creates:** Basic table with CRUD API.

### Pattern 2: Table with Relationships

```yaml
Tables:
  - TableName: comment
    Columns:
      - Name: body
        DataType: text
        ColumnType: content
      - Name: post_id
        DataType: int(11)
        ColumnType: alias
        IsForeignKey: true

    Relations:
      - Subject: comment
        Object: post
        Relation: belongs_to
```

**Creates:** Foreign key from comment to post.

### Pattern 3: Table with Workflow

```yaml
Tables:
  - TableName: order
    IsStateTrackingEnabled: true
    Columns:
      - Name: customer_name
        DataType: varchar(200)
        ColumnType: name

StateMachineDescriptions:
  - Name: order_workflow
    InitialState: pending
    Events:
      - Name: confirm
        Src: [pending]
        Dst: confirmed
```

**Creates:** Order table + order_state table + workflow.

Note: State machine API currently has bugs ([#170](https://github.com/daptin/daptin/issues/170), [#171](https://github.com/daptin/daptin/issues/171)). Use SQL workaround from [State Machines](State-Machines.md).

### Pattern 4: Table with Audit Trail

```yaml
Tables:
  - TableName: account
    IsAuditEnabled: true
    Columns:
      - Name: balance
        DataType: float(10,2)
        ColumnType: float
```

**Creates:** Account table + account_audit table. Every update recorded.

---

## File Naming Convention

Daptin loads all `schema_*.yaml` files from the working directory:

```
your-app/
├── main.go
├── schema_users.yaml      ← Loaded
├── schema_content.yaml    ← Loaded
├── schema_commerce.yaml   ← Loaded
├── config.yaml            ← NOT loaded (missing schema_ prefix)
└── daptin.db
```

**Naming:**
- ✅ `schema_myapp.yaml`
- ✅ `schema_users.yaml`
- ❌ `myapp.yaml` (won't be loaded)
- ❌ `users_schema.yaml` (won't be loaded)

---

## Troubleshooting

### Tables Not Created

**Check:**
1. File named `schema_*.yaml`?
2. YAML syntax valid?
3. Restart Daptin after creating file?

**Verify:**
```bash
sqlite3 daptin.db ".tables"
```

### Can't Access via API

**Check:**
1. Table name 5-10 characters?
2. Stale Olric process killed?
3. Permissions correct?

**Fix:**
```bash
# Kill all processes including Olric
pkill -9 -f daptin
lsof -i :5336 -t | xargs kill -9 2>/dev/null || true
sleep 2
./daptin
```

### Permission Denied

**Check DefaultPermission** - Default (`2097151`) allows everything. For restricted access:

```yaml
Tables:
  - TableName: sensitive
    DefaultPermission: 16256  # Only owner can access
```

See [Permissions](Permissions.md) for permission calculator.

---

## What's Next?

- **Add more tables:** Create additional schema files
- **Add relationships:** Link tables with foreign keys
- **Add workflows:** Use IsStateTrackingEnabled for process automation
- **Add history:** Use IsAuditEnabled for compliance
- **Go multi-language:** Use TranslationsEnabled for i18n

---

**Last Updated:** 2026-01-25
**Test Coverage:** Based on test suites 1-8
