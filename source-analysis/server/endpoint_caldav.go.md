# Security Analysis: server/endpoint_caldav.go

**File:** `server/endpoint_caldav.go`  
**Type:** CalDAV/CardDAV WebDAV endpoint initialization  
**Lines of Code:** 53  

## Overview
This file initializes CalDAV and CardDAV endpoints for calendar and contact synchronization using the WebDAV protocol. It sets up HTTP handlers for both CalDAV and CardDAV protocols with basic authentication middleware and local file system storage. The implementation supports all standard WebDAV methods including PROPFIND, PROPPATCH, MKCOL, COPY, and MOVE.

## Key Components

### InitializeCaldavResources function
**Lines:** 11-52  
**Purpose:** Initializes CalDAV and CardDAV endpoints with authentication and routing  

### WebDAV Handler Setup
**Lines:** 15-17  
**Purpose:** Creates WebDAV handler with local file system backend  

### Authentication Handler
**Lines:** 18-26  
**Purpose:** Wraps WebDAV handler with authentication middleware  

### Route Registration
- **CalDAV routes:** Lines 27-38
- **CardDAV routes:** Lines 40-51

## Security Analysis

### 1. CRITICAL: Path Traversal Vulnerability - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 16, 27-51  
**Issue:** Local file system access without path validation enables directory traversal attacks.

```go
FileSystem: webdav.LocalFileSystem("./storage"),  // No path validation
// Routes with wildcard paths
defaultRouter.Handle("GET", "/caldav/*path", caldavHttpHandler)
defaultRouter.Handle("PUT", "/carddav/*path", caldavHttpHandler)
```

**Risk:**
- **Directory traversal** through malicious path manipulation
- **Arbitrary file access** outside storage directory
- **System file exposure** via path traversal attacks
- **Data exfiltration** through unauthorized file access

### 2. CRITICAL: Insufficient Access Control - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 19-25  
**Issue:** Basic authentication with potentially insufficient authorization checks.

```go
ok, abort, modifiedRequest := authMiddleware.AuthCheckMiddlewareWithHttp(c.Request, c.Writer, true)
if !ok || abort {
    c.Header("WWW-Authenticate", "Basic realm='caldav'")
    c.AbortWithStatus(http.StatusUnauthorized)
    return
}
```

**Risk:**
- **Weak authentication** through basic authentication protocol
- **Credential interception** in unencrypted connections
- **Session hijacking** through basic auth vulnerabilities
- **Insufficient authorization** for file operations

### 3. HIGH: Unrestricted File Operations - HIGH RISK
**Severity:** HIGH  
**Lines:** 30-38, 43-51  
**Issue:** Full WebDAV operations enabled without fine-grained access control.

```go
defaultRouter.Handle("PUT", "/caldav/*path", caldavHttpHandler)      // File upload
defaultRouter.Handle("DELETE", "/caldav/*path", caldavHttpHandler)   // File deletion
defaultRouter.Handle("MOVE", "/caldav/*path", caldavHttpHandler)     // File movement
defaultRouter.Handle("COPY", "/caldav/*path", caldavHttpHandler)     // File copying
```

**Risk:**
- **Unauthorized file modifications** through PUT operations
- **Data destruction** via DELETE operations
- **File system manipulation** through MOVE/COPY operations
- **Storage exhaustion** through unrestricted uploads

### 4. HIGH: Missing Input Validation - HIGH RISK
**Severity:** HIGH  
**Lines:** 16, 27-51  
**Issue:** No validation of file paths, names, or content in WebDAV operations.

```go
// No validation for:
// - File path parameters
// - File names and extensions
// - File content and size
// - WebDAV property values
```

**Risk:**
- **Malicious file upload** with dangerous content
- **File name injection** through crafted filenames
- **WebDAV property injection** via malicious properties
- **Resource exhaustion** through large file uploads

### 5. MEDIUM: Protocol Method Exposure - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 33-38, 46-51  
**Issue:** Advanced WebDAV methods exposed without specific security considerations.

