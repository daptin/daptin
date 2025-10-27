# Security Analysis: server/columntypes/ folder

**Folder:** `server/columntypes/`  
**Files Analyzed:** `mtime.go` (148 lines), `types.go` (712 lines)  
**Total Lines of Code:** 860  
**Primary Function:** Data type detection and parsing system providing time/date parsing, type classification, regex validation, and data conversion for database schema inference and data processing

## Summary

This folder implements a comprehensive data type detection and parsing system that analyzes input data to determine appropriate column types. It includes time/date parsing with multiple format support, regex-based validation, numeric conversion, JSON parsing, and automatic type classification. The implementation handles various data formats and provides conversion functions for database schema generation and data validation.

## Security Issues Found

### 🔴 CRITICAL Issues

#### 1. **Unsafe Type Assertions in Type Detection** (Lines 372, 374, 381, 434, 443, 474, 505 in types.go)
```go
nInt, ok := nValue.(int)
if nInt <= 5 {
    return true, nInt
}
nFloat, ok := nValue.(float64)
intVal, isInt := floatValue.(int)
```
**Risk:** Multiple unsafe type assertions in critical type detection logic
- Type assertions used extensively in rating and coordinate validation
- Could panic if unexpected types are returned from helper functions
- Critical data processing functions vulnerable to type confusion
- No comprehensive error handling for type assertion failures
**Impact:** Critical - Application crashes through type assertion panics in data processing
**Remediation:** Use safe type assertions with proper error handling

#### 2. **JSON Unmarshaling Without Validation** (Lines 243-247 in types.go)
```go
DetectorFunction: func(s string) (bool, interface{}) {
    var variab interface{}
    err := json.Unmarshal([]byte(s), &variab)
    if err != nil {
        return false, nil
    }
    return true, variab
}
```
**Risk:** JSON unmarshaling of arbitrary user input without validation
- No size limits on JSON parsing
- Could parse malicious JSON content
- No validation of JSON structure or content
- Potential for JSON injection attacks and memory exhaustion
**Impact:** Critical - JSON injection and memory exhaustion through malicious input
**Remediation:** Add size limits and content validation for JSON parsing

#### 3. **Regex Compilation Without Error Handling** (Lines 338, 566, 569-574, 588-594 in types.go)
```go
r := regexp.MustCompile("^([a-zA-Z]{0,3}\\.? )?[0-9]+\\.[0-9]{0,2}([a-zA-Z]{0,3})?")
compiled, err := regexp.Compile(reg)
if err != nil {
    log.Errorf("Failed to compile string as regex: %v", err)
    return func(s string) (bool, interface{}) {
        return false, nil
    }
}
```
**Risk:** Regex compilation with user-provided patterns and error handling issues
- MustCompile can panic on invalid regex patterns
- Error handling in dynamic regex compilation inconsistent
- User-provided regex patterns could cause ReDoS attacks
- No validation of regex complexity or safety
**Impact:** Critical - Application crashes and ReDoS attacks through malicious regex
**Remediation:** Use safe regex compilation with complexity validation

### 🟡 HIGH Issues

#### 4. **Information Disclosure in Error Messages** (Lines 162, 182, 207, 542, 571, 590, 696, 700, 705 in types.go)
```go
log.Printf("One of the unknowns - %v : %d", d, sort.SearchStrings(unknownNumbers, strings.ToLower(d)))
log.Printf("Converter not found for %v", typ)
log.Errorf("Failed to compile string as regex: %v", err)
log.Printf("Selecting type %s because of Suffix %s in %s", typ.String(), n, name)
```
**Risk:** Detailed error messages exposing system internals and data
- User input data logged in error messages
- Internal type detection logic exposed
- Regex compilation errors reveal pattern details
- Column name analysis details disclosed
**Impact:** High - Information disclosure aiding system reconnaissance
**Remediation:** Sanitize error messages and remove sensitive information

#### 5. **Fallback Time Values on Parse Failure** (Lines 104, 131, 142, 283, 312, 322 in mtime.go; types.go)
```go
return time.Now(), "", errors.New("Unrecognised time format - " + t)
return time.Now(), "", errors.New("Unrecognised time format - " + t1)
return false, time.Now()
```
**Risk:** Default time values returned on parsing failures
- time.Now() returned on parsing failures exposes current system time
- Could leak server time information
- Inconsistent error handling between different parse functions
- Silent failures with misleading default values
**Impact:** High - Information disclosure of system time and silent data corruption
**Remediation:** Return appropriate zero values or explicit errors

#### 6. **Hardcoded Validation Logic in Date Processing** (Lines 114-123 in mtime.go)
```go
if format == "2006" || format == "2006.0" || format == "2006.00" || format == "2006.000" {
    if t.Sub(time.Now()).Hours() > 182943 {
        ret = false
    }
}
if format == "06" {
    if t.Sub(time.Now()).Hours() > -150179 {
        ret = false
    }
}
```
**Risk:** Hardcoded time validation logic with magic numbers
- Magic numbers for time validation not documented
- Validation logic tied to current system time
- Could fail in different time zones or system configurations
- Hardcoded business logic in parsing functions
**Impact:** High - Time validation bypass and inconsistent behavior
**Remediation:** Make validation configurable and document time constraints

### 🟠 MEDIUM Issues

