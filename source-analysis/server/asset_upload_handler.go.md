# Security Analysis: server/asset_upload_handler.go

**File:** `server/asset_upload_handler.go`  
**Lines of Code:** 766  
**Primary Function:** Asset upload handler managing file uploads to cloud storage with streaming support, multipart uploads, presigned URLs, and database integration for file management

## Summary

This file implements a comprehensive asset upload system for the Daptin CMS. It handles various upload methods including direct streaming, presigned URLs, and S3 multipart uploads. The system supports multiple cloud storage providers through rclone integration, manages upload sessions with database tracking, implements permission validation, and provides upload completion verification. This is a critical security component as it handles file uploads, cloud storage access, and asset management with extensive functionality.

## Key Components

### AssetUploadHandler function
**Lines:** 29-156  
**Purpose:** Main HTTP handler for asset uploads with operation routing  

### Upload initialization and completion
**Lines:** 158-545  
**Purpose:** Handles upload session management and finalization  

### Cloud storage integration
**Lines:** 572-765  
**Purpose:** Manages cloud storage credentials and file operations  

## Security Analysis

### 1. CRITICAL: Type Assertion Vulnerabilities - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 102, 175, 387, 397, 420, 421, 458, 498, 501, 504, 643, 716  
**Issue:** Multiple unsafe type assertions without validation that can panic the application.

```go
sessionUser = user.(*auth.SessionUser)  // Can panic if wrong type
if providerType, ok := assetCache.Credentials["type"].(string); ok && providerType == "s3" {
s3UploadId, _ := metadata["s3_upload_id"].(string)  // Ignores error
fileName, ok := metadata["fileName"].(string)       // No error handling
```

**Risk:**
- **Application crashes** from type assertion failures
- **Service disruption** affecting all upload functionality
- **DoS attacks** through crafted metadata or credentials
- **Runtime panics** causing system instability

### 2. CRITICAL: Path Injection Vulnerability - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 178, 190, 286, 323, 430, 442, 590, 602, 613, 657, 669, 730, 742  
**Issue:** File paths constructed from user input without validation.

```go
keyPath := assetCache.Keyname + "/" + fileName  // User-controlled path construction
localPath := filepath.Join(assetCache.LocalSyncPath, fileName)  // Direct path join
_, err := fs.NewFs(ctx, assetCache.CloudStore.RootPath+"/"+assetCache.Keyname)  // Path injection
```

**Risk:**
- **Directory traversal attacks** accessing unauthorized locations
- **File system manipulation** through crafted file names
- **Cloud storage path injection** accessing other buckets/containers
- **System file overwrite** through path manipulation

### 3. CRITICAL: Credential Information Exposure - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 174-175, 198, 412-413, 420, 464, 578-581, 638, 643, 683, 711, 716, 754  
**Issue:** Cloud storage credentials accessed and used without encryption.

```go
if assetCache.Credentials != nil {
    for key, val := range assetCache.Credentials {
        config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))  // Raw credential exposure
    }
}
```

**Risk:**
- **Credential leakage** through memory dumps or logs
- **Unauthorized cloud access** using stolen credentials
- **Data breaches** through compromised storage accounts
- **Financial impact** from unauthorized cloud usage

### 4. HIGH: File Upload Size Bypass - HIGH RISK
**Severity:** HIGH  
**Lines:** 161, 369, 498  
**Issue:** File size controls can be bypassed through header manipulation.

```go
fileSize, _ := strconv.ParseInt(c.GetHeader("X-File-Size"), 10, 64)  // Client-controlled header
// No validation of actual uploaded content size against declared size
```

**Risk:**
- **Storage quota exhaustion** through oversized uploads
- **DoS attacks** via large file uploads
- **Resource consumption** affecting system performance
- **Cost impact** from unlimited storage usage

### 5. HIGH: Insufficient Input Validation - HIGH RISK
**Severity:** HIGH  
**Lines:** 31-33, 36, 57, 275, 363-365, 623-624, 651, 700-702, 724  
**Issue:** URL parameters and form data not validated.

```go
typeName := c.Param("typename")        // No validation
resourceUuid := c.Param("resource_id") // No validation
columnName := c.Param("columnname")    // No validation
fileName := c.Query("filename")        // No validation
```

**Risk:**
- **Injection attacks** through malformed parameters
- **Path traversal** through parameter manipulation
- **Database query injection** through crafted identifiers
- **Resource enumeration** through parameter abuse

### 6. HIGH: Transaction Resource Leak - HIGH RISK
**Severity:** HIGH  
**Lines:** 108-115, 126-154  
**Issue:** Database transactions not properly cleaned up in error scenarios.

