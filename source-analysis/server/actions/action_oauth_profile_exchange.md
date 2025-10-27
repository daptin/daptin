# action_oauth_profile_exchange.go

**File:** server/actions/action_oauth_profile_exchange.go

## Code Summary

### Type: ouathProfileExchangePerformer (lines 19-22)
**Fields:**
- `responseAttrs map[string]interface{}` - Response attributes (unused in implementation)
- `cruds map[string]*resource.DbResource` - Database resource access map

**Note:** Type name has typo: "ouath" instead of "oauth"

### Function: Name() (lines 24-26)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"oauth.profile.exchange"`

### Function: GetTokensScope() (lines 28-77)
**Inputs:**
- `tokUrl string` - Token exchange URL
- `scope string` - OAuth scope parameter (unused)
- `clientId string` - OAuth client ID (unused)
- `clientSecret string` - OAuth client secret (unused)
- `token string` - Access token for authorization

**Process:**

**1. Debug Logging (line 30):**
- Line 30: Logs profile URL to stdout

**2. URL Parameters Setup (lines 31-49):**
- Lines 31-46: Commented out URL parameter construction logic
- Line 42: Trims whitespace from scope
- Lines 47-49: Creates request body and appends unused urlParams to URL

**3. HTTP Request Creation (lines 51-56):**
- Line 51: Creates GET request: `http.NewRequest("GET", tokUrl, body)`
- Line 52: Sets content type header (incorrect for GET request)
- Lines 54-56: Sets Bearer authorization header if token provided

**4. HTTP Client Setup and Execution (lines 58-65):**
- Lines 58-61: Creates HTTP client with 10-second timeout
- Line 62: Executes request: `client.Do(req)`
- Lines 63-65: Error handling for request failure

**5. Response Processing (lines 67-76):**
- Line 67: Defers response body close
- Line 68: Reads response body: `io.ReadAll(resp.Body)`
- Line 70: Debug logs response body
- Line 71: **COMPILATION ERROR** - `json.Unmarshal()` used without importing `encoding/json`
- Lines 72-74: Error handling for JSON parsing

**Output:**
- Returns `(map[string]interface{}, error)` with parsed response

**Edge Cases:**
- **Line 71:** **COMPILATION ERROR** - `json` package not imported
- **Debug logging:** Profile URL and response logged to stdout (lines 30, 70) - potential security issue
- **Unused parameters:** clientId, clientSecret, scope parameters not used
- **Incorrect content type:** Sets form content type for GET request with JSON response
- **No response validation:** HTTP status code not checked
- **Request body on GET:** Sends body with GET request (against HTTP specs)

### Type: TokenResponse (lines 79-82)
**Fields:**
- Embeds `oauth2.Token`
- `Scope string` - Additional scope field

### Function: DoAction() (lines 84-142)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFieldMap map[string]interface{}` - Input parameters with OAuth data
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Parameter Extraction (lines 86-87):**
- Line 86: Type assertion: `inFieldMap["authenticator"].(string)`
- Line 87: Type assertion: `inFieldMap["token"].(string)`

**2. OAuth Configuration Retrieval (lines 89-93):**
- Line 89: Gets OAuth config: `GetOauthConnectionDescription(authenticator, d.cruds["oauth_connect"], transaction)`
- Lines 91-93: Error handling for configuration retrieval

**3. Token Retrieval (lines 95-100):**
- Line 96: Gets token type from input
- Lines 97-100: Retrieves existing token if token_type provided:
  - Line 98: Type assertion: `token_type.(string)`
  - Line 99: `resource.CheckErr()` call (may panic instead of returning error)

