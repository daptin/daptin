# Security Analysis: server/hostswitch/ folder

**Folder:** `server/hostswitch/`  
**Files Analyzed:** `utils.go` (76 lines), `host_switch.go` (140 lines)  
**Total Lines of Code:** 216  
**Primary Function:** Host-based routing system providing virtual hosting functionality with authentication middleware and subsite management for multi-tenant applications

## Summary

This folder implements a host-based routing system that enables virtual hosting capabilities in the application. It includes utility functions for string manipulation and checking, plus a main HostSwitch component that routes requests based on hostname and path patterns. The system integrates with authentication middleware and permission systems to control access to different subsites and hosts.

## Security Issues Found

### 🔴 CRITICAL Issues

#### 1. **Unsafe Type Assertion Without Error Handling** (Lines 69, 98 in host_switch.go)
```go
user = userI.(*auth.SessionUser)
user = userI.(*auth.SessionUser)
```
**Risk:** Type assertion can panic if userI is not the expected type
- Runtime panics if context value is not SessionUser type
- No validation of type assertion before use
- Could crash application with malformed context data
- Attackers could trigger panics with unexpected context values
**Impact:** Critical - Application crashes through type assertion panics
**Remediation:** Use safe type assertions with ok checks

#### 2. **Unsafe Variadic Function with Type Assertion** (Line 44 in utils.go)
```go
fmtString := message[0].(string)
```
**Risk:** Type assertion on variadic arguments without validation
- Panic if first argument is not a string
- No bounds checking on message slice
- Could crash application with malformed arguments
- Potential for denial of service attacks
**Impact:** Critical - Application crashes through unsafe type assertions
**Remediation:** Add bounds checking and safe type assertions

### 🟡 HIGH Issues

#### 3. **Information Disclosure Through Error Logging** (Lines 50, 130 in host_switch.go, Line 50 in utils.go)
```go
log.Errorf(fmtString+": %v", args...)
log.Errorf("Failed to find dashboard route")
```
**Risk:** Sensitive information exposed in error messages and logs
- Error details could reveal system internals
- Hostname and routing information in logs
- Potential for information gathering attacks
- Debug information accessible to attackers
**Impact:** High - Information disclosure for reconnaissance attacks
**Remediation:** Sanitize error messages, avoid exposing internal details

#### 4. **Authentication Bypass Through Path Manipulation** (Lines 87-115 in host_switch.go)
```go
if len(pathParts) > 1 && !constants.WellDefinedApiPaths[pathParts[1]] {
    firstSubFolder := pathParts[1]
    subSite, isSubSite := hs.SiteMap[firstSubFolder]
    if isSubSite {
        r.URL.Path = "/" + strings.Join(pathParts[2:], "/")
```
**Risk:** Path manipulation could bypass authentication checks
- URL path modification without proper validation
- Could access protected resources through path manipulation
- No validation of modified path components
- Potential for directory traversal attacks
**Impact:** High - Authentication bypass through path manipulation
**Remediation:** Validate and sanitize path components before modification

#### 5. **Host Header Injection Vulnerability** (Line 42 in host_switch.go)
```go
hostName := strings.Split(r.Host, ":")[0]
```
**Risk:** Host header injection through unvalidated Host header
- Host header used directly for routing decisions
- No validation of Host header format or content
- Could route to unexpected handlers
- Potential for HTTP host header attacks
**Impact:** High - Host header injection leading to routing manipulation
**Remediation:** Validate and sanitize Host header before use

### 🟠 MEDIUM Issues

#### 6. **Race Conditions in Map Access** (Lines 23, 29-34, 46, 50, 57, 90, 107, 118, 128 in host_switch.go)
```go
return hs.HandlerMap[name]
for key, h := range hs.HandlerMap {
subSite := hs.SiteMap[hostName]
```
**Risk:** Concurrent access to maps without synchronization
- HandlerMap and SiteMap accessed without locks
- Race conditions during map reads and writes
- Could lead to inconsistent routing behavior
- Potential for panic during concurrent map access
**Impact:** Medium - Race conditions and inconsistent routing
**Remediation:** Add proper synchronization for map access

#### 7. **Default Fallback Without Authentication** (Lines 117-138 in host_switch.go)
```go
if !BeginsWithCheck(r.Host, "dashboard.") && !BeginsWithCheck(r.Host, "api.") {
    handler, ok := hs.HandlerMap["dashboard"]
    if !ok {
        //log.Errorf("Failed to find default route")
    } else {
        handler.ServeHTTP(w, r)
        return
    }
}
```
**Risk:** Default routing without authentication checks
- Dashboard handler used as default without permission verification
- Could provide access to protected resources
- No user authentication for fallback routing
- Bypass of intended access controls
**Impact:** Medium - Unauthorized access through default routing
**Remediation:** Add authentication checks for default routing

#### 8. **Hardcoded String Comparison in Routing Logic** (Lines 30-32 in host_switch.go)
```go
if key == "api" {
    continue
}
```
**Risk:** Hardcoded routing exclusions could be bypassed
- Fixed string comparison for API routing
- No configuration for routing exclusions
- Could be bypassed with similar hostnames
- Inflexible routing logic
**Impact:** Medium - Hardcoded routing logic could be exploited
**Remediation:** Use configurable routing patterns

### 🔵 LOW Issues

#### 9. **Missing Input Validation for String Functions** (Lines 11-38 in utils.go)
```go
func EndsWithCheck(str string, endsWith string) bool {
    if len(endsWith) > len(str) {
        return false
    }
```
**Risk:** No validation of input parameters for utility functions
- No validation of string inputs
- Could handle malformed or extremely large strings
- No protection against resource exhaustion
- Potential for unexpected behavior
**Impact:** Low - Potential for unexpected behavior with malformed inputs
**Remediation:** Add input validation for utility functions

#### 10. **Debug Information in Production Code** (Line 39 in host_switch.go)
```go
log.Debugf("HostSwitch.ServeHTTP RequestUrl: %v", r.URL)
```
**Risk:** Debug logging could expose sensitive information
- Request URLs logged at debug level
- Could expose sensitive path information
- Debug logs might be enabled in production
- Potential for information disclosure
**Impact:** Low - Information disclosure through debug logging
**Remediation:** Remove or sanitize debug logging

## Code Quality Issues

1. **Error Handling**: Inconsistent error handling patterns throughout the code
2. **Type Safety**: Unsafe type assertions without proper validation
3. **Concurrency**: No synchronization for shared map access
4. **Input Validation**: Limited validation of inputs and headers
5. **Security Context**: Some operations lack proper authentication checks

## Recommendations

### Immediate Actions Required

1. **Type Safety**: Replace unsafe type assertions with safe alternatives
2. **Input Validation**: Validate Host headers and path components
3. **Authentication**: Add authentication checks for all routing paths
4. **Synchronization**: Add proper locking for map access

### Security Improvements

1. **Header Validation**: Validate and sanitize HTTP headers before use
2. **Path Security**: Implement secure path handling and validation
3. **Access Control**: Ensure all routes have proper authorization checks
4. **Error Security**: Sanitize error messages to prevent information disclosure

### Code Quality Enhancements

1. **Error Management**: Implement consistent error handling patterns
2. **Configuration**: Make routing logic configurable instead of hardcoded
3. **Documentation**: Add comprehensive security and usage documentation
4. **Testing**: Add security-focused unit and integration tests

## Attack Vectors

1. **Type Confusion**: Trigger panics through malformed context values
2. **Host Header Injection**: Manipulate routing through Host header attacks
3. **Path Traversal**: Exploit path manipulation for unauthorized access
4. **Information Gathering**: Extract system information through error messages
5. **Authentication Bypass**: Access protected resources through routing manipulation
6. **Race Exploitation**: Exploit race conditions in map access
7. **Debug Information**: Extract sensitive information from debug logs
8. **Resource Exhaustion**: Cause resource exhaustion through large string inputs

## Impact Assessment

- **Confidentiality**: HIGH - Information disclosure through error messages and debug logging
- **Integrity**: HIGH - Authentication bypass could allow unauthorized modifications
- **Availability**: MEDIUM - Type assertion panics could cause service disruption
- **Authentication**: HIGH - Multiple authentication bypass vulnerabilities
- **Authorization**: HIGH - Routing manipulation could bypass access controls

This host switching system has several critical security vulnerabilities that need immediate attention.

## Technical Notes

The hostswitch system:
1. Provides virtual hosting capabilities based on hostname routing
2. Integrates with authentication middleware for access control
3. Manages subsites and their permissions
4. Handles default routing fallbacks
5. Includes utility functions for string manipulation
6. Supports path-based routing for subsites

The main security concerns revolve around input validation, authentication, and safe type handling.

## Host Switching Security Considerations

For host switching systems:
- **Header Security**: Validate all HTTP headers before use
- **Routing Security**: Implement secure routing with proper validation
- **Authentication Security**: Ensure all routes have authentication checks
- **Path Security**: Validate and sanitize path components
- **Type Security**: Use safe type assertions with error handling
- **Concurrency Security**: Protect shared resources with proper synchronization

The current implementation needs comprehensive security enhancements.

## Recommended Security Enhancements

1. **Input Security**: Comprehensive validation for all inputs and headers
2. **Type Security**: Safe type assertions with proper error handling
3. **Authentication Security**: Authentication checks for all routing paths
4. **Path Security**: Secure path handling and validation
5. **Error Security**: Sanitized error messages without sensitive information
6. **Concurrency Security**: Proper synchronization for shared map access