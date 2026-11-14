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

In the dashboard, open the account and use **Groups → Add Group**. This creates
membership. To configure what that group may do to another resource, edit the
other resource's **Groups** tab; see [[Permissions#share-with-a-group]].

For API link creation, PATCH the account's relationship using the account and
group public reference IDs. Replace `USER_ID` and `GROUP_ID` below:

```bash
curl -X PATCH http://localhost:6336/api/user_account/USER_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "user_account",
      "id": "USER_ID",
      "relationships": {
        "usergroup_id": {
          "data": [{"type": "usergroup", "id": "GROUP_ID"}]
        }
      }
    }
  }'
```

This adds the link; it does not choose an explicit relation permission. Do not
add nested `attributes.permission` and assume it performs the same save as the
dashboard's permission controls. See [[Permissions#api-boundaries-and-verification]].

### Verify Group Membership

Read the account's relationship identifiers:

```bash
curl http://localhost:6336/api/user_account/USER_ID/relationships/usergroup_id \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

The `data` identifiers here are actual group reference IDs. To see group names
and relation attributes, use the related-resource endpoint:

```bash
curl http://localhost:6336/api/user_account/USER_ID/usergroup_id \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

For these to-many related-resource results, `data.id` identifies the relation
row, while `attributes.relation_reference_id` identifies the group. Do not use
a relation ID as a group ID. See [[Relationships#understanding-ids-in-relationship-responses]].

### Remove User from Group

Open the account's **Groups** tab and click **Remove** on the intended group.
Refresh and verify the membership is absent. Clearing the relation's permission
bits does not remove membership. Removing one membership does not revoke grants
from other groups, ownership, guest access, or administrator membership.

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

As an existing administrator, open the intended account's **Groups** tab and
attach the built-in `administrators` group. Use the membership workflow above;
do not invent a special administrator join resource. Verify the selected
account and group before granting this access, then test as that account.

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

**Note**: See [[SMTP-Server|SMTP Server]] for email configuration.

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

See [[Two-Factor-Auth|Two-Factor Auth]] for complete setup including OTP generation.

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
| **Can't add user to group** | Use the account’s Groups tab or parent relationship PATCH; verify public reference IDs and authorization |
| **403 on password reset** | Guest users cannot trigger password reset. Admin must initiate. |
| **Password reset email not received** | SMTP not configured. See [[SMTP-Server|SMTP Server]]. |
| **OTP "no reference id"** | `register_otp` requires `user_account_id` in attributes. |

---

## See Also

- [[Getting-Started-Guide|Getting Started]] - First admin setup
- [[Permissions|Permissions]] - Control access with groups
- [[Authorization-Scenarios|Authorization Scenarios]] - Public, private, mixed, owner-row, and shared workspace patterns
- [[Two-Factor-Auth|Two-Factor Auth]] - 2FA setup
- [[Authentication|Authentication]] - OAuth and JWT details