```go
tx, err := dbResource.Connection().Beginx()
if err != nil {
    c.AbortWithError(500, err)  // Returns without cleanup
    return
}
tx.Rollback() // May not execute if earlier error occurs
```

**Risk:**
- **Database connection leaks** from abandoned transactions
- **Lock contention** from held transaction locks
- **Performance degradation** from connection pool exhaustion
- **Database instability** under high load

### 7. HIGH: Metadata Injection Vulnerability - HIGH RISK
**Severity:** HIGH  
**Lines:** 379-382, 384-385, 455-461, 497-506, 523  
**Issue:** User-provided metadata used without validation.

```go
var metadata map[string]interface{}
if err := c.ShouldBindJSON(&metadata); err != nil {
    metadata = make(map[string]interface{})  // No validation of bound data
}
// Metadata used directly in database operations and S3 calls
```

**Risk:**
- **JSON injection** through crafted metadata
- **Database corruption** through malicious metadata
- **Cloud storage manipulation** through crafted part data
- **Memory exhaustion** through oversized metadata

### 8. MEDIUM: Progress Tracking Information Disclosure - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 334-338, 555-568  
**Issue:** Upload progress tracking without access controls.

```go
progressReader := &progressReader{
    reader:   c.Request.Body,
    total:    c.Request.ContentLength,  // Client-controlled value
    uploadId: uploadId,                 // May be predictable
}
```

**Risk:**
- **Upload enumeration** through predictable upload IDs
- **Progress information leakage** to unauthorized users
- **Resource monitoring** by attackers
- **Timing attacks** through progress observation

### 9. MEDIUM: File Verification Bypass - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 510, 587-609  
**Issue:** File existence verification can be bypassed or manipulated.

```go
fileExists := verifyFileInCloud(assetCache, fileName)
if !fileExists {
    c.JSON(http.StatusBadRequest, gin.H{
        "error": "file not found in cloud storage",  // May reveal storage structure
    })
}
```

**Risk:**
- **Storage enumeration** through file existence checks
- **Information disclosure** about storage structure
- **Race conditions** in file verification
- **Bypass of completion checks** through timing

### 10. LOW: Error Information Disclosure - LOW RISK
**Severity:** LOW  
**Lines:** 67, 73, 80, 203, 225, 238, 257, 294, 303, 325, 345, 466, 482, 525, 533, 685, 756  
**Issue:** Detailed error messages exposing system information.

```go
log.Errorf("table not found [%v]", typeName)
log.Errorf("Failed to complete S3 multipart upload: %v", err)
c.JSON(http.StatusInternalServerError, gin.H{
    "error": fmt.Sprintf("failed to complete multipart upload: %v", err),
})
```

**Risk:**
- **System information disclosure** through error messages
- **Cloud infrastructure enumeration** through error patterns
- **Attack surface mapping** from detailed errors
- **Configuration information leakage** in error responses

## Potential Attack Vectors

### File Upload Attacks
1. **Path Traversal Upload:** Upload files to unauthorized locations through path manipulation
2. **Size Bypass Upload:** Upload oversized files by manipulating size headers
3. **Metadata Injection:** Inject malicious data through upload metadata
4. **Multipart Abuse:** Exploit multipart upload features for resource exhaustion

### Cloud Storage Attacks
1. **Credential Harvesting:** Extract cloud storage credentials from memory or logs
2. **Bucket Enumeration:** Discover storage buckets through path manipulation
3. **Unauthorized Access:** Access other users' files through path injection
4. **Storage Quota Abuse:** Exhaust storage quotas through unlimited uploads

### Database-Based Attacks
1. **Transaction DoS:** Exhaust database connections through transaction leaks
2. **Metadata Injection:** Corrupt database through malicious upload metadata
3. **Permission Bypass:** Bypass upload permissions through race conditions
4. **Resource Enumeration:** Discover resources through parameter manipulation

### Denial of Service Attacks
1. **Type Assertion DoS:** Trigger panics through malformed credentials or metadata
2. **Memory Exhaustion:** Upload large files to exhaust system memory
3. **Connection Exhaustion:** Hold database connections through transaction leaks
4. **Storage Exhaustion:** Fill storage through unlimited uploads

## Recommendations

### Immediate Actions
1. **Add Input Validation:** Validate all URL parameters, headers, and metadata
2. **Fix Type Assertions:** Add proper type validation with error handling
3. **Sanitize File Paths:** Validate and sanitize all file path constructions
4. **Secure Credential Handling:** Encrypt credentials and implement secure storage

