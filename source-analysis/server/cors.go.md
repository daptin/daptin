# Security Analysis: server/cors.go

**File:** `server/cors.go`  
**Type:** CORS (Cross-Origin Resource Sharing) middleware implementation  
**Lines of Code:** 97  

## Overview
This file implements CORS middleware for handling cross-origin requests in the Daptin server. It provides a configurable CORS implementation with support for origin validation, method restrictions, header controls, and credential handling. However, the current implementation has several critical security vulnerabilities.

## Key Components

### CorsMiddleware struct
**Lines:** 16-50  
**Purpose:** Configurable CORS middleware with validation and control options  

### NewCorsMiddleware function
**Lines:** 52-66  
**Purpose:** Constructor for CORS middleware with default values  

### CorsMiddlewareFunc method
**Lines:** 68-83  
**Purpose:** Main middleware function that handles CORS headers  

### CorsInfo struct
**Lines:** 85-96  
**Purpose:** Data structure for CORS request information  

## Security Analysis

### 1. CRITICAL: Overly Permissive CORS Configuration - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 71-73, 76, 79  
**Issue:** Extremely permissive CORS settings that allow any origin and credentials.

```go
c.Header("Access-Control-Allow-Origin", c.Request.Header.Get("Origin"))  // Reflects any origin
c.Header("Access-Control-Allow-Methods", "*")                           // Allows all methods
c.Header("Access-Control-Allow-Credentials", "true")                    // Always allows credentials
c.Header("Access-Control-Allow-Headers", "*")                          // Allows all headers
```

**Risk:**
- **Complete CORS bypass** allowing any website to make authenticated requests
- **Credential theft** through malicious cross-origin requests
- **CSRF attacks** via unrestricted cross-origin access
- **Data exfiltration** from authenticated sessions

### 2. CRITICAL: Unsafe Origin Reflection - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 71  
**Issue:** Origin header value directly reflected without validation.

```go
c.Header("Access-Control-Allow-Origin", c.Request.Header.Get("Origin"))
```

**Risk:**
- **Origin spoofing** allowing bypass of same-origin policy
- **Malicious origin reflection** enabling cross-site attacks
- **Authentication bypass** through origin manipulation
- **Complete CORS security model circumvention**

### 3. HIGH: Unused Security Configuration - HIGH RISK
**Severity:** HIGH  
**Lines:** 28, 34, 40, 46  
**Issue:** Security configuration options defined but not implemented in the middleware function.

```go
OriginValidator func(origin string, request *http.Request) bool  // Not used
AllowedMethods []string                                          // Not used
AllowedHeaders []string                                          // Not used
AccessControlAllowCredentials bool                               // Not used
```

**Risk:**
- **False security impression** from unused validation options
- **Configuration drift** between intended and actual security
- **Maintenance complexity** with dead code
- **Security misunderstanding** by developers

### 4. HIGH: Missing Input Validation - HIGH RISK
**Severity:** HIGH  
**Lines:** 71, 76  
**Issue:** HTTP headers used without validation or sanitization.

```go
c.Request.Header.Get("Origin")                              // No validation
c.Request.Header.Get("Access-Control-Request-Headers")     // No validation
```

**Risk:**
- **Header injection** through malicious header values
- **Response splitting** via crafted header content
- **Cache poisoning** through header manipulation
- **Protocol confusion** from malformed headers

### 5. MEDIUM: Inconsistent Header Handling - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 76, 79  
**Issue:** Different header handling logic for OPTIONS vs other methods.

```go
if c.Request.Method == "OPTIONS" {
    c.Header("Access-Control-Allow-Headers", c.Request.Header.Get("Access-Control-Request-Headers"))
} else {
    c.Header("Access-Control-Allow-Headers", "*")
}
```

**Risk:**
- **Inconsistent security policies** between preflight and actual requests
- **Security bypass** through method manipulation
- **Confusion attacks** exploiting different behaviors
- **Policy enforcement gaps** in CORS implementation

### 6. LOW: Missing Security Headers - LOW RISK
**Severity:** LOW  
**Lines:** All middleware function  
**Issue:** No implementation of security-related CORS headers like max-age.

