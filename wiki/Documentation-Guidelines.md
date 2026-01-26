# Documentation Guidelines for Daptin

Guidelines for documenting Daptin features accurately.

---

## Step Zero: Fresh Database

**Before ANY testing session, start fresh:**

```bash
# 1. Stop Daptin (Ctrl+C)
# 2. Delete the database
rm daptin.db

# 3. Restart Daptin
go run main.go
```

This is mandatory because:
- Existing admin locks down the system
- Signup becomes disabled
- You can't test the full user journey

**Never try to test with an existing database where you don't have credentials.**

---

## Golden Rule

**ALWAYS test against the running API before documenting.**

Reading code alone is insufficient. The code shows what performers exist, but:
- Action names may differ from performer names
- OnType (entity) may not be obvious
- Parameters may have different names than expected
- Response formats vary

## Critical: Don't Assume Features Are Broken

**When testing fails, verify your testing approach before concluding features are broken.**

Common mistakes that look like broken features:
- ❌ Using wrong testing tools (generic tools for specialized protocols)
- ❌ Reading only client errors, not checking server logs
- ❌ Assuming protocol works one way without verifying
- ❌ Not understanding authentication mechanisms for that protocol

**Process when features seem broken:**
1. Check server logs for actual HTTP status (not just client error)
2. Use protocol-appropriate testing tools
3. Search for real usage examples in example repositories
4. Read authentication middleware to understand token extraction
5. Test with minimal reproduction case
6. Ask for guidance if user says it should work

---

## Feature Discovery Process

### 1. Find Examples First, Then Test

**Order of investigation:**

1. **Check example applications** - Look for official example repos (e.g., dadadash)
   ```bash
   # Clone example repo
   git clone https://github.com/daptin/dadadash

   # Search for feature usage
   grep -r "WebSocket\|yjs\|feature_name" .

   # Check git history for removed/commented code
   git log --all --full-history -p -- "**/filename.vue"
   ```

2. **Read implementation code** - Understand what the code actually does
   ```bash
   # Find handler implementations
   grep -rn "endpoint_pattern\|HandlerFunc" server/

   # Trace authentication flow
   grep -rn "middleware\|AuthCheckMiddleware" server/auth/
   ```

3. **Check server logs** - See what's actually happening
   ```bash
   # Start server with logs visible
   go run main.go 2>&1 | tee /tmp/daptin.log

   # In another terminal, watch logs
   tail -f /tmp/daptin.log
   ```

4. **Test with appropriate tools** - Use protocol-specific clients
   - REST: `curl`
   - WebSocket: Node.js `ws` package, NOT wscat
   - GraphQL: GraphQL clients with proper transport
   - YJS: `y-websocket` provider

### 2. Cross-Reference Client and Server Perspectives

**Always check both sides:**

| You See | Check Server Logs For | Actual Meaning |
|---------|----------------------|----------------|
| Client 403 error | HTTP 200 response | Wrong client tool or protocol mismatch |
| Connection refused | Server startup errors | Server not running or wrong port |
| Timeout | Handler execution logs | Endpoint doesn't exist or wrong route |
| Empty response | Permission filter logs | User lacks read permission |
| Invalid response | Handler return values | Check actual response format in code |

**Example:**
```bash
# Terminal 1: Watch server logs
tail -f /tmp/daptin.log | grep -i "websocket\|live\|yjs"

# Terminal 2: Make test request
node test-connection.js

# Compare what client receives vs. what server sends
```

### 3. Understand Authentication at Protocol Level

**Different protocols, different auth mechanisms:**

```go
// Read server/auth/auth.go to understand token extraction
Extractor: jwtmiddleware.FromFirst(
    jwtmiddleware.FromAuthHeader,      // Authorization: Bearer TOKEN
    jwtmiddleware.FromParameter("token"), // ?token=TOKEN
    func(r *http.Request) (string, error) {
        cookie, e := r.Cookie("token")  // Cookie: token=TOKEN
        ...
    },
)
```

**Testing checklist:**
- [ ] Understand which token extraction methods are supported
- [ ] Know if middleware runs before your endpoint
- [ ] Check if protocol-specific handling exists (WebSocket upgrade)
- [ ] Verify token format matches what middleware expects

### 4. Search Git History for Usage Patterns

**When documentation is sparse:**

