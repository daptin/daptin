# Daptin Documentation Guide

This guide captures effective techniques for understanding, testing, and documenting Daptin features.

## Core Patterns to Understand First

### 1. DaptinReferenceId (UUID System)

Every entity in Daptin has two IDs:
- **Numeric `id`**: Internal database primary key (e.g., `1`, `2`, `3`)
- **`reference_id`**: 16-byte UUID exposed externally (e.g., `019BEBDB52B673EF8D1A46F9511858B2`)

**Key insight**: Foreign key columns store the numeric ID but filters/queries often need the UUID reference_id.

```go
// Converting reference_id to string
daptinid.InterfaceToDIR(row["reference_id"]).String()

// The DaptinReferenceId type is a [16]byte wrapper
type DaptinReferenceId [16]byte
```

### 2. Transaction Management

Daptin uses SQLite with `MaxOpenConnections=1`, meaning:
- Only ONE active transaction at a time
- Uncommitted transactions block ALL other operations
- Always use `defer transaction.Commit()` after `Beginx()`

**Pattern to follow**:
```go
transaction, err := resource.Connection().Beginx()
if err != nil {
    return err
}
defer transaction.Commit()  // CRITICAL: Never forget this
```

### 3. Permission Model

Every request needs user context for permission checks:
```go
ctx := context.WithValue(context.Background(), "user", sessionUser)
httpRequest := httpRequest.WithContext(ctx)
```

Without user context, queries may return empty results due to permission filtering.

### 4. CRUD Resource Pattern

All tables are accessed through `DbResource` structs:
```go
dbResource["table_name"].FindOne(id, transaction)
dbResource["table_name"].GetAllObjectsWithWhere(...)
dbResource["table_name"].PaginatedFindAllWithoutFilters(request, transaction)
```

## Effective Documentation Techniques

### 1. Live Testing with Real Tools

**SMTP Testing** - Use `swaks`:
```bash
# Basic send test
swaks --to test@localhost --from sender@test.com \
  --server localhost --port 2525 \
  --auth LOGIN --auth-user test@test.com --auth-password password

# With TLS
swaks --to test@localhost --from sender@test.com \
  --server localhost --port 2525 --tls \
  --auth LOGIN --auth-user test@test.com --auth-password password
```

**IMAP Testing** - Use `openssl s_client`:
```bash
# Connect with STARTTLS
openssl s_client -connect localhost:1993 -starttls imap -crlf

# Then send IMAP commands:
a LOGIN user@test.com password
b LIST "" "*"
c SELECT INBOX
d SEARCH ALL
e FETCH 1 BODY[]
f LOGOUT
```

**Alternative: telnet for non-TLS**:
```bash
telnet localhost 1993
```

### 2. Add Debug Logging to Trace Flow

When a feature isn't working, add logging at entry/exit points:

```go
log.Printf("[FEATURE] FunctionName called with param=%v", param)
// ... function body ...
log.Printf("[FEATURE] FunctionName returning result=%v", result)
```

**Naming convention**: Use `[IMAP]`, `[SMTP]`, `[AUTH]` prefixes for easy filtering:
```bash
# Filter logs by feature
tail -f /tmp/daptin.log | grep "\[IMAP\]"
```

### 3. Database State Verification

Check database directly to verify state:
```bash
sqlite3 daptin.db

# Check table structure
.schema mail
.schema mail_box

# Verify data
SELECT id, reference_id, mail_box_id FROM mail;
SELECT id, reference_id, name FROM mail_box;

# Check foreign key relationships
SELECT m.id, m.subject, mb.name
FROM mail m
JOIN mail_box mb ON m.mail_box_id = mb.id;
```

### 4. Trace Code from Entry Points

For any feature, identify the entry point and follow the flow:

| Feature | Entry Point | Key Files |
|---------|-------------|-----------|
| SMTP | `server/resource/smtp_server.go` | smtp_server.go, mail_functions.go |
| IMAP | `server/resource/imap_backend.go` | imap_backend.go, imap_user.go, imap_mailbox.go |
| REST API | `server/resource/resource_*.go` | action handlers, CRUD methods |
| GraphQL | `server/resource/graphql_*.go` | schema generation, resolvers |
| OAuth | `server/resource/oauth_*.go` | oauth handlers, token management |