```go
// Missing Access-Control-Max-Age implementation
// Missing proper Vary header handling
```

**Risk:**
- **Cache poisoning** from missing Vary headers
- **Performance impact** from repeated preflight requests
- **Browser security** feature underutilization
- **Incomplete CORS implementation**

## Potential Attack Vectors

### Cross-Site Request Forgery (CSRF)
1. **Credential-Enabled CSRF:** Exploit always-true credentials flag for authenticated attacks
2. **Origin Spoofing:** Bypass origin checks through header manipulation
3. **Method Exploitation:** Use unrestricted method access for state-changing operations
4. **Header Manipulation:** Inject malicious headers through permissive header policies

### Data Exfiltration Attacks
1. **Cross-Origin Data Theft:** Extract sensitive data through permissive CORS
2. **Authentication Bypass:** Use reflected origins to bypass authentication
3. **Session Hijacking:** Steal session data through cross-origin requests
4. **API Abuse:** Access protected APIs from malicious origins

### Browser Security Bypass
1. **Same-Origin Policy Bypass:** Circumvent browser security through CORS misconfiguration
2. **Content Security Policy Bypass:** Use CORS to bypass CSP restrictions
3. **Mixed Content Bypass:** Exploit CORS for mixed content attacks
4. **XSS Amplification:** Use CORS to amplify XSS attack impact

### Cache and Response Manipulation
1. **Response Splitting:** Inject malicious content through header manipulation
2. **Cache Poisoning:** Corrupt shared caches through header injection
3. **Protocol Confusion:** Exploit inconsistent header handling
4. **Browser Cache Abuse:** Manipulate browser caching behavior

## Recommendations

### Immediate Actions
1. **Implement Origin Validation:** Use proper origin validation instead of reflection
2. **Restrict Methods:** Limit allowed methods to only what's necessary
3. **Validate Headers:** Implement proper header validation and sanitization
4. **Remove Credential Flag:** Disable credentials unless absolutely necessary

### Enhanced Security Implementation

```go
package server

import (
    "fmt"
    "net/http"
    "net/url"
    "regexp"
    "strings"
    "time"
    
    "github.com/gin-gonic/gin"
    log "github.com/sirupsen/logrus"
)

const (
    maxOriginLength = 253  // Maximum domain name length
    maxHeaderLength = 8192 // Maximum header value length
)

var (
    // Safe origin patterns
    validOriginPattern = regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+(?::[0-9]+)?$`)
    
    // Default allowed methods (restrict to necessary methods only)
    defaultAllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
    
    // Default allowed headers (restrict to necessary headers only)
    defaultAllowedHeaders = []string{
        "Accept", "Accept-Language", "Content-Language", "Content-Type",
        "Authorization", "X-Requested-With", "X-Request-ID",
    }
    
    // Dangerous headers to never allow
    dangerousHeaders = map[string]bool{
        "host":               true,
        "origin":             true,
        "referer":            true,
        "cookie":             true,
        "set-cookie":         true,
        "x-forwarded-for":    true,
        "x-forwarded-host":   true,
        "x-forwarded-proto":  true,
    }
)

// SecureCorsMiddleware provides a security-focused CORS implementation
type SecureCorsMiddleware struct {
    allowedOrigins        map[string]bool
    allowedMethods        map[string]bool
    allowedMethodsCsv     string
    allowedHeaders        map[string]bool
    allowedHeadersCsv     string
    
    // Security settings
    AllowCredentials      bool
    MaxAge                int
    AllowedOrigins        []string
    AllowedMethods        []string
    AllowedHeaders        []string
    ExposeHeaders         []string
    
    // Validation functions
    OriginValidator       func(origin string) bool
    
    // Security options
    LogSuspiciousRequests bool
    StrictValidation      bool
}

