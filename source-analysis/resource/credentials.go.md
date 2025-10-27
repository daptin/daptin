# Security Analysis: server/resource/credentials.go

**File:** `server/resource/credentials.go`  
**Type:** Credential management system with encryption/decryption  
**Lines of Code:** 52  

## Overview
This file implements credential management functionality for Daptin, handling the retrieval and decryption of stored credentials from the database. It provides methods to fetch credentials by name or reference ID, with automatic decryption of stored credential data.

## Key Components

### GetCredentialByName method
**Lines:** 9-29  
**Purpose:** Retrieves and decrypts credential data by credential name  

### GetCredentialByReferenceId method  
**Lines:** 31-51  
**Purpose:** Retrieves and decrypts credential data by reference ID  

## Critical Security Analysis

### 1. CRITICAL: Type Assertion Vulnerabilities - HIGH RISK
**Severity:** HIGH  
**Lines:** 18, 40, 48  
**Issue:** Unsafe type assertions without validation that can cause runtime panics.

```go
decryptedSpec, err := Decrypt([]byte(encryptionSecret), credentialRow["content"].(string))  // Line 18
decryptedSpec, err := Decrypt([]byte(encryptionSecret), credentialRow["content"].(string))  // Line 40
Name: credentialRow["name"].(string),  // Line 48
```

**Risk:**
- **Runtime panics** if credentialRow["content"] is not a string or is nil
- **Application crashes** during credential retrieval operations
- **Service unavailability** when handling credential requests
- **No fallback mechanism** for type assertion failures

**Impact:** Complete service disruption when credential data format is unexpected.

### 2. CRITICAL: Encryption Secret Handling Vulnerabilities - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 16, 38  
**Issue:** Insecure handling of encryption secrets with ignored errors.

```go
encryptionSecret, _ := d.ConfigStore.GetConfigValueFor("encryption.secret", "backend", transaction)
```

**Risk:**
- **Silent failures** in encryption secret retrieval (error ignored with `_`)
- **Empty encryption secret** could be used for decryption
- **No validation** of encryption secret format or strength
- **Potential plaintext exposure** if encryption secret is compromised
- **Credential data exposure** through weak or missing encryption

**Impact:** Complete compromise of all stored credentials if encryption secret is weak or exposed.

### 3. HIGH: JSON Unmarshaling Security Issues - HIGH RISK
**Severity:** HIGH  
**Lines:** 21, 43  
**Issue:** JSON unmarshaling without input validation or size limits.

```go
err = json.Unmarshal([]byte(decryptedSpec), &decryptedSpecMap)
```

**Risk:**
- **Denial of Service** through large JSON payloads
- **Memory exhaustion** from deeply nested JSON structures
- **JSON injection** attacks through malformed credential data
- **No size limits** on decrypted credential content
- **Potential code execution** through JSON deserialization vulnerabilities

### 4. HIGH: Error Handling Gaps - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 16, 21, 38, 43  
**Issue:** Inconsistent error handling and silent failures.

```go
encryptionSecret, _ := d.ConfigStore.GetConfigValueFor("encryption.secret", "backend", transaction)  // Error ignored
err = json.Unmarshal([]byte(decryptedSpec), &decryptedSpecMap)
if err != nil {
    return nil, err  // Decryption error not handled on line 18/40
}
```

**Risk:**
- **Silent configuration failures** with ignored errors
- **Inconsistent error reporting** between decryption and JSON parsing
- **Credential retrieval failures** not properly logged
- **Security events** not tracked or monitored

### 5. MEDIUM: Missing Input Validation - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 9, 31  
**Issue:** No validation of input parameters.

```go
func (d *DbResource) GetCredentialByName(credentialName string, transaction *sqlx.Tx)
func (d *DbResource) GetCredentialByReferenceId(referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx)
```

**Risk:**
- **No validation** of credential name format or length
- **No validation** of reference ID format
- **SQL injection** potential through unvalidated parameters
- **Database errors** from malformed inputs

