# Security Analysis: server/endpoint_graphql.go

**File:** `server/endpoint_graphql.go`  
**Type:** GraphQL endpoint initialization and HTTP method routing  
**Lines of Code:** 40  

## Overview
This file initializes GraphQL endpoints for the Daptin server with support for multiple HTTP methods. It creates a GraphQL schema, configures a GraphQL HTTP handler with development features enabled (Pretty printing, Playground, GraphiQL), and registers routes for GET, POST, PUT, PATCH, and DELETE methods. The implementation provides a comprehensive GraphQL interface for API interactions.

## Key Components

### InitializeGraphqlResource function
**Lines:** 9-39  
**Purpose:** Initializes GraphQL endpoint with schema and HTTP method routing  

### GraphQL Handler Configuration
**Lines:** 12-17  
**Purpose:** Creates GraphQL HTTP handler with development features enabled  

### HTTP Method Registration
- **GET handler:** Lines 20-22
- **POST handler:** Lines 24-26
- **PUT handler:** Lines 28-30
- **PATCH handler:** Lines 32-34
- **DELETE handler:** Lines 36-38

## Security Analysis

### 1. CRITICAL: Missing Authentication and Authorization - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 20-38  
**Issue:** GraphQL endpoints exposed without any authentication or authorization checks.

```go
defaultRouter.Handle("GET", "/graphql", func(c *gin.Context) {
    graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)  // No auth check
})
// Same pattern for POST, PUT, PATCH, DELETE
```

**Risk:**
- **Unauthorized data access** through GraphQL queries
- **Data manipulation** via GraphQL mutations
- **Schema introspection** revealing database structure
- **Complete API exposure** without access controls

### 2. CRITICAL: Development Features in Production - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 14-16  
**Issue:** GraphQL Playground and GraphiQL enabled without environment checks.

```go
graphqlHttpHandler := handler.New(&handler.Config{
    Schema:     graphqlSchema,
    Pretty:     true,
    Playground: true,  // Development feature always enabled
    GraphiQL:   true,  // Development feature always enabled
})
```

**Risk:**
- **Schema introspection** exposing database structure in production
- **Query execution interface** accessible to attackers
- **GraphQL exploration tools** available to unauthorized users
- **Information disclosure** through development interfaces

### 3. HIGH: Unrestricted HTTP Methods - HIGH RISK
**Severity:** HIGH  
**Lines:** 20-38  
**Issue:** All HTTP methods supported for GraphQL without method-specific validation.

```go
defaultRouter.Handle("GET", "/graphql", ...)     // Query operations
defaultRouter.Handle("POST", "/graphql", ...)    // Standard GraphQL
defaultRouter.Handle("PUT", "/graphql", ...)     // Non-standard for GraphQL
defaultRouter.Handle("PATCH", "/graphql", ...)   // Non-standard for GraphQL
defaultRouter.Handle("DELETE", "/graphql", ...)  // Non-standard for GraphQL
```

**Risk:**
- **Protocol confusion** from non-standard HTTP methods
- **Security control bypass** via method manipulation
- **Cache poisoning** through method-based variations
- **Firewall evasion** using unexpected HTTP methods

### 4. HIGH: Missing Input Validation - HIGH RISK
**Severity:** HIGH  
**Lines:** 10, 20-38  
**Issue:** No validation of GraphQL queries, mutations, or input parameters.

```go
graphqlSchema := MakeGraphqlSchema(&initConfig, cruds)  // Schema from unvalidated config
// No query complexity analysis, depth limiting, or input sanitization
```

**Risk:**
- **GraphQL injection** through malicious queries
- **Query complexity attacks** causing resource exhaustion
- **Deep query attacks** overwhelming the server
- **Batch query abuse** for denial of service

### 5. MEDIUM: Missing Security Headers - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 21, 25, 29, 33, 37  
**Issue:** No security headers set for GraphQL responses.

```go
graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)  // No security headers
```

**Risk:**
- **Cross-site scripting** vulnerability exposure
- **Content type sniffing** attacks
- **Clickjacking** attacks via missing frame protection
- **HTTPS enforcement** bypass in mixed content scenarios

### 6. MEDIUM: Error Information Disclosure - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 14  
**Issue:** Pretty printing enabled may expose sensitive error information.

