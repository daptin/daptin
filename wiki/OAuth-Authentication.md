# OAuth Authentication

**Tested ✓ 2026-01-26** (Infrastructure and parameter validation tested; full provider integration requires real OAuth credentials)

OAuth 2.0 authentication enables users to sign in using external providers like Google, GitHub, Microsoft, Facebook, and others. Daptin implements the standard OAuth 2.0 Authorization Code Flow.

---

## Quick Start (5 minutes)

### Prerequisites
- OAuth provider account (Google, GitHub, etc.)
- Registered OAuth application with provider
- Client ID and Client Secret from provider

### 1. Create OAuth Connection

```bash
TOKEN="your-jwt-token"

curl -X POST http://localhost:6336/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "google-login",
        "client_id": "YOUR_CLIENT_ID.apps.googleusercontent.com",
        "client_secret": "YOUR_CLIENT_SECRET",
        "scope": "https://www.googleapis.com/auth/userinfo.email,https://www.googleapis.com/auth/userinfo.profile",
        "auth_url": "https://accounts.google.com/o/oauth2/v2/auth",
        "token_url": "https://oauth2.googleapis.com/token",
        "profile_url": "https://www.googleapis.com/oauth2/v1/userinfo?alt=json",
        "redirect_uri": "http://localhost:6336/oauth/response",
        "allow_login": true
      }
    }
  }'
```

**Response:**
```json
{
  "data": {
    "type": "oauth_connect",
    "id": "019bf936-3dc0-7105-9cf9-468d766cae66",
    "attributes": {
      "name": "google-login",
      "client_id": "YOUR_CLIENT_ID.apps.googleusercontent.com",
      "client_secret": "52sx2TF-o_FBe5ap5l5wTiXaBQ5wTGto1FeHfn8ZjA==",
      "allow_login": 1,
      ...
    }
  }
}
```

Note: `client_secret` is automatically encrypted when stored.

### 2. Initiate OAuth Flow

```bash
# Call oauth_login_begin action on oauth_connect
curl -X POST http://localhost:6336/action/oauth_connect/oauth_login_begin \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "oauth_connect_id": "019bf936-3dc0-7105-9cf9-468d766cae66"
  }'
```

**Response:**
```json
[
  {
    "ResponseType": "client.store.set",
    "Attributes": {
      "key": "secret",
      "value": "101004"
    }
  },
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "delay": 0,
      "location": "https://accounts.google.com/o/oauth2/v2/auth?access_type=offline&client_id=YOUR_CLIENT_ID&redirect_uri=http%3A%2F%2Flocalhost%3A6336%2Foauth%2Fresponse%3Fauthenticator%3Dgoogle-login&response_type=code&scope=...&state=101004",
      "window": "self"
    }
  }
]
```

The action returns:
- **State token** (e.g., "101004") - TOTP-based CSRF protection, 5-minute validity
- **Redirect URL** - OAuth provider authorization endpoint with all parameters

### 3. User Authorizes (Browser Flow)

User is redirected to provider's authorization page → grants permissions → provider redirects back to your `redirect_uri` with:
- `code`: Authorization code
- `state`: Same state token from step 2

### 4. Handle OAuth Callback

```bash
# Provider redirects to: http://localhost:6336/oauth/response?code=AUTH_CODE&state=101004&authenticator=google-login

# Your application calls oauth.login.response action
curl -X POST http://localhost:6336/action/oauth_token/oauth.login.response \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "code": "4/0AfJohXm...",
      "state": "101004",
      "authenticator": "google-login"
    }
  }'
```

**Success Response:**
```json
[
  {
    "ResponseType": "client.store.set",
    "Attributes": {
      "key": "token",
      "value": "ya29.a0AfH6SM..."
    }
  },
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "title": "Successfully connected",
      "message": "You can use this connection now",
      "type": "success"
    }
  },
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "delay": 0,
      "location": "/in/item/oauth_token",
      "window": "self"
    }
  }
]
```

**Error Response (Invalid State):**
```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "No ongoing authentication",
      "title": "failed",
      "type": "error"
    }
  }
]
```

---

## Core Concepts

### OAuth 2.0 Authorization Code Flow

Daptin implements the standard OAuth 2.0 authorization code flow:

