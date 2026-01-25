# Audit Logging

Automatic history tracking for record changes using audit tables.

**Last Tested:** 2026-01-25 | **Status:** ✅ Fully functional | **Test Report:** `test-results/03-audit-logging.md`

## Overview

When `IsAuditEnabled: true` is set on a table, Daptin automatically:
- Creates a `{tablename}_audit` table with the same column structure
- Records a snapshot of each record **before** every UPDATE operation
- Tracks who made changes via `user_account_id`
- Links audit records to originals via `source_reference_id`
- Provides complete chronological history via API and SQL

## Enabling Audit Logging

### In Schema Definition

```yaml
Tables:
  - TableName: account
    IsAuditEnabled: true  # Enable audit logging
    DefaultPermission: 704385
    Columns:
      - Name: account_name
        DataType: varchar(200)
        ColumnType: label

      - Name: balance
        DataType: float(10,2)
        ColumnType: float
```

When Daptin loads this schema, it automatically creates:
- `account` table (main table)
- `account_audit` table (audit history)
- `account_account_id_has_usergroup_usergroup_id` (permissions)

## How Audit Logging Works

### Record Lifecycle

```
CREATE → No audit record
   ↓
UPDATE → Audit record created (stores OLD values)
   ↓
UPDATE → Another audit record (stores previous values)
   ↓
DELETE → Record removed (audit history preserved)
```

### What Gets Audited

**On UPDATE:**
1. Before updating the main record, Daptin creates an audit record
2. Audit record contains:
   - All column values **before** the update
   - `source_reference_id` linking to the original record
   - `user_account_id` of the user making the change
   - `created_at` timestamp of when the change happened
3. Main record then gets updated with new values
4. Main record's `version` field increments

**On CREATE:**
- No audit record created (nothing to audit yet)

**On DELETE:**
- Main record deleted
- Audit history preserved (not deleted)

## Audit Table Structure

For a main table:
```sql
CREATE TABLE account (
  id INTEGER PRIMARY KEY,
  version INTEGER,
  account_name varchar(200),
  balance float(10,2)
);
```

Auto-created audit table:
```sql
CREATE TABLE account_audit (
  -- System columns
  id INTEGER PRIMARY KEY,
  version INTEGER,
  created_at timestamp,
  reference_id BLOB,
  permission int(11),

  -- All columns from main table
  account_name varchar(200),
  balance float(10,2),

  -- Audit-specific columns
  user_account_id varchar,         -- Who made the change
  source_reference_id varchar(64)  -- Links to account.reference_id
);
```

## Complete Example

### 1. Create a Record

```bash
./scripts/testing/test-runner.sh post /api/account '{
  "data":{
    "type":"account",
    "attributes":{
      "account_name":"John Savings",
      "balance":1000.00
    }
  }
}'
```

**Response:**
```json
{
  "data": {
    "id": "019bf5d5-b8ee-7bb4-98b1-cb374cf5d4df",
    "attributes": {
      "account_name": "John Savings",
      "balance": 1000.0,
      "version": 1
    }
  }
}
```

**Audit Count:** 0 (no audit on CREATE)

### 2. First Update

```bash
curl -X PATCH "http://localhost:6336/api/account/019bf5d5-b8ee-7bb4-98b1-cb374cf5d4df" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"account","attributes":{"balance":1500.75}}}'
```

**Main Table After:**
```
account_name: John Savings
balance: 1500.75
version: 2
```

**Audit Table After:**
```
id: 1
account_name: John Savings
balance: 1000.00  ← OLD value
source_reference_id: 019bf5d5-b8ee-7bb4-98b1-cb374cf5d4df
created_at: 2026-01-25 15:46:23
```

### 3. Second Update

```bash
curl -X PATCH "http://localhost:6336/api/account/$ACCOUNT_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"data":{"type":"account","attributes":{"account_name":"John Updated","balance":2000.00}}}'
```

**Main Table After:**
```
account_name: John Updated
balance: 2000.00
version: 3
```

**Audit Table After:**
```
id: 1, account_name: John Savings, balance: 1000.00  ← After 1st update
id: 2, account_name: John Savings, balance: 1500.75  ← After 2nd update
```

### 4. Query Complete History

**Via API:**
```bash
curl "http://localhost:6336/api/account_audit?filter=source_reference_id||eq||019bf5d5-b8ee-7bb4-98b1-cb374cf5d4df&sort=created_at" \
  -H "Authorization: Bearer $TOKEN"
```

