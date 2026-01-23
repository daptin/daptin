# Authentication

Daptin uses JWT tokens for authentication.

## JWT Authentication

### Get Token (Sign In)

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

### Use Token

Include in Authorization header:

```bash
curl http://localhost:6336/api/todo \
  -H "Authorization: Bearer eyJhbG..."
```

Or as cookie:

```bash
curl http://localhost:6336/api/todo \
  --cookie "token=eyJhbG..."
```

## Token Structure

```json
{
  "email": "user@example.com",
  "exp": 1729321122,
  "iat": 1729061922,
  "iss": "daptin-instance-id",
  "jti": "unique-token-id",
  "name": "User Name",
  "nbf": 1729061922,
  "sub": "user-reference-id"
}
```

| Claim | Description |
|-------|-------------|
| email | User email |
| exp | Expiration (Unix timestamp) |
| iat | Issued at |
| iss | Issuer (Daptin instance) |
| jti | Unique token ID |
| name | Display name |
| nbf | Not valid before |
| sub | User reference ID |

## Token Lifetime

Default: **3 days** (72 hours)

## Configure JWT

### Set JWT Secret

```bash
curl -X POST http://localhost:6336/_config/backend/jwt.secret \
  -H "Authorization: Bearer $TOKEN" \
  -d '"your-secure-secret-key"'
```

### Set Issuer

```bash
curl -X POST http://localhost:6336/_config/backend/jwt.token.issuer \
  -H "Authorization: Bearer $TOKEN" \
  -d '"my-application"'
```

## OAuth Authentication

### Supported Providers

- Google
- GitHub
- LinkedIn
- Custom OAuth2

### Configure OAuth

Create oauth_connect record:

```bash
curl -X POST http://localhost:6336/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "google",
        "client_id": "YOUR_CLIENT_ID",
        "client_secret": "YOUR_CLIENT_SECRET",
        "redirect_uri": "http://localhost:6336/oauth/response",
        "auth_url": "https://accounts.google.com/o/oauth2/auth",
        "token_url": "https://oauth2.googleapis.com/token",
        "profile_url": "https://www.googleapis.com/oauth2/v1/userinfo",
        "scope": "email profile"
      }
    }
  }'
```

### OAuth Flow

1. **Begin OAuth**

```bash
curl -X POST http://localhost:6336/action/oauth_connect/oauth_login_begin \
  -d '{"attributes": {"provider": "google"}}'
```

Returns redirect URL to OAuth provider.

2. **Callback** (automatic)

Provider redirects to `/oauth/response` with code.

3. **Token Exchange** (automatic)

Daptin exchanges code for tokens and creates/updates user.

## Two-Factor Authentication

### Enable 2FA

```bash
curl -X POST http://localhost:6336/action/user_account/otp_generate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{}'
```

Returns QR code and secret for authenticator app.

### Verify 2FA

```bash
curl -X POST http://localhost:6336/action/user_account/otp_login_verify \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "otp": "123456"
    }
  }'
```

### Configure TOTP Secret

```bash
curl -X POST http://localhost:6336/_config/backend/totp.secret \
  -H "Authorization: Bearer $TOKEN" \
  -d '"your-totp-secret"'
```

## WebSocket Authentication

For WebSocket connections, pass token as query parameter:

```javascript
const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);
```

## Password Reset

### Request Reset

```bash
curl -X POST http://localhost:6336/action/user_account/generate_password_reset_flow \
  -d '{"attributes": {"email": "user@example.com"}}'
```

### Complete Reset

```bash
curl -X POST http://localhost:6336/action/user_account/generate_password_reset_verify_flow \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "verification": "RESET_TOKEN",
      "password": "newpassword123"
    }
  }'
```

## Session Impersonation (Admin)

Admins can impersonate users:

```bash
curl -X POST http://localhost:6336/action/user_account/switch_session_user \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"attributes": {"user_account_id": "TARGET_USER_ID"}}'
```

## Security Best Practices

1. **Use HTTPS** in production
2. **Set strong JWT secret**
3. **Rotate secrets** periodically
4. **Enable 2FA** for admin accounts
5. **Use short token lifetimes** for sensitive apps
6. **Validate tokens** on every request
