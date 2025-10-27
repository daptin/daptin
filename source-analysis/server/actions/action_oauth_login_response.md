# action_oauth_login_response.go

**File:** server/actions/action_oauth_login_response.go

## Code Summary

### Type: oauthLoginResponseActionPerformer (lines 21-26)
**Fields:**
- `responseAttrs map[string]interface{}` - Response attributes (unused in implementation)
- `cruds map[string]*resource.DbResource` - Database resource access map
- `configStore *resource.ConfigStore` - Configuration storage
- `otpKey string` - TOTP secret key for state validation

### Function: Name() (lines 28-30)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"oauth.login.response"`

### Function: GetOauthConnectionDescription() (lines 32-58)
**Inputs:**
- `authenticator string` - OAuth authenticator name
- `dbResource *resource.DbResource` - Database resource for oauth_connect
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Database Query (lines 34-46):**
- Lines 34-35: Queries oauth_connect table: `dbResource.Cruds["oauth_connect"].GetRowsByWhereClauseWithTransaction("oauth_connect", nil, transaction, goqu.Ex{"name": authenticator})`
- Lines 37-40: Error handling for database query failure
- Lines 42-46: Validation that results exist

**2. Encryption Secret Retrieval (lines 48-52):**
- Line 48: Gets encryption secret: `dbResource.ConfigStore.GetConfigValueFor("encryption.secret", "backend", transaction)`
- Lines 49-52: Error handling for secret retrieval failure

**3. OAuth Config Mapping (lines 54-56):**
- Line 54: Maps database row to OAuth config: `mapToOauthConfig(rows[0], secret)`
- Line 55: Debug logging of OAuth config
- Line 56: Returns config and reference ID

**Output:**
- Returns `(*oauth2.Config, daptinid.DaptinReferenceId, error)`

**Edge Cases:**
- **Debug logging:** OAuth config printed to logs (line 55) - potential secret exposure
- **No input validation:** Authenticator name not validated before database query
- **Single result assumption:** Uses `rows[0]` without checking array bounds after length check

### Function: mapToOauthConfig() (lines 60-90)
**Inputs:**
- `authConnectorData map[string]interface{}` - Database row with OAuth configuration
- `secret string` - Encryption secret for decrypting client secret

**Process:**

**1. Redirect URI Construction (lines 62-69):**
- Line 62: Type assertion: `authConnectorData["redirect_uri"].(string)`
- Line 63: Type assertion: `authConnectorData["name"].(string)`
- Lines 65-69: Appends authenticator parameter to redirect URI

**2. Client Secret Decryption (lines 71-76):**
- Line 71: Type assertion: `authConnectorData["client_secret"].(string)`
- Line 72: Decrypts client secret: `resource.Decrypt([]byte(secret), clientSecretEncrypted)`
- Lines 73-76: Error handling for decryption failure

**3. OAuth Config Construction (lines 78-87):**
- Lines 78-87: Creates oauth2.Config with multiple type assertions:
  - Line 79: `authConnectorData["client_id"].(string)`
  - Line 82: `authConnectorData["scope"].(string)`
  - Line 84: `authConnectorData["auth_url"].(string)`
  - Line 85: `authConnectorData["token_url"].(string)`

**Output:**
- Returns `(*oauth2.Config, error)`

**Edge Cases:**
- **Multiple type assertions:** Lines 62, 63, 71, 79, 82, 84, 85 can panic if fields missing or wrong type
- **URL manipulation:** String concatenation for redirect URI without proper URL encoding
- **Scope parsing:** Assumes comma-separated scopes without validation

### Function: DoAction() (lines 92-158)
**Inputs:**
- `request actionresponse.Outcome` - Action request details with Type field
- `inFieldMap map[string]interface{}` - Input parameters with OAuth response data
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Input Parameter Extraction (lines 94-110):**
- Line 94: Type assertion: `inFieldMap["state"].(string)`
- Line 108: Type assertion: `inFieldMap["authenticator"].(string)`
- Line 109: Type assertion: `inFieldMap["code"].(string)`
- Line 110: Reference ID conversion: `daptinid.InterfaceToDIR(inFieldMap["user_reference_id"])`

**2. TOTP State Validation (lines 97-106):**
- Lines 97-102: Validates TOTP state: `totp.ValidateCustom(state, d.otpKey, time.Now().UTC(), totp.ValidateOpts{...})`
- Lines 103-106: Returns error if state validation fails