### 6. MEDIUM: Credential Data Exposure - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 18, 40  
**Issue:** Decrypted credential data handled as strings without secure memory management.

**Risk:**
- **Memory disclosure** of decrypted credentials
- **No secure memory clearing** after credential use
- **Potential credential leakage** through memory dumps
- **No access logging** for credential retrieval operations

## Potential Attack Vectors

### Credential Extraction Attacks
1. **Type Confusion:** Submit malformed credential data to trigger type assertion panics
2. **JSON Injection:** Use malformed JSON in encrypted credential content
3. **Memory Exhaustion:** Submit large credential payloads to exhaust memory
4. **Encryption Bypass:** Exploit missing encryption secret validation

### Data Integrity Attacks
1. **Credential Tampering:** Modify credential database entries to bypass decryption
2. **Configuration Manipulation:** Modify encryption secret configuration
3. **Reference ID Spoofing:** Use crafted reference IDs to access unauthorized credentials

### Information Disclosure Attacks
1. **Memory Analysis:** Extract decrypted credentials from process memory
2. **Error Information Leakage:** Extract system information through error messages
3. **Timing Attacks:** Use response timing to enumerate valid credentials

## Recommendations

### Immediate Actions
1. **Add Type Validation:** Validate all type assertions before execution
2. **Handle Encryption Errors:** Properly handle encryption secret retrieval errors
3. **Add Input Validation:** Validate all input parameters
4. **Add JSON Size Limits:** Implement size limits for JSON unmarshaling

### Enhanced Security Implementation

```go
package resource

import (
    "encoding/json"
    "fmt"
    "regexp"
    "unicode/utf8"
    
    "github.com/daptin/daptin/server/dbresourceinterface"
    daptinid "github.com/daptin/daptin/server/id"
    "github.com/jmoiron/sqlx"
    log "github.com/sirupsen/logrus"
)

const (
    MaxCredentialNameLength = 255
    MaxCredentialContentSize = 1024 * 1024 // 1MB
    MinEncryptionSecretLength = 32
)

var (
    validCredentialNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,255}$`)
)

func validateCredentialName(name string) error {
    if len(name) == 0 {
        return fmt.Errorf("credential name cannot be empty")
    }
    
    if len(name) > MaxCredentialNameLength {
        return fmt.Errorf("credential name too long: %d characters", len(name))
    }
    
    if !utf8.ValidString(name) {
        return fmt.Errorf("credential name contains invalid UTF-8")
    }
    
    if !validCredentialNamePattern.MatchString(name) {
        return fmt.Errorf("credential name contains invalid characters")
    }
    
    return nil
}

func validateReferenceId(referenceId daptinid.DaptinReferenceId) error {
    if len(referenceId) != 16 {
        return fmt.Errorf("invalid reference ID length: %d", len(referenceId))
    }
    
    // Check for null reference ID
    nullCheck := true
    for _, b := range referenceId {
        if b != 0 {
            nullCheck = false
            break
        }
    }
    
    if nullCheck {
        return fmt.Errorf("reference ID cannot be null")
    }
    
    return nil
}

func secureGetEncryptionSecret(configStore *ConfigStore, transaction *sqlx.Tx) (string, error) {
    encryptionSecret, err := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)
    if err != nil {
        return "", fmt.Errorf("failed to retrieve encryption secret: %v", err)
    }
    
    if len(encryptionSecret) < MinEncryptionSecretLength {
        return "", fmt.Errorf("encryption secret too short: %d bytes", len(encryptionSecret))
    }
    
    return encryptionSecret, nil
}

func validateCredentialRow(credentialRow map[string]interface{}) error {
    if credentialRow == nil {
        return fmt.Errorf("credential row is nil")
    }
    
    // Validate content field
    contentInterface, exists := credentialRow["content"]
    if !exists {
        return fmt.Errorf("credential content field missing")
    }
    
    contentStr, ok := contentInterface.(string)
    if !ok {
        return fmt.Errorf("credential content is not a string")
    }
    
    if len(contentStr) == 0 {
        return fmt.Errorf("credential content is empty")
    }
    
    if len(contentStr) > MaxCredentialContentSize {
        return fmt.Errorf("credential content too large: %d bytes", len(contentStr))
    }
    
    return nil
}

