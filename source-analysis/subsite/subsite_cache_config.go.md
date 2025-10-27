# Security Analysis: server/subsite/subsite_cache_config.go

**File:** `server/subsite/subsite_cache_config.go`  
**Type:** Cache configuration structure and parser  
**Lines of Code:** 298  

## Overview
This file defines the CacheConfig structure for controlling HTTP and in-memory caching behavior in subsite endpoints. It includes comprehensive cache control options and a parser function to convert interface{} input to structured configuration.

## Key Components

### CacheConfig struct
**Lines:** 9-78  
**Purpose:** Comprehensive cache configuration structure with HTTP and in-memory cache controls  

### GetCacheConfig function
**Lines:** 81-117  
**Purpose:** Parses cache configuration from interface{} input with JSON unmarshaling  

### Documentation Block
**Lines:** 119-297  
**Purpose:** Extensive documentation with usage examples and best practices  

## Critical Security Analysis

### 1. CRITICAL: Type Assertion Vulnerability - HIGH RISK
**Severity:** HIGH  
**Line:** 106  
**Issue:** Unsafe type assertion without validation that can cause runtime panics.

```go
actionReqStr := cacheConfigInterface.(string)  // No validation that input is a string
```

**Risk:**
- **Runtime panic** if `cacheConfigInterface` is not a string type
- **Application crashes** during cache configuration parsing
- **Service unavailability** when processing cache settings
- **No fallback mechanism** for type assertion failures

**Impact:** Complete service disruption when cache configuration data is not the expected string type.

### 2. HIGH: JSON Injection Vulnerability - HIGH RISK
**Severity:** HIGH  
**Line:** 111  
**Issue:** Unmarshaling user-controlled JSON without validation or size limits.

```go
err := json.Unmarshal([]byte(actionReqStr), &cacheConfig)  // No validation of JSON content
```

**Risk:**
- **JSON injection** through malicious cache configuration
- **Memory exhaustion** through deeply nested JSON structures
- **Denial of Service** through large JSON payloads
- **Configuration manipulation** through crafted JSON
- **No size limits** on input JSON

**Impact:** Application compromise through malicious cache configuration leading to DoS or configuration manipulation.

### 3. MEDIUM: Cache Configuration Security Issues - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 47-48, 57-58  
**Issue:** User-controlled cache headers and key prefixes without validation.

```go
CustomHeaders map[string]string `json:"custom_headers"`    // User-controlled headers
CacheKeyPrefix string `json:"cache_key_prefix"`            // User-controlled prefix
```

**Risk:**
- **HTTP header injection** through CustomHeaders
- **Cache key manipulation** through CacheKeyPrefix
- **Response manipulation** through crafted cache headers
- **Cache poisoning** through controlled key prefixes

### 4. MEDIUM: Missing Input Validation - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 35, 38, 53  
**Issue:** Array and slice fields without size or content validation.

```go
VaryByHeaders []string `json:"vary_by_headers"`              // No size limits
VaryByQueryParams []string `json:"vary_by_query_params"`     // No validation
```

**Risk:**
- **Memory exhaustion** through extremely large arrays
- **Header injection** through crafted header names
- **Parameter pollution** through malicious query parameters
- **Resource exhaustion** from processing large lists

### 5. LOW: Configuration Value Ranges - LOW RISK
**Severity:** LOW  
**Lines:** 15, 45, 67, 71  
**Issue:** Numeric configuration values without bounds checking.

```go
MaxAge int `json:"max_age"`                                // No bounds
StaleWhileRevalidate int `json:"stale_while_revalidate"`   // No validation
InMemoryCacheTTL int `json:"in_memory_cache_ttl"`          // No limits
InMemoryCacheMaxSize int `json:"in_memory_cache_max_size"` // No bounds
```

**Risk:**
- **Resource exhaustion** through extremely large cache sizes
- **Invalid configuration** through negative values
- **Memory issues** from unbounded cache settings

## Potential Attack Vectors

### Type Confusion Attacks
1. **Non-String Input:** Pass non-string types to trigger type assertion panic
2. **Interface Manipulation:** Use interface{} type confusion for unexpected behavior
3. **Nil Pointer Exploitation:** Exploit null pointer scenarios

### JSON Injection Attacks
1. **Malicious JSON:** Inject malicious JSON structures in cache configuration
2. **Memory Exhaustion:** Use deeply nested JSON for memory DoS
3. **Configuration Bypass:** Use unexpected JSON structures to bypass intended behavior
4. **Large Payload Attacks:** Submit extremely large JSON for resource exhaustion