1. **Authorization Request** - User clicks "Sign in with Provider"
2. **User Consent** - Redirected to provider to grant permissions
3. **Authorization Code** - Provider redirects back with temporary code
4. **Token Exchange** - Server exchanges code for access/refresh tokens
5. **User Creation/Login** - Optionally create user account from profile

### State Validation (CSRF Protection)

- State tokens are generated using TOTP (Time-based One-Time Password)
- **Validity**: 5 minutes (300 seconds)
- **Algorithm**: SHA1 with 6 digits
- **Skew**: ±1 period (allows small clock drift)
- State tokens expire automatically for security

### Security Features

1. **Client Secret Encryption** - Secrets encrypted at rest in database
2. **State Token Validation** - CSRF protection prevents token hijacking
3. **HTTPS Recommended** - Always use HTTPS in production
4. **Token Encryption** - Access and refresh tokens encrypted in database

---

## Complete Examples

### Google OAuth

#### 1. Register OAuth Application

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing
3. Enable Google+ API
4. Create OAuth 2.0 credentials (Web application)
5. Add authorized redirect URI: `http://localhost:6336/oauth/response`
6. Copy Client ID and Client Secret

#### 2. Create oauth_connect

```bash
curl -X POST http://localhost:6336/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "google-login",
        "client_id": "123456789-abc.apps.googleusercontent.com",
        "client_secret": "GOCSPX-abc123...",
        "scope": "https://www.googleapis.com/auth/userinfo.email,https://www.googleapis.com/auth/userinfo.profile",
        "auth_url": "https://accounts.google.com/o/oauth2/v2/auth",
        "token_url": "https://oauth2.googleapis.com/token",
        "profile_url": "https://www.googleapis.com/oauth2/v1/userinfo?alt=json",
        "redirect_uri": "http://localhost:6336/oauth/response",
        "response_type": "code",
        "allow_login": true,
        "access_type_offline": false,
        "profile_email_path": "email"
      }
    }
  }'
```

#### 3. Google Profile Response Format

```json
{
  "id": "1234567890",
  "email": "user@gmail.com",
  "verified_email": true,
  "name": "John Doe",
  "given_name": "John",
  "family_name": "Doe",
  "picture": "https://lh3.googleusercontent.com/..."
}
```

### GitHub OAuth

#### 1. Register OAuth Application

1. Go to GitHub Settings → Developer settings → OAuth Apps
2. Click "New OAuth App"
3. Set Authorization callback URL: `http://localhost:6336/oauth/response`
4. Copy Client ID and Client Secret

#### 2. Create oauth_connect

```bash
curl -X POST http://localhost:6336/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "github-login",
        "client_id": "Iv1.abc123...",
        "client_secret": "1234567890abcdef...",
        "scope": "read:user,user:email",
        "auth_url": "https://github.com/login/oauth/authorize",
        "token_url": "https://github.com/login/oauth/access_token",
        "profile_url": "https://api.github.com/user",
        "redirect_uri": "http://localhost:6336/oauth/response",
        "response_type": "code",
        "allow_login": true,
        "profile_email_path": "email"
      }
    }
  }'
```

#### 3. GitHub Profile Response Format

```json
{
  "login": "johndoe",
  "id": 1234567,
  "email": "john@example.com",
  "name": "John Doe",
  "avatar_url": "https://avatars.githubusercontent.com/u/..."
}
```

### Microsoft OAuth

```bash
curl -X POST http://localhost:6336/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "microsoft-login",
        "client_id": "12345678-1234-1234-1234-123456789012",
        "client_secret": "abc~123...",
        "scope": "openid,profile,email",
        "auth_url": "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
        "token_url": "https://login.microsoftonline.com/common/oauth2/v2.0/token",
        "profile_url": "https://graph.microsoft.com/v1.0/me",
        "redirect_uri": "http://localhost:6336/oauth/response",
        "response_type": "code",
        "allow_login": true,
        "profile_email_path": "mail"
      }
    }
  }'
```

---

## Configuration Reference

