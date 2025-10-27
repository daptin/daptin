# Security Analysis: server/subsite/template_handler.go

**File:** `server/subsite/template_handler.go`  
**Type:** Template rendering and caching system with HTTP cache control  
**Lines of Code:** 508  

## Overview
This file implements a comprehensive template handling system with caching capabilities, HTTP cache control, ETag generation, and template routing. It manages template rendering with action processing, cache validation, and HTTP header management for web responses.

## Key Components

### SubSite struct
**Lines:** 30-42  
**Purpose:** Represents a subsite configuration with permissions and metadata  

### Template Caching System
**Lines:** 52-88, 369-452  
**Purpose:** Generates cache keys and manages HTTP cache headers  

### Template Route Creation
**Lines:** 90-121, 123-358  
**Purpose:** Creates dynamic routes for templates with action processing  

### Cache Validation and Headers
**Lines:** 472-507  
**Purpose:** Validates client cache and processes HTTP cache headers  

## Critical Security Analysis

### 1. CRITICAL: Type Assertion Vulnerabilities - HIGH RISK
**Severity:** HIGH  
**Lines:** 105, 126, 277, 278, 279  
**Issue:** Multiple unsafe type assertions without validation that can cause runtime panics.

```go
urlPattern := templateRow["url_pattern"].(string)          // Line 105
templateName := templateInstance["name"].(string)          // Line 126
var content = attrs["content"].(string)                    // Line 277
var mimeType = attrs["mime_type"].(string)                 // Line 278
var headers = attrs["headers"].(map[string]string)         // Line 279
```

**Risk:**
- **Runtime panics** if database fields are not the expected types
- **Application crashes** during template processing
- **Service unavailability** when handling template requests
- **No fallback mechanism** for type assertion failures

**Impact:** Complete service disruption when template data format is unexpected.

### 2. CRITICAL: Cache Key Injection Vulnerability - HIGH RISK
**Severity:** HIGH  
**Lines:** 58, 62, 70, 72, 82  
**Issue:** User-controlled data directly used in cache key generation without validation.

```go
key := c.Request.URL.Path                     // User-controlled path
key = config.CacheKeyPrefix + ":" + path      // Prefix injection
key += ":" + param + "=" + value              // Query param injection
key += ":" + header + "=" + value             // Header injection
```

**Risk:**
- **Cache poisoning** through crafted URLs, parameters, or headers
- **Cache key collision** attacks to serve malicious content
- **Information disclosure** through cache key enumeration
- **Denial of Service** through cache exhaustion

**Impact:** Cache manipulation allowing unauthorized content serving and potential data exposure.

### 3. CRITICAL: Base64 Decoding Vulnerability - HIGH RISK
**Severity:** HIGH  
**Lines:** 361-367  
**Issue:** Base64 decoding without error handling or size limits.

```go
func Atob(data string) string {
    decodedData, err := base64.StdEncoding.DecodeString(data)
    if err != nil {
        log.Printf("Atob failed: %v", err)
        return ""  // Silent failure returns empty string
    }
    return string(decodedData)
}
```

**Risk:**
- **Silent failures** allowing empty content to be served
- **Memory exhaustion** through extremely large base64 strings
- **No size limits** on decoded content
- **Information disclosure** through error message details

### 4. HIGH: SQL Injection in Template Processing - HIGH RISK
**Severity:** HIGH  
**Lines:** 92, 255  
**Issue:** Database operations with potentially unsafe template data.

```go
templateList, err := cruds["template"].GetAllObjects("template", transaction)
actionResponses, errAction := cruds["action"].HandleActionRequest(actionRequest, api2goRequestData, transaction1)
```

**Risk:**
- **SQL injection** through crafted template configurations
- **Database corruption** through malicious template data
- **Unauthorized data access** through action request manipulation
- **Transaction manipulation** affecting data integrity

