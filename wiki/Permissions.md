# Permissions

Daptin uses a Linux filesystem-like permission model with a 21-bit bitmask.

## Permission Levels

| Level | Description |
|-------|-------------|
| None | No access |
| Peek | See existence |
| Read | View data |
| Create | Create new records |
| Update | Modify records |
| Delete | Remove records |
| Execute | Run actions |
| Refer | Reference in relationships |

## Permission Scopes

Permissions apply at three scopes:

| Scope | Description |
|-------|-------------|
| User | Owner of the record |
| Group | Members of associated groups |
| Guest | Unauthenticated users |

## Permission Bitmask

21-bit permission value (max: 2097151):

```
Bits 0-6:   Guest permissions
Bits 7-13:  User permissions (NOT Group!)
Bits 14-20: Group permissions (NOT User!)
```

**Important:** The order is Guest → User → Group (not Guest → Group → User).

### Permission Bits

| Permission | Guest | User | Group |
|------------|-------|------|-------|
| Peek | 1 | 128 | 16384 |
| Read | 2 | 256 | 32768 |
| Create | 4 | 512 | 65536 |
| Update | 8 | 1024 | 131072 |
| Delete | 16 | 2048 | 262144 |
| Execute | 32 | 4096 | 524288 |
| Refer | 64 | 8192 | 1048576 |

### Common Permission Values

| Value | Meaning |
|-------|---------|
| 2097151 | Full access (all permissions) |
| 262142 | Admin default (all except guest refer) |
| 6 | Guest read + create |
| 786432 | User read + update |

## Setting Permissions

### Table Default Permission

```yaml
Tables:
  - TableName: private_data
    DefaultPermission: 786432  # User can read/update only
```

### Row-Level Permission

```bash
curl -X PATCH http://localhost:6336/api/todo/123 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "123",
      "attributes": {
        "permission": 786432
      }
    }
  }'
```

## Permission Checking

For each operation, Daptin checks:

1. **User ownership**: Is the requester the record owner?
2. **Admin membership**: Is user in administrators group?
3. **Group membership**: Does user share a group with the record?
4. **Guest permission**: Is the operation allowed for guests?

### Check Order

```
1. If user is owner AND user permission allows -> ALLOW
2. If user is admin -> ALLOW
3. If user shares group AND group permission allows -> ALLOW
4. If guest permission allows -> ALLOW
5. -> DENY
```

## Administrator Group

The `administrators` usergroup has special privileges:

```bash
# First user becomes admin
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN"

# Add user to admin group
curl -X POST http://localhost:6336/api/user_account_administrators_has_usergroup_administrators \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account_administrators_has_usergroup_administrators",
      "attributes": {
        "user_account_id": "USER_REFERENCE_ID"
      }
    }
  }'
```

## User Groups

Create groups for access control:

```bash
# Create group
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

# Add user to group
curl -X POST http://localhost:6336/api/user_account_editors_has_usergroup_editors \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account_editors_has_usergroup_editors",
      "attributes": {
        "user_account_id": "USER_ID"
      }
    }
  }'
```

## Action Permissions

Actions also have permission requirements:

```yaml
Actions:
  - Name: publish_article
    OnType: article
    InFields:
      - Name: article_id
        ColumnType: id
    RequiredPermission: Execute
```

## Permission Examples

### Private Table (Owner Only)

```yaml
Tables:
  - TableName: private_notes
    DefaultPermission: 2031616  # User: full, Group: none, Guest: none
```

### Public Read, Auth Write

```yaml
Tables:
  - TableName: blog_posts
    DefaultPermission: 786434  # User: read/update, Guest: read
```

### Team Collaboration

```yaml
Tables:
  - TableName: team_documents
    DefaultPermission: 1056768  # User: full, Group: read/update, Guest: none
```

## Permission Calculation Helper

```javascript
function calculatePermission(guest, user, group) {
  const levels = { none: 0, peek: 1, read: 2, create: 4, update: 8, delete: 16, execute: 32, refer: 64 };
  // Order: Guest (bits 0-6) | User (bits 7-13) | Group (bits 14-20)
  return levels[guest] | (levels[user] << 7) | (levels[group] << 14);
}

// Example: Guest none (0), User full (127), Group read (2)
// = 0 | (127 << 7) | (2 << 14) = 16256 + 32768 = 49024

// Common values:
// User full only: calculatePermission('none', 'refer', 'none') = 0 | (127 << 7) | 0 = 16256
// Public read: calculatePermission('read', 'read', 'read') = 2 | 256 | 32768 = 33026
// All permissions: calculatePermission('refer', 'refer', 'refer') = 127 | 16256 | 2080768 = 2097151
```
