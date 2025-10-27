# Security Analysis: server/utils.go

**File:** `server/utils.go`  
**Lines of Code:** 415  
**Primary Function:** Utility functions for server initialization, configuration management, middleware setup, and system operations

## Summary

This file provides essential utility functions for the Daptin server including configuration validation, secret generation, middleware chain building, table initialization, and file cleanup operations. It handles critical system initialization tasks and provides core functionality used throughout the application.

## Security Issues Found

### 🟡 HIGH Issues

#### 1. **Weak Cryptographic Key Generation** (Lines 76-77, 85-86)
```go
u, _ := uuid.NewV7()
jwtSecret = u.String()
u, _ := uuid.NewV7()
newSecret := strings.Replace(u.String(), "-", "", -1)
```
**Risk:** UUID-based cryptographic key generation
- UUIDs not designed for cryptographic purposes
- Predictable patterns in UUID generation reduce entropy
- Critical JWT and encryption secrets generated with insufficient randomness
- Error from uuid.NewV7() ignored with blank identifier
**Impact:** High - Weak cryptographic keys compromise authentication and encryption
**Remediation:** Use cryptographically secure random number generation (crypto/rand)

#### 2. **Panic on Critical Resource Creation** (Lines 104-106)
```go
res, err := resource.NewDbResource(model, db, ms, cruds, configStore, olricDb, table)
if err != nil {
    panic(err)
}
```
**Risk:** Uncontrolled application termination on resource creation failure
- Panic causes entire server to crash on database resource errors
- No graceful degradation or recovery mechanism
- Single point of failure for resource initialization
**Impact:** High - Denial of service through panic
**Remediation:** Implement graceful error handling and recovery

#### 3. **Environment Variable Path Injection** (Lines 343-344)
```go
schemaFolderDefinedByEnv, _ := os.LookupEnv("DAPTIN_SCHEMA_FOLDER")
files, _ = filepath.Glob(schemaFolderDefinedByEnv + string(os.PathSeparator) + "*_uploaded_*")
```
**Risk:** Environment variable used for file path construction without validation
- DAPTIN_SCHEMA_FOLDER could be manipulated to target arbitrary directories
- No validation of environment variable content
- Could lead to deletion of files outside intended directory
**Impact:** High - Unauthorized file deletion and directory traversal
**Remediation:** Validate and sanitize environment variable values

#### 4. **Unsafe Type Assertion in Error Handling** (Line 61)
```go
fmtString := message[0].(string)
```
**Risk:** Type assertion without validation can panic
- No validation that first message parameter is a string
- Could panic if called with incorrect parameter types
- Used in error handling context where stability is critical
**Impact:** High - Panic in error handling could mask original errors
**Remediation:** Use safe type assertion with ok check

### 🟠 MEDIUM Issues

#### 5. **SQL Injection Through Dynamic Query Construction** (Lines 132-152)
```go
sql, args, err := statementbuilder.Squirrel.
    Select("table_name", "permission", "default_permission",
        "world_schema_json", "is_top_level", "is_hidden", "is_state_tracking_enabled", "default_order", "icon",
    ).Prepared(true).
    From("world").
    Where(goqu.Ex{
        "table_name": goqu.Op{
            "notlike": "%_has_%",
        },
    })
```
**Risk:** Complex SQL query construction with pattern matching
- While using prepared statements, the pattern matching logic could be vulnerable
- Complex WHERE conditions with LIKE operations
- Multiple exclusion patterns that could be bypassed
**Impact:** Medium - Potential for SQL injection or query manipulation
**Remediation:** Validate all query patterns and use strict allowlists

#### 6. **JSON Unmarshaling Without Validation** (Line 195)
```go
err = json.Unmarshal([]byte(world_schema_json), &t)
```
**Risk:** JSON deserialization of database content without validation
- Database content directly unmarshaled into structs
- No validation of JSON structure before unmarshaling
- Could cause errors or unexpected behavior with malformed JSON
**Impact:** Medium - JSON injection and potential DoS
**Remediation:** Validate JSON structure before unmarshaling

#### 7. **File Operations Without Path Validation** (Lines 335, 339, 344, 347)
```go
files, _ := filepath.Glob("*_uploaded_*")
err := os.Remove(fileName)
files, _ = filepath.Glob(schemaFolderDefinedByEnv + string(os.PathSeparator) + "*_uploaded_*")
err := os.Remove(fileName)
```
**Risk:** File operations on glob patterns without path validation
- No validation that files are within intended directories
- Could delete files outside intended scope
- Error handling present but glob operations could target wrong files
**Impact:** Medium - Unintended file deletion
**Remediation:** Validate file paths before deletion operations

### 🔵 LOW Issues