### Enhanced Security Implementation

```go
package server

import (
    "context"
    "errors"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"
    "unicode/utf8"
    
    "github.com/daptin/daptin/server/assetcachepojo"
    "github.com/daptin/daptin/server/auth"
    "github.com/jmoiron/sqlx"
    "github.com/artpar/rclone/fs"
    "github.com/artpar/rclone/fs/config"
    "github.com/artpar/rclone/fs/operations"
    daptinid "github.com/daptin/daptin/server/id"
    "github.com/daptin/daptin/server/resource"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    log "github.com/sirupsen/logrus"
)

const (
    MaxFileNameLength = 255
    MaxFileSizeLimit = 10 * 1024 * 1024 * 1024 // 10GB
    MaxMetadataSize = 64 * 1024 // 64KB
    MaxUploadIdLength = 36
    MaxTypeNameLength = 255
    MaxColumnNameLength = 255
    MultipartThreshold = 100 * 1024 * 1024 // 100MB
    MaxPartNumber = 10000
    MinPartSize = 5 * 1024 * 1024 // 5MB
)

var (
    validFileNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
    validTypeNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
    validColumnNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]*$`)
    validUploadIdPattern = regexp.MustCompile(`^[a-fA-F0-9-]{36}$`)
    validOperationPattern = regexp.MustCompile(`^(init|complete|stream|get_part_url|abort)$`)
)

// validateFileName validates file names for security
func validateFileName(fileName string) error {
    if len(fileName) == 0 {
        return fmt.Errorf("file name cannot be empty")
    }
    
    if len(fileName) > MaxFileNameLength {
        return fmt.Errorf("file name too long: %d", len(fileName))
    }
    
    if !utf8.ValidString(fileName) {
        return fmt.Errorf("file name contains invalid UTF-8")
    }
    
    if !validFileNamePattern.MatchString(fileName) {
        return fmt.Errorf("invalid file name format")
    }
    
    // Check for dangerous patterns
    dangerousPatterns := []string{"..", "/", "\\", "\x00", "\n", "\r"}
    for _, pattern := range dangerousPatterns {
        if strings.Contains(fileName, pattern) {
            return fmt.Errorf("file name contains dangerous pattern")
        }
    }
    
    return nil
}

// validateFileSize validates file size limits
func validateFileSize(fileSize int64) error {
    if fileSize < 0 {
        return fmt.Errorf("invalid file size: %d", fileSize)
    }
    
    if fileSize > MaxFileSizeLimit {
        return fmt.Errorf("file size exceeds limit: %d", fileSize)
    }
    
    return nil
}

// validateMetadata validates upload metadata
func validateMetadata(metadata map[string]interface{}) error {
    if metadata == nil {
        return nil
    }
    
    // Check metadata size
    metadataBytes := 0
    for key, value := range metadata {
        metadataBytes += len(key)
        if str, ok := value.(string); ok {
            metadataBytes += len(str)
        }
    }
    
    if metadataBytes > MaxMetadataSize {
        return fmt.Errorf("metadata too large: %d bytes", metadataBytes)
    }
    
    // Validate metadata keys and values
    for key, value := range metadata {
        if len(key) > 100 {
            return fmt.Errorf("metadata key too long: %s", key)
        }
        
        if !utf8.ValidString(key) {
            return fmt.Errorf("metadata key contains invalid UTF-8")
        }
        
        // Validate value based on type
        switch v := value.(type) {
        case string:
            if len(v) > 10000 {
                return fmt.Errorf("metadata value too long")
            }
            if !utf8.ValidString(v) {
                return fmt.Errorf("metadata value contains invalid UTF-8")
            }
        case float64:
            // Numeric values are acceptable
        case bool:
            // Boolean values are acceptable
        case []interface{}:
            if len(v) > 100 {
                return fmt.Errorf("metadata array too large")
            }
        default:
            return fmt.Errorf("unsupported metadata type: %T", value)
        }
    }
    
    return nil
}

// safeTypeAssertion performs type assertion with error handling
func safeTypeAssertion[T any](value interface{}, fieldName string) (T, error) {
    var zero T
    if value == nil {
        return zero, fmt.Errorf("field '%s' is nil", fieldName)
    }
    
    result, ok := value.(T)
    if !ok {
        return zero, fmt.Errorf("field '%s' has invalid type, expected %T, got %T", fieldName, zero, value)
    }
    
    return result, nil
}

