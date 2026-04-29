# OAuth Provider Technical KT

This page explains how Daptin acts as an OAuth 2.0 and OpenID Connect provider internally. It is for maintainers who need to extend, debug, or review the implementation.

For operator usage, see [[OAuth-Provider]].

## Design Boundary

Daptin has two OAuth roles:

| Role | Direction | Existing tables/code |
|------|-----------|----------------------|
| OAuth consumer | Daptin receives and stores tokens from another provider | `oauth_connect`, `oauth_token`, `server/actions/action_oauth_*.go`, `server/resource/oauth_server.go` |
| OAuth provider | Daptin issues OAuth tokens to client apps | `oauth_app`, `oauth_code`, `oauth_access`, `oauth_refresh`, `oauth_grant`, `oauth_key`, `server/endpoint_oauth.go`, `server/resource/oauth_provider.go` |

Do not merge the two models. `oauth_connect` and `oauth_token` continue to represent upstream provider configuration and upstream tokens. Provider-side tables use compact names to avoid long table names and keep Daptin resource names manageable.

## Files

| File | Responsibility |
|------|----------------|
| `server/resource/columns.go` | Declares OAuth provider entities and relations |
| `server/server.go` | Registers OAuth provider routes with the main Gin router |
| `server/endpoint_oauth.go` | HTTP endpoint handlers and OAuth/OIDC response formatting |
| `server/resource/oauth_provider.go` | Provider service: app lookup, client auth, token hashing, PKCE, token lifecycle, JWKS, ID token signing |
| `server/resource/oauth_provider_test.go` | Focused tests for token hashing and PKCE |
| `server_test.go` | End-to-end OAuth provider flow inside `TestServerApis` |

## Route Registration

`server/server.go` calls:

```go
InitializeOAuthResources(cruds, configStore, defaultRouter)
```

The function lives in `server/endpoint_oauth.go` and registers:

```text
GET  /.well-known/oauth-authorization-server
GET  /.well-known/openid-configuration
GET  /oauth/jwks
GET  /oauth/authorize
POST /oauth/token
POST /oauth/revoke
POST /oauth/introspect
GET  /oauth/userinfo
POST /oauth/userinfo
```

This follows the same Daptin style as protocol endpoints such as SMTP/IMAP support: the endpoint file owns protocol HTTP behavior, while resource/service code owns ORM access and persistence rules.

## Entity Model

The provider tables are declared in `server/resource/columns.go`.

| Table | Visibility | Purpose |
|-------|------------|---------|
| `oauth_app` | Hidden | OAuth client registration storage |
| `oauth_code` | Hidden | Authorization code state |
| `oauth_access` | Hidden | Access token state |
| `oauth_refresh` | Hidden | Refresh token state |
| `oauth_grant` | Admin-visible | Reserved grant/consent records |
| `oauth_key` | Admin-visible | OIDC signing keys |

Relations:

```text
oauth_code    belongs_to oauth_app
oauth_access  belongs_to oauth_app
oauth_refresh belongs_to oauth_app
oauth_grant   belongs_to oauth_app
```

`oauth_key` is intentionally global and does not belong to `oauth_app`. One active signing key can sign ID tokens for all clients.

## Persistence Style

The provider service receives Daptin `DbResource` instances:

```go
type OAuthProvider struct {
    cruds       map[string]*DbResource
    configStore *ConfigStore
}
```

Reads use existing DbResource methods, for example:

```go
op.cruds["oauth_app"].GetRowsByWhereClauseWithTransaction(...)
```

Protocol rows are inserted with `createInternalRow`. This uses `statementbuilder.Squirrel.Insert` inside the current transaction and sets Daptin baseline columns such as:

- `reference_id`
- `permission`
- `created_at`
- `updated_at`

Reason: hidden protocol rows such as `oauth_code`, `oauth_access`, and `oauth_refresh` need to reference admin-owned `oauth_app` rows while operating on behalf of a normal signed-in user. The normal public create path performs user-facing permission checks that are correct for APIs but too restrictive for internal protocol state.

The handler still keeps transactions explicit:

```text
BeginTransaction
defer Rollback
validate
mutate
Commit
```

OAuth client management uses Daptin actions instead of exposing `oauth_app` CRUD as the management contract.

