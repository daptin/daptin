# Users and Groups

Manage user accounts and organize them into groups for access control.

---

## User Registration

### Sign Up (Fresh Install Only)

On a fresh install, anyone can sign up:

```bash
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "name": "John Doe",
      "email": "john@example.com",
      "password": "password123",
      "passwordConfirm": "password123"
    }
  }'
```

**After admin exists**: Signup is disabled. See [Admin: Create a User](#admin-create-a-user).

---

## Sign In

```bash
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "john@example.com",
      "password": "password123"
    }
  }'
```

**Response:**
```json
[
  {"ResponseType": "client.store.set", "Attributes": {"key": "token", "value": "eyJhbG..."}},
  {"ResponseType": "client.cookie.set", "Attributes": {"key": "token", "value": "eyJhbG..."}},
  {"ResponseType": "client.notify", "Attributes": {"message": "Logged in", "type": "success"}}
]
```

### Extract the Token

**Using jq** (recommended):

```bash
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"john@example.com","password":"password123"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "$TOKEN" > /tmp/daptin-token.txt
echo "Token saved!"
```

### Use the Token

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

curl http://localhost:6336/api/todo \
  -H "Authorization: Bearer $TOKEN"
```

**Token validity**: 3 days (72 hours) from sign-in.

---

## Admin: Create a User

After admin setup, signup is disabled. Only admins can create new users via the API.

**Option 1: With plain-text password** (Daptin hashes it automatically):

```bash
curl -X POST http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "attributes": {
        "name": "New User",
        "email": "newuser@example.com",
        "password": "userpassword123"
      }
    }
  }'
```

**Option 2: With bcrypt hash** (for pre-hashed passwords):

```bash
# This hash = "password123"
HASH='$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy'

curl -X POST http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "attributes": {
        "name": "New User",
        "email": "newuser@example.com",
        "password": "'$HASH'"
      }
    }
  }'
```

**Note**: Bcrypt hashes start with `$2a$` or `$2y$`. Use this when importing users from another system.

---

## User Groups

Groups let you share access to records with multiple users.

### List Groups

```bash
curl http://localhost:6336/api/usergroup \
  -H "Authorization: Bearer $TOKEN"
```

### Create a Group

```bash
curl -X POST http://localhost:6336/api/usergroup \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
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

**Tested ✓** - Use the junction table `user_account_user_account_id_has_usergroup_usergroup_id`.

**Method 1: Using attributes** (simpler, recommended):

```bash
# Get user and group IDs
USER_ID=$(curl --get \
  --data-urlencode 'query=[{"column":"email","operator":"is","value":"newuser@example.com"}]' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:6336/api/user_account" | jq -r '.data[0].id')

GROUP_ID=$(curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"is","value":"editors"}]' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:6336/api/usergroup" | jq -r '.data[0].id')

# Add user to group
curl -X POST http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"user_account_user_account_id_has_usergroup_usergroup_id\",
      \"attributes\": {
        \"user_account_id\": \"$USER_ID\",
        \"usergroup_id\": \"$GROUP_ID\"
      }
    }
  }"
```

**Method 2: Using relationships**:

```bash
curl -X POST http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account_user_account_id_has_usergroup_usergroup_id",
      "relationships": {
        "user_account_id": {
          "data": {"type": "user_account", "id": "USER_REFERENCE_ID"}
        },
        "usergroup_id": {
          "data": {"type": "usergroup", "id": "GROUP_REFERENCE_ID"}
        }
      }
    }
  }'
```

**Both methods work**, but the attributes method is easier to use with variables.

### Verify Group Membership

**Tested ✓** - Check which users are in which groups:

```bash
# Via API
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id?page%5Bsize%5D=100" | \
  jq '.data[] | {user_id: .attributes.user_account_id, group_id: .attributes.usergroup_id}'

# Or via database (more readable)
sqlite3 daptin.db "
SELECT u.name as User, ug.name as UserGroup
FROM user_account_user_account_id_has_usergroup_usergroup_id j
JOIN user_account u ON j.user_account_id = u.id
JOIN usergroup ug ON j.usergroup_id = ug.id
ORDER BY u.name, ug.name;
"
```

### Remove User from Group

Delete the junction record:

