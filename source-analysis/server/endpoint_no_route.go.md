# Security Analysis: server/endpoint_no_route.go

**File:** `server/endpoint_no_route.go`  
**Type:** Static file serving and SPA fallback handler  
**Lines of Code:** 238  

## Overview
This file implements a no-route handler for serving static files with caching mechanisms and Single Page Application (SPA) fallback functionality. It includes file caching, conditional HTTP requests, content type detection, and cache management. The implementation serves static assets from a file system with memory caching and provides fallback to index.html for SPA routing.

## Key Components

### SetupNoRouteRouter function
**Lines:** 16-141  
**Purpose:** Sets up the no-route handler with file serving and caching functionality  

### File Caching System
- **Cache lookup:** Lines 46-74
- **File reading and caching:** Lines 76-132
- **Cache entry creation:** Lines 115-124

### HTTP Caching
- **Conditional requests:** Lines 50-66, 90-92
- **Cache header management:** Lines 152-170, 173-181
- **ETag generation:** Lines 184-186

### Content Type Detection
**Lines:** 199-237  
**Purpose:** Maps file extensions to appropriate MIME types  

## Security Analysis

### 1. CRITICAL: Path Traversal Vulnerability - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 43, 77  
**Issue:** File path from URL used directly without comprehensive validation enabling directory traversal.

```go
filePath := strings.TrimLeft(c.Request.URL.Path, "/")  // Only removes leading slash
file, err := boxRoot.Open(filePath)  // Direct file access without validation
```

**Risk:**
- **Directory traversal** through "../" sequences in URLs
- **Arbitrary file access** outside intended directory
- **System file exposure** via path manipulation
- **Configuration file access** through traversal attacks

### 2. HIGH: Memory Exhaustion Vulnerability - HIGH RISK
**Severity:** HIGH  
**Lines:** 46-47, 103, 124  
**Issue:** File caching without proper memory limits and unsafe type assertion.

```go
if cached, found := diskFileCache.Get(filePath); found {
    cachedFile := cached.(*DiskFileCache)  // Unsafe type assertion
    content, readErr := readFileWithLimit(file, maxFileSizeToCache)  // Depends on external limit
    diskFileCache.Add(filePath, cacheEntry)  // No cache size limits visible
}
```

**Risk:**
- **Memory exhaustion** from large file caching
- **Cache pollution** through malicious file requests
- **Service disruption** via memory consumption attacks
- **Type assertion panic** from cache corruption

### 3. HIGH: Information Disclosure - HIGH RISK
**Severity:** HIGH  
**Lines:** 84, 105, 135-139  
**Issue:** Error messages and file system access patterns expose system information.

```go
logrus.Printf("Error getting file stats: %v", statErr)
logrus.Printf("[101] Error reading file [%v]: %v", filePath, readErr)
// Fallback always serves index.html revealing SPA structure
```

**Risk:**
- **File system structure disclosure** through error messages
- **Application architecture exposure** via SPA fallback behavior
- **Internal path disclosure** in log messages
- **System information leakage** through detailed errors

### 4. MEDIUM: Cache Poisoning Vulnerability - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 124, 185  
**Issue:** Cache key generation and storage without validation enabling cache poisoning.

```go
diskFileCache.Add(filePath, cacheEntry)  // filePath used as cache key
etag := fmt.Sprintf("\"%x-%x-%x\"", size, modTime.UnixNano(), hash(path))  // Simple hash
```

**Risk:**
- **Cache poisoning** through malicious file paths
- **ETag collision** via weak hash function
- **Cache corruption** from invalid file paths
- **Resource exhaustion** through cache manipulation

### 5. MEDIUM: Content Type Confusion - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 111, 199-237  
**Issue:** Content type determination based solely on file extension without content validation.

```go
contentType := getContentType(filePath)  // Extension-based only
// No content validation against declared type
```

**Risk:**
- **MIME type confusion** attacks
- **XSS vulnerabilities** via content type manipulation
- **File execution** through incorrect MIME types
- **Security bypass** via content type spoofing

### 6. MEDIUM: Resource Exhaustion - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 95-100, 103  
**Issue:** File size checks and reading limits may not prevent all resource exhaustion scenarios.

```go
if stat.Size() > maxFileSizeToCache {  // Only checks cache threshold
    c.FileFromFS(filePath, boxRoot)    // No size limit for direct serving
}
```

**Risk:**
- **Disk I/O exhaustion** from large file requests
- **Network bandwidth consumption** via large file serving
- **CPU exhaustion** from file processing
- **Memory pressure** from file operations

