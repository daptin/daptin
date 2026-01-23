# Users and Groups

## User Registration

### Signup Action

```bash
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "password": "password123",
      "name": "John Doe",
      "passwordConfirm": "password123"
    }
  }'
```

**Requirements:**
- Password minimum 8 characters
- Email must be unique
- passwordConfirm must match password

**Response:**
```json
[
  {"ResponseType": "client.notify", "Attributes": {"message": "Sign-up successful", "type": "success"}},
  {"ResponseType": "client.redirect", "Attributes": {"location": "/auth/signin"}}
]
```

## Authentication

### Sign In

```bash
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
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

### JWT Token Structure

```json
{
  "email": "user@example.com",
  "exp": 1729321122,
  "iat": 1729061922,
  "iss": "daptin-019228",
  "jti": "0192941f-260e-7b46-a1ae-f10fae700179",
  "name": "John Doe",
  "nbf": 1729061922,
  "sub": "01922e1a-d5ea-71c9-bd3e-616d23780f93"
}
```

Token validity: **3 days** (72 hours)

### Using the Token

```bash
export TOKEN="eyJhbG..."

# In Authorization header
curl http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $TOKEN"

# In cookie (automatically set by signin)
curl http://localhost:6336/api/user_account \
  --cookie "token=$TOKEN"
```

## Administrator Setup

### Become Administrator

First user can become admin:

```bash
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN"
```

**Effects:**
- Creates `administrators` usergroup
- Adds user to administrators
- Locks down all tables (admin-only access)

### Multi-Admin Setup

Add additional administrators:

```bash
# Get the user's reference_id
USER_ID=$(curl 'http://localhost:6336/api/user_account?query=[{"column":"email","operator":"is","value":"newadmin@example.com"}]' \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id')

# Add to administrators group
curl -X POST http://localhost:6336/api/user_account_administrators_has_usergroup_administrators \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"user_account_administrators_has_usergroup_administrators\",
      \"attributes\": {
        \"user_account_id\": \"$USER_ID\"
      }
    }
  }"
```

## User Groups

### Create Group

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

Junction table format: `user_account_{groupname}_has_usergroup_{groupname}`

```bash
curl -X POST http://localhost:6336/api/user_account_editors_has_usergroup_editors \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account_editors_has_usergroup_editors",
      "attributes": {
        "user_account_id": "USER_REFERENCE_ID"
      }
    }
  }'
```

### List User's Groups

```bash
curl http://localhost:6336/api/user_account/USER_ID/usergroup \
  -H "Authorization: Bearer $TOKEN"
```

## Password Management

### Password Reset Flow

```bash
# 1. Request reset
curl -X POST http://localhost:6336/action/user_account/generate_password_reset_flow \
  -d '{"attributes": {"email": "user@example.com"}}'

# 2. User receives email with reset link
# 3. Verify and set new password
curl -X POST http://localhost:6336/action/user_account/generate_password_reset_verify_flow \
  -d '{"attributes": {"email": "user@example.com", "verification": "TOKEN", "password": "newpassword123"}}'
```

### Configure Reset Email

```bash
curl -X POST http://localhost:6336/_config/backend/password.reset.email.from \
  -H "Authorization: Bearer $TOKEN" \
  -d '"noreply@yourdomain.com"'
```

## OAuth Social Login

### Configure OAuth

```yaml
Actions:
  - Name: oauth_login_begin
    OnType: oauth_connect
    InFields:
      - Name: provider
        ColumnType: label
    Outcomes:
      - Type: oauth.client.redirect
        Attributes:
          client_id: YOUR_CLIENT_ID
          client_secret: YOUR_CLIENT_SECRET
          redirect_uri: http://localhost:6336/oauth/response
```

### Supported Providers

- Google
- GitHub
- LinkedIn
- Custom OAuth2 endpoints

### OAuth Login Flow

```bash
# 1. Begin OAuth
curl -X POST http://localhost:6336/action/oauth_connect/oauth_login_begin \
  -d '{"attributes": {"provider": "google"}}'

# 2. User redirected to provider
# 3. Callback handled automatically
# 4. JWT token issued
```

## Two-Factor Authentication

### Generate OTP Secret

```bash
curl -X POST http://localhost:6336/action/user_account/otp_generate \
  -H "Authorization: Bearer $TOKEN"
```

**Response includes:**
- QR code for authenticator app
- Secret key for manual entry

### Verify OTP

```bash
curl -X POST http://localhost:6336/action/user_account/otp_login_verify \
  -d '{"attributes": {"email": "user@example.com", "otp": "123456"}}'
```

## Session Management

### Switch Session User

Admin can impersonate users:

```bash
curl -X POST http://localhost:6336/action/user_account/switch_session_user \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"attributes": {"user_account_id": "TARGET_USER_ID"}}'
```

### Generate JWT Token

Create token for API access:

```bash
curl -X POST http://localhost:6336/action/user_account/generate_jwt_token \
  -H "Authorization: Bearer $TOKEN"
```

## User Account Table

Default columns:

| Column | Type | Description |
|--------|------|-------------|
| email | varchar(200) | Unique email |
| name | varchar(200) | Display name |
| password | varchar(200) | Bcrypt hash |
| confirmed | bool | Email confirmed |
| reference_id | varchar(40) | UUID |
| permission | int | Row permission |
| created_at | datetime | Registration time |