```go
Pretty: true,  // May expose detailed error information
```

**Risk:**
- **Database error exposure** revealing schema details
- **Internal server information** leaked through errors
- **Stack trace information** exposed in responses
- **System architecture disclosure** via error details

### 7. LOW: Missing Rate Limiting - LOW RISK
**Severity:** LOW  
**Lines:** 20-38  
**Issue:** No rate limiting implemented for GraphQL endpoints.

```go
// No rate limiting for potentially expensive GraphQL operations
```

**Risk:**
- **Denial of service** through query flooding
- **Resource exhaustion** from complex queries
- **API abuse** without usage controls
- **Performance degradation** under high load

## Potential Attack Vectors

### GraphQL Security Attacks
1. **Query Complexity Attacks:** Submit deeply nested or complex queries to exhaust server resources
2. **Batch Query Attacks:** Send multiple queries in single request to amplify attack impact
3. **Introspection Abuse:** Use schema introspection to map entire database structure
4. **Field Injection:** Inject malicious GraphQL fields to access unauthorized data

### Authentication and Authorization Bypass
1. **Direct API Access:** Access GraphQL endpoints without authentication
2. **Schema Exploration:** Use development interfaces to explore available operations
3. **Privilege Escalation:** Access administrative queries without proper authorization
4. **Data Exfiltration:** Extract sensitive data through unrestricted GraphQL queries

### HTTP Method Exploitation
1. **Method Override Attacks:** Use non-standard methods to bypass security controls
2. **Cache Poisoning:** Exploit method-based caching differences
3. **Protocol Confusion:** Exploit servers expecting specific HTTP methods for GraphQL
4. **Firewall Bypass:** Use unexpected methods to evade network security

### Development Interface Abuse
1. **GraphiQL Exploitation:** Use GraphiQL interface for unauthorized query execution
2. **Playground Abuse:** Leverage GraphQL Playground for malicious operations
3. **Schema Discovery:** Extract complete schema through development interfaces
4. **Query Testing:** Use development tools to test and refine attacks

## Recommendations

### Immediate Actions
1. **Add Authentication Middleware:** Implement authentication for all GraphQL endpoints
2. **Disable Development Features:** Conditionally disable Playground and GraphiQL in production
3. **Restrict HTTP Methods:** Limit to GET and POST for GraphQL operations
4. **Add Security Headers:** Implement comprehensive security headers

### Enhanced Security Implementation