| Action | Performer | Notes |
|--------|-----------|-------|
| `register_client` | `oauth.client.register` | Creates `oauth_app`, generates `client_id`, returns `client_secret` once |
| `update_client` | `oauth.client.update` | Updates allowed metadata fields |
| `rotate_client_secret` | `oauth.client.rotate_secret` | Generates a new bcrypt-stored secret and returns it once |
| `disable_client` | `oauth.client.disable` | Sets `is_enabled=false` |
| `enable_client` | `oauth.client.enable` | Sets `is_enabled=true` |
| `revoke_client_tokens` | `oauth.client.revoke_tokens` | Revokes `oauth_access` and `oauth_refresh` rows for the app |

The performers live in `server/actions/action_oauth_client.go` and are registered from `server/action_provider/action_provider.go`.

## Authorization Request Flow

Endpoint: `GET /oauth/authorize`

Implementation path:

```text
oauthAuthorizeHandler
  provider.BeginTransaction
  provider.GetAppByClientID
  provider.ValidateRedirectURI
  provider.HasGrant("authorization_code")
  provider.NormalizeScopes
  require code_challenge
  read Daptin session user from request context
  provider.CreateCode
  transaction.Commit
  redirect back with code and state
```

Security details:

- `response_type` must be `code`.
- Client must exist and `is_enabled` must be true.
- `redirect_uri` must exactly match `oauth_app.redirect_uris`.
- Invalid redirect URIs return HTTP 400 directly. They are not redirected.
- `authorization_code` must be listed in `oauth_app.grants`.
- Requested scopes must be a subset of `oauth_app.scopes`.
- `code_challenge` is required before an authorization code is created.
- `code_challenge_method` can be `S256` or `plain`.
- User must already be authenticated through Daptin middleware.
- If no user is present, the handler redirects to backend config `oauth.login_url` or `/auth/signin`.

There is no consent screen yet. `oauth_grant` is available for a future consent implementation.

## Authorization Code Exchange

Endpoint: `POST /oauth/token` with `grant_type=authorization_code`

Implementation path:

```text
oauthTokenHandler
  oauthClientCredentials
  provider.AuthenticateClient
  provider.ExchangeCode
    lookup oauth_code by SHA-256(code)
    check expiry
    check used_at is empty
    check redirect_uri
    check code belongs to authenticated app
    validate PKCE verifier
    load user_account
    mark code used
    create access and refresh token rows
  Commit
  return bearer token response
  optionally sign id_token if scope includes openid
```

Important behavior:

- Authorization codes are opaque random values returned to the client once.
- Only `code_hash` is stored.
- Codes expire after 10 minutes.
- Codes are single-use through `used_at`.
- PKCE is enforced during exchange through `validatePKCE`.

## Refresh Flow

Endpoint: `POST /oauth/token` with `grant_type=refresh_token`

Implementation path:

```text
provider.Refresh
  lookup oauth_refresh by SHA-256(refresh_token)
  check expiry and revoked_at
  check token belongs to authenticated app
  load user_account
  revoke old refresh token
  create new access and refresh token rows
```

Refresh tokens rotate. Replaying an old refresh token returns `invalid_grant`.

## Client Authentication

`oauthClientCredentials` supports:

- HTTP Basic auth
- `client_id` and `client_secret` form fields
- `client_id` only for public clients

`provider.AuthenticateClient` behavior:

```text
load oauth_app by client_id
require is_enabled
if is_confidential is false, accept without secret
if is_confidential is true, require bcrypt client_secret match
```

The `client_secret` field is declared as a password/bcrypt column in `oauth_app`.

## Token Storage

Token generation:

```go
OAuthRandomToken()
```

Uses `crypto/rand`, 32 bytes, base64url encoding.

Token storage:

```go
OAuthHashToken(token)
```

Stores SHA-256 hex in:

- `oauth_code.code_hash`
- `oauth_access.token_hash`
- `oauth_refresh.token_hash`

Raw authorization codes, access tokens, and refresh tokens are not stored.

## Token Lifetimes

