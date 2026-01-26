# Daptin Documentation Guide

This guide captures the documentation process itself - challenges, techniques, and learnings that apply across all features.

---

## First: Fresh Database

**Every documentation session must start with a fresh database.**

```bash
# Stop Daptin, delete db, restart
rm daptin.db && go run main.go
```

Why:
- Fresh system has no admin (wide open permissions)
- Signup works for guests
- You can test the complete user journey
- No unknown credentials blocking you

**If you forget this, you'll waste time debugging 403 errors.**

---

## Getting Started with a Feature

### 1. Find What Exists

```bash
# Tables related to feature
SELECT name FROM sqlite_master WHERE type='table' AND name LIKE '%keyword%';

# Actions related to feature
SELECT action_name, world_id FROM action WHERE action_name LIKE '%keyword%';

# Code entry points
grep -rn "keyword" server/ --include="*.go" | head -20
```

### 2. Understand the Layers

Daptin has distinct layers - discovering which layer a feature lives in determines how to document it:

| Layer | How to Find | Documentation Focus |
|-------|-------------|---------------------|
| Tables | `columns.go`, database schema | Data model, relationships |
| Actions | `action` table, `columns.go` | REST endpoints, parameters |
| Performers | `server/actions/*.go` | Internal behavior, not user-callable |
| Config | `_config` table | Setup requirements |

### 3. Trace Dependencies

Features often depend on other features. Before testing:
- What tables must exist first?
- What config values are required?
- Does it need a restart to take effect?

---

## Critical: Don't Assume Features Are Broken

**When testing fails, investigate your testing approach before concluding the feature is broken.**

### Common False Negatives

These situations look like broken features but are actually testing problems:

1. **Wrong tool for the protocol**
   - Using generic HTTP tools for WebSocket protocols
   - Using curl for protocols that need persistent connections
   - Using text-based tools for binary protocols

2. **Reading only client errors**
   - Client shows error but server logs show success
   - Protocol mismatch between client and server expectations
   - Client library limitations not present in server

3. **Authentication method mismatch**
   - Using Bearer token in header when protocol needs query parameter
   - Using query parameter when protocol needs header
   - Not understanding how auth is extracted for that specific endpoint

4. **Missing context or prerequisites**
   - Testing WebSocket without understanding pub/sub model
   - Testing real-time features without knowing what topics exist
   - Testing without reading protocol specifications

### Systematic Investigation Process

When a feature appears broken:

```bash
# 1. Check server logs for actual response
tail -f /tmp/daptin.log | grep "GET\|POST\|ERROR"

# 2. Look for HTTP status in logs
# Status 200 = server accepted request
# Status 403/401 = auth problem
# Status 404 = endpoint doesn't exist

# 3. Search for working examples
# Check example applications (e.g., dadadash)
git clone https://github.com/daptin/dadadash
grep -r "feature_keyword" dadadash/

# 4. Check git history for usage patterns
cd dadadash
git log --all --full-history -p -- "**/*feature*"

# 5. Read authentication middleware
# Find how tokens are extracted for this endpoint
grep -rn "endpoint_path" server/
```

### Example: Feature Appeared Broken

**What happened**: Testing tool returned error message

**Initial conclusion**: ❌ "Feature is broken"

**Investigation revealed**:
- Server logs showed success (HTTP 200)
- Testing tool incompatible with protocol
- Auth mechanism different than assumed
- Used protocol-appropriate tool
- ✅ **Result**: Feature works perfectly

**Lesson**: Server logs reveal truth. Client error messages can be misleading.

---

## Process Challenges & Solutions

### Challenge: Feature works in code but not via API

**Cause**: Permission model filters results without user context.

**Solution**: Always test with authenticated user:
```bash
TOKEN=$(curl -s -X POST .../signin | jq -r '.[0].Attributes.value')
curl -H "Authorization: Bearer $TOKEN" ...
```

### Challenge: Can't tell if data was created