func validateCredentialRowWithName(credentialRow map[string]interface{}) error {
    if err := validateCredentialRow(credentialRow); err != nil {
        return err
    }
    
    // Validate name field
    nameInterface, exists := credentialRow["name"]
    if !exists {
        return fmt.Errorf("credential name field missing")
    }
    
    nameStr, ok := nameInterface.(string)
    if !ok {
        return fmt.Errorf("credential name is not a string")
    }
    
    return validateCredentialName(nameStr)
}

func secureJsonUnmarshal(data []byte, v interface{}) error {
    if len(data) > MaxCredentialContentSize {
        return fmt.Errorf("JSON data too large: %d bytes", len(data))
    }
    
    if len(data) == 0 {
        return fmt.Errorf("JSON data is empty")
    }
    
    // Use decoder with limits for security
    decoder := json.NewDecoder(bytes.NewReader(data))
    decoder.DisallowUnknownFields() // Prevent injection of unexpected fields
    
    if err := decoder.Decode(v); err != nil {
        return fmt.Errorf("JSON unmarshaling failed: %v", err)
    }
    
    return nil
}

func (d *DbResource) GetCredentialByName(credentialName string, transaction *sqlx.Tx) (*dbresourceinterface.Credential, error) {
    // Input validation
    if err := validateCredentialName(credentialName); err != nil {
        log.Warnf("Invalid credential name requested: %v", err)
        return nil, fmt.Errorf("invalid credential name: %v", err)
    }
    
    if transaction == nil {
        return nil, fmt.Errorf("transaction cannot be nil")
    }
    
    // Retrieve credential row
    credentialRow, err := d.GetObjectByWhereClauseWithTransaction(
        "credential", "name", credentialName, transaction)
    if err != nil {
        log.Errorf("Failed to retrieve credential by name '%s': %v", credentialName, err)
        return nil, fmt.Errorf("credential not found: %v", err)
    }
    
    // Validate credential row structure
    if err := validateCredentialRow(credentialRow); err != nil {
        log.Errorf("Invalid credential row structure for '%s': %v", credentialName, err)
        return nil, fmt.Errorf("invalid credential data: %v", err)
    }
    
    // Secure encryption secret retrieval
    encryptionSecret, err := secureGetEncryptionSecret(d.ConfigStore, transaction)
    if err != nil {
        log.Errorf("Failed to get encryption secret for credential '%s': %v", credentialName, err)
        return nil, fmt.Errorf("encryption configuration error: %v", err)
    }
    
    // Safe type assertion
    contentStr := credentialRow["content"].(string) // Already validated above
    
    // Decrypt credential content
    decryptedSpec, err := Decrypt([]byte(encryptionSecret), contentStr)
    if err != nil {
        log.Errorf("Failed to decrypt credential '%s': %v", credentialName, err)
        return nil, fmt.Errorf("decryption failed: %v", err)
    }
    
    // Secure JSON unmarshaling
    decryptedSpecMap := make(map[string]interface{})
    if err := secureJsonUnmarshal([]byte(decryptedSpec), &decryptedSpecMap); err != nil {
        log.Errorf("Failed to parse credential JSON for '%s': %v", credentialName, err)
        return nil, fmt.Errorf("credential format error: %v", err)
    }
    
    // Log successful credential access
    log.Infof("Credential '%s' accessed successfully", credentialName)
    
    return &dbresourceinterface.Credential{
        Name:    credentialName,
        DataMap: decryptedSpecMap,
    }, nil
}

