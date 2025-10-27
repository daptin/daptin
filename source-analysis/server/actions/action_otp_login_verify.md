# action_otp_login_verify.go

**File:** server/actions/action_otp_login_verify.go

## Code Summary

### Type: otpLoginVerifyActionPerformer (lines 26-36)
**Fields:**
- `responseAttrs map[string]interface{}` - Response attributes (unused in implementation)
- `cruds map[string]*resource.DbResource` - Database resource access map
- `configStore *resource.ConfigStore` - Configuration storage
- `encryptionSecret []byte` - Secret for decrypting OTP secrets
- `tokenLifeTime int` - JWT token lifetime in hours
- `jwtTokenIssuer string` - JWT token issuer identifier
- `otpKey string` - OTP key (unused in implementation)
- `secret []byte` - JWT signing secret
- `totpSecret string` - TOTP secret (unused in implementation)

### Function: Name() (lines 38-40)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"otp.login.verify"`

### Function: DoAction() (lines 42-168)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFieldMap map[string]interface{}` - Input parameters with OTP and user identifiers
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. OTP State Extraction (lines 46-52):**
- Line 46: Type assertion: `inFieldMap["otp"].(string)`
- Lines 47-52: Fallback conversion to string if type assertion fails

**2. User Identification (lines 53-78):**
**2a. Email-Based Lookup (lines 56-78):**
- Line 68: Type assertion and user lookup: `email.(string)`
- Lines 69-71: Error handling for invalid email
- Lines 72-75: Validates user account ID exists
- Lines 76-78: Gets OTP profile by user account ID

**2b. Mobile-Based Lookup (lines 57-66):**
- Line 61: Type assertion and OTP profile lookup: `phone.(string)`
- Lines 62-64: Error handling for unregistered mobile
- Lines 65-66: Gets user account by reference ID

**3. OTP Profile Validation (lines 80-82):**
- Lines 80-82: Returns error if OTP profile not found

**4. OTP Secret Decryption (line 84):**
- Line 84: Type assertion and decryption: `userOtpProfile["otp_secret"].(string)` (error ignored)

**5. OTP Validation (lines 86-97):**
- Lines 86-87: Gets current time and adds 2-minute skew (note: line 87 has no effect)
- Lines 88-93: Validates OTP: `totp.ValidateCustom(state, key, timeInstance, totp.ValidateOpts{...})`
- Configuration: 300s period, 1 skew, 4 digits, SHA1 algorithm
- Lines 94-97: Error handling with user email logging (type assertion: `userAccount["email"].(string)`)

**6. Verification Status Update (lines 99-138):**
**Complex verification status handling:**
- Lines 99-112: Type-flexible verification status checking (int64, bool, string)
- Lines 113-137: Updates OTP profile to verified if not already verified
- Lines 114-117: Creates API model for update
- Lines 118-131: Creates mock HTTP request with user context
- Lines 124-125: Type assertions: `userAccount["id"].(int64)`
- Line 132: Updates OTP profile using API

**7. JWT Token Generation (lines 140-159):**
- Line 140: Generates UUID v7 for JTI claim
- Lines 141-151: Creates JWT token with claims:
  - Line 148: Type assertion: `userAccount["email"].(string)`
  - Line 148: Gravatar URL generation with MD5 hash
- Lines 154-159: Signs token and handles errors
- Line 155: Debug output: Prints token and error to stdout

**8. Response Creation (lines 161-167):**
- Lines 161-165: Creates client store action with JWT token
- Line 167: Returns responses

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with JWT token