### 5. Understand the Data Model First

Before testing a feature, examine its tables:

```sql
-- Find all tables related to a feature
SELECT name FROM sqlite_master WHERE type='table' AND name LIKE '%mail%';

-- Check columns and types
PRAGMA table_info(mail);
PRAGMA table_info(mail_box);

-- Check foreign keys
PRAGMA foreign_key_list(mail);
```

### 6. Incremental Testing

Test features in order of dependency:

**Example for Mail**:
1. User authentication (prerequisite)
2. Mailbox creation/listing
3. Email storage (SMTP receive)
4. Email retrieval (IMAP fetch)
5. Email search/filter
6. Email operations (delete, move, copy)

### 7. Error Message Tracing

When you see an error, search for it in code:
```bash
# Find where error message originates
grep -r "error message text" server/

# Find function that generates error
grep -rn "failed to prepare" server/
```

## Common Pitfalls to Document

### 1. SQL Alias Issues (SQLite)

SQLite has quirks with column aliases:
```go
// BAD - can't scan result
Select("max(id)")

// GOOD - explicit alias
Select(goqu.L("COALESCE(MAX(id), 0)").As("max_id"))
```

### 2. Foreign Key Filter Values

When filtering by foreign key columns, use UUID not numeric ID:
```go
// BAD - will error "invalid value type in foreign key column"
queries := []Query{{ColumnName: "mail_box_id", Value: 1}}

// GOOD - use reference_id (UUID)
queries := []Query{{ColumnName: "mail_box_id", Value: mailBoxReferenceId}}
```

### 3. Empty Context = Empty Results

Missing user context causes permission filtering:
```go
// BAD - no user context, results filtered by permissions
httpRequest := &http.Request{}

// GOOD - include session user
ctx := context.WithValue(context.Background(), "user", sessionUser)
httpRequest = httpRequest.WithContext(ctx)
```

## Documentation Structure Template

For each feature, document:

```markdown
## Feature Name

### Overview
Brief description of what the feature does.

### Configuration
Required config in `config.json` or via API.

### Data Model
Tables involved, their relationships.

### Usage Examples
Concrete commands/code to use the feature.

### Troubleshooting
Common issues and solutions.

### Limitations
Known constraints or unsupported scenarios.
```

## Testing Checklist

Before marking a feature as documented:

- [ ] Tested basic happy path
- [ ] Tested with authentication
- [ ] Tested error conditions
- [ ] Verified data in database
- [ ] Checked logs for warnings/errors
- [ ] Tested after server restart
- [ ] Documented any bugs found (in GitHub issues)

## Useful Commands Reference

```bash
# Start Daptin with verbose logging
./daptin -log-level=debug

# Watch logs
tail -f /tmp/daptin.log

# Test HTTP endpoints
curl -X GET http://localhost:6336/api/tablename
curl -X POST http://localhost:6336/api/tablename -d '{"column":"value"}'

# Test with authentication
curl -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/tablename

# Check server status
curl http://localhost:6336/_health

# Database inspection
sqlite3 daptin.db ".tables"
sqlite3 daptin.db "SELECT * FROM tablename LIMIT 5"
```

## Key Source Files by Feature

| Feature | Primary Files |
|---------|--------------|
| Core CRUD | `server/resource/dbresource.go`, `resource_create.go`, `resource_read.go` |
| Authentication | `server/resource/user_auth.go`, `auth/auth.go` |
| Mail/SMTP | `server/resource/smtp_server.go`, `mail_functions.go` |
| Mail/IMAP | `server/resource/imap_*.go` (backend, user, mailbox) |
| Actions | `server/resource/action_*.go` |
| Cloud Storage | `server/resource/cloud_store.go` |
| OAuth | `server/resource/oauth_*.go` |
| GraphQL | `server/resource/graphql_*.go` |
| Permissions | `server/resource/permission.go` |
| State Machine | `server/resource/fsm*.go` |
