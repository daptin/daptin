# Security Analysis: server/graphql.go

**File:** `server/graphql.go`  
**Lines of Code:** 1071  
**Primary Function:** GraphQL schema generation and resolver implementation for database entities and actions

## Summary

This file implements comprehensive GraphQL schema generation with automatic query, mutation, and aggregation resolvers for database tables. It includes authentication, authorization, CRUD operations, and action execution through GraphQL API with extensive metadata handling.

## Security Issues Found

### 🔴 CRITICAL Issues

#### 1. **Multiple Unsafe Type Assertions - DoS Vulnerability** (Lines 56, 328, 330-332, 360, 364, 471, 491, 494, 499, 502, 507, 510, 515, 518, 522, 525, 529, 532, 658, 667, 766)
```go
return responder.Result().(api2go.Api2GoModel).GetAttributes(), err
q := qu.(map[string]interface{})
query := resource.Query{
    ColumnName: q["column"].(string),
    Operator:   q["operator"].(string),
    Value:      q["value"].(string),
}
sessionUser = user.(*auth.SessionUser)
referenceId = daptinid.DaptinReferenceId(uuid.MustParse(referenceIdInf.(string)))
```
**Risk:** Extensive application crash points
- Numerous unsafe type assertions throughout GraphQL resolvers
- User-controlled GraphQL input can trigger panics
- No validation before type assertions
**Impact:** High - Denial of service through malformed GraphQL queries
**Remediation:** Implement safe type assertion patterns throughout

#### 2. **UUID Parsing Panic** (Lines 658, 766)
```go
referenceId = daptinid.DaptinReferenceId(uuid.MustParse(referenceIdInf.(string)))
_, err = resources[table.TableName].DeleteWithTransaction(daptinid.DaptinReferenceId(uuid.MustParse(params.Args["reference_id"].(string))), req, transaction)
```
**Risk:** DoS through malformed UUID input
- `uuid.MustParse` panics on invalid UUID format
- User-controlled GraphQL parameters can trigger crashes
- No input validation before UUID parsing
**Impact:** High - Denial of service
**Remediation:** Use `uuid.Parse` with proper error handling

#### 3. **SQL Injection Through Aggregation Parameters** (Lines 491-533)
```go
aggReq.GroupBy = append(aggReq.GroupBy, grp.(string))
aggReq.Filter = append(aggReq.GroupBy, grp.(string))  // Bug: should be Filter
aggReq.Having = append(aggReq.GroupBy, grp.(string))  // Bug: should be Having
```
**Risk:** SQL injection through malformed aggregation queries
- GraphQL parameters passed directly to SQL aggregation
- Copy-paste bugs cause incorrect field assignments
- No input validation or sanitization
**Impact:** High - SQL injection, data breach
**Remediation:** Validate and sanitize all aggregation parameters

### 🟡 HIGH Issues

#### 4. **Authorization Bypass in Mutations** (Lines 604-637, 649-735)
```go
mutationFields["add"+strcase.ToCamel(table.TableName)] = &graphql.Field{
    // ... no authorization check
    Resolve: func(params graphql.ResolveParams) (interface{}, error) {
        // Direct creation without permission validation
        created, err := resources[table.TableName].CreateWithTransaction(obj, req, transaction)
```
**Risk:** Unauthorized data creation and modification
- Create mutations lack authorization checks
- Delete mutations have minimal validation
- Update mutations check permissions but inconsistently
**Impact:** Medium - Unauthorized data manipulation
**Remediation:** Implement consistent authorization for all mutations

#### 5. **Information Disclosure Through Error Messages** (Lines 386, 466, 476, 631, 685, 728, 769)
```go
return nil, fmt.Errorf("no such entity - [%v]", table.TableName)
log.Printf("GraphQL Aggregate Query Arguments: %v", params.Args)
resource.CheckErr(err, "Failed to begin transaction [548]")
```
**Risk:** Internal system information leakage
- Database error details exposed in GraphQL responses
- Query parameters logged with sensitive data
- Internal table names and structure revealed
**Impact:** Medium - Information disclosure
**Remediation:** Sanitize error messages and implement structured logging

#### 6. **Transaction Resource Management Issues** (Lines 474-484, 623-627, 670-680, 760-764, 839-848)
```go
transaction, err := resources[table.TableName].Connection().Beginx()
defer transaction.Commit()
```
**Risk:** Database connection exhaustion
- Inconsistent transaction cleanup patterns
- defer Commit without error checking
- Potential connection leaks on error conditions
**Impact:** Medium - Resource exhaustion
**Remediation:** Standardize transaction management with proper cleanup

