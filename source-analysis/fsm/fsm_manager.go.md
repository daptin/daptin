# Security Analysis: server/fsm/fsm_manager.go

**File:** `server/fsm/fsm_manager.go`  
**Type:** Finite State Machine (FSM) management system  
**Lines of Code:** 249  

## Overview
This file implements a Finite State Machine (FSM) manager that handles state transitions for objects in the Daptin system. It provides functionality to load state machine definitions from the database, manage state instances, and apply state transition events.

## Key Components

### fsmManager struct
**Lines:** 18-20  
**Purpose:** Core FSM manager with database connection  

### StateMachineInstance struct
**Lines:** 22-26  
**Purpose:** Represents a state machine instance for an object  

### getStateMachineInstance()
**Lines:** 28-72  
**Purpose:** Retrieves state machine instance from database  

### stateMachineRunnerFor()
**Lines:** 96-147  
**Purpose:** Creates FSM runner from database configuration  

### ApplyEvent()
**Lines:** 149-185  
**Purpose:** Applies state transition events to objects  

## Security Analysis

### 1. SQL Injection Vulnerabilities - CRITICAL
**Severity:** HIGH  
**Lines:** 30-34, 98-100, 188-189  
**Issue:** Dynamic table name construction without proper validation.

```go
Select("current_state", objType+"_smd", "is_state_of_"+objType, "id", "created_at", "permission").
From(objType + "_state").
Where(goqu.Ex{"is_state_of_" + objType: objId})
```

**Risk:**
- Table names constructed from user input without validation
- Potential SQL injection through objType parameter
- Column names dynamically constructed without sanitization
- Could allow unauthorized database access or schema manipulation

**Impact:** Database compromise, unauthorized data access, potential data corruption.

### 2. Type Assertion Vulnerabilities - CRITICAL
**Severity:** HIGH  
**Lines:** 68, 69, 151, 152  
**Issue:** Multiple unhandled type assertions that can cause application panic.

```go
res.StateMachineId = responseMap[objType+"_smd"].(int64)        // Line 68
res.ObjectId = responseMap["is_state_of_"+objType].(int64)     // Line 69
objType := subject["__type"].(string)                          // Line 151
```

**Risk:** Application crash if database returns unexpected data types.

### 3. JSON Deserialization Without Validation
**Severity:** HIGH  
**Lines:** 130-133  
**Issue:** JSON unmarshaling of state machine events without size or structure validation.

```go
var events []LoopbackEventDesc
err = json.Unmarshal([]byte(jsonValue), &events)
```

**Risk:**
- JSON bomb attacks through malicious state machine definitions
- Memory exhaustion via large or deeply nested JSON
- No validation of event structure or content

### 4. Reference ID Security Issues
**Severity:** MEDIUM  
**Lines:** 152, 154, 186-210  
**Issue:** Reference ID handling without proper validation.

**Risk:**
- Reference ID spoofing attacks
- Unauthorized access to objects through manipulated reference IDs
- No validation that user has permission to access referenced objects

### 5. State Machine Definition Tampering
**Severity:** HIGH  
**Lines:** 98-147  
**Issue:** State machine definitions loaded directly from database without validation.

**Risk:**
- Malicious state machine definitions could be injected
- State transitions could be manipulated to bypass business logic
- No validation of state machine integrity or authorization

### 6. Error Information Disclosure
**Severity:** MEDIUM  
**Lines:** 38, 46, 58, 110, 156, 161, 180-182  
**Issue:** Detailed error messages that could expose system information.

**Risk:**
- Database schema information in error messages
- Internal system details disclosed to users
- Stack traces and debugging information exposure

### 7. Resource Management Issues
**Severity:** MEDIUM  
**Lines:** 44-53, 108-117, 196-205  
**Issue:** Statement preparation and cleanup patterns with potential leaks.

**Risk:**
- Database connection exhaustion
- Memory leaks from unclosed statements
- Resource abuse through repeated operations

