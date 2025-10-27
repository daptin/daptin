# Security Analysis: server/subsite_engine.go

**File:** `server/subsite_engine.go`  
**Lines of Code:** 47  
**Primary Function:** Creates and configures Gin engines for individual subsites with middleware and request handling

## Summary

This file implements the creation of subsite-specific Gin engines for serving content from different sources. It sets up middleware chains, statistics tracking, and request routing for individual subsites. The implementation includes statistics endpoints and delegates main request handling to a specialized subsite request handler.

## Security Issues Found

### 🟠 MEDIUM Issues

#### 1. **Missing Authentication for Statistics Endpoints** (Lines 32-34, 42-44)
```go
hostRouter.GET("/stats", func(c *gin.Context) {
    c.JSON(200, subsiteStats.Data())
})
hostRouter.Handle("GET", "/statistics", func(c *gin.Context) {
    c.JSON(http.StatusOK, Stats.Data())
})
```
**Risk:** Unauthenticated access to subsite statistics
- Statistics endpoints exposed without authentication
- Performance and usage data available to unauthorized users
- Could reveal traffic patterns and system behavior
- Duplicate endpoints serving similar data
**Impact:** Medium - Information disclosure through performance metrics
**Remediation:** Add authentication middleware for statistics endpoints

#### 2. **Global Statistics Variable Access** (Lines 21, 43)
```go
defer Stats.End(beginning, stats.WithRecorder(recorder))
c.JSON(http.StatusOK, Stats.Data())
```
**Risk:** Uncontrolled access to global statistics
- Global Stats variable accessed without validation
- Could expose system-wide performance data
- No separation between subsite and global statistics
- Potential race conditions on global state
**Impact:** Medium - Information disclosure and potential race conditions
**Remediation:** Use scoped statistics and proper access controls

#### 3. **Information Disclosure Through Debug Logging** (Line 36)
```go
log.Tracef("Serve subsite[%s] from source [%s]", site.Name, assetCache.LocalSyncPath)
```
**Risk:** File system paths exposed in trace logs
- Local file system paths logged without sanitization
- Could reveal server directory structure
- Subsite names and paths available in logs
- Information useful for reconnaissance
**Impact:** Medium - File system information disclosure
**Remediation:** Use debug-level logging or sanitize logged paths

### 🔵 LOW Issues

#### 4. **Hardcoded HTTP Status Codes** (Lines 33, 43)
```go
c.JSON(200, subsiteStats.Data())
c.JSON(http.StatusOK, Stats.Data())
```
**Risk:** Inconsistent HTTP status code usage
- Mixed use of numeric and constant status codes
- Potential for incorrect status code responses
- Reduces code maintainability
**Impact:** Low - Code consistency issues
**Remediation:** Use consistent HTTP status constants

#### 5. **Duplicate Statistics Endpoints** (Lines 32-34, 42-44)
```go
hostRouter.GET("/stats", func(c *gin.Context) {
    c.JSON(200, subsiteStats.Data())
})
hostRouter.Handle("GET", "/statistics", func(c *gin.Context) {
    c.JSON(http.StatusOK, Stats.Data())
})
```
**Risk:** Confusing API design with duplicate functionality
- Two endpoints serving similar but different statistics
- /stats serves subsite-specific statistics
- /statistics serves global statistics
- Unclear API design for consumers
**Impact:** Low - API confusion and maintenance overhead
**Remediation:** Consolidate or clearly differentiate endpoint purposes

#### 6. **Commented Code Without Context** (Line 39)
```go
//hostRouter.Any("/", SubsiteRequestHandler(site, tempDirectoryPath))
```
**Risk:** Commented code indicates potential alternative implementation
- May indicate previous vulnerability or design change
- Could be accidentally uncommented
- Lacks context about why it was commented out
**Impact:** Low - Potential for accidental activation
**Remediation:** Remove commented code or add explanatory comments

#### 7. **Missing Input Validation** (Lines 12, 28-30)
```go
func CreateSubsiteEngine(site subsite.SubSite, assetCache *assetcachepojo.AssetFolderCache, middlewares []gin.HandlerFunc) *gin.Engine {
    for _, mid := range middlewares {
        hostRouter.Use(mid)
    }
}
```
**Risk:** No validation of function parameters
- site parameter not validated for required fields
- assetCache could be nil causing runtime panic
- middlewares array not validated for nil entries
**Impact:** Low - Runtime stability issues
**Remediation:** Add parameter validation and nil checks

## Code Quality Issues

1. **Error Handling**: No error handling for statistics or middleware operations
2. **Input Validation**: Missing validation for function parameters
3. **API Design**: Duplicate endpoints with unclear differentiation
4. **Global State**: Direct access to global statistics variables
5. **Logging**: Potential information disclosure through trace logging

## Recommendations

### Immediate Actions Required

1. **Access Control**: Add authentication middleware for statistics endpoints
2. **Information Security**: Sanitize or remove file path logging
3. **Code Cleanup**: Remove or document commented code
4. **Parameter Validation**: Add validation for function parameters

### Security Improvements

1. **Statistics Security**: Implement access controls for performance data
2. **Logging Security**: Use appropriate log levels and sanitize sensitive information
3. **Global State**: Reduce dependencies on global variables
4. **API Security**: Ensure all endpoints have appropriate security controls

### Code Quality Enhancements

1. **Error Handling**: Add comprehensive error handling for all operations
2. **API Design**: Clarify the purpose and scope of different statistics endpoints
3. **Documentation**: Add documentation for security considerations
4. **Testing**: Add unit tests for edge cases and security scenarios

## Attack Vectors

1. **Information Gathering**: Access statistics endpoints to gather system performance data
2. **Reconnaissance**: Use trace logs to gather file system structure information
3. **Resource Enumeration**: Identify subsite configurations through exposed data
4. **Performance Analysis**: Monitor system behavior through statistics data

## Impact Assessment

- **Confidentiality**: MEDIUM - Statistics and file system information exposed
- **Integrity**: LOW - Read-only operations don't modify system state
- **Availability**: LOW - No direct availability impact from this code
- **Authentication**: MEDIUM - Statistics endpoints lack authentication
- **Authorization**: MEDIUM - No authorization controls for sensitive information

This file provides subsite engine functionality with some security considerations primarily around information disclosure through statistics endpoints and logging. While the security impact is moderate, proper access controls and information sanitization would improve the overall security posture.

## Technical Notes

The subsite engine implementation:
1. Creates isolated Gin engines for each subsite
2. Applies middleware chains for request processing
3. Provides statistics tracking at both subsite and global levels
4. Uses NoRoute handler for catch-all request processing
5. Exposes performance metrics through HTTP endpoints

The main security concerns are around the exposure of performance statistics and file system information through logging and unprotected endpoints. For a multi-tenant subsite system, proper isolation and access controls are essential.

## Subsite Architecture Considerations

In a subsite architecture:
- Each subsite should have isolated statistics and logging
- Access to performance data should be controlled
- File system paths should not be exposed in logs
- Global statistics should be separated from subsite-specific data
- Authentication should be enforced for administrative endpoints