// validateUploadPath validates and sanitizes upload paths
func validateUploadPath(basePath, fileName string) (string, error) {
    if err := validateFileName(fileName); err != nil {
        return "", fmt.Errorf("invalid file name: %v", err)
    }
    
    // Clean the path to remove any traversal attempts
    cleanFileName := filepath.Clean(fileName)
    fullPath := filepath.Join(basePath, cleanFileName)
    
    // Ensure the resulting path is within the base directory
    if !strings.HasPrefix(fullPath, basePath) {
        return "", fmt.Errorf("path traversal detected")
    }
    
    return fullPath, nil
}

// secureTransactionExecute executes database operations with proper cleanup
func secureTransactionExecute(dbResource *resource.DbResource, operation func(*sqlx.Tx) error) error {
    tx, err := dbResource.Connection().Beginx()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %v", err)
    }
    
    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            panic(p) // Re-panic after cleanup
        }
    }()
    
    err = operation(tx)
    if err != nil {
        if rollbackErr := tx.Rollback(); rollbackErr != nil {
            log.Printf("Failed to rollback transaction: %v", rollbackErr)
        }
        return err
    }
    
    if err = tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %v", err)
    }
    
    return nil
}

// SecureAssetUploadHandler creates a secure asset upload handler
func SecureAssetUploadHandler(cruds map[string]*resource.DbResource) func(c *gin.Context) {
    return func(c *gin.Context) {
        // Validate input parameters
        typeName := c.Param("typename")
        if !validTypeNamePattern.MatchString(typeName) || len(typeName) > MaxTypeNameLength {
            log.Warnf("Invalid type name: %s", typeName)
            c.AbortWithStatus(http.StatusBadRequest)
            return
        }
        
        resourceUuid := c.Param("resource_id")
        uuidDir := daptinid.InterfaceToDIR(resourceUuid)
        if uuidDir == daptinid.NullReferenceId {
            c.AbortWithStatus(http.StatusBadRequest)
            return
        }
        
        columnName := c.Param("columnname")
        if !validColumnNamePattern.MatchString(columnName) || len(columnName) > MaxColumnNameLength {
            log.Warnf("Invalid column name: %s", columnName)
            c.AbortWithStatus(http.StatusBadRequest)
            return
        }
        
        // Validate operation parameter
        operation := c.Query("operation")
        if operation != "" && !validOperationPattern.MatchString(operation) {
            log.Warnf("Invalid operation: %s", operation)
            c.AbortWithStatus(http.StatusBadRequest)
            return
        }
        
        // Set default operation based on HTTP method
        if operation == "" {
            switch c.Request.Method {
            case "GET":
                operation = "get_part_url"
            case "DELETE":
                operation = "abort"
            case "POST":
                if c.Query("upload_id") != "" || c.PostForm("upload_id") != "" {
                    operation = "complete"
                } else {
                    operation = "init"
                }
            default:
                operation = "stream"
            }
        }
        
        // Validate filename for operations that require it
        fileName := c.Query("filename")
        if fileName != "" {
            if err := validateFileName(fileName); err != nil {
                log.Warnf("Invalid filename: %v", err)
                c.AbortWithStatus(http.StatusBadRequest)
                return
            }
        } else if operation != "complete" {
            c.AbortWithError(400, errors.New("filename query parameter is required"))
            return
        }
        
        // Validate file size if provided
        if fileSizeStr := c.GetHeader("X-File-Size"); fileSizeStr != "" {
            if fileSize, err := strconv.ParseInt(fileSizeStr, 10, 64); err != nil {
                log.Warnf("Invalid file size header: %s", fileSizeStr)
                c.AbortWithStatus(http.StatusBadRequest)
                return
            } else if err := validateFileSize(fileSize); err != nil {
                log.Warnf("File size validation failed: %v", err)
                c.AbortWithStatus(http.StatusRequestEntityTooLarge)
                return
            }
        }
        
        // Validate table and column
        dbResource, ok := cruds[typeName]
        if !ok || dbResource == nil {
            log.Warnf("Table not found: %s", typeName)
            c.AbortWithStatus(http.StatusNotFound)
            return
        }
        
        colInfo, ok := dbResource.TableInfo().GetColumnByName(columnName)
        if !ok || colInfo == nil || !colInfo.IsForeignKey || colInfo.ForeignKeyData.DataSource != "cloud_store" {
            log.Warnf("Invalid cloud_store column: %s", columnName)
            c.AbortWithStatus(http.StatusBadRequest)
            return
        }
        
        // Get asset cache
        assetCache, ok := cruds["world"].AssetFolderCache[typeName][columnName]
        if !ok {
            c.AbortWithStatus(http.StatusInternalServerError)
            return
        }
        
        // Validate user authentication
        user := c.Request.Context().Value("user")
        sessionUser, err := safeTypeAssertion[*auth.SessionUser](user, "user")
        if err != nil {
            log.Warnf("Invalid user context: %v", err)
            c.AbortWithStatus(http.StatusUnauthorized)
            return
        }
        
        originalRowReference := map[string]interface{}{
            "__type":                typeName,
            "reference_id":          uuidDir,
            "relation_reference_id": daptinid.NullReferenceId,
        }
        
        // Check permissions with secure transaction handling
        err = secureTransactionExecute(dbResource, func(tx *sqlx.Tx) error {
            permission := dbResource.GetRowPermissionWithTransaction(originalRowReference, tx)
            if !permission.CanUpdate(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId) {
                return fmt.Errorf("insufficient permissions")
            }
            return nil
        })
        
        if err != nil {
            c.AbortWithStatus(http.StatusUnauthorized)
            return
        }
        
        // Route to appropriate handler based on operation
        switch operation {
        case "stream":
            handleSecureStreamUpload(c, fileName, assetCache)
        case "init":
            handleSecureUploadInit(c, cruds, typeName, columnName, fileName, uuidDir, assetCache)
        case "complete":
            handleSecureUploadComplete(c, cruds, typeName, columnName, fileName, uuidDir)
        case "get_part_url":
            handleSecureGetPartPresignedURL(c, assetCache)
        case "abort":
            handleSecureAbortMultipartUpload(c, assetCache)
        default:
            c.AbortWithStatus(http.StatusBadRequest)
        }
    }
}

