# action_generate_jwt_token.go

**File:** server/actions/action_generate_jwt_token.go

## Code Summary

### Type: generateJwtTokenActionPerformer (lines 17-22)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map
- `secret []byte` - JWT signing secret
- `tokenLifeTime int` - Token lifetime in hours
- `jwtTokenIssuer string` - JWT issuer identifier

### Function: Name() (lines 24-26)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"jwt.token"`

### Function: DoAction() (lines 28-132)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFieldMap map[string]interface{}` - Input parameters with email and password
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Parameter Extraction (lines 32-48):**
- Line 32: Gets email parameter: `inFieldMap["email"]`
- Line 33: Initializes password as empty string
- Lines 37-40: Gets skipPasswordCheck parameter with type assertion: `skipPasswordCheckStr.(bool)`
- Lines 42-48: Password handling:
  - If not skipping password check, gets password with type assertion: `inFieldMap["password"].(string)`
  - Returns error if password missing

**2. Input Validation (lines 50-52):**
- Line 50-52: Validates email and password requirements based on skipPasswordCheck flag

**3. User Lookup (lines 54-63):**
- Line 54: Queries user by email: `d.cruds[resource.USER_ACCOUNT_TABLE_NAME].GetRowsByWhereClauseWithTransaction("user_account", nil, transaction, goqu.Ex{"email": email})`
- Lines 57-63: If user not found, returns error response with "Invalid username or password"

**4. Authentication (lines 65-127):**
- **Password Verification (line 66):**
  - Checks if skipPasswordCheck is true OR password matches: `resource.BcryptCheckStringHash(password, existingUser["password"].(string))`
- **Token Generation (lines 70-83):**
  - Line 70: Generates UUID for JTI: `uuid.NewV7()`
  - Line 71: Gets current time
  - Line 73: Adjusts time for clock skew: `timeNow.Add(-2 * time.Minute)`
  - Lines 74-83: Creates JWT with claims:
    - `email`: User email
    - `sub`: User reference ID as string
    - `name`: User name
    - `nbf`: Not before time (with clock skew)
    - `exp`: Expiration time (current + tokenLifeTime hours)
    - `iss`: JWT issuer
    - `iat`: Issued at time
    - `jti`: JWT ID (UUID)

**5. Token Signing (lines 86-91):**
- Line 86: Signs token: `token.SignedString(d.secret)`
- Lines 88-91: Returns error if signing fails

**6. Response Generation (lines 93-118):**
- **Client Store Response (lines 93-98):** Sets token in client storage
- **Cookie Response (lines 100-105):** Sets token as HTTP cookie with SameSite=Strict
- **Notification Response (lines 107-111):** Shows success notification
- **Redirect Response (lines 113-118):** Redirects to "/" after 2 second delay

**7. Authentication Failure (lines 120-127):**
- Lines 121-126: Returns error notification for invalid credentials

**8. Return (line 131):**
- Returns nil responder, responses array, and nil errors

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with authentication responses

**Edge Cases:**
- **Line 39:** Type assertion `skipPasswordCheckStr.(bool)` ignores error with `_` pattern
- **Line 44:** Type assertion `inFieldMap["password"].(string)` can panic if password is not string
- **Line 66:** Type assertion `existingUser["password"].(string)` can panic if password field is not string
- **Line 73:** Time calculation bug - `timeNow.Add(-2 * time.Minute)` result is not assigned, so clock skew adjustment is ineffective
- **Email handling:** Email parameter not type-checked before database query
- **Password bypass:** skipPasswordCheck allows authentication without password verification
- **Token lifetime:** No validation on tokenLifeTime value from configuration
- **Secret validation:** No verification that JWT secret is strong or properly configured
- **User state:** No check for user account status (active/disabled/locked)

### Function: NewGenerateJwtTokenPerformer() (lines 134-163)
**Inputs:**
- `configStore *resource.ConfigStore` - Configuration storage
- `cruds map[string]*resource.DbResource` - Database resource map
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Secret Retrieval (line 136):**
- Gets JWT secret from configuration: `configStore.GetConfigValueFor("jwt.secret", "backend", transaction)`

**2. Token Lifetime Configuration (lines 138-144):**
- Line 138: Gets token lifetime from config: `configStore.GetConfigIntValueFor("jwt.token.life.hours", "backend", transaction)`
- Lines 140-144: If not configured, sets default to 72 hours (3 days)

**3. Issuer Configuration (lines 146-152):**
- Line 146: Gets JWT issuer from config
- Lines 148-152: If not configured, generates random issuer: `"daptin-" + uid.String()[0:6]`

**4. Handler Creation (lines 154-159):**
- Creates performer with configuration values

**5. Return (line 161):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Secret handling:** No validation that JWT secret exists or has adequate entropy
- **Configuration defaults:** Automatically creates configuration if missing without user awareness
- **Issuer generation:** Random issuer generation could create conflicts in distributed deployments
- **Error handling:** Uses `resource.CheckErr()` which may log but doesn't necessarily fail the operation

**Side Effects:**
- **Authentication:** Validates user credentials and generates JWT tokens
- **Client state:** Sets authentication token in browser storage and cookies
- **Session management:** Creates stateless JWT-based authentication sessions
- **Configuration:** Automatically creates missing JWT configuration values
- **Client behavior:** Triggers client-side notifications and page redirects