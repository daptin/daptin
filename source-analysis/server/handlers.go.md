# Security Analysis: server/handlers.go

**File:** `server/handlers.go`  
**Lines of Code:** 211  
**Primary Function:** Finite state machine event handlers providing HTTP endpoints for state machine management, event processing, and state transitions with authentication and permission validation

## Summary

This file implements HTTP handlers for finite state machine (FSM) functionality in the Daptin system. It provides endpoints for creating and executing state machine events, managing state transitions, and handling audit logging. The handlers integrate with the authentication system, permission validation, and database transactions to ensure secure state machine operations. These handlers are critical for workflow and business process management within the application.

## Security Issues Found

### 🔴 CRITICAL Issues

#### 1. **Unsafe Type Assertion Without Error Handling** (Lines 23, 52, 131, 142, 144, 145, 165, 186)
```go
sessionUser := gincontext.Request.Context().Value("user").(*auth.SessionUser)
casted := included.(api2go.Api2GoModel)
requestBodyMap["typeName"].(string)
refId := uuid.MustParse(requestBodyMap["referenceId"].(string))
stateMachineInstance := response.Result().(api2go.Api2GoModel)
```
**Risk:** Multiple unsafe type assertions can panic if types don't match
- Type assertions without proper error checking throughout both handlers
- Panic if user context contains unexpected data types
- Could crash state machine handling with malformed requests or context
- No validation of JSON structure before type assertions
**Impact:** Critical - State machine service crashes through malformed data or context
**Remediation:** Use safe type assertions with ok checks for all type conversions

#### 2. **Multiple UUID Parsing Panics** (Lines 59, 145)
```go
stateMachineId := uuid.MustParse(objectStateMachine.GetID())
refId := uuid.MustParse(requestBodyMap["referenceId"].(string))
```
**Risk:** DoS through malformed UUID input
- `uuid.MustParse` panics on invalid UUID format
- User-controlled input from URL parameters and request body
- No input validation before UUID parsing
**Impact:** High - Denial of service
**Remediation:** Use `uuid.Parse` with proper error handling

#### 3. **Unsafe Type Assertions on User Input** (Lines 44, 52, 107, 144, 165, 186)
```go
objectStateMachine := objectStateMachineResponse.Result().(api2go.Api2GoModel)
casted := included.(api2go.Api2GoModel)
"version": stateObject["version"].(int64) + 1,
typename := requestBodyMap["typeName"].(string)
stateMachineInstance := response.Result().(api2go.Api2GoModel)
subjectInstanceModel := subjectInstanceResponse.Result().(api2go.Api2GoModel).GetAttributes()
```
**Risk:** Multiple application crash points
- Type assertions on database results and user input without validation
- Any malformed database data or request can crash server
- No error handling for type assertion failures
**Impact:** High - Denial of service through malformed data
**Remediation:** Implement safe type assertion patterns

#### 4. **JSON Injection Through Request Body** (Line 142)
```go
json.Unmarshal(requestBodyBytes, &requestBodyMap)
```
**Risk:** Deserialization of untrusted data
- Request body unmarshaled without validation
- No size limits or structure validation
- Attacker-controlled JSON data processed directly
**Impact:** High - Code injection, memory exhaustion
**Remediation:** Validate JSON structure and implement size limits

### 🟡 HIGH Issues

#### 5. **SQL Injection Through State Update** (Lines 104-109)
```go
s, v, err := statementbuilder.Squirrel.Update(typename + "_state").
    Set(goqu.Record{
        "current_state": nextState,
        "version":       stateObject["version"].(int64) + 1,
    }).
    Where(goqu.Ex{"reference_id": stateMachineId}).ToSQL()
```
**Risk:** SQL injection through state values
- `nextState` and `typename` values not validated before SQL construction
- State machine data may contain malicious SQL fragments
- Version calculation based on potentially manipulated data
**Impact:** Medium - SQL injection, data corruption
**Remediation:** Validate and sanitize all values before SQL construction

#### 6. **Transaction Resource Management** (Lines 62-68, 167-173)
```go
transaction, err := db.Beginx()
defer transaction.Commit()
```
**Risk:** Database connection exhaustion
- defer Commit without error checking or rollback
- Early returns bypass transaction cleanup
- Potential connection leaks on error conditions
**Impact:** Medium - Resource exhaustion
**Remediation:** Implement proper transaction cleanup with rollback