**Edge Cases:**
- **Lines 61, 68, 84, 95, 124, 125, 148:** Multiple type assertions can panic if fields missing or wrong type
- **Line 84:** Decryption error silently ignored
- **Line 87:** Time modification has no effect (time.Add returns new value, doesn't modify receiver)
- **Line 155:** JWT token printed to stdout - **SECURITY ISSUE**
- **Complex verification logic:** Type-flexible verification checking may be unreliable
- **Mock HTTP request:** Creates artificial HTTP request for internal API operations
- **No rate limiting:** No protection against OTP brute-force attacks

### Function: NewOtpLoginVerifyActionPerformer() (lines 170-204)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map
- `configStore *resource.ConfigStore` - Configuration storage
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Configuration Retrieval (lines 172-183):**
- Line 172: Redundant call to get JWT secret (result ignored)
- Line 174: Gets JWT secret: `configStore.GetConfigValueFor("jwt.secret", "backend", transaction)`
- Line 175: Gets encryption secret (error ignored)
- Lines 177-183: Gets or sets JWT token lifetime with default 72 hours
- Lines 178, 181: Uses `resource.CheckErr()` (may panic instead of returning error)

**2. JWT Issuer Configuration (lines 185-191):**
- Lines 185-191: Gets or generates JWT token issuer with UUID-based default
- Line 186: Uses `resource.CheckErr()` (may panic instead of returning error)

**3. Handler Creation (lines 193-200):**
- Creates performer with all configuration values
- Does not set responseAttrs, otpKey, and totpSecret fields

**4. Return (line 202):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Line 172:** Redundant configuration call with ignored result
- **Line 175:** Encryption secret error silently ignored
- **Lines 178, 181, 186:** `resource.CheckErr()` calls may panic instead of returning error
- **Unused fields:** responseAttrs, otpKey, and totpSecret declared but never used

**Side Effects:**
- **JWT token generation:** Creates authentication tokens for successful OTP verification
- **User verification:** Marks user OTP accounts as verified on first successful login
- **Client token storage:** Stores JWT tokens in client-side storage
- **Configuration management:** Creates default JWT configuration if missing
- **Debug output:** Prints JWT tokens to stdout (major security issue)

## Critical Issues Found

### 🚨 Critical Security Vulnerabilities
1. **JWT token exposure** (line 155): JWT tokens printed to stdout in production - **MAJOR SECURITY BREACH**
2. **Silent error ignoring** (lines 84, 175): Critical decryption errors silently ignored

### ⚠️ Runtime Safety Issues
3. **Multiple type assertion panics** (lines 61, 68, 84, 95, 124, 125, 148): Can panic if database fields missing or wrong type
4. **CheckErr panic risk** (lines 178, 181, 186): `resource.CheckErr()` calls may panic instead of returning error
5. **Time operation bug** (line 87): `time.Add()` result not assigned, skew calculation ineffective

### 🔐 Authentication Security Issues
6. **Weak OTP security**: 4-digit OTP provides only 10,000 combinations (easily brute-forced)
7. **No rate limiting**: No protection against OTP brute-force attacks
8. **Long validity window**: 300-second period with skew tolerance
9. **No failed attempt tracking**: No tracking or limiting of failed OTP attempts

### 🎫 JWT Implementation Issues
10. **Token exposure in logs**: JWT tokens logged to stdout accessible to all system users
11. **No token revocation**: No mechanism to revoke issued JWT tokens
12. **Long token lifetime**: Default 72-hour token lifetime may be excessive
13. **No token refresh**: No automatic token refresh mechanism

### 🏗️ Design Issues
14. **Complex verification logic**: Type-flexible verification checking unreliable and error-prone
15. **Mock HTTP requests**: Creates artificial HTTP requests for internal API operations
16. **Unused struct fields**: responseAttrs, otpKey, and totpSecret declared but never used
17. **Redundant configuration calls**: Line 172 makes unnecessary configuration call

### 📊 Data Handling Issues
18. **Mixed user lookup methods**: Different code paths for email vs mobile lookup
19. **Inconsistent error messages**: Different error messages for similar failure scenarios
20. **Context manipulation**: Manually creates HTTP request contexts for internal operations
21. **UUID v7 usage**: Uses UUID v7 for JTI claims without validation of implementation support

### 🔒 Cryptographic Issues
22. **No JWT algorithm validation**: Uses HS256 without validating algorithm security
23. **Gravatar MD5 usage**: Uses MD5 for Gravatar URLs (not security-critical but deprecated)
24. **No key rotation**: No mechanism for rotating JWT signing keys