# action_otp_generate.go

**File:** server/actions/action_otp_generate.go

## Code Summary

### Type: otpGenerateActionPerformer (lines 20-25)
**Fields:**
- `responseAttrs map[string]interface{}` - Response attributes (unused in implementation)
- `cruds map[string]*resource.DbResource` - Database resource access map
- `configStore *resource.ConfigStore` - Configuration storage
- `encryptionSecret []byte` - Secret for encrypting/decrypting OTP secrets

### Function: Name() (lines 27-29)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"otp.generate"`

### Function: DoAction() (lines 31-158)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFieldMap map[string]interface{}` - Input parameters with email/mobile
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Input Parameter Extraction (lines 33-40):**
- Line 33: Gets email from input fields
- Line 34: Gets mobile from input fields
- Lines 39-40: Returns error if neither email nor mobile provided

**2. Email-Based User Lookup (lines 41-51):**
- Line 42: Type assertion and user lookup: `email.(string)` and `GetUserAccountRowByEmail()`
- Lines 43-45: Error handling for invalid email
- Lines 46-49: Validates user account ID exists
- Line 50: Type assertion and OTP profile lookup: `i.(int64)`

**3. Mobile-Based User Lookup (lines 53-66):**
- Line 54: Gets OTP profile by mobile number
- Lines 55-57: Error handling for unregistered number
- Line 58: Reference ID conversion: `daptinid.InterfaceToDIR(userOtpProfile["otp_of_account"])`
- Lines 59-61: Validates reference ID
- Line 62: Gets user account by reference ID
- Lines 63-65: Error handling for unregistered number

**4. Mobile Parameter Normalization (lines 68-71):**
- Lines 68-71: Sets mobile to empty string if nil

**5. HTTP Request Context Setup (lines 73-84):**
- Lines 73-76: Creates mock HTTP request for API operations
- Lines 77-80: Creates session user with type assertions: `userAccount["id"].(int64)`
- Lines 81-84: Sets up API request context

**6. OTP Profile Creation (lines 86-116):**
**If OTP profile doesn't exist:**
- Lines 88-94: Generates TOTP key: `totp.Generate(totp.GenerateOpts{...})`
- Configuration: 300s period, 4 digits, 10-byte secret, hardcoded issuer
- Line 90: Type assertion: `userAccount["email"].(string)`
- Lines 96-99: Error handling for key generation
- Lines 101-106: Creates OTP profile data structure
- Lines 108-115: Creates OTP profile in database using API

**7. OTP Secret Decryption (lines 136-139):**
- Line 136: Type assertion and decryption: `userOtpProfile["otp_secret"].(string)`
- Lines 137-139: Error handling with client notification

**8. OTP Code Generation (lines 141-150):**
- Lines 141-146: Generates current OTP code: `totp.GenerateCodeCustom(key, time.Now(), totp.ValidateOpts{...})`
- Configuration: 300s period, 1 skew, 4 digits, SHA1 algorithm
- Lines 147-150: Error handling with client notification

**9. Response Creation (lines 152-157):**
- Lines 152-155: Creates API response with generated OTP code
- Line 157: Returns response with empty action responses

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with OTP code

**Edge Cases:**
- **Lines 42, 50, 78, 79, 90, 136:** Multiple type assertions can panic if fields missing or wrong type
- **Weak OTP configuration:** 4 digits (10,000 combinations) may be insufficient for security
- **Hardcoded values:** Issuer hardcoded to "site.daptin.com"
- **No rate limiting:** No protection against OTP generation abuse
- **Long validity period:** 300-second period with 1-skew tolerance allows 10-minute window
- **Mock HTTP request:** Creates artificial HTTP request for internal API calls
- **Missing validation:** No validation of email format or mobile number format
- **Security logging gap:** OTP generation not logged for audit purposes

### Function: NewOtpGenerateActionPerformer() (lines 160-172)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map
- `configStore *resource.ConfigStore` - Configuration storage
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Encryption Secret Retrieval (line 162):**
- Line 162: Gets encryption secret (error ignored): `configStore.GetConfigValueFor("encryption.secret", "backend", transaction)`

**2. Handler Creation (lines 164-168):**
- Creates performer with cruds, encryption secret, and config store
- Does not set responseAttrs field

**3. Return (line 170):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Error ignored:** Encryption secret retrieval error silently ignored
- **Unused field:** responseAttrs field declared but never used

**Side Effects:**
- **OTP profile creation:** Creates user OTP profiles in database if they don't exist
- **Secret generation:** Generates and stores TOTP secrets
- **Time-based code generation:** Generates time-sensitive OTP codes
- **Database operations:** Performs multiple database queries and potential creation
- **Client notifications:** Sends error notifications to client on failures

## Critical Issues Found

### ⚠️ Runtime Safety Issues
1. **Multiple type assertion panics** (lines 42, 50, 78, 79, 90, 136): Can panic if database fields missing or wrong type
2. **Error ignored** (line 162): Encryption secret retrieval error silently ignored

### 🔐 Security Concerns
3. **Weak OTP security**: 4-digit OTP provides only 10,000 combinations (easily brute-forced)
4. **Long validity window**: 300-second period with 1-skew tolerance creates 10-minute attack window
5. **No rate limiting**: No protection against OTP generation/brute-force attacks
6. **Insufficient secret entropy**: 10-byte TOTP secret may be insufficient
7. **No audit logging**: OTP generation not logged for security monitoring

### 🏗️ Design Issues
8. **Hardcoded values**: Issuer hardcoded to "site.daptin.com"
9. **Mock HTTP requests**: Creates artificial HTTP requests for internal API operations
10. **Complex user lookup logic**: Overly complex email/mobile lookup with duplicate code paths
11. **Unused struct field**: responseAttrs declared but never used

### 📱 OTP Implementation Issues
12. **No format validation**: Email and mobile number formats not validated
13. **Inconsistent mobile handling**: Mobile parameter handling logic is complex and potentially error-prone
14. **No expiry tracking**: No tracking of when OTP codes were generated
15. **Missing verification state**: No clear verification workflow or state management

### 🌐 API Design Issues
16. **Context manipulation**: Manually creates HTTP request contexts for internal operations
17. **Mixed authentication methods**: Supports both email and mobile but with different code paths
18. **Error message consistency**: Different error messages for similar failure scenarios
19. **Response format inconsistency**: Uses different response formats for errors vs success

### 💾 Data Handling Issues
20. **Unencrypted storage assumptions**: Assumes OTP secrets are encrypted but no validation
21. **Reference ID complexity**: Complex reference ID handling with potential conversion errors
22. **Database transaction scope**: Uses single transaction across multiple operations without proper rollback handling