#### 7. **Hard-Coded Permission Values** (Line 193)
```go
newStateMachine["permission"] = int64(auth.None | auth.UserRead | auth.UserExecute | auth.GroupCreate | auth.GroupExecute)
```
**Risk:** Insecure default permissions
- Fixed permission bits may be overly permissive
- No validation of permission appropriateness
- May grant unintended access to state machines
**Impact:** Medium - Authorization bypass
**Remediation:** Use configurable, minimal default permissions

### 🟠 MEDIUM Issues

#### 8. **Missing Input Validation** (Lines 34-35, 60, 146)
```go
objectStateMachineUuidString := gincontext.Param("objectStateId")
typename := gincontext.Param("typename")
eventName := gincontext.Param("eventName")
stateMachineUuidString := gincontext.Param("stateMachineId")
```
**Risk:** Parameter injection and validation bypass
- URL parameters used directly without validation
- No format or content validation for critical parameters
- Typename parameter could be manipulated for table injection
**Impact:** Medium - Parameter injection, logic bypass
**Remediation:** Validate all URL parameters and user input

#### 9. **Error Information Disclosure** (Lines 39, 64, 101, 136, 169, 201)
```go
log.Errorf("Failed to get object state machine: %v", err)
resource.CheckErr(err, "Failed to begin transaction [59]")
log.Errorf("Failed to read post body: %v", err)
```
**Risk:** Information leakage through error messages
- Database error details exposed in logs and responses
- Internal system information revealed
- Stack traces may contain sensitive data
**Impact:** Medium - Information disclosure
**Remediation:** Sanitize error messages and use structured logging

### 🔵 LOW Issues

#### 10. **Unused Variable and Dead Code** (Lines 25-31, 48-49, 87-95, 148-156)
```go
pr := &http.Request{
    URL: gincontext.Request.URL,
}
pr.Method = "GET"
// var stateMachineDescriptionInstance *api2go.Api2GoModel (commented out)
```
**Risk:** Code maintenance and clarity issues
- Unused request objects created but not used properly
- Dead code comments indicating incomplete implementation
- May indicate incomplete security controls
**Impact:** Low - Code maintainability
**Remediation:** Remove unused code and complete implementations

## Code Quality Issues

1. **Type Safety**: Extensive unsafe type assertions without validation
2. **Error Handling**: Inconsistent error handling and information disclosure
3. **Resource Management**: Improper transaction lifecycle management
4. **Input Validation**: Missing validation for URL parameters and request body
5. **Security Controls**: Hard-coded permissions and missing authorization checks

## Recommendations

### Immediate Actions Required

1. **Fix Type Assertions**: Implement safe type assertion patterns throughout
2. **UUID Validation**: Replace MustParse with proper error handling
3. **Authentication Safety**: Add nil checks and safe assertion for session user
4. **JSON Validation**: Implement request body validation and size limits

### Security Improvements

1. **Input Validation**: Validate all URL parameters and request body content
2. **SQL Security**: Validate all values before SQL construction
3. **Permission Security**: Review and configure appropriate default permissions
4. **Error Handling**: Sanitize error messages and implement proper logging

### Code Quality Enhancements

1. **Transaction Management**: Implement consistent transaction patterns with rollback
2. **Code Cleanup**: Remove unused variables and complete implementations
3. **Logging**: Use structured logging without sensitive information
4. **Testing**: Add unit tests for state machine security scenarios

## Attack Vectors

1. **DoS via Malformed Input**: Crash server through invalid UUIDs or type assertions
2. **Authentication Bypass**: Exploit type assertion vulnerabilities in auth handling
3. **SQL Injection**: Inject malicious SQL through state machine parameters
4. **JSON Injection**: Exploit request body deserialization vulnerabilities
5. **Permission Bypass**: Exploit hard-coded permissions or validation gaps

## Impact Assessment

- **Confidentiality**: MEDIUM - Information disclosure through error messages
- **Integrity**: HIGH - SQL injection and state manipulation vulnerabilities
- **Availability**: HIGH - Multiple DoS vectors through panics and resource leaks
- **Authentication**: HIGH - Authentication bypass through type assertion failures
- **Authorization**: MEDIUM - Hard-coded permissions and potential bypass

This file contains critical security vulnerabilities requiring immediate attention, particularly around authentication safety, type assertion validation, and state machine security controls.