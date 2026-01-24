# Documentation Guidelines for Daptin

Guidelines for documenting Daptin features accurately.

---

## Golden Rule

**ALWAYS test against the running API before documenting.**

Reading code alone is insufficient. The code shows what performers exist, but:
- Action names may differ from performer names
- OnType (entity) may not be obvious
- Parameters may have different names than expected
- Response formats vary

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

## See Also

- [Getting-Started-Guide](Getting-Started-Guide.md) - User onboarding with admin setup
- [Documentation-TODO](Documentation-TODO.md) - Track documentation progress
- [Actions-Overview](Actions-Overview.md) - Action system details