## Potential Attack Vectors

### State Machine Manipulation
1. **State Definition Injection:** Inject malicious state machine definitions into database
2. **Transition Bypass:** Manipulate state transitions to bypass business logic
3. **Unauthorized State Changes:** Apply state events without proper authorization

### SQL Injection Attacks
1. **Table Name Injection:** Inject SQL through objType parameters
2. **Column Name Injection:** Manipulate dynamic column name construction
3. **Data Extraction:** Use SQL injection to extract sensitive data

### Resource Exhaustion Attacks
1. **JSON Bomb:** Submit large JSON state machine definitions
2. **Statement Exhaustion:** Exhaust database connections through repeated operations
3. **Memory Exhaustion:** Cause memory exhaustion through large state machine instances

## Recommendations

### Immediate Actions
1. **Add Input Validation:** Validate all input parameters including objType
2. **Fix Type Assertions:** Add proper error handling for type assertions
3. **Validate JSON Input:** Add size and structure limits for JSON deserialization
4. **Sanitize Table Names:** Use allowlist validation for table names

### Enhanced Security Implementation

```go
package fsm

import (
    "context"
    "encoding/json"
    "fmt"
    "regexp"
    "strings"
    "github.com/daptin/daptin/server/database"
    daptinid "github.com/daptin/daptin/server/id"
    "github.com/daptin/daptin/server/statementbuilder"
    "github.com/doug-martin/goqu/v9"
    "github.com/jmoiron/sqlx"
    loopfsm "github.com/looplab/fsm"
    "github.com/pkg/errors"
    log "github.com/sirupsen/logrus"
)

const (
    MaxEventJSONSize = 1024 * 1024 // 1MB limit for event JSON
    MaxEventCount = 1000           // Maximum events per state machine
)

// ValidateObjectType validates object type name for SQL safety
func ValidateObjectType(objType string) error {
    if len(objType) == 0 {
        return fmt.Errorf("object type cannot be empty")
    }
    
    if len(objType) > 50 {
        return fmt.Errorf("object type too long: %d characters", len(objType))
    }
    
    // Only allow alphanumeric and underscore
    matched, _ := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9_]*$", objType)
    if !matched {
        return fmt.Errorf("invalid object type format: %s", objType)
    }
    
    return nil
}

// SafeGetStateMachineInstance with input validation
func (fsm *fsmManager) getStateMachineInstance(objType string, objId int64, machineInstanceId daptinid.DaptinReferenceId) (StateMachineInstance, error) {
    var res StateMachineInstance
    
    // Validate object type
    if err := ValidateObjectType(objType); err != nil {
        return res, fmt.Errorf("invalid object type: %v", err)
    }
    
    // Validate object ID
    if objId <= 0 {
        return res, fmt.Errorf("invalid object ID: %d", objId)
    }
    
    // Validate machine instance ID
    if machineInstanceId == daptinid.NullReferenceId {
        return res, fmt.Errorf("invalid machine instance ID")
    }
    
    s, v, err := statementbuilder.Squirrel.
        Select("current_state", objType+"_smd", "is_state_of_"+objType, "id", "created_at", "permission").
        Prepared(true).
        From(objType + "_state").
        Where(goqu.Ex{"reference_id": machineInstanceId[:]}).
        Where(goqu.Ex{"is_state_of_" + objType: objId}).ToSQL()
    
    if err != nil {
        log.Errorf("Failed to create query for state select: %v", err)
        return res, fmt.Errorf("query creation failed")
    }
    
    responseMap := make(map[string]interface{})
    
    stmt1, err := fsm.db.Preparex(s)
    if err != nil {
        return res, fmt.Errorf("failed to prepare statement")
    }
    defer func(stmt1 *sqlx.Stmt) {
        if err := stmt1.Close(); err != nil {
            log.Errorf("failed to close prepared statement: %v", err)
        }
    }(stmt1)
    
    err = stmt1.QueryRowx(v...).MapScan(responseMap)
    if err != nil {
        log.Errorf("Failed to map scan state row: %v", err)
        return res, fmt.Errorf("state machine instance not found")
    }
    
    // Safe type assertion for current state
    currentState, err := SafeStringExtraction(responseMap["current_state"])
    if err != nil {
        return res, fmt.Errorf("invalid current state format: %v", err)
    }
    res.CurrestState = currentState
    
    // Safe type assertion for state machine ID
    stateMachineId, err := SafeInt64Extraction(responseMap[objType+"_smd"])
    if err != nil {
        return res, fmt.Errorf("invalid state machine ID format: %v", err)
    }
    res.StateMachineId = stateMachineId
    
    // Safe type assertion for object ID
    objectId, err := SafeInt64Extraction(responseMap["is_state_of_"+objType])
    if err != nil {
        return res, fmt.Errorf("invalid object ID format: %v", err)
    }
    res.ObjectId = objectId
    
    return res, nil
}

// SafeStringExtraction safely extracts string from interface{}
func SafeStringExtraction(value interface{}) (string, error) {
    if value == nil {
        return "", fmt.Errorf("value is nil")
    }
    
    switch v := value.(type) {
    case string:
        return v, nil
    case []uint8:
        return string(v), nil
    case []byte:
        return string(v), nil
    default:
        return "", fmt.Errorf("cannot convert %T to string", value)
    }
}

// SafeInt64Extraction safely extracts int64 from interface{}
func SafeInt64Extraction(value interface{}) (int64, error) {
    if value == nil {
        return 0, fmt.Errorf("value is nil")
    }
    
    switch v := value.(type) {
    case int64:
        return v, nil
    case int:
        return int64(v), nil
    case int32:
        return int64(v), nil
    default:
        return 0, fmt.Errorf("cannot convert %T to int64", value)
    }
}

// ValidateEventJSON validates JSON structure and size
func ValidateEventJSON(jsonData string) error {
    if len(jsonData) > MaxEventJSONSize {
        return fmt.Errorf("event JSON too large: %d bytes", len(jsonData))
    }
    
    var events []LoopbackEventDesc
    if err := json.Unmarshal([]byte(jsonData), &events); err != nil {
        return fmt.Errorf("invalid JSON format: %v", err)
    }
    
    if len(events) > MaxEventCount {
        return fmt.Errorf("too many events: %d, maximum allowed: %d", len(events), MaxEventCount)
    }
    
    // Validate each event
    for i, event := range events {
        if err := ValidateEvent(event); err != nil {
            return fmt.Errorf("invalid event at index %d: %v", i, err)
        }
    }
    
    return nil
}

// ValidateEvent validates individual event structure
func ValidateEvent(event LoopbackEventDesc) error {
    if len(event.Name) == 0 {
        return fmt.Errorf("event name cannot be empty")
    }
    
    if len(event.Name) > 100 {
        return fmt.Errorf("event name too long: %d characters", len(event.Name))
    }
    
    // Validate event name format
    matched, _ := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9_]*$", event.Name)
    if !matched {
        return fmt.Errorf("invalid event name format: %s", event.Name)
    }
    
    if len(event.Src) == 0 {
        return fmt.Errorf("event must have source states")
    }
    
    if len(event.Dst) == 0 {
        return fmt.Errorf("event must have destination state")
    }
    
    return nil
}

// Enhanced ApplyEvent with security checks
func (fsm *fsmManager) ApplyEvent(subject map[string]interface{}, stateMachineEvent StateMachineEvent) (string, error) {
    // Validate subject map
    if subject == nil {
        return "", fmt.Errorf("subject cannot be nil")
    }
    
    // Safe type extraction for object type
    objTypeInterface, exists := subject["__type"]
    if !exists {
        return "", fmt.Errorf("subject missing __type field")
    }
    
    objType, err := SafeStringExtraction(objTypeInterface)
    if err != nil {
        return "", fmt.Errorf("invalid __type format: %v", err)
    }
    
    // Validate object type
    if err := ValidateObjectType(objType); err != nil {
        return "", fmt.Errorf("invalid object type: %v", err)
    }
    
    // Safe reference ID extraction
    refIdInterface, exists := subject["reference_id"]
    if !exists {
        return "", fmt.Errorf("subject missing reference_id field")
    }
    
    objReferenceId := daptinid.InterfaceToDIR(refIdInterface)
    if objReferenceId == daptinid.NullReferenceId {
        return "", fmt.Errorf("invalid reference ID")
    }
    
    // Validate state machine event
    if stateMachineEvent == nil {
        return "", fmt.Errorf("state machine event cannot be nil")
    }
    
    eventName := stateMachineEvent.GetEventName()
    if len(eventName) == 0 {
        return "", fmt.Errorf("event name cannot be empty")
    }
    
    // Continue with validated inputs...
    objectIntegerId, err := ReferenceIdToIntegerId(objType, objReferenceId, fsm.db)
    if err != nil {
        return "", fmt.Errorf("failed to resolve object ID: %v", err)
    }
    
    stateMachineInstance, err := fsm.getStateMachineInstance(objType, objectIntegerId, stateMachineEvent.GetStateMachineInstanceId())
    if err != nil {
        return "", fmt.Errorf("failed to get state machine instance: %v", err)
    }
    
    stateMachineRunner, err := fsm.stateMachineRunnerFor(stateMachineInstance.CurrestState, objType, stateMachineInstance.StateMachineId)
    if err != nil {
        return "", fmt.Errorf("failed to create state machine runner: %v", err)
    }
    
    if !stateMachineRunner.Can(eventName) {
        return stateMachineInstance.CurrestState, 
            fmt.Errorf("cannot apply event %s at state %s", eventName, stateMachineInstance.CurrestState)
    }
    
    ctx := context.TODO()
    err = stateMachineRunner.Event(ctx, eventName)
    nextState := stateMachineRunner.Current()
    
    if err != nil && err.Error() != "no transition" {
        return nextState, fmt.Errorf("state transition failed: %v", err)
    }
    
    return nextState, nil
}
```