**Via SQL:**
```bash
sqlite3 daptin.db "SELECT id, created_at, account_name, balance FROM account_audit WHERE source_reference_id = '019bf5d5-b8ee-7bb4-98b1-cb374cf5d4df' ORDER BY created_at;"
```

**Result:** Complete chronological history of all changes

## Practical Use Cases

### 1. Compliance & Auditing

Track all changes to sensitive data for regulatory compliance:

```yaml
Tables:
  - TableName: patient_record
    IsAuditEnabled: true  # HIPAA compliance
    Columns:
      - Name: patient_name
        ColumnType: encrypted  # Encrypted + audited
      - Name: diagnosis
        ColumnType: content
```

Every update creates an audit trail showing who changed what and when.

### 2. Rollback/Undo

Restore previous values from audit table:

```sql
-- View previous states
SELECT created_at, account_name, balance
FROM account_audit
WHERE source_reference_id = 'ACCOUNT_ID'
ORDER BY created_at DESC;

-- Rollback to specific version
UPDATE account SET
  account_name = (SELECT account_name FROM account_audit WHERE id = 2),
  balance = (SELECT balance FROM account_audit WHERE id = 2),
  version = version + 1
WHERE reference_id = 'ACCOUNT_ID';
```

### 3. Change Detection

Find what changed between versions:

```sql
-- Compare consecutive audit records
SELECT
  a1.created_at as change_time,
  a1.balance as old_balance,
  a2.balance as new_balance,
  (a2.balance - a1.balance) as difference
FROM account_audit a1
LEFT JOIN account_audit a2 ON a2.id = a1.id + 1
WHERE a1.source_reference_id = 'ACCOUNT_ID';
```

### 4. Who Changed What

Track user activity:

```sql
SELECT
  a.created_at,
  u.email,
  a.account_name,
  a.balance as old_value,
  m.balance as new_value
FROM account_audit a
JOIN user_account u ON a.user_account_id = u.id
JOIN account m ON a.source_reference_id = hex(m.reference_id)
ORDER BY a.created_at DESC;
```

## Combining with State Machines

Use both features together for complete workflow tracking:

```yaml
Tables:
  - TableName: order
    IsStateTrackingEnabled: true  # Track workflow states
    IsAuditEnabled: true          # Track all changes
    Columns:
      - Name: customer_name
        ColumnType: label
      - Name: total
        ColumnType: float
```

Creates three tables:
- `order` - Main table
- `order_state` - Current workflow state
- `order_audit` - Complete change history

**Benefits:**
- State transitions audited
- Field changes audited
- Complete compliance trail

## API Operations

### List All Audit Records

```bash
GET /api/{tablename}_audit

# With filtering by source record
GET /api/{tablename}_audit?filter=source_reference_id||eq||RECORD_ID

# With sorting
GET /api/{tablename}_audit?sort=created_at&order=desc
```

### Query Specific Audit Record

```bash
GET /api/{tablename}_audit/AUDIT_ID
```

### Delete Audit Records (Admin Only)

```bash
DELETE /api/{tablename}_audit/AUDIT_ID
```

**Warning:** Deleting audit records removes history. Only do this for data cleanup/GDPR compliance.

## Storage Considerations

### Audit Table Growth

Audit tables grow with every UPDATE operation:

```
1 record × 10 updates = 10 audit records
1000 records × 10 updates each = 10,000 audit records
```

### Cleanup Strategies

**Archive old audits:**
```sql
-- Export audits older than 1 year
SELECT * FROM account_audit WHERE created_at < date('now', '-1 year');

-- Delete after archiving
DELETE FROM account_audit WHERE created_at < date('now', '-1 year');
```

**Selective auditing:**
Only enable for sensitive tables, not all tables:
```yaml
Tables:
  - TableName: financial_transaction
    IsAuditEnabled: true  # Critical data

  - TableName: user_preferences
    IsAuditEnabled: false  # Non-critical
```

## Permissions

Audit tables inherit permissions from the main table. Users need:
- **Read permission** on `{table}_audit` to view history
- **Delete permission** to remove audit records
- **Admin rights** recommended for audit access

## Limitations

1. **No automatic DELETE tracking** - Deleted records not preserved in audit (main record lost)
2. **No diff/changelog** - Must compare records manually to see what changed
3. **No size limits** - Audit tables grow indefinitely without cleanup
4. **No change descriptions** - No automatic "Reason for change" field

## Related

- [State Machines](State-Machines.md) - Combine for workflow + history
- [Permissions](Permissions.md) - Control audit record access
- [Data Export](Data-Actions.md) - Export audit trails for archival

---

**Tested:** ✅ All features verified 2026-01-25
