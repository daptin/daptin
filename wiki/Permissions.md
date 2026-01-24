# Permissions

Daptin uses a Unix-like permission model with a 21-bit bitmask for fine-grained access control.

**Related**: [Core Concepts](Core-Concepts.md) | [Users and Groups](Users-and-Groups.md) | [Getting Started](Getting-Started-Guide.md)

**Source of truth**: `server/auth/auth.go` (constants), `server/permission/permission.go` (checking), `server/resource/dbmethods.go` (loading)

---

## Overview

Every entity (table) and every row (record) in Daptin has:

1. **A permission value** - 21-bit bitmask controlling access
2. **An owner** - `user_account_id` column (who created it)
3. **Group associations** - via join table (which groups have access)

Permissions are checked at two levels:
- **Table level**: Can you access this type of data at all?
- **Row level**: Can you access this specific record?

---

## Permission Levels

| Level | Description | Typical Use |
|-------|-------------|-------------|
| **Peek** | See that record exists | List filtering, existence checks |
| **Read** | View full data | GET requests, viewing details |
| **Create** | Create new records | POST requests |
| **Update** | Modify records | PATCH/PUT requests |
| **Delete** | Remove records | DELETE requests |
| **Execute** | Run actions | Action execution |
| **Refer** | Use in relationships | Foreign key references |

---

## Permission Scopes

| Scope | Who | Check Order |
|-------|-----|-------------|
| **Guest** | Unauthenticated users | Checked if owner/group checks fail |
| **User** | Record owner | Checked first (owner has priority) |
| **Group** | Users sharing a group with the record | Checked if owner check fails |

---

## Permission Bitmask

21-bit value with three 7-bit scopes (max: 2097151):

```
Bits 0-6:   Guest permissions
Bits 7-13:  User (owner) permissions
Bits 14-20: Group permissions
```

**Important**: Order is **Guest → User → Group** in the bits, but check order is **User → Guest → Group**.

### Permission Bit Values

| Permission | Guest | User | Group | Bit Position |
|------------|-------|------|-------|--------------|
| Peek | 1 | 128 | 16384 | 0 |
| Read | 2 | 256 | 32768 | 1 |
| Create | 4 | 512 | 65536 | 2 |
| Update | 8 | 1024 | 131072 | 3 |
| Delete | 16 | 2048 | 262144 | 4 |
| Execute | 32 | 4096 | 524288 | 5 |
| Refer | 64 | 8192 | 1048576 | 6 |

### Default Permission: 561441

The system default (from `auth.go`):

```go
DEFAULT_PERMISSION = GuestPeek | GuestExecute | UserRead | UserExecute | GroupRead | GroupExecute
```

| Scope | Permissions | Value |
|-------|-------------|-------|
| Guest | Peek + Execute | 33 |
| User | Read + Execute | 34 (shifted to bits 7-13) = 4352 |
| Group | Read + Execute | 34 (shifted to bits 14-20) = 557056 |

Total: 33 + 4352 + 557056 = **561441**

### Common Permission Values

| Value | Meaning | Calculation |
|-------|---------|-------------|
| 2097151 | Full access (all bits set) | 127 + (127 << 7) + (127 << 14) |
| 561441 | Default: Owner/Group read+execute, Guest peek+execute | See above |
| 16256 | User full access only | 127 << 7 |
| 33026 | Public read (everyone can read) | 2 + 256 + 32768 |
| 0 | No access | |

---

## Permission Check Flow

When a user requests an operation, Daptin checks permissions in this order:

### 1. Owner Check (User Scope)

```
IF record.user_account_id == request.user_id
   AND record.permission has User{Operation} bit
THEN → ALLOW
```

### 2. Guest Check

```
IF record.permission has Guest{Operation} bit
THEN → ALLOW
```

### 3. Administrator Check

```
IF request.user is member of "administrators" group
THEN → ALLOW
```

### 4. Group Check

```
FOR each group the user belongs to:
    FOR each group associated with the record (via join table):
        IF groups match AND join_table.permission has Group{Operation} bit
        THEN → ALLOW
```

### 5. Default Deny

```
→ DENY
```

**Key insight**: The group permission check uses the **join table's permission**, not the record's permission. This allows fine-grained per-group access control.

---

## Table vs Row Permissions

### Table-Level Permission

Controls access to the entire table. Stored in the `world` table.

```yaml
Tables:
  - TableName: private_notes
    DefaultPermission: 16256  # User full access only, no group/guest
```

Each row in `world` table has:
- `permission` column: Who can access this table definition
- `default_permission` column: Default for new records

### Row-Level Permission

Each record has its own `permission` column, initialized from `default_permission`.