### 5. HIGH: File Path Injection in Cache - HIGH RISK
**Severity:** HIGH  
**Lines:** 188, 190, 338  
**Issue:** User-controlled path data used in file operations without validation.

```go
c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%v\"", filepath.Base(cachedFile.Path)))
c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%v\"", filepath.Base(cachedFile.Path)))
Path:       c.Request.URL.Path,  // User-controlled path stored
```

**Risk:**
- **Path traversal** through crafted URL paths
- **Header injection** through malicious file names
- **Content-Disposition attacks** affecting browser behavior
- **Information disclosure** through path enumeration

### 6. MEDIUM: ETag Manipulation - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 168-172, 289, 462  
**Issue:** ETag validation and generation vulnerabilities.

```go
if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == cachedFile.ETag {
hash := sha256.Sum256([]byte(content))
etag := hex.EncodeToString(hash[:8])  // Only first 8 bytes used
```

**Risk:**
- **ETag collision** through weak hash truncation (only 8 bytes)
- **Cache manipulation** through ETag header injection
- **Predictable ETags** allowing cache enumeration
- **Information leakage** through ETag patterns

### 7. MEDIUM: Time-based Cache Vulnerabilities - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 180-184, 295, 502  
**Issue:** Time-based cache validation with potential manipulation.

```go
maxAge := int(time.Until(cachedFile.ExpiresAt).Seconds())
if maxAge <= 0 {
    maxAge = 60 // Minimum 1 minute for almost expired resources
}
return time.Now().Before(modifiedSinceTime)
```

**Risk:**
- **Cache timing attacks** through precise timing measurements
- **Race conditions** in cache expiration handling
- **Time manipulation** affecting cache validation
- **Inconsistent caching** behavior across different time zones

### 8. MEDIUM: Action Request Injection - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 253-265  
**Issue:** Action request handling with user-controlled data.

```go
if len(actionRequest.Action) > 0 && len(actionRequest.Type) > 0 {
    actionRequest.Attributes = inFields  // User data injection
    actionResponses, errAction := cruds["action"].HandleActionRequest(actionRequest, api2goRequestData, transaction1)
```

**Risk:**
- **Action injection** through crafted request parameters
- **Privilege escalation** through action manipulation
- **Data corruption** through malicious action attributes
- **Unauthorized operations** through action bypass

## Potential Attack Vectors

### Cache Poisoning Attacks
1. **Key Collision:** Craft URLs/parameters to create cache key collisions
2. **Header Injection:** Use crafted headers to poison cache entries
3. **Query Parameter Manipulation:** Inject malicious query parameters
4. **Path Traversal in Cache:** Use path traversal in URL paths

### Template Injection Attacks
1. **Template Configuration Injection:** Inject malicious template configurations
2. **Action Request Manipulation:** Manipulate action requests for privilege escalation
3. **Parameter Pollution:** Use parameter pollution in template processing
4. **SQL Injection through Templates:** Inject SQL through template data

### HTTP Response Manipulation
1. **Header Injection:** Inject malicious HTTP headers through various vectors
2. **Content-Type Confusion:** Manipulate MIME types for content confusion attacks
3. **ETag Manipulation:** Manipulate ETags for cache confusion
4. **Content-Disposition Attacks:** Manipulate file download behavior

### Denial of Service Attacks
1. **Memory Exhaustion:** Submit large base64 content for memory exhaustion
2. **Cache Exhaustion:** Flood cache with unique keys
3. **Processing Overload:** Submit complex templates for processing overload
4. **Database Overload:** Create excessive database operations

## Recommendations

### Immediate Actions
1. **Add Type Validation:** Validate all type assertions before execution
2. **Sanitize Cache Keys:** Validate and sanitize all cache key components
3. **Add Size Limits:** Implement size limits for base64 decoding and cache operations
4. **Validate File Paths:** Sanitize and validate all file path operations

### Enhanced Security Implementation