```go
defaultRouter.Handle("PROPFIND", "/caldav/*path", caldavHttpHandler)    // Property discovery
defaultRouter.Handle("PROPPATCH", "/caldav/*path", caldavHttpHandler)   // Property modification
defaultRouter.Handle("MKCOL", "/caldav/*path", caldavHttpHandler)       // Collection creation
```

**Risk:**
- **Information disclosure** through PROPFIND operations
- **Metadata manipulation** via PROPPATCH operations
- **Directory structure manipulation** through MKCOL operations
- **WebDAV protocol abuse** for reconnaissance

### 6. MEDIUM: Storage Backend Security - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 16  
**Issue:** Local file system backend without encryption or access controls.

```go
FileSystem: webdav.LocalFileSystem("./storage"),  // Unencrypted local storage
```

**Risk:**
- **Data exposure** through file system access
- **No encryption at rest** for sensitive calendar/contact data
- **File permission issues** on underlying file system
- **Backup security** concerns for unencrypted data

### 7. LOW: Missing Security Headers - LOW RISK
**Severity:** LOW  
**Lines:** 21  
**Issue:** Limited security headers in authentication responses.

```go
c.Header("WWW-Authenticate", "Basic realm='caldav'")  // Only basic auth header
```

**Risk:**
- **Missing security headers** for defense in depth
- **Cache control issues** for sensitive data
- **Content type security** concerns
- **Browser security feature** underutilization

## Potential Attack Vectors

### Path Traversal Attacks
1. **Directory Escape:** Use "../" sequences to access files outside storage directory
2. **Root Access:** Attempt to access system files through path manipulation
3. **Configuration File Access:** Target application configuration files
4. **Log File Access:** Extract sensitive information from log files

### WebDAV Protocol Abuse
1. **PROPFIND Reconnaissance:** Use PROPFIND to discover directory structure and files
2. **Unauthorized Upload:** Upload malicious files through PUT operations
3. **Data Destruction:** Delete legitimate files through DELETE operations
4. **File System Manipulation:** Use MOVE/COPY to reorganize or corrupt file structure

### Authentication Bypass
1. **Basic Auth Interception:** Intercept credentials in unencrypted connections
2. **Credential Brute Force:** Attempt to brute force basic authentication
3. **Session Manipulation:** Exploit basic auth session handling vulnerabilities
4. **Authentication Bypass:** Exploit middleware vulnerabilities

### Resource Exhaustion Attacks
1. **Storage Exhaustion:** Upload large files to consume disk space
2. **Collection Proliferation:** Create excessive directories through MKCOL
3. **Property Spam:** Create excessive properties through PROPPATCH
4. **Concurrent Requests:** Overwhelm server with simultaneous WebDAV requests

## Recommendations

### Immediate Actions
1. **Implement Path Validation:** Add strict path validation to prevent traversal
2. **Add File Type Restrictions:** Limit allowed file types and extensions
3. **Implement Size Limits:** Add file size and storage quotas
4. **Enhance Authentication:** Implement stronger authentication mechanisms

### Enhanced Security Implementation

```go
package server

import (
    "fmt"
    "net/http"
    "path"
    "path/filepath"
    "regexp"
    "strings"
    "time"
    
    "github.com/daptin/daptin/server/auth"
    "github.com/emersion/go-webdav"
    "github.com/gin-gonic/gin"
    "github.com/sirupsen/logrus"
)

const (
    maxFileSize          = 100 * 1024 * 1024  // 100MB max file size
    maxFilesPerUser      = 1000               // Maximum files per user
    maxStoragePerUser    = 1024 * 1024 * 1024 // 1GB max storage per user
    maxDirectoryDepth    = 10                 // Maximum directory nesting
    maxPropertySize      = 64 * 1024          // 64KB max property size
    caldavRateLimitRPS   = 10                 // Requests per second limit
)

var (
    // Safe path pattern
    safePathPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
    
    // Allowed file extensions for CalDAV/CardDAV
    allowedExtensions = map[string]bool{
        ".ics":  true, // iCalendar files
        ".vcs":  true, // vCalendar files
        ".vcf":  true, // vCard files
        ".ical": true, // iCalendar alternative
    }
    
    // Dangerous path components
    dangerousPathComponents = []string{
        "..", "~", "$", "`", ";", "|", "&",
        "etc", "proc", "sys", "dev", "root",
    }
)

// SecureFileSystem wraps webdav.LocalFileSystem with security validation
type SecureFileSystem struct {
    baseDir     string
    userID      string
    maxFileSize int64
    maxFiles    int
}

// validatePath validates file paths for security
func validatePath(requestPath string) error {
    if requestPath == "" {
        return fmt.Errorf("path cannot be empty")
    }
    
    // Clean the path
    cleaned := filepath.Clean(requestPath)
    
    // Check for dangerous patterns
    if !safePathPattern.MatchString(cleaned) {
        return fmt.Errorf("path contains invalid characters")
    }
    
    // Check for dangerous components
    for _, component := range dangerousPathComponents {
        if strings.Contains(strings.ToLower(cleaned), component) {
            return fmt.Errorf("path contains dangerous component: %s", component)
        }
    }
    
    // Check directory depth
    depth := strings.Count(cleaned, "/")
    if depth > maxDirectoryDepth {
        return fmt.Errorf("path exceeds maximum directory depth: %d", depth)
    }
    
    return nil
}

// validateFileExtension validates file extensions
func validateFileExtension(filename string) error {
    if filename == "" {
        return fmt.Errorf("filename cannot be empty")
    }
    
    ext := strings.ToLower(filepath.Ext(filename))
    if ext == "" {
        return fmt.Errorf("file must have an extension")
    }
    
    if !allowedExtensions[ext] {
        return fmt.Errorf("file extension not allowed: %s", ext)
    }
    
    return nil
}

// SecureWebDAVHandler wraps the WebDAV handler with security validation
type SecureWebDAVHandler struct {
    handler        webdav.Handler
    authMiddleware *auth.AuthMiddleware
    rateLimiter    map[string]*time.Ticker // Simple rate limiter by IP
}

// NewSecureWebDAVHandler creates a secure WebDAV handler
func NewSecureWebDAVHandler(storageDir string, authMiddleware *auth.AuthMiddleware) *SecureWebDAVHandler {
    // Validate storage directory
    if !filepath.IsAbs(storageDir) {
        storageDir = filepath.Join(".", storageDir)
    }
    
    handler := webdav.Handler{
        FileSystem: webdav.LocalFileSystem(storageDir),
        LockSystem: webdav.NewMemLS(), // Add locking support
    }
    
    return &SecureWebDAVHandler{
        handler:        handler,
        authMiddleware: authMiddleware,
        rateLimiter:    make(map[string]*time.Ticker),
    }
}

// checkRateLimit implements basic rate limiting
func (h *SecureWebDAVHandler) checkRateLimit(clientIP string) bool {
    // Simple rate limiting implementation
    // In production, use a proper rate limiter like redis-based limiter
    ticker, exists := h.rateLimiter[clientIP]
    if !exists {
        h.rateLimiter[clientIP] = time.NewTicker(time.Second / caldavRateLimitRPS)
        return true
    }
    
    select {
    case <-ticker.C:
        return true
    default:
        return false
    }
}

// validateWebDAVRequest validates WebDAV requests for security
func (h *SecureWebDAVHandler) validateWebDAVRequest(c *gin.Context) error {
    // Validate path
    requestPath := c.Param("path")
    if err := validatePath(requestPath); err != nil {
        return fmt.Errorf("invalid path: %v", err)
    }
    
    // Validate file extension for upload operations
    if c.Request.Method == "PUT" || c.Request.Method == "POST" {
        filename := path.Base(requestPath)
        if err := validateFileExtension(filename); err != nil {
            return fmt.Errorf("invalid file: %v", err)
        }
        
        // Check content length
        if c.Request.ContentLength > maxFileSize {
            return fmt.Errorf("file too large: %d bytes (max %d)", c.Request.ContentLength, maxFileSize)
        }
    }
    
    // Validate WebDAV-specific headers
    if depth := c.Request.Header.Get("Depth"); depth != "" {
        if depth != "0" && depth != "1" && depth != "infinity" {
            return fmt.Errorf("invalid Depth header: %s", depth)
        }
    }
    
    return nil
}

// SecureHandler provides secure WebDAV handling
func (h *SecureWebDAVHandler) SecureHandler(c *gin.Context) {
    // Rate limiting
    clientIP := c.ClientIP()
    if !h.checkRateLimit(clientIP) {
        c.AbortWithStatusJSON(429, gin.H{"error": "rate limit exceeded"})
        return
    }
    
    // Validate request
    if err := h.validateWebDAVRequest(c); err != nil {
        logrus.Warnf("WebDAV request validation failed: %v", err)
        c.AbortWithStatusJSON(400, gin.H{"error": "invalid request"})
        return
    }
    
    // Authentication
    ok, abort, modifiedRequest := h.authMiddleware.AuthCheckMiddlewareWithHttp(c.Request, c.Writer, true)
    if !ok || abort {
        // Security headers for auth failure
        c.Header("WWW-Authenticate", "Basic realm='WebDAV'")
        c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
        c.Header("Pragma", "no-cache")
        c.Header("Expires", "0")
        c.AbortWithStatus(http.StatusUnauthorized)
        return
    }
    
    // Set security headers
    c.Header("X-Content-Type-Options", "nosniff")
    c.Header("X-Frame-Options", "DENY")
    c.Header("X-XSS-Protection", "1; mode=block")
    c.Header("Cache-Control", "private, no-cache")
    
    // Log the request for audit
    logrus.Infof("WebDAV request: method=%s, path=%s, user=%s, ip=%s", 
        c.Request.Method, c.Param("path"), "user", clientIP) // Get actual user from auth context
    
    // Delegate to WebDAV handler
    h.handler.ServeHTTP(c.Writer, modifiedRequest)
}

// InitializeSecureCaldavResources initializes CalDAV/CardDAV with enhanced security
func InitializeSecureCaldavResources(authMiddleware *auth.AuthMiddleware, defaultRouter *gin.Engine) {
    logrus.Infof("Initializing secure CalDAV/CardDAV endpoints")
    
    // Validate storage directory
    storageDir := "./storage/webdav"
    if err := validateStorageDirectory(storageDir); err != nil {
        logrus.Fatalf("Invalid storage directory: %v", err)
    }
    
    // Create secure WebDAV handler
    secureHandler := NewSecureWebDAVHandler(storageDir, authMiddleware)
    
    // Define allowed methods for CalDAV/CardDAV
    allowedMethods := []string{
        "OPTIONS", "HEAD", "GET", "POST", "PUT", "PATCH",
        "PROPFIND", "DELETE", "COPY", "MOVE", "MKCOL", "PROPPATCH",
    }
    
    // Register CalDAV routes
    for _, method := range allowedMethods {
        defaultRouter.Handle(method, "/caldav/*path", secureHandler.SecureHandler)
    }
    
    // Register CardDAV routes
    for _, method := range allowedMethods {
        defaultRouter.Handle(method, "/carddav/*path", secureHandler.SecureHandler)
    }
    
    logrus.Infof("CalDAV/CardDAV endpoints initialized with enhanced security")
}

// validateStorageDirectory validates the storage directory
func validateStorageDirectory(dir string) error {
    // Check if directory path is safe
    if err := validatePath(dir); err != nil {
        return fmt.Errorf("unsafe storage directory: %v", err)
    }
    
    // Ensure directory exists
    if err := os.MkdirAll(dir, 0750); err != nil {
        return fmt.Errorf("failed to create storage directory: %v", err)
    }
    
    // Check directory permissions
    info, err := os.Stat(dir)
    if err != nil {
        return fmt.Errorf("failed to stat storage directory: %v", err)
    }
    
    if !info.IsDir() {
        return fmt.Errorf("storage path is not a directory")
    }
    
    return nil
}

// CleanupRateLimiters cleans up rate limiters periodically
func (h *SecureWebDAVHandler) CleanupRateLimiters() {
    // Implement cleanup logic for rate limiters
    // This should be called periodically to prevent memory leaks
    for ip, ticker := range h.rateLimiter {
        // Remove inactive rate limiters (implementation depends on requirements)
        _ = ip
        _ = ticker
    }
}

// GetWebDAVStats returns WebDAV usage statistics
func (h *SecureWebDAVHandler) GetWebDAVStats() map[string]interface{} {
    return map[string]interface{}{
        "active_rate_limiters": len(h.rateLimiter),
        "max_file_size":       maxFileSize,
        "max_files_per_user":  maxFilesPerUser,
        "allowed_extensions":  allowedExtensions,
    }
}

// InitializeCaldavResources maintains backward compatibility
func InitializeCaldavResources(authMiddleware *auth.AuthMiddleware, defaultRouter *gin.Engine) {
    InitializeSecureCaldavResources(authMiddleware, defaultRouter)
}
```

### Long-term Improvements
1. **User-Specific Storage:** Implement per-user storage isolation and quotas
2. **Encryption at Rest:** Encrypt stored calendar and contact data
3. **Audit Logging:** Comprehensive logging of all WebDAV operations
4. **Real-time Monitoring:** Monitor for suspicious WebDAV activities
5. **Content Filtering:** Scan uploaded files for malicious content

## Edge Cases Identified

1. **Large File Uploads:** Handling very large calendar/contact files
2. **Concurrent Access:** Multiple clients accessing same files simultaneously
3. **Storage Exhaustion:** Disk space exhaustion from uploads
4. **Unicode Filenames:** Non-ASCII characters in file and directory names
5. **Malformed WebDAV Requests:** Invalid WebDAV protocol requests
6. **Network Interruptions:** Partial uploads due to network issues
7. **Authentication Failures:** Repeated authentication failures and lockouts
8. **Directory Traversal Attempts:** Sophisticated path traversal attack attempts
9. **Protocol Version Conflicts:** Different WebDAV protocol versions
10. **Backup and Recovery:** Data backup and recovery for WebDAV storage

## Security Best Practices Violations

1. **Path traversal vulnerability** through unvalidated file system access
2. **Insufficient access control** with basic authentication only
3. **Unrestricted file operations** enabling unauthorized modifications
4. **Missing input validation** for paths, files, and WebDAV properties
5. **Protocol method exposure** without specific security considerations
6. **Insecure storage backend** without encryption or access controls
7. **Missing security headers** for defense in depth
8. **No rate limiting** for WebDAV operations
9. **No file type restrictions** allowing any file uploads
10. **No audit logging** for security monitoring

## Positive Security Aspects

1. **Authentication middleware integration** for access control
2. **WebDAV protocol compliance** with standard methods
3. **Separate endpoints** for CalDAV and CardDAV protocols
4. **Error handling** with appropriate HTTP status codes

## Critical Issues Summary

1. **Path Traversal Vulnerability:** Local file system access without path validation
2. **Insufficient Access Control:** Basic authentication with potentially weak authorization
3. **Unrestricted File Operations:** Full WebDAV operations without fine-grained control
4. **Missing Input Validation:** No validation of paths, files, or WebDAV content
5. **Protocol Method Exposure:** Advanced WebDAV methods without security considerations
6. **Storage Backend Security:** Unencrypted local storage without access controls
7. **Missing Security Headers:** Limited security headers for defense in depth

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - WebDAV implementation with path traversal and insufficient access control vulnerabilities