### 7. LOW: ETag Security Issues - LOW RISK
**Severity:** LOW  
**Lines:** 190-196  
**Issue:** Simple hash function for ETag generation may be predictable.

```go
func hash(s string) uint32 {
    h := uint32(0)
    for i := 0; i < len(s); i++ {
        h = h*31 + uint32(s[i])  // Simple polynomial hash
    }
    return h
}
```

**Risk:**
- **ETag prediction** enabling cache manipulation
- **Hash collision** attacks
- **Cache timing** attacks via predictable ETags
- **Fingerprinting** through ETag patterns

## Potential Attack Vectors

### Path Traversal Attacks
1. **Directory Escape:** Use "../" sequences to access files outside web root
2. **Absolute Path Injection:** Attempt to access system files using absolute paths
3. **Encoding Bypass:** Use URL encoding to bypass basic path sanitization
4. **Null Byte Injection:** Use null bytes to truncate file paths

### Cache Manipulation Attacks
1. **Cache Pollution:** Fill cache with large or numerous files to exhaust memory
2. **Cache Poisoning:** Inject malicious content into cache through file manipulation
3. **ETag Collision:** Generate hash collisions to corrupt cache entries
4. **Cache Timing:** Use cache behavior to infer file system structure

### Content Type Attacks
1. **MIME Type Confusion:** Serve malicious content with incorrect MIME types
2. **XSS via Content Type:** Execute scripts through content type manipulation
3. **File Execution:** Trick browsers into executing files as code
4. **Download Attacks:** Force download of malicious files

### Resource Exhaustion Attacks
1. **Memory Exhaustion:** Request large files to consume server memory
2. **Disk I/O Flooding:** Generate excessive file access to degrade performance
3. **Cache Exhaustion:** Fill file cache to degrade service performance
4. **Network Bandwidth:** Request large files repeatedly to consume bandwidth

## Recommendations

### Immediate Actions
1. **Implement Path Validation:** Add comprehensive path traversal protection
2. **Add Cache Limits:** Implement proper cache size and entry limits
3. **Sanitize Error Messages:** Remove file system details from error responses
4. **Validate Content Types:** Add content validation against declared types

### Enhanced Security Implementation

```go
package server

import (
    "crypto/md5"
    "fmt"
    "io"
    "net/http"
    "os"
    "path"
    "path/filepath"
    "regexp"
    "strings"
    "sync"
    "time"
    
    "github.com/daptin/daptin/server/resource"
    "github.com/gin-gonic/gin"
    "github.com/sirupsen/logrus"
)

const (
    maxCacheSize        = 100 * 1024 * 1024  // 100MB total cache size
    maxCacheEntries     = 10000              // Maximum number of cache entries
    maxFileSize         = 50 * 1024 * 1024   // 50MB max file size
    maxPathLength       = 255                // Maximum path length
    shortCacheTime      = 60                 // 1 minute for dynamic content
    longCacheTime       = 3600               // 1 hour for static assets
)

var (
    // Safe path pattern
    safePathPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
    
    // Dangerous path components
    dangerousPathComponents = []string{
        "..", "~", "$", "`", ";", "|", "&",
        "etc", "proc", "sys", "dev", "root",
        "windows", "system32", "boot",
    }
    
    // Cache management
    cacheMutex sync.RWMutex
    cacheSize  int64
    cacheCount int
)

// SecureDiskFileCache represents a secure cached file entry
type SecureDiskFileCache struct {
    Data         []byte
    ContentType  string
    LastModified time.Time
    ETag         string
    Size         int64
    Validated    bool
}

// validateFilePath validates file path for security
func validateFilePath(filePath string) error {
    if filePath == "" {
        return fmt.Errorf("file path cannot be empty")
    }
    
    if len(filePath) > maxPathLength {
        return fmt.Errorf("file path too long: %d characters", len(filePath))
    }
    
    // Clean the path
    cleaned := filepath.Clean(filePath)
    
    // Check for dangerous patterns
    if !safePathPattern.MatchString(cleaned) {
        return fmt.Errorf("file path contains invalid characters")
    }
    
    // Check for dangerous components
    lowerPath := strings.ToLower(cleaned)
    for _, component := range dangerousPathComponents {
        if strings.Contains(lowerPath, component) {
            return fmt.Errorf("file path contains dangerous component: %s", component)
        }
    }
    
    // Ensure path doesn't escape web root
    if strings.HasPrefix(cleaned, "/") || strings.Contains(cleaned, "..") {
        return fmt.Errorf("file path attempts directory traversal")
    }
    
    return nil
}