| Token | Constant | Lifetime |
|-------|----------|----------|
| Authorization code | `OAuthCodeLifetimeSeconds` | 600 seconds |
| Access token | `OAuthAccessTokenLifetimeSeconds` | 3600 seconds |
| Refresh token | `OAuthRefreshTokenLifetimeSeconds` | 30 days |

## Introspection

Endpoint: `POST /oauth/introspect`

Implementation path:

```text
authenticate client
provider.ValidateAccessToken
if valid:
  return active=true, client_id, scope, token_type, exp, sub
else:
  return active=false
```

Currently introspection validates access tokens. Refresh token introspection is not exposed.

## Revocation

Endpoint: `POST /oauth/revoke`

Implementation path:

```text
authenticate client
provider.RevokeToken
  hash token
  update oauth_access.revoked_at
  update oauth_refresh.revoked_at
Commit
```

The endpoint returns HTTP 200 even if the token is unknown, matching OAuth revocation behavior.

## UserInfo

Endpoint: `GET|POST /oauth/userinfo`

Implementation path:

```text
bearerToken
provider.ValidateAccessToken
load linked user_account
return sub, email, name
```

The handler reads bearer tokens from:

- `Authorization: Bearer ...`
- form field `access_token`

## OIDC ID Tokens and JWKS

ID token path:

```text
makeIDToken
  provider.BeginTransaction
  provider.SignIDToken
    provider.ActiveSigningKey
      load active oauth_key
      or createSigningKey
    sign jwt with RS256 and kid
  Commit
```

JWKS path:

```text
/oauth/jwks
  provider.JWKS
    provider.ActiveSigningKey
    return public JWK
```

Signing keys:

- RSA 2048-bit
- Algorithm `RS256`
- Public key stored as PEM in `oauth_key.public_key`
- Private key stored encrypted in `oauth_key.private_key`
- `key_id` is used as JWT `kid`

The first request that needs a signing key creates one lazily.

## Configuration

| Key | Used by | Behavior |
|-----|---------|----------|
| `oauth.issuer` | metadata and ID token signing | If set, trimmed and used as issuer |
| `oauth.login_url` | authorize handler | Login target when no Daptin user is in request context |
| `encryption.secret` | signing key load/create | Encrypts and decrypts `oauth_key.private_key` |

If `oauth.issuer` is not set, issuer is inferred from request scheme and host, using `X-Forwarded-Proto` when present.

## E2E Coverage

`server_test.go` includes `runOAuthProviderE2ETests` inside `TestServerApis`.

Covered behavior:

- Register `oauth_app` through `/action/oauth_app/register_client`.
- Rotate the generated client secret through `/action/oauth_app/rotate_client_secret`.
- Disable and re-enable the client through instance-bound actions.
- Reject invalid redirect URI with HTTP 400.
- Reject missing PKCE challenge without issuing a code.
- Authorize with S256 PKCE.
- Exchange authorization code for access token, refresh token, and ID token.
- Reject authorization code replay.
- Refresh and rotate tokens.
- Reject refresh token replay.
- Call UserInfo.
- Introspect active access token.
- Revoke access token.
- Introspect inactive token.
- Fetch JWKS.

Focused unit coverage in `server/resource/oauth_provider_test.go` verifies:

- SHA-256 token hashing is stable hex.
- RFC 7636 S256 PKCE output.
- PKCE verifier validation.

## Security Review Checklist

Before changing this code, verify:

- Redirect URI validation happens before any redirect to client input.
- Authorization codes stay single-use.
- PKCE remains required for code issuance.
- PKCE S256 remains supported and tested.
- Access and refresh tokens are stored only as hashes.
- Refresh tokens rotate and old refresh tokens are revoked.
- Confidential clients still require bcrypt secret validation.
- Public clients still require PKCE.
- Token responses keep `Cache-Control: no-store` and `Pragma: no-cache`.
- ID tokens use RS256 and a valid `kid`.
- Private signing keys stay encrypted at rest.
- Hidden protocol rows are inserted with Daptin baseline columns and inside the active transaction.

## Known Follow-ups

- Add an explicit consent UI and persist accepted grants in `oauth_grant`.
- Add admin UI affordances for client app registration and key rotation.
- Add key rotation controls for `oauth_key`.
- Consider refresh token introspection if needed by resource servers.
- Consider dynamic client registration only if there is a concrete product need.
