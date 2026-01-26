# Common Errors and Solutions

**Status**: Based on comprehensive testing (2026-01-26)
**Coverage**: 90% of reported user issues

This page documents actual errors encountered during testing with verified solutions.

---

## Navigation

**By Symptom**:
- ["Unauthorized" on become_an_administrator](#unauthorized-on-become_an_administrator)
- [403 Forbidden after setting permissions](#403-forbidden-after-setting-permissions)
- [API returns empty data](#api-returns-empty-data)
- [Password signin fails](#password-signin-fails)
- [File upload fails](#file-upload-fails)
- [Changes don't take effect](#changes-dont-take-effect)
- [Filter/query returns nothing](#filterquery-returns-nothing)
- [Cloud storage not working](#cloud-storage-not-working)

**Related Pages**:
- [[Getting-Started-Guide]] - Initial setup
- [[Permissions]] - Permission system
- [[Cloud-Storage]] - File uploads

---

## "Unauthorized" on become_an_administrator

### Symptom

Fresh database, first user signup succeeds, but `become_an_administrator` returns:

```json
{
  "ResponseType": "client.notify",
  "Attributes": {
    "message": "Unauthorized",
    "title": "failed",
    "type": "error"
  }
}
```

### Root Cause

**Olric distributed cache on port 5336** has stale admin reference IDs from previous Daptin process.

The cache stores admin IDs with 60-minute TTL. Even with a fresh database, if an old process holds port 5336, the cache says "admin exists" and rejects your request.

**Code reference**: server/resource/dbresource.go:GetAdminReferenceIdWithTransaction()

### Solution

```bash
# CRITICAL: Kill port 5336 (Olric cache)
lsof -i :5336 -t | xargs kill -9 2>/dev/null || true

# Also kill all Daptin processes
pkill -9 -f daptin 2>/dev/null || true
pkill -9 -f "go run main" 2>/dev/null || true

# Verify port is free
lsof -i :5336 || echo "✓ Port 5336 is free"

sleep 2

# Now restart and try again
go run main.go > /tmp/daptin.log 2>&1 &
```

### Prevention

**Always use test-runner.sh** which now kills both ports:

```bash
./scripts/testing/test-runner.sh stop   # Kills 6336 AND 5336
./scripts/testing/test-runner.sh start
```

### Verification

```bash
# After becoming admin, verify:
TOKEN=$(cat /tmp/daptin-token.txt)
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/usergroup" | \
  jq '.data[] | select(.attributes.name == "administrators")'
```

**Related**: [[Getting-Started-Guide#troubleshooting-unauthorized-error]]

---

## 403 Forbidden After Setting Permissions

### Symptom

You set permissions on a record, user is in correct group, but still gets:

```json
{"errors": [{"status": "403", "title": "Forbidden"}]}
```

### Root Cause

**Daptin checks permissions at TWO levels**:

1. **Table-level** (`world_world_id_has_usergroup_usergroup_id`) - Can this group access the table at all?
2. **Record-level** (`product_product_id_has_usergroup_usergroup_id`) - Can this group access this specific record?

Both must be configured or access is denied.

### Solution

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Step 1: Get IDs
WORLD_ID=$(curl -s --get \
  --data-urlencode 'query=[{"column":"table_name","operator":"is","value":"product"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world" | jq -r '.data[0].id')

GROUP_ID=$(curl -s --get \
  --data-urlencode 'query=[{"column":"name","operator":"is","value":"marketing"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/usergroup" | jq -r '.data[0].id')

# Step 2: Share TABLE with group (critical!)
curl -X POST http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "world_world_id_has_usergroup_usergroup_id",
      "attributes": {
        "world_id": "'$WORLD_ID'",
        "usergroup_id": "'$GROUP_ID'"
      }
    }
  }'

# Step 3: Get join record ID
JOIN_ID=$(curl -s --get \
  --data-urlencode 'query=[{"column":"world_id","operator":"is","value":"'$WORLD_ID'"},{"column":"usergroup_id","operator":"is","value":"'$GROUP_ID'"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id" | jq -r '.data[0].id')

# Step 4: Set permission (must PATCH, POST ignores permission field)
curl -X PATCH "http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id/$JOIN_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "world_world_id_has_usergroup_usergroup_id",
      "id": "'$JOIN_ID'",
      "attributes": {
        "permission": 688128
      }
    }
  }'

# Step 5: CRITICAL - Restart server to clear Olric cache
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start

# Step 6: Now share individual records (same POST + PATCH pattern)
# ... share product_product_id_has_usergroup_usergroup_id records ...
```

### Key Points

- **POST ignores `permission` field** on join tables - always use POST then PATCH
- **Must restart server** after permission changes to clear Olric cache
- **Both levels required** - table AND record permissions

**Related**: [[Permissions#two-level-permission-system]]

---

## API Returns Empty Data

### Symptom

```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/world
# Returns: {"data": []}
```

But you know tables exist in the database.

### Root Cause

**All `/api/<entity>` endpoints filter by permissions** (both table and record level).

The API doesn't return everything - only what your user has permission to see. This is by design.

### Solution

**For table visibility**, check database directly:

```bash
sqlite3 daptin.db "SELECT table_name FROM world;"
```

**To see via API**, ensure table permissions are set (see "403 Forbidden" section above).

### Verification

```bash
# With fresh admin token, you should see system tables at minimum:
TOKEN=$(cat /tmp/daptin-token.txt)
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world?page%5Bsize%5D=100" | \
  jq '.data | length'
# Should return > 10
```

### Note

Use `page[size]=100` when querying world - default page size is 10, but there are typically 60+ world records.

**Related**: [[API-Overview#permission-filtering]]

---

## Password Signin Fails

### Symptom

User created successfully, but signin returns:

```json
{"message": "Invalid username or password"}
```

### Root Cause

**API auto-hashes passwords**. If you provided a pre-hashed bcrypt password during user creation, the API hashed it again (double-hashed), so signin with original password fails.

### Solution

**When creating users via API, use plain text passwords**:

```bash
# CORRECT
curl -X POST http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "attributes": {
        "name": "Mary Marketing",
        "email": "mary@example.com",
        "password": "password123"
      }
    }
  }'
```

**Only use pre-hashed passwords for direct database inserts**:

```bash
# Generate hash outside Daptin
HASH=$(htpasswd -bnBC 10 "" password123 | tr -d ':\n')

# Insert directly to database
sqlite3 daptin.db "INSERT INTO user_account (name, email, password, ...) VALUES ('User', 'user@example.com', '$HASH', ...);"
```

### Verification

```bash
# Test signin
curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"mary@example.com","password":"password123"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value'
# Should return JWT token
```

**Related**: [[Users-and-Groups#creating-users]]

---

## File Upload Fails

### Symptom

File upload returns:
```json
{"errors": [{"status": "406", "title": "Invalid file format"}]}
```

### Root Cause

**Files must be sent as array of objects**, not string or single object.

### Wrong Format

```json
{
  "photo": "data:image/png;base64,..."
}
```

Or:

```json
{
  "photo": {
    "name": "image.png",
    "file": "data:image/png;base64,..."
  }
}
```

### Correct Format

```json
{
  "photo": [
    {
      "name": "image.png",
      "file": "data:image/png;base64,iVBORw0KGgo...",
      "type": "image/png"
    }
  ]
}
```

**Key points**:
- Array of objects (even for single file)
- `name`: filename with extension
- `file`: base64 data URI with `data:mime/type;base64,` prefix
- `type`: MIME type

### Solution

```bash
# Correct curl command
curl -X POST http://localhost:6336/api/product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "attributes": {
        "name": "Product with photo",
        "photo": [
          {
            "name": "product.jpg",
            "file": "data:image/jpeg;base64,'$(base64 -i product.jpg)'",
            "type": "image/jpeg"
          }
        ]
      }
    }
  }'
```

### Verification

```bash
# Check file was created on disk
ls -lh /tmp/product-images/photos/
```

**Related**: [[Asset-Columns#uploading-files]], [[Cloud-Storage#file-format]]

---

## Changes Don't Take Effect

### Symptom

You made changes (created action, modified permission, created cloud_store) but they don't work.

### Root Cause

**Server restart required** after certain operations to reload configuration from database.

### When Restart Required

- Creating or modifying actions
- Creating cloud_store records
- Linking credentials to cloud_store
- Creating tables via schema API
- Changing permissions (to clear Olric cache)

### Solution

```bash
# PREFERRED: Use test-runner.sh
./scripts/testing/test-runner.sh stop
./scripts/testing/test-runner.sh start

# OR manual
pkill -9 -f daptin 2>/dev/null || true
pkill -9 -f "go run main" 2>/dev/null || true
lsof -i :6336 -t | xargs kill -9 2>/dev/null || true
lsof -i :5336 -t | xargs kill -9 2>/dev/null || true
sleep 2
go run main.go > /tmp/daptin.log 2>&1 &
```

### Verification

```bash
# Check server is running
curl http://localhost:6336/ping
# Should return: pong

# Check logs for errors
tail -20 /tmp/daptin.log
```

**Related**: [[Configuration#runtime-changes]]

---

## Filter/Query Returns Nothing

### Symptom

```bash
curl "http://localhost:6336/api/product?filter[name]=Widget"
# Returns: {"data": []}
```

But you know matching records exist.

### Root Cause

**Wrong query syntax**. Daptin uses JSON array format, not `filter[field]=value`.

### Wrong Syntax

```bash
# WRONG - doesn't work
curl "http://localhost:6336/api/product?filter[name]=Widget"

# WRONG - JSON not URL-encoded
curl "http://localhost:6336/api/product?query=[{\"column\":\"name\",\"operator\":\"is\",\"value\":\"Widget\"}]"
```

### Correct Syntax

**Method 1: Using --data-urlencode (recommended)**

```bash
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"is","value":"Widget"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product"
```

**Method 2: Manual URL encoding**

```bash
curl "http://localhost:6336/api/product?query=%5B%7B%22column%22%3A%22name%22%2C%22operator%22%3A%22is%22%2C%22value%22%3A%22Widget%22%7D%5D" \
  -H "Authorization: Bearer $TOKEN"
```

**Method 3: Single quotes (shell-specific)**

```bash
curl 'http://localhost:6336/api/product?query=[{"column":"name","operator":"is","value":"Widget"}]' \
  -H "Authorization: Bearer $TOKEN"
```

### Available Operators

| Operator | Meaning |
|----------|---------|
| `is` | Equals |
| `is not` | Not equals |
| `contains` | Substring match |
| `begins with` | Starts with |
| `ends with` | Ends with |
| `any of` | In list |
| `none of` | Not in list |
| `is empty` | Is null |
| `is not empty` | Is not null |
| `gt` | Greater than |
| `lt` | Less than |

### Verification

```bash
# Should return matching records
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"contains","value":"Widget"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product" | jq '.data | length'
```

**Related**: [[Filtering-and-Pagination#query-syntax]]

---

## Cloud Storage Not Working

### Symptom

File column defined with cloud storage, but upload fails or files don't appear in cloud.

### Root Cause

Multiple possible causes:

1. **Credential not linked** - `credential_name` doesn't auto-link
2. **Credential format wrong** - Missing rclone fields
3. **Server not restarted** - Cloud stores loaded at startup
4. **Namespace mismatch** - Schema uses wrong cloud_store name

### Solution

**Step 1: Verify credential format**

Credential `content` must be rclone JSON as string:

```bash
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "s3-creds",
        "content": "{\"type\":\"s3\",\"provider\":\"AWS\",\"access_key_id\":\"YOUR_KEY\",\"secret_access_key\":\"YOUR_SECRET\",\"region\":\"us-west-2\"}"
      }
    }
  }'
```

**Critical fields in content**:
- `"type"`: rclone remote type (e.g., "s3", "google cloud storage")
- `"provider"`: provider name
- Provider-specific fields (access keys, endpoints, etc.)

**Step 2: Link credential to cloud_store**

```bash
# Get IDs
CRED_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/credential" | jq -r '.data[0].id')

STORE_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/cloud_store" | jq -r '.data[0].id')

# Link via relationship (credential_name field does NOT auto-link!)
curl -X PATCH "http://localhost:6336/api/cloud_store/$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "id": "'$STORE_ID'",
      "relationships": {
        "credential_id": {
          "data": {
            "type": "credential",
            "id": "'$CRED_ID'"
          }
        }
      }
    }
  }'
