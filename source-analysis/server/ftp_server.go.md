# Security Analysis: server/ftp_server.go

**File:** `server/ftp_server.go`  
**Lines of Code:** 471  
**Primary Function:** FTP server implementation with authentication, file operations, and site-based virtual directories

## Summary

This file implements a complete FTP server with authentication against user accounts, virtual site directories, TLS support, and comprehensive file operations including read, write, delete, rename, and directory management. The implementation includes external IP resolution and certificate management.

## Security Issues Found

### 🔴 CRITICAL Issues

#### 1. **Path Traversal Vulnerability** (Lines 216, 246, 258, 291, 324, 343, 358, 366, 374-377)
```go
path = driver.FtpDriver.Sites[driver.CurrentDir].LocalSyncPath + string(os.PathSeparator) + strings.Join(dirParts[2:], string(os.PathSeparator))
```
**Risk:** Directory traversal attacks through malicious FTP commands
- Path construction uses user-controlled input without validation
- No bounds checking for path escaping
- Can access files outside intended site directories
**Impact:** High - Unauthorized file system access
**Remediation:** Implement path validation and canonicalization

#### 2. **Unsafe Type Assertion** (Line 187)
```go
if !resource.BcryptCheckStringHash(pass, userAccount["password"].(string)) {
```
**Risk:** Application crash during authentication
- Direct type assertion on database field
- Can panic if password field is not string type
- Authentication bypass through DoS
**Impact:** High - Authentication bypass via panic
**Remediation:** Use safe type assertion with ok check

#### 3. **Information Disclosure in Authentication** (Lines 183-189)
```go
userAccount, err := driver.cruds["user_account"].GetUserAccountRowByEmail(user, transaction)
if err != nil {
    return nil, err
}
if !resource.BcryptCheckStringHash(pass, userAccount["password"].(string)) {
    return nil, fmt.Errorf("could not authenticate you")
}
```
**Risk:** User enumeration through timing attacks
- Different error paths for non-existent vs wrong password
- Database errors exposed in authentication responses
- Enables username enumeration
**Impact:** High - User enumeration and information disclosure
**Remediation:** Consistent timing and error responses

#### 4. **Insecure File Permissions** (Lines 269, 337)
```go
return os.Mkdir(path, 0750)
return os.OpenFile(path, flag, 0600)
```
**Risk:** Inappropriate file system permissions
- Fixed permissions may not match security requirements
- No validation of file creation permissions
- Created files/directories may be too permissive
**Impact:** Medium - File system security compromise
**Remediation:** Use configurable, secure default permissions

### 🟡 HIGH Issues

#### 5. **External HTTP Request Without Validation** (Lines 453-469)
```go
rsp, err := http.Get("http://checkip.amazonaws.com")
```
**Risk:** Network dependency and potential SSRF
- Unencrypted HTTP request to external service
- No timeout or retry limits
- Dependency on external service availability
**Impact:** Medium - Network security and availability
**Remediation:** Use HTTPS, implement timeouts, add fallback options

#### 6. **Transaction Resource Management** (Lines 134-148, 175-182)
```go
transaction, err := driver.cruds["world"].Connection().Beginx()
// ... processing
transaction.Commit()
```
**Risk:** Resource leaks and improper transaction handling
- Manual transaction management without proper error handling
- defer rollback pattern inconsistent
- Potential database connection leaks
**Impact:** Medium - Resource exhaustion
**Remediation:** Consistent defer rollback and error handling

#### 7. **Concurrent Access to Atomic Counter** (Lines 153-158, 200)
```go
nbClients := atomic.AddInt32(&driver.nbClients, 1)
if nbClients > driver.DaptinFtpServerSettings.MaxConnections {
```
**Risk:** Race condition in connection limiting
- Check-then-act race condition
- Client count can exceed limit between check and increment
- No synchronization for connection rejection
**Impact:** Medium - Connection limit bypass
**Remediation:** Use atomic compare-and-swap operations

### 🟠 MEDIUM Issues

