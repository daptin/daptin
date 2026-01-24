# Authentication

Daptin supports multiple authentication methods: JWT tokens, OAuth providers, and Two-Factor Authentication.

**Related**: [Permissions](Permissions.md) | [Users and Groups](Users-and-Groups.md) | [Two-Factor Auth](Two-Factor-Auth.md)

**Source of truth**: `server/resource/columns.go` (actions), `server/actions/action_oauth_*.go` (OAuth performers)

---

## JWT Authentication

### Sign Up

```bash
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "name": "User Name",
      "email": "user@example.com",
      "password": "password123",
      "passwordConfirm": "password123"
    }
  }'
```

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

**Response**: Returns a `client.store.set` response with the JWT token:
```json
[
  {
    "ResponseType": "client.store.set",
    "Attributes": {
      "key": "token",
      "value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
    }
  }
]
```

### Using JWT Tokens

**Authorization header** (recommended):
```bash
curl http://localhost:6336/api/todo \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Cookie**:
```bash
curl http://localhost:6336/api/todo \
  --cookie "token=YOUR_JWT_TOKEN"
```

### Token Structure

```json
{
  "email": "user@example.com",
  "exp": 1729321122,
  "iat": 1729061922,
  "iss": "daptin-INSTANCE_ID",
  "jti": "unique-token-id",
  "name": "User Name",
  "nbf": 1729061922,
  "sub": "user-reference-id"
}
```

| Claim | Description |
|-------|-------------|
| `email` | User email address |
| `exp` | Expiration time (Unix timestamp) |
| `iat` | Issued at time |
| `iss` | Issuer (Daptin instance ID) |
| `jti` | Unique token identifier |
| `name` | User display name |
| `nbf` | Not valid before time |
| `sub` | User reference_id (UUID) |

**Default lifetime**: 3 days (72 hours)

### Generate Custom JWT

Generate a new JWT for the current user:

```bash
curl -X POST http://localhost:6336/action/user_account/jwt.token \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

### Configure JWT Settings

```bash
# Set JWT secret (admin only)
curl -X POST http://localhost:6336/_config/backend/jwt.secret \
  -H "Authorization: Bearer $TOKEN" \
  -d '"your-secure-secret-key"'

# Set issuer name
curl -X POST http://localhost:6336/_config/backend/jwt.token.issuer \
  -H "Authorization: Bearer $TOKEN" \
  -d '"my-application"'
```

---

## OAuth Authentication

Daptin supports OAuth 2.0 for authenticating users via external providers (Google, GitHub, etc.) or for obtaining API tokens for integrations.

### System Tables

| Table | Purpose |
|-------|---------|
| `oauth_connect` | Provider configuration (client_id, secrets, URLs) |
| `oauth_token` | Stored access/refresh tokens |

**Note**: These tables have `DefaultGroups: adminsGroup` - only administrators can manage OAuth configurations.

### oauth_connect Columns

| Column | Type | Description |
|--------|------|-------------|
| `name` | label | Unique provider name (e.g., "google", "github") |
| `client_id` | label | OAuth client ID from provider |
| `client_secret` | encrypted | OAuth client secret (stored encrypted) |
| `scope` | content | Comma-separated scopes (e.g., "email,profile") |
| `response_type` | label | OAuth response type (default: "code") |
| `redirect_uri` | url | Callback URL (default: "/oauth/response") |
| `auth_url` | url | Provider's authorization endpoint |
| `token_url` | url | Provider's token endpoint |
| `profile_url` | url | Provider's user info endpoint |
| `profile_email_path` | label | JSON path to email in profile response |
| `allow_login` | truefalse | Enable user authentication via this provider |

### Configure OAuth Provider

**Admin required** - Create oauth_connect record:

```bash
curl -X POST http://localhost:6336/api/oauth_connect \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "google",
        "client_id": "YOUR_CLIENT_ID.apps.googleusercontent.com",
        "client_secret": "YOUR_CLIENT_SECRET",
        "redirect_uri": "/oauth/response",
        "auth_url": "https://accounts.google.com/o/oauth2/auth",
        "token_url": "https://oauth2.googleapis.com/token",
        "profile_url": "https://www.googleapis.com/oauth2/v1/userinfo?alt=json",
        "scope": "email,profile",
        "allow_login": true
      }
    }
  }'
```

### Provider Examples

**Google**:
```yaml
name: google
auth_url: https://accounts.google.com/o/oauth2/auth
token_url: https://oauth2.googleapis.com/token
profile_url: https://www.googleapis.com/oauth2/v1/userinfo?alt=json
scope: email,profile
```