```bash
# Find when feature was added
git log --all --grep="websocket\|yjs\|feature_name" --oneline

# See implementation at that time
git show COMMIT_HASH

# Check if there were test files
git log --all --full-history -- "**/test*.js" "**/test*.go"

# Look for commented-out code (often contains working examples)
git log --all -p | grep -B5 -A10 "// const provider"
```

### 5. Read Error Messages Carefully

**Error messages reveal requirements:**

| Error | What It Really Means | What To Check |
|-------|---------------------|---------------|
| "Unauthorized" | Token invalid or missing | JWT format, expiration, signature |
| "Forbidden" | User lacks permission | Permission bits, group membership |
| "no reference id" | Wrong OnType for action | Check columns.go for correct OnType |
| "invalid value type in foreign key" | Using ID instead of UUID | Use reference_id, not numeric id |
| "Unexpected server response: 403" | Protocol-level issue | Check if using correct client for protocol |

---

## Session Setup: Become Admin First

Before testing any protected features, you MUST set up admin access.

### Why Admin is Required

1. **Permissions**: Many API operations require admin privileges
2. **"Refer object not allowed"** errors mean you lack permission, not that the API is wrong
3. **Until an admin exists**, all authenticated users have elevated access - but this is unreliable for testing

### Admin Setup Steps (VERIFIED)

```bash
# 1. Start fresh Daptin instance
go run main.go

# 2. Sign up a test user
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "name": "doctest",
      "email": "doctest@example.com",
      "password": "doctest123",
      "passwordConfirm": "doctest123"
    }
  }'

# 3. Sign in to get JWT token
RESPONSE=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"doctest@example.com","password":"doctest123"}}')

TOKEN=$(echo $RESPONSE | jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')
echo "Token: $TOKEN"

# 4. Become administrator
# CRITICAL: Action is on "world" table, NOT "user_account"
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'

# Response will show: {"ResponseType": "Restart", ...}
# Server restarts automatically

# 5. Wait for restart, then sign in again
sleep 5
RESPONSE=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"doctest@example.com","password":"doctest123"}}')

TOKEN=$(echo $RESPONSE | jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')
echo "Admin Token: $TOKEN"
```

### Common Mistakes

| Mistake | Error | Solution |
|---------|-------|----------|
| Wrong OnType | `no reference id` | Check `columns.go` for correct OnType |
| `/action/user_account/become_an_administrator` | `no reference id` | Use `/action/world/become_an_administrator` |
| Using old token after restart | `401 Unauthorized` | Re-signin after server restart |
| Not waiting for restart | Connection refused | Wait ~5 seconds |

### Verify Admin Status

```sql
-- Check if user is in administrators group
SELECT ua.name, ug.name as group_name
FROM user_account ua
JOIN user_account_user_account_id_has_usergroup_usergroup_id j ON ua.id = j.user_account_id
JOIN usergroup ug ON j.usergroup_id = ug.id
WHERE ug.name = 'administrators';
```

---

## Log Interpretation and Visibility

### Making Logs Useful

**Start server with full log visibility:**

```bash
# Option 1: Direct to terminal
go run main.go

# Option 2: Tee to file and terminal
go run main.go 2>&1 | tee /tmp/daptin.log

# Option 3: Background with log file
nohup go run main.go > /tmp/daptin.log 2>&1 &
```

**Filter logs for your feature:**

```bash
# Watch specific feature logs
tail -f /tmp/daptin.log | grep -i "websocket\|live\|feature_name"

# See errors only
tail -f /tmp/daptin.log | grep -i "error\|failed\|panic"

# See HTTP requests
tail -f /tmp/daptin.log | grep -i "GET\|POST\|PUT\|PATCH\|DELETE"
```

### What Server Logs Tell You

**HTTP Status in logs reveals truth:**

```
[GIN] 2026/01/26 - 12:34:56 | 200 |    1.234ms |  127.0.0.1 | GET "/live"
```

- Status 200 = Server accepted request successfully
- If client shows 403 but logs show 200, client tool is wrong

**Middleware execution order:**

```
[Auth] Checking token from query parameter
[Auth] Token validated for user: admin@admin.com
[WebSocket] Client connected
```

Shows authentication succeeded before WebSocket handler

**Missing logs mean:**
- Handler never called (wrong endpoint)
- Middleware blocked request (returns early)
- Server crashed (check for panic)

### Common Log Patterns

| Log Pattern | Meaning | Action |
|-------------|---------|--------|
| `HTTP 200` + client error | Wrong client tool | Use proper protocol client |
| No logs for request | Wrong endpoint or port | Verify URL and port |
| Auth logs + 403 | Permission issue | Check user permissions |
| Handler logs + error | Feature issue | Check handler implementation |
| Panic + stack trace | Server crash | Fix the crash first |

