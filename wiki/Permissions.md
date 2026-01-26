# Permissions

Daptin uses a permission system to control who can access what data.

**Related**: [Getting Started](Getting-Started-Guide.md) | [Users and Groups](Users-and-Groups.md)

---

## Quick Start

### Fresh Install

On a fresh install, the system is **wide open** - anyone can do anything. This lets you:

1. Sign up your first user
2. Sign in
3. Become administrator
4. System locks down automatically

### After Admin Setup

Once an admin exists:
- Public signup is disabled
- Guests can only view public data
- Only admins can create new users

---

## Permission Basics

Every table and every record has permissions controlling:

| Permission | What it allows |
|------------|----------------|
| **Peek** | See that something exists (appears in lists) |
| **Read** | View the full data |
| **Create** | Make new records |
| **Update** | Change existing records |
| **Delete** | Remove records |
| **Execute** | Run actions |
| **Refer** | Use in relationships |

### Who Gets Checked

Permissions apply to three groups:

| Group | Who |
|-------|-----|
| **Guest** | Anyone not logged in |
| **Owner** | The user who created the record |
| **Group** | Users who share a group with the record |

---

## Default Behavior

### Before First Admin

| Who | Can do |
|-----|--------|
| Guest | Everything (create, read, update, delete, execute) |
| Users | Everything |

### After First Admin

| Who | Can do |
|-----|--------|
| Guest | Peek at public data, execute signin |
| Owner | Read their own data, execute actions |
| Group members | Read shared data, execute actions |
| Administrators | Everything |

---

## Common Tasks

### Check Who Can Access a Table

```bash
# Get table permission
curl --get \
  --data-urlencode 'query=[{"column":"table_name","operator":"is","value":"todo"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world" | jq '.data[0].attributes.permission'
```

### Check Record Permission

```bash
curl "http://localhost:6336/api/todo/RECORD_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '.data.attributes.permission'
```

### Make a Record Public (Readable by Guests)

```bash
curl -X PATCH "http://localhost:6336/api/todo/RECORD_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "RECORD_ID",
      "attributes": {
        "permission": 33026
      }
    }
  }'
```

### Share with a Group

**IMPORTANT**: POST requests to join tables ignore the `permission` attribute. You must:
1. Create the join record first
2. PATCH to set the permission

```bash
# Step 1: Add record to a group
JOIN_ID=$(curl -s -X POST "http://localhost:6336/api/todo_todo_id_has_usergroup_usergroup_id" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo_todo_id_has_usergroup_usergroup_id",
      "attributes": {
        "todo_id": "RECORD_ID",
        "usergroup_id": "GROUP_ID"
      }
    }
  }' | jq -r '.data.id')

# Step 2: Set permission on the join record
# Permission 32768 = 2 << 14 (Read permission for group)
curl -X PATCH "http://localhost:6336/api/todo_todo_id_has_usergroup_usergroup_id/$JOIN_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo_todo_id_has_usergroup_usergroup_id",
      "id": "'$JOIN_ID'",
      "attributes": {
        "permission": 32768
      }
    }
  }'
```

**Permission values for join tables** (group permissions only):
```javascript
// Group permissions are bit-shifted to position 14-20
const groupRead = 2 << 14;        // 32768
const groupUpdate = 8 << 14;      // 131072
const groupReadWrite = (2+8) << 14; // 163840 = 10 << 14
const groupFull = 127 << 14;      // 2080768
```

---

## Permission Values

Permissions are stored as numbers. Here are common values:

| Value | Meaning |
|-------|---------|
| 0 | No access |
| 2 | Guest can read |
| 256 | Owner can read |
| 32768 | Group can read |
| 33026 | Everyone can read |
| 16256 | Owner has full access |
| 561441 | Default (owner/group read, guest peek) |
| 2097151 | Full access for everyone |

### Calculate Your Own

```javascript
// Permission calculator
function permission(guest, owner, group) {
  return guest | (owner << 7) | (group << 14);
}

// Bit values: Peek=1, Read=2, Create=4, Update=8, Delete=16, Execute=32, Refer=64
// Full = 127 (all bits)

permission(2, 127, 0);    // 16258 - Guest read, owner full
permission(0, 127, 127);  // 2080768 - Owner and group full, no guest
permission(2, 2, 2);      // 33026 - Everyone can read
```

---

## Administrator Group

Members of the `administrators` group bypass all permission checks.

### Become First Admin

```bash
# After signing up and signing in
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

Server restarts. Sign in again to get admin token.

### Add User to Admin Group

```bash
curl -X PATCH "http://localhost:6336/api/user_account/USER_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "id": "USER_ID",
      "relationships": {
        "usergroup_id": {
          "data": [{"type": "usergroup", "id": "ADMIN_GROUP_ID"}]
        }
      }
    }
  }'
```

---

## Enable Public Signup (After Admin Exists)

By default, signup is disabled after admin setup. To re-enable:

1. Find the signup action ID
2. Add guest execute permission (value: current + 32)

```bash
# Find signup action
curl --get \
  --data-urlencode 'query=[{"column":"action_name","operator":"is","value":"signup"}]' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:6336/api/action" | jq '.data[0].id'

# Update permission to add guest execute (32)
# Current permission + (32) = 2085120 + 32 = 2085152
curl -X PATCH "http://localhost:6336/api/action/ACTION_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "action",
      "id": "ACTION_ID",
      "attributes": {
        "permission": 2085152
      }
    }
  }'