**4. Token Exchange Logic (lines 101-127):**
**4a. Token Exchange Path (lines 102-120):**
- Line 102: Checks if token is invalid or missing
- Line 103: Calls GetTokensScope with type assertion: `inFieldMap["profileUrl"].(string)`
- Lines 104-107: Error handling for token exchange
- Line 108: Debug logs token response
- Lines 110-119: **Problematic token handling:**
  - Line 111: Gets token again (redundant)
  - Line 113: Creates new oauth2.Token (shadows oauthToken variable)
  - Lines 114-118: Processes expires_in with type assertion: `tokenResponse["expires_in"].(float64)`

**4b. Existing Token Path (lines 121-127):**
- Lines 122-127: Creates response from existing valid token

**5. Response Creation (lines 129-141):**
- Lines 129-131: Creates API response with token data
- Lines 133-141: Creates action response with redirect to signin page

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with profile exchange result

**Edge Cases:**
- **Lines 86, 87, 98, 103, 115:** Multiple type assertions can panic if fields missing or wrong type
- **Line 99:** `resource.CheckErr()` may panic instead of returning error
- **Line 113:** Variable shadowing - creates new oauthToken that shadows outer variable
- **Token handling logic:** Complex and potentially incorrect token management
- **No input validation:** Parameters not validated before use
- **Redirect hardcoded:** Always redirects to "/auth/signin" regardless of context

### Function: NewOuathProfileExchangePerformer() (lines 144-152)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration (unused)
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Handler Creation (lines 146-148):**
- Creates performer with cruds only
- Does not set responseAttrs field

**2. Return (line 150):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Unused parameter:** initConfig parameter completely ignored
- **Unused field:** responseAttrs field declared but never used
- **Type name typo:** Constructor name has same typo as type name

**Side Effects:**
- **External HTTP requests:** Makes OAuth profile exchange requests to external providers
- **Token management:** Retrieves and processes OAuth tokens from database
- **Debug logging:** Logs sensitive OAuth data to stdout
- **User redirection:** Redirects users to signin page after profile exchange

## Critical Issues Found

### 🚨 Compilation Errors
1. **Line 71:** `json.Unmarshal()` used without importing `encoding/json` package

### ⚠️ Runtime Safety Issues
2. **Multiple type assertion panics** (lines 86, 87, 98, 103, 115): Can panic if fields missing or wrong type
3. **CheckErr panic risk** (line 99): `resource.CheckErr()` may panic instead of returning error
4. **Variable shadowing** (line 113): Creates new oauthToken variable that shadows outer scope variable

### 🔐 Security Concerns
5. **Debug logging exposure** (lines 30, 70, 108): OAuth URLs and responses logged to stdout in production
6. **Token exposure in logs**: Access tokens and responses logged without redaction
7. **No input validation**: OAuth parameters not validated before use
8. **No response validation**: HTTP status codes not checked before processing

### 🌐 HTTP Implementation Issues
9. **Incorrect content type**: Sets form content type for GET request expecting JSON response
10. **Request body on GET**: Sends body with GET request (violates HTTP specification)
11. **No timeout validation**: Uses hardcoded 10-second timeout without configuration
12. **No retry logic**: No handling of network failures or retries

### 🏗️ Design Issues
13. **Unused parameters**: clientId, clientSecret, scope parameters in GetTokensScope not used
14. **Complex token logic**: Overly complex and potentially incorrect token handling in DoAction
15. **Hardcoded redirect**: Always redirects to "/auth/signin" regardless of context
16. **Type name typo**: "ouath" instead of "oauth" in type and function names
17. **Unused struct field**: responseAttrs declared but never used

### 🔑 OAuth Implementation Issues
18. **No scope validation**: OAuth scopes not properly validated or used
19. **Token expiry handling**: Complex and potentially incorrect expiry time calculation
20. **No token refresh**: No automatic token refresh mechanism
21. **Profile exchange logic**: Unclear and potentially flawed profile exchange workflow

### 📊 Data Handling Issues
22. **JSON parsing without validation**: Unmarshals JSON response without validating structure
23. **Float to int conversion**: Unsafe conversion of expires_in from float64 to int
24. **Missing error propagation**: Some errors logged but not properly returned