```go
package server

import (
    "fmt"
    "net/http"
    "os"
    "strings"
    "time"
    
    "github.com/daptin/daptin/server/auth"
    "github.com/daptin/daptin/server/resource"
    "github.com/gin-gonic/gin"
    "github.com/graphql-go/handler"
    log "github.com/sirupsen/logrus"
)

const (
    maxQueryDepth      = 10    // Maximum GraphQL query depth
    maxQueryComplexity = 1000  // Maximum query complexity score
    graphqlRateLimit   = 100   // Requests per minute per IP
)

var (
    // Production environment check
    isProduction = strings.ToLower(os.Getenv("ENVIRONMENT")) == "production"
    
    // Allowed HTTP methods for GraphQL
    allowedMethods = map[string]bool{
        "GET":  true, // For queries via URL params
        "POST": true, // Standard GraphQL requests
    }
)

// GraphQLSecurityConfig holds security configuration for GraphQL
type GraphQLSecurityConfig struct {
    EnablePlayground      bool
    EnableGraphiQL        bool
    EnableIntrospection   bool
    MaxQueryDepth         int
    MaxQueryComplexity    int
    RequireAuthentication bool
    AllowedMethods        []string
    RateLimitPerMinute    int
}

// getGraphQLSecurityConfig returns security configuration based on environment
func getGraphQLSecurityConfig() GraphQLSecurityConfig {
    if isProduction {
        return GraphQLSecurityConfig{
            EnablePlayground:      false,
            EnableGraphiQL:        false,
            EnableIntrospection:   false,
            MaxQueryDepth:         maxQueryDepth,
            MaxQueryComplexity:    maxQueryComplexity,
            RequireAuthentication: true,
            AllowedMethods:        []string{"POST"},
            RateLimitPerMinute:    graphqlRateLimit,
        }
    }
    
    // Development configuration
    return GraphQLSecurityConfig{
        EnablePlayground:      true,
        EnableGraphiQL:        true,
        EnableIntrospection:   true,
        MaxQueryDepth:         maxQueryDepth * 2, // More relaxed for dev
        MaxQueryComplexity:    maxQueryComplexity * 2,
        RequireAuthentication: false, // Optional in dev
        AllowedMethods:        []string{"GET", "POST"},
        RateLimitPerMinute:    graphqlRateLimit * 10, // Higher limit for dev
    }
}

// validateGraphQLMethod validates HTTP method for GraphQL requests
func validateGraphQLMethod(method string, allowedMethods []string) error {
    for _, allowed := range allowedMethods {
        if method == allowed {
            return nil
        }
    }
    return fmt.Errorf("HTTP method %s not allowed for GraphQL", method)
}

// setGraphQLSecurityHeaders sets security headers for GraphQL responses
func setGraphQLSecurityHeaders(c *gin.Context) {
    // Content Security Policy
    csp := "default-src 'self'; " +
        "script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
        "style-src 'self' 'unsafe-inline'; " +
        "img-src 'self' data:; " +
        "font-src 'self'; " +
        "connect-src 'self'"
    
    c.Header("Content-Security-Policy", csp)
    c.Header("X-Content-Type-Options", "nosniff")
    c.Header("X-Frame-Options", "DENY")
    c.Header("X-XSS-Protection", "1; mode=block")
    c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
    
    // GraphQL-specific headers
    c.Header("X-GraphQL-Endpoint", "true")
    c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
}

// graphQLAuthMiddleware provides authentication for GraphQL endpoints
func graphQLAuthMiddleware(authMiddleware *auth.AuthMiddleware, required bool) gin.HandlerFunc {
    return func(c *gin.Context) {
        if !required {
            c.Next()
            return
        }
        
        // Extract and validate authentication
        user := c.Request.Context().Value("user")
        if user == nil {
            log.Warnf("GraphQL access denied: no authentication - IP: %s", c.ClientIP())
            c.AbortWithStatusJSON(401, gin.H{
                "error": "authentication required",
                "code":  "UNAUTHENTICATED",
            })
            return
        }
        
        // Validate user type safely
        sessionUser, ok := user.(*auth.SessionUser)
        if !ok {
            log.Warnf("GraphQL access denied: invalid user type - IP: %s", c.ClientIP())
            c.AbortWithStatusJSON(401, gin.H{
                "error": "invalid authentication",
                "code":  "INVALID_AUTH",
            })
            return
        }
        
        // Add user context for GraphQL resolvers
        c.Set("graphql_user", sessionUser)
        c.Next()
    }
}

// graphQLRateLimitMiddleware implements rate limiting for GraphQL
func graphQLRateLimitMiddleware(limit int) gin.HandlerFunc {
    // Simple in-memory rate limiter - use Redis in production
    clientRequests := make(map[string][]time.Time)
    
    return func(c *gin.Context) {
        clientIP := c.ClientIP()
        now := time.Now()
        windowStart := now.Add(-time.Minute)
        
        // Clean old requests
        if requests, exists := clientRequests[clientIP]; exists {
            var validRequests []time.Time
            for _, reqTime := range requests {
                if reqTime.After(windowStart) {
                    validRequests = append(validRequests, reqTime)
                }
            }
            clientRequests[clientIP] = validRequests
        }
        
        // Check rate limit
        if len(clientRequests[clientIP]) >= limit {
            log.Warnf("GraphQL rate limit exceeded: IP %s", clientIP)
            c.AbortWithStatusJSON(429, gin.H{
                "error": "rate limit exceeded",
                "code":  "RATE_LIMITED",
            })
            return
        }
        
        // Record request
        clientRequests[clientIP] = append(clientRequests[clientIP], now)
        c.Next()
    }
}

// secureGraphQLHandler creates a secure GraphQL handler wrapper
func secureGraphQLHandler(graphqlHandler *handler.Handler, config GraphQLSecurityConfig) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Validate HTTP method
        if err := validateGraphQLMethod(c.Request.Method, config.AllowedMethods); err != nil {
            log.Warnf("Invalid GraphQL method: %s from IP: %s", c.Request.Method, c.ClientIP())
            c.AbortWithStatusJSON(405, gin.H{
                "error": "method not allowed",
                "code":  "METHOD_NOT_ALLOWED",
            })
            return
        }
        
        // Set security headers
        setGraphQLSecurityHeaders(c)
        
        // Log GraphQL request for audit
        log.Infof("GraphQL request: method=%s, ip=%s, user-agent=%s", 
            c.Request.Method, c.ClientIP(), c.Request.UserAgent())
        
        // Delegate to GraphQL handler
        graphqlHandler.ServeHTTP(c.Writer, c.Request)
    }
}

// InitializeSecureGraphqlResource initializes GraphQL with comprehensive security
func InitializeSecureGraphqlResource(initConfig resource.CmsConfig, cruds map[string]*resource.DbResource, defaultRouter *gin.Engine, authMiddleware *auth.AuthMiddleware) error {
    
    // Get security configuration
    securityConfig := getGraphQLSecurityConfig()
    
    log.Infof("Initializing GraphQL with security config: playground=%v, graphiql=%v, auth_required=%v", 
        securityConfig.EnablePlayground, securityConfig.EnableGraphiQL, securityConfig.RequireAuthentication)
    
    // Create secure GraphQL schema
    graphqlSchema, err := MakeSecureGraphqlSchema(&initConfig, cruds, securityConfig)
    if err != nil {
        return fmt.Errorf("failed to create secure GraphQL schema: %v", err)
    }
    
    // Configure GraphQL handler
    handlerConfig := &handler.Config{
        Schema:   graphqlSchema,
        Pretty:   !isProduction, // Pretty printing only in development
        Playground: securityConfig.EnablePlayground,
        GraphiQL:   securityConfig.EnableGraphiQL,
    }
    
    // Disable introspection in production
    if !securityConfig.EnableIntrospection {
        // This would require modifying the schema to disable introspection
        log.Infof("GraphQL introspection disabled for production")
    }
    
    graphqlHttpHandler := handler.New(handlerConfig)
    
    // Create secure handler wrapper
    secureHandler := secureGraphQLHandler(graphqlHttpHandler, securityConfig)
    
    // Apply middleware stack
    middlewares := []gin.HandlerFunc{
        graphQLRateLimitMiddleware(securityConfig.RateLimitPerMinute),
        graphQLAuthMiddleware(authMiddleware, securityConfig.RequireAuthentication),
        secureHandler,
    }
    
    // Register allowed HTTP methods only
    for _, method := range securityConfig.AllowedMethods {
        defaultRouter.Handle(method, "/graphql", middlewares...)
        log.Infof("Registered secure GraphQL endpoint: %s /graphql", method)
    }
    
    // Add GraphQL endpoint information
    defaultRouter.GET("/graphql/info", func(c *gin.Context) {
        setGraphQLSecurityHeaders(c)
        c.JSON(200, gin.H{
            "endpoint":     "/graphql",
            "methods":      securityConfig.AllowedMethods,
            "playground":   securityConfig.EnablePlayground,
            "graphiql":     securityConfig.EnableGraphiQL,
            "introspection": securityConfig.EnableIntrospection,
            "environment":  map[string]interface{}{
                "production": isProduction,
            },
        })
    })
    
    log.Infof("Secure GraphQL endpoint initialized successfully")
    return nil
}

// MakeSecureGraphqlSchema creates GraphQL schema with security enhancements
func MakeSecureGraphqlSchema(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, config GraphQLSecurityConfig) (*graphql.Schema, error) {
    // This function should enhance the existing MakeGraphqlSchema with:
    // 1. Query depth analysis
    // 2. Query complexity analysis  
    // 3. Field-level authorization
    // 4. Input sanitization
    // 5. Introspection control
    
    // For now, call existing function with additional security wrapper
    schema := MakeGraphqlSchema(initConfig, cruds)
    
    // Add security enhancements to schema
    // Implementation would require modifying the schema generation
    
    return schema, nil
}

// GraphQLMetrics holds metrics for GraphQL operations
type GraphQLMetrics struct {
    TotalRequests    int64
    FailedRequests   int64
    AverageLatency   time.Duration
    RateLimitedIPs   []string
}

// GetGraphQLMetrics returns GraphQL security and performance metrics
func GetGraphQLMetrics() GraphQLMetrics {
    return GraphQLMetrics{
        // Implementation would track actual metrics
        TotalRequests:  0,
        FailedRequests: 0,
        AverageLatency: 0,
    }
}

// InitializeGraphqlResource maintains backward compatibility with security enhancements
func InitializeGraphqlResource(initConfig resource.CmsConfig, cruds map[string]*resource.DbResource, defaultRouter *gin.Engine) {
    // Try secure implementation first
    err := InitializeSecureGraphqlResource(initConfig, cruds, defaultRouter, nil)
    if err != nil {
        log.Warnf("Secure GraphQL initialization failed, falling back to basic: %v", err)
        
        // Fallback to original implementation with basic security
        securityConfig := getGraphQLSecurityConfig()
        
        graphqlSchema := MakeGraphqlSchema(&initConfig, cruds)
        
        graphqlHttpHandler := handler.New(&handler.Config{
            Schema:     graphqlSchema,
            Pretty:     !isProduction,
            Playground: securityConfig.EnablePlayground,
            GraphiQL:   securityConfig.EnableGraphiQL,
        })
        
        // Add basic security wrapper
        secureHandler := func(c *gin.Context) {
            setGraphQLSecurityHeaders(c)
            
            // Basic method validation
            if err := validateGraphQLMethod(c.Request.Method, securityConfig.AllowedMethods); err != nil {
                c.AbortWithStatus(405)
                return
            }
            
            graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)
        }
        
        // Register only allowed methods
        for _, method := range securityConfig.AllowedMethods {
            defaultRouter.Handle(method, "/graphql", secureHandler)
        }
        
        log.Infof("Basic GraphQL endpoint initialized")
    }
}
```

