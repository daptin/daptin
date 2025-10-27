# Security Analysis: server/smtp_server.go

**File:** `server/smtp_server.go`  
**Lines of Code:** 139  
**Primary Function:** SMTP mail server initialization and configuration system managing TLS certificates, authentication, and server setup for email communication services

## Summary

This file implements the SMTP mail server initialization functionality for the Daptin system. It sets up mail servers with TLS configuration, certificate management, authentication, and various SMTP parameters. The function reads mail server configurations from the database, generates TLS certificates, creates temporary certificate files, and initializes the Guerrilla SMTP daemon with custom processors and authenticators. This is a security-critical component as it handles email communication and certificate management.

## Security Issues Found

### 🔴 CRITICAL Issues

#### 1. **Insecure File Permissions for Private Keys** (Lines 59, 63, 68)
```go
err = os.WriteFile(publicKeyFilePath, []byte(string(cert.PublicPEMDecrypted)+"\n"+string(cert.CertPEM)), 0666)
err = os.WriteFile(rootCaFile, []byte(cert.RootCert), 0666)
err = os.WriteFile(privateKeyFilePath, cert.PrivatePEMDecrypted, 0666)
```
**Risk:** Private keys and certificates written with world-readable permissions
- File permissions 0666 allow read/write access for all users
- Private keys exposed to any user on the system
- Could enable certificate theft and impersonation attacks
- No protection against local privilege escalation
**Impact:** Critical - Complete certificate compromise and impersonation attacks
**Remediation:** Use restrictive permissions (0600) for private keys and certificates

#### 2. **Unsafe Type Assertions Without Validation** (Lines 42, 99)
```go
hostname := server["hostname"].(string)
ListenInterface: server["listen_interface"].(string)
```
**Risk:** Unsafe type assertions can panic the application
- No validation that database values are strings
- Panic could cause entire SMTP server to crash
- Missing error handling for type conversion failures
**Impact:** High - Denial of service through panic
**Remediation:** Add safe type assertions with validation

#### 3. **Predictable Temporary Directory Creation** (Line 27)
```go
tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
```
**Risk:** Predictable temporary directory for certificate storage
- Uses fixed prefix "daptin-certs" for temporary directories
- Environment variable injection possible through DAPTIN_CACHE_FOLDER
- Temporary certificates could be accessed by other processes
**Impact:** High - Certificate exposure and tampering
**Remediation:** Use secure temporary directory creation with random names

#### 4. **Missing Certificate File Cleanup** (Lines 59, 63, 68)
```go
err = os.WriteFile(publicKeyFilePath, ...)
err = os.WriteFile(rootCaFile, ...)
err = os.WriteFile(privateKeyFilePath, ...)
```
**Risk:** Certificate files left in temporary storage indefinitely
- No cleanup mechanism for temporary certificate files
- Sensitive cryptographic material persists on disk
- Could accumulate over time and consume storage
**Impact:** High - Persistent exposure of cryptographic material
**Remediation:** Implement proper cleanup for temporary certificate files

### 🟠 MEDIUM Issues

#### 5. **Hardcoded TLS Configuration** (Lines 75-95)
```go
serverTlsConfig = guerrilla.ServerTLSConfig{
    StartTLSOn:               true,
    AlwaysOn:                 alwaysOnTls,
    ClientAuthType:           "NoClientCert",
    PreferServerCipherSuites: true,
}
```
**Risk:** Inflexible TLS configuration with potentially weak settings
- "NoClientCert" disables client certificate verification
- Commented out cipher and protocol restrictions
- No configuration validation for security settings
**Impact:** Medium - Reduced TLS security and flexibility
**Remediation:** Make TLS configuration configurable and enforce secure defaults

#### 6. **Error Handling Without Security Context** (Lines 45-47, 60-62, 71-73)
```go
if err != nil {
    log.Printf("Failed to generate Certificates for SMTP server for %s", hostname)
}
```
**Risk:** Errors logged without stopping server initialization
- Certificate generation failures are logged but not fatal
- Server may continue with invalid or missing certificates
- Could lead to insecure SMTP operation
**Impact:** Medium - Insecure server operation with invalid certificates
**Remediation:** Fail server initialization on critical certificate errors

#### 7. **Environment Variable Injection** (Line 27)
```go
tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
```
**Risk:** Environment variable used for path construction without validation
- DAPTIN_CACHE_FOLDER could be manipulated to write files anywhere
- No validation of environment variable content
- Path traversal potential through environment manipulation
**Impact:** Medium - Directory traversal and arbitrary file creation
**Remediation:** Validate and sanitize environment variable inputs

