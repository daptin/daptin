# OAuth Authentication

**Tested ✓ 2026-01-26** (Infrastructure and parameter validation tested; full provider integration requires real OAuth credentials)

OAuth 2.0 authentication enables users to sign in using external providers like Google, GitHub, Microsoft, Facebook, and others. Daptin implements the standard OAuth 2.0 Authorization Code Flow.

This page covers Daptin as an OAuth consumer: Daptin connects to an upstream provider and stores provider tokens in `oauth_token`. For Daptin acting as an OAuth/OIDC provider to other applications, see [[OAuth-Provider|OAuth Provider]].

For browser sign-in, keep the roles separate:

| Role | Responsibility |
|------|----------------|
| OAuth provider | Authenticates the user and redirects back with an authorization code |
| Daptin OAuth client backend | Starts OAuth, validates state, exchanges the code, stores provider tokens, creates or links the local Daptin user, and creates the Daptin client session |
| Browser-facing frontend origin | The host the user actually visits; it must receive the callback if the resulting browser session should belong to that frontend |

The OAuth callback must be on the browser-facing OAuth client origin. If an app runs at `https://app.example.com` and proxies to a Daptin OAuth client backend, configure `oauth_connect.redirect_uri` as `https://app.example.com/oauth/response`, not as an unrelated admin dashboard or backend-only host. Cookies and browser storage are origin-scoped; a callback completed on one host does not log the browser into another host.

---

## Quick Start (5 minutes)

### Prerequisites
- OAuth provider account (Google, GitHub, etc.)
- Registered OAuth application with provider
- Client ID and Client Secret from provider

### 1. Create OAuth Connection

Use the browser-facing OAuth client origin in `redirect_uri`. For a direct local Daptin server this can be `http://localhost:6336/oauth/response`. For an app or router in front of Daptin, use that public origin, for example `https://app.example.com/oauth/response`.

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
        "allow_login": true,
        "access_type_offline": true,
        "pkce_enabled": false,
        "pkce_challenge_method": "S256"
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

For browser sign-in, redirect the browser to the generic Daptin login start URL:

```text
GET http://localhost:6336/oauth/login/google-login
```

This route is only for `oauth_connect` rows where `allow_login=true`. It starts the same `oauth_login_begin` action, creates the server-side OAuth state, and redirects the browser to the configured provider `auth_url`.

If a frontend proxy is the user-facing origin, start from that origin instead:

```text
GET https://app.example.com/oauth/login/google-login
```

The frontend proxy should forward this request to the Daptin OAuth client backend without changing the externally visible host used by the browser.

Authenticated applications and dashboards can also call the action directly:

```bash
# oauth_login_begin is an instance action on oauth_connect.
# The selected row supplies the authenticator name and configured scope.
curl -X POST http://localhost:6336/action/oauth_connect/oauth_login_begin \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "oauth_connect_id": "019bf936-3dc0-7105-9cf9-468d766cae66"
    }
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
- **State token** - TOTP-based CSRF protection for non-PKCE flows, or a stored random state for PKCE flows
- **Redirect URL** - OAuth provider authorization endpoint with all parameters

### 3. User Authorizes (Browser Flow)

User is redirected to provider's authorization page → grants permissions → provider redirects back to your `redirect_uri` with:
- `code`: Authorization code
- `state`: Same state token from step 2
- `authenticator`: The `oauth_connect.name` that started the flow

### 4. Handle OAuth Callback

```bash
# Provider redirects to: http://localhost:6336/oauth/response?code=AUTH_CODE&state=101004&authenticator=google-login
```

For browser flows, Daptin now provides the default callback endpoint:

```text
GET /oauth/response
```

That endpoint reads `code`, `state`, and `authenticator`, runs the existing `oauth.login.response` action, applies any returned Daptin session cookie, and follows the final safe same-origin redirect.

For proxied frontends, the browser should see this endpoint on the frontend origin, for example:

```text
https://app.example.com/oauth/response?code=AUTH_CODE&state=101004&authenticator=google-login
```

The proxy forwards it to the Daptin OAuth client backend. The response then belongs to `app.example.com`, so any Daptin client session cookie is scoped to the app users actually use.

You can still call the action directly from an application or test:

```bash
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

## Browser Client Contract

A browser client does not create OAuth state and does not exchange the code itself. Daptin owns those server-side steps.

1. Configure an `oauth_connect` row with `name`, `client_id`, `client_secret`, `scope`, `auth_url`, `token_url`, `profile_url`, `redirect_uri`, and `allow_login`.
2. Set `oauth_connect.redirect_uri` to the browser-facing callback base URL:

```text
https://app.example.com/oauth/response
```

For direct local development this can be:

```text
http://localhost:6336/oauth/response
```

Do not use an admin dashboard host for a user-facing app login unless the desired result is logging into that admin dashboard host.

3. Register the exact callback URL with the provider. For Daptin browser login, this is the consumer callback plus the authenticator query parameter:

```text
https://app.example.com/oauth/response?authenticator=google-login
```

Daptin appends `authenticator=<oauth_connect.name>` when it builds the provider authorization URL.

4. Start browser login with one of these:
   - Public sign-in page: link to `/oauth/login/<oauth_connect.name>` on the same browser-facing origin as the callback.
   - Authenticated dashboard/app: call `/action/oauth_connect/oauth_login_begin` for the selected row and navigate to the returned `client.redirect.location`.

5. Let the provider redirect back to `/oauth/response`. Daptin validates `state`, exchanges the authorization code at `token_url`, fetches the profile from `profile_url`, stores the token, and, when `allow_login=true`, creates or finds the Daptin user and sets the normal HttpOnly Daptin session cookie.

The browser callback endpoint is for browser redirects. API clients and tests can still call `/action/oauth_token/oauth.login.response` directly when they already have an appropriate Daptin session.

No Daptin-specific provider behavior is required. The provider can be Google, GitHub, another Daptin instance, or any compatible OAuth 2.0 authorization-code provider.

### Proxying Daptin Behind an App Frontend

When Daptin is the OAuth client backend for an app frontend, the frontend origin should proxy the OAuth client routes and the authenticated API routes it needs.

Minimum browser routes:

```text
/oauth/login/:authenticator
/oauth/response
```

Minimum API routes depend on the app. Calls that rely on the Daptin client session should go through the same browser-facing origin so the browser sends the session cookie.

Example:

| Component | Example |
|-----------|---------|
| User-facing app | `https://app.example.com` |
| Daptin OAuth client backend | internal Daptin service |
| Upstream OAuth provider | Google, GitHub, or another Daptin provider |
| `oauth_connect.redirect_uri` | `https://app.example.com/oauth/response` |
| Provider registered redirect URI | `https://app.example.com/oauth/response?authenticator=google-login` |
| Start URL shown to users | `https://app.example.com/oauth/login/google-login` |

This is still a standard OAuth client/server flow. The provider only redirects to the registered client callback. The Daptin OAuth client backend owns code exchange and local session creation. The frontend origin owns the browser session because it is the host the browser sees.

## Daptin Consuming Daptin OAuth Provider

This is useful when one Daptin instance should be the identity provider for another app/router. The two instances communicate through the standard OAuth HTTP endpoints; the consumer does not need any Daptin-specific provider logic.

1. Register an OAuth client on the provider side using [[OAuth-Provider|OAuth Provider]]:

```bash
curl -X POST http://localhost:6337/action/oauth_app/register_client \
  -H "Authorization: Bearer $PROVIDER_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "name": "Daptin Login",
      "redirect_uris": "http://localhost:6336/oauth/response?authenticator=daptin-login",
      "scopes": "openid profile email",
      "grants": "authorization_code,refresh_token",
      "is_confidential": true
    }
  }'
```

2. Create an `oauth_connect` row that points back to the Daptin provider:

```bash
curl -X POST http://localhost:6336/api/oauth_connect \
  -H "Authorization: Bearer $CONSUMER_ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "daptin-login",
        "client_id": "CLIENT_ID_FROM_REGISTER_CLIENT",
        "client_secret": "CLIENT_SECRET_FROM_REGISTER_CLIENT",
        "scope": "openid,profile,email",
        "auth_url": "http://localhost:6337/oauth/authorize",
        "token_url": "http://localhost:6337/oauth/token",
        "profile_url": "http://localhost:6337/oauth/userinfo",
        "redirect_uri": "http://localhost:6336/oauth/response",
        "allow_login": true,
        "access_type_offline": true,
        "pkce_enabled": true,
        "pkce_challenge_method": "S256"
      }
    }
  }'
```

The registered OAuth app redirect URI must include the authenticator query parameter because Daptin appends it when starting the consumer flow.

3. Start login by sending the browser to the consumer:

```text
http://localhost:6336/oauth/login/daptin-login
```