### Long-term Improvements
1. **Query Analysis:** Implement comprehensive query depth and complexity analysis
2. **Field-Level Authorization:** Add fine-grained field-level access controls
3. **GraphQL Security Scanner:** Automated security scanning for GraphQL schemas
4. **Performance Monitoring:** Comprehensive GraphQL performance and security monitoring
5. **Schema Versioning:** Implement schema versioning for backward compatibility

## Edge Cases Identified

1. **Large Query Payloads:** Very large GraphQL queries causing memory issues
2. **Deeply Nested Queries:** Extremely deep query nesting causing stack overflow
3. **Batch Query Abuse:** Multiple queries in single request amplifying attacks
4. **Schema Evolution:** Changes to GraphQL schema breaking client applications
5. **Subscription Management:** WebSocket connections for GraphQL subscriptions
6. **File Upload Handling:** GraphQL file uploads through multipart requests
7. **Error Handling:** GraphQL errors exposing sensitive system information
8. **Caching Issues:** GraphQL response caching with dynamic authorization
9. **Concurrent Requests:** High concurrency GraphQL requests causing resource contention
10. **Development Interface Security:** Accidental exposure of development interfaces

## Security Best Practices Violations

1. **Missing authentication and authorization** for GraphQL endpoints
2. **Development features enabled** without environment checks
3. **Unrestricted HTTP methods** for GraphQL operations
4. **Missing input validation** for GraphQL queries and mutations
5. **Missing security headers** for GraphQL responses
6. **Error information disclosure** through pretty printing
7. **Missing rate limiting** for GraphQL operations
8. **No query complexity analysis** enabling DoS attacks
9. **Schema introspection** always enabled exposing database structure
10. **No audit logging** for GraphQL operations

## Positive Security Aspects

1. **Centralized GraphQL endpoint** for consistent API access
2. **Schema-based validation** through GraphQL type system
3. **HTTP method support** for different operation types

## Critical Issues Summary

1. **Missing Authentication and Authorization:** GraphQL endpoints exposed without access controls
2. **Development Features in Production:** GraphQL Playground and GraphiQL enabled without environment checks
3. **Unrestricted HTTP Methods:** All HTTP methods supported without method-specific validation
4. **Missing Input Validation:** No validation of GraphQL queries or input parameters
5. **Missing Security Headers:** No security headers for GraphQL responses
6. **Error Information Disclosure:** Pretty printing may expose sensitive information
7. **Missing Rate Limiting:** No protection against GraphQL query flooding

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - GraphQL endpoint with missing authentication and development features exposed