# Security Analysis: server/fsm/fsm_manager.go

**File:** `server/fsm/fsm_manager.go`  
**Lines of Code:** 249  
**Primary Function:** Finite State Machine (FSM) manager providing state transition functionality for objects with state machine definitions, including event application, state tracking, and transition validation

## Summary

This file implements a finite state machine manager that handles state transitions for objects in the system. It manages state machine instances, applies events to trigger transitions, validates transition capabilities, and tracks current states in the database. The implementation integrates with the database layer to persist state information and uses the looplab/fsm library for state machine logic.

## Security Issues Found

### 🔴 CRITICAL Issues

#### 1. **Unsafe Type Assertions Without Error Handling** (Lines 62-69, 151)
```go
res.StateMachineId = responseMap[objType+"_smd"].(int64)
res.ObjectId = responseMap["is_state_of_"+objType].(int64)
objType := subject["__type"].(string)
```
**Risk:** Type assertions can panic if types don't match
- Runtime panics if database values are not expected types
- No validation of type assertions before use
- Could crash application with malformed data
- Attackers could trigger panics with unexpected data types
**Impact:** Critical - Application crashes through type assertion panics
**Remediation:** Use safe type assertions with ok checks

#### 2. **SQL Injection Through String Concatenation** (Lines 31-34, 188-189)
```go
Select("current_state", objType+"_smd", "is_state_of_"+objType, "id", "created_at", "permission").
From(objType + "_state").
Where(goqu.Ex{"is_state_of_" + objType: objId})
```
**Risk:** SQL injection through objType parameter concatenation
- objType used directly in SQL query construction
- No validation or sanitization of objType
- Could execute arbitrary SQL commands
- Bypass access controls through SQL injection
**Impact:** Critical - SQL injection leading to data breach
**Remediation:** Validate and sanitize objType, use parameterized queries

### 🟡 HIGH Issues

#### 3. **JSON Unmarshaling Without Validation** (Line 130)
```go
err = json.Unmarshal([]byte(jsonValue), &events)
```
**Risk:** JSON unmarshaling without input validation
- No validation of JSON content before unmarshaling
- Could trigger memory exhaustion with large JSON
- Potential for denial of service attacks
- No size limits on JSON input
**Impact:** High - Memory exhaustion and denial of service
**Remediation:** Add JSON size limits and content validation

#### 4. **No Authentication Context in State Operations** (Lines 149-185)
```go
func (fsm *fsmManager) ApplyEvent(subject map[string]interface{}, stateMachineEvent StateMachineEvent) (string, error)
```
**Risk:** State transitions without user authentication context
- No verification of user permissions for state changes
- Could allow unauthorized state transitions
- No audit trail for state changes
- Potential for privilege escalation
**Impact:** High - Unauthorized state transitions and privilege escalation
**Remediation:** Add user authentication and authorization checks

#### 5. **Information Disclosure Through Error Messages** (Lines 156, 161, 180-182)
```go
log.Errorf("Failed to get object [%v] by reference id [%v]", objType, objReferenceId)
return stateMachineInstance.CurrestState,
    errors.New(fmt.Sprintf("Cannot apply event %s at this state [%v]",
        stateMachineEvent.GetEventName(), stateMachineInstance.CurrestState),
    )
```
**Risk:** Sensitive information exposed in error messages
- Object IDs and state information in error messages
- Error details could reveal system internals
- Potential for information gathering attacks
- Logs might be accessible to attackers
**Impact:** High - Information disclosure for reconnaissance attacks
**Remediation:** Sanitize error messages, avoid exposing internal details

### 🟠 MEDIUM Issues

#### 6. **Race Conditions in State Machine Operations** (Lines 159-184)
```go
stateMachineInstance, err := fsm.getStateMachineInstance(objType, objectIntegerId, stateMachineEvent.GetStateMachineInstanceId())
// ... time gap ...
if stateMachineRunner.Can(stateMachineEvent.GetEventName()) {
    err := stateMachineRunner.Event(ctx, stateMachineEvent.GetEventName())
```
**Risk:** Race conditions between state check and transition
- State could change between check and execution
- Multiple concurrent transitions could conflict
- No atomic operations for state transitions
- Could lead to inconsistent state
**Impact:** Medium - State inconsistency and race conditions
**Remediation:** Use database transactions and locking for atomic state transitions

#### 7. **No Resource Limits for State Machine Operations** (Lines 96-147)
```go
listOfEvents := make([]loopfsm.EventDesc, 0)
for _, e := range events {
    // No limit on number of events
}
```
**Risk:** No limits on state machine complexity
- Unlimited number of events could cause memory exhaustion
- No validation of state machine definition size
- Could create resource exhaustion attacks
- No timeout protection for state operations
**Impact:** Medium - Resource exhaustion and denial of service
**Remediation:** Add limits on state machine complexity and operation timeouts

