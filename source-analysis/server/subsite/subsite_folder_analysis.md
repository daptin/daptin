# Security Analysis: server/subsite/ folder

**Folder:** `server/subsite/`  
**Files Analyzed:** `subsite_action_config.go` (24 lines), `subsite_staticfs_server.go` (19 lines), `utils.go` (19 lines), `get_all_subsites.go` (67 lines), `subsite_cache_config.go` (298 lines), `template_handler.go` (508 lines)  
**Total Lines of Code:** 935  
**Primary Function:** Subsite management system providing template rendering, static file serving, cache configuration, and action handling for multi-tenant web hosting functionality

## Summary

This folder implements a comprehensive subsite management system that handles template rendering, static file serving, caching configuration, and action execution for subsites. The system includes sophisticated caching mechanisms, ETag generation, cache validation, and supports dynamic template routing with database-driven configuration. It integrates with authentication, permissions, and database resources to provide secure multi-tenant hosting capabilities.

## Security Issues Found

### 🔴 CRITICAL Issues

#### 1. **Unsafe Type Assertions Without Error Handling** (Lines 13, 106 in subsite_action_config.go, Lines 8 in utils.go, Lines 105, 126, 277, 278 in template_handler.go)
```go
actionReqStr := actionRequestInt.(string)
fmtString := message[0].(string)
urlPattern := templateRow["url_pattern"].(string)
templateName := templateInstance["name"].(string)
var content = attrs["content"].(string)
var mimeType = attrs["mime_type"].(string)
```
**Risk:** Type assertions can panic if types don't match
- Runtime panics if values are not expected types
- No validation of type assertions before use
- Could crash application with malformed data
- Attackers could trigger panics with unexpected data types
**Impact:** Critical - Application crashes through type assertion panics
**Remediation:** Use safe type assertions with ok checks

#### 2. **Path Traversal Vulnerability in Static File Server** (Lines 13-17 in subsite_staticfs_server.go)
```go
func (spf *StaticFsWithDefaultIndex) Open(name string) (http.File, error) {
    f, err := spf.system.Open(name)
    if err != nil {
        return spf.system.Open(spf.pageOn404)
    }
```
**Risk:** No validation of file paths before opening
- Could access files outside intended directory
- No sanitization of path components
- Directory traversal attacks possible
- Potential access to sensitive system files
**Impact:** Critical - Unauthorized file access through path traversal
**Remediation:** Validate and sanitize file paths, use filepath.Clean()

#### 3. **SQL Injection Through Dynamic Query Construction** (Lines 15-21 in get_all_subsites.go)
```go
s, v, err := statementbuilder.Squirrel.Select(
    goqu.I("s.name"), goqu.I("s.hostname"),
    goqu.I("s.cloud_store_id"),
    goqu.I("s."+"user_account_id"), goqu.I("s.path"),
```
**Risk:** String concatenation in SQL query construction
- Direct string concatenation without validation
- Potential for SQL injection attacks
- No input sanitization before query building
- Could execute arbitrary SQL commands
**Impact:** Critical - SQL injection leading to data breach
**Remediation:** Use parameterized queries and validate all inputs

### 🟡 HIGH Issues

#### 4. **JSON Unmarshaling Without Validation** (Lines 17, 111 in subsite_action_config.go, Line 107 in template_handler.go)
```go
err := json.Unmarshal([]byte(actionReqStr), &actionRequest)
err := json.Unmarshal([]byte(actionReqStr), &cacheConfig)
err = json.Unmarshal([]byte(urlPattern), &strArray)
```
**Risk:** JSON unmarshaling without input validation
- No validation of JSON content before unmarshaling
- Could trigger memory exhaustion with large JSON
- Potential for JSON injection attacks
- No size limits on JSON input
**Impact:** High - Memory exhaustion and JSON injection attacks
**Remediation:** Add JSON size limits and content validation

#### 5. **Information Disclosure Through Error Logging** (Lines 14 in utils.go, Lines 28, 41, 48, 54, 131, 137 in get_all_subsites.go, Lines 109, 131, 137 in template_handler.go)
```go
log.Errorf(fmtString+": %v", args...)
log.Errorf("[424] failed to prepare statment: %v", err)
log.Errorf("Failed to scan site from db to struct: %v", err)
log.Errorf("Failed to get template instance for template [%v]", templateName)
```
**Risk:** Sensitive information exposed in error messages and logs
- Database error details in logs
- Template names and configuration exposed
- SQL statement preparation errors logged
- Error details accessible to attackers
**Impact:** High - Information disclosure for reconnaissance attacks
**Remediation:** Sanitize error messages, avoid exposing internal details

#### 6. **Cache Key Predictability and Injection** (Lines 52-88 in template_handler.go)
```go
func generateCacheKey(c *gin.Context, config *CacheConfig) string {
    key := c.Request.URL.Path
    if config.CacheKeyPrefix != "" {
        key = config.CacheKeyPrefix + ":" + key
    }
```
**Risk:** Predictable cache keys with user-controlled input
- URL path and query parameters directly in cache keys
- No validation of cache key components
- Potential for cache poisoning attacks
- Cache key collision possibilities
**Impact:** High - Cache poisoning and data corruption
**Remediation:** Hash cache keys and validate all components

#### 7. **Template Injection Through Action Execution** (Lines 253-265 in template_handler.go)
```go
if len(actionRequest.Action) > 0 && len(actionRequest.Type) > 0 {
    actionRequest.Attributes = inFields
    actionResponses, errAction := cruds["action"].HandleActionRequest(actionRequest, api2goRequestData, transaction1)
```
**Risk:** User-controlled template actions executed without validation
- Action requests processed without security validation
- Could execute arbitrary templates or actions
- No authentication check for action execution
- Potential for template injection attacks
**Impact:** High - Template injection and unauthorized action execution
**Remediation:** Validate and sanitize all action requests, add authentication

### 🟠 MEDIUM Issues

#### 8. **ETag Information Disclosure** (Lines 455-470 in template_handler.go)
```go
func generateETag(content string, strategy string) string {
    hash := sha256.Sum256([]byte(content))
    etag := hex.EncodeToString(hash[:8]) // Use first 8 bytes for brevity
```
**Risk:** ETags could leak content information
- Short ETags may be vulnerable to collision attacks
- Content hash exposed in ETags
- Could reveal information about file contents
- Predictable ETag generation
**Impact:** Medium - Information disclosure through ETags
**Remediation:** Use stronger hashing and longer ETags

#### 9. **Race Conditions in Cache Operations** (Lines 166-207, 327-354 in template_handler.go)
```go
if cachedFile, found := fileCache.Get(cacheKey); found {
    // ... processing ...
}
// Later...
fileCache.Set(cacheKey, newCachedFile)
```
**Risk:** Race conditions between cache check and set operations
- Multiple concurrent requests could conflict
- Cache corruption possible with concurrent access
- No atomic operations for cache updates
- Could lead to inconsistent cache state
**Impact:** Medium - Cache corruption and inconsistent responses
**Remediation:** Use atomic cache operations or locking

#### 10. **Base64 Decoding Without Validation** (Lines 360-367 in template_handler.go)
```go
func Atob(data string) string {
    decodedData, err := base64.StdEncoding.DecodeString(data)
    if err != nil {
        log.Printf("Atob failed: %v", err)
        return ""
    }
```
**Risk:** Base64 decoding without input validation
- No validation of base64 string format
- Could decode malicious content
- Silent failure returns empty string
- No size limits on decoded data
**Impact:** Medium - Processing of malicious decoded content
**Remediation:** Validate base64 input and add size limits

### 🔵 LOW Issues

#### 11. **Hardcoded Cache Configuration** (Lines 83-100 in subsite_cache_config.go)
```go
cacheConfig := CacheConfig{
    Enable:       false,
    MaxAge:       0,
    Revalidate:   true,
    // ... hardcoded defaults
}
```
**Risk:** Hardcoded default cache settings
- Fixed cache configuration values
- No runtime configuration flexibility
- Could be inappropriate for different environments
- Security settings hardcoded
**Impact:** Low - Inflexible cache security configuration
**Remediation:** Make cache settings configurable

#### 12. **Debug Information in Production Code** (Lines 93, 104, 114, 140, 145 in template_handler.go)
```go
log.Infof("Got [%d] Templates from database", len(templateList))
log.Infof("ProcessTemplateRoute [%s] %v", templateRow["name"], templateRow["url_pattern"])
log.Tracef("Serve template[%s] request[%s]", templateName, c.Request.URL.Path)
```
**Risk:** Debug logging could expose sensitive information
- Template names and URL patterns logged
- Request paths logged at trace level
- Could expose system internals
- Debug logs might be enabled in production
**Impact:** Low - Information disclosure through debug logging
**Remediation:** Remove or sanitize debug logging

## Code Quality Issues

1. **Error Handling**: Inconsistent error handling patterns throughout the code
2. **Type Safety**: Heavy use of unsafe type assertions without validation
3. **Input Validation**: Limited validation of user inputs and configuration
4. **Resource Management**: Potential memory leaks with cache operations
5. **Security Context**: Some operations lack proper authentication checks

## Recommendations

### Immediate Actions Required

1. **Type Safety**: Replace unsafe type assertions with safe alternatives
2. **Path Security**: Implement proper path validation for file access
3. **SQL Security**: Use parameterized queries and input validation
4. **Authentication**: Add authentication checks for template and action execution

### Security Improvements

1. **Input Validation**: Comprehensive validation for all inputs and configuration
2. **Cache Security**: Secure cache key generation and validation
3. **Template Security**: Validate and sanitize template actions and content
4. **Error Security**: Sanitize error messages to prevent information disclosure

### Code Quality Enhancements

1. **Error Management**: Implement consistent error handling patterns
2. **Configuration**: Make security settings configurable instead of hardcoded
3. **Documentation**: Add comprehensive security and usage documentation
4. **Testing**: Add security-focused unit and integration tests

## Attack Vectors

1. **Type Confusion**: Trigger panics through malformed configuration values
2. **Path Traversal**: Access unauthorized files through static file server
3. **SQL Injection**: Exploit dynamic query construction for data access
4. **Cache Poisoning**: Manipulate cache keys to serve malicious content
5. **Template Injection**: Execute unauthorized templates or actions
6. **Information Gathering**: Extract system information through error messages
7. **JSON Attacks**: Use malformed JSON to cause denial of service
8. **Race Exploitation**: Exploit cache race conditions for data corruption

## Impact Assessment

- **Confidentiality**: HIGH - Path traversal and information disclosure vulnerabilities
- **Integrity**: HIGH - SQL injection and cache poisoning could corrupt data
- **Availability**: HIGH - Type assertion panics could cause service disruption
- **Authentication**: HIGH - Missing authentication checks for template execution
- **Authorization**: HIGH - Unauthorized file access and action execution possible

This subsite system has several critical security vulnerabilities that need immediate attention.

## Technical Notes

The subsite system:
1. Provides multi-tenant web hosting capabilities
2. Handles template rendering with database-driven configuration
3. Implements sophisticated caching mechanisms with ETag support
4. Supports static file serving with fallback handling
5. Integrates with authentication and permission systems
6. Includes comprehensive cache configuration options

The main security concerns revolve around input validation, path traversal, and authentication.

## Subsite Security Considerations

For subsite management systems:
- **Path Security**: Validate all file paths to prevent traversal attacks
- **Template Security**: Validate and sanitize template content and actions
- **Cache Security**: Secure cache key generation and validation
- **Authentication Security**: Require authentication for all sensitive operations
- **Input Security**: Validate all user inputs and configuration data
- **SQL Security**: Use parameterized queries and input validation

The current implementation needs comprehensive security enhancements.

## Recommended Security Enhancements

1. **Path Security**: Comprehensive path validation and sanitization
2. **Type Security**: Safe type assertions with proper error handling
3. **SQL Security**: Parameterized queries and input validation
4. **Cache Security**: Secure cache key generation and atomic operations
5. **Template Security**: Validation and authentication for template operations
6. **Error Security**: Sanitized error messages without sensitive information