// Additional secure handler functions would follow similar patterns for:
// - handleSecureStreamUpload
// - handleSecureUploadInit
// - handleSecureUploadComplete
// - handleSecureGetPartPresignedURL
// - handleSecureAbortMultipartUpload
// Each with comprehensive input validation and security controls

// AssetUploadHandler maintains backward compatibility
func AssetUploadHandler(cruds map[string]*resource.DbResource) func(c *gin.Context) {
    return SecureAssetUploadHandler(cruds)
}
```

### Long-term Improvements
1. **Virus Scanning:** Implement malware scanning for uploaded files
2. **Content Validation:** Validate file content matches declared MIME types
3. **Rate Limiting:** Add rate limiting for upload operations
4. **Audit Logging:** Log all upload attempts and decisions
5. **Monitoring Integration:** Add comprehensive metrics and alerting

## Edge Cases Identified

1. **Large File Uploads:** Handling files approaching storage limits
2. **Concurrent Uploads:** Multiple simultaneous uploads to same resource
3. **Network Interruptions:** Connection drops during multipart uploads
4. **Cloud Service Outages:** Storage service unavailability during uploads
5. **Malformed Multipart Data:** Invalid part data in multipart completions
6. **Memory Pressure:** Upload processing under high memory pressure
7. **Transaction Timeouts:** Long-running upload transactions failing
8. **Credential Rotation:** Handling of rotated or expired cloud credentials

## Security Best Practices Violations

1. **Unsafe type assertions** throughout the codebase without validation
2. **Path injection vulnerabilities** from user-controlled path construction
3. **Raw credential access** without encryption or secure handling
4. **Missing input validation** for critical parameters and metadata
5. **Transaction resource leaks** from improper cleanup
6. **File size bypass** through client-controlled headers
7. **Information disclosure** through detailed error messages
8. **Insufficient access controls** for upload operations

## Positive Security Aspects

1. **Permission checking** integration with database transactions
2. **Progress tracking** for upload monitoring
3. **Multipart upload support** for large files
4. **Transaction-based operations** for data consistency

## Critical Issues Summary

1. **Type Assertion Vulnerabilities:** Multiple unsafe type assertions can panic application
2. **Path Injection Vulnerability:** File paths constructed from user input without validation
3. **Credential Information Exposure:** Cloud storage credentials accessed without encryption
4. **File Upload Size Bypass:** Size controls can be bypassed through header manipulation
5. **Insufficient Input Validation:** Parameters and metadata not validated for security
6. **Transaction Resource Leak:** Database transactions not properly cleaned up
7. **Metadata Injection Vulnerability:** User metadata used without validation
8. **Progress Tracking Information Disclosure:** Upload progress exposed without access controls
9. **File Verification Bypass:** File existence verification can be manipulated
10. **Error Information Disclosure:** Detailed error messages expose system information

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - Asset upload handler with multiple critical vulnerabilities including path injection, credential exposure, and type assertion failures