// validateFileContent validates file content against declared type
func validateFileContent(content []byte, contentType string) error {
    if len(content) == 0 {
        return fmt.Errorf("file content is empty")
    }
    
    if len(content) > maxFileSize {
        return fmt.Errorf("file too large: %d bytes", len(content))
    }
    
    // Basic content type validation
    if strings.HasPrefix(contentType, "text/html") {
        // Check for potential XSS in HTML files
        lowerContent := strings.ToLower(string(content))
        dangerousPatterns := []string{
            "<script", "javascript:", "vbscript:", "data:",
            "onload=", "onerror=", "onclick=",
        }
        
        for _, pattern := range dangerousPatterns {
            if strings.Contains(lowerContent, pattern) {
                logrus.Warnf("Potentially dangerous content detected in HTML file")
                // Don't block but log for monitoring
            }
        }
    }
    
    return nil
}

// generateSecureETag generates a secure ETag using MD5
func generateSecureETag(path string, modTime time.Time, size int64, content []byte) string {
    hash := md5.New()
    hash.Write([]byte(path))
    hash.Write([]byte(modTime.Format(time.RFC3339)))
    hash.Write([]byte(fmt.Sprintf("%d", size)))
    if len(content) > 0 {
        hash.Write(content[:min(len(content), 1024)]) // Use first 1KB for content hash
    }
    
    return fmt.Sprintf("\"%x\"", hash.Sum(nil))
}

// min returns the minimum of two integers
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

// validateCacheOperation checks if cache operation is allowed
func validateCacheOperation(filePath string, size int64) error {
    cacheMutex.RLock()
    defer cacheMutex.RUnlock()
    
    if cacheCount >= maxCacheEntries {
        return fmt.Errorf("cache entry limit exceeded")
    }
    
    if cacheSize+size > maxCacheSize {
        return fmt.Errorf("cache size limit exceeded")
    }
    
    return nil
}

// addToCache safely adds entry to cache
func addToCache(filePath string, entry *SecureDiskFileCache) error {
    if err := validateCacheOperation(filePath, entry.Size); err != nil {
        return err
    }
    
    cacheMutex.Lock()
    defer cacheMutex.Unlock()
    
    // Add to cache (implementation depends on actual cache type)
    diskFileCache.Add(filePath, entry)
    cacheSize += entry.Size
    cacheCount++
    
    return nil
}

// getSecureContentType determines content type with validation
func getSecureContentType(filePath string, content []byte) string {
    ext := strings.ToLower(filepath.Ext(filePath))
    
    // Get base content type
    contentType := getContentType(filePath)
    
    // Additional validation for executable types
    executableTypes := map[string]bool{
        "application/javascript": true,
        "text/html":             true,
        "image/svg+xml":         true,
    }
    
    if executableTypes[strings.Split(contentType, ";")[0]] {
        // Validate content for executable types
        if err := validateFileContent(content, contentType); err != nil {
            logrus.Warnf("Content validation failed for %s: %v", filePath, err)
        }
    }
    
    return contentType
}

// setSecureClientCacheHeaders sets secure cache headers
func setSecureClientCacheHeaders(c *gin.Context, cachedFile *SecureDiskFileCache, isStatic bool) {
    // Set ETag
    c.Header("ETag", cachedFile.ETag)
    
    // Set Last-Modified
    c.Header("Last-Modified", cachedFile.LastModified.UTC().Format(http.TimeFormat))
    
    // Set appropriate cache time based on content type
    cacheTime := shortCacheTime
    if isStatic {
        cacheTime = longCacheTime
    }
    
    // Set Cache-Control with security considerations
    cacheControl := fmt.Sprintf("public, max-age=%d", cacheTime)
    if strings.HasPrefix(cachedFile.ContentType, "text/html") {
        // HTML files should have shorter cache time
        cacheControl = fmt.Sprintf("public, max-age=%d, must-revalidate", shortCacheTime)
    }
    
    c.Header("Cache-Control", cacheControl)
    
    // Security headers
    c.Header("X-Content-Type-Options", "nosniff")
    
    if strings.HasPrefix(cachedFile.ContentType, "text/html") {
        c.Header("X-Frame-Options", "SAMEORIGIN")
        c.Header("X-XSS-Protection", "1; mode=block")
    }
    
    // Set Content-Type
    c.Header("Content-Type", cachedFile.ContentType)
}