### oauth_connect Table Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | - | Unique identifier for this OAuth connection |
| `client_id` | string | Yes | - | OAuth client ID from provider |
| `client_secret` | string | Yes | - | OAuth client secret (encrypted on save) |
| `scope` | string | Yes | Google Sheets scope | Comma-separated list of OAuth scopes |
| `auth_url` | string | Yes | Google auth endpoint | Provider's authorization endpoint |
| `token_url` | string | Yes | Google token endpoint | Provider's token exchange endpoint |
| `profile_url` | string | Yes | Google userinfo endpoint | Provider's user profile endpoint |
| `redirect_uri` | string | Yes | `/oauth/response` | Callback URL after authorization |
| `response_type` | string | No | `code` | OAuth response type (usually 'code') |
| `allow_login` | boolean | No | `false` | Enable user authentication via this provider |
| `access_type_offline` | boolean | No | `false` | Request refresh token for offline access |
| `profile_email_path` | string | No | `email` | JSON path to extract email from profile |

### Important Notes

1. **redirect_uri** must match exactly what's registered with OAuth provider
2. **Authenticator parameter** is automatically appended: `redirect_uri?authenticator={name}`
3. **Client secret** is automatically encrypted when stored in database
4. **access_type=offline** is automatically added to auth URL when multiple scopes are present

---

## OAuth Actions

### oauth_login_begin

Start OAuth authentication flow by generating authorization URL.

**Action:** `oauth_login_begin`
**On Type:** `oauth_connect`
**Endpoint:** `/action/oauth_connect/{reference_id}/oauth_login_begin` or `/action/oauth_connect/oauth_login_begin`

**Parameters:**
- `oauth_connect_id`: Reference ID of oauth_connect record

**Returns:**
- `client.store.set`: State token for CSRF validation
- `client.redirect`: OAuth provider authorization URL

**Example:**
```bash
curl -X POST http://localhost:6336/action/oauth_connect/oauth_login_begin \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "oauth_connect_id": "019bf936-3dc0-7105-9cf9-468d766cae66"
  }'
```

### oauth.login.response

Handle OAuth provider callback with authorization code.

**Action:** `oauth.login.response`
**On Type:** `oauth_token`
**Endpoint:** `/action/oauth_token/oauth.login.response`

**Parameters:**
- `code`: Authorization code from provider
- `state`: State token from oauth_login_begin
- `authenticator`: Name of oauth_connect record

**Returns:**
- Success: Token data + user profile + redirect to dashboard
- Error: Notification with error message

**Example:**
```bash
curl -X POST http://localhost:6336/action/oauth_token/oauth.login.response \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "code": "4/0AfJohXm...",
      "state": "101004",
      "authenticator": "google-login"
    }
  }'
```

**Workflow (when allow_login=true):**
1. Validates state token (TOTP)
2. Exchanges authorization code for access/refresh tokens
3. Stores tokens in oauth_token table (encrypted)
4. Fetches user profile from provider
5. Searches for existing user by email
6. If no user exists, creates new user account
7. Creates home usergroup for new user
8. Generates JWT token for user
9. Redirects to dashboard

---

## User Account Creation

When `allow_login` is `true`, Daptin automatically creates user accounts for new OAuth users.

### User Creation Flow