#### 8. **Missing Input Validation** (Lines 205-217, 231-249)
```go
dirParts := strings.Split(path, "/")
subsiteName := dirParts[1]
_, ok := driver.FtpDriver.Sites[subsiteName]
```
**Risk:** Array bounds violations and injection
- No validation of path format before splitting
- Array access without bounds checking
- Site name used directly from user input
**Impact:** Medium - Application crash, path confusion
**Remediation:** Validate input format and array bounds

#### 9. **Debug Information Exposure** (Lines 161, 190, 224, 247, 276, 295, 302, 316, 346)
```go
cc.SetDebug(true)
log.Infof("FTP Login [%s][%s][%s]", driver.BaseDir, user, cc.RemoteAddr())
```
**Risk:** Information leakage through debug logs
- Debug mode enabled by default
- Sensitive information logged (usernames, IPs, paths)
- Internal state exposed in logs
**Impact:** Medium - Information disclosure
**Remediation:** Disable debug mode, sanitize log messages

#### 10. **Error Logic Inversion** (Lines 294-297)
```go
filesDirEntries, err := os.ReadDir(path)
if err == nil {
    log.Errorf("Failed to read path ["+path+"] => ", err)
    return nil, nil
}
```
**Risk:** Logic error causing silent failures
- Error condition inverted (err == nil should be err != nil)
- Returns nil on successful directory read
- Logs error when operation succeeds
**Impact:** Medium - Functional failure, misleading logs
**Remediation:** Fix error condition logic

### 🔵 LOW Issues

#### 11. **Hard-Coded Configuration Values** (Lines 67, 76-81)
```go
MaxConnections: 100,
IdleTimeout:   5,
ConnectionTimeout: 5,
```
**Risk:** Inflexible security configuration
- Fixed timeout and connection limits
- May not suit all deployment environments
- No runtime configuration options
**Impact:** Low - Operational flexibility
**Remediation:** Make configuration externally configurable

#### 12. **Missing File Operation Validation** (Lines 331-334)
```go
if err := os.Remove(path); err != nil {
    fmt.Println("Problem removing file", path, "err:", err)
}
```
**Risk:** File operation errors not properly handled
- File removal errors only logged, not returned
- May lead to unexpected behavior
- Error information exposed in output
**Impact:** Low - Operational reliability
**Remediation:** Proper error handling and logging

## Code Quality Issues

1. **Error Handling**: Inconsistent error handling patterns throughout
2. **Input Validation**: Missing validation for user-controlled paths and parameters
3. **Resource Management**: Inconsistent transaction and connection management
4. **Logging**: Sensitive information exposed in debug logs
5. **Type Safety**: Unsafe type assertions without proper checking

## Recommendations

### Immediate Actions Required

1. **Path Security**: Implement proper path validation and canonicalization
2. **Type Safety**: Fix unsafe type assertions in authentication
3. **Authentication**: Implement constant-time authentication responses
4. **Error Handling**: Fix inverted error logic in directory reading

### Security Improvements

1. **Input Validation**: Validate all FTP command parameters and paths
2. **Access Control**: Implement proper file system access controls
3. **Network Security**: Use HTTPS for external requests, add timeouts
4. **Logging**: Sanitize log messages to prevent information disclosure

### Code Quality Enhancements

1. **Configuration**: Make security settings externally configurable
2. **Resource Management**: Implement consistent transaction patterns
3. **Concurrency**: Fix race conditions in connection management
4. **Testing**: Add unit tests for security-critical operations

## Attack Vectors

1. **Directory Traversal**: Access files outside intended directories through malicious paths
2. **Authentication Bypass**: Crash authentication through malformed data
3. **User Enumeration**: Determine valid usernames through timing attacks
4. **DoS via Path Injection**: Crash server through malformed FTP commands
5. **Resource Exhaustion**: Bypass connection limits through race conditions

## Impact Assessment

- **Confidentiality**: HIGH - Path traversal enables unauthorized file access
- **Integrity**: HIGH - File operations without proper validation
- **Availability**: HIGH - Multiple crash points and resource exhaustion
- **Authentication**: HIGH - Multiple authentication vulnerabilities
- **Authorization**: HIGH - Path traversal bypasses file access controls

This file contains critical security vulnerabilities that require immediate attention, particularly around path validation, authentication security, and input validation for FTP operations.