```bash
# Get the junction record ID
JUNCTION_ID=$(curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id?page%5Bsize%5D=100" | \
  jq -r '.data[] | select(.attributes.user_account_id == "USER_ID" and .attributes.usergroup_id == "GROUP_ID") | .id')

# Delete it
curl -X DELETE "http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id/$JUNCTION_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## Administrators

The `administrators` group has full access to everything.

### Become First Admin

On fresh install, the first user to run this becomes admin:

```bash
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

Server restarts. Sign in again.

### Add Another Admin

Add user to the administrators group:

```bash
# 1. Find the administrators group ID
ADMIN_GROUP_ID=$(curl --get \
  --data-urlencode 'query=[{"column":"name","operator":"is","value":"administrators"}]' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:6336/api/usergroup" | jq -r '.data[0].id')

echo "Administrators group ID: $ADMIN_GROUP_ID"

# 2. Get the user ID to promote
USER_ID=$(curl --get \
  --data-urlencode 'query=[{"column":"email","operator":"is","value":"newuser@example.com"}]' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:6336/api/user_account" | jq -r '.data[0].id')

echo "User ID: $USER_ID"

# 3. Add user to administrators group
curl -X POST http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"user_account_user_account_id_has_usergroup_usergroup_id\",
      \"attributes\": {
        \"user_account_id\": \"$USER_ID\",
        \"usergroup_id\": \"$ADMIN_GROUP_ID\"
      }
    }
  }"
```

---

## Password Reset

**Important:** Password reset requires:
1. SMTP configured (for sending verification email)
2. Admin must initiate the reset (guest access is blocked by default permissions)

### Admin-Initiated Reset

```bash
# Admin requests password reset for a user
curl -X POST http://localhost:6336/action/user_account/reset-password \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com"
    }
  }'
```

User receives email with verification code.

### Verify and Set New Password

```bash
curl -X POST http://localhost:6336/action/user_account/reset-password-verify \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "otp": "123456",
      "password": "newpassword123"
    }
  }'
```

**Note**: See [SMTP Server](SMTP-Server.md) for email configuration.

---

## Two-Factor Authentication (2FA)

Daptin supports TOTP-based OTP authentication.

### Enable 2FA

Requires the user's reference ID:

```bash
# Get user reference ID
USER_REF=$(curl -s http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id')

# Register OTP
curl -X POST http://localhost:6336/action/user_account/register_otp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"attributes\": {
      \"mobile_number\": \"1234567890\",
      \"user_account_id\": \"$USER_REF\"
    }
  }"
```

### Sign In with OTP

```bash
curl -X POST http://localhost:6336/action/user_account/verify_otp \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "otp": "1234"
    }
  }'
```

See [Two-Factor Auth](Two-Factor-Auth.md) for complete setup including OTP generation.

---

## View User Details

### Get Current User

Query by email from your token:

```bash
curl --get \
  --data-urlencode 'query=[{"column":"email","operator":"is","value":"your@email.com"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/user_account"
```

Or if you know your user ID:

```bash
curl "http://localhost:6336/api/user_account/YOUR_USER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### List All Users (Admin)

```bash
curl http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## User Account Fields

| Field | Description |
|-------|-------------|
| `name` | Display name |
| `email` | Login email (unique) |
| `password` | Stored as bcrypt hash |
| `confirmed` | Email verified (true/false) |
| `reference_id` | UUID for API operations |
| `created_at` | Registration timestamp |

---

## Common Issues

| Problem | Solution |
|---------|----------|
| **403 on signup** | Admin exists, signup disabled. Ask admin to create account. |
| **Invalid password** | Check email/password. Passwords are case-sensitive. |
| **Can't add user to group** | Use full junction table name: `user_account_user_account_id_has_usergroup_usergroup_id` |
| **403 on password reset** | Guest users cannot trigger password reset. Admin must initiate. |
| **Password reset email not received** | SMTP not configured. See [SMTP Server](SMTP-Server.md). |
| **OTP "no reference id"** | `register_otp` requires `user_account_id` in attributes. |

---

## See Also

- [Getting Started](Getting-Started-Guide.md) - First admin setup
- [Permissions](Permissions.md) - Control access with groups
- [Two-Factor Auth](Two-Factor-Auth.md) - 2FA setup
- [Authentication](Authentication.md) - OAuth and JWT details