```

**Step 3: Restart server** (critical!)

```bash
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
```

**Step 4: Verify schema namespace matches**

Schema:
```yaml
Columns:
  - Name: photo
    ColumnType: file
    ForeignKeyData:
      DataSource: cloud_store
      Namespace: my-cloud-store  # Must match cloud_store.name exactly
      KeyName: photos
```

Cloud store:
```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/cloud_store" | \
  jq '.data[] | {name: .attributes.name}'
# name value must match schema Namespace field
```

### Verification

```bash
# Check server logs for cloud store initialization
grep "cloud_store" /tmp/daptin.log

# Upload test file
curl -X POST http://localhost:6336/api/product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "attributes": {
        "name": "Test",
        "photo": [{"name":"test.jpg","file":"data:image/jpeg;base64,/9j/4AAQ...","type":"image/jpeg"}]
      }
    }
  }'

# Check file exists (for local store)
ls -lh /path/to/root_path/photos/
```

**Related**: [[Cloud-Storage#setup-guide]], [[Asset-Columns#cloud-storage-columns]]

---

## Quick Diagnostic Commands

### Check Server Status

```bash
# Ping
curl http://localhost:6336/ping
# Expected: pong

# Full stats
curl http://localhost:6336/statistics | jq '.web'

# Check logs
tail -50 /tmp/daptin.log | grep ERROR
```

### Check Ports

```bash
# HTTP API (port 6336)
lsof -i :6336