**Cause**: API returns success but permissions hide the record.

**Solution**: Check database directly:
```bash
sqlite3 daptin.db "SELECT * FROM tablename ORDER BY id DESC LIMIT 1;"
```

### Challenge: Action exists but does nothing

**Cause**: Actions call "performers" internally. The action may exist but performer may have conditions.

**Solution**: Read the performer code in `server/actions/action_*.go` to understand what it actually does.

### Challenge: Server must restart for changes

**Cause**: Some features initialize at startup only (SMTP daemon, IMAP server).

**Solution**: Document which features need restart vs runtime reload.

### Challenge: Foreign key errors on queries

**Cause**: Foreign key columns expect UUID reference_id, not numeric id.

**Solution**: Always use reference_id when filtering:
```go
// Wrong: queries by numeric id
// Right: queries by reference_id (UUID)
```

### Challenge: Transaction deadlocks

**Cause**: SQLite allows only one write transaction at a time.

**Solution**: Ensure all code paths commit/rollback transactions. Look for missing `defer tx.Commit()`.

### Challenge: Protocol-specific authentication

**Cause**: Different protocols extract authentication differently.

**Solution**: Read the middleware code for that endpoint:
```bash
# Find how endpoint extracts tokens
grep -A 20 "endpoint_path" server/websockets/*.go
grep -A 20 "endpoint_path" server/resource/*.go

# Common patterns:
# - REST API: Authorization: Bearer TOKEN header
# - WebSocket: ?token=TOKEN query parameter
# - GraphQL: Both header and query parameter supported
```

---

## Discovering Feature Behavior from Code

### Investigation Order (Most Effective First)

1. **Check example applications FIRST**
   - Look for official example repos (e.g., dadadash)
   - Search for feature usage in example app code
   - Check git history for commented-out or removed examples

2. **Read server logs SECOND**
   - See what's actually happening during requests
   - Cross-reference client errors with server responses
   - Identify which endpoints are being called

3. **Read implementation code THIRD**
   - Understand what the code actually does
   - Don't assume based on naming
   - Trace auth middleware and token extraction

4. **Test with appropriate tools FOURTH**
   - Use protocol-specific clients, not generic tools
   - WebSocket needs WebSocket library (not HTTP client)
   - Real-time features need persistent connections

5. **Document what you verified FIFTH**
   - Only document features you've tested
   - Include actual examples you ran
   - Note any prerequisites or gotchas

### Finding Usage Examples

**Example applications are gold mines:**

```bash
# Clone the example app
git clone https://github.com/daptin/dadadash

# Search for feature usage
grep -r "feature_keyword" dadadash/

# Check git history for removed/commented code
cd dadadash
git log --all --full-history -p -- "**/*feature*"

# Look for specific files that might use the feature
git log --all --full-history -- "**/*ComponentName*"
```

**Why git history matters:**
- Features may have been implemented then temporarily disabled
- Commented-out code shows correct usage patterns
- Commit messages explain why changes were made

### Finding Entry Points

| If documenting... | Start at |
|-------------------|----------|
| REST endpoint | `server/resource/resource_*.go` |
| Action | `server/actions/action_*.go` |
| Real-time features | `server/websockets/` or specialized subdirectories |
| Background process | `server/*.go` (look for goroutines) |
| Config handling | `server/resource/config_*.go` |

### Finding Relationships

```bash
# What tables reference this table?
grep -rn "tablename" server/resource/columns.go

# What actions use this performer?
grep -rn "performer.name" server/resource/columns.go

# What endpoints exist for a feature?
grep -rn "router.*endpoint_path" server/
```

### Finding Undocumented Behavior

Look for:
- `log.Printf` statements reveal hidden logic
- Switch statements show different code paths
- Error messages indicate failure conditions
- Protocol upgrade handlers (WebSocket, etc.)

### Cross-Reference Client and Server Perspectives

**Always check both sides when investigating:**

