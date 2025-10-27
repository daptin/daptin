# Security Analysis: server/subsite/subsite_action_config.go

**File:** `server/subsite/subsite_action_config.go`  
**Type:** Action configuration parsing utility  
**Lines of Code:** 24  

## Overview
This file provides a utility function for parsing action configuration from interface{} to ActionRequest struct. It handles JSON unmarshaling with basic fallback for nil and empty values.

## Key Components

### GetActionConfig function
**Lines:** 8-23  
**Purpose:** Converts interface{} action configuration to structured ActionRequest  

## Critical Security Analysis

### 1. CRITICAL: Type Assertion Vulnerability - HIGH RISK
**Severity:** HIGH  
**Line:** 13  
**Issue:** Unsafe type assertion without validation that can cause runtime panics.

```go
actionReqStr := actionRequestInt.(string)  // No validation that actionRequestInt is a string
```

**Risk:**
- **Runtime panic** if `actionRequestInt` is not a string type
- **Application crashes** during action configuration parsing
- **Service unavailability** when processing action requests
- **No fallback mechanism** for type assertion failures

**Impact:** Complete service disruption when action configuration data is not the expected string type.

### 2. HIGH: JSON Injection Vulnerability - HIGH RISK
**Severity:** HIGH  
**Line:** 17  
**Issue:** Unmarshaling user-controlled JSON without validation or size limits.

```go
err := json.Unmarshal([]byte(actionReqStr), &actionRequest)  // No validation of JSON content
```

**Risk:**
- **JSON injection** through malicious action configuration
- **Memory exhaustion** through deeply nested JSON structures
- **Denial of Service** through large JSON payloads
- **Code execution** through deserialization vulnerabilities
- **No size limits** on input JSON

**Impact:** Application compromise through malicious JSON configuration leading to DoS or potential code execution.

### 3. MEDIUM: Missing Input Validation - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 10-16  
**Issue:** Basic null checks but no comprehensive input validation.

```go
if actionRequestInt == nil {
    actionRequestInt = "{}"  // Default to empty JSON object
}
if len(actionReqStr) == 0 {
    actionReqStr = "{}"      // Default to empty JSON object
}
```

**Risk:**
- **No validation** of JSON content structure
- **No sanitization** of input data
- **Potential injection** through malformed JSON
- **Unlimited input size** acceptance

### 4. LOW: Error Information Leakage - LOW RISK
**Severity:** LOW  
**Line:** 20  
**Issue:** JSON parsing errors returned directly to caller.

```go
return actionresponse.ActionRequest{}, err  // Direct error propagation
```

**Risk:**
- **Information disclosure** through JSON parsing error messages
- **Internal structure exposure** through error details
- **Debugging information leakage** to potential attackers

## Potential Attack Vectors

### Type Confusion Attacks
1. **Non-String Input:** Pass non-string types to trigger type assertion panic
2. **Interface Manipulation:** Use interface{} type confusion for unexpected behavior
3. **Nil Pointer Exploitation:** Exploit null pointer scenarios

### JSON Injection Attacks
1. **Malicious JSON:** Inject malicious JSON structures
2. **Memory Exhaustion:** Use deeply nested JSON for memory DoS
3. **Parser Exploitation:** Exploit JSON parser vulnerabilities
4. **Structure Confusion:** Use unexpected JSON structures to bypass validation

### Data Validation Bypass
1. **Empty Value Exploitation:** Use empty strings to bypass validation
2. **Large Payload Attacks:** Submit extremely large JSON for resource exhaustion
3. **Encoding Attacks:** Use various JSON encoding techniques for bypass

## Recommendations

### Immediate Actions
1. **Add Type Validation:** Validate input type before assertion
2. **Add JSON Size Limits:** Implement size limits for JSON input
3. **Add Structure Validation:** Validate JSON structure and content
4. **Improve Error Handling:** Sanitize error messages

### Enhanced Security Implementation

```go
package subsite

import (
    "encoding/json"
    "fmt"
    "strings"
    "unicode/utf8"
    
    "github.com/daptin/daptin/server/actionresponse"
    log "github.com/sirupsen/logrus"
)

const (
    MaxActionConfigSize = 64 * 1024 // 64KB limit
    MaxJSONDepth = 10
    DefaultActionConfig = "{}"
)

// validateActionConfigInput validates the input before processing
func validateActionConfigInput(actionRequestInt interface{}) error {
    if actionRequestInt == nil {
        return nil // Nil is acceptable and will use default
    }
    
    // Check if it's a string
    if _, ok := actionRequestInt.(string); !ok {
        return fmt.Errorf("action config must be a string, got %T", actionRequestInt)
    }
    
    return nil
}

// validateJSONString validates JSON string content and structure
func validateJSONString(jsonStr string) error {
    if len(jsonStr) > MaxActionConfigSize {
        return fmt.Errorf("action config too large: %d bytes, maximum allowed: %d", len(jsonStr), MaxActionConfigSize)
    }
    
    if !utf8.ValidString(jsonStr) {
        return fmt.Errorf("action config contains invalid UTF-8")
    }
    
    // Check for basic JSON structure
    trimmed := strings.TrimSpace(jsonStr)
    if len(trimmed) == 0 {
        return nil // Empty is valid, will use default
    }
    
    // Basic JSON format validation
    if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
        return fmt.Errorf("action config must be a valid JSON object")
    }
    
    // Check for deeply nested structures (basic protection)
    depth := 0
    maxDepth := 0
    for _, char := range trimmed {
        switch char {
        case '{', '[':
            depth++
            if depth > maxDepth {
                maxDepth = depth
            }
        case '}', ']':
            depth--
        }
        
        if maxDepth > MaxJSONDepth {
            return fmt.Errorf("action config JSON too deeply nested: %d levels, maximum allowed: %d", maxDepth, MaxJSONDepth)
        }
    }
    
    return nil
}

// sanitizeActionRequest validates and sanitizes the parsed action request
func sanitizeActionRequest(actionRequest *actionresponse.ActionRequest) error {
    if actionRequest == nil {
        return fmt.Errorf("action request cannot be nil")
    }
    
    // Validate action name
    if len(actionRequest.Action) > 255 {
        return fmt.Errorf("action name too long: %d characters", len(actionRequest.Action))
    }
    
    // Validate type name
    if len(actionRequest.Type) > 255 {
        return fmt.Errorf("action type too long: %d characters", len(actionRequest.Type))
    }
    
    // Check for dangerous characters in action/type
    dangerousChars := []string{"\x00", "\n", "\r", "\t", "\\", "\"", "'", ";", "--"}
    for _, dangerous := range dangerousChars {
        if strings.Contains(actionRequest.Action, dangerous) {
            return fmt.Errorf("action name contains dangerous characters")
        }
        if strings.Contains(actionRequest.Type, dangerous) {
            return fmt.Errorf("action type contains dangerous characters")
        }
    }
    
    return nil
}

// GetActionConfigSecure provides secure action configuration parsing with comprehensive validation
func GetActionConfigSecure(actionRequestInt interface{}) (actionresponse.ActionRequest, error) {
    // Input type validation
    if err := validateActionConfigInput(actionRequestInt); err != nil {
        log.Warnf("Invalid action config input: %v", err)
        return actionresponse.ActionRequest{}, fmt.Errorf("invalid input type: %v", err)
    }
    
    var actionReqStr string
    
    // Handle nil input
    if actionRequestInt == nil {
        actionReqStr = DefaultActionConfig
    } else {
        // Safe type assertion (already validated above)
        actionReqStr = actionRequestInt.(string)
        
        // Handle empty string
        if len(strings.TrimSpace(actionReqStr)) == 0 {
            actionReqStr = DefaultActionConfig
        }
    }
    
    // Validate JSON string
    if err := validateJSONString(actionReqStr); err != nil {
        log.Warnf("Invalid action config JSON: %v", err)
        return actionresponse.ActionRequest{}, fmt.Errorf("invalid JSON format: %v", err)
    }
    
    // Parse JSON with controlled unmarshaling
    var actionRequest actionresponse.ActionRequest
    err := json.Unmarshal([]byte(actionReqStr), &actionRequest)
    if err != nil {
        log.Warnf("Failed to parse action config JSON: %v", err)
        return actionresponse.ActionRequest{}, fmt.Errorf("JSON parsing failed")
    }
    
    // Validate and sanitize parsed request
    if err := sanitizeActionRequest(&actionRequest); err != nil {
        log.Warnf("Action request validation failed: %v", err)
        return actionresponse.ActionRequest{}, fmt.Errorf("invalid action configuration")
    }
    
    log.Debugf("Successfully parsed action config: action=%s, type=%s", actionRequest.Action, actionRequest.Type)
    return actionRequest, nil
}

// GetActionConfig maintains backward compatibility while providing secure parsing
func GetActionConfig(actionRequestInt interface{}) (actionresponse.ActionRequest, error) {
    return GetActionConfigSecure(actionRequestInt)
}

// SafeParseActionConfig provides an alternative with explicit error handling
func SafeParseActionConfig(jsonConfig string) (actionresponse.ActionRequest, error) {
    if len(jsonConfig) == 0 {
        jsonConfig = DefaultActionConfig
    }
    
    // Validate JSON string
    if err := validateJSONString(jsonConfig); err != nil {
        return actionresponse.ActionRequest{}, err
    }
    
    var actionRequest actionresponse.ActionRequest
    err := json.Unmarshal([]byte(jsonConfig), &actionRequest)
    if err != nil {
        return actionresponse.ActionRequest{}, fmt.Errorf("JSON parsing failed: %v", err)
    }
    
    // Validate parsed request
    if err := sanitizeActionRequest(&actionRequest); err != nil {
        return actionresponse.ActionRequest{}, err
    }
    
    return actionRequest, nil
}

// ValidateActionConfig validates action configuration without parsing
func ValidateActionConfig(actionRequestInt interface{}) error {
    if err := validateActionConfigInput(actionRequestInt); err != nil {
        return err
    }
    
    if actionRequestInt == nil {
        return nil // Nil is valid
    }
    
    actionReqStr := actionRequestInt.(string)
    if len(strings.TrimSpace(actionReqStr)) == 0 {
        return nil // Empty is valid
    }
    
    return validateJSONString(actionReqStr)
}
```

### Long-term Improvements
1. **Schema Validation:** Implement JSON schema validation for action configurations
2. **Configuration Templates:** Provide predefined safe configuration templates
3. **Security Scanning:** Scan action configurations for malicious patterns
4. **Rate Limiting:** Add rate limiting for configuration parsing operations
5. **Audit Logging:** Log all action configuration parsing attempts

## Edge Cases Identified

1. **Null Input Values:** Various null and undefined input scenarios
2. **Empty String Inputs:** Different empty string patterns
3. **Large JSON Payloads:** Very large action configuration data
4. **Malformed JSON:** Various invalid JSON patterns
5. **Unicode Content:** Action configurations with unicode characters
6. **Nested Objects:** Deeply nested JSON structures
7. **Type Confusion:** Different interface{} type scenarios
8. **Memory Pressure:** Parsing under high memory pressure conditions

## Security Best Practices Violations

1. **No type validation before type assertion**
2. **Unlimited JSON input size acceptance**
3. **No JSON structure validation**
4. **Direct error propagation exposing internal details**
5. **Missing input sanitization**

## Positive Security Aspects

1. **Null Handling:** Proper handling of nil input values
2. **Default Values:** Safe default configuration for empty inputs
3. **Error Propagation:** Returns errors rather than panicking

## Critical Issues Summary

1. **Type Assertion Vulnerability:** Runtime panics from unsafe type assertion
2. **JSON Injection Vulnerability:** Unvalidated JSON unmarshaling allowing malicious input
3. **Missing Input Validation:** No size limits or content validation for JSON input
4. **Information Leakage:** Direct error propagation potentially exposing internal details

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** HIGH - Critical type assertion and JSON injection vulnerabilities