# Security Analysis: server/subsite_handler.go

**File:** `server/subsite_handler.go`  
**Lines of Code:** 282  
**Primary Function:** HTTP request handler for serving subsite content with caching, SPA support, and static file serving

## Summary

This file implements the core request handling logic for subsites, including intelligent caching strategies, SPA fallback support, and optimized static file serving. It manages both positive and negative caching, supports zero-copy file serving, and provides fallback mechanisms for single-page applications.

## Security Issues Found

### 🟡 HIGH Issues

#### 1. **Path Traversal Vulnerability** (Lines 48-51, 63, 137, 207, 213)
```go
if site.SiteType == "hugo" {
    filePath = filepath.Join("public", path)
} else {
    filePath = path
}
filePath = filepath.Join(filePath, "index.html")
content, err := os.ReadFile(filepath.Join(assetCache.LocalSyncPath, filePath))
fullPath := filepath.Join(assetCache.LocalSyncPath, filePath)
```
**Risk:** User-controlled path parameter used in file operations
- No validation or sanitization of URL path before file operations
- filepath.Join() alone doesn't prevent directory traversal
- Could access files outside intended directory structure
- Multiple file operations using unsanitized paths
**Impact:** High - Unauthorized file system access and information disclosure
**Remediation:** Implement path validation and restrict access to intended directories

#### 2. **Unsafe Type Assertions** (Lines 80, 109, 259)
```go
negEntry := entry.(*NegativeCacheEntry)
cacheEntry := entry.(*IndexCacheEntry)
cacheEntry := entry.(*IndexCacheEntry)
```
**Risk:** Type assertions without validation can panic
- sync.Map.Load() returns interface{} requiring type assertion
- No validation that loaded values are of expected type
- Panic could cause denial of service
**Impact:** High - Application crash and denial of service
**Remediation:** Use safe type assertions with ok checks

#### 3. **Host Header Injection in Cache Keys** (Lines 44, 54, 106, 122, 153, 194, 208, 253, 258)
```go
host := c.Request.Host
negativeKey := host + ":" + filePath
indexCache.Load(host)
indexCache.Store(host, cacheEntry)
```
**Risk:** Host header manipulation affecting cache behavior
- Host header value used directly in cache keys without validation
- Malicious Host headers could pollute cache
- Cache poisoning attacks possible
- No validation of Host header format
**Impact:** High - Cache poisoning and potential request routing manipulation
**Remediation:** Validate and sanitize Host header values before use

### 🟠 MEDIUM Issues

#### 4. **Information Disclosure Through Fallback Content** (Lines 270-274)
```go
c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!DOCTYPE html>
<html>
<head><title>Site</title></head>
<body><h1>Welcome</h1><p>Site is loading...</p></body>
</html>`))
```
**Risk:** Hardcoded fallback HTML reveals application structure
- Generic fallback content confirms site existence
- Could aid in reconnaissance by confirming valid hosts
- Reveals that site uses SPA architecture
**Impact:** Medium - Information disclosure for reconnaissance
**Remediation:** Use configurable fallback content or proper error pages

#### 5. **Cache Timing Side Channel** (Lines 78-87, 108-117)
```go
if entry, exists := negativeCache.Load(key); exists {
    negEntry := entry.(*NegativeCacheEntry)
    if time.Now().Before(negEntry.ExpiresAt) {
        return true
    }
}
```
**Risk:** Cache behavior reveals information about file existence
- Different response times for cached vs non-cached requests
- Negative cache timing could reveal file system structure
- Cache expiration timing patterns observable
**Impact:** Medium - Information disclosure through timing analysis
**Remediation:** Implement consistent response timing regardless of cache state

#### 6. **Missing Rate Limiting for Cache Operations** (Lines 95, 153)
```go
negativeCache.Store(key, entry)
indexCache.Store(host, cacheEntry)
```
**Risk:** Cache exhaustion through excessive requests
- No limits on cache entries per host
- Could exhaust memory with many unique requests
- No protection against cache flooding attacks
**Impact:** Medium - Memory exhaustion and denial of service
**Remediation:** Implement cache size limits and rate limiting

#### 7. **File System Race Conditions** (Lines 130-137, 199-203)
```go
fileInfo, err := file.Stat()
content, err := os.ReadFile(filepath.Join(assetCache.LocalSyncPath, filePath))
```
**Risk:** TOCTOU race condition between file operations
- File could be modified between stat and read operations
- Inconsistent file metadata and content
- Potential for serving stale or corrupted content
**Impact:** Medium - Content integrity issues
**Remediation:** Use atomic file operations or file locking

### 🔵 LOW Issues

#### 8. **Hardcoded Cache TTL Values** (Lines 37-38)
```go
IndexCacheTTL    = 5 * time.Minute // 5 minutes for index.html
NegativeCacheTTL = 2 * time.Minute // 2 minutes for 404s
```
**Risk:** Fixed cache durations reduce operational flexibility
- No runtime configuration capability
- May not suit all deployment scenarios
- Different content types may need different TTLs
**Impact:** Low - Operational inflexibility
**Remediation:** Make cache TTL values configurable

#### 9. **Global Cache Variables** (Lines 31-32)
```go
indexCache    sync.Map // map[string]*IndexCacheEntry (keyed by host)
negativeCache sync.Map // map[string]*NegativeCacheEntry (keyed by host:path)
```
**Risk:** Global state management issues
- Shared cache state across all requests
- Difficult to isolate for testing
- No cleanup mechanism for old entries
**Impact:** Low - Code maintainability and testing issues
**Remediation:** Use dependency injection and proper cache lifecycle management

#### 10. **Missing Input Validation for File Extensions** (Line 244)
```go
if contentType := mime.TypeByExtension(filepath.Ext(fullPath)); contentType != "" {
    c.Header("Content-Type", contentType)
}
```
**Risk:** Potential for serving unexpected content types
- No validation of file extensions before MIME type detection
- Could serve content with unexpected MIME types
- May affect browser security policies
**Impact:** Low - Content type confusion
**Remediation:** Validate file extensions against allowed types

## Code Quality Issues

1. **Error Handling**: Inconsistent error handling patterns across functions
2. **Path Security**: Multiple file operations without proper path validation
3. **Cache Management**: No cleanup mechanism for expired entries
4. **Type Safety**: Unsafe type assertions without validation
5. **Resource Management**: File handles closed but error handling could be improved

## Recommendations

### Immediate Actions Required

1. **Path Validation**: Implement comprehensive path validation to prevent directory traversal
2. **Type Safety**: Add safe type assertions with proper error handling
3. **Host Validation**: Validate and sanitize Host header values
4. **Cache Security**: Implement cache size limits and validation

### Security Improvements

1. **Access Control**: Restrict file access to intended directory trees
2. **Input Sanitization**: Validate all user-controlled inputs including paths and headers
3. **Cache Protection**: Implement protection against cache pollution and exhaustion
4. **Content Security**: Validate file types and content before serving

### Code Quality Enhancements

1. **Configuration**: Make cache parameters and timeouts configurable
2. **Error Handling**: Implement consistent error handling throughout
3. **Resource Management**: Add proper cleanup for cache entries and file handles
4. **Testing**: Add comprehensive tests for security scenarios

## Attack Vectors

1. **Directory Traversal**: Use "../" sequences in URL paths to access files outside intended directories
2. **Host Header Injection**: Manipulate Host header to poison cache or redirect requests
3. **Cache Exhaustion**: Send many unique requests to exhaust cache memory
4. **Type Confusion**: Trigger panic through cache entry type manipulation
5. **Timing Analysis**: Use cache timing differences to enumerate file system structure

## Impact Assessment

- **Confidentiality**: HIGH - Path traversal could expose sensitive files
- **Integrity**: MEDIUM - Race conditions could affect content integrity
- **Availability**: HIGH - Type assertion panics and cache exhaustion could cause DoS
- **Authentication**: N/A - File serving doesn't involve authentication
- **Authorization**: HIGH - Path validation critical for proper access control

This subsite handler provides sophisticated caching and serving capabilities but has several critical security vulnerabilities primarily around path traversal and input validation. The caching mechanisms also introduce potential attack vectors that need proper protection.

## Technical Notes

The subsite handler implements:
1. Intelligent caching with both positive and negative caching
2. SPA fallback support for client-side routing
3. Zero-copy file serving for performance
4. ETag and Last-Modified header support
5. Memory caching for index.html files
6. Content type detection and proper HTTP headers

The main security concerns revolve around the lack of proper input validation for paths and host headers, combined with unsafe type assertions that could lead to application crashes. The caching mechanisms, while performance-enhancing, also create potential attack surfaces that require careful security considerations.

## SPA Architecture Security Notes

The Single Page Application (SPA) support includes:
- Fallback to index.html for missing routes
- Negative caching to avoid repeated cloud requests
- Memory caching for frequently accessed index files

However, this architecture requires careful security implementation to prevent:
- Path traversal attacks through route manipulation
- Cache poisoning through host header manipulation
- Information disclosure through timing analysis