# Security Analysis: server/asset_presigned_url.go

**File:** `server/asset_presigned_url.go`  
**Type:** Presigned URL generation for cloud storage with S3 implementation  
**Lines of Code:** 489  

## Overview
This file implements presigned URL generation for cloud storage uploads, primarily focusing on AWS S3 with multipart upload support. It handles credential management, URL signing, and multipart upload lifecycle operations including initiation, part uploads, completion, and abortion.

## Key Components

### generatePresignedURL function
**Lines:** 23-80  
**Purpose:** Main function to generate presigned URLs based on storage provider  

### S3 multipart upload functions
**Lines:** 83-488  
**Purpose:** Complete S3 multipart upload implementation with credential handling  

### Placeholder functions for other providers
**Lines:** 273-293  
**Purpose:** Stub implementations for GCS and Azure storage providers  

## Security Analysis

### 1. CRITICAL: Credential Information Exposure - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 30-34, 85-86, 90-91, 139-140, 144-145, 200-201, 205-206, 309-310, 314-315, 437-438, 442-443  
**Issue:** Cloud storage credentials accessed and used without encryption or secure handling.

```go
for key, val := range assetCache.Credentials {
    config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))  // Raw credential exposure
}

accessKeyID, ok := credentials["access_key_id"].(string)  // Type assertion without validation
secretAccessKey, ok := credentials["secret_access_key"].(string)  // Raw credential access
```

**Risk:**
- **Credential leakage** through memory dumps, logs, or error messages
- **Unauthorized cloud access** if credentials are compromised
- **Data breaches** through stolen cloud storage access
- **Financial impact** from unauthorized cloud usage

### 2. CRITICAL: Type Assertion Vulnerabilities - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 38, 85-86, 90-91, 95-96, 100, 139-140, 144-145, 149-150, 154, 200-201, 205-206, 210-211, 215, 309-310, 314-315, 319-320, 324, 354-359, 365, 437-438, 442-443, 447-448, 452  
**Issue:** Multiple unsafe type assertions without validation that can panic the application.

```go
if providerType, ok := assetCache.Credentials["type"].(string); ok && providerType == "s3" {
accessKeyID, ok := credentials["access_key_id"].(string)  // Can panic if wrong type
// ... many more similar patterns throughout file
```

**Risk:**
- **Application crashes** from type assertion failures
- **Service disruption** affecting all users
- **DoS attacks** through crafted credential data
- **Runtime panics** causing system instability

### 3. HIGH: Path Injection Vulnerability - HIGH RISK
**Severity:** HIGH  
**Lines:** 41, 54, 71  
**Issue:** File paths and keys constructed from user input without validation.

```go
keyPath := assetCache.Keyname + "/" + fileName  // User-controlled path construction
cloudPath := assetCache.CloudStore.RootPath + "/" + assetCache.Keyname  // Path injection risk
```

**Risk:**
- **Path traversal attacks** through crafted filenames
- **Unauthorized file access** outside intended directories
- **Storage bucket enumeration** through path manipulation
- **Data exfiltration** through controlled key paths

### 4. HIGH: Insufficient Input Validation - HIGH RISK
**Severity:** HIGH  
**Lines:** 23, 83, 137, 296, 307, 435  
**Issue:** Function parameters not validated for security constraints.

```go
func generatePresignedURL(assetCache *assetcachepojo.AssetFolderCache, fileName string, uploadId string) {
    // No validation of fileName, uploadId, or assetCache fields
}

func InitiateS3MultipartUpload(credentials map[string]interface{}, bucketName string, keyPath string) {
    // No validation of bucketName or keyPath
}
```

**Risk:**
- **Injection attacks** through malformed parameters
- **Storage service abuse** through invalid requests
- **Resource consumption** from unlimited parameter sizes
- **System compromise** through crafted input data

### 5. HIGH: Error Information Disclosure - HIGH RISK
**Severity:** HIGH  
**Lines:** 61, 74, 87, 92, 111, 129, 141, 146, 165, 191, 202, 207, 226, 257, 280, 292, 303, 311, 316, 335, 421, 431, 439, 444, 463, 483  
**Issue:** Detailed error messages exposing internal system information.

```go
return nil, fmt.Errorf("could not extract bucket name from root path: %s", rootPath)
return "", fmt.Errorf("missing access_key_id in S3 credentials")
return "", fmt.Errorf("failed to create AWS config: %v", err)
```

