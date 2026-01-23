# User Actions

Actions for user registration, authentication, and account management.

## signup

Register a new user account.

```bash
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "password": "securepass123",
      "name": "John Doe",
      "passwordConfirm": "securepass123"
    }
  }'
```

**Parameters:**

| Field | Required | Description |
|-------|----------|-------------|
| email | Yes | Unique email address |
| password | Yes | Min 8 characters |
| name | No | Display name |
| passwordConfirm | Yes | Must match password |

**Response:**
```json
[
  {"ResponseType": "client.notify", "Attributes": {"message": "Sign-up successful", "type": "success"}},
  {"ResponseType": "client.redirect", "Attributes": {"location": "/auth/signin", "delay": 2000}}
]
```

## signin

Authenticate and receive JWT token.

```bash
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "password": "securepass123"
    }
  }'
```

**Response:**
```json
[
  {"ResponseType": "client.store.set", "Attributes": {"key": "token", "value": "eyJhbG..."}},
  {"ResponseType": "client.cookie.set", "Attributes": {"key": "token", "value": "eyJhbG...; SameSite=Strict"}},
  {"ResponseType": "client.notify", "Attributes": {"message": "Logged in", "type": "success"}},
  {"ResponseType": "client.redirect", "Attributes": {"location": "/", "delay": 2000}}
]
```

Token validity: **3 days**

## generate_password_reset_flow

Request password reset email.

```bash
curl -X POST http://localhost:6336/action/user_account/generate_password_reset_flow \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com"
    }
  }'
```

**Prerequisites:**
- SMTP server configured
- `password.reset.email.from` config set

```bash
curl -X POST http://localhost:6336/_config/backend/password.reset.email.from \
  -H "Authorization: Bearer $TOKEN" \
  -d '"noreply@yourdomain.com"'
```

## generate_password_reset_verify_flow

Complete password reset with verification token.

```bash
curl -X POST http://localhost:6336/action/user_account/generate_password_reset_verify_flow \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "verification": "RESET_TOKEN_FROM_EMAIL",
      "password": "newpassword123"
    }
  }'
```

## otp_generate

Generate TOTP secret for two-factor authentication.

```bash
curl -X POST http://localhost:6336/action/user_account/otp_generate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {}
  }'
```

**Response includes:**
- QR code image (base64)
- Secret key for manual entry

User scans QR code with authenticator app (Google Authenticator, Authy, etc.)

## otp_login_verify

Verify OTP code during login.

```bash
curl -X POST http://localhost:6336/action/user_account/otp_login_verify \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "otp": "123456"
    }
  }'
```

**Parameters:**

| Field | Description |
|-------|-------------|
| email | User email |
| otp | 6-digit code from authenticator |

## switch_session_user

Admin-only: Impersonate another user.

```bash
curl -X POST http://localhost:6336/action/user_account/switch_session_user \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "user_account_id": "TARGET_USER_REFERENCE_ID"
    }
  }'
```

Returns new JWT token for the target user.

## generate_jwt_token

Generate a new JWT token for current session.

```bash
curl -X POST http://localhost:6336/action/user_account/generate_jwt_token \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {}
  }'
```

Useful for extending session or generating API tokens.

## JWT Token Structure

```json
{
  "email": "user@example.com",
  "exp": 1729321122,
  "iat": 1729061922,
  "iss": "daptin-instance-id",
  "jti": "unique-token-id",
  "name": "John Doe",
  "nbf": 1729061922,
  "sub": "user-reference-id"
}
```

| Claim | Description |
|-------|-------------|
| email | User email |
| exp | Expiration time |
| iat | Issued at time |
| iss | Issuer (daptin instance) |
| jti | Unique token ID |
| name | User display name |
| nbf | Not valid before |
| sub | User reference ID |

## Configure JWT

```bash
# Set JWT secret
curl -X POST http://localhost:6336/_config/backend/jwt.secret \
  -H "Authorization: Bearer $TOKEN" \
  -d '"your-secret-key"'

# Set issuer name
curl -X POST http://localhost:6336/_config/backend/jwt.token.issuer \
  -H "Authorization: Bearer $TOKEN" \
  -d '"my-app"'
```

## Error Responses

### Invalid Credentials

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "type": "error",
      "title": "Failed",
      "message": "Invalid username or password"
    }
  }
]
```

### Password Too Short

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "type": "error",
      "message": "Password too short, minimum 8 characters"
    }
  }
]
```

### Email Already Exists

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "type": "error",
      "message": "Email already registered"
    }
  }
]
```