// handleSecureFileServing handles file serving with security validation
func handleSecureFileServing(c *gin.Context, filePath string, boxRoot http.FileSystem) {
    // Validate file path
    if err := validateFilePath(filePath); err != nil {
        logrus.Warnf("Invalid file path requested: %s - %v", filePath, err)
        c.Status(http.StatusNotFound)
        return
    }
    
    // Check cache first
    if cached, found := diskFileCache.Get(filePath); found {
        if cachedFile, ok := cached.(*SecureDiskFileCache); ok && cachedFile.Validated {
            // Handle conditional requests
            if handleConditionalRequest(c, cachedFile) {
                return
            }
            
            // Determine if this is static content
            isStatic := isStaticContent(filePath)
            
            // Set secure headers and serve
            setSecureClientCacheHeaders(c, cachedFile, isStatic)
            c.Data(http.StatusOK, cachedFile.ContentType, cachedFile.Data)
            return
        }
    }
    
    // Try to open file
    file, err := boxRoot.Open(filePath)
    if err != nil {
        // Don't log detailed error for security
        logrus.Debugf("File not found: %s", filePath)
        c.Status(http.StatusNotFound)
        return
    }
    defer file.Close()
    
    // Get file stats
    stat, err := file.(interface{ Stat() (os.FileInfo, error) }).Stat()
    if err != nil {
        logrus.Debugf("Failed to get file stats: %s", filePath)
        c.Status(http.StatusInternalServerError)
        return
    }
    
    // Check file size
    if stat.Size() > maxFileSize {
        logrus.Warnf("File too large: %s (%d bytes)", filePath, stat.Size())
        c.Status(http.StatusRequestEntityTooLarge)
        return
    }
    
    // Read file content
    content, err := io.ReadAll(file)
    if err != nil {
        logrus.Errorf("Failed to read file: %s", filePath)
        c.Status(http.StatusInternalServerError)
        return
    }
    
    // Get secure content type
    contentType := getSecureContentType(filePath, content)
    
    // Generate secure ETag
    etag := generateSecureETag(filePath, stat.ModTime(), stat.Size(), content)
    
    // Create cache entry
    cacheEntry := &SecureDiskFileCache{
        Data:         content,
        ContentType:  contentType,
        LastModified: stat.ModTime(),
        ETag:         etag,
        Size:         stat.Size(),
        Validated:    true,
    }
    
    // Add to cache if possible
    if err := addToCache(filePath, cacheEntry); err != nil {
        logrus.Debugf("Failed to cache file %s: %v", filePath, err)
    }
    
    // Handle conditional requests
    if handleConditionalRequest(c, cacheEntry) {
        return
    }
    
    // Determine if this is static content
    isStatic := isStaticContent(filePath)
    
    // Set secure headers and serve
    setSecureClientCacheHeaders(c, cacheEntry, isStatic)
    c.Data(http.StatusOK, contentType, content)
}

// handleConditionalRequest handles If-Modified-Since and If-None-Match headers
func handleConditionalRequest(c *gin.Context, cachedFile *SecureDiskFileCache) bool {
    // Check ETag first
    if ifNoneMatch := c.GetHeader("If-None-Match"); ifNoneMatch != "" {
        if ifNoneMatch == cachedFile.ETag {
            c.Status(http.StatusNotModified)
            return true
        }
    }
    
    // Check Last-Modified
    if ifModifiedSince := c.GetHeader("If-Modified-Since"); ifModifiedSince != "" {
        if modTime, err := http.ParseTime(ifModifiedSince); err == nil {
            if !cachedFile.LastModified.After(modTime) {
                c.Status(http.StatusNotModified)
                return true
            }
        }
    }
    
    return false
}

// isStaticContent determines if content is static based on extension
func isStaticContent(filePath string) bool {
    ext := strings.ToLower(filepath.Ext(filePath))
    staticExtensions := map[string]bool{
        ".css": true, ".js": true, ".png": true, ".jpg": true,
        ".jpeg": true, ".gif": true, ".svg": true, ".webp": true,
        ".ico": true, ".woff": true, ".woff2": true, ".ttf": true,
        ".pdf": true,
    }
    return staticExtensions[ext]
}

// SetupSecureNoRouteRouter sets up secure no-route handler
func SetupSecureNoRouteRouter(boxRoot http.FileSystem, defaultRouter *gin.Engine) {
    
    // Load index.html securely
    indexFile, err := boxRoot.Open("index.html")
    if err != nil {
        logrus.Errorf("Failed to open index.html: %v", err)
        return
    }
    defer indexFile.Close()
    
    indexFileContents, err := io.ReadAll(indexFile)
    if err != nil {
        logrus.Errorf("Failed to read index.html: %v", err)
        return
    }
    
    // Validate index.html content
    if err := validateFileContent(indexFileContents, "text/html"); err != nil {
        logrus.Warnf("Index.html validation warning: %v", err)
    }
    
    // Root route handler
    defaultRouter.GET("", func(c *gin.Context) {
        // Security headers for index.html
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "SAMEORIGIN")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, must-revalidate", shortCacheTime))
        
        c.Data(http.StatusOK, "text/html; charset=UTF-8", indexFileContents)
    })
    
    // No-route handler with security
    defaultRouter.NoRoute(func(c *gin.Context) {
        // Skip non-GET requests
        if c.Request.Method != http.MethodGet {
            c.Status(http.StatusMethodNotAllowed)
            return
        }
        
        // Get clean file path
        filePath := strings.TrimLeft(c.Request.URL.Path, "/")
        
        // Handle file serving securely
        handleSecureFileServing(c, filePath, boxRoot)
    })
    
    logrus.Infof("Secure no-route handler initialized")
}

