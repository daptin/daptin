# action_generate_self_tls_certificate.go

**File:** server/actions/action_generate_self_tls_certificate.go

## Code Summary

### Type: selfTlsCertificateGenerateActionPerformer (lines 11-17)
**Fields:**
- `responseAttrs map[string]interface{}` - Response attributes (unused in implementation)
- `cruds map[string]*resource.DbResource` - Database resource access map (unused in implementation)
- `configStore *resource.ConfigStore` - Configuration storage (unused in implementation)
- `encryptionSecret []byte` - Encryption secret (unused in implementation)
- `certificateManager *resource.CertificateManager` - Certificate management service

### Function: Name() (lines 19-21)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"self.tls.generate"`

### Function: DoAction() (lines 23-38)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused in implementation)
- `inFieldMap map[string]interface{}` - Input parameters with certificate configuration
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Certificate Subject Extraction (lines 24-27):**
- Line 24: Type assertion: `inFieldMap["certificate"].(map[string]interface{})`
- Line 25: Logs certificate subject information
- Line 27: Type assertion: `certificateSubject["hostname"].(string)`

**2. Certificate Generation (lines 28-31):**
- Line 28: Generates TLS certificate: `d.certificateManager.GetTLSConfig(hostname, true, transaction)`
  - `hostname`: Target hostname for certificate
  - `true`: Likely indicates self-signed certificate generation
  - `transaction`: Database transaction for certificate storage
- Lines 29-31: Returns error if certificate generation fails

**3. Success Logging (line 33):**
- Line 33: Logs generated certificate PEM data

**4. Response Creation (lines 35-37):**
- Lines 35-37: Returns success notification with message "Certificate generated for " + hostname

**5. Return (line 37):**
- Returns nil responder, success notification response, and nil errors

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with success notification

**Edge Cases:**
- **Line 24:** Type assertion `inFieldMap["certificate"].(map[string]interface{})` can panic if certificate parameter is not map type
- **Line 27:** Type assertion `certificateSubject["hostname"].(string)` can panic if hostname field is not string type
- **Certificate generation dependency:** Relies entirely on `certificateManager.GetTLSConfig()` implementation
- **No input validation:** Hostname not validated for format, length, or security requirements
- **Certificate logging:** Line 33 logs certificate PEM data which could expose sensitive information
- **No authorization:** Any authenticated user can generate certificates for any hostname
- **Self-signed certificates:** Generates self-signed certificates which browsers will show security warnings for
- **No hostname ownership verification:** No validation that user owns or controls the requested hostname

### Function: NewSelfTlsCertificateGenerateActionPerformer() (lines 40-53)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map
- `configStore *resource.ConfigStore` - Configuration storage
- `certificateManager *resource.CertificateManager` - Certificate management service
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Encryption Secret Retrieval (line 42):**
- Gets encryption secret from configuration: `configStore.GetConfigValueFor("encryption.secret", "backend", transaction)`

**2. Handler Creation (lines 44-49):**
- Creates performer with dependencies including encryption secret

**3. Return (line 51):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Unused fields:** Multiple fields (responseAttrs, cruds, configStore, encryptionSecret) stored but never used in DoAction
- **Parameter validation:** No validation of input parameters
- **Encryption secret:** Retrieved but not used in implementation

**Side Effects:**
- **TLS certificate generation:** Creates self-signed TLS certificates for specified hostnames
- **Certificate storage:** Certificates likely stored in database via certificateManager
- **Security implications:** Self-signed certificates reduce security warnings for HTTPS but don't provide actual certificate authority validation
- **Log exposure:** Certificate PEM data logged, potentially exposing certificate contents in log files