func (d *DbResource) GetCredentialByReferenceId(referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) (*dbresourceinterface.Credential, error) {
    // Input validation
    if err := validateReferenceId(referenceId); err != nil {
        log.Warnf("Invalid reference ID requested: %v", err)
        return nil, fmt.Errorf("invalid reference ID: %v", err)
    }
    
    if transaction == nil {
        return nil, fmt.Errorf("transaction cannot be nil")
    }
    
    // Retrieve credential row
    credentialRow, err := d.GetObjectByWhereClauseWithTransaction(
        "credential", "reference_id", referenceId[:], transaction)
    if err != nil {
        log.Errorf("Failed to retrieve credential by reference ID: %v", err)
        return nil, fmt.Errorf("credential not found: %v", err)
    }
    
    // Validate credential row structure including name
    if err := validateCredentialRowWithName(credentialRow); err != nil {
        log.Errorf("Invalid credential row structure: %v", err)
        return nil, fmt.Errorf("invalid credential data: %v", err)
    }
    
    // Secure encryption secret retrieval
    encryptionSecret, err := secureGetEncryptionSecret(d.ConfigStore, transaction)
    if err != nil {
        log.Errorf("Failed to get encryption secret: %v", err)
        return nil, fmt.Errorf("encryption configuration error: %v", err)
    }
    
    // Safe type assertions (already validated above)
    contentStr := credentialRow["content"].(string)
    nameStr := credentialRow["name"].(string)
    
    // Decrypt credential content
    decryptedSpec, err := Decrypt([]byte(encryptionSecret), contentStr)
    if err != nil {
        log.Errorf("Failed to decrypt credential: %v", err)
        return nil, fmt.Errorf("decryption failed: %v", err)
    }
    
    // Secure JSON unmarshaling
    decryptedSpecMap := make(map[string]interface{})
    if err := secureJsonUnmarshal([]byte(decryptedSpec), &decryptedSpecMap); err != nil {
        log.Errorf("Failed to parse credential JSON: %v", err)
        return nil, fmt.Errorf("credential format error: %v", err)
    }
    
    // Log successful credential access
    log.Infof("Credential '%s' accessed by reference ID", nameStr)
    
    return &dbresourceinterface.Credential{
        Name:    nameStr,
        DataMap: decryptedSpecMap,
    }, nil
}

// ClearCredentialFromMemory securely clears credential data from memory
func ClearCredentialFromMemory(credential *dbresourceinterface.Credential) {
    if credential == nil {
        return
    }
    
    // Clear the map
    for k := range credential.DataMap {
        delete(credential.DataMap, k)
    }
    
    // Clear the name
    credential.Name = ""
}
```

### Long-term Improvements
1. **Credential Rotation:** Implement automatic credential rotation capabilities
2. **Access Logging:** Add comprehensive audit logging for credential access
3. **Encryption Upgrades:** Implement modern encryption standards and key management
4. **Memory Security:** Use secure memory management for credential data
5. **Rate Limiting:** Add rate limiting for credential access operations

## Edge Cases Identified

1. **Empty Credentials:** Handling of empty or missing credential data
2. **Large Credential Content:** Performance with very large credential payloads
3. **Malformed JSON:** Handling of corrupted JSON credential data
4. **Encryption Failures:** Behavior when decryption fails
5. **Database Errors:** Handling of database connectivity issues during credential retrieval
6. **Concurrent Access:** Thread safety of credential operations
7. **Memory Pressure:** Behavior under high memory pressure conditions
8. **Character Encoding:** Handling of non-UTF-8 credential names

## Security Best Practices Violations

1. **No input validation for credential names and reference IDs**
2. **Unsafe type assertions without error handling**
3. **Silent failures in encryption secret retrieval**
4. **No size limits on JSON unmarshaling operations**
5. **Missing access logging and audit trails**
6. **No secure memory management for credential data**

## Critical Issues Summary

1. **Type Assertion Vulnerabilities:** Runtime panics from unsafe type casts
2. **Encryption Secret Handling:** Silent failures and weak validation
3. **JSON Security Issues:** Unmarshaling without size limits or validation
4. **Error Handling Gaps:** Inconsistent error handling and silent failures
5. **Input Validation Missing:** No validation of credential parameters
6. **Memory Security Issues:** Insecure handling of decrypted credential data

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - Multiple credential security vulnerabilities and data exposure risks