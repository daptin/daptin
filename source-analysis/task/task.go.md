# Security Analysis: server/task/task.go

**File:** `server/task/task.go`  
**Type:** Task data structure definition  
**Lines of Code:** 15  

## Overview
This file defines a simple Task struct that represents scheduled tasks in the Daptin system. The structure contains task metadata including scheduling information, user context, and task attributes.

## Key Components

### Task struct
**Lines:** 3-14  
**Purpose:** Data structure representing a scheduled task with user context and attributes  

## Security Analysis

### 1. MEDIUM: Generic Interface Map - MEDIUM RISK
**Severity:** MEDIUM  
**Line:** 9  
**Issue:** Generic interface{} map for task attributes lacks type safety.

```go
Attributes map[string]interface{}  // Generic interface map without validation
```

**Risk:**
- **Type confusion** in attribute handling
- **No validation** of attribute types or values
- **Potential injection** through untyped attributes
- **Runtime errors** from unexpected attribute types
- **Memory exhaustion** through large attribute objects

### 2. MEDIUM: User Context Security - MEDIUM RISK
**Severity:** MEDIUM  
**Line:** 10  
**Issue:** User email stored as plain string without validation.

```go
AsUserEmail string  // No email validation or sanitization
```

**Risk:**
- **Email injection** through malformed email addresses
- **Privilege escalation** through crafted email values
- **No format validation** for email addresses
- **Impersonation attacks** through email manipulation

### 3. MEDIUM: Action and Entity Name Injection - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 11-12  
**Issue:** Action and entity names without validation.

```go
ActionName string  // No validation of action names
EntityName string  // No validation of entity names
```

**Risk:**
- **Command injection** through crafted action names
- **Entity enumeration** through entity name manipulation
- **Privilege escalation** through unauthorized action names
- **SQL injection** if names are used in database queries

### 4. LOW: JSON Attribute Storage - LOW RISK
**Severity:** LOW  
**Line:** 13  
**Issue:** JSON attributes stored as string without validation.

```go
AttributesJson string  // Raw JSON string without validation
```

**Risk:**
- **JSON injection** through malformed JSON
- **Parser vulnerabilities** from crafted JSON
- **Size limitations** not enforced
- **Encoding issues** with special characters

### 5. LOW: Schedule Format Security - LOW RISK
**Severity:** LOW  
**Line:** 6  
**Issue:** Schedule stored as string without format validation.

```go
Schedule string  // No cron expression validation
```

**Risk:**
- **Schedule manipulation** through invalid expressions
- **Parser exploitation** in schedule processing
- **Resource exhaustion** through complex schedules
- **Timing attacks** through predictable schedules

## Potential Attack Vectors

### Task Manipulation Attacks
1. **Attribute Injection:** Inject malicious data through Attributes map
2. **User Impersonation:** Use crafted email addresses for privilege escalation
3. **Action Hijacking:** Use unauthorized action names to execute privileged operations
4. **Entity Enumeration:** Discover system entities through entity name manipulation

### Data Injection Attacks
1. **JSON Injection:** Inject malicious JSON through AttributesJson field
2. **Schedule Manipulation:** Use invalid or malicious cron expressions
3. **Name Injection:** Inject malicious content through string fields

### Resource Exhaustion Attacks
1. **Large Attributes:** Submit extremely large attribute maps
2. **Complex JSON:** Use deeply nested JSON structures
3. **Memory Flooding:** Create tasks with excessive data

## Recommendations

### Immediate Actions
1. **Add Input Validation:** Validate all string fields for format and content
2. **Type Safety:** Add validation for Attributes map content
3. **Email Validation:** Implement proper email format validation
4. **JSON Validation:** Add JSON format and size validation

### Enhanced Security Implementation