### Using Logs to Verify Features

**Pattern: Feature investigation**

```bash
# Terminal 1: Watch logs
tail -f /tmp/daptin.log | grep -i "feature_name"

# Terminal 2: Test feature
curl http://localhost:6336/api/feature

# Terminal 1 should show:
# - HTTP status
# - Handler execution
# - Any errors
```

**If logs show nothing:**
- Feature endpoint doesn't exist at that URL
- Server not handling that route
- Check route registration in code

**If logs show errors:**
- Feature exists but has issues
- Error message guides next investigation step

---

## Understanding Daptin's Action System

### Two Levels

1. **Actions** - REST API endpoints users call
   - Defined in `server/resource/columns.go` (SystemActions array)
   - URL pattern: `/action/{OnType}/{ActionName}`
   - Example: `POST /action/user_account/signin`

2. **Performers** - Internal executors
   - Defined in `server/actions/` directory
   - NOT directly callable via REST
   - Used in action OutFields
   - Example: `mail.send` performer used by password reset action

### Finding the Correct Action

```bash
# List all actions for a table
curl -s "http://localhost:6336/api/action" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.data[] | select(.attributes.action_name | test("SEARCH_TERM"))'

# Find which table an action belongs to
curl -s "http://localhost:6336/api/action" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.data[] | select(.attributes.action_name == "ACTION_NAME") | .relationships.world_id'
```

### Action Properties (from columns.go)

| Property | Description |
|----------|-------------|
| `Name` | Action name used in URL |
| `OnType` | Table the action belongs to |
| `InstanceOptional` | If true, no entity ID required |
| `InFields` | Input parameters |
| `OutFields` | Performers to execute |

---

## Common Documentation Bugs

### Bug Pattern 1: Action vs Performer Confusion

**Wrong**: Documenting `mail.send` as a REST action
```bash
# WRONG - mail.send is a performer, not an action
curl -X POST http://localhost:6336/action/world/mail.send
```

**Right**: Document what actions USE the performer
```
mail.send is an internal performer used by:
- Password reset flows
- Custom actions that send email
```

### Bug Pattern 2: Wrong OnType

**Wrong**: Assuming all admin actions are on `user_account`
```bash
# WRONG
curl -X POST http://localhost:6336/action/user_account/become_an_administrator
```

**Right**: Check `columns.go` for actual OnType
```bash
# RIGHT - OnType is "world"
curl -X POST http://localhost:6336/action/world/become_an_administrator
```

### Bug Pattern 3: Action Name Mismatch

**Wrong**: Using performer name as action name
```bash
# WRONG - otp.generate is performer name
curl -X POST http://localhost:6336/action/user_account/otp.generate
```

**Right**: Use actual action name
```bash
# RIGHT - register_otp is action name
curl -X POST http://localhost:6336/action/user_account/register_otp
```

---

## Verification Checklist

Before marking documentation as complete:

- [ ] Started fresh Daptin instance
- [ ] Set up admin user (become_an_administrator)
- [ ] Tested every curl command shown
- [ ] Verified response format matches documentation
- [ ] Tested error cases
- [ ] Checked action names against `columns.go`
- [ ] Confirmed OnType for each action

---

## Source of Truth Files

| Information | File |
|-------------|------|
| Action definitions | `server/resource/columns.go` (SystemActions) |
| Table definitions | `server/resource/columns.go` (StandardTables) |
| Relationships | `server/resource/columns.go` (StandardRelations) |
| Performer implementations | `server/actions/action_*.go` |
| Column types | `server/resource/column_types.go` |

---

## Testing Tips

### Export Token for Shell Session

```bash
# Sign in and export token in one step
export TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"doctest@example.com","password":"doctest123"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')
```

### Quick Permission Check

```bash
# If you get permission errors, verify admin status
curl -s "http://localhost:6336/api/user_account" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.data[] | {name: .attributes.name, email: .attributes.email}'
```

### Debug Action Calls

```bash
# Check server logs when action fails
# Look for performer execution errors
tail -f /path/to/daptin.log
```

---

## Debugging: Getting Blocked and Unblocked

### When You Get Stuck

**Systematic debugging approach:**

1. **State the assumption that's failing**
   - "I assume WebSocket connections should work with Bearer token in header"
   - Write down your assumption explicitly