// validateOrigin safely validates an origin string
func validateOrigin(origin string) error {
    if origin == "" {
        return fmt.Errorf("origin is empty")
    }
    
    if len(origin) > maxOriginLength {
        return fmt.Errorf("origin too long: %d characters", len(origin))
    }
    
    if !validOriginPattern.MatchString(origin) {
        return fmt.Errorf("origin has invalid format")
    }
    
    // Parse URL to validate structure
    parsedURL, err := url.Parse(origin)
    if err != nil {
        return fmt.Errorf("origin is not a valid URL: %v", err)
    }
    
    // Security checks
    if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
        return fmt.Errorf("origin has invalid scheme: %s", parsedURL.Scheme)
    }
    
    // Check for dangerous patterns
    host := strings.ToLower(parsedURL.Host)
    dangerousPatterns := []string{
        "localhost", "127.0.0.1", "0.0.0.0", "::1",
        "example.com", "test.com", "invalid",
    }
    
    for _, pattern := range dangerousPatterns {
        if strings.Contains(host, pattern) {
            return fmt.Errorf("origin contains dangerous pattern: %s", pattern)
        }
    }
    
    return nil
}

// validateHeader safely validates header names and values
func validateHeader(name, value string) error {
    if name == "" {
        return fmt.Errorf("header name is empty")
    }
    
    if len(value) > maxHeaderLength {
        return fmt.Errorf("header value too long: %d characters", len(value))
    }
    
    // Check for dangerous headers
    if dangerousHeaders[strings.ToLower(name)] {
        return fmt.Errorf("header is dangerous: %s", name)
    }
    
    // Check for header injection attempts
    if strings.Contains(value, "\n") || strings.Contains(value, "\r") {
        return fmt.Errorf("header contains line breaks")
    }
    
    return nil
}

// NewSecureCorsMiddleware creates a secure CORS middleware with safe defaults
func NewSecureCorsMiddleware() *SecureCorsMiddleware {
    return &SecureCorsMiddleware{
        allowedOrigins:        make(map[string]bool),
        allowedMethods:        make(map[string]bool),
        allowedHeaders:        make(map[string]bool),
        AllowCredentials:      false, // Default to false for security
        MaxAge:                300,   // 5 minutes
        AllowedMethods:        defaultAllowedMethods,
        AllowedHeaders:        defaultAllowedHeaders,
        LogSuspiciousRequests: true,
        StrictValidation:      true,
    }
}

// SetAllowedOrigins configures allowed origins with validation
func (cm *SecureCorsMiddleware) SetAllowedOrigins(origins []string) error {
    cm.allowedOrigins = make(map[string]bool)
    
    for _, origin := range origins {
        if err := validateOrigin(origin); err != nil {
            return fmt.Errorf("invalid origin %s: %v", origin, err)
        }
        cm.allowedOrigins[origin] = true
    }
    
    cm.AllowedOrigins = origins
    return nil
}

// SetAllowedMethods configures allowed HTTP methods
func (cm *SecureCorsMiddleware) SetAllowedMethods(methods []string) {
    cm.allowedMethods = make(map[string]bool)
    validMethods := make([]string, 0)
    
    for _, method := range methods {
        upperMethod := strings.ToUpper(method)
        cm.allowedMethods[upperMethod] = true
        validMethods = append(validMethods, upperMethod)
    }
    
    cm.AllowedMethods = validMethods
    cm.allowedMethodsCsv = strings.Join(validMethods, ", ")
}

// SetAllowedHeaders configures allowed headers with validation
func (cm *SecureCorsMiddleware) SetAllowedHeaders(headers []string) error {
    cm.allowedHeaders = make(map[string]bool)
    validHeaders := make([]string, 0)
    
    for _, header := range headers {
        if err := validateHeader(header, ""); err != nil {
            return fmt.Errorf("invalid header %s: %v", header, err)
        }
        
        canonicalHeader := http.CanonicalHeaderKey(header)
        cm.allowedHeaders[canonicalHeader] = true
        validHeaders = append(validHeaders, canonicalHeader)
    }
    
    cm.AllowedHeaders = validHeaders
    cm.allowedHeadersCsv = strings.Join(validHeaders, ", ")
    return nil
}