```go
package task

import (
    "encoding/json"
    "fmt"
    "regexp"
    "strings"
    "unicode/utf8"
)

const (
    MaxNameLength = 255
    MaxEmailLength = 320 // RFC 5321 limit
    MaxScheduleLength = 100
    MaxAttributesSize = 64 * 1024 // 64KB
    MaxAttributeCount = 100
    MaxReferenceIdLength = 64
)

var (
    validEmailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
    validNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]*$`)
    validCronPattern = regexp.MustCompile(`^(\*|[0-5]?\d)(\s+(\*|[0-5]?\d)){4}$|^@(annually|yearly|monthly|weekly|daily|hourly)$`)
    validReferenceIdPattern = regexp.MustCompile(`^[a-fA-F0-9-]{36}$`)
)

// Task represents a secure scheduled task
type Task struct {
    Id             int64                  `json:"id"`
    ReferenceId    string                 `json:"reference_id" validate:"required,reference_id"`
    Schedule       string                 `json:"schedule" validate:"required,cron"`
    Active         bool                   `json:"active"`
    Name           string                 `json:"name" validate:"required,task_name"`
    Attributes     map[string]interface{} `json:"attributes" validate:"attributes"`
    AsUserEmail    string                 `json:"as_user_email" validate:"required,email"`
    ActionName     string                 `json:"action_name" validate:"required,action_name"`
    EntityName     string                 `json:"entity_name" validate:"required,entity_name"`
    AttributesJson string                 `json:"attributes_json" validate:"json"`
}

// Validate validates the entire Task structure
func (t *Task) Validate() error {
    if err := t.validateReferenceId(); err != nil {
        return fmt.Errorf("invalid reference ID: %v", err)
    }
    
    if err := t.validateSchedule(); err != nil {
        return fmt.Errorf("invalid schedule: %v", err)
    }
    
    if err := t.validateName(); err != nil {
        return fmt.Errorf("invalid name: %v", err)
    }
    
    if err := t.validateAttributes(); err != nil {
        return fmt.Errorf("invalid attributes: %v", err)
    }
    
    if err := t.validateAsUserEmail(); err != nil {
        return fmt.Errorf("invalid user email: %v", err)
    }
    
    if err := t.validateActionName(); err != nil {
        return fmt.Errorf("invalid action name: %v", err)
    }
    
    if err := t.validateEntityName(); err != nil {
        return fmt.Errorf("invalid entity name: %v", err)
    }
    
    if err := t.validateAttributesJson(); err != nil {
        return fmt.Errorf("invalid attributes JSON: %v", err)
    }
    
    return nil
}

// validateReferenceId validates the reference ID format
func (t *Task) validateReferenceId() error {
    if len(t.ReferenceId) == 0 {
        return fmt.Errorf("reference ID cannot be empty")
    }
    
    if len(t.ReferenceId) > MaxReferenceIdLength {
        return fmt.Errorf("reference ID too long: %d", len(t.ReferenceId))
    }
    
    if !validReferenceIdPattern.MatchString(t.ReferenceId) {
        return fmt.Errorf("invalid reference ID format")
    }
    
    return nil
}

// validateSchedule validates cron expression format
func (t *Task) validateSchedule() error {
    if len(t.Schedule) == 0 {
        return fmt.Errorf("schedule cannot be empty")
    }
    
    if len(t.Schedule) > MaxScheduleLength {
        return fmt.Errorf("schedule too long: %d", len(t.Schedule))
    }
    
    if !utf8.ValidString(t.Schedule) {
        return fmt.Errorf("schedule contains invalid UTF-8")
    }
    
    // Basic cron validation - in production, use a proper cron parser
    schedule := strings.TrimSpace(t.Schedule)
    if !validCronPattern.MatchString(schedule) {
        return fmt.Errorf("invalid cron expression format")
    }
    
    return nil
}

// validateName validates task name
func (t *Task) validateName() error {
    if len(t.Name) == 0 {
        return fmt.Errorf("name cannot be empty")
    }
    
    if len(t.Name) > MaxNameLength {
        return fmt.Errorf("name too long: %d", len(t.Name))
    }
    
    if !utf8.ValidString(t.Name) {
        return fmt.Errorf("name contains invalid UTF-8")
    }
    
    if !validNamePattern.MatchString(t.Name) {
        return fmt.Errorf("invalid name format")
    }
    
    return nil
}