**GitHub**:
```yaml
name: github
auth_url: https://github.com/login/oauth/authorize
token_url: https://github.com/login/oauth/access_token
profile_url: https://api.github.com/user
scope: user:email
```

### OAuth Flow

#### Step 1: Begin OAuth (Redirect to Provider)

```bash
# Action requires an oauth_connect instance ID
curl -X POST "http://localhost:6336/action/oauth_connect/OAUTH_CONNECT_ID/oauth_login_begin" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

**Response**: Returns redirect URL and stores state:
```json
[
  {
    "ResponseType": "client.store.set",
    "Attributes": {"key": "secret", "value": "123456"}
  },
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "location": "https://accounts.google.com/o/oauth2/auth?client_id=...&state=123456",
      "window": "self",
      "delay": 0
    }
  }
]
```

The `state` is a 6-digit TOTP code valid for 300 seconds (5 minutes), used to prevent CSRF attacks.

#### Step 2: User Authorization (External)

User is redirected to the OAuth provider, authenticates, and grants permissions.

#### Step 3: Callback Handling

Provider redirects back to `/oauth/response?code=AUTH_CODE&state=123456&authenticator=google`

This triggers the `oauth.login.response` action:

```bash
# Called automatically by the OAuth callback
curl -X POST "http://localhost:6336/action/oauth_token/oauth.login.response" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "code": "AUTHORIZATION_CODE",
      "state": "123456",
      "authenticator": "google"
    }
  }'
```

**What happens**:
1. Validates state (TOTP check)
2. Exchanges code for access/refresh tokens
3. Stores tokens in `oauth_token` table
4. If `allow_login=true`:
   - Fetches user profile from provider
   - Creates or updates user account
   - Returns JWT token for the user

### OAuth for API Integration (Not Login)

Set `allow_login: false` to use OAuth only for obtaining API tokens (e.g., Google Sheets access):

```bash
curl -X POST http://localhost:6336/api/oauth_connect \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "google_sheets",
        "client_id": "YOUR_CLIENT_ID",
        "client_secret": "YOUR_CLIENT_SECRET",
        "scope": "https://www.googleapis.com/auth/spreadsheets",
        "allow_login": false
      }
    }
  }'
```

Tokens are stored in `oauth_token` and can be used for data exchange integrations.

---

## Two-Factor Authentication

See [Two-Factor Auth](Two-Factor-Auth.md) for complete documentation.

### Quick Reference

**Register OTP** (action name is `register_otp`, not `otp_generate`):
```bash
curl -X POST "http://localhost:6336/action/user_account/USER_ID/register_otp" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {"email": "user@example.com"}}'
```

**Verify OTP** (separate flow after signin):
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

**Important**: Daptin uses 4-digit codes with 300-second (5-minute) validity, not the standard 6-digit/30-second TOTP.

---

## WebSocket Authentication

Pass JWT token as query parameter:

```javascript
const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);

// Subscribe to table changes
ws.send(JSON.stringify({
  method: 'subscribe',
  attributes: { topicName: 'todo' }
}));
```

---

## Password Reset

### Request Reset

```bash
curl -X POST http://localhost:6336/action/user_account/generate_password_reset_flow \
  -H "Content-Type: application/json" \
  -d '{"attributes": {"email": "user@example.com"}}'
```

**Note**: This requires mail configuration to send the reset email.

### Complete Reset

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

---

## Session Impersonation (Admin)

Administrators can impersonate users for debugging:

```bash
curl -X POST http://localhost:6336/action/user_account/switch_session_user \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {"user_account_id": "TARGET_USER_REFERENCE_ID"}}'
```

---

## Security Best Practices

1. **Use HTTPS** in production
2. **Set strong JWT secret** via `/_config/backend/jwt.secret`
3. **Rotate secrets** periodically
4. **Enable 2FA** for admin accounts
5. **Use short token lifetimes** for sensitive applications
6. **Validate tokens** on every request
7. **Keep OAuth client_secret secure** - stored encrypted but protect your database
8. **Use `allow_login: false`** for OAuth connections that should only be used for API access

---

## See Also

- [Permissions](Permissions.md) - Access control system
- [Users and Groups](Users-and-Groups.md) - User management
- [Two-Factor Auth](Two-Factor-Auth.md) - Complete 2FA documentation
- [Getting Started Guide](Getting-Started-Guide.md) - Admin bootstrapping