```

---

## State Machine Permissions

State machines have special permission requirements for the **Refer** permission on the `smd` table.

### The Refer Permission

When creating a state machine instance with `/track/start/:smdId`, the system creates a relationship from `{table}_state` to the `smd` table. This requires **Refer** permission on the `smd` record.

**Without Refer permission, you'll see:**
```
[ERROR] User cannot refer this object [smd][<uuid>]
Failed to execute state insert query: refer object not allowed [smd][<uuid>]
HTTP 500 Internal Server Error
```

### Grant Refer Permission to SMD

**Option 1: Via API (Recommended)**

```bash
# Get your SMD ID
SMD_ID=$(curl -s "http://localhost:6336/api/smd" \
  -H "Authorization: Bearer $TOKEN" | \
  jq -r '.data[] | select(.attributes.name == "ticket_workflow") | .id')

# Update SMD to allow users to reference it
curl -X PATCH "http://localhost:6336/api/smd/$SMD_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "smd",
      "id": "'$SMD_ID'",
      "attributes": {
        "permission": 1621954
      }
    }
  }'
# Permission 1621954 = Guest:Read|Refer (66) + Owner:Full (127) + Group:Read|Execute|Refer (98)
# Calculation: 66 | (127 << 7) | (98 << 14) = 66 + 16256 + 1605632 = 1621954
```

**Option 2: Via SQL (If server stopped)**

```bash
# Stop server
./scripts/testing/test-runner.sh stop

# Get SMD reference_id
sqlite3 daptin.db "SELECT hex(reference_id), name FROM smd;"

# Update permission
sqlite3 daptin.db "UPDATE smd SET permission = 1621954 WHERE reference_id = X'<UUID_HEX>';"

# Restart server
./scripts/testing/test-runner.sh start
```

### State Transition Permissions

The `/track/event/:typename/:stateId/:eventName` endpoint requires **Execute** permission on the `smd` table:

```bash
# Verify SMD has execute permission
curl "http://localhost:6336/api/smd/$SMD_ID" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.data.attributes.permission'

# Permission bits needed:
# - Guest Execute: 32 (bit 5)
# - Owner Execute: 4096 (32 << 7)
# - Group Execute: 524288 (32 << 14)
```

**Full SMD Permission Example:**
```javascript
// Permission bits: Peek=1, Read=2, Create=4, Update=8, Delete=16, Execute=32, Refer=64
const guestPerms = 2 | 64;        // Read + Refer = 66
const ownerPerms = 127;           // Full access = 127
const groupPerms = 2 | 32 | 64;   // Read + Execute + Refer = 98

const smdPermission = guestPerms | (ownerPerms << 7) | (groupPerms << 14);
// = 66 | 16256 | 1605632 = 1621954
```

### Why Two Permissions?

1. **Refer (64)** - Required for `/track/start/:smdId`
   - Allows creating relationships to the SMD
   - Checked when creating state instances

2. **Execute (32)** - Required for `/track/event/:typename/:stateId/:eventName`
   - Allows applying state transitions
   - Checked before executing FSM events

**Related**: [State Machines](State-Machines.md) - Full state machine documentation

---

## Two-Level Permission Check (CRITICAL)

**Important**: Daptin checks permissions at TWO levels for group-based access:

1. **Table-level** (world record): Can this group access the table at all?
2. **Record-level**: Can this group access this specific record?

**Both levels must be configured**, or you'll get 403 Forbidden errors even when the record permissions look correct.

### Example: Share Product Table with Marketing Group

```bash
# Get the table's world ID
WORLD_ID=$(curl --get \
  --data-urlencode 'query=[{"column":"table_name","operator":"is","value":"product"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world" | jq -r '.data[0].id')

# CRITICAL STEP 1: Share the TABLE (world record) with the group
curl -X POST "http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "world_world_id_has_usergroup_usergroup_id",
      "attributes": {
        "world_id": "'$WORLD_ID'",
        "usergroup_id": "MARKETING_GROUP_ID"
      }
    }
  }'

# CRITICAL STEP 2: Set permission on the world-group join record
# 688128 = 42 << 14 (Read + Update + Execute for group)
WORLD_JOIN_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id" | \
  jq -r '.data[0].id')

curl -X PATCH "http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id/$WORLD_JOIN_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "world_world_id_has_usergroup_usergroup_id",
      "id": "'$WORLD_JOIN_ID'",
      "attributes": {
        "permission": 688128
      }
    }
  }'

# CRITICAL STEP 3: Restart server to clear Olric permission cache
pkill -9 -f daptin
sleep 2
./daptin &
sleep 10
```

**After this**, the marketing group can access records in the product table (subject to record-level permissions).

---

## Troubleshooting

### 403 Forbidden (Most Common Causes)

**1. Table not shared with group** (MOST COMMON)

The user's group has permission on the record, but not on the table itself.

**Solution**: Share the world (table) record with the group (see "Two-Level Permission Check" above), then restart the server.

**2. Stale Olric permission cache**

Permissions are cached for 10 minutes. After changing permissions, stale cache may cause 403 errors.

**Solution**: Restart the server to clear the cache immediately.

**3. Other checks**:
- Are you logged in?
- Do you own the record?
- Are you in a group that has access to the record?
- Is the operation allowed for your role?

### Can't Sign Up

Signup is disabled after admin setup. Ask admin to create your account or enable public signup.

### Can't See Records

The records exist but you don't have Peek permission. Contact the owner or admin.

---

## See Also

- [Getting Started Guide](Getting-Started-Guide.md) - First-time setup
- [Users and Groups](Users-and-Groups.md) - Managing users
- [Actions Overview](Actions-Overview.md) - Running actions