The consumer redirects to provider `/oauth/authorize`. If the browser does not already have a provider-side session, the provider redirects to `/auth/signin?return_to=...`. After password login, the provider returns to `/oauth/authorize`, issues the code, redirects to the consumer `/oauth/response`, and the consumer completes `oauth.login.response` through `token_url` and `profile_url`.

If a separate frontend sits in front of the consumer, replace `localhost:6336` in the consumer callback and start URL with that frontend origin, and proxy the OAuth client routes to the consumer backend. The provider does not need to know that the upstream provider is Daptin; it only needs the exact registered redirect URI.

## Core Concepts

### OAuth 2.0 Authorization Code Flow

Daptin implements the standard OAuth 2.0 authorization code flow:

1. **Authorization Request** - User clicks "Sign in with Provider"
2. **User Consent** - Redirected to provider to grant permissions
3. **Authorization Code** - Provider redirects back with temporary code
4. **Token Exchange** - Server exchanges code for access/refresh tokens
5. **User Creation/Login** - Optionally create user account from profile

### State Validation (CSRF Protection)

Daptin supports two state modes:

- **Non-PKCE**: state tokens are generated using TOTP (Time-based One-Time Password), valid for a 300 second period, using SHA1 with 6 digits and ±1 period skew.
- **PKCE enabled**: when `oauth_connect.pkce_enabled` is true, Daptin creates a random state, stores the hashed state and code verifier in `oauth_state`, and sends a PKCE code challenge using `pkce_challenge_method`.

The OAuth callback includes `authenticator=<oauth_connect.name>`, which lets `oauth.login.response` load the same provider configuration for state validation and token exchange.

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
5. Add authorized redirect URI: `http://localhost:6336/oauth/response?authenticator=google-login`
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
3. Set Authorization callback URL: `http://localhost:6336/oauth/response?authenticator=github-login`
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
| `redirect_uri` | string | Yes | `/oauth/response` | Browser-facing callback base URL after authorization |
| `response_type` | string | No | `code` | OAuth response type (usually 'code') |
| `allow_login` | boolean | No | `false` | Enable user authentication via this provider |
| `access_type_offline` | boolean | No | `false` | Request refresh token for offline access |
| `pkce_enabled` | boolean | No | `false` | Store a random state and code verifier for PKCE authorization code flow |
| `pkce_challenge_method` | string | No | `S256` | PKCE challenge method; `S256` is recommended |
| `profile_email_path` | string | No | `email` | JSON path to extract email from profile |

### Important Notes

1. Register the exact browser-facing callback URL with the provider, including `authenticator={name}`
2. Store the browser-facing base callback in `oauth_connect.redirect_uri`; Daptin automatically appends `?authenticator={name}` or `&authenticator={name}`
3. **Client secret** is automatically encrypted when stored in database
4. **access_type=offline** is added to the auth URL when `access_type_offline` is true

---

## OAuth Actions

### oauth_login_begin

Start OAuth authentication flow by generating authorization URL.

**Action:** `oauth_login_begin`
**On Type:** `oauth_connect`
**Endpoint:** `/action/oauth_connect/oauth_login_begin`
**Browser Endpoint:** `/oauth/login/<authenticator>`

**Parameters:**
- `oauth_connect_id`: Reference ID of the `oauth_connect` record, passed inside `attributes`

The action has no provider-specific input fields. It is an instance action: the selected `oauth_connect` row provides `$.name` as the callback authenticator and `$.scope` as the requested scope.

**Returns:**
- `client.store.set`: State token for CSRF validation
- `client.redirect`: OAuth provider authorization URL

**Example:**
```bash
curl -X POST http://localhost:6336/action/oauth_connect/oauth_login_begin \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "oauth_connect_id": "019bf936-3dc0-7105-9cf9-468d766cae66"
    }
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
1. Validates state token (TOTP for non-PKCE flows, `oauth_state` lookup for PKCE flows)
2. Exchanges authorization code for access/refresh tokens
3. Stores tokens in oauth_token table (encrypted)
4. Fetches user profile from provider
5. Searches for existing user by email
6. If no user exists, creates new user account
7. Creates home usergroup for new user
8. Generates JWT token for user
9. Redirects to dashboard

---

## OAuth for API Integrations

Set `allow_login` to `false` when the connection is for API access instead of user login. The OAuth flow still creates an `oauth_token` row for the current user.

OpenAPI integrations should store only provider-level auth wiring:

```json
{
  "authentication_type": "oauth2",
  "authentication_specification": {
    "oauth_connect_id": "OAUTH_CONNECT_REFERENCE_ID"
  }
}
```

Each operation execution supplies the user's token reference:

```bash
curl -X POST "http://localhost:6336/integration/asana.com/getWorkspaces" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "oauth_token_id": "USER_OAUTH_TOKEN_REFERENCE_ID",
    "input": {
      "opt_fields": ["name"]
    }
  }'