**Risk:**
- **Information disclosure** about system configuration
- **Cloud infrastructure enumeration** through error messages
- **Credential field discovery** through error patterns
- **Attack surface expansion** from exposed internals

### 6. MEDIUM: Hardcoded Configuration Values - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 97, 151, 188, 212, 254, 267, 321, 449  
**Issue:** Hardcoded default values and configuration without security validation.

```go
region = "us-east-1"  // Hardcoded default region
opts.Expires = time.Duration(3600 * time.Second)  // Fixed 1-hour expiry
```

**Risk:**
- **Configuration bypass** through hardcoded defaults
- **Extended token validity** without proper expiration controls
- **Regional security issues** from forced regions
- **Inflexible security policies** from fixed configurations

### 7. MEDIUM: Insufficient Access Controls - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 246-268  
**Issue:** Presigned URLs generated without proper authorization checks.

```go
// Generate standard presigned PUT URL for single file upload
putObjectRequest := &s3.PutObjectInput{
    Bucket: aws.String(bucketName),
    Key:    aws.String(keyPath),
}
// No authorization check for the requesting user
```

**Risk:**
- **Unauthorized file uploads** to cloud storage
- **Storage quota abuse** through unrestricted uploads
- **Data pollution** from malicious file uploads
- **Cost impact** from unlimited storage usage

### 8. MEDIUM: Memory Management Issues - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 347-384  
**Issue:** Large data structures processed without memory limits.

```go
for i, part := range parts {  // No limit on parts array size
    // Processing potentially large arrays without bounds checking
}
```

**Risk:**
- **Memory exhaustion** from large part arrays
- **Performance degradation** from excessive processing
- **DoS attacks** through oversized multipart data
- **System instability** under memory pressure

### 9. LOW: Incomplete Implementation Security - LOW RISK
**Severity:** LOW  
**Lines:** 273-293, 296-304, 429-432  
**Issue:** Placeholder functions that may be exploited if enabled.

```go
func generateGCSSignedURL(...) (map[string]interface{}, error) {
    return nil, fmt.Errorf("GCS signed URL generation not yet implemented")
}
```

**Risk:**
- **Future vulnerabilities** if implementations are added without security review
- **Feature bypass** through error responses
- **Development artifacts** indicating incomplete security
- **Attack surface expansion** from unfinished features

## Potential Attack Vectors

### Credential-Based Attacks
1. **Credential Harvesting:** Extract cloud credentials from memory or logs
2. **Credential Injection:** Inject malicious credentials through parameters
3. **Cross-Account Access:** Use stolen credentials to access other accounts
4. **Privilege Escalation:** Escalate cloud permissions through credential abuse

### Cloud Storage Attacks
1. **Bucket Enumeration:** Discover storage buckets through path manipulation
2. **Unauthorized Uploads:** Upload malicious files using presigned URLs
3. **Data Exfiltration:** Access sensitive files through path traversal
4. **Storage Abuse:** Consume storage quotas through excessive uploads

### Denial of Service Attacks
1. **Memory Exhaustion:** Send large multipart upload requests
2. **Service Disruption:** Trigger type assertion panics
3. **Resource Consumption:** Create many long-lived presigned URLs
4. **API Rate Limiting:** Exhaust cloud API quotas

### Information Disclosure Attacks
1. **Error Mining:** Extract system configuration through error messages
2. **Credential Discovery:** Learn credential structure through error patterns
3. **Infrastructure Mapping:** Map cloud storage configuration
4. **Service Enumeration:** Discover available cloud storage services

## Recommendations

### Immediate Actions
1. **Encrypt Credentials:** Implement secure credential handling and storage
2. **Add Type Validation:** Validate all type assertions with proper error handling
3. **Sanitize Paths:** Validate and sanitize all file paths and keys
4. **Add Authorization:** Implement proper access controls for presigned URL generation

### Enhanced Security Implementation