### Cache Manipulation Attacks
1. **Header Injection:** Inject malicious HTTP headers through CustomHeaders
2. **Key Collision:** Manipulate CacheKeyPrefix to cause cache key collisions
3. **Cache Poisoning:** Use crafted configurations to poison cache entries
4. **Response Manipulation:** Control HTTP responses through cache headers

### Resource Exhaustion Attacks
1. **Memory DoS:** Configure extremely large cache sizes
2. **Array Flooding:** Submit large arrays for VaryByHeaders/VaryByQueryParams
3. **Processing Overload:** Create complex cache configurations requiring excessive processing

## Recommendations

### Immediate Actions
1. **Add Type Validation:** Validate input type before assertion
2. **Add JSON Size Limits:** Implement size limits for JSON input
3. **Validate Configuration Values:** Add bounds checking for all numeric values
4. **Sanitize Headers:** Validate and sanitize custom headers

### Enhanced Security Implementation

```go
package subsite

import (
    "encoding/json"
    "fmt"
    "regexp"
    "strings"
    "time"
    "unicode/utf8"
    
    log "github.com/sirupsen/logrus"
)

const (
    MaxCacheConfigSize = 32 * 1024 // 32KB limit
    MaxCustomHeaders = 20
    MaxVaryHeaders = 10
    MaxVaryQueryParams = 20
    MaxCacheKeyPrefixLength = 255
    MaxHeaderNameLength = 100
    MaxHeaderValueLength = 2048
    MaxCacheSize = 100000
    MaxCacheTTL = 7 * 24 * 3600 // 7 days
)

var (
    validHeaderNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]*$`)
    validCacheKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]*$`)
    validETagStrategies = map[string]bool{"weak": true, "strong": true, "none": true}
    validCacheStrategies = map[string]bool{"lru": true, "lfu": true}
    dangerousHeaderChars = regexp.MustCompile(`[\r\n\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`)
)

// CacheConfig provides secure cache configuration with validation
type CacheConfig struct {
    // HTTP cache control settings
    Enable       bool `json:"enable"`
    MaxAge       int  `json:"max_age"`
    Revalidate   bool `json:"revalidate"`
    NoCache      bool `json:"no_cache"`
    NoStore      bool `json:"no_store"`
    Private      bool `json:"private"`
    VaryByPath   bool `json:"vary_by_path"`
    
    // Validated arrays with size limits
    VaryByHeaders     []string `json:"vary_by_headers"`
    VaryByQueryParams []string `json:"vary_by_query_params"`
    
    StaleWhileRevalidate int               `json:"stale_while_revalidate"`
    CustomHeaders        map[string]string `json:"custom_headers"`
    ExpiresAt           *time.Time         `json:"expires_at,omitempty"`
    ETagStrategy        string             `json:"etag_strategy"`
    CacheKeyPrefix      string             `json:"cache_key_prefix"`
    
    // In-memory cache controls
    EnableInMemoryCache       bool   `json:"enable_in_memory_cache"`
    InMemoryCacheTTL         int    `json:"in_memory_cache_ttl"`
    InMemoryCacheMaxSize     int    `json:"in_memory_cache_max_size"`
    InMemoryCacheStrategy    string `json:"in_memory_cache_strategy"`
    InMemoryCacheCompression bool   `json:"in_memory_cache_compression"`
}

// validateCacheConfigInput validates input before processing
func validateCacheConfigInput(cacheConfigInterface interface{}) error {
    if cacheConfigInterface == nil {
        return nil // Nil is acceptable
    }
    
    if _, ok := cacheConfigInterface.(string); !ok {
        return fmt.Errorf("cache config must be a string, got %T", cacheConfigInterface)
    }
    
    return nil
}

// validateCacheConfig validates the parsed cache configuration
func validateCacheConfig(config *CacheConfig) error {
    if config == nil {
        return fmt.Errorf("cache config cannot be nil")
    }
    
    // Validate numeric ranges
    if config.MaxAge < 0 || config.MaxAge > MaxCacheTTL {
        return fmt.Errorf("invalid MaxAge: %d, must be between 0 and %d", config.MaxAge, MaxCacheTTL)
    }
    
    if config.StaleWhileRevalidate < 0 || config.StaleWhileRevalidate > MaxCacheTTL {
        return fmt.Errorf("invalid StaleWhileRevalidate: %d", config.StaleWhileRevalidate)
    }
    
    if config.InMemoryCacheTTL < 0 || config.InMemoryCacheTTL > MaxCacheTTL {
        return fmt.Errorf("invalid InMemoryCacheTTL: %d", config.InMemoryCacheTTL)
    }
    
    if config.InMemoryCacheMaxSize < 0 || config.InMemoryCacheMaxSize > MaxCacheSize {
        return fmt.Errorf("invalid InMemoryCacheMaxSize: %d", config.InMemoryCacheMaxSize)
    }
    
    // Validate arrays
    if len(config.VaryByHeaders) > MaxVaryHeaders {
        return fmt.Errorf("too many VaryByHeaders: %d, maximum: %d", len(config.VaryByHeaders), MaxVaryHeaders)
    }
    
    if len(config.VaryByQueryParams) > MaxVaryQueryParams {
        return fmt.Errorf("too many VaryByQueryParams: %d, maximum: %d", len(config.VaryByQueryParams), MaxVaryQueryParams)
    }
    
    // Validate header names
    for _, header := range config.VaryByHeaders {
        if err := validateHeaderName(header); err != nil {
            return fmt.Errorf("invalid VaryByHeader '%s': %v", header, err)
        }
    }
    
    // Validate query parameter names
    for _, param := range config.VaryByQueryParams {
        if err := validateParameterName(param); err != nil {
            return fmt.Errorf("invalid VaryByQueryParam '%s': %v", param, err)
        }
    }
    
    // Validate custom headers
    if len(config.CustomHeaders) > MaxCustomHeaders {
        return fmt.Errorf("too many CustomHeaders: %d, maximum: %d", len(config.CustomHeaders), MaxCustomHeaders)
    }
    
    for name, value := range config.CustomHeaders {
        if err := validateHeaderName(name); err != nil {
            return fmt.Errorf("invalid CustomHeader name '%s': %v", name, err)
        }
        if err := validateHeaderValue(value); err != nil {
            return fmt.Errorf("invalid CustomHeader value for '%s': %v", name, err)
        }
    }
    
    // Validate string fields
    if len(config.CacheKeyPrefix) > MaxCacheKeyPrefixLength {
        return fmt.Errorf("CacheKeyPrefix too long: %d", len(config.CacheKeyPrefix))
    }
    
    if len(config.CacheKeyPrefix) > 0 && !validCacheKeyPattern.MatchString(config.CacheKeyPrefix) {
        return fmt.Errorf("invalid CacheKeyPrefix format")
    }
    
    if !validETagStrategies[config.ETagStrategy] {
        return fmt.Errorf("invalid ETagStrategy: %s", config.ETagStrategy)
    }
    
    if !validCacheStrategies[config.InMemoryCacheStrategy] {
        return fmt.Errorf("invalid InMemoryCacheStrategy: %s", config.InMemoryCacheStrategy)
    }
    
    return nil
}

// validateHeaderName validates HTTP header names
func validateHeaderName(name string) error {
    if len(name) == 0 {
        return fmt.Errorf("header name cannot be empty")
    }
    
    if len(name) > MaxHeaderNameLength {
        return fmt.Errorf("header name too long: %d", len(name))
    }
    
    if !validHeaderNamePattern.MatchString(name) {
        return fmt.Errorf("invalid header name format")
    }
    
    // Check for dangerous characters
    if dangerousHeaderChars.MatchString(name) {
        return fmt.Errorf("header name contains dangerous characters")
    }
    
    return nil
}

// validateHeaderValue validates HTTP header values
func validateHeaderValue(value string) error {
    if len(value) > MaxHeaderValueLength {
        return fmt.Errorf("header value too long: %d", len(value))
    }
    
    if !utf8.ValidString(value) {
        return fmt.Errorf("header value contains invalid UTF-8")
    }
    
    // Check for header injection characters
    if dangerousHeaderChars.MatchString(value) {
        return fmt.Errorf("header value contains dangerous characters")
    }
    
    return nil
}

// validateParameterName validates query parameter names
func validateParameterName(name string) error {
    if len(name) == 0 {
        return fmt.Errorf("parameter name cannot be empty")
    }
    
    if len(name) > 100 {
        return fmt.Errorf("parameter name too long: %d", len(name))
    }
    
    if !utf8.ValidString(name) {
        return fmt.Errorf("parameter name contains invalid UTF-8")
    }
    
    // Allow alphanumeric, underscore, hyphen
    for _, r := range name {
        if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
             (r >= '0' && r <= '9') || r == '_' || r == '-') {
            return fmt.Errorf("parameter name contains invalid characters")
        }
    }
    
    return nil
}

// GetCacheConfigSecure provides secure cache configuration parsing
func GetCacheConfigSecure(cacheConfigInterface interface{}) (*CacheConfig, error) {
    // Input validation
    if err := validateCacheConfigInput(cacheConfigInterface); err != nil {
        return nil, fmt.Errorf("invalid input: %v", err)
    }
    
    // Default configuration with secure defaults
    cacheConfig := CacheConfig{
        Enable:       false,
        MaxAge:       0,
        Revalidate:   true,
        NoCache:      false,
        NoStore:      false,
        Private:      false,
        VaryByPath:   true,
        ETagStrategy: "weak",
        
        EnableInMemoryCache:      false,
        InMemoryCacheTTL:         300,
        InMemoryCacheMaxSize:     100,
        InMemoryCacheStrategy:    "lru",
        InMemoryCacheCompression: false,
    }
    
    var configStr string
    if cacheConfigInterface == nil {
        configStr = "{}"
    } else {
        configStr = cacheConfigInterface.(string)
        if len(strings.TrimSpace(configStr)) == 0 {
            configStr = "{}"
        }
    }
    
    // Validate JSON size
    if len(configStr) > MaxCacheConfigSize {
        return nil, fmt.Errorf("cache config too large: %d bytes", len(configStr))
    }
    
    // Parse JSON
    err := json.Unmarshal([]byte(configStr), &cacheConfig)
    if err != nil {
        log.Warnf("Failed to parse cache config JSON: %v", err)
        return nil, fmt.Errorf("JSON parsing failed")
    }
    
    // Validate parsed configuration
    if err := validateCacheConfig(&cacheConfig); err != nil {
        log.Warnf("Cache config validation failed: %v", err)
        return nil, fmt.Errorf("invalid cache configuration: %v", err)
    }
    
    log.Debugf("Successfully parsed cache config: enable=%t, maxAge=%d", cacheConfig.Enable, cacheConfig.MaxAge)
    return &cacheConfig, nil
}

// GetCacheConfig maintains backward compatibility
func GetCacheConfig(cacheConfigInterface interface{}) (*CacheConfig, error) {
    return GetCacheConfigSecure(cacheConfigInterface)
}

// DefaultCacheConfig returns a secure default cache configuration
func DefaultCacheConfig() *CacheConfig {
    return &CacheConfig{
        Enable:                   false,
        MaxAge:                  0,
        Revalidate:              true,
        NoCache:                 false,
        NoStore:                 false,
        Private:                 false,
        VaryByPath:              true,
        ETagStrategy:            "weak",
        EnableInMemoryCache:     false,
        InMemoryCacheTTL:        300,
        InMemoryCacheMaxSize:    100,
        InMemoryCacheStrategy:   "lru",
        InMemoryCacheCompression: false,
    }
}
```

### Long-term Improvements
1. **Configuration Schema:** Implement JSON schema validation for cache configurations
2. **Security Profiles:** Provide predefined secure configuration profiles
3. **Runtime Monitoring:** Monitor cache configuration changes and usage patterns
4. **Configuration Auditing:** Log all cache configuration modifications
5. **Performance Optimization:** Optimize configuration parsing and validation

## Edge Cases Identified

1. **Null Configuration Values:** Various null and undefined configuration scenarios
2. **Empty String Configurations:** Different empty string patterns
3. **Large Configuration Objects:** Very large cache configuration data
4. **Malformed JSON:** Various invalid JSON patterns in configurations
5. **Unicode Content:** Configuration values with unicode characters
6. **Negative Numeric Values:** Handling of negative cache sizes and TTL values
7. **Time Zone Issues:** ExpiresAt handling across different time zones
8. **Memory Pressure:** Configuration parsing under high memory pressure

## Security Best Practices Violations

1. **No type validation before type assertion**
2. **Unlimited JSON input size acceptance**
3. **No validation of custom headers and cache keys**
4. **Missing bounds checking on numeric configuration values**
5. **No sanitization of user-controlled strings**

## Positive Security Aspects

1. **Comprehensive Documentation:** Extensive documentation with security considerations
2. **Default Security:** Secure defaults with caching disabled by default
3. **Flexible Configuration:** Allows fine-grained security control over caching behavior

## Critical Issues Summary

1. **Type Assertion Vulnerability:** Runtime panics from unsafe type assertion in configuration parsing
2. **JSON Injection Vulnerability:** Unvalidated JSON unmarshaling allowing malicious configurations
3. **Cache Configuration Manipulation:** User-controlled cache headers and key prefixes without validation
4. **Resource Exhaustion:** No bounds checking on configuration arrays and numeric values

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** HIGH - Critical type assertion and JSON injection vulnerabilities in cache configuration