### 🟠 MEDIUM Issues

#### 7. **Aggregation Logic Bugs** (Lines 502, 510)
```go
aggReq.Filter = append(aggReq.GroupBy, grp.(string))  // Should append to Filter
aggReq.Having = append(aggReq.GroupBy, grp.(string))  // Should append to Having
```
**Risk:** Incorrect query execution and data exposure
- Copy-paste errors in aggregation parameter handling
- Filter and Having clauses assigned to GroupBy
- May expose unintended data through incorrect queries
**Impact:** Medium - Data integrity and security issues
**Remediation:** Fix aggregation parameter assignments

#### 8. **Missing Input Validation** (Lines 323-337, 354-367)
```go
query, isQueried := params.Args["query"]
queryMap, ok := query.([]interface{})
pageParams, ok := params.Args["page"]
pageParamsMap, ok := pageParams.(map[string]interface{})
```
**Risk:** GraphQL parameter manipulation
- No validation of query structure or content
- Arbitrary page sizes and numbers allowed
- Query operators not validated
**Impact:** Medium - Resource exhaustion, unexpected behavior
**Remediation:** Validate all GraphQL input parameters

#### 9. **Hard-Coded Default Values** (Lines 352-353, 125-132)
```go
pageNumber := 1
pageSize := 10
DefaultValue: 10,
```
**Risk:** Potential resource exhaustion
- Fixed pagination defaults may not suit all scenarios
- No configurable limits for large data sets
- Could enable resource exhaustion attacks
**Impact:** Low - Resource management issues
**Remediation:** Make defaults configurable and add limits

### 🔵 LOW Issues

#### 10. **Debug Information Exposure** (Lines 61, 466, 688, 728)
```go
log.Printf("Type resolve query: %v", p)
log.Printf("GraphQL Aggregate Query Arguments: %v", params.Args)
log.Printf("Get row permission before update: %v", existingObj)
```
**Risk:** Information leakage through debug logs
- Sensitive data logged in debug messages
- GraphQL query structure exposed
- Database row content logged
**Impact:** Low - Information disclosure
**Remediation:** Remove or sanitize debug logging

## Code Quality Issues

1. **Type Safety**: Extensive unsafe type assertions without validation
2. **Error Handling**: Inconsistent error handling and information disclosure
3. **Resource Management**: Improper transaction lifecycle management
4. **Input Validation**: Missing validation for GraphQL parameters
5. **Code Duplication**: Repeated patterns without proper abstraction

## Recommendations

### Immediate Actions Required

1. **Fix Type Assertions**: Implement safe type assertion patterns throughout
2. **UUID Validation**: Replace MustParse with proper error handling
3. **SQL Injection**: Validate and sanitize aggregation parameters
4. **Authorization**: Implement consistent permission checks for all mutations

### Security Improvements

1. **Input Validation**: Validate all GraphQL parameters and query structure
2. **Error Handling**: Sanitize error messages and implement proper logging
3. **Resource Limits**: Implement pagination limits and query complexity controls
4. **Authentication**: Ensure consistent authentication across all resolvers

### Code Quality Enhancements

1. **Transaction Management**: Standardize transaction patterns and cleanup
2. **Logging**: Use structured logging without sensitive information
3. **Configuration**: Make limits and defaults configurable
4. **Testing**: Add unit tests for security-critical resolvers

## Attack Vectors

1. **DoS via Malformed Input**: Crash server through invalid GraphQL parameters
2. **SQL Injection**: Exploit aggregation parameters to inject malicious SQL
3. **Authorization Bypass**: Create/modify data without proper permissions
4. **Resource Exhaustion**: Abuse pagination and complex queries
5. **Information Disclosure**: Extract system details through error messages

## Impact Assessment

- **Confidentiality**: HIGH - Information disclosure and potential data exposure
- **Integrity**: HIGH - Unauthorized data modification through authorization bypass
- **Availability**: HIGH - Multiple DoS vectors through panics and resource exhaustion
- **Authentication**: MEDIUM - Inconsistent authentication checks
- **Authorization**: HIGH - Authorization bypass in critical mutations

This file contains critical security vulnerabilities requiring immediate attention, particularly around type safety, SQL injection prevention, and consistent authorization across all GraphQL operations.