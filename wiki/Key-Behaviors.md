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

**Related**: [[Common-Errors#api-returns-empty-data]] | [[Permissions#two-level-permission-check-critical]]

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

For a group-based grant, configure the table's world record and the individual
resource separately using their **Groups** tabs. See
[[Permissions#share-with-a-group]] for the verified workflow and
[[Permissions#two-level-permission-check-critical]] for the two gates.

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

**Why port 5336 matters:** Daptin uses this cache for permission-related
lookups. An older Daptin process connected to a different database can return
stale values, so stop it before starting a fresh instance.

**Tested symptom**:
```bash
# Fresh database, old process on 5336
rm daptin.db
go run ./cmd/daptin &
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

## Group Relation Permission Editing

Creation uses configured defaults. Dashboard group permission edits are a
separate operation and persist after refresh; no restart is required. Use
[[Permissions#share-with-a-group]] rather than CRUD on generated join resources.
Nested parent relationship permission attributes do not necessarily follow the
dashboard save path; verify the persisted grant and effective access.

---

## Cloud Storage Credential Selection

`cloud_store.credential_name` is the sole credential selector. Set it to the
credential row's exact `name`:

```bash
curl -X POST http://localhost:6336/api/cloud_store \
  -d '{"data":{"type":"cloud_store","attributes":{"name":"my-store","store_type":"s3","credential_name":"my-creds","root_path":"my-store:bucket"}}}'

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