// isOriginAllowed checks if an origin is allowed
func (cm *SecureCorsMiddleware) isOriginAllowed(origin string) bool {
    if origin == "" {
        return false
    }
    
    // Validate origin format first
    if err := validateOrigin(origin); err != nil {
        if cm.LogSuspiciousRequests {
            log.Warnf("Invalid origin format: %s - %v", origin, err)
        }
        return false
    }
    
    // Check custom validator first
    if cm.OriginValidator != nil {
        return cm.OriginValidator(origin)
    }
    
    // Check allowed origins list
    return cm.allowedOrigins[origin]
}

// isMethodAllowed checks if a method is allowed
func (cm *SecureCorsMiddleware) isMethodAllowed(method string) bool {
    if method == "" {
        return false
    }
    return cm.allowedMethods[strings.ToUpper(method)]
}

// areHeadersAllowed checks if requested headers are allowed
func (cm *SecureCorsMiddleware) areHeadersAllowed(headers []string) bool {
    for _, header := range headers {
        canonical := http.CanonicalHeaderKey(strings.TrimSpace(header))
        if !cm.allowedHeaders[canonical] {
            return false
        }
    }
    return true
}

// SecureCorsMiddlewareFunc provides secure CORS handling
func (cm *SecureCorsMiddleware) SecureCorsMiddlewareFunc(c *gin.Context) {
    origin := c.Request.Header.Get("Origin")
    
    // Skip CORS processing for same-origin requests
    if origin == "" {
        return
    }
    
    // Validate and check origin
    if !cm.isOriginAllowed(origin) {
        if cm.LogSuspiciousRequests {
            log.Warnf("CORS request from disallowed origin: %s", origin)
        }
        c.AbortWithStatusJSON(403, gin.H{"error": "origin not allowed"})
        return
    }
    
    // Set Vary header for caching
    c.Header("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
    
    // Set allowed origin (specific, not reflected)
    c.Header("Access-Control-Allow-Origin", origin)
    
    // Handle preflight requests
    if c.Request.Method == "OPTIONS" {
        requestMethod := c.Request.Header.Get("Access-Control-Request-Method")
        requestHeaders := c.Request.Header.Get("Access-Control-Request-Headers")
        
        // Validate requested method
        if requestMethod != "" && !cm.isMethodAllowed(requestMethod) {
            if cm.LogSuspiciousRequests {
                log.Warnf("CORS preflight with disallowed method: %s", requestMethod)
            }
            c.AbortWithStatusJSON(405, gin.H{"error": "method not allowed"})
            return
        }
        
        // Validate requested headers
        if requestHeaders != "" {
            headerList := strings.Split(requestHeaders, ",")
            for i, header := range headerList {
                headerList[i] = strings.TrimSpace(header)
            }
            
            if !cm.areHeadersAllowed(headerList) {
                if cm.LogSuspiciousRequests {
                    log.Warnf("CORS preflight with disallowed headers: %s", requestHeaders)
                }
                c.AbortWithStatusJSON(400, gin.H{"error": "headers not allowed"})
                return
            }
            
            // Return only the requested headers that are allowed
            c.Header("Access-Control-Allow-Headers", cm.allowedHeadersCsv)
        } else {
            c.Header("Access-Control-Allow-Headers", cm.allowedHeadersCsv)
        }
        
        // Set allowed methods
        c.Header("Access-Control-Allow-Methods", cm.allowedMethodsCsv)
        
        // Set max age for preflight caching
        if cm.MaxAge > 0 {
            c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", cm.MaxAge))
        }
        
        // Set credentials if allowed
        if cm.AllowCredentials {
            c.Header("Access-Control-Allow-Credentials", "true")
        }
        
        c.AbortWithStatus(204) // No Content for preflight
        return
    }
    
    // For actual requests, set response headers
    if cm.AllowCredentials {
        c.Header("Access-Control-Allow-Credentials", "true")
    }
    
    // Set exposed headers
    if len(cm.ExposeHeaders) > 0 {
        c.Header("Access-Control-Expose-Headers", strings.Join(cm.ExposeHeaders, ", "))
    }
    
    // Log successful CORS request
    log.Debugf("CORS request allowed: origin=%s, method=%s", origin, c.Request.Method)
}

// DefaultSecureCorsMiddleware creates a CORS middleware with secure defaults for development
func DefaultSecureCorsMiddleware() *SecureCorsMiddleware {
    middleware := NewSecureCorsMiddleware()
    
    // Set secure defaults - customize based on your needs
    middleware.SetAllowedOrigins([]string{
        "https://localhost:3000",  // Common dev server
        "https://127.0.0.1:3000", // Alternative dev server
    })
    
    middleware.SetAllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
    
    middleware.SetAllowedHeaders([]string{
        "Accept", "Accept-Language", "Content-Language", "Content-Type",
        "Authorization", "X-Requested-With", "X-Request-ID",
    })
    
    middleware.AllowCredentials = false // Start with false, enable only if needed
    middleware.MaxAge = 300             // 5 minutes
    
    return middleware
}

// Production CORS middleware - very restrictive
func ProductionSecureCorsMiddleware(allowedOrigins []string) *SecureCorsMiddleware {
    middleware := NewSecureCorsMiddleware()
    
    if err := middleware.SetAllowedOrigins(allowedOrigins); err != nil {
        log.Fatalf("Failed to set allowed origins: %v", err)
    }
    
    middleware.SetAllowedMethods([]string{"GET", "POST", "PUT", "DELETE"}) // No OPTIONS in production
    
    middleware.SetAllowedHeaders([]string{
        "Accept", "Content-Type", "Authorization", "X-Request-ID",
    })
    
    middleware.AllowCredentials = true // Enable for authenticated requests
    middleware.MaxAge = 86400          // 24 hours for production
    middleware.LogSuspiciousRequests = true
    middleware.StrictValidation = true
    
    return middleware
}
```

### Long-term Improvements
1. **Origin Whitelist Management:** Implement dynamic origin whitelist management
2. **Rate Limiting:** Add rate limiting for CORS preflight requests
3. **Monitoring and Alerting:** Implement comprehensive CORS security monitoring
4. **Policy Templates:** Create policy templates for different security levels
5. **Configuration Management:** Add runtime configuration management for CORS policies

## Edge Cases Identified

1. **Missing Origin Header:** Requests without Origin header in CORS contexts
2. **Malformed Origins:** Origins with invalid URL formats or dangerous patterns
3. **Header Injection:** Attempts to inject headers through CORS request headers
4. **Protocol Mismatch:** HTTPS/HTTP origin mismatches
5. **Port Variations:** Same origin with different ports
6. **Subdomain Handling:** Subdomain vs main domain CORS policies
7. **Case Sensitivity:** Header name case variations
8. **Unicode Origins:** Origins with international domain names
9. **IP Address Origins:** Origins using IP addresses instead of domain names
10. **Null Origins:** Requests with null or undefined origins

## Security Best Practices Violations

1. **Overly permissive CORS configuration** allowing any origin and credentials
2. **Unsafe origin reflection** without validation
3. **Unused security configuration** creating false security impression
4. **Missing input validation** for HTTP headers
5. **Inconsistent header handling** between request types
6. **Missing security headers** like Vary and proper max-age
7. **No origin whitelist enforcement**
8. **No method restriction implementation**
9. **No header validation or restriction**
10. **Complete bypass of CORS security model**

## Positive Security Aspects

1. **Configurable design** allowing for proper security implementation
2. **Support for validation functions** in the structure design
3. **Separation of concerns** with dedicated CORS handling

## Critical Issues Summary

1. **Overly Permissive CORS Configuration:** Allows any origin with credentials
2. **Unsafe Origin Reflection:** Origin header directly reflected without validation
3. **Unused Security Configuration:** Security options defined but not implemented
4. **Missing Input Validation:** HTTP headers used without validation
5. **Inconsistent Header Handling:** Different logic for OPTIONS vs other methods
6. **Missing Security Headers:** No proper Vary or max-age implementation

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - CORS implementation with complete security bypass vulnerabilities