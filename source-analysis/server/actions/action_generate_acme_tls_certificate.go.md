# action_generate_acme_tls_certificate.go

**File:** server/actions/action_generate_acme_tls_certificate.go

## Code Summary

### Type: acmeUser (lines 31-45)
**Fields:**
- `Email string` - User email for ACME registration
- `Registration *registration.Resource` - ACME registration resource
- `key crypto.PrivateKey` - Private key for ACME operations

**Methods:**
- `GetEmail() string` - Returns user email
- `GetRegistration() *registration.Resource` - Returns registration resource
- `GetPrivateKey() crypto.PrivateKey` - Returns private key

### Type: acmeTlsCertificateGenerateActionPerformer (lines 47-54)
**Fields:**
- `responseAttrs map[string]interface{}` - Response attributes (unused)
- `cruds map[string]*resource.DbResource` - Database resource access map
- `configStore *resource.ConfigStore` - Configuration storage
- `encryptionSecret []byte` - Encryption secret for key storage
- `hostSwitch *gin.Engine` - HTTP router for ACME challenges
- `challenge map[string]string` - ACME challenge token storage

### Function: Name() (lines 56-58)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"acme.tls.generate"`

### Function: Present() (lines 60-64)
**Inputs:**
- `domain string` - Domain for challenge
- `token string` - Challenge token
- `keyAuth string` - Key authorization

**Process:**
- Line 61: Logs challenge details
- Line 62: Stores challenge: `d.challenge[token] = keyAuth`

**Output:** Always returns `nil` error

### Function: CleanUp() (lines 65-69)
**Inputs:**
- `domain string` - Domain for challenge
- `token string` - Challenge token
- `keyAuth string` - Key authorization

**Process:**
- Line 66: Logs cleanup details
- Line 67: Removes challenge: `delete(d.challenge, token)`

**Output:** Always returns `nil` error

### Function: DoAction() (lines 71-275)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFieldMap map[string]interface{}` - Input parameters with email and certificate details
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Email Validation (lines 73-89):**
- Lines 73-74: Gets and validates email parameter
- Lines 78-79: Returns error if email invalid
- Lines 81-88: Gets user account by email and validates existence

**2. User Context Setup (lines 90-104):**
- Line 90: Type assertion: `userAccount["email"].(string)`
- Lines 91-101: Creates HTTP request with user context
- Lines 98-99: Type assertions: `userAccount["id"].(int64)` and reference ID conversion

**3. Certificate Subject Processing (lines 110-112):**
- Line 110: Type assertion: `inFieldMap["certificate"].(map[string]interface{})`
- Line 111: Type assertion: `certificateSubject["hostname"].(string)`

**4. Private Key Management (lines 106-160):**
- **New Key Creation (lines 114-141):**
  - Line 119: Creates new public/private key pair: `resource.CreateNewPublicPrivateKeyPEMBytes()`
  - Lines 124-127: Creates ACME user with email and key
  - Line 129: Encrypts private key: `resource.Encrypt(d.encryptionSecret, string(privateKeyPem))`
  - Lines 134-141: Stores encrypted private key and public key in config
- **Existing Key Loading (lines 143-159):**
  - Line 145: Decrypts stored private key: `resource.Decrypt(d.encryptionSecret, userPrivateKeyEncrypted)`
  - Line 150: Parses RSA private key: `ParseRsaPrivateKeyFromPemStr(privateKeyPem)`
  - Lines 155-158: Creates ACME user with existing key

**5. ACME Client Configuration (lines 164-184):**
- Line 164: Creates LEGO config: `lego.NewConfig(&myUser)`
- Line 168: Sets production ACME directory: `lego.LEDirectoryProduction`
- Line 169: Sets key type: `certcrypto.RSA2048`
- Lines 170-184: Configures HTTP client with insecure TLS: `InsecureSkipVerify: true`

**6. ACME Client Setup (lines 187-209):**
- Line 187: Creates LEGO client: `lego.NewClient(config)`
- Line 196: Sets HTTP-01 challenge provider: `client.Challenge.SetHTTP01Provider(d)`
- Line 204: Registers user: `client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})`
- Line 209: Stores registration in user object

**7. Certificate Generation (lines 211-221):**
- Lines 211-214: Creates certificate request with hostname
- Line 216: Obtains certificate: `client.Certificate.Obtain(certificateRequest)`

**8. Certificate Processing (lines 223-244):**
- Line 225: Extracts first certificate from chain
- Line 227: Gets issuer certificate
- Lines 230-244: Extracts public key from private key

**9. Database Storage (lines 246-264):**
- Lines 246-255: Creates certificate record with all components
- Lines 257-261: Updates certificate in database

**10. Return (line 274):**
- Returns empty response arrays and nil errors

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)`

**Edge Cases:**
- **Line 90:** Type assertion `userAccount["email"].(string)` can panic if email is not string
- **Line 98:** Type assertion `userAccount["id"].(int64)` can panic if id is not int64
- **Line 110:** Type assertion `inFieldMap["certificate"].(map[string]interface{})` can panic if certificate is not map
- **Line 111:** Type assertion `certificateSubject["hostname"].(string)` can panic if hostname is not string
- **Line 181:** TLS configuration has `InsecureSkipVerify: true` - **SECURITY VULNERABILITY**
- **Line 225:** String split assumes certificate format - could panic if malformed
- **No transaction rollback:** If certificate generation fails after key storage, keys remain in config
- **Network timeouts:** ACME operations can be slow, no overall timeout configured
- **Challenge cleanup:** Challenge map grows but only cleaned up on explicit CleanUp calls
- **No hostname validation:** Accepts any hostname without format or domain validation
- **Production ACME:** Uses production Let's Encrypt - generates real certificates

### Function: ParseRsaPrivateKeyFromPemStr() (lines 277-286)
**Inputs:**
- `privPEM string` - PEM-encoded private key

**Process:**
- Line 278: Decodes PEM block: `pem.Decode([]byte(privPEM))`
- Lines 279-281: Returns error if no PEM block found
- Line 283: Parses PKCS1 private key: `x509.ParsePKCS1PrivateKey(block.Bytes)`

**Output:**
- Returns `(*rsa.PrivateKey, error)`

**Edge Cases:**
- **Only supports PKCS1 format:** Will fail on PKCS8 or other formats
- **No key validation:** Doesn't verify key strength or validity

### Function: NewAcmeTlsCertificateGenerateActionPerformer() (lines 288-309)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map
- `configStore *resource.ConfigStore` - Configuration storage
- `hostSwitch *gin.Engine` - HTTP router
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Encryption Secret Retrieval (line 290):**
- Gets encryption secret from config store

**2. Handler Creation (lines 292-298):**
- Creates performer with dependencies and empty challenge map

**3. Challenge Route Setup (lines 300-305):**
- Line 305: Registers ACME challenge route: `hostSwitch.GET("/.well-known/acme-challenge/:token", challengeResponse)`
- Lines 300-304: Creates challenge response handler

**4. Return (line 307):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Route registration:** Registers global HTTP route on system router
- **Challenge exposure:** Challenge tokens accessible via HTTP without authentication
- **Memory storage:** Challenge map stored in memory - lost on restart

**Side Effects:**
- **Certificate generation:** Creates valid TLS certificates via Let's Encrypt ACME
- **Key storage:** Stores encrypted private keys in system configuration
- **Route registration:** Adds HTTP route for ACME challenge verification
- **Network operations:** Makes external HTTPS requests to Let's Encrypt servers
- **Database updates:** Stores certificate data in database