// validateAttributes validates the attributes map
func (t *Task) validateAttributes() error {
    if t.Attributes == nil {
        return nil // Nil is acceptable
    }
    
    if len(t.Attributes) > MaxAttributeCount {
        return fmt.Errorf("too many attributes: %d", len(t.Attributes))
    }
    
    // Estimate size by marshaling to JSON
    jsonData, err := json.Marshal(t.Attributes)
    if err != nil {
        return fmt.Errorf("attributes not JSON serializable: %v", err)
    }
    
    if len(jsonData) > MaxAttributesSize {
        return fmt.Errorf("attributes too large: %d bytes", len(jsonData))
    }
    
    // Validate attribute keys and values
    for key, value := range t.Attributes {
        if err := validateAttributeKey(key); err != nil {
            return fmt.Errorf("invalid attribute key '%s': %v", key, err)
        }
        
        if err := validateAttributeValue(value); err != nil {
            return fmt.Errorf("invalid attribute value for key '%s': %v", key, err)
        }
    }
    
    return nil
}

// validateAttributeKey validates attribute keys
func validateAttributeKey(key string) error {
    if len(key) == 0 {
        return fmt.Errorf("attribute key cannot be empty")
    }
    
    if len(key) > 100 {
        return fmt.Errorf("attribute key too long: %d", len(key))
    }
    
    if !utf8.ValidString(key) {
        return fmt.Errorf("attribute key contains invalid UTF-8")
    }
    
    // Allow alphanumeric, underscore, hyphen, dot
    for _, r := range key {
        if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
             (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
            return fmt.Errorf("attribute key contains invalid characters")
        }
    }
    
    return nil
}

// validateAttributeValue validates attribute values
func validateAttributeValue(value interface{}) error {
    if value == nil {
        return nil // Nil is acceptable
    }
    
    switch v := value.(type) {
    case string:
        if len(v) > 10000 {
            return fmt.Errorf("string attribute too long: %d", len(v))
        }
        if !utf8.ValidString(v) {
            return fmt.Errorf("string attribute contains invalid UTF-8")
        }
    case map[string]interface{}:
        // Recursively validate nested maps
        for key, val := range v {
            if err := validateAttributeKey(key); err != nil {
                return err
            }
            if err := validateAttributeValue(val); err != nil {
                return err
            }
        }
    case []interface{}:
        if len(v) > 1000 {
            return fmt.Errorf("array attribute too large: %d elements", len(v))
        }
        // Validate array elements
        for _, val := range v {
            if err := validateAttributeValue(val); err != nil {
                return err
            }
        }
    case bool, int, int64, float64:
        // These types are acceptable
    default:
        return fmt.Errorf("unsupported attribute type: %T", value)
    }
    
    return nil
}

// validateAsUserEmail validates email format
func (t *Task) validateAsUserEmail() error {
    if len(t.AsUserEmail) == 0 {
        return fmt.Errorf("user email cannot be empty")
    }
    
    if len(t.AsUserEmail) > MaxEmailLength {
        return fmt.Errorf("user email too long: %d", len(t.AsUserEmail))
    }
    
    if !utf8.ValidString(t.AsUserEmail) {
        return fmt.Errorf("user email contains invalid UTF-8")
    }
    
    if !validEmailPattern.MatchString(t.AsUserEmail) {
        return fmt.Errorf("invalid email format")
    }
    
    return nil
}

// validateActionName validates action name
func (t *Task) validateActionName() error {
    if len(t.ActionName) == 0 {
        return fmt.Errorf("action name cannot be empty")
    }
    
    if len(t.ActionName) > MaxNameLength {
        return fmt.Errorf("action name too long: %d", len(t.ActionName))
    }
    
    if !utf8.ValidString(t.ActionName) {
        return fmt.Errorf("action name contains invalid UTF-8")
    }
    
    if !validNamePattern.MatchString(t.ActionName) {
        return fmt.Errorf("invalid action name format")
    }
    
    return nil
}