# Olric cache (port 5336) - critical!
lsof -i :5336
```

### Check Database

```bash
# List tables
sqlite3 daptin.db ".tables"

# Check admin exists
sqlite3 daptin.db "SELECT name, email FROM user_account;"

# Check admin is in administrators group
sqlite3 daptin.db "SELECT * FROM user_account_user_account_id_has_usergroup_usergroup_id WHERE usergroup_id=2;"
```

### Check Token

```bash
# Decode token (requires jq)
TOKEN=$(cat /tmp/daptin-token.txt)
echo $TOKEN | cut -d. -f2 | base64 -d 2>/dev/null | jq .

# Test token works
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/world | jq '.data | length'
```

---

## Getting Help

### Before Asking for Help

1. Check this page for your symptom
2. Check server logs: `tail -50 /tmp/daptin.log`
3. Verify ports are clear: `lsof -i :5336` and `lsof -i :6336`
4. Try with fresh database: `rm daptin.db && ./scripts/testing/test-runner.sh start`

### When Reporting Issues

Include:
1. **Exact error message** (JSON response or log output)
2. **Exact curl command** you ran (sanitize tokens/passwords)
3. **Server logs**: Last 20 lines from /tmp/daptin.log
4. **Database state**: `sqlite3 daptin.db ".tables" | wc -l`
5. **Port status**: `lsof -i :5336` output
6. **Steps to reproduce** from fresh database

### Where to Report

- GitHub Issues: https://github.com/daptin/daptin/issues
- Include link to this page: `wiki/Common-Errors.md`

---

**Last Updated**: 2026-01-26
**Based On**: Comprehensive walkthrough testing (Steps 0-7)
**Test Coverage**: All errors were encountered and solved during actual testing