#### 8. **Information Disclosure Through Detailed Logging** (Lines 97, 159, 172, 189, 203, 209, 225)
```go
log.Errorf("Table name is empty, not adding to JSON API, as it will create conflict: %v", table)
log.Errorf("[106] failed to prepare statment: %v", err)
log.Printf("Failed to select from world table: %v", err)
log.Printf("Error, column without name in existing tables: %v", t)
log.Printf("Loaded %d tables from world table", len(ts))
```
**Risk:** Detailed system information exposed in logs
- Error messages reveal internal system state
- Database structure and table information logged
- Could aid in reconnaissance for attackers
**Impact:** Low - Information disclosure for system reconnaissance
**Remediation:** Use generic error messages and appropriate log levels

#### 9. **Missing Input Validation in Utility Functions** (Lines 27-55)
```go
func EndsWithCheck(str string, endsWith string) bool {
    if len(endsWith) > len(str) {
        return false
    }
    suffix := str[len(str)-len(endsWith):]
}
```
**Risk:** String manipulation without bounds checking
- While basic length checks exist, edge cases not fully validated
- Could cause panics with unexpected input combinations
- No validation for empty strings or nil inputs
**Impact:** Low - Potential for runtime errors
**Remediation:** Add comprehensive input validation

#### 10. **Global JSON Configuration** (Line 25)
```go
var json = jsoniter.ConfigCompatibleWithStandardLibrary
```
**Risk:** Global JSON configuration could affect security settings
- Global configuration affects all JSON operations
- Could inadvertently change security-relevant JSON parsing behavior
- No explicit security configuration for JSON operations
**Impact:** Low - Potential for JSON parsing security issues
**Remediation:** Use explicit configuration for security-sensitive operations

## Code Quality Issues

1. **Error Handling**: Inconsistent error handling patterns throughout
2. **Input Validation**: Missing validation in several utility functions
3. **Resource Management**: Panic on errors prevents graceful degradation
4. **Security**: Weak cryptographic key generation for critical secrets
5. **Logging**: Excessive information disclosure through detailed logging

## Recommendations

### Immediate Actions Required

1. **Cryptographic Keys**: Replace UUID-based key generation with crypto/rand
2. **Error Handling**: Replace panic calls with graceful error handling
3. **Path Validation**: Validate environment variables and file paths
4. **Type Safety**: Add safe type assertions with proper validation

### Security Improvements

1. **Key Generation**: Use cryptographically secure random number generation
2. **Input Validation**: Add comprehensive validation for all utility functions
3. **Environment Security**: Validate and sanitize all environment variable usage
4. **JSON Security**: Add validation for JSON unmarshaling operations

### Code Quality Enhancements

1. **Error Management**: Implement consistent error handling strategies
2. **Resource Safety**: Add proper resource lifecycle management
3. **Logging Security**: Reduce information disclosure in log messages
4. **Documentation**: Add security considerations for utility functions

## Attack Vectors

1. **Weak Keys**: Exploit predictable UUID-based JWT and encryption keys
2. **Environment Manipulation**: Manipulate DAPTIN_SCHEMA_FOLDER to delete arbitrary files
3. **Type Confusion**: Trigger panics through incorrect function parameter types
4. **JSON Injection**: Provide malformed JSON in database to cause parsing errors
5. **Information Gathering**: Use detailed error logs to gather system information

## Impact Assessment

- **Confidentiality**: HIGH - Weak cryptographic keys compromise all encrypted data
- **Integrity**: MEDIUM - JSON injection and type assertion issues affect data integrity
- **Availability**: HIGH - Panic conditions could cause denial of service
- **Authentication**: HIGH - Weak JWT secrets compromise authentication system
- **Authorization**: MEDIUM - Key generation issues could affect authorization controls

This utility file contains several critical security vulnerabilities primarily around cryptographic key generation and error handling. The weak key generation particularly poses a significant risk to the entire authentication and encryption system.

## Technical Notes

The utility functions provide:
1. String manipulation and validation helpers
2. System secret generation and management
3. Database table initialization and loading
4. Middleware chain configuration
5. File cleanup and maintenance operations
6. API resource registration and management

The main security concerns revolve around the cryptographic key generation using UUIDs instead of proper cryptographic random number generation, along with various input validation issues and potential for denial of service through panic conditions.

## Critical Security Functions

Key security-relevant functions:
- `CheckSystemSecrets()`: Generates JWT and encryption secrets
- `CheckErr()`: Error handling used throughout the application
- `GetTablesFromWorld()`: Loads table configurations from database
- `CleanUpConfigFiles()`: File cleanup with environment variable usage

These functions are fundamental to system security and require immediate attention to address the identified vulnerabilities.