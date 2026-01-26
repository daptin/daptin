# Key Behaviors

**Prerequisites**: [[First-Admin-Setup]] completed
**Related**: [[Common-Errors]] | [[Permissions]] | [[API-Overview]]

Critical behaviors discovered from code testing that affect all Daptin operations.

---

## API Filtering (Universal Behavior)

### ALL `/api/<entity>` endpoints filter by permissions

**Tested behavior**:
```bash
# Admin token
TOKEN=$(cat /tmp/daptin-token.txt)

# API returns filtered results
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/world | jq '.data | length'
# Returns: 8 tables

# Database shows all tables
sqlite3 daptin.db "SELECT COUNT(*) FROM world;"
# Returns: 60+ tables
```

**Why this matters**:
- Empty API response doesn't mean data doesn't exist
- Check permissions before assuming data is missing
- Use database queries to see unfiltered data
- This applies to ALL entities (world, user_account, custom tables, etc.)

**Filtering happens at TWO levels**:
1. **Table-level**: Does user's group have access to the table itself?
2. **Record-level**: Does user's group have access to specific records?

**Related**: [[Common-Errors#api-returns-empty-data]] | [[Permissions#two-level-permission-system]]

---

## Two-Level Permission System

### Both levels must be configured for access

**Level 1: Table-Level**
- Stored in: `world_world_id_has_usergroup_usergroup_id`
- Question: Can this group access the table at all?
- Effect: If denied, user sees NO records from this table via API

**Level 2: Record-Level**
- Stored in: `{table}_{table}_id_has_usergroup_usergroup_id`
- Question: Can this group access specific records?
- Effect: If denied, user sees only records they have permission for

**Tested example**:
```bash
# Create marketing group
GROUP_ID=$(curl -X POST http://localhost:6336/api/usergroup \
  -H "Authorization: Bearer $TOKEN" -d '{"data":{"type":"usergroup","attributes":{"name":"marketing"}}}' | jq -r '.data.id')

# Share TABLE with group (Level 1)
WORLD_ID=$(curl --get --data-urlencode 'query=[{"column":"table_name","operator":"is","value":"product"}]' \
  -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/world | jq -r '.data[0].id')

curl -X POST http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $TOKEN" -d '{"data":{"type":"world_world_id_has_usergroup_usergroup_id","attributes":{"world_id":"'$WORLD_ID'","usergroup_id":"'$GROUP_ID'"}}}' | jq -r '.data.id'

# Set permission on table-group join (MUST PATCH, POST ignores permission)
JOIN_ID=$(curl --get --data-urlencode 'query=[{"column":"world_id","operator":"is","value":"'$WORLD_ID'"},{"column":"usergroup_id","operator":"is","value":"'$GROUP_ID'"}]' \
  -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id | jq -r '.data[0].id')

curl -X PATCH "http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id/$JOIN_ID" \
  -H "Authorization: Bearer $TOKEN" -d '{"data":{"type":"world_world_id_has_usergroup_usergroup_id","id":"'$JOIN_ID'","attributes":{"permission":688128}}}'

# MUST restart server to clear Olric cache
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start

# Share RECORDS with group (Level 2)
PRODUCT_ID=$(curl -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/product | jq -r '.data[0].id')

curl -X POST http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $TOKEN" -d '{"data":{"type":"product_product_id_has_usergroup_usergroup_id","attributes":{"product_id":"'$PRODUCT_ID'","usergroup_id":"'$GROUP_ID'"}}}'

# Set permission on record-group join (MUST PATCH)
RECORD_JOIN_ID=$(curl --get --data-urlencode 'query=[{"column":"product_id","operator":"is","value":"'$PRODUCT_ID'"},{"column":"usergroup_id","operator":"is","value":"'$GROUP_ID'"}]' \
  -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id | jq -r '.data[0].id')

curl -X PATCH "http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id/$RECORD_JOIN_ID" \
  -H "Authorization: Bearer $TOKEN" -d '{"data":{"attributes":{"permission":688128}}}'
```

**Key point**: POST ignores `permission` field on join tables - always use POST then PATCH

**Related**: [[Common-Errors#403-forbidden-after-setting-permissions]] | [[Permissions]]

---

## Port Architecture

### Two ports, both critical

**Port 6336: HTTP API**
- REST/GraphQL endpoints
- User-facing API
- Standard HTTP

**Port 5336: Olric Distributed Cache**
- Stores: admin reference IDs, permissions, user-group memberships
- TTL: 60 minutes
- **Critical**: Stale cache causes "Unauthorized" errors even with fresh database

**Why port 5336 matters**:

Code reference (server/resource/dbresource.go):
```go
func GetAdminReferenceIdWithTransaction(transaction *sqlx.Tx) map[uuid.UUID]bool {
    if OlricCache != nil {
        cacheValueGet, err := OlricCache.Get(context.Background(), "administrator_reference_id")
        if err == nil {
            cacheValueGet.Scan(&adminMap)
            return adminMap  // Returns cached data!
        }
    }
    // ... query database if cache miss
}
```

**Tested symptom**:
```bash
# Fresh database, old process on 5336
rm daptin.db
go run main.go &
curl -X POST .../signup  # Works
curl -X POST .../become_an_administrator
# Returns: {"message": "Unauthorized"}
```

**Solution**:
```bash
# Always kill BOTH ports
lsof -i :6336 -t | xargs kill -9 2>/dev/null || true
lsof -i :5336 -t | xargs kill -9 2>/dev/null || true
```

**Related**: [[First-Admin-Setup#prerequisites-check]] | [[Common-Errors#unauthorized-on-become_an_administrator]]

---

## Server Restart Requirements

### Configuration changes require restart

**Restart required after**:
1. Creating actions (schema or API)
2. Creating cloud_store records
3. Linking credentials to cloud_store
4. Permission changes (to clear Olric cache)
5. Creating tables via schema API

**Why restart required**:
- Actions loaded at startup from database
- Cloud stores initialized at startup
- Olric cache not automatically invalidated
- Schema changes reload table metadata

**Tested example**:
```bash
# Create action
curl -X POST http://localhost:6336/api/action -d '...'

# Try to call it immediately
curl -X POST http://localhost:6336/action/entity/my_action
# Returns: 404 Not Found

# Restart server
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start

# Now it works
curl -X POST http://localhost:6336/action/entity/my_action
# Returns: Success
```

**Related**: [[Common-Errors#changes-dont-take-effect]]

---

## Password Handling

### API auto-hashes passwords

**Tested behavior**:
```bash
# Create user with plain password via API
curl -X POST http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"data":{"attributes":{"email":"user@example.com","password":"password123"}}}'

# Check database - password is bcrypt hashed
sqlite3 daptin.db "SELECT password FROM user_account WHERE email='user@example.com';"
# Shows: $2a$10$...
```

**If you provide pre-hashed password**:
```bash
# Pre-hash password
HASH=$(htpasswd -bnBC 10 "" password123 | tr -d ':\n')

# Provide to API
curl -X POST http://localhost:6336/api/user_account \
  -d '{"data":{"attributes":{"password":"'$HASH'"}}}'

# Database now has double-hashed password
# Signin with "password123" will FAIL
```

**Rules**:
- **Via API**: Use plain text passwords (API will hash)
- **Direct DB insert**: Use pre-hashed bcrypt passwords
- **Never mix**: Don't hash then pass to API

**Related**: [[Common-Errors#password-signin-fails]] | [[Users-and-Groups#creating-users]]

---

## File Upload Format

### Must be array of objects

**Tested wrong formats**:
```json
// WRONG - string
{"photo": "data:image/png;base64,..."}

// WRONG - single object
{"photo": {"name": "file.png", "file": "..."}}
```

**Correct format**:
```json
{
  "photo": [
    {
      "name": "file.png",
      "file": "data:image/png;base64,iVBORw0KGgo...",
      "type": "image/png"
    }
  ]
}
```

**Key points**:
- Array (even for single file)
- Each object has: name, file, type
- file is base64 data URI with `data:mime/type;base64,` prefix

**Related**: [[Common-Errors#file-upload-fails]] | [[Asset-Columns]]

---

## Query Syntax

### JSON array format, not filter[field]

**Tested wrong syntax**:
```bash
# WRONG - doesn't work
curl "http://localhost:6336/api/product?filter[name]=Widget"

# WRONG - JSON not URL-encoded
curl "http://localhost:6336/api/product?query=[{\"column\":\"name\",\"operator\":\"is\",\"value\":\"Widget\"}]"
```

**Correct syntax**:
```bash
# Method 1: --data-urlencode (recommended)
curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"is","value":"Widget"}]' \
  http://localhost:6336/api/product

# Method 2: Manual URL encoding
curl "http://localhost:6336/api/product?query=%5B%7B%22column%22%3A%22name%22%2C%22operator%22%3A%22is%22%2C%22value%22%3A%22Widget%22%7D%5D"

# Method 3: Single quotes (shell-specific, fragile)
curl 'http://localhost:6336/api/product?query=[{"column":"name","operator":"is","value":"Widget"}]'
```

**Related**: [[Common-Errors#filterquery-returns-nothing]] | [[Filtering-and-Pagination]]

---

## POST Ignores Permission on Join Tables

### Must PATCH after POST to set permission

**Tested behavior**:
```bash
# Create join with permission field
curl -X POST http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id \
  -d '{"data":{"attributes":{"world_id":"...","usergroup_id":"...","permission":688128}}}'

# Check database - permission is 2097151 (ignored the value we sent!)
sqlite3 daptin.db "SELECT permission FROM world_world_id_has_usergroup_usergroup_id WHERE ..."
# Shows: 2097151
```

**Correct approach**:
```bash
# Step 1: POST to create
JOIN_ID=$(curl -X POST ... | jq -r '.data.id')

# Step 2: PATCH to set permission
curl -X PATCH "http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id/$JOIN_ID" \
  -d '{"data":{"attributes":{"permission":688128}}}'

# Now permission is correct
```

**Related**: [[Common-Errors#403-forbidden-after-setting-permissions]] | [[Permissions]]

---

## Cloud Storage Credential Linking

### credential_name field does NOT auto-link

**Tested wrong approach**:
```bash
# Create cloud_store with credential_name
curl -X POST http://localhost:6336/api/cloud_store \
  -d '{"data":{"attributes":{"name":"my-store","credential_name":"my-creds"}}}'

# Check database - credential_id is NULL
sqlite3 daptin.db "SELECT credential_id FROM cloud_store WHERE name='my-store';"
# Shows: (empty)
```

**Correct approach**:
```bash
# Create cloud_store
STORE_ID=$(curl -X POST http://localhost:6336/api/cloud_store ... | jq -r '.data.id')

# Get credential ID
CRED_ID=$(curl http://localhost:6336/api/credential | jq -r '.data[0].id')

# Link via relationship PATCH
curl -X PATCH "http://localhost:6336/api/cloud_store/$STORE_ID" \
  -d '{"data":{"relationships":{"credential_id":{"data":{"type":"credential","id":"'$CRED_ID'"}}}}}'

# MUST restart server
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
```

**Related**: [[Common-Errors#cloud-storage-not-working]] | [[Cloud-Storage]]

---

## Next Steps

Now that you understand key behaviors:

- **Build features**: [[Schema-Definition]] | [[Users-and-Groups]] | [[Custom-Actions]]
- **When stuck**: [[Common-Errors]]
- **Deep dive**: [[Permissions]] | [[API-Overview]] | [[Cloud-Storage]]

---

**Last Updated**: 2026-01-26
**Based On**: Comprehensive code testing (walkthrough Steps 0-7)
**Test Coverage**: All behaviors verified through actual testing