### Long-term Improvements
1. **Authorization Framework:** Add permission checking for state transitions
2. **Audit Logging:** Log all state machine operations for security monitoring
3. **State Machine Validation:** Validate state machine definitions before loading
4. **Rate Limiting:** Implement rate limiting for state transition operations
5. **Encryption:** Encrypt sensitive state machine data

## Edge Cases Identified

1. **Null Reference IDs:** Handling of null or invalid reference IDs
2. **Malformed State Data:** Database corruption or invalid state information
3. **Circular State Transitions:** State machines with circular transition loops
4. **Large State Machines:** State machines with thousands of states or transitions
5. **Concurrent State Changes:** Multiple threads modifying same state instance
6. **Invalid Object Types:** Non-existent or malformed object type names
7. **JSON Structure Manipulation:** Malicious JSON in state machine definitions
8. **Database Connectivity Issues:** Handling of database connection failures

## Security Best Practices Violations

1. **No input validation**
2. **SQL injection vulnerabilities**
3. **Unhandled type assertions**
4. **No authorization checking**
5. **Information disclosure in errors**
6. **No resource limits**

## Critical Issues Summary

1. **SQL Injection:** Dynamic table/column construction from user input
2. **Type Assertion Panics:** Multiple crash points with invalid data
3. **JSON Bomb Attacks:** Unvalidated JSON deserialization
4. **State Machine Tampering:** No validation of state machine integrity
5. **Authorization Bypass:** No permission checking for state transitions

## Files Requiring Further Review

1. **State machine usage** - How FSM is used throughout the application
2. **Database schema** - State machine table definitions and constraints
3. **Authorization system** - Permission checking for state transitions
4. **Event handlers** - Code that triggers state machine events

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - SQL injection and type assertion vulnerabilities require immediate attention