#### 8. **Context.TODO() Used for State Operations** (Line 171)
```go
ctx := context.TODO()
err := stateMachineRunner.Event(ctx, stateMachineEvent.GetEventName())
```
**Risk:** No proper context for cancellation or timeouts
- Operations could run indefinitely
- No way to cancel long-running state transitions
- No timeout protection
- Could lead to resource exhaustion
**Impact:** Medium - Operations without timeout protection
**Remediation:** Use proper context with timeouts and cancellation

### 🔵 LOW Issues

#### 9. **Typo in Struct Field Name** (Line 23)
```go
CurrestState   string  // Should be "CurrentState"
```
**Risk:** Typo could cause confusion and maintenance issues
- Inconsistent naming conventions
- Could lead to programming errors
- Affects code readability
- Potential for misuse of field
**Impact:** Low - Code quality and maintainability issues
**Remediation:** Fix typo to "CurrentState"

#### 10. **No Validation of State Machine Events** (Lines 135-143)
```go
for _, e := range events {
    e1 := loopfsm.EventDesc{
        Name: e.Name,
        Src:  e.Src,
        Dst:  e.Dst,
    }
    listOfEvents = append(listOfEvents, e1)
}
```
**Risk:** No validation of event definitions
- Event names not validated for format or safety
- Source and destination states not validated
- Could create invalid state machines
- No checks for circular transitions
**Impact:** Low - Invalid state machine configurations
**Remediation:** Add validation for event definitions and state machine logic

## Code Quality Issues

1. **Error Handling**: Inconsistent error handling patterns throughout the code
2. **Type Safety**: Heavy use of unsafe type assertions without validation
3. **Security Context**: No user authentication or authorization checks
4. **Resource Management**: No limits on resource usage or operation timeouts
5. **Concurrency**: No protection against race conditions in state operations

## Recommendations

### Immediate Actions Required

1. **Type Safety**: Replace unsafe type assertions with safe alternatives
2. **SQL Security**: Validate and sanitize objType parameter before SQL operations
3. **Authentication**: Add user context and permission checks for state operations
4. **Error Security**: Sanitize error messages to prevent information disclosure

### Security Improvements

1. **Input Validation**: Validate all inputs including JSON content and parameters
2. **Access Control**: Implement proper authorization for state transitions
3. **Audit Trail**: Add logging for all state transitions with user context
4. **Race Protection**: Use database transactions for atomic state operations

### Code Quality Enhancements

1. **Error Management**: Implement consistent error handling patterns
2. **Resource Limits**: Add limits on state machine complexity and operation timeouts
3. **Context Management**: Use proper context with timeouts and cancellation
4. **Documentation**: Add comprehensive security and usage documentation

## Attack Vectors

1. **Type Confusion**: Trigger panics through malformed database values
2. **SQL Injection**: Exploit objType parameter for SQL injection attacks
3. **Resource Exhaustion**: Create complex state machines to exhaust memory
4. **Information Disclosure**: Extract sensitive information through error messages
5. **State Manipulation**: Perform unauthorized state transitions
6. **Race Conditions**: Exploit timing windows for inconsistent state changes
7. **JSON Attacks**: Use malformed JSON to cause denial of service
8. **Audit Bypass**: Perform state changes without proper audit trail

## Impact Assessment

- **Confidentiality**: HIGH - Information disclosure through error messages and logs
- **Integrity**: HIGH - Unauthorized state transitions could corrupt business logic
- **Availability**: MEDIUM - Resource exhaustion and panic attacks could cause downtime
- **Authentication**: HIGH - No authentication context for state operations
- **Authorization**: HIGH - No authorization checks for state transitions

This FSM manager has significant security vulnerabilities that could allow unauthorized state manipulation and system compromise.

## Technical Notes

The FSM manager system:
1. Manages finite state machine instances for objects
2. Applies events to trigger state transitions
3. Validates transition capabilities using looplab/fsm library
4. Persists state information in database tables
5. Integrates with database layer for state tracking
6. Provides interface for state machine operations

The main security concerns revolve around input validation, SQL injection, and access control.

## State Machine Security Considerations

For state machine managers:
- **Input Security**: Validate all state machine definitions and events
- **Access Security**: Implement proper authentication and authorization
- **Transition Security**: Ensure atomic and validated state transitions
- **Audit Security**: Log all state changes with user context
- **Resource Security**: Limit state machine complexity and operation timeouts
- **Context Security**: Use proper context for cancellation and timeouts

The current implementation needs comprehensive security enhancements for production use.

## Recommended Security Enhancements

1. **Input Security**: Comprehensive validation for all inputs and parameters
2. **Access Security**: User authentication and authorization for state operations
3. **SQL Security**: Parameterized queries and input sanitization
4. **Error Security**: Sanitized error messages without sensitive information
5. **Concurrency Security**: Atomic operations with proper locking
6. **Resource Security**: Limits on complexity and operation timeouts