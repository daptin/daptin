# action_oauth_login_begin.go

**File:** server/actions/action_oauth_login_begin.go

## Code Summary

### Type: oauthLoginBeginActionPerformer (lines 17-22)
**Fields:**
- `responseAttrs map[string]interface{}` - Response attributes (unused in implementation)
- `cruds map[string]*resource.DbResource` - Database resource access map
- `configStore *resource.ConfigStore` - Configuration storage
- `otpKey string` - TOTP secret key for state generation

### Function: Name() (lines 24-26)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"oauth.client.redirect"`

### Function: DoAction() (lines 28-78)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFieldMap map[string]interface{}` - Input parameters with authenticator name
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. TOTP State Generation (lines 30-39):**
- Lines 30-35: Generates TOTP code for OAuth state: `totp.GenerateCodeCustom(d.otpKey, time.Now(), totp.ValidateOpts{...})`
- Configuration: 300s period, 1 skew, 6 digits, SHA1 algorithm
- Lines 36-39: Error handling for TOTP generation failure

**2. Authenticator Extraction (line 41):**
- Line 41: Type assertion: `inFieldMap["authenticator"].(string)`

**3. OAuth Configuration Retrieval (lines 51-52):**
- Line 51: Gets OAuth config: `GetOauthConnectionDescription(authConnectorData, d.cruds["oauth_connect"], transaction)`
- Line 52: Error check using `resource.CheckErr()` (may panic instead of returning error)

**4. Authorization URL Generation (lines 56-62):**
- Lines 57-61: Conditional URL generation based on scopes count:
  - If multiple scopes: `conf.AuthCodeURL(state, oauth2.AccessTypeOffline)`
  - If single scope: `conf.AuthCodeURL(state)`
- Line 63: Debug output: Prints authorization URL to stdout

**5. Response Preparation (lines 65-75):**
- Lines 65-69: Creates redirect response attributes
- Lines 71-74: Creates client store action to save state
- Line 75: Creates client redirect action with URL

**6. Return (line 77):**
- Returns nil responder with store and redirect actions

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with OAuth redirect

**Edge Cases:**
- **Line 41:** Type assertion `inFieldMap["authenticator"].(string)` can panic if field missing or wrong type
- **Line 52:** `resource.CheckErr()` may panic instead of returning error gracefully
- **Debug output:** Authorization URL printed to stdout (line 63) - potential security issue
- **State management:** TOTP-based state may be predictable or reusable
- **Scope handling:** Different behavior based on scope count (lines 57-61)
- **No input validation:** Authenticator name not validated before use

### Function: NewOauthLoginBeginActionPerformer() (lines 80-108)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration (unused)
- `cruds map[string]*resource.DbResource` - Database resource map
- `configStore *resource.ConfigStore` - Configuration storage
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. TOTP Secret Retrieval (lines 82-98):**
- Line 82: Gets existing TOTP secret: `configStore.GetConfigValueFor("totp.secret", "backend", transaction)`
- **Secret Generation (lines 84-98):** If secret doesn't exist:
  - Lines 85-90: Generates new TOTP key: `totp.Generate(totp.GenerateOpts{...})`
  - Configuration: 300s period, 10-byte secret size, hardcoded issuer/account
  - Lines 92-95: Error handling for generation failure
  - Line 96: Stores new secret: `configStore.SetConfigValueFor("totp.secret", key.Secret(), "backend", transaction)`
  - Line 97: Uses new secret

**2. Handler Creation (lines 100-104):**
- Creates performer with cruds, TOTP key, and config store
- Does not set responseAttrs field

**3. Return (line 106):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Unused parameter:** initConfig parameter completely ignored
- **Hardcoded values:** TOTP issuer and account name hardcoded
- **Secret storage:** TOTP secret stored in database without additional encryption
- **Secret generation:** 10-byte secret size may be insufficient for security
- **Unused field:** responseAttrs field declared but never initialized or used

**Side Effects:**
- **OAuth authorization flow:** Initiates OAuth authorization flow with external providers
- **Client-side state storage:** Stores OAuth state in client-side storage
- **Browser redirection:** Triggers browser redirect to OAuth provider
- **TOTP secret management:** Creates and stores TOTP secrets for state generation
- **Configuration persistence:** Stores TOTP secrets in database configuration

## Critical Issues Found

### ⚠️ Runtime Safety Issues
1. **Type assertion panic** (line 41): `inFieldMap["authenticator"].(string)` can panic if field missing or wrong type
2. **CheckErr panic risk** (line 52): `resource.CheckErr()` may panic instead of returning error
3. **Missing input validation**: Authenticator name not validated before use

### 🔐 Security Concerns
4. **Debug output exposure** (line 63): OAuth authorization URL printed to stdout in production
5. **Predictable state generation**: TOTP-based state may be predictable with known secret
6. **State reuse potential**: TOTP state valid for 5-minute window with 1-skew tolerance
7. **Secret storage**: TOTP secret stored in database without additional encryption
8. **Hardcoded credentials**: TOTP issuer and account hardcoded to daptin.com

### 🔑 OAuth Security Issues
9. **No PKCE implementation**: No Proof Key for Code Exchange for additional security
10. **State validation gaps**: No explicit state validation mechanism shown
11. **Insufficient secret entropy**: 10-byte TOTP secret may be insufficient
12. **No redirect URI validation**: No validation of OAuth redirect URIs

### 🏗️ Design Issues
13. **Unused struct field**: responseAttrs declared but never used
14. **Unused parameter**: initConfig parameter ignored in constructor
15. **Scope-based logic**: Different OAuth behavior based on scope count
16. **No error propagation**: Some errors handled with panic instead of proper error return

### 🌐 OAuth Flow Issues
17. **No CSRF protection**: Beyond TOTP state, no additional CSRF protection
18. **Client-side state storage**: OAuth state stored in client without server-side tracking
19. **No timeout handling**: No explicit timeout for OAuth authorization flow
20. **Missing offline access validation**: Offline access requested but not validated