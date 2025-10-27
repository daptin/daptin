# Security Analysis: server/subsite/utils.go

**File:** `server/subsite/utils.go`  
**Type:** Error checking utility function  
**Lines of Code:** 19  

## Overview
This file provides a utility function for error checking and logging. It's a simple wrapper around error handling that formats and logs error messages with additional context information.

## Key Components

### CheckErr function
**Lines:** 5-18  
**Purpose:** Checks for errors and logs them with formatted messages and additional context  

## Critical Security Analysis

### 1. CRITICAL: Type Assertion Vulnerability - HIGH RISK
**Severity:** HIGH  
**Line:** 8  
**Issue:** Unsafe type assertion without validation that can cause runtime panics.

```go
fmtString := message[0].(string)  // No validation that message[0] is a string
```

**Risk:**
- **Runtime panic** if `message[0]` is not a string or if `message` slice is empty
- **Application crashes** when error checking is performed with invalid parameters
- **Service unavailability** during error handling operations
- **No bounds checking** on message slice access

**Impact:** Application crashes during error handling, potentially masking the original error.

### 2. MEDIUM: Log Injection Vulnerability - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 8, 14  
**Issue:** User-controlled data directly passed to logging without sanitization.

```go
fmtString := message[0].(string)  // User-controlled format string
log.Errorf(fmtString+": %v", args...)  // Direct use in logging
```

**Risk:**
- **Log injection** through crafted error messages
- **Log format string attacks** if format string contains log injection patterns
- **Information disclosure** through manipulated log entries
- **Log parsing errors** from malformed format strings

### 3. MEDIUM: Format String Vulnerabilities - MEDIUM RISK
**Severity:** MEDIUM  
**Line:** 14  
**Issue:** User-controlled format string used in logging function.

```go
log.Errorf(fmtString+": %v", args...)  // fmtString from user input
```

**Risk:**
- **Format string attacks** through crafted format strings
- **Information disclosure** through format string exploitation
- **Application behavior manipulation** through format string injection
- **Log corruption** through malformed format specifiers

### 4. LOW: Bounds Checking Missing - LOW RISK
**Severity:** LOW  
**Line:** 8  
**Issue:** No validation of message slice bounds before access.

```go
fmtString := message[0].(string)  // No check if message has elements
```

**Risk:**
- **Index out of bounds** panic if message slice is empty
- **Unexpected behavior** with empty message parameters
- **Inconsistent error handling** across different call patterns

## Potential Attack Vectors

### Application Stability Attacks
1. **Type Confusion:** Call CheckErr with non-string first parameter to trigger panic
2. **Empty Parameter Lists:** Call CheckErr with empty message slice to cause index panic
3. **Format String Exploitation:** Use crafted format strings to manipulate logging behavior

### Log Injection Attacks
1. **Log Entry Manipulation:** Inject malicious content into log entries
2. **Log Parsing Confusion:** Create malformed log entries to confuse log parsers
3. **Information Disclosure:** Extract sensitive information through log injection

### Format String Attacks
1. **Information Leakage:** Use format specifiers to leak memory contents
2. **Log Corruption:** Use invalid format specifiers to corrupt log output
3. **Parser Exploitation:** Exploit log parsing vulnerabilities through crafted entries

## Recommendations

### Immediate Actions
1. **Add Input Validation:** Validate all input parameters before use
2. **Sanitize Format Strings:** Validate and sanitize format strings
3. **Add Bounds Checking:** Check slice bounds before access
4. **Fixed Format Strings:** Use fixed format strings instead of user-controlled ones

### Enhanced Security Implementation

```go
package subsite

import (
    "fmt"
    "regexp"
    "strings"
    
    log "github.com/sirupsen/logrus"
)

const (
    MaxMessageLength = 1024
    MaxArgsCount = 20
)

var (
    // Pattern to detect potentially dangerous format string patterns
    dangerousFormatPattern = regexp.MustCompile(`%[^a-zA-Z0-9\s\-+#.]`)
    // Pattern for safe log content (alphanumeric, spaces, common punctuation)
    safeLogPattern = regexp.MustCompile(`^[a-zA-Z0-9\s\-_.,:;()\[\]{}]+$`)
)

// sanitizeLogMessage sanitizes log message content
func sanitizeLogMessage(message string) string {
    if len(message) > MaxMessageLength {
        message = message[:MaxMessageLength] + "...[TRUNCATED]"
    }
    
    // Remove potentially dangerous characters
    message = strings.ReplaceAll(message, "\x00", "")
    message = strings.ReplaceAll(message, "\n", " ")
    message = strings.ReplaceAll(message, "\r", " ")
    message = strings.ReplaceAll(message, "\t", " ")
    
    // Replace any remaining control characters
    cleaned := ""
    for _, r := range message {
        if r >= 32 && r <= 126 || r == ' ' {
            cleaned += string(r)
        } else {
            cleaned += "?"
        }
    }
    
    return cleaned
}

// validateFormatString validates and sanitizes format strings
func validateFormatString(formatStr string) (string, error) {
    if len(formatStr) == 0 {
        return "", fmt.Errorf("format string cannot be empty")
    }
    
    if len(formatStr) > MaxMessageLength {
        return "", fmt.Errorf("format string too long: %d characters", len(formatStr))
    }
    
    // Check for dangerous format patterns
    if dangerousFormatPattern.MatchString(formatStr) {
        return "", fmt.Errorf("format string contains dangerous patterns")
    }
    
    // Sanitize the format string
    sanitized := sanitizeLogMessage(formatStr)
    
    return sanitized, nil
}

// SafeCheckErr provides secure error checking with input validation
func SafeCheckErr(err error, message ...interface{}) bool {
    if err == nil {
        return false
    }
    
    // Validate we have at least one message parameter
    if len(message) == 0 {
        log.Errorf("Error occurred but no message provided: %v", err)
        return true
    }
    
    // Validate first parameter is a string
    fmtStringInterface := message[0]
    fmtString, ok := fmtStringInterface.(string)
    if !ok {
        log.Errorf("Error occurred but message format is not a string (type: %T): %v", fmtStringInterface, err)
        return true
    }
    
    // Validate and sanitize format string
    safeFmtString, validationErr := validateFormatString(fmtString)
    if validationErr != nil {
        log.Errorf("Error occurred but format string is invalid (%v): %v", validationErr, err)
        return true
    }
    
    // Limit number of arguments to prevent abuse
    args := make([]interface{}, 0)
    if len(message) > 1 {
        maxArgs := len(message) - 1
        if maxArgs > MaxArgsCount {
            maxArgs = MaxArgsCount
        }
        
        for i := 1; i <= maxArgs; i++ {
            // Sanitize string arguments
            if str, ok := message[i].(string); ok {
                args = append(args, sanitizeLogMessage(str))
            } else {
                args = append(args, message[i])
            }
        }
    }
    
    // Add the error as the final argument
    args = append(args, err)
    
    // Use fixed format string pattern to prevent format string attacks
    log.Errorf("Error: %s - Details: %v", safeFmtString, args)
    return true
}

// CheckErr maintains backward compatibility while providing secure error checking
func CheckErr(err error, message ...interface{}) bool {
    return SafeCheckErr(err, message...)
}

// SecureLogError provides a completely safe logging function with fixed format
func SecureLogError(err error, context string, details map[string]interface{}) bool {
    if err == nil {
        return false
    }
    
    // Sanitize context
    safeContext := sanitizeLogMessage(context)
    
    // Sanitize details
    safeDetails := make(map[string]interface{})
    for key, value := range details {
        safeKey := sanitizeLogMessage(key)
        if str, ok := value.(string); ok {
            safeDetails[safeKey] = sanitizeLogMessage(str)
        } else {
            safeDetails[safeKey] = value
        }
    }
    
    log.WithFields(log.Fields{
        "context": safeContext,
        "details": safeDetails,
        "error":   err.Error(),
    }).Error("Operation failed")
    
    return true
}

// LogErrorWithCategory provides categorized error logging
func LogErrorWithCategory(err error, category, operation string, additionalInfo ...string) bool {
    if err == nil {
        return false
    }
    
    safeCategory := sanitizeLogMessage(category)
    safeOperation := sanitizeLogMessage(operation)
    
    fields := log.Fields{
        "category":  safeCategory,
        "operation": safeOperation,
        "error":     err.Error(),
    }
    
    // Add additional info if provided
    for i, info := range additionalInfo {
        if i >= 5 { // Limit additional info count
            break
        }
        fields[fmt.Sprintf("info_%d", i)] = sanitizeLogMessage(info)
    }
    
    log.WithFields(fields).Error("Categorized error occurred")
    return true
}

// ValidateErrorCheckParams validates parameters for error checking functions
func ValidateErrorCheckParams(message ...interface{}) error {
    if len(message) == 0 {
        return fmt.Errorf("no message parameters provided")
    }
    
    if len(message) > MaxArgsCount+1 { // +1 for format string
        return fmt.Errorf("too many message parameters: %d", len(message))
    }
    
    // Validate first parameter is a string
    if _, ok := message[0].(string); !ok {
        return fmt.Errorf("first parameter must be a string format")
    }
    
    return nil
}
```

### Long-term Improvements
1. **Structured Logging:** Migrate to structured logging with fixed schemas
2. **Error Categorization:** Implement error categorization and tracking
3. **Log Security Monitoring:** Monitor logs for injection attempts
4. **Error Handling Framework:** Develop comprehensive error handling framework
5. **Log Sanitization Pipeline:** Implement automated log content sanitization

## Edge Cases Identified

1. **Empty Message Arrays:** Calling CheckErr with no message parameters
2. **Non-String Format Parameters:** Passing non-string types as format strings
3. **Extremely Long Messages:** Very long error messages and format strings
4. **Unicode Content:** Error messages with unicode characters
5. **Nested Errors:** Complex error types with nested information
6. **Concurrent Error Handling:** Thread safety of error checking operations
7. **Memory Pressure:** Error handling under high memory pressure
8. **Log System Failures:** Error handling when logging system fails

## Security Best Practices Violations

1. **No input validation for function parameters**
2. **Unsafe type assertion without error checking**
3. **User-controlled format strings in logging**
4. **No sanitization of log content**
5. **Missing bounds checking on slice access**

## Positive Security Aspects

1. **Simple Function Design:** Minimal complexity reduces attack surface
2. **Error Propagation:** Maintains error information for debugging
3. **Consistent Logging:** Provides consistent error logging pattern

## Critical Issues Summary

1. **Type Assertion Vulnerability:** Runtime panics from unsafe type assertion on message parameters
2. **Log Injection Vulnerability:** User-controlled data directly passed to logging functions
3. **Format String Vulnerabilities:** User-controlled format strings used in logging
4. **Bounds Checking Missing:** No validation of slice bounds before access

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** HIGH - Critical type assertion vulnerability and log injection risks