```go
package server

import (
    "context"
    "crypto/rand"
    "fmt"
    "net/url"
    "path/filepath"
    "regexp"
    "sort"
    "strings"
    "time"
    "unicode/utf8"
    
    "github.com/artpar/rclone/fs"
    "github.com/artpar/rclone/fs/config"
    "github.com/aws/aws-sdk-go-v2/aws"
    awsconfig "github.com/aws/aws-sdk-go-v2/config"
    awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/s3/types"
    "github.com/daptin/daptin/server/assetcachepojo"
    "github.com/daptin/daptin/server/resource"
    log "github.com/sirupsen/logrus"
)

const (
    MaxFileNameLength = 255
    MaxPathLength = 1024
    MaxUploadIdLength = 100
    MaxBucketNameLength = 63
    MaxKeyPathLength = 1024
    MaxPartsCount = 10000
    MinPresignedURLExpiry = 5 * time.Minute
    MaxPresignedURLExpiry = 24 * time.Hour
    DefaultPresignedURLExpiry = 1 * time.Hour
)

var (
    validFileNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
    validBucketNamePattern = regexp.MustCompile(`^[a-z0-9.-]+$`)
    validKeyPathPattern = regexp.MustCompile(`^[a-zA-Z0-9/._-]+$`)
    validUploadIdPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
    validRegionPattern = regexp.MustCompile(`^[a-z0-9-]+$`)
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
        return fmt.Errorf("file name contains invalid characters")
    }
    
    // Check for dangerous patterns
    dangerousPatterns := []string{"..", "/", "\\", "\x00"}
    for _, pattern := range dangerousPatterns {
        if strings.Contains(fileName, pattern) {
            return fmt.Errorf("file name contains dangerous pattern: %s", pattern)
        }
    }
    
    return nil
}

// validateBucketName validates S3 bucket names
func validateBucketName(bucketName string) error {
    if len(bucketName) == 0 {
        return fmt.Errorf("bucket name cannot be empty")
    }
    
    if len(bucketName) > MaxBucketNameLength {
        return fmt.Errorf("bucket name too long: %d", len(bucketName))
    }
    
    if !validBucketNamePattern.MatchString(bucketName) {
        return fmt.Errorf("bucket name contains invalid characters")
    }
    
    return nil
}

// validateKeyPath validates S3 key paths
func validateKeyPath(keyPath string) error {
    if len(keyPath) == 0 {
        return fmt.Errorf("key path cannot be empty")
    }
    
    if len(keyPath) > MaxKeyPathLength {
        return fmt.Errorf("key path too long: %d", len(keyPath))
    }
    
    if !utf8.ValidString(keyPath) {
        return fmt.Errorf("key path contains invalid UTF-8")
    }
    
    if !validKeyPathPattern.MatchString(keyPath) {
        return fmt.Errorf("key path contains invalid characters")
    }
    
    // Prevent path traversal
    if strings.Contains(keyPath, "..") {
        return fmt.Errorf("key path contains path traversal")
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

// validateCredentials validates and sanitizes credential data
func validateCredentials(credentials map[string]interface{}) error {
    if credentials == nil {
        return fmt.Errorf("credentials cannot be nil")
    }
    
    // Validate required S3 credentials
    accessKeyID, err := safeTypeAssertion[string](credentials["access_key_id"], "access_key_id")
    if err != nil {
        return fmt.Errorf("invalid access_key_id: %v", err)
    }
    
    if len(accessKeyID) == 0 || len(accessKeyID) > 128 {
        return fmt.Errorf("invalid access_key_id length")
    }
    
    secretAccessKey, err := safeTypeAssertion[string](credentials["secret_access_key"], "secret_access_key")
    if err != nil {
        return fmt.Errorf("invalid secret_access_key: %v", err)
    }
    
    if len(secretAccessKey) == 0 || len(secretAccessKey) > 256 {
        return fmt.Errorf("invalid secret_access_key length")
    }
    
    // Validate optional region
    if regionValue, exists := credentials["region"]; exists {
        region, err := safeTypeAssertion[string](regionValue, "region")
        if err != nil {
            return fmt.Errorf("invalid region: %v", err)
        }
        
        if len(region) > 0 && !validRegionPattern.MatchString(region) {
            return fmt.Errorf("invalid region format")
        }
    }
    
    // Validate optional endpoint
    if endpointValue, exists := credentials["endpoint"]; exists {
        endpoint, err := safeTypeAssertion[string](endpointValue, "endpoint")
        if err != nil {
            return fmt.Errorf("invalid endpoint: %v", err)
        }
        
        if len(endpoint) > 0 {
            if _, err := url.Parse(endpoint); err != nil {
                return fmt.Errorf("invalid endpoint URL: %v", err)
            }
        }
    }
    
    return nil
}

// generateSecurePresignedURL generates presigned URLs with comprehensive security validation
func generateSecurePresignedURL(assetCache *assetcachepojo.AssetFolderCache, fileName string, uploadId string) (map[string]interface{}, error) {
    // Input validation
    if assetCache == nil {
        return nil, fmt.Errorf("asset cache cannot be nil")
    }
    
    if err := validateFileName(fileName); err != nil {
        return nil, fmt.Errorf("invalid file name: %v", err)
    }
    
    if uploadId != "" && !validUploadIdPattern.MatchString(uploadId) {
        return nil, fmt.Errorf("invalid upload ID format")
    }
    
    // Validate credentials
    if err := validateCredentials(assetCache.Credentials); err != nil {
        return nil, fmt.Errorf("credential validation failed: %v", err)
    }
    
    // Setup secure credentials
    configSetName := assetCache.CloudStore.Name
    if strings.Contains(assetCache.CloudStore.RootPath, ":") {
        parts := strings.Split(assetCache.CloudStore.RootPath, ":")
        if len(parts) > 0 {
            configSetName = parts[0]
        }
    }
    
    // Validate config set name
    if !validFileNamePattern.MatchString(configSetName) {
        return nil, fmt.Errorf("invalid config set name")
    }
    
    // Set credentials securely (in production, use encrypted storage)
    for key, val := range assetCache.Credentials {
        if !validFileNamePattern.MatchString(key) {
            continue // Skip invalid keys
        }
        
        valStr, err := safeTypeAssertion[string](val, key)
        if err != nil {
            continue // Skip invalid values
        }
        
        config.Data().SetValue(configSetName, key, valStr)
    }
    
    // Check if this is S3 storage
    providerType, err := safeTypeAssertion[string](assetCache.Credentials["type"], "type")
    if err == nil && providerType == "s3" {
        return generateSecureS3PresignedURL(assetCache, fileName, uploadId)
    }
    
    // For non-S3 providers, return appropriate error
    return nil, fmt.Errorf("presigned URLs not implemented for provider: %s", providerType)
}

// generateSecureS3PresignedURL generates S3 presigned URLs with security validation
func generateSecureS3PresignedURL(assetCache *assetcachepojo.AssetFolderCache, fileName string, uploadId string) (map[string]interface{}, error) {
    // Extract and validate bucket name from RootPath
    rootPath := assetCache.CloudStore.RootPath
    bucketName := ""
    
    if strings.Contains(rootPath, ":") {
        parts := strings.Split(rootPath, ":")
        if len(parts) >= 2 {
            bucketName = strings.TrimPrefix(parts[1], "/")
            if strings.Contains(bucketName, "/") {
                pathParts := strings.SplitN(bucketName, "/", 2)
                bucketName = pathParts[0]
            }
        }
    }
    
    if err := validateBucketName(bucketName); err != nil {
        return nil, fmt.Errorf("invalid bucket name: %v", err)
    }
    
    // Construct and validate key path
    keyPath := filepath.Join(assetCache.Keyname, fileName)
    keyPath = strings.ReplaceAll(keyPath, "\\", "/") // Ensure forward slashes
    
    if err := validateKeyPath(keyPath); err != nil {
        return nil, fmt.Errorf("invalid key path: %v", err)
    }
    
    // Validate credentials again for S3
    if err := validateCredentials(assetCache.Credentials); err != nil {
        return nil, fmt.Errorf("S3 credential validation failed: %v", err)
    }
    
    accessKeyID, _ := safeTypeAssertion[string](assetCache.Credentials["access_key_id"], "access_key_id")
    secretAccessKey, _ := safeTypeAssertion[string](assetCache.Credentials["secret_access_key"], "secret_access_key")
    
    region := "us-east-1" // Default
    if regionValue, exists := assetCache.Credentials["region"]; exists {
        if r, err := safeTypeAssertion[string](regionValue, "region"); err == nil && len(r) > 0 {
            region = r
        }
    }
    
    var endpoint string
    if endpointValue, exists := assetCache.Credentials["endpoint"]; exists {
        if e, err := safeTypeAssertion[string](endpointValue, "endpoint"); err == nil {
            endpoint = e
        }
    }
    
    // Create AWS config with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    cfg, err := awsconfig.LoadDefaultConfig(ctx,
        awsconfig.WithRegion(region),
        awsconfig.WithCredentialsProvider(
            awscredentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create secure AWS config")
    }
    
    // Create S3 client with security options
    s3Options := func(o *s3.Options) {
        if endpoint != "" {
            o.BaseEndpoint = aws.String(endpoint)
            o.UsePathStyle = true
        }
    }
    
    s3Client := s3.NewFromConfig(cfg, s3Options)
    presignClient := s3.NewPresignClient(s3Client)
    
    // Generate presigned URL with secure expiry
    putObjectRequest := &s3.PutObjectInput{
        Bucket: aws.String(bucketName),
        Key:    aws.String(keyPath),
    }
    
    presignedReq, err := presignClient.PresignPutObject(ctx, putObjectRequest,
        func(opts *s3.PresignOptions) {
            opts.Expires = DefaultPresignedURLExpiry
        })
    if err != nil {
        return nil, fmt.Errorf("failed to create secure presigned URL")
    }
    
    log.Infof("Generated secure S3 presigned URL for bucket: %s, key: %s", bucketName, keyPath)
    
    return map[string]interface{}{
        "upload_type":   "presigned",
        "presigned_url": presignedReq.URL,
        "method":        presignedReq.Method,
        "headers":       presignedReq.SignedHeader,
        "expires_at":    time.Now().Add(DefaultPresignedURLExpiry).Unix(),
    }, nil
}

// Additional secure functions would follow similar patterns for:
// - InitiateSecureS3MultipartUpload
// - GetSecureS3PartPresignedURL  
// - CompleteSecureS3MultipartUpload
// - AbortSecureS3MultipartUpload
// Each with comprehensive input validation, error handling, and security controls

// generatePresignedURL maintains backward compatibility
func generatePresignedURL(assetCache *assetcachepojo.AssetFolderCache, fileName string, uploadId string) (map[string]interface{}, error) {
    result, err := generateSecurePresignedURL(assetCache, fileName, uploadId)
    if err != nil {
        log.Errorf("Secure presigned URL generation failed: %v", err)
        return nil, fmt.Errorf("presigned URL generation failed")
    }
    return result, nil
}
```