#### 8. **Hardcoded Backend Configuration** (Lines 121-127)
```go
BackendConfig: backends.BackendConfig{
    "save_process":       "HeadersParser|Debugger|Hasher|Header|Compressor|DaptinSql",
    "log_received_mails": true,
    "mail_table":         "mail",
    "save_workers_size":  1,
    "primary_mail_host":  primaryHostname,
},
```
**Risk:** Fixed configuration reduces security flexibility
- Debugging enabled by default in backend configuration
- "log_received_mails" may log sensitive email content
- No configuration validation or customization options
**Impact:** Medium - Information disclosure and reduced flexibility
**Remediation:** Make backend configuration customizable with secure defaults

### 🔵 LOW Issues

#### 9. **Integer Overflow in Configuration Parsing** (Lines 35-36)
```go
maxSize, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_size"]), 10, 32)
maxClients, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_clients"]), 10, 32)
```
**Risk:** Integer overflow and parsing errors ignored
- No bounds checking for configuration values
- ParseInt errors are ignored with blank identifier
- Could result in unexpected behavior with malformed config
**Impact:** Low - Configuration parsing issues
**Remediation:** Add proper error handling and bounds checking

#### 10. **Hardcoded Authentication Configuration** (Line 107)
```go
AuthTypes: []string{"LOGIN"},
```
**Risk:** Limited authentication methods
- Only LOGIN authentication method supported
- No support for more secure authentication mechanisms
- Fixed configuration reduces security options
**Impact:** Low - Limited authentication flexibility
**Remediation:** Make authentication types configurable

#### 11. **Information Disclosure Through Logging** (Lines 46, 111)
```go
log.Printf("Failed to generate Certificates for SMTP server for %s", hostname)
log.Infof("Setup SMTP server at [%v] for hostname [%v]", server["listen_interface"], hostname)
```
**Risk:** Detailed server configuration information in logs
- Hostnames and network interfaces logged
- Could aid in reconnaissance for attackers
- Certificate generation failures exposed
**Impact:** Low - Information disclosure
**Remediation:** Use appropriate log levels and sanitize sensitive information

## Code Quality Issues

1. **Error Handling**: Inconsistent error handling patterns and ignored errors
2. **Resource Management**: Missing cleanup for temporary files and directories
3. **Configuration**: Hardcoded values reduce flexibility and security options
4. **Type Safety**: Unsafe type assertions without validation
5. **File Security**: Insecure file permissions for sensitive cryptographic material

## Recommendations

### Immediate Actions Required

1. **File Permissions**: Change certificate file permissions to 0600 (owner read/write only)
2. **Type Safety**: Add safe type assertions with proper error handling
3. **Certificate Cleanup**: Implement cleanup mechanism for temporary certificate files
4. **Error Handling**: Make certificate generation failures fatal for server startup

### Security Improvements

1. **TLS Configuration**: Make TLS settings configurable with secure defaults
2. **Environment Validation**: Validate and sanitize environment variable inputs
3. **Access Control**: Implement proper access controls for certificate files
4. **Configuration Security**: Add validation for all configuration parameters

### Code Quality Enhancements

1. **Error Management**: Implement consistent error handling throughout the function
2. **Resource Lifecycle**: Add proper resource cleanup and management
3. **Configuration**: Make hardcoded values configurable through secure mechanisms
4. **Logging Security**: Sanitize log output to prevent information disclosure

## Attack Vectors

1. **Certificate Theft**: Access world-readable private key files to impersonate mail server
2. **Environment Manipulation**: Manipulate DAPTIN_CACHE_FOLDER to write certificates to attacker-controlled locations
3. **Configuration Injection**: Provide malformed configuration values to cause parsing errors
4. **Directory Traversal**: Use predictable temporary directories to access or replace certificates
5. **Information Gathering**: Use log output to gather server configuration information

## Impact Assessment

- **Confidentiality**: HIGH - Private keys exposed with world-readable permissions
- **Integrity**: MEDIUM - Certificate tampering possible through insecure storage
- **Availability**: MEDIUM - Type assertion panics could crash SMTP server
- **Authentication**: MEDIUM - Limited authentication mechanisms reduce security
- **Authorization**: MEDIUM - Weak TLS configuration affects connection security

This file handles critical SMTP server initialization with several high-severity security vulnerabilities primarily around cryptographic material handling, file permissions, and type safety. The main concerns are the exposure of private keys through insecure file permissions and potential denial of service through unsafe type assertions.

## Technical Notes

The SMTP server setup process includes:
1. Database query for mail server configurations
2. Temporary directory creation for certificate storage
3. Certificate file generation with configurable TLS settings
4. Server configuration creation with authentication requirements
5. Integration with Daptin's authentication and database systems

The use of the go-guerrilla library provides a solid foundation, but the security vulnerabilities in certificate handling and configuration management need immediate attention.