2. **Test the assumption in isolation**
   ```bash
   # Minimal test case
   node test-websocket.js

   # Check what actually happens
   # - Client error?
   # - Server logs?
   # - Network traffic?
   ```

3. **Find contradictory evidence**
   - Client says 403, but server logs say 200 → Wrong client tool
   - Code supports feature, but you can't use it → Wrong approach
   - User says it works, but you see errors → Testing wrong way

4. **Search for working examples**
   ```bash
   # Look in example repos
   grep -r "feature_usage" /path/to/examples/

   # Check git history
   git log --all -p | grep "feature_name"

   # Search issues/discussions
   # (GitHub issues often contain working examples)
   ```

5. **Read the source of truth**
   - For auth: `server/auth/auth.go`
   - For routes: `server/server.go`
   - For handlers: `server/*.go` matching feature name
   - For middleware: `server/middleware/*.go`

6. **Test with absolutely minimal code**
   ```javascript
   // Simplest possible test
   const WebSocket = require('ws');
   const ws = new WebSocket('ws://localhost:6336/endpoint?token=TOKEN');
   ws.on('open', () => console.log('Connected'));
   ws.on('error', (err) => console.log('Error:', err.message));
   ```

### Common Blocking Patterns

**Pattern 1: Using wrong tool for protocol**

❌ Blocked:
```bash
# Using generic tool for specialized protocol
generic-tool --connect protocol://localhost:6336/endpoint
# Error: Unexpected response or protocol error
```

✅ Unblocked:
```bash
# Use protocol-specific client library
# Research appropriate tools for the protocol
# Follow protocol specifications
```

**Lesson:** Protocol-specific clients handle handshakes and features correctly.

---

**Pattern 2: Misreading error messages**

❌ Blocked:
```
Client shows: "Unexpected server response: 403"
Assumption: "Server returned 403, feature is broken"
```

✅ Unblocked:
```bash
# Check server logs
tail /tmp/daptin.log
# Shows: HTTP 200 responses

# Realization: Client tool problem, not server problem
```

**Lesson:** Cross-reference client errors with server logs.

---

**Pattern 3: Not finding usage examples**

❌ Blocked:
```
Documentation doesn't show how to use YJS endpoints
Code shows implementation but not usage
```

✅ Unblocked:
```bash
# Check example app git history
cd /tmp && git clone https://github.com/daptin/dadadash
cd dadadash
git log --all --full-history -p -- "**/*yjs*"

# Found: Commented-out code showing correct usage
```

**Lesson:** Git history contains examples even if current code doesn't.

---

**Pattern 4: Assuming auth works one way**

❌ Blocked:
```
Assumption: All endpoints use Authorization header
Reality: Some endpoints use different auth mechanisms
```

✅ Unblocked:
```go
// Read server/auth/auth.go or middleware for the endpoint
Extractor: jwtmiddleware.FromFirst(
    jwtmiddleware.FromAuthHeader,
    jwtmiddleware.FromParameter("token"),
    fromCookie,
)
// Found: Endpoint uses query parameter, not header
```

**Lesson:** Read auth middleware to understand all token extraction methods.

---

**Pattern 5: Testing in isolation without context**

❌ Blocked:
```
Test WebSocket endpoint with no understanding of:
- How authentication works
- What messages the protocol expects
- What the response format is
```

✅ Unblocked:
```bash
# 1. Understand protocol
grep -rn "websocket" server/ | grep "\.go:"

# 2. Find message format in code
grep -A20 "func.*Handle.*WebSocket" server/

# 3. Test with proper message format
```

**Lesson:** Understand the protocol before testing the endpoint.

---

### Recovery Checklist

When stuck, work through this checklist:

- [ ] Are you using the right tool for this protocol?
- [ ] Do server logs match what the client shows?
- [ ] Have you checked example repositories for usage?
- [ ] Have you read the authentication middleware code?
- [ ] Are you testing with minimal reproduction case?
- [ ] Have you searched git history for examples?
- [ ] Have you verified the endpoint exists in route registration?
- [ ] Are you checking both client AND server perspectives?

If all checklist items pass and feature still doesn't work, THEN it might be broken. But first exhaust all investigation avenues.

---

## See Also

- [Getting-Started-Guide](Getting-Started-Guide.md) - User onboarding with admin setup
- [Documentation-TODO](Documentation-TODO.md) - Track documentation progress
- [Actions-Overview](Actions-Overview.md) - Action system details
- [Documentation-Guide](Documentation-Guide.md) - In-depth documentation techniques