### Long-term Improvements
1. **Credential Encryption:** Implement proper encryption for credential storage and handling
2. **Access Control Integration:** Add proper authorization and user permission checks
3. **Audit Logging:** Log all presigned URL generation and usage
4. **Rate Limiting:** Implement rate limiting for URL generation
5. **Monitoring Integration:** Add comprehensive metrics and alerting

## Edge Cases Identified

1. **Malformed Credentials:** Invalid or corrupted credential data
2. **Network Timeouts:** Cloud API timeouts during URL generation
3. **Large Multipart Uploads:** Handling uploads with thousands of parts
4. **Invalid Cloud Paths:** Malformed or dangerous cloud storage paths
5. **Credential Rotation:** Handling of rotated or expired credentials
6. **Concurrent Uploads:** Multiple simultaneous multipart uploads
7. **Memory Pressure:** Large credential or part data under memory constraints
8. **Service Outages:** Cloud storage service unavailability

## Security Best Practices Violations

1. **Raw credential access** without encryption or secure handling
2. **Unsafe type assertions** throughout the codebase without validation
3. **Path injection vulnerabilities** from user-controlled path construction
4. **Missing input validation** for critical function parameters
5. **Information disclosure** through detailed error messages
6. **Hardcoded configuration** values without security validation
7. **Missing authorization checks** for presigned URL generation
8. **Insufficient memory management** for large data structures

## Positive Security Aspects

1. **AWS SDK integration** providing established security patterns
2. **Context usage** for timeout management
3. **Error handling** for operational failures
4. **Multipart upload support** for large file handling

## Critical Issues Summary

1. **Credential Information Exposure:** Raw cloud credentials accessed without encryption
2. **Type Assertion Vulnerabilities:** Multiple unsafe type assertions can panic application
3. **Path Injection Vulnerability:** User input used in path construction without validation
4. **Insufficient Input Validation:** Function parameters not validated for security
5. **Error Information Disclosure:** Detailed errors expose system configuration
6. **Hardcoded Configuration Values:** Fixed settings without security controls
7. **Insufficient Access Controls:** URLs generated without authorization checks
8. **Memory Management Issues:** Large data processed without limits
9. **Incomplete Implementation Security:** Placeholder functions may become attack vectors

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - Cloud storage integration with severe credential exposure and multiple injection vulnerabilities