```

Daptin validates that the token belongs to the current user and was issued for the same `oauth_connect_id` configured on the integration.

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

### Retrieving Stored Tokens

Use the `get_token` action to retrieve decrypted OAuth tokens for API calls:

**Action:** `get_token`
**On Type:** `oauth_token`
**Endpoint:** `/action/oauth_token/{referenceId}/get_token`
**Instance Required:** Yes (must specify the oauth_token record)

```bash
# Get the oauth_token reference ID
TOKEN_REF=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/oauth_token | jq -r '.data[0].id')

# Retrieve the decrypted token
curl -X POST "http://localhost:6336/action/oauth_token/$TOKEN_REF/get_token" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {}}'
```

**Response:**
```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "access_token": "ya29.a0AfH6SM...",
      "refresh_token": "1//0dx...",
      "expiry": "2026-03-22T15:00:00Z"
    }
  }
]
```

The access and refresh tokens are stored encrypted in the database and decrypted only when retrieved through this action.

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

Update oauth_connect `redirect_uri` to use HTTPS on the browser-facing client origin:
```
https://yourdomain.com/oauth/response
```

### Environment-Specific Configuration

**Development:**
```
redirect_uri: http://localhost:6336/oauth/response
```

**Production direct to Daptin:**
```
redirect_uri: https://yourdomain.com/oauth/response
```

**Production behind an app frontend or router:**
```
redirect_uri: https://app.yourdomain.com/oauth/response
```

**Important:** Register the generated callback URLs with the OAuth provider, including `?authenticator=<oauth_connect.name>`.

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
- Ensure the provider registration matches the generated callback exactly
- Store only the base callback in `oauth_connect.redirect_uri`
- Include authenticator parameter in the provider registration: `?authenticator={name}`
- Use the browser-facing app/router host if the user flow is proxied through one
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

### Using OAuth Tokens for Integration Calls

Once OAuth tokens are stored, integration actions can use them. The integration stores only the provider connection reference; the executing user supplies the token reference at execution time.

```javascript
// Frontend: Initiate OAuth
fetch('/action/oauth_connect/oauth_login_begin', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${userToken}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    attributes: {
      oauth_connect_id: 'google-login-reference-id'
    }
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

- [[Action-Reference|Action Reference]] - All available actions
- [[User-Management|User Management]] - User account operations
- [[Permissions|Permissions]] - Access control
- [[TLS-Certificates|TLS Certificates]] - HTTPS setup
- [[Security|Security Best Practices]] - Security guidelines

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
- `access_type_offline=true` adds offline-access OAuth parameters when generating the authorization URL
- Profile email path supports dot notation (e.g., "emails[0].value")
- Token expiry is tracked but automatic refresh not shown in basic flow

## Using OAuth Tokens With Integrations

Integrations reuse the same OAuth connection flow documented above. There is no separate integration-specific OAuth callback.

1. Create an `oauth_connect` record for the provider.
2. The user completes `oauth_login_begin` and `oauth.login.response`.
3. Daptin stores that user's encrypted token in `oauth_token`.
4. The integration stores the provider/app reference in `authentication_specification.oauth_connect_id`.
5. The installed integration action call supplies the current user's `oauth_token_id` in `attributes`.

Example OAuth integration auth configuration:

```json
{
  "authentication_type": "oauth2",
  "authentication_specification": {
    "oauth_connect_id": "OAUTH_CONNECT_REFERENCE_ID"
  }
}
```

Example integration execution:

```bash
curl -X POST "http://localhost:6336/action/integration/listRepos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "oauth_token_id": "USER_OAUTH_TOKEN_REFERENCE_ID"
    }
  }'
```

Daptin validates that the supplied token belongs to the current user and matches the integration's configured `oauth_connect_id` before using it for the outbound request.

The integration must not store `oauth_token_id`. A token is user-specific and is selected per execution, which prevents one user's installed integration from accidentally or maliciously using another user's OAuth token.

If an OpenAPI operation exposes an auth-looking header or query parameter, Daptin protects the resolved OAuth auth fields. A user-supplied action attribute such as `Authorization` cannot override the bearer token Daptin resolved from `oauth_token_id`.

For questions or issues, see [[Common-Errors|Common Errors]] or file an issue on GitHub.