```bash
# Server side: What did server actually respond?
tail -f /tmp/daptin.log | grep "GET\|POST\|websocket"

# Look for HTTP status codes in logs:
# [GIN] 2026/01/26 | 200 |  1.234ms | GET "/live"
#                   ^^^
#                This is what actually happened

# Client side: What error did client show?
# "Unexpected server response: 403"
#
# If server logs show 200 but client shows 403:
# → Client tool is incompatible, not a server problem
```

**Protocol understanding matters:**

- Some protocols have handshakes or multi-step initialization
- Generic clients may not handle protocol-specific features
- Read protocol specifications to understand expected flow
- Use protocol-specific clients for accurate testing

---

## Documentation Depth Levels

### Level 1: Reference
- List of endpoints/actions
- Parameter names and types
- Quick start example

### Level 2: Guide
- Prerequisites and setup order
- Complete working examples
- Common errors and fixes

### Level 3: Complete
- Full lifecycle coverage
- Internal behavior explanation
- Edge cases and limitations
- Verified with tests

---

## After Documenting a Feature

Update this guide with:
- New challenges encountered
- New techniques that helped
- Patterns that might apply elsewhere
- Tools or commands that were useful

---

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

## Understanding Authentication Per Protocol

**Different protocols extract authentication tokens differently.**

### Different Protocols, Different Auth Mechanisms

**Key insight**: Each protocol/endpoint may extract authentication differently.

Common patterns:
- HTTP REST APIs typically use `Authorization: Bearer TOKEN` header
- Some protocols use query parameters (`?token=TOKEN`)
- Some support multiple extraction methods
- Protocol upgrade connections (like persistent connections) may authenticate once then maintain state

### Finding Auth Implementation

```bash
# For any endpoint, search for token extraction:
grep -A 20 "endpoint_path" server/

# Common patterns in code:
# - req.Header.Get("Authorization")
# - req.URL.Query().Get("token")
# - req.Context().Value("user")
```

### Testing Auth Flow

```bash
# 1. Get token
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

# 2. Save for reuse
echo "$TOKEN" > /tmp/daptin-token.txt

# 3. Test with different protocols
curl -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/world
wscat -c "ws://localhost:6336/live?token=$TOKEN"
```

---

## Effective Documentation Techniques

### 1. Use Protocol-Appropriate Testing Tools

**Critical principle**: Use tools designed for the protocol you're testing.

❌ **Wrong**: Using generic tools for specialized protocols
- Using HTTP clients for persistent connection protocols
- Using text-based tools for binary protocols
- Using basic tools that don't support protocol-specific auth

✅ **Right**: Using protocol-specific libraries
- SMTP: Use `swaks` or `telnet` (for text protocols)
- IMAP: Use `openssl s_client` or IMAP-specific clients
- Real-time protocols: Use libraries that maintain persistent connections
- Binary protocols: Use tools that handle binary data

**Why protocol-specific tools matter:**
- Handle protocol handshakes/upgrades correctly
- Support authentication mechanisms specific to that protocol
- Parse responses in the expected format
- Show full message exchange in protocol-native format
- Reveal whether issues are server-side or client-side

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

### 2. Cross-Reference Server Logs with Client Behavior

**Always check both sides when investigating issues.**

```bash
# Terminal 1: Watch server logs
tail -f /tmp/daptin.log

# Terminal 2: Run your test
curl http://localhost:6336/api/world
```

**What to look for in logs:**

```
[GIN] 2026/01/26 - 12:34:56 | 200 |    1.234ms |  127.0.0.1 | GET "/live"
                              ^^^
                         HTTP status code
```

**Interpreting status codes:**
- `200` = Success - server accepted request
- `401` = Unauthorized - missing/invalid token
- `403` = Forbidden - token valid but no permission
- `404` = Not Found - endpoint doesn't exist
- `500` = Server Error - something crashed

**Critical insight**: If logs show `200` but client shows error, the problem is with your client tool, not the server.

