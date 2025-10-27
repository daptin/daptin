# action_generate_password_reset_flow.go

**File:** server/actions/action_generate_password_reset_flow.go

## Code Summary

### Type: generatePasswordResetActionPerformer (lines 24-30)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map
- `secret []byte` - JWT signing secret
- `tokenLifeTime int` - Token lifetime in hours (unused in implementation)
- `jwtTokenIssuer string` - JWT issuer identifier
- `passwordResetEmailFrom string` - Email sender address

### Function: Name() (lines 32-34)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"password.reset.begin"`

### Function: DoAction() (lines 36-120)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFieldMap map[string]interface{}` - Input parameters with email
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Email Parameter Extraction (line 40):**
- Line 40: Gets email parameter: `inFieldMap["email"]`
- **No type validation:** Email not type-checked before use

**2. User Lookup (lines 42-51):**
- Line 42: Queries user by email: `d.cruds[resource.USER_ACCOUNT_TABLE_NAME].GetRowsByWhereClause("user_account", nil, transaction, goqu.Ex{"email": email})`
- Lines 45-50: If user not found, returns error notification "No Such account"

**3. Password Reset Token Generation (lines 52-77):**
- **User Processing (lines 52-68):**
  - Line 56: Generates UUID for JTI: `uuid.NewV7()`
  - Line 57: Type assertion: `existingUser["email"].(string)`
  - Line 58: Gets current time
  - Lines 59-68: Creates JWT with claims:
    - `email`: User email
    - `name`: User name
    - `nbf`: Not before time
    - `exp`: Expiration (30 minutes from now)
    - `sub`: User reference ID as string
    - `iss`: JWT issuer
    - `iat`: Issued at time
    - `jti`: JWT ID (UUID)
- **Token Signing (lines 71-77):**
  - Line 71: Signs token: `token.SignedString(d.secret)`
  - Line 72: Base64 encodes token: `base64.StdEncoding.EncodeToString([]byte(tokenString))`
  - Line 73: Prints token and error: `fmt.Printf("%v %v", tokenStringBase64, err)`
  - Lines 74-77: Returns error if signing fails

**4. Email Composition (lines 79-98):**
- Line 79: Creates mail body with token: `"Reset your password by clicking on this link: " + tokenStringBase64`
- Lines 81-98: Creates mail envelope:
  - **Subject:** "Reset password for account " + email
  - **Recipient parsing (lines 84-88):** Splits email on "@" to get user and host
  - **Sender parsing (lines 90-93):** Splits passwordResetEmailFrom on "@"
  - **Headers:** Sets date header
  - **Body:** Sets mail body data

**5. Email Sending (lines 100-116):**
- Line 100: Sends email: `d.cruds["mail"].MailSender(&mailEnvelop, backends.TaskSaveMail)`
- Lines 101-107: Success notification if mail sent
- Lines 108-115: Error notification if mail sending failed

**6. Return (line 119):**
- Returns nil responder, responses array, and nil errors

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with notification responses

**Edge Cases:**
- **Line 57:** Type assertion `existingUser["email"].(string)` can panic if email field is not string
- **Email parsing vulnerabilities (lines 86-87, 91-92):** `strings.Split(email, "@")` can panic if:
  - Email contains no "@" character (array index out of bounds)
  - Email contains multiple "@" characters (unexpected format)
  - Email is empty string
- **Token exposure:** Line 73 prints reset token to stdout/logs - **SECURITY VULNERABILITY**
- **Email validation:** No validation that email parameter or user email field contains valid email format
- **Token in email body:** Base64 token sent as plain text in email body, not as proper reset link
- **No rate limiting:** No protection against password reset spam
- **User enumeration:** Different error messages reveal whether email exists in system
- **Fixed token lifetime:** Hard-coded 30-minute expiration regardless of configuration

### Function: NewGeneratePasswordResetActionPerformer() (lines 122-170)
**Inputs:**
- `configStore *resource.ConfigStore` - Configuration storage
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Transaction Management (lines 123-129):**
- Lines 123-127: Creates database transaction
- Line 129: Defers transaction commit

**2. Configuration Retrieval (lines 130-146):**
- Line 130: Gets JWT secret from config
- Lines 132-138: Gets/sets token lifetime (unused in DoAction)
- Lines 140-146: Gets/sets JWT issuer

**3. Email Configuration (lines 148-157):**
- Lines 148-149: Gets password reset email sender
- Lines 150-157: If not configured, generates default using hostname

**4. Handler Creation (lines 159-166):**
- Creates performer with configuration values

**5. Return (line 168):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Transaction scope:** Creates transaction in constructor but defers commit - could cause issues if constructor fails
- **Configuration generation:** Auto-generates missing configuration without user awareness
- **Line 155:** Bug - assigns to `jwtTokenIssuer` instead of `passwordResetEmailFrom`
- **Line 156:** Bug - stores `hostname` instead of constructed email address in config
- **Hostname fallback:** Uses `os.Hostname()` if hostname config missing, could expose internal system names
- **Error handling:** Uses `resource.CheckErr()` which logs but may not fail the operation

**Side Effects:**
- **Password reset email:** Sends email with JWT token for password reset
- **Token generation:** Creates time-limited JWT tokens for password reset verification
- **User enumeration:** Response reveals whether email address has associated account
- **Email infrastructure:** Depends on configured mail sending capabilities
- **Configuration creation:** Automatically creates missing JWT and email configuration
- **Security token exposure:** Prints sensitive reset tokens to application logs