1. **Profile Retrieval**: Fetch user profile from `profile_url`
2. **Email Extraction**: Extract email using `profile_email_path`
3. **User Lookup**: Search for existing user by email
4. **Create User**: If not found, create new user_account:
   - `email`: From profile (via profile_email_path)
   - `name`: From profile.displayName or profile.name
   - `password`: Set to profile.id (OAuth users don't need password)
5. **Create Usergroup**: Create personal usergroup for user
6. **JWT Token**: Generate JWT token for immediate login

### Existing User Linking

If a user_account with matching email already exists:
- OAuth token is linked to existing user
- User is logged in with existing account
- No duplicate account created

---

## Token Management

### oauth_token Table

Stores OAuth access and refresh tokens for API access.

| Field | Type | Description |
|-------|------|-------------|
| `access_token` | string (encrypted) | OAuth access token for API requests |
| `refresh_token` | string (encrypted) | Refresh token for token renewal |
| `expires_in` | integer | Token lifetime in seconds |
| `token_type` | string | Token type (usually "Bearer") |

### Token Relationships

- `oauth_token` belongs_to `oauth_connect`
- `oauth_token` belongs_to `user_account` (when allow_login=true)

---

## Production Deployment

### HTTPS Configuration

Always use HTTPS in production for OAuth:

```bash
# Generate TLS certificates (see TLS-Certificates.md)
curl -X POST http://localhost:6336/action/world/generate_acme_tls_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "domain": "yourdomain.com",
      "email": "admin@yourdomain.com"
    }
  }'
```

Update oauth_connect `redirect_uri` to use HTTPS:
```
https://yourdomain.com/oauth/response
```

### Environment-Specific Configuration

**Development:**
```
redirect_uri: http://localhost:6336/oauth/response
```

**Production:**
```
redirect_uri: https://yourdomain.com/oauth/response
```

**Important:** Register both development and production redirect URIs with OAuth provider.

---

## Troubleshooting

### "No ongoing authentication"

**Problem:** State validation failed

**Causes:**
- State token expired (5-minute window)
- Invalid state parameter
- Clock drift between servers

**Solution:**
- Restart OAuth flow with fresh oauth_login_begin call
- Ensure system clocks are synchronized (NTP)
- Complete OAuth flow within 5 minutes

### Invalid client_id or client_secret

**Problem:** OAuth provider rejects credentials

**Causes:**
- Incorrect credentials copied from provider
- Credentials not yet activated (some providers have delay)
- IP restrictions on OAuth app

**Solution:**
- Verify credentials in provider console
- Wait a few minutes after creating OAuth app
- Check IP whitelist settings
- Test with provider's OAuth playground first

### Redirect URI mismatch

**Problem:** Provider shows "redirect_uri_mismatch" error

**Causes:**
- redirect_uri not registered with provider
- Mismatch between registered URI and configured URI
- Missing or extra trailing slash
- HTTP vs HTTPS mismatch

**Solution:**
- Check exact redirect_uri in provider console
- Ensure it matches oauth_connect.redirect_uri exactly
- Include authenticator parameter: `?authenticator={name}`
- Use same protocol (HTTP/HTTPS) as registered

### User profile email not found

**Problem:** Cannot extract email from profile response

**Causes:**
- Incorrect `profile_email_path`
- Missing email scope
- User hasn't verified email with provider

**Solution:**
- Check provider's profile response format
- Update `profile_email_path` to correct JSON path
- Request appropriate email scope (e.g., `user:email` for GitHub)
- Require verified email in OAuth app settings

### Tokens not refreshing

**Problem:** Access tokens expire and don't refresh

**Causes:**
- `access_type_offline` not enabled
- Missing `offline_access` scope (Microsoft)
- Refresh token not stored

**Solution:**
- Set `access_type_offline: true` in oauth_connect
- Add appropriate offline scope for provider
- Re-authorize user to get refresh token

---

## Common OAuth Providers

### Provider Configuration Quick Reference

| Provider | auth_url | token_url | profile_url | Scopes |
|----------|----------|-----------|-------------|--------|
| **Google** | `https://accounts.google.com/o/oauth2/v2/auth` | `https://oauth2.googleapis.com/token` | `https://www.googleapis.com/oauth2/v1/userinfo?alt=json` | `userinfo.email,userinfo.profile` |
| **GitHub** | `https://github.com/login/oauth/authorize` | `https://github.com/login/oauth/access_token` | `https://api.github.com/user` | `read:user,user:email` |
| **Microsoft** | `https://login.microsoftonline.com/common/oauth2/v2.0/authorize` | `https://login.microsoftonline.com/common/oauth2/v2.0/token` | `https://graph.microsoft.com/v1.0/me` | `openid,profile,email` |
| **Facebook** | `https://www.facebook.com/v12.0/dialog/oauth` | `https://graph.facebook.com/v12.0/oauth/access_token` | `https://graph.facebook.com/me?fields=id,name,email` | `email,public_profile` |

---

## Security Best Practices

1. **Always use HTTPS** in production
2. **Validate state tokens** (handled automatically)
3. **Encrypt client secrets** (handled automatically)
4. **Store tokens encrypted** (handled automatically)
5. **Use minimal scopes** - only request what you need
6. **Rotate client secrets** periodically
7. **Monitor OAuth usage** via audit logs
8. **Implement rate limiting** on OAuth endpoints
9. **Require email verification** with provider
10. **Handle token expiry** gracefully

---

## API Integration

### Using OAuth Tokens for API Calls

Once OAuth tokens are stored, use them for API requests:

```javascript
// Frontend: Initiate OAuth
fetch('/action/oauth_connect/oauth_login_begin', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${userToken}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    oauth_connect_id: 'google-login-id'
  })
})
.then(res => res.json())
.then(data => {
  // Store state
  localStorage.setItem('oauth_state', data[0].Attributes.value);
  // Redirect to OAuth provider
  window.location.href = data[1].Attributes.location;
});

// Handle OAuth callback
const urlParams = new URLSearchParams(window.location.search);
fetch('/action/oauth_token/oauth.login.response', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${userToken}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    attributes: {
      code: urlParams.get('code'),
      state: urlParams.get('state'),
      authenticator: urlParams.get('authenticator')
    }
  })
})
.then(res => res.json())
.then(data => {
  // OAuth successful, user logged in
  const newToken = data[0].Attributes.value;
  localStorage.setItem('token', newToken);
});
```

---

## Related Documentation

- [Action Reference](Action-Reference.md) - All available actions
- [User Management](User-Management.md) - User account operations
- [Permissions](Permissions.md) - Access control
- [TLS Certificates](TLS-Certificates.md) - HTTPS setup
- [Security Best Practices](Security.md) - Security guidelines

---

## Log Messages

### Expected Log Output

When OAuth operations execute, you'll see these log messages:

**Creating oauth_connect:**
```
[GIN] 2026/01/26 - 13:00:40 | 201 | 2.297208ms | ::1 | POST "/api/oauth_connect"
```

**Starting OAuth flow (oauth_login_begin):**
```
[INFO][2026-01-26 13:00:47] [google-test] oauth config: &{test-client-id.apps.googleusercontent.com...}
Visit the URL for the auth dialog: https://accounts.google.com/o/oauth2/v2/auth?access_type=offline&client_id=test-client-id.apps.googleusercontent.com&redirect_uri=http%3A%2F%2Flocalhost%3A6336%2Foauth%2Fresponse%3Fauthenticator%3Dgoogle-test&response_type=code&scope=...&state=101004
[GIN] 2026/01/26 - 13:00:47 | 200 | 2.618417ms | ::1 | POST "/action/oauth_connect/oauth_login_begin"
```

**Successful OAuth callback:**
```
[GIN] 2026/01/26 - 13:00:59 | 200 | 15.234ms | ::1 | POST "/action/oauth_token/oauth.login.response"
```

**Failed state validation:**
```
[ERRO][2026-01-26 13:00:59] Failed to validate otp key
[GIN] 2026/01/26 - 13:00:59 | 500 | 2.359125ms | ::1 | POST "/action/oauth_token/oauth.login.response"
```

### Debugging OAuth Issues

To monitor OAuth operations in real-time:

```bash
# Watch all logs
./scripts/testing/test-runner.sh logs

# Watch only errors
./scripts/testing/test-runner.sh errors

# Filter OAuth-specific logs
./scripts/testing/test-runner.sh logs | grep -i oauth

# Follow logs in real-time
tail -f /tmp/daptin.log | grep oauth
```

**Key log messages to look for:**

- `Visit the URL for the auth dialog:` - OAuth URL generated successfully
- `oauth config:` - OAuth connection loaded, shows decrypted config
- `Failed to validate otp key` - State token validation failed (expired or invalid)
- `Failed to exchange code for token` - Token exchange with provider failed
- HTTP 200 on oauth.login.response - Successful OAuth flow
- HTTP 500 on oauth.login.response - OAuth flow failed (check error logs)

---

## Testing Status

**Tested Infrastructure ✓:**
- oauth_connect record creation and database persistence
- Client secret encryption
- oauth_login_begin action and authorization URL generation
- State token generation (TOTP-based)
- Invalid/expired state rejection
- Redirect URI formatting

**Requires Real OAuth Provider ⚠:**
- Complete OAuth callback flow
- Token exchange with real authorization codes
- User profile retrieval from actual providers
- Automatic user account creation
- Refresh token handling

**Test Environment:** Fresh database, Daptin running on localhost:6336

---

## Additional Notes

- State tokens use TOTP with 300-second period and SHA1 algorithm
- Redirect URIs automatically append `?authenticator={name}` parameter
- Multiple scopes trigger `access_type=offline` in authorization URL
- Profile email path supports dot notation (e.g., "emails[0].value")
- Token expiry is tracked but automatic refresh not shown in basic flow

For questions or issues, see [Common Errors](Common-Errors.md) or file an issue on GitHub.