**Example scenario:**

```bash
# Client shows error
error: Unexpected server response: 403

# But server logs show success
[GIN] 2026/01/26 | 200 | GET "/endpoint"
                   ^^^
                 Actually succeeded!
```

**Conclusion**: Client tool doesn't handle protocol correctly. Use proper client library for that protocol.

### 3. Add Debug Logging to Trace Flow

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

# Filter out noise
tail -f /tmp/daptin.log | grep -v "become_an_administrator"
```

### 3. Getting Unblocked: Systematic Debugging

**When stuck, work through this checklist systematically:**

#### Step 1: Verify Server Is Running

```bash
# Check if process is running
lsof -i :6336 || echo "Server not running"

# Check if endpoint responds
curl http://localhost:6336/_health
```

#### Step 2: Check Logs for Actual Response

```bash
# Watch logs while testing
tail -f /tmp/daptin.log | grep "GET\|POST\|ERROR"

# Look for HTTP status in logs (not client error message)
```

#### Step 3: Test with Minimal Reproduction

```bash
# Remove complexity, test simplest case
curl http://localhost:6336/api/world

# If that works, add auth
curl -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/world

# If that works, test specific feature
```

#### Step 4: Search for Working Examples

```bash
# Check example applications
git clone https://github.com/daptin/dadadash
grep -r "feature_name" dadadash/

# Check git history for removed examples
cd dadadash
git log --all --full-history -p -- "**/*feature*"
```

#### Step 5: Read Auth Middleware for Protocol

```bash
# Find how this endpoint extracts tokens
grep -A 20 "endpoint_path" server/

# Common auth patterns:
# - REST: Authorization header
# - WebSocket: query parameter
# - GraphQL: both header and query param
```

#### Step 6: Use Appropriate Testing Tool

```bash
# Don't use curl for WebSocket
# Don't use wscat for complex protocols
# Don't use generic tools for specialized protocols

# Use protocol-specific libraries instead
```

#### Step 7: Document What Actually Works

```bash
# Only document what you've verified
# Include the actual commands you ran
# Note any prerequisites or gotchas
```

#### Common Blocking Patterns

**Pattern 1: Wrong Testing Tool**

❌ Blocked:
```
Using generic tool for specialized protocol
Tool doesn't support protocol-specific features
```

✅ Unblocked:
```
Use protocol-specific client library
Research appropriate tools for that protocol
```

**Lesson**: Match testing tool to protocol requirements.

**Pattern 2: Misreading Error Sources**

❌ Blocked:
```
Client shows error message
Conclusion: Server has bug
```

✅ Unblocked:
```
Check server logs - shows successful response
Conclusion: Client tool incompatible
```

**Lesson**: Server logs reveal truth, not client error messages.

**Pattern 3: Not Finding Usage Examples**

❌ Blocked:
```
No documentation found
Conclusion: Feature not implemented
```

✅ Unblocked:
```bash
# Check example applications
git clone https://github.com/daptin/dadadash
grep -r "feature_keyword" dadadash/

# Check git history for removed examples
git log --all --full-history -p -- "**/*feature*"
```

**Lesson**: Check git history and example apps for usage patterns.

**Pattern 4: Auth Mechanism Assumptions**

❌ Blocked:
```
Using standard auth method
Doesn't work for this endpoint
```

✅ Unblocked:
```bash
# Read middleware to find how auth is extracted
grep -A 20 "endpoint_path" server/
# Found different auth mechanism for this protocol
```

**Lesson**: Read auth middleware to understand token extraction per protocol.

**Pattern 5: Testing Without Context**

❌ Blocked:
```
Feature doesn't respond as expected
Conclusion: Feature is broken
```

✅ Unblocked:
```
Read protocol specification
Understand request/response model
Realize additional steps needed (subscribe, initialize, etc.)
```

**Lesson**: Understand the protocol model before testing.

### 4. Database State Verification

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