// validateEntityName validates entity name
func (t *Task) validateEntityName() error {
    if len(t.EntityName) == 0 {
        return fmt.Errorf("entity name cannot be empty")
    }
    
    if len(t.EntityName) > MaxNameLength {
        return fmt.Errorf("entity name too long: %d", len(t.EntityName))
    }
    
    if !utf8.ValidString(t.EntityName) {
        return fmt.Errorf("entity name contains invalid UTF-8")
    }
    
    if !validNamePattern.MatchString(t.EntityName) {
        return fmt.Errorf("invalid entity name format")
    }
    
    return nil
}

// validateAttributesJson validates JSON string
func (t *Task) validateAttributesJson() error {
    if len(t.AttributesJson) == 0 {
        return nil // Empty is acceptable
    }
    
    if len(t.AttributesJson) > MaxAttributesSize {
        return fmt.Errorf("attributes JSON too large: %d bytes", len(t.AttributesJson))
    }
    
    if !utf8.ValidString(t.AttributesJson) {
        return fmt.Errorf("attributes JSON contains invalid UTF-8")
    }
    
    // Validate JSON format
    var temp interface{}
    if err := json.Unmarshal([]byte(t.AttributesJson), &temp); err != nil {
        return fmt.Errorf("invalid JSON format: %v", err)
    }
    
    return nil
}

// Sanitize sanitizes task data for safe usage
func (t *Task) Sanitize() {
    t.ReferenceId = strings.TrimSpace(t.ReferenceId)
    t.Schedule = strings.TrimSpace(t.Schedule)
    t.Name = strings.TrimSpace(t.Name)
    t.AsUserEmail = strings.ToLower(strings.TrimSpace(t.AsUserEmail))
    t.ActionName = strings.TrimSpace(t.ActionName)
    t.EntityName = strings.TrimSpace(t.EntityName)
    t.AttributesJson = strings.TrimSpace(t.AttributesJson)
}

// IsValid returns true if the task passes all validations
func (t *Task) IsValid() bool {
    return t.Validate() == nil
}

// GetSanitizedCopy returns a sanitized copy of the task
func (t *Task) GetSanitizedCopy() *Task {
    // Create a copy
    copy := *t
    
    // Deep copy attributes map
    if t.Attributes != nil {
        copy.Attributes = make(map[string]interface{})
        for k, v := range t.Attributes {
            copy.Attributes[k] = v
        }
    }
    
    // Sanitize the copy
    copy.Sanitize()
    
    return &copy
}
```

### Long-term Improvements
1. **Task Execution Security:** Implement secure task execution with sandboxing
2. **Audit Logging:** Log all task creation, modification, and execution
3. **Permission System:** Integrate with user permission system for action authorization
4. **Rate Limiting:** Add rate limiting for task creation and execution
5. **Schedule Validation:** Use proper cron expression parser and validator

## Edge Cases Identified

1. **Empty Task Fields:** Handling of empty or missing required fields
2. **Large Attribute Maps:** Performance with very large attribute collections
3. **Malformed JSON:** Various invalid JSON patterns in AttributesJson
4. **Unicode Content:** Task fields with unicode characters
5. **Circular References:** Attributes with circular references
6. **Deep Nesting:** Deeply nested attribute structures
7. **Email Variations:** Different email address formats and edge cases
8. **Schedule Complexity:** Complex cron expressions and edge cases

## Security Best Practices Violations

1. **No input validation for any fields**
2. **Generic interface{} map without type safety**
3. **No email format validation**
4. **No JSON validation for AttributesJson field**
5. **Missing size limits on all string fields**

## Positive Security Aspects

1. **Simple Structure:** Minimal complexity reduces attack surface
2. **Clear Data Separation:** Separate fields for different data types
3. **Immutable Design:** Structure-only definition without operations

## Critical Issues Summary

1. **Generic Interface Map:** Attributes map lacks type safety and validation
2. **User Context Security:** No validation of user email format
3. **Action/Entity Name Injection:** No validation of action and entity names
4. **JSON Attribute Storage:** No validation of JSON content and format
5. **Schedule Format Security:** No validation of cron expression format

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** MEDIUM - Data structure requiring comprehensive input validation for security