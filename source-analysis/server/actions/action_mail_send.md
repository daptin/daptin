# action_mail_send.go

**File:** server/actions/action_mail_send.go

## Code Summary

### Type: mailSendActionPerformer (lines 21-25)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map
- `mailDaemon *guerrilla.Daemon` - Mail daemon (unused in implementation)
- `certificateManager *resource.CertificateManager` - Certificate manager for TLS/DKIM

### Function: Name() (lines 27-29)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"mail.send"`

### Function: DoAction() (lines 31-167)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with email data
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Email Parameter Extraction (lines 36-40):**
- Line 36: Type assertion: `inFields["to"].([]string)`
- Line 37: Type assertion: `inFields["subject"].(string)`
- Line 38: Type assertion: `inFields["from"].(string)`
- Line 39: Type assertion: `inFields["body"].(string)`
- Line 40: Checks for optional mail server: `inFields["mail_server_hostname"]`

**2. Direct Sending Path (lines 42-66):**
**When no mail server specified:**
- Lines 44-47: Creates MIME header structure
- Line 49: Adds mail body to buffer
- Lines 51-55: Validates FROM address: `mail.NewAddress(mailFrom)`
- Lines 56-58: Creates MTA sender with hostname from FROM address
- Line 60: Sends mail: `(&i2).Send(mailFrom, mailTo, bytes.NewReader(bodyBytes))`
- Lines 61-65: Error handling for send failure

**3. Mail Server Path (lines 67-164):**
**When mail server is specified:**

**3a. Mail Server Lookup (lines 69-73):**
- Line 69: Gets mail server: `d.cruds["mail_server"].GetObjectByWhereClause("mail_server", "hostname", mailServer, transaction)`
- Lines 70-73: Error handling for server lookup failure

**3b. Address Processing (lines 75-91):**
- Lines 76-80: Validates FROM address: `mail.NewAddress(mailFrom)`
- Lines 81-87: Converts TO addresses to mail.Address objects
- Lines 88-91: Error handling for address validation

**3c. Envelope Creation (lines 93-98):**
- Lines 93-98: Creates mail envelope with addresses and delivery header

**3d. Certificate/Key Retrieval (lines 102-118):**
- Line 102: Gets TLS config: `d.certificateManager.GetTLSConfig(emailEnvelope.MailFrom.Host, false, transaction)`
- Lines 103-107: Error handling for certificate retrieval
- Line 112: Decodes PEM private key: `pem.Decode([]byte(cert.PrivatePEMDecrypted))`
- Line 114: Parses PKCS1 private key: `x509.ParsePKCS1PrivateKey(block.Bytes)`
- Lines 116-118: Error handling for key parsing

**3e. DKIM Signing Setup (lines 120-126):**
- Lines 120-126: Creates DKIM signing options with relaxed canonicalization

**3f. Email Header Construction (lines 128-142):**
- Lines 128-129: Creates basic email headers (From, Subject, To, Date)
- Lines 131-140: Adds additional envelope headers (excluding duplicates)
- Line 142: Appends mail body

**3g. DKIM Signing (lines 144-148):**
- Line 145: Signs email with DKIM: `dkim.Sign(&b, bytes.NewReader([]byte(newMailString)), options)`
- Lines 146-148: Error handling for signing failure

**3h. Final Sending (lines 154-162):**
- Lines 154-157: Creates MTA sender and sends signed email
- Lines 159-162: Error handling for send failure

**4. Return (line 166):**
- Returns nil responder, empty responses, and nil errors

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with send result

**Edge Cases:**
- **Lines 36-39:** Type assertions can panic if parameters are not expected types
- **Line 97:** Type assertion `mailServerObj["hostname"].(string)` can panic if hostname is not string
- **Line 84:** `resource.CheckErr()` may panic instead of returning error
- **DKIM requirement:** Refuses to send mail without DKIM signing when mail server is specified
- **Certificate dependency:** Requires valid TLS certificate for DKIM signing
- **Address validation:** Email address parsing failures stop entire operation
- **Debug output:** Mail content printed to stdout (lines 100, 151) - potential security issue

### Function: NewMailSendActionPerformer() (lines 169-179)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map
- `mailDaemon *guerrilla.Daemon` - Mail daemon
- `certificateManager *resource.CertificateManager` - Certificate manager

**Process:**

**1. Handler Creation (lines 171-175):**
- Creates performer with all provided components

**2. Return (line 177):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **No validation:** Parameters not validated for nil values
- **Unused component:** mailDaemon stored but never used in implementation

**Side Effects:**
- **Email delivery:** Sends actual emails to real recipients
- **DKIM signing:** Signs emails with domain private keys when mail server specified
- **External SMTP:** Makes SMTP connections to external mail servers
- **Certificate usage:** Accesses and uses TLS private keys for DKIM signing
- **Debug output:** Prints email content to stdout (security concern)
- **MTA operations:** Uses external MTA sender for delivery

## Critical Issues Found

### ⚠️ Runtime Safety Issues
1. **Panic-prone type assertions** (lines 36-39, 97): No error handling for type conversions
2. **CheckErr panic risk** (line 84): `resource.CheckErr()` may panic instead of returning error
3. **PEM decode failure** (line 112): No error handling for PEM decode operation

### 🔐 Security Concerns
4. **Debug output exposure** (lines 100, 151): Email content printed to stdout in production
5. **Private key exposure**: DKIM private keys loaded and used without additional protection
6. **No input sanitization**: Email headers and body not sanitized for injection attacks
7. **Certificate access**: TLS certificates accessed without additional authentication

### 📧 Email Security Issues
8. **No sender validation**: FROM address not validated against authorized senders
9. **No recipient limits**: No limits on number of recipients per email
10. **Header injection**: Email headers constructed without injection protection
11. **DKIM mandatory**: Refuses to send without DKIM when mail server specified (good security practice)

### 🏗️ Design Issues
12. **Unused component**: mailDaemon stored but never used in implementation
13. **Mixed sending logic**: Two different sending paths with different security requirements
14. **No rate limiting**: No protection against email spam or abuse
15. **Resource management**: No cleanup or connection pooling for SMTP connections

### 🌐 External Dependencies
16. **MTA dependency**: Relies on external MTA sender without timeout or retry logic
17. **Certificate dependency**: Requires valid certificates for DKIM signing
18. **SMTP connectivity**: No handling of network connectivity issues