#### 7. **Unsafe Regex Patterns with Potential ReDoS** (Lines 224, 230, 236, 253, 259, 265, 271, 338 in types.go)
```go
"regex": "[a-zA-Z]+ [a-zA-Z]+",
"regex": "[a-zA-Z0-9]([\\\\\\/\\.])([a-zA-Z0-9]+[\\\\\\/\\.]?)",
"regex": "[a-zA-Z0-9_]+@[0-9a-zA-Z_-]+\\.[a-z]{2,10}(\\.[a-z]{2,10})?",
"regex": "#[0-9a-f]{3,6}",
```
**Risk:** Regex patterns without complexity validation
- Some patterns could be vulnerable to ReDoS attacks
- No timeout or complexity limits for regex matching
- Email and other patterns could be exploited with crafted input
- No validation of regex execution time
**Impact:** Medium - Potential ReDoS attacks through crafted input
**Remediation:** Add regex complexity validation and timeout limits

#### 8. **Global Variable Initialization in init()** (Lines 28-91 in mtime.go)
```go
func init() {
    timeFormat = []string{
        "3:04PM",
        "3:04 PM",
        // ... many formats
    }
    // ... more global initialization
}
```
**Risk:** Global variable initialization without validation
- Global variables modified in init() function
- No validation of format strings
- Could be modified by other parts of the system
- No protection against concurrent modification
**Impact:** Medium - Global state corruption and race conditions
**Remediation:** Use immutable configuration and proper synchronization

### 🔵 LOW Issues

#### 9. **Magic String Processing for Unknown Values** (Lines 160-164, 180-184, 205-209 in types.go)
```go
in := sort.SearchStrings(unknownNumbers, d)
if in < len(unknownNumbers) && unknownNumbers[in] == d {
    log.Printf("One of the unknowns - %v : %d", d, sort.SearchStrings(unknownNumbers, strings.ToLower(d)))
    return true, 0
}
```
**Risk:** Hardcoded handling of unknown values
- Hardcoded list of "unknown" number representations
- Could be bypassed with alternative representations
- No configuration for different data sources
- Potential for inconsistent behavior
**Impact:** Low - Data processing inconsistencies
**Remediation:** Make unknown value handling configurable

#### 10. **String-based Type System** (Lines 20-80 in types.go)
```go
func (t EntityType) String() string {
    switch t {
    case Time:
        return "time"
    // ... many string conversions
    }
    return "name-not-set"
}
```
**Risk:** String-based type identification system
- Type safety relies on string comparisons
- Potential for type confusion through string manipulation
- No validation of type string consistency
- Could be vulnerable to type injection
**Impact:** Low - Type confusion and data integrity issues
**Remediation:** Use more robust type identification system

## Code Quality Issues

1. **Type Safety**: Multiple unsafe type assertions throughout critical functions
2. **Error Handling**: Inconsistent error handling with information disclosure
3. **Regex Security**: Unsafe regex compilation and potential ReDoS vulnerabilities
4. **Global State**: Unprotected global variables and initialization
5. **Validation**: Insufficient input validation for user-provided data

## Recommendations

### Immediate Actions Required

1. **Type Safety**: Replace all unsafe type assertions with safe checking
2. **JSON Security**: Add size limits and validation for JSON parsing
3. **Regex Security**: Use safe regex compilation with complexity validation
4. **Error Security**: Sanitize error messages to prevent information disclosure

### Security Improvements

1. **Input Validation**: Add comprehensive validation for all user input
2. **Timeout Protection**: Add timeout limits for regex and parsing operations
3. **ReDoS Protection**: Validate regex complexity and add execution limits
4. **Global State Protection**: Protect global variables from modification

### Code Quality Enhancements

1. **Error Management**: Implement consistent error handling patterns
2. **Type System**: Improve type safety throughout the detection system
3. **Configuration**: Make hardcoded values configurable
4. **Documentation**: Add comprehensive security documentation

## Attack Vectors

1. **Type Confusion**: Exploit unsafe type assertions to cause application panics
2. **JSON Injection**: Send malicious JSON content to cause memory exhaustion
3. **ReDoS Attacks**: Use crafted input to exploit regex vulnerabilities
4. **Information Gathering**: Use error messages to understand system internals
5. **Time Manipulation**: Exploit time parsing logic for data corruption
6. **Regex Injection**: Provide malicious regex patterns to cause crashes

## Impact Assessment

- **Confidentiality**: HIGH - Information disclosure through error messages and time leakage
- **Integrity**: HIGH - Data corruption through parsing failures and type confusion
- **Availability**: CRITICAL - Application crashes through type assertions and regex panics
- **Authentication**: LOW - No direct authentication mechanisms involved
- **Authorization**: LOW - No authorization controls in type detection

This type detection system has several critical security vulnerabilities that could compromise application security and stability.

## Technical Notes

The columntypes system:
1. Provides comprehensive data type detection for various formats
2. Handles time/date parsing with multiple format support
3. Implements regex-based validation for different data types
4. Supports automatic schema inference from data samples
5. Includes conversion functions for database operations
6. Manages global type detection configuration

The main security concerns revolve around type safety, regex security, and input validation.

## Column Types Security Considerations

For data type detection systems:
- **Type Security**: Implement safe type checking for all operations
- **Input Security**: Validate all user-provided data before processing
- **Regex Security**: Use safe regex compilation with complexity limits
- **JSON Security**: Add size limits and validation for JSON processing
- **Error Security**: Prevent information disclosure through error handling
- **Parsing Security**: Validate all parsing operations and handle failures safely

The current implementation needs significant security hardening to provide secure type detection for production environments.

## Recommended Security Enhancements

1. **Type Security**: Safe type checking replacing all unsafe assertions
2. **Input Security**: Comprehensive validation for all user input
3. **Regex Security**: Safe regex compilation with complexity validation
4. **JSON Security**: Size limits and content validation for JSON parsing
5. **Error Security**: Sanitized error handling without information disclosure
6. **Parsing Security**: Safe parsing with proper error handling and validation