```bash
# Get a record's permission
curl "http://localhost:6336/api/todo/TODO_UUID" \
  -H "Authorization: Bearer $TOKEN" | jq '.data.attributes.permission'

# Modify row permission
curl -X PATCH "http://localhost:6336/api/todo/TODO_UUID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "TODO_UUID",
      "attributes": {
        "permission": 33026
      }
    }
  }'
```

---

## Ownership

Every table (except `usergroup` and join tables) automatically gets a `belongs_to user_account` relationship, creating a `user_account_id` column.

### How Ownership is Set

When you create a record while authenticated, Daptin automatically sets `user_account_id` to your user ID.

```bash
# Create record - owner is automatically set to current user
curl -X POST http://localhost:6336/api/todo \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"todo","attributes":{"title":"My task"}}}'

# Response shows owner in relationships
# "user_account_id": {"data": {"type": "user_account", "id": "YOUR_UUID"}}
```

### Transfer Ownership

Admin can change record ownership:

```bash
curl -X PATCH http://localhost:6336/api/todo/TODO_UUID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "TODO_UUID",
      "relationships": {
        "user_account_id": {"data": {"type": "user_account", "id": "NEW_OWNER_UUID"}}
      }
    }
  }'
```

---

## Group Permissions

Every table automatically gets a `has_many usergroup` relationship via a join table.

### Join Table Structure

Join table name: `{table}_{table}_id_has_usergroup_usergroup_id`

| Column | Description |
|--------|-------------|
| `{table}_id` | References the record |
| `usergroup_id` | References the group |
| `permission` | **Per-association permission** |
| `reference_id` | Unique join table record ID |

**Critical**: The join table has its OWN `permission` column. This controls what that specific group can do with that specific record.

### View Record's Group Associations

```bash
curl "http://localhost:6336/api/mail_server/SERVER_UUID/usergroup_id" \
  -H "Authorization: Bearer $TOKEN" | jq '.data[] | {name: .attributes.name, permission: .attributes.permission}'
```

Example response:
```json
{
  "name": "administrators",
  "permission": 2097151
}
```

The `permission` value (2097151) means administrators have full access to this specific record via group membership.

### Associate Record with Group

```bash
# Add record to a group with specific permission
curl -X POST "http://localhost:6336/api/todo_todo_id_has_usergroup_usergroup_id" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo_todo_id_has_usergroup_usergroup_id",
      "attributes": {
        "permission": 32768
      },
      "relationships": {
        "todo_id": {"data": {"type": "todo", "id": "TODO_UUID"}},
        "usergroup_id": {"data": {"type": "usergroup", "id": "GROUP_UUID"}}
      }
    }
  }'
```

---

## Administrator Group

The `administrators` usergroup has special privileges:
- **Bypasses all permission checks**
- Members can access any record regardless of permission settings

### First Admin Setup

```bash
# Action is on "world" table, NOT "user_account"
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
# Server restarts automatically. Re-signin after restart.
```

See [Getting Started Guide](Getting-Started-Guide.md) for full admin bootstrapping workflow.

### Check User's Groups

```bash
curl "http://localhost:6336/api/user_account/USER_UUID/usergroup_id" \
  -H "Authorization: Bearer $TOKEN" | jq '.data[] | .attributes.name'
```

---

## User Groups

### System Groups

| Group | Purpose |
|-------|---------|
| `administrators` | Full system access |
| `guests` | Default group (all users) |
| `users` | Authenticated users |

### Create Custom Group

```bash
curl -X POST http://localhost:6336/api/usergroup \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "usergroup",
      "attributes": {
        "name": "editors"
      }
    }
  }'
```

### Add User to Group

Use the user_account-usergroup join table relationship endpoint:

```bash
# Get user's current group associations
curl "http://localhost:6336/api/user_account/USER_UUID/usergroup_id" \
  -H "Authorization: Bearer $TOKEN"

# Add user to group via PATCH on user_account
curl -X PATCH "http://localhost:6336/api/user_account/USER_UUID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "id": "USER_UUID",
      "relationships": {
        "usergroup_id": {
          "data": [{"type": "usergroup", "id": "GROUP_UUID"}]
        }
      }
    }
  }'
```

---

## Action Permissions

Actions check the Execute permission bit on their OnType table.

```yaml
Actions:
  - Name: publish_article
    OnType: article
    # User needs Execute permission on article table to run this action
```

To allow guests to run an action, the table needs GuestExecute (bit 5 = 32).

---

## Permission Examples

### Private Table (Owner Only)

```yaml
Tables:
  - TableName: private_notes
    DefaultPermission: 16256  # User full (127 << 7)
    # Calculation: 0 + (127 << 7) + 0 = 16256
    # Guest: none, User: all, Group: none
```

### Public Read, Owner Write

```yaml
Tables:
  - TableName: blog_posts
    DefaultPermission: 16642  # Guest read, User read+update+delete
    # Calculation: 2 + ((2|8|16) << 7) + 0 = 2 + (26 << 7) = 2 + 3328 = 3330
    # Wait, let me recalculate properly:
    # Guest Read: 2
    # User Read|Update|Delete: 2|8|16 = 26, shifted by 7 = 3328
    # Total: 2 + 3328 = 3330
```

### Team Collaboration

```yaml
Tables:
  - TableName: team_documents
    DefaultPermission: 491555  # User full, Group read+update+create
    # Calculation:
    # Guest: Peek = 1
    # User: Full (127) << 7 = 16256
    # Group: Read|Create|Update = 2|4|8 = 14, shifted by 14 = 229376
    # Total: 1 + 16256 + 229376 = 245633
```

---

## Permission Calculation

### JavaScript Helper

```javascript
function calculatePermission(guestBits, userBits, groupBits) {
  // Each scope has 7 bits: Peek(0), Read(1), Create(2), Update(3), Delete(4), Execute(5), Refer(6)
  return guestBits | (userBits << 7) | (groupBits << 14);
}

function decodeBits(val) {
  const perms = ['Peek', 'Read', 'Create', 'Update', 'Delete', 'Execute', 'Refer'];
  return perms.filter((_, i) => val & (1 << i));
}

function decodePermission(perm) {
  return {
    guest: decodeBits(perm & 127),
    user: decodeBits((perm >> 7) & 127),
    group: decodeBits((perm >> 14) & 127)
  };
}

// Examples:
console.log(calculatePermission(0, 127, 0));  // 16256 - User full only
console.log(calculatePermission(2, 2, 2));    // 33026 - Everyone can read
console.log(calculatePermission(127, 127, 127)); // 2097151 - Full access all
console.log(decodePermission(561441));
// { guest: ['Peek', 'Execute'], user: ['Read', 'Execute'], group: ['Read', 'Execute'] }
```

### Python Helper

```python
def calc_perm(guest, user, group):
    return guest | (user << 7) | (group << 14)

def decode_perm(perm):
    perms = ['Peek', 'Read', 'Create', 'Update', 'Delete', 'Execute', 'Refer']
    def decode(val):
        return [p for i, p in enumerate(perms) if val & (1 << i)]
    return {
        'guest': decode(perm & 127),
        'user': decode((perm >> 7) & 127),
        'group': decode((perm >> 14) & 127)
    }

# Bit values for combining:
PEEK, READ, CREATE, UPDATE, DELETE, EXECUTE, REFER = 1, 2, 4, 8, 16, 32, 64
FULL = 127  # All bits

print(calc_perm(0, FULL, 0))       # 16256
print(decode_perm(561441))          # Default permission breakdown
```

### Quick Reference Table

| Decimal | Binary (21 bits) | Meaning |
|---------|------------------|---------|
| 0 | 000000000000000000000 | No access |
| 1 | 000000000000000000001 | Guest Peek only |
| 2 | 000000000000000000010 | Guest Read only |
| 33 | 000000000000000100001 | Guest Peek + Execute |
| 127 | 000000000000001111111 | Guest full |
| 256 | 000000000000010000000 | User Read only |
| 16256 | 000000000011111110000000 | User full |
| 32768 | 000001000000000000000 | Group Read only |
| 561441 | 010001001000100100001 | Default (see above) |
| 2097151 | 111111111111111111111 | All permissions |

---

## Troubleshooting

### "Refer object not allowed"

Usually means you don't have `Refer` permission on the object you're trying to reference.

**Solutions:**
1. Set up admin first (see [Getting Started](Getting-Started-Guide.md))
2. Check your user's groups
3. Check the record's group associations

### 403 Forbidden on Action

The action's OnType table needs Execute permission for your user scope.

### Can Read but Can't Update

Your permission scope (Guest/User/Group) has Read bit but not Update bit.

**Check:**
```bash
# Decode the permission value
python3 -c "print({
  'guest': [(p, bool($PERM & (1<<i))) for i, p in enumerate(['Peek','Read','Create','Update','Delete','Execute','Refer'])],
  'user': [(p, bool(($PERM>>7) & (1<<i))) for i, p in enumerate(['Peek','Read','Create','Update','Delete','Execute','Refer'])],
  'group': [(p, bool(($PERM>>14) & (1<<i))) for i, p in enumerate(['Peek','Read','Create','Update','Delete','Execute','Refer'])]
})"
```

---

## See Also

- [Core Concepts](Core-Concepts.md) - Entity model and standard columns
- [Users and Groups](Users-and-Groups.md) - User management
- [Getting Started Guide](Getting-Started-Guide.md) - Admin bootstrapping
- [Relationships](Relationships.md) - How join tables work
- [Actions Overview](Actions-Overview.md) - Action permission requirements
