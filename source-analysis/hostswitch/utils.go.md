# Security Analysis: server/hostswitch/utils.go

**File:** `server/hostswitch/utils.go`  
**Type:** Utility functions for host switching operations  
**Lines of Code:** 76  

## Overview
This file provides utility functions for string operations, error handling, and text transformation used by the host switching system. It includes string matching functions and error logging utilities.

## Functions

### EndsWithCheck(str string, endsWith string) bool
**Lines:** 11-23  
**Purpose:** Checks if string ends with specified suffix  

### BeginsWithCheck(str string, beginsWith string) bool
**Lines:** 25-39  
**Purpose:** Checks if string begins with specified prefix  

### CheckErr(err error, message ...interface{}) bool
**Lines:** 41-54  
**Purpose:** Error logging utility with format string support  

### EndsWith(str string, endsWith string) (string, bool)
**Lines:** 56-70  
**Purpose:** Returns prefix and boolean if string ends with suffix  

### SmallSnakeCaseText(str string) string
**Lines:** 72-75  
**Purpose:** Transforms string to lowercase snake_case format  

## Security Analysis

### 1. Type Assertion Vulnerability - CRITICAL
**Severity:** HIGH  
**Lines:** 44  
**Issue:** Unhandled type assertion that can cause application panic.

```go
fmtString := message[0].(string)
```

**Risk:**
- Application crash if first message parameter is not a string
- Denial of service attacks through malformed function calls
- Runtime instability

**Impact:** Service unavailability through panic.

### 2. Format String Injection Vulnerability
**Severity:** MEDIUM  
**Lines:** 50  
**Issue:** User-controlled format strings passed to logging function.

```go
log.Errorf(fmtString+": %v", args...)
```

**Risk:**
- Format string attacks through malicious error messages
- Information disclosure via format specifiers
- Log injection attacks

### 3. Array Bounds Safety Issue
**Severity:** MEDIUM  
**Lines:** 44  
**Issue:** Direct access to message[0] without bounds checking.

**Risk:**
- Index out of bounds panic if message slice is empty
- Application crash on malformed input

### 4. String Processing Edge Cases
**Severity:** LOW  
**Lines:** Throughout string functions  
**Issue:** String functions don't validate input parameters.

**Missing Validations:**
- No null string checks
- No length validation
- No Unicode handling considerations

### 5. Error Information Leakage
**Severity:** LOW  
**Lines:** 49-50  
**Issue:** Error details included in log messages without sanitization.

**Risk:**
- Sensitive information exposure in logs
- Internal system details in error messages

## Potential Attack Vectors

### Format String Attacks
1. **Log Injection:** Include format specifiers in error messages to manipulate logs
2. **Information Disclosure:** Use format strings to extract memory information
3. **DoS Attacks:** Craft format strings that cause excessive processing

### Parameter Manipulation
1. **Empty Arrays:** Call functions with empty message arrays to trigger panics
2. **Type Confusion:** Pass non-string first parameters to cause type assertion failures
3. **Large Strings:** Submit extremely long strings to cause memory issues

## Recommendations

### Immediate Actions
1. **Add Parameter Validation:** Check message array length before accessing elements
2. **Safe Type Conversion:** Replace type assertion with safe string conversion
3. **Sanitize Format Strings:** Validate format strings before logging
4. **Add Input Validation:** Validate string parameters in utility functions

### Enhanced Implementation

```go
package hostswitch

import (
    "fmt"
    "strings"
    "unicode/utf8"
    "github.com/artpar/conform"
    jsoniter "github.com/json-iterator/go"
    log "github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
    MaxStringLength = 10000
    MaxLogMessageLength = 1000
)

// SafeEndsWithCheck performs secure string suffix checking
func SafeEndsWithCheck(str string, endsWith string) (bool, error) {
    if err := validateString(str, "str"); err != nil {
        return false, err
    }
    if err := validateString(endsWith, "endsWith"); err != nil {
        return false, err
    }
    
    if len(endsWith) > len(str) {
        return false, nil
    }
    
    if len(endsWith) == len(str) {
        return endsWith == str, nil
    }
    
    suffix := str[len(str)-len(endsWith):]
    return suffix == endsWith, nil
}

// SafeBeginsWithCheck performs secure string prefix checking
func SafeBeginsWithCheck(str string, beginsWith string) (bool, error) {
    if err := validateString(str, "str"); err != nil {
        return false, err
    }
    if err := validateString(beginsWith, "beginsWith"); err != nil {
        return false, err
    }
    
    if len(beginsWith) > len(str) {
        return false, nil
    }
    
    if len(beginsWith) == len(str) {
        return beginsWith == str, nil
    }
    
    prefix := str[:len(beginsWith)]
    return prefix == beginsWith, nil
}

// validateString validates string parameters
func validateString(s string, paramName string) error {
    if len(s) > MaxStringLength {
        return fmt.Errorf("%s too long: %d characters", paramName, len(s))
    }
    
    if !utf8.ValidString(s) {
        return fmt.Errorf("%s contains invalid UTF-8", paramName)
    }
    
    return nil
}

// SafeCheckErr provides secure error logging with validation
func SafeCheckErr(err error, message ...interface{}) bool {
    if err == nil {
        return false
    }
    
    // Validate message parameters
    if len(message) == 0 {
        log.Errorf("Error occurred: %v", err)
        return true
    }
    
    // Safe format string extraction
    fmtString, err2 := safeStringConversion(message[0])
    if err2 != nil {
        log.Errorf("Error with invalid format string: %v", err)
        return true
    }
    
    // Sanitize format string
    sanitizedFormat := sanitizeFormatString(fmtString)
    
    // Limit message length
    if len(sanitizedFormat) > MaxLogMessageLength {
        sanitizedFormat = sanitizedFormat[:MaxLogMessageLength] + "..."
    }
    
    args := make([]interface{}, 0)
    if len(message) > 1 {
        args = message[1:]
    }
    args = append(args, err)
    
    log.Errorf(sanitizedFormat+": %v", args...)
    return true
}

// safeStringConversion safely converts interface{} to string
func safeStringConversion(value interface{}) (string, error) {
    if value == nil {
        return "", fmt.Errorf("value is nil")
    }
    
    switch v := value.(type) {
    case string:
        return v, nil
    case fmt.Stringer:
        return v.String(), nil
    default:
        return fmt.Sprintf("%v", value), nil
    }
}

// sanitizeFormatString removes dangerous format specifiers
func sanitizeFormatString(format string) string {
    // Remove potentially dangerous format specifiers
    dangerous := []string{"%p", "%x", "%X", "%#x", "%#X"}
    
    for _, d := range dangerous {
        format = strings.ReplaceAll(format, d, "%s")
    }
    
    // Limit number of format specifiers
    count := strings.Count(format, "%")
    if count > 10 {
        // Truncate format string if too many specifiers
        return "Error message too complex"
    }
    
    return format
}

// SafeEndsWith returns prefix and boolean with validation
func SafeEndsWith(str string, endsWith string) (string, bool, error) {
    if err := validateString(str, "str"); err != nil {
        return "", false, err
    }
    if err := validateString(endsWith, "endsWith"); err != nil {
        return "", false, err
    }
    
    if len(endsWith) > len(str) {
        return "", false, nil
    }
    
    if len(endsWith) == len(str) {
        if endsWith == str {
            return "", true, nil
        }
        return "", false, nil
    }
    
    suffix := str[len(str)-len(endsWith):]
    prefix := str[:len(str)-len(endsWith)]
    return prefix, suffix == endsWith, nil
}

// SafeSmallSnakeCaseText performs secure text transformation
func SafeSmallSnakeCaseText(str string) (string, error) {
    if err := validateString(str, "str"); err != nil {
        return "", err
    }
    
    // Additional validation for transformation input
    if strings.ContainsAny(str, "<>\"'&") {
        return "", fmt.Errorf("string contains dangerous characters")
    }
    
    transformed := conform.TransformString(str, "lower,snake")
    
    // Validate transformed output
    if err := validateString(transformed, "transformed"); err != nil {
        return "", fmt.Errorf("transformation resulted in invalid string: %v", err)
    }
    
    return transformed, nil
}

// Backward compatibility functions with logging warnings
func EndsWithCheck(str string, endsWith string) bool {
    result, err := SafeEndsWithCheck(str, endsWith)
    if err != nil {
        log.Warnf("EndsWithCheck deprecated function called with invalid input: %v", err)
        return false
    }
    return result
}

func BeginsWithCheck(str string, beginsWith string) bool {
    result, err := SafeBeginsWithCheck(str, beginsWith)
    if err != nil {
        log.Warnf("BeginsWithCheck deprecated function called with invalid input: %v", err)
        return false
    }
    return result
}

func CheckErr(err error, message ...interface{}) bool {
    log.Warn("CheckErr deprecated function called, use SafeCheckErr instead")
    return SafeCheckErr(err, message...)
}

func EndsWith(str string, endsWith string) (string, bool) {
    prefix, matches, err := SafeEndsWith(str, endsWith)
    if err != nil {
        log.Warnf("EndsWith deprecated function called with invalid input: %v", err)
        return "", false
    }
    return prefix, matches
}

func SmallSnakeCaseText(str string) string {
    result, err := SafeSmallSnakeCaseText(str)
    if err != nil {
        log.Warnf("SmallSnakeCaseText deprecated function called with invalid input: %v", err)
        return str // Return original on error
    }
    return result
}
```

### Long-term Improvements
1. **Input Validation Framework:** Comprehensive input validation for all functions
2. **Structured Logging:** Replace format string logging with structured logging
3. **Error Handling Standards:** Consistent error handling patterns
4. **Unicode Support:** Proper Unicode string handling
5. **Performance Optimization:** Optimize string operations for large inputs

## Edge Cases Identified

1. **Empty Strings:** Functions called with empty string parameters
2. **Null Parameters:** Functions called with nil or empty interface{} values
3. **Unicode Strings:** Strings containing non-ASCII characters
4. **Very Long Strings:** Extremely long input strings
5. **Invalid UTF-8:** Strings with invalid byte sequences
6. **Format String Edge Cases:** Complex format strings with many specifiers
7. **Concurrent Access:** Thread safety for string operations
8. **Memory Exhaustion:** Large strings causing memory issues

## Security Best Practices Violations

1. **No input validation**
2. **Unsafe type assertions**
3. **Format string vulnerabilities**
4. **No bounds checking**
5. **Information disclosure in logs**

## Impact Assessment

- **Runtime Safety:** HIGH RISK - Type assertion panics
- **Log Security:** MEDIUM RISK - Format string injection
- **Input Validation:** MEDIUM RISK - No string validation
- **Information Security:** LOW RISK - Limited information disclosure

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** High - Type assertion vulnerability and format string issues require attention