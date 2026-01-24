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
curl "http://localhost:6336/api/world?filter[table_name]=todo" \
  -H "Authorization: Bearer $TOKEN" | jq '.data[0].attributes.permission'
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

```bash
# Add record to a group
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
        "todo_id": {"data": {"type": "todo", "id": "RECORD_ID"}},
        "usergroup_id": {"data": {"type": "usergroup", "id": "GROUP_ID"}}
      }
    }
  }'
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
curl "http://localhost:6336/api/action?filter[action_name]=signup" \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Update permission
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

## Troubleshooting

### 403 Forbidden

You don't have permission for that operation. Check:
- Are you logged in?
- Do you own the record?
- Are you in a group that has access?
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