**3. OAuth Configuration Retrieval (lines 112-116):**
- Line 112: Gets OAuth config: `GetOauthConnectionDescription(authenticator, d.cruds["oauth_connect"], transaction)`
- Lines 114-116: Error handling for configuration retrieval

**4. OAuth Token Exchange (lines 118-123):**
- Line 118: Creates background context
- Line 119: Exchanges authorization code for token: `conf.Exchange(ctx, code)`
- Lines 120-123: Error handling for token exchange failure

**5. Token Storage (lines 125-129):**
- Line 125: Stores OAuth token: `d.cruds["oauth_token"].StoreToken(token, authenticator, authReferenceId, user_reference_id, transaction)`
- Lines 126-128: Error handling for token storage failure
- Line 129: Additional error check using `resource.CheckErr()` (may panic)

**6. Response Generation (lines 131-157):**
- Lines 131-137: Creates success notification response
- Lines 139-142: Creates client store action with access token
- Lines 144-148: Creates redirect response to oauth_token page
- Lines 150-155: Creates model response with token details

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with OAuth completion

**Edge Cases:**
- **Lines 94, 108, 109:** Type assertions can panic if parameters missing or wrong type
- **Line 129:** `resource.CheckErr()` may panic instead of returning error
- **Token exposure:** Access token stored in client-side storage (line 141)
- **No token validation:** No validation of received OAuth token before storage
- **Context timeout:** No timeout set on OAuth token exchange context

### Function: NewOauthLoginResponseActionPerformer() (lines 160-187)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration (unused)
- `cruds map[string]*resource.DbResource` - Database resource map
- `configStore *resource.ConfigStore` - Configuration storage
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. TOTP Secret Management (lines 162-177):**
- Line 162: Gets existing TOTP secret: `configStore.GetConfigValueFor("totp.secret", "backend", transaction)`
- Lines 163-177: Generates new secret if not exists (same logic as login begin action)

**2. Handler Creation (lines 179-183):**
- Creates performer with cruds, TOTP key, and config store
- Does not set responseAttrs field

**3. Return (line 185):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Unused parameter:** initConfig parameter completely ignored
- **Duplicate TOTP logic:** Same TOTP secret generation logic as in login begin action
- **Unused field:** responseAttrs field declared but never used

**Side Effects:**
- **OAuth token exchange:** Exchanges authorization codes for access tokens with external OAuth providers
- **Token persistence:** Stores OAuth tokens in database for future use
- **Client-side storage:** Stores access tokens in client-side storage
- **Browser redirection:** Redirects user to OAuth token management page
- **External API calls:** Makes HTTP requests to OAuth provider token endpoints

## Critical Issues Found

### ⚠️ Runtime Safety Issues
1. **Multiple type assertion panics** (lines 62, 63, 71, 79, 82, 84, 85, 94, 108, 109): Can panic if database fields missing or wrong type
2. **CheckErr panic risk** (line 129): `resource.CheckErr()` may panic instead of returning error
3. **Array bounds assumption:** Uses `rows[0]` after length check but potential race conditions

### 🔐 Security Concerns
4. **OAuth config logging** (line 55): OAuth configuration including secrets printed to logs
5. **Token exposure in client storage** (line 141): Access token stored in client-side storage without encryption
6. **No token validation**: OAuth tokens not validated before storage
7. **Secret exposure in logs**: Debug output may expose sensitive OAuth configuration

### 🔑 OAuth Security Issues
8. **No PKCE validation**: No Proof Key for Code Exchange validation
9. **State reuse window**: TOTP state valid for 5-minute window with skew tolerance
10. **No authorization code validation**: Authorization codes not validated before exchange
11. **No token expiry handling**: No validation or handling of token expiry during exchange

### 🏗️ Design Issues
12. **Duplicate TOTP logic**: Same TOTP secret generation code duplicated from login begin action
13. **Unused struct field**: responseAttrs declared but never used
14. **Unused parameter**: initConfig parameter ignored in constructor
15. **No input sanitization**: OAuth parameters not sanitized before use

### 🌐 External Dependencies
16. **No timeout on token exchange**: OAuth token exchange has no timeout configuration
17. **No retry logic**: No handling of OAuth provider connectivity issues
18. **Context management**: Background context used without timeout or cancellation

### 📊 Data Handling Issues
19. **URL manipulation without encoding**: Redirect URI constructed with string concatenation
20. **Scope parsing assumptions**: Assumes comma-separated scopes without validation
21. **Reference ID conversion**: No validation of user reference ID format