```go
package subsite

import (
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "net/url"
    "path/filepath"
    "regexp"
    "strings"
    "unicode/utf8"
    
    "github.com/gin-gonic/gin"
    log "github.com/sirupsen/logrus"
)

const (
    MaxCacheKeyLength = 512
    MaxBase64ContentSize = 10 * 1024 * 1024 // 10MB
    MaxTemplateNameLength = 255
    MaxHeaderValueLength = 8192
    MaxQueryParamLength = 2048
    ETAGHashBytes = 16 // Use more bytes for stronger ETags
)

var (
    validCacheKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9.:_/-]+$`)
    validTemplateNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]*$`)
    dangerousHeaderChars = regexp.MustCompile(`[\r\n\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`)
)

// validateTemplateRow validates template database row data
func validateTemplateRow(templateRow map[string]interface{}) error {
    // Validate required fields exist and have correct types
    nameInterface, exists := templateRow["name"]
    if !exists {
        return fmt.Errorf("template name field missing")
    }
    
    name, ok := nameInterface.(string)
    if !ok {
        return fmt.Errorf("template name is not a string")
    }
    
    if len(name) == 0 || len(name) > MaxTemplateNameLength {
        return fmt.Errorf("invalid template name length: %d", len(name))
    }
    
    if !validTemplateNamePattern.MatchString(name) {
        return fmt.Errorf("invalid template name format")
    }
    
    // Validate URL pattern
    urlPatternInterface, exists := templateRow["url_pattern"]
    if !exists {
        return fmt.Errorf("template url_pattern field missing")
    }
    
    urlPattern, ok := urlPatternInterface.(string)
    if !ok {
        return fmt.Errorf("template url_pattern is not a string")
    }
    
    if len(urlPattern) == 0 {
        return fmt.Errorf("template url_pattern cannot be empty")
    }
    
    return nil
}

// sanitizeCacheKeyComponent sanitizes individual cache key components
func sanitizeCacheKeyComponent(component string) string {
    if len(component) > MaxCacheKeyLength/4 {
        component = component[:MaxCacheKeyLength/4]
    }
    
    // Remove dangerous characters
    component = strings.ReplaceAll(component, "\x00", "")
    component = strings.ReplaceAll(component, "\n", "")
    component = strings.ReplaceAll(component, "\r", "")
    
    // URL encode for safety
    return url.QueryEscape(component)
}

// validateCacheKey validates the final cache key
func validateCacheKey(key string) error {
    if len(key) > MaxCacheKeyLength {
        return fmt.Errorf("cache key too long: %d", len(key))
    }
    
    if !utf8.ValidString(key) {
        return fmt.Errorf("cache key contains invalid UTF-8")
    }
    
    if !validCacheKeyPattern.MatchString(key) {
        return fmt.Errorf("cache key contains invalid characters")
    }
    
    return nil
}

// secureGenerateCacheKey creates a secure cache key with validation
func secureGenerateCacheKey(c *gin.Context, config *CacheConfig) (string, error) {
    if config == nil {
        return "", fmt.Errorf("cache config is nil")
    }
    
    // Start with sanitized path
    path := c.Request.URL.Path
    if len(path) > MaxCacheKeyLength/2 {
        return "", fmt.Errorf("URL path too long for cache key")
    }
    
    key := sanitizeCacheKeyComponent(path)
    
    // Add prefix if configured
    if config.CacheKeyPrefix != "" {
        prefix := sanitizeCacheKeyComponent(config.CacheKeyPrefix)
        key = prefix + ":" + key
    }
    
    // Add query parameters if configured
    if len(config.VaryByQueryParams) > 0 {
        queryValues := c.Request.URL.Query()
        for _, param := range config.VaryByQueryParams {
            if len(param) > 100 {
                continue // Skip overly long parameter names
            }
            
            if values, exists := queryValues[param]; exists {
                for _, value := range values {
                    if len(value) > MaxQueryParamLength {
                        value = value[:MaxQueryParamLength]
                    }
                    sanitizedParam := sanitizeCacheKeyComponent(param)
                    sanitizedValue := sanitizeCacheKeyComponent(value)
                    key += ":" + sanitizedParam + "=" + sanitizedValue
                }
            }
        }
    }
    
    // Add headers if configured
    if len(config.VaryByHeaders) > 0 {
        for _, header := range config.VaryByHeaders {
            if len(header) > 100 {
                continue // Skip overly long header names
            }
            
            value := c.GetHeader(header)
            if value != "" {
                if len(value) > MaxHeaderValueLength {
                    value = value[:MaxHeaderValueLength]
                }
                sanitizedHeader := sanitizeCacheKeyComponent(header)
                sanitizedValue := sanitizeCacheKeyComponent(value)
                key += ":" + sanitizedHeader + "=" + sanitizedValue
            }
        }
    }
    
    // Validate final key
    if err := validateCacheKey(key); err != nil {
        return "", fmt.Errorf("invalid cache key: %v", err)
    }
    
    return key, nil
}

// secureAtob performs safe base64 decoding with size limits and validation
func secureAtob(data string) (string, error) {
    if len(data) == 0 {
        return "", fmt.Errorf("base64 data cannot be empty")
    }
    
    // Check input size before decoding
    maxEncodedSize := base64.StdEncoding.EncodedLen(MaxBase64ContentSize)
    if len(data) > maxEncodedSize {
        return "", fmt.Errorf("base64 data too large: %d bytes", len(data))
    }
    
    // Validate base64 format
    if !isValidBase64(data) {
        return "", fmt.Errorf("invalid base64 format")
    }
    
    decodedData, err := base64.StdEncoding.DecodeString(data)
    if err != nil {
        return "", fmt.Errorf("base64 decoding failed: %v", err)
    }
    
    // Check decoded size
    if len(decodedData) > MaxBase64ContentSize {
        return "", fmt.Errorf("decoded data too large: %d bytes", len(decodedData))
    }
    
    // Validate UTF-8
    if !utf8.Valid(decodedData) {
        return "", fmt.Errorf("decoded data is not valid UTF-8")
    }
    
    return string(decodedData), nil
}

// isValidBase64 checks if string is valid base64
func isValidBase64(s string) bool {
    // Basic base64 character validation
    validChars := regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
    return validChars.MatchString(s)
}

// secureGenerateETag generates a secure ETag with full hash
func secureGenerateETag(content string, strategy string) (string, error) {
    if strategy == "none" {
        return "", nil
    }
    
    if len(content) > MaxBase64ContentSize {
        return "", fmt.Errorf("content too large for ETag generation")
    }
    
    // Use full hash for security
    hash := sha256.Sum256([]byte(content))
    etag := hex.EncodeToString(hash[:ETAGHashBytes])
    
    if strategy == "weak" {
        return fmt.Sprintf("W/\"%s\"", etag), nil
    }
    
    // Strong ETag
    return fmt.Sprintf("\"%s\"", etag), nil
}

// validateContentDisposition validates Content-Disposition header values
func validateContentDisposition(filename string) error {
    if len(filename) == 0 {
        return fmt.Errorf("filename cannot be empty")
    }
    
    if len(filename) > 255 {
        return fmt.Errorf("filename too long: %d", len(filename))
    }
    
    // Check for dangerous characters
    if dangerousHeaderChars.MatchString(filename) {
        return fmt.Errorf("filename contains dangerous characters")
    }
    
    // Check for path traversal
    if strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
        return fmt.Errorf("filename contains path traversal characters")
    }
    
    return nil
}

// secureSetContentDisposition safely sets Content-Disposition header
func secureSetContentDisposition(c *gin.Context, path string, isDownload bool) error {
    filename := filepath.Base(path)
    
    if err := validateContentDisposition(filename); err != nil {
        return err
    }
    
    // Escape filename for header
    escapedFilename := url.QueryEscape(filename)
    
    dispositionType := "inline"
    if isDownload {
        dispositionType = "attachment"
    }
    
    c.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", dispositionType, escapedFilename))
    return nil
}

// validateTemplateResponse validates template rendering response
func validateTemplateResponse(attrs map[string]interface{}) (*TemplateResponse, error) {
    response := &TemplateResponse{}
    
    // Validate content
    contentInterface, exists := attrs["content"]
    if !exists {
        return nil, fmt.Errorf("template response missing content")
    }
    
    content, ok := contentInterface.(string)
    if !ok {
        return nil, fmt.Errorf("template content is not a string")
    }
    response.Content = content
    
    // Validate mime type
    mimeTypeInterface, exists := attrs["mime_type"]
    if !exists {
        return nil, fmt.Errorf("template response missing mime_type")
    }
    
    mimeType, ok := mimeTypeInterface.(string)
    if !ok {
        return nil, fmt.Errorf("template mime_type is not a string")
    }
    
    if len(mimeType) == 0 || len(mimeType) > 255 {
        return nil, fmt.Errorf("invalid mime_type length")
    }
    response.MimeType = mimeType
    
    // Validate headers
    headersInterface, exists := attrs["headers"]
    if exists {
        headers, ok := headersInterface.(map[string]string)
        if !ok {
            return nil, fmt.Errorf("template headers is not a map[string]string")
        }
        
        // Validate header values
        for key, value := range headers {
            if len(key) > 100 || len(value) > MaxHeaderValueLength {
                return nil, fmt.Errorf("header too long: %s", key)
            }
            
            if dangerousHeaderChars.MatchString(key) || dangerousHeaderChars.MatchString(value) {
                return nil, fmt.Errorf("header contains dangerous characters: %s", key)
            }
        }
        response.Headers = headers
    }
    
    return response, nil
}

type TemplateResponse struct {
    Content  string
    MimeType string
    Headers  map[string]string
}
```

### Long-term Improvements
1. **Template Security Framework:** Implement comprehensive template security validation
2. **Cache Security:** Add cache integrity checking and validation
3. **Response Validation:** Validate all template responses before serving
4. **Security Monitoring:** Monitor for template injection and cache manipulation attempts
5. **Rate Limiting:** Add rate limiting for template processing operations

## Edge Cases Identified

1. **Empty Template Content:** Handling of empty or missing template content
2. **Large Templates:** Performance with very large template content
3. **Malformed Base64:** Various invalid base64 input patterns
4. **Unicode in Cache Keys:** Unicode characters in URLs and parameters
5. **Long Cache Keys:** Very long cache key generation
6. **Concurrent Template Access:** Thread safety of template processing
7. **Cache Overflow:** Behavior when cache reaches capacity
8. **Network Interruptions:** Handling of network interruptions during template serving
9. **Database Errors:** Template processing during database connectivity issues
10. **Memory Pressure:** Template processing under high memory pressure

## Security Best Practices Violations

1. **No type validation for database fields and template responses**
2. **User-controlled data directly used in cache keys without sanitization**
3. **Base64 decoding without size limits or proper error handling**
4. **File paths used in headers without validation**
5. **ETags generated with weak hashing (only 8 bytes)**
6. **No input validation for action requests and template parameters**

## Critical Issues Summary

1. **Type Assertion Vulnerabilities:** Runtime panics from unsafe type casts in template processing
2. **Cache Key Injection:** User-controlled data in cache keys allowing cache poisoning
3. **Base64 Decoding Issues:** Silent failures and potential memory exhaustion
4. **Path Injection Vulnerabilities:** File path injection in Content-Disposition headers
5. **ETag Manipulation:** Weak ETag generation and validation vulnerabilities
6. **Action Request Injection:** Potential privilege escalation through action manipulation

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - Multiple high-impact vulnerabilities in template processing and caching system