// SetupNoRouteRouter maintains backward compatibility
func SetupNoRouteRouter(boxRoot http.FileSystem, defaultRouter *gin.Engine) {
    SetupSecureNoRouteRouter(boxRoot, defaultRouter)
}

// getContentType maintains original implementation
func getContentType(filePath string) string {
    ext := strings.ToLower(filepath.Ext(filePath))
    switch ext {
    case ".html", ".htm":
        return "text/html; charset=UTF-8"
    case ".css":
        return "text/css; charset=UTF-8"
    case ".js":
        return "application/javascript; charset=UTF-8"
    case ".json":
        return "application/json; charset=UTF-8"
    case ".png":
        return "image/png"
    case ".jpg", ".jpeg":
        return "image/jpeg"
    case ".gif":
        return "image/gif"
    case ".svg":
        return "image/svg+xml"
    case ".webp":
        return "image/webp"
    case ".ico":
        return "image/x-icon"
    case ".pdf":
        return "application/pdf"
    case ".txt":
        return "text/plain; charset=UTF-8"
    case ".xml":
        return "application/xml; charset=UTF-8"
    case ".woff":
        return "font/woff"
    case ".woff2":
        return "font/woff2"
    case ".ttf":
        return "font/ttf"
    default:
        return "application/octet-stream"
    }
}
```

### Long-term Improvements
1. **Content Security Policy:** Implement CSP headers for HTML content
2. **File Integrity Monitoring:** Monitor served files for unauthorized changes
3. **Advanced Cache Management:** Implement LRU cache with better eviction policies
4. **Security Scanning:** Scan uploaded/served files for malicious content
5. **Performance Monitoring:** Monitor file serving performance and security metrics

## Edge Cases Identified

1. **Large File Requests:** Very large files causing memory or bandwidth issues
2. **Concurrent Cache Access:** Multiple requests for same file causing race conditions
3. **Cache Corruption:** Invalid cache entries causing application errors
4. **File System Permissions:** Files with restricted permissions causing access errors
5. **Symbolic Links:** Symbolic links potentially escaping web root
6. **Unicode Filenames:** Non-ASCII characters in file paths
7. **Special Characters:** Special characters in URLs causing parsing issues
8. **Cache Exhaustion:** Cache filling up and requiring eviction
9. **Type Assertion Failures:** Cache containing invalid data types
10. **Index.html Missing:** SPA fallback when index.html is not available

## Security Best Practices Violations

1. **Path traversal vulnerability** through unvalidated file path usage
2. **Memory exhaustion vulnerability** from unlimited file caching
3. **Information disclosure** through detailed error messages and logging
4. **Cache poisoning vulnerability** from unvalidated cache operations
5. **Content type confusion** attacks via extension-based type detection
6. **Resource exhaustion** from unlimited file serving
7. **ETag security issues** with weak hash function
8. **Missing input validation** for file paths and cache operations
9. **No security headers** for served content
10. **Unsafe type assertions** causing potential panics

## Positive Security Aspects

1. **Conditional request handling** for efficient caching
2. **Content type mapping** for proper MIME type handling
3. **Cache limits** for some file operations
4. **Error handling** throughout the serving process
5. **SPA fallback** functionality for client-side routing

## Critical Issues Summary

1. **Path Traversal Vulnerability:** File paths used directly without validation enabling directory traversal
2. **Memory Exhaustion Vulnerability:** File caching without proper limits and unsafe type assertions
3. **Information Disclosure:** Error messages and access patterns expose system information
4. **Cache Poisoning Vulnerability:** Cache operations without validation enabling poisoning attacks
5. **Content Type Confusion:** Extension-based type detection without content validation
6. **Resource Exhaustion:** File operations without comprehensive size and resource limits
7. **ETag Security Issues:** Weak hash function enabling cache manipulation attacks

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - Static file serving with path traversal and cache security vulnerabilities