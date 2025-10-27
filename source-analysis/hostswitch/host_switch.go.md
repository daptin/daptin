# Security Analysis: server/hostswitch/host_switch.go

**File:** `server/hostswitch/host_switch.go`  
**Type:** HTTP host routing and authentication system  
**Lines of Code:** 140  

## Overview
This file implements a host switching mechanism that routes HTTP requests based on hostname and path, with authentication middleware integration. It provides subdomain and path-based routing with permission checking for different subsites.

## Key Components

### HostSwitch struct
**Lines:** 14-19  
**Purpose:** Core router that maps hostnames to handlers with authentication  

### ServeHTTP()
**Lines:** 38-139  
**Purpose:** Main HTTP routing logic with authentication and authorization  

## Security Analysis

### 1. Host Header Injection Vulnerability - CRITICAL
**Severity:** HIGH  
**Lines:** 42  
**Issue:** Direct use of Host header without validation for routing decisions.

```go
hostName := strings.Split(r.Host, ":")[0]
```

**Risk:**
- Host header injection attacks
- Cache poisoning through malicious Host headers
- Routing bypass through crafted Host headers
- DNS rebinding attacks

**Impact:** Request routing manipulation, potential SSRF, cache poisoning.

### 2. Type Assertion Vulnerability
**Severity:** HIGH  
**Lines:** 69, 98  
**Issue:** Unhandled type assertion that can cause application panic.

```go
user = userI.(*auth.SessionUser)
```

**Risk:** Application crash if context contains unexpected type.

### 3. Authentication Bypass Through Path Manipulation - CRITICAL
**Severity:** HIGH  
**Lines:** 50, 87  
**Issue:** Complex routing logic with potential bypass conditions.

```go
if handler := hs.HandlerMap[hostName]; handler != nil && !(len(pathParts) > 1 && constants.WellDefinedApiPaths[pathParts[1]]) {
```

**Risk:**
- Authentication bypass through path manipulation
- Access to restricted subsites through crafted URLs
- Logic flaws in routing decision tree

### 4. URL Path Traversal Risk
**Severity:** MEDIUM  
**Lines:** 106  
**Issue:** URL path reconstruction without proper validation.

```go
r.URL.Path = "/" + strings.Join(pathParts[2:], "/")
```

**Risk:**
- Path traversal attacks through manipulated URLs
- Access to unauthorized resources
- Directory traversal vulnerabilities

### 5. Inconsistent Error Responses
**Severity:** LOW  
**Lines:** 63, 82, 111  
**Issue:** Different error messages and status codes for similar authorization failures.

```go
w.Write([]byte("unauthorized"))     // Line 63
w.Write([]byte("unauthorised"))     // Line 82  
w.Write([]byte("Unauthorized"))     // Line 111
```

**Risk:**
- Information disclosure through error message variations
- Potential for enumeration attacks
- Inconsistent security responses

### 6. Default Route Security Issues
**Severity:** MEDIUM  
**Lines:** 117-138  
**Issue:** Default routing to dashboard without proper authentication checks.

**Risk:**
- Unintended access to dashboard functionality
- Potential information disclosure
- Bypass of host-specific security controls

### 7. Well-Known Path Bypass
**Severity:** MEDIUM  
**Lines:** 45-48  
**Issue:** Special handling of .well-known paths without authentication.

```go
if BeginsWithCheck(r.URL.Path, "/.well-known") {
    hs.HandlerMap["dashboard"].ServeHTTP(w, r)
    return
}
```

**Risk:**
- Bypass of normal authentication flow
- Potential exposure of sensitive .well-known resources
- Uncontrolled access to dashboard handler

## Potential Attack Vectors

### Host Header Attacks
1. **Cache Poisoning:** Send requests with malicious Host headers to poison web caches
2. **Routing Bypass:** Use crafted Host headers to access unintended handlers
3. **SSRF Attacks:** Manipulate Host header to trigger server-side requests

### Authentication Bypass
1. **Path Manipulation:** Craft URLs to bypass authentication checks
2. **Subsite Access:** Access restricted subsites through path manipulation
3. **API Path Confusion:** Exploit API path detection logic for bypass

### Route Enumeration
1. **Hostname Enumeration:** Probe different hostnames to discover available sites
2. **Path Discovery:** Use path traversal to discover hidden resources
3. **Error Analysis:** Analyze error responses to map system behavior

## Recommendations

### Immediate Actions
1. **Validate Host Headers:** Implement Host header validation against allowlist
2. **Fix Type Assertions:** Add proper error handling for type assertions
3. **Simplify Routing Logic:** Reduce complexity to prevent bypass conditions
4. **Standardize Errors:** Use consistent error responses for security failures

### Enhanced Security Implementation

```go
package hostswitch

import (
    "fmt"
    "net"
    "net/http"
    "regexp"
    "strings"
    "github.com/daptin/daptin/server/auth"
    "github.com/daptin/daptin/server/constants"
    daptinid "github.com/daptin/daptin/server/id"
    "github.com/daptin/daptin/server/subsite"
    "github.com/gin-gonic/gin"
    log "github.com/sirupsen/logrus"
)

const (
    MaxHostnameLength = 253
    MaxPathLength = 2048
)

// SecureHostSwitch provides enhanced security features
type SecureHostSwitch struct {
    HandlerMap           map[string]*gin.Engine
    SiteMap              map[string]subsite.SubSite
    AuthMiddleware       *auth.AuthMiddleware
    AdministratorGroupId daptinid.DaptinReferenceId
    AllowedHosts         map[string]bool
    DefaultHost          string
}

// ValidateHostHeader validates the Host header against security criteria
func (hs *SecureHostSwitch) ValidateHostHeader(host string) (string, error) {
    if len(host) == 0 {
        return "", fmt.Errorf("empty host header")
    }
    
    if len(host) > MaxHostnameLength {
        return "", fmt.Errorf("host header too long: %d characters", len(host))
    }
    
    // Remove port if present
    hostname, _, err := net.SplitHostPort(host)
    if err != nil {
        // If no port present, use the original host
        hostname = host
    }
    
    // Validate hostname format
    if !isValidHostname(hostname) {
        return "", fmt.Errorf("invalid hostname format: %s", hostname)
    }
    
    // Check against allowlist
    if !hs.AllowedHosts[hostname] {
        return "", fmt.Errorf("hostname not allowed: %s", hostname)
    }
    
    return hostname, nil
}

// isValidHostname validates hostname format
func isValidHostname(hostname string) bool {
    if len(hostname) == 0 || len(hostname) > 253 {
        return false
    }
    
    // Check for valid hostname pattern
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`, hostname)
    return matched
}

// ValidatePath validates and sanitizes URL path
func (hs *SecureHostSwitch) ValidatePath(path string) ([]string, error) {
    if len(path) > MaxPathLength {
        return nil, fmt.Errorf("path too long: %d characters", len(path))
    }
    
    // Check for path traversal attempts
    if strings.Contains(path, "..") {
        return nil, fmt.Errorf("path traversal attempt detected")
    }
    
    // Split and validate path parts
    pathParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
    
    for i, part := range pathParts {
        // Remove empty parts
        if part == "" {
            pathParts = append(pathParts[:i], pathParts[i+1:]...)
            continue
        }
        
        // Validate path component
        if len(part) > 255 {
            return nil, fmt.Errorf("path component too long: %s", part)
        }
        
        // Check for dangerous characters
        if strings.ContainsAny(part, "<>\"'&") {
            return nil, fmt.Errorf("dangerous characters in path: %s", part)
        }
    }
    
    return pathParts, nil
}

// SafeGetUser safely extracts user from request context
func (hs *SecureHostSwitch) SafeGetUser(r *http.Request) (*auth.SessionUser, error) {
    userI := r.Context().Value("user")
    if userI == nil {
        return &auth.SessionUser{
            UserReferenceId: daptinid.NullReferenceId,
            Groups:          auth.GroupPermissionList{},
        }, nil
    }
    
    user, ok := userI.(*auth.SessionUser)
    if !ok {
        return nil, fmt.Errorf("invalid user type in context: %T", userI)
    }
    
    return user, nil
}

// SendSecurityError sends standardized security error response
func (hs *SecureHostSwitch) SendSecurityError(w http.ResponseWriter, statusCode int, message string) {
    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(statusCode)
    w.Write([]byte(message))
    
    // Log security event
    log.Warnf("Security error: %d - %s", statusCode, message)
}

// Enhanced ServeHTTP with security improvements
func (hs *SecureHostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Validate Host header
    hostName, err := hs.ValidateHostHeader(r.Host)
    if err != nil {
        log.Warnf("Invalid host header from %s: %v", r.RemoteAddr, err)
        hs.SendSecurityError(w, http.StatusBadRequest, "Invalid host")
        return
    }
    
    // Validate and parse path
    pathParts, err := hs.ValidatePath(r.URL.Path)
    if err != nil {
        log.Warnf("Invalid path from %s: %v", r.RemoteAddr, err)
        hs.SendSecurityError(w, http.StatusBadRequest, "Invalid path")
        return
    }
    
    // Handle .well-known paths with validation
    if len(pathParts) > 0 && pathParts[0] == ".well-known" {
        if handler := hs.HandlerMap["dashboard"]; handler != nil {
            handler.ServeHTTP(w, r)
            return
        }
        hs.SendSecurityError(w, http.StatusNotFound, "Not found")
        return
    }
    
    // Check for direct hostname routing
    if handler := hs.HandlerMap[hostName]; handler != nil {
        // Check if this is an API path that should bypass hostname routing
        if len(pathParts) > 0 && constants.WellDefinedApiPaths[pathParts[0]] {
            hs.routeToDefault(w, r)
            return
        }
        
        // Handle hostname-based routing with authentication
        if err := hs.handleHostnameRoute(w, r, hostName, handler); err != nil {
            log.Errorf("Hostname routing error: %v", err)
            hs.SendSecurityError(w, http.StatusInternalServerError, "Internal error")
        }
        return
    }
    
    // Check for subsite routing
    if len(pathParts) > 0 && !constants.WellDefinedApiPaths[pathParts[0]] {
        if err := hs.handleSubsiteRoute(w, r, pathParts); err != nil {
            log.Errorf("Subsite routing error: %v", err)
            hs.SendSecurityError(w, http.StatusForbidden, "Access denied")
        }
        return
    }
    
    // Default routing
    hs.routeToDefault(w, r)
}

// handleHostnameRoute handles hostname-based routing with authentication
func (hs *SecureHostSwitch) handleHostnameRoute(w http.ResponseWriter, r *http.Request, hostName string, handler *gin.Engine) error {
    // Apply authentication middleware
    ok, abort, modifiedRequest := hs.AuthMiddleware.AuthCheckMiddlewareWithHttp(r, w, true)
    if ok {
        r = modifiedRequest
    }
    
    if abort {
        w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, hostName))
        hs.SendSecurityError(w, http.StatusUnauthorized, "Authentication required")
        return nil
    }
    
    // Get and validate user
    user, err := hs.SafeGetUser(r)
    if err != nil {
        return fmt.Errorf("failed to get user: %v", err)
    }
    
    // Check subsite permissions
    subSite, exists := hs.SiteMap[hostName]
    if !exists {
        return fmt.Errorf("subsite not found: %s", hostName)
    }
    
    if !subSite.Permission.CanExecute(user.UserReferenceId, user.Groups, hs.AdministratorGroupId) {
        w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, hostName))
        hs.SendSecurityError(w, http.StatusUnauthorized, "Insufficient permissions")
        return nil
    }
    
    handler.ServeHTTP(w, r)
    return nil
}

// handleSubsiteRoute handles subsite routing
func (hs *SecureHostSwitch) handleSubsiteRoute(w http.ResponseWriter, r *http.Request, pathParts []string) error {
    if len(pathParts) == 0 {
        return fmt.Errorf("empty path parts")
    }
    
    firstSubFolder := pathParts[0]
    subSite, exists := hs.SiteMap[firstSubFolder]
    if !exists {
        hs.routeToDefault(w, r)
        return nil
    }
    
    // Get and validate user
    user, err := hs.SafeGetUser(r)
    if err != nil {
        return fmt.Errorf("failed to get user: %v", err)
    }
    
    // Check permissions
    if !subSite.Permission.CanExecute(user.UserReferenceId, user.Groups, hs.AdministratorGroupId) {
        hs.SendSecurityError(w, http.StatusForbidden, "Access denied")
        return nil
    }
    
    // Reconstruct path safely
    if len(pathParts) > 1 {
        r.URL.Path = "/" + strings.Join(pathParts[1:], "/")
    } else {
        r.URL.Path = "/"
    }
    
    handler := hs.HandlerMap[subSite.Hostname]
    if handler == nil {
        return fmt.Errorf("handler not found for hostname: %s", subSite.Hostname)
    }
    
    handler.ServeHTTP(w, r)
    return nil
}

// routeToDefault routes to default handler
func (hs *SecureHostSwitch) routeToDefault(w http.ResponseWriter, r *http.Request) {
    handler := hs.HandlerMap[hs.DefaultHost]
    if handler == nil {
        handler = hs.HandlerMap["dashboard"]
    }
    
    if handler == nil {
        log.Error("No default handler available")
        hs.SendSecurityError(w, http.StatusNotFound, "Not found")
        return
    }
    
    handler.ServeHTTP(w, r)
}
```

### Long-term Improvements
1. **Host Allowlisting:** Implement comprehensive host validation
2. **Rate Limiting:** Add rate limiting per hostname and IP
3. **Security Headers:** Add security headers for all responses
4. **Audit Logging:** Log all routing decisions and security events
5. **Path Validation:** Implement comprehensive path validation

## Edge Cases Identified

1. **Empty Host Headers:** Requests without Host header
2. **IPv6 Addresses:** Host headers with IPv6 addresses
3. **International Domain Names:** IDN hostnames with special characters
4. **Very Long Paths:** URLs exceeding normal length limits
5. **Port Variations:** Different port numbers in Host header
6. **Case Sensitivity:** Uppercase/lowercase hostname variations
7. **Malformed URLs:** URLs with invalid encoding or structure
8. **Concurrent Requests:** Thread safety with shared handler maps

## Security Best Practices Violations

1. **No Host header validation**
2. **Unhandled type assertions**
3. **Complex routing logic**
4. **Inconsistent error handling**
5. **No input sanitization**
6. **Missing security headers**

## Critical Issues Summary

1. **Host Header Injection:** Direct use of Host header for routing
2. **Authentication Bypass:** Complex logic with potential bypass conditions
3. **Type Assertion Panics:** Application crash risks
4. **Path Traversal:** URL path reconstruction without validation
5. **Inconsistent Security:** Different error responses and logic paths

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - Host header injection and authentication bypass vulnerabilities require immediate attention