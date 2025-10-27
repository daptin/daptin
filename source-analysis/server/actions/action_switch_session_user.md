# action_switch_session_user.go

**File:** server/actions/action_switch_session_user.go

## Code Summary

### Type: switchSessionUserActionPerformer (lines 17-22)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map
- `secret []byte` - JWT signing secret
- `tokenLifeTime int` - Token lifetime in hours
- `jwtTokenIssuer string` - JWT token issuer

### Function: Name() (lines 24-26)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"jwt.token"`

### Function: DoAction() (lines 28-130)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFieldMap map[string]interface{}` - Input parameters with email, password, and optional skipPasswordCheck
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Parameter Extraction (lines 32-48):**
- Line 32: Gets email from input fields
- Lines 33-40: Gets optional skipPasswordCheck flag
- Lines 42-48: Gets password if not skipping password check

**2. Input Validation (lines 50-52):**
- Line 50: Validates email and password presence
- Lines 51-52: Returns error if validation fails

**3. User Lookup (lines 54-63):**
- Line 54: Gets user by email: `d.cruds[resource.USER_ACCOUNT_TABLE_NAME].GetRowsByWhereClauseWithTransaction("user_account", nil, transaction, goqu.Ex{"email": email})`
- Lines 57-62: Returns error notification if user not found

**4. Authentication and Token Generation (lines 64-90):**
- Line 65: Validates password or skips if flag set: `resource.BcryptCheckStringHash(password, existingUser["password"].(string))`
- Line 69: Generates UUID for token: `uuid.NewV7()`
- Line 70: Gets current time
- Line 72: INCORRECT clock skew adjustment (creates time in past, should add to current time)
- Lines 73-82: Creates JWT token with claims:
  - email, sub (user reference ID), name
  - nbf (not before), exp (expiration), iss (issuer), iat (issued at), jti (JWT ID)
- Lines 85-90: Signs token and handles errors

**5. Response Creation (lines 92-118):**
- Lines 92-97: Sets token in client store
- Lines 99-104: Sets token as HTTP cookie with SameSite=Strict
- Lines 106-110: Shows success notification
- Lines 112-117: Redirects to home page after 2 second delay

**6. Authentication Failure (lines 119-125):**
- Lines 120-124: Shows error notification for invalid credentials

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with authentication result

**Edge Cases:**
- **Line 32:** Email field not validated for format or SQL injection
- **Line 44:** Type assertion `inFieldMap["password"].(string)` can panic if not string
- **Line 65:** Password comparison vulnerable to timing attacks
- **Line 72:** Clock skew adjustment creates time in past instead of allowing for skew
- **JWT claims:** No validation of user status (enabled/disabled, confirmed, etc.)
- **No rate limiting:** Vulnerable to brute force attacks
- **Password policy:** No validation of password complexity or history

### Function: NewSwitchSessionUserActionPerformer() (lines 132-140)
**Inputs:**
- `configStore *resource.ConfigStore` - Configuration store (unused)
- `cruds map[string]*resource.DbResource` - Database resource map
- `transaction *sqlx.Tx` - Database transaction (unused)

**Process:**

**1. Handler Creation (lines 134-136):**
- Creates performer with cruds only
- Missing initialization of secret, tokenLifeTime, and jwtTokenIssuer

**2. Return (line 138):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Critical:** secret, tokenLifeTime, and jwtTokenIssuer fields not initialized
- **No validation:** cruds parameter not validated for nil

**Side Effects:**
- **JWT token generation:** Creates signed JWT tokens for authentication
- **Client state modification:** Sets tokens in client store and cookies
- **User session creation:** Establishes authenticated user sessions
- **Client notifications:** Shows success/error messages to user
- **Client redirects:** Redirects user after successful authentication

## Critical Issues Found

### 🚨 Critical Security Vulnerabilities
1. **Uninitialized security fields:** secret, tokenLifeTime, and jwtTokenIssuer not initialized in constructor
2. **Broken JWT security:** Tokens signed with nil/zero secret making them forgeable
3. **No input validation:** Email field not validated for format or injection attacks
4. **Type assertion panic:** Password type assertion can crash the application
5. **Timing attack vulnerability:** Password comparison vulnerable to timing attacks

### ⚠️ Authentication Security Issues
6. **No rate limiting:** Vulnerable to brute force password attacks
7. **No account lockout:** No protection against repeated failed login attempts
8. **No user status validation:** No checks for disabled, suspended, or unconfirmed accounts
9. **Weak password policy:** No validation of password complexity requirements
10. **Clock skew bug:** Clock skew adjustment creates time in past, not allowing for skew

### 🔐 JWT Security Issues
11. **Token forgery risk:** Uninitialized secret allows token forgery
12. **No token revocation:** No mechanism to revoke or blacklist tokens
13. **No refresh tokens:** No secure token refresh mechanism
14. **Information disclosure:** JWT contains user information in readable format
15. **No scope/role claims:** JWT lacks authorization scope or role information

### 🌐 Session Security Issues
16. **Cookie security:** Cookie lacks HttpOnly, Secure flags for production
17. **No CSRF protection:** No CSRF token or additional protection
18. **Long-lived tokens:** No short-lived access tokens with refresh mechanism
19. **No session tracking:** No tracking of active sessions for security monitoring

### 🏗️ Design Issues
20. **Unused parameters:** configStore and transaction parameters not used in constructor
21. **Misleading name:** Action named "jwt.token" but performs user authentication
22. **Mixed concerns:** Combines authentication, token generation, and session management
23. **No error differentiation:** Same error message for missing user and wrong password

### 📂 Input Handling Issues
24. **No sanitization:** Email input not sanitized before database query
25. **Missing validation:** No validation of email format or length limits
26. **Skip password bypass:** skipPasswordCheck flag allows authentication bypass
27. **No audit logging:** Authentication attempts not logged for security monitoring

### ⚙️ Operational Issues
28. **No configuration validation:** JWT configuration not validated on startup
29. **No metrics:** No metrics for authentication success/failure rates
30. **No alerting:** No alerting for unusual authentication patterns
31. **Hard-coded redirects:** Hard-coded redirect to "/" may not be appropriate for all contexts

### 🔒 Access Control Issues
32. **No permission validation:** No validation of user permissions or roles
33. **No context validation:** No validation of authentication context or source
34. **Privilege escalation risk:** No validation of user privilege level changes
35. **No audit trail:** Authentication events not logged for compliance