# action_mail_servers_sync.go

**File:** server/actions/action_mail_servers_sync.go

## Code Summary

### Type: mailServersSyncActionPerformer (lines 18-22)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map
- `mailDaemon *guerrilla.Daemon` - Guerrilla mail daemon for SMTP server management
- `certificateManager *resource.CertificateManager` - Certificate manager for TLS

### Function: Name() (lines 24-26)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"mail.servers.sync"`

### Function: DoAction() (lines 28-144)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters (unused)
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Mail Daemon Validation (lines 30-33):**
- Lines 30-33: Early return if mail daemon or backend not initialized

**2. Mail Server Retrieval (lines 37-41):**
- Line 37: Gets all mail servers: `d.cruds["mail_server"].GetAllObjects("mail_server", transaction)`
- Lines 39-41: Error handling for server retrieval failure

**3. Certificate Directory Setup (lines 43-45):**
- Line 44: Sets source directory name: `"daptin-certs"`
- Line 45: Creates temp directory: `os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)`

**4. Server Configuration Loop (lines 47-124):**
**For each mail server:**

**4a. Configuration Parsing (lines 54-58):**
- Line 54: Parses max size: `strconv.ParseInt(fmt.Sprintf("%v", server["max_size"]), 10, 32)`
- Line 55: Parses max clients: `strconv.ParseInt(fmt.Sprintf("%v", server["max_clients"]), 10, 32)`
- Line 56: Parses always-on TLS: `fmt.Sprintf("%v", server["always_on_tls"]) == "1"`
- Line 57: Parses auth required: `fmt.Sprintf("%v", server["authentication_required"]) == "1"`

**4b. Hostname and Certificate Processing (lines 61-66):**
- Line 61: Type assertion: `server["hostname"].(string)`
- Line 62: Gets TLS config: `d.certificateManager.GetTLSConfig(hostname, true, transaction)`
- Lines 64-66: Error handling for certificate generation failure

**4c. Certificate File Path Setup (lines 69-71):**
- Line 69: Private key path: `filepath.Join(tempDirectoryPath, hostname+".private.cert.pem")`
- Line 70: Public key path: `filepath.Join(tempDirectoryPath, hostname+".public.cert.pem")`
- Line 71: Root CA path: `filepath.Join(tempDirectoryPath, hostname+".root.cert.pem")`

**4d. Certificate File Writing (lines 78-92):**
- Line 78: Writes public key file: `os.WriteFile(publicKeyFilePath, []byte(string(cert.PublicPEMDecrypted)+"\n"+string(cert.CertPEM)+"\n"+string(cert.RootCert)), 0666)`
- Line 82: Writes root CA file: `os.WriteFile(rootCaFile, []byte(string(cert.RootCert)), 0666)`
- Line 87: Writes private key file: `os.WriteFile(privateKeyFilePath, cert.PrivatePEMDecrypted, 0666)`
- Lines 79-92: Error handling for file write failures

**4e. TLS Configuration (lines 94-105):**
- Lines 94-105: Creates guerrilla ServerTLSConfig with certificate paths and TLS settings

**4f. Server Configuration (lines 107-118):**
- Lines 107-118: Creates guerrilla ServerConfig with:
  - Line 108: Enabled status: `fmt.Sprintf("%v", server["is_enabled"]) == "1"`
  - Line 109: Listen interface: `server["listen_interface"].(string)`
  - Line 115: XClient setting: `fmt.Sprintf("%v", server["xclient_on"]) == "1"`
  - Line 117: Fixed auth types: `[]string{"LOGIN"}`

**4g. Host and Config Collection (lines 120-123):**
- Line 120: Type assertion: `server["hostname"].(string)`
- Lines 122: Appends server config to collection

**5. Mail Daemon Reload (lines 126-141):**
- Line 126: Adds wildcard host: `hosts = append(hosts, "*")`
- Lines 127-136: Reloads daemon config with:
  - Backend configuration with hardcoded pipeline
  - Fixed primary mail host: `"localhost"`
- Lines 139-141: Error handling for reload failure

**6. Return (line 143):**
- Returns nil responder, empty responses, and nil errors

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with sync result

**Edge Cases:**
- **Line 61:** Type assertion `server["hostname"].(string)` can panic if hostname is not string
- **Line 109:** Type assertion `server["listen_interface"].(string)` can panic if interface is not string
- **Line 120:** Type assertion `server["hostname"].(string)` can panic if hostname is not string
- **File permissions:** Certificate files written with 0666 permissions (world-readable/writable)
- **Private key exposure:** Private keys written to temporary files with permissive permissions
- **Error handling gaps:** Certificate generation errors logged but don't stop processing
- **Temp directory:** Certificate files left in temp directory (potential cleanup issue)
- **ParseInt errors:** strconv.ParseInt errors silently ignored (lines 54-55)
- **Environment dependency:** Relies on DAPTIN_CACHE_FOLDER environment variable

### Function: NewMailServersSyncActionPerformer() (lines 146-156)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map
- `mailDaemon *guerrilla.Daemon` - Guerrilla mail daemon
- `certificateManager *resource.CertificateManager` - Certificate manager

**Process:**

**1. Handler Creation (lines 148-152):**
- Creates performer with all provided components

**2. Return (line 154):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **No validation:** Parameters not validated for nil values

**Side Effects:**
- **Certificate file creation:** Creates temporary certificate files with private keys
- **Mail daemon reconfiguration:** Reloads mail daemon with new server configurations
- **File system operations:** Creates temporary directories and files
- **TLS certificate generation:** Generates and writes TLS certificates for mail servers
- **SMTP server configuration:** Configures multiple SMTP servers based on database entries

## Critical Issues Found

### 🚨 Security Vulnerabilities
1. **Private key exposure** (line 87): Private keys written to temp files with 0666 permissions (world-readable/writable)
2. **Certificate file permissions** (lines 78, 82): Certificate files written with overly permissive 0666 permissions
3. **Temp file cleanup**: Certificate files may remain in temp directory after use

### ⚠️ Runtime Safety Issues
4. **Panic-prone type assertions** (lines 61, 109, 120): No error handling for type conversions
5. **Silent error ignoring** (lines 54-55): strconv.ParseInt errors silently ignored
6. **Missing error propagation**: Certificate generation errors logged but don't stop processing

### 🏗️ Design Issues
7. **Hardcoded values**: Backend pipeline and primary mail host hardcoded in configuration
8. **Fixed authentication**: Auth types hardcoded to `[]string{"LOGIN"}` regardless of database settings
9. **Resource management**: No cleanup mechanism for temporary certificate files
10. **Environment dependency**: Relies on DAPTIN_CACHE_FOLDER environment variable without validation

### 🔐 Configuration Security
11. **No input validation**: Mail server configuration values not validated before use
12. **Wildcard host**: Adds `"*"` to allowed hosts without restriction
13. **TLS configuration**: No validation of TLS settings or certificate validity
14. **File path injection**: Hostname used directly in file paths without sanitization

### 📂 File System Issues
15. **Temp directory security**: Uses MkdirTemp but with potentially predictable naming
16. **File permission issues**: Certificate files accessible to all users on system
17. **Path traversal risk**: Hostname used in file paths without validation
18. **Disk space**: No limits on certificate file creation or cleanup