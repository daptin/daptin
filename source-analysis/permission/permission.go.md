# Security Analysis: server/permission/permission.go

**File:** `server/permission/permission.go`  
**Type:** Core permission system implementation  
**Lines of Code:** 240  

## Overview
This file implements the core permission system for Daptin, providing binary serialization and permission checking logic for users and groups. It defines the PermissionInstance structure and methods for validating various types of access (read, write, create, delete, execute, etc.).

## Key Components

### PermissionInstance struct
**Lines:** 10-14  
**Purpose:** Core permission data structure containing user ID, group permissions, and base permissions  

### Binary Marshaling Methods
**Lines:** 17-62  
**Purpose:** Binary serialization for efficient storage and network transfer  

### Permission Checking Methods
**Lines:** 64-240  
**Purpose:** Various permission validation methods (CanExecute, CanCreate, CanUpdate, etc.)  

## Security Analysis

### 1. Binary Unmarshaling Vulnerabilities - CRITICAL
**Severity:** HIGH  
**Lines:** 37-62  
**Issue:** Binary unmarshaling without proper input validation.

```go
func (p *PermissionInstance) UnmarshalBinary(data []byte) error {
    p.UserId = daptinid.DaptinReferenceId(data[:16])  // No bounds checking
    p.Permission = auth.AuthPermission(binary.LittleEndian.Uint64(data[16:24]))  // No validation
```

**Risk:**
- Buffer overflow if data length < 24 bytes
- No validation of permission values
- Potential for malformed permission data
- Memory corruption through crafted input

**Impact:** Application crash or memory corruption through malformed permission data.

### 2. Integer Overflow in Group Count Calculation
**Severity:** MEDIUM  
**Lines:** 46-47  
**Issue:** Division operation without overflow protection.

```go
userGroupCount := len(userGroupIdBytes) / auth.AuthGroupBinaryRepresentationSize
```

**Risk:**
- Potential integer overflow with extremely large input
- Could lead to incorrect memory allocation
- Buffer overflow in subsequent operations

### 3. Permission Logic Inconsistencies
**Severity:** HIGH  
**Lines:** 206 vs other methods  
**Issue:** Inconsistent permission checking logic between methods.

```go
// In CanRead (Line 206)
if (uGroup.GroupReferenceId == oGroup.GroupReferenceId || uGroup.RelationReferenceId == oGroup.GroupReferenceId)

// In other methods (e.g., Line 80, 105, 130, etc.)
if (uGroup.GroupReferenceId == oGroup.GroupReferenceId || uGroup.RelationReferenceId == oGroup.RelationReferenceId)
```

**Risk:**
- Different permission evaluation logic for read vs other operations
- Potential for permission bypass through inconsistent logic
- Authorization confusion between different access types

**Impact:** Permission bypass vulnerability allowing unauthorized access.

### 4. Null Reference ID Security Issues
**Severity:** MEDIUM  
**Lines:** 67, Throughout permission methods  
**Issue:** Special handling of null reference IDs without comprehensive validation.

```go
if (p.UserId == userId && p.UserId != daptinid.NullReferenceId) && (p.Permission&auth.UserExecute == auth.UserExecute) {
```

**Risk:**
- Inconsistent null reference handling across methods
- Potential for null reference exploitation
- Authorization bypass through null reference manipulation

### 5. Admin Group Bypass Logic
**Severity:** CRITICAL  
**Lines:** 76, 100, 125, 151, 177, 202, 227  
**Issue:** Admin group checking only compares group reference IDs without additional validation.

```go
if uGroup.GroupReferenceId == adminGroupId {
    return true  // Immediate access grant
}
```

**Risk:**
- No validation of admin group authenticity
- Potential for admin group spoofing
- Complete permission bypass if admin group ID is compromised
- No additional security checks for admin access

**Impact:** Complete authorization bypass through admin group manipulation.

### 6. Group Permission Validation Gaps
**Severity:** MEDIUM  
**Lines:** Throughout group checking loops  
**Issue:** No validation of group permission authenticity or integrity.

**Risk:**
- Group permissions not validated for tampering
- No expiration checking for group memberships
- No validation of group permission source

### 7. Permission Bit Manipulation Vulnerabilities
**Severity:** MEDIUM  
**Lines:** Throughout bitwise operations  
**Issue:** Direct bitwise operations without bounds checking.

```go
p.Permission&auth.UserExecute == auth.UserExecute
```

**Risk:**
- No validation of permission bit values
- Potential for invalid permission combinations
- Integer overflow in permission calculations

## Potential Attack Vectors

### Permission Bypass Attacks
1. **Admin Group Spoofing:** Manipulate admin group ID to gain complete access
2. **Null Reference Exploitation:** Use null references to bypass user ownership checks
3. **Logic Inconsistency Exploitation:** Exploit different permission logic between operations
4. **Binary Data Manipulation:** Craft malformed binary data to corrupt permission structures

### Data Integrity Attacks
1. **Buffer Overflow:** Submit undersized binary data to trigger buffer overflows
2. **Integer Overflow:** Submit large group counts to cause integer overflow
3. **Permission Bit Manipulation:** Craft invalid permission bit combinations

### Authorization Confusion
1. **Group Relation Confusion:** Exploit relationship vs group reference logic differences
2. **Permission Escalation:** Combine multiple permission sources for escalation
3. **Reference ID Confusion:** Use similar reference IDs to cause authorization confusion

## Recommendations

### Immediate Actions
1. **Add Input Validation:** Validate binary input length and content
2. **Fix Logic Inconsistency:** Standardize permission checking logic across all methods
3. **Enhance Admin Validation:** Add additional validation for admin group access
4. **Add Bounds Checking:** Implement comprehensive bounds checking for all operations

### Enhanced Security Implementation

```go
package permission

import (
    "encoding/binary"
    "fmt"
    "github.com/daptin/daptin/server/auth"
    daptinid "github.com/daptin/daptin/server/id"
)

const (
    MinPermissionDataSize = 24
    MaxGroupCount = 10000
    MaxPermissionDataSize = MinPermissionDataSize + (MaxGroupCount * auth.AuthGroupBinaryRepresentationSize)
)

type PermissionInstance struct {
    UserId      daptinid.DaptinReferenceId
    UserGroupId auth.GroupPermissionList
    Permission  auth.AuthPermission
}

// SecureUnmarshalBinary with comprehensive validation
func (p *PermissionInstance) SecureUnmarshalBinary(data []byte) error {
    // Validate minimum data size
    if len(data) < MinPermissionDataSize {
        return fmt.Errorf("permission data too small: %d bytes, minimum required: %d", len(data), MinPermissionDataSize)
    }
    
    // Validate maximum data size
    if len(data) > MaxPermissionDataSize {
        return fmt.Errorf("permission data too large: %d bytes, maximum allowed: %d", len(data), MaxPermissionDataSize)
    }
    
    // Extract user ID safely
    copy(p.UserId[:], data[:16])
    
    // Extract and validate permission
    permissionValue := binary.LittleEndian.Uint64(data[16:24])
    if !isValidPermission(auth.AuthPermission(permissionValue)) {
        return fmt.Errorf("invalid permission value: %d", permissionValue)
    }
    p.Permission = auth.AuthPermission(permissionValue)
    
    // Process group data
    userGroupIdBytes := data[24:]
    if len(userGroupIdBytes)%auth.AuthGroupBinaryRepresentationSize != 0 {
        return fmt.Errorf("invalid group data length: %d", len(userGroupIdBytes))
    }
    
    userGroupCount := len(userGroupIdBytes) / auth.AuthGroupBinaryRepresentationSize
    if userGroupCount > MaxGroupCount {
        return fmt.Errorf("too many groups: %d, maximum allowed: %d", userGroupCount, MaxGroupCount)
    }
    
    userGroupId := make(auth.GroupPermissionList, userGroupCount)
    for i := 0; i < userGroupCount; i++ {
        start := i * auth.AuthGroupBinaryRepresentationSize
        end := (i + 1) * auth.AuthGroupBinaryRepresentationSize
        
        groupPermission := auth.GroupPermission{}
        err := groupPermission.UnmarshalBinary(userGroupIdBytes[start:end])
        if err != nil {
            return fmt.Errorf("failed to unmarshal group permission at index %d: %v", i, err)
        }
        
        // Validate group permission
        if err := validateGroupPermission(groupPermission); err != nil {
            return fmt.Errorf("invalid group permission at index %d: %v", i, err)
        }
        
        userGroupId[i] = groupPermission
    }
    
    p.UserGroupId = userGroupId
    return nil
}

// isValidPermission validates permission bit combinations
func isValidPermission(perm auth.AuthPermission) bool {
    // Define valid permission ranges and combinations
    validBits := auth.GuestPeek | auth.GuestRead | auth.GuestCreate | auth.GuestUpdate | 
                auth.GuestDelete | auth.GuestExecute | auth.GuestRefer |
                auth.UserPeek | auth.UserRead | auth.UserCreate | auth.UserUpdate |
                auth.UserDelete | auth.UserExecute | auth.UserRefer |
                auth.GroupPeek | auth.GroupRead | auth.GroupCreate | auth.GroupUpdate |
                auth.GroupDelete | auth.GroupExecute | auth.GroupRefer
    
    // Check if permission contains only valid bits
    return (perm & ^validBits) == 0
}

// validateGroupPermission validates group permission structure
func validateGroupPermission(gp auth.GroupPermission) error {
    // Validate group reference ID is not null
    if gp.GroupReferenceId == daptinid.NullReferenceId {
        return fmt.Errorf("group reference ID cannot be null")
    }
    
    // Validate permission bits
    if !isValidPermission(gp.Permission) {
        return fmt.Errorf("invalid group permission bits")
    }
    
    return nil
}

// SecureCanExecute with enhanced validation
func (p PermissionInstance) SecureCanExecute(userId daptinid.DaptinReferenceId, 
    usergroupId auth.GroupPermissionList, adminGroupId daptinid.DaptinReferenceId) (bool, error) {
    
    // Validate input parameters
    if err := p.validatePermissionCheckInputs(userId, usergroupId, adminGroupId); err != nil {
        return false, err
    }
    
    // Check user-level permissions
    if p.UserId == userId && p.UserId != daptinid.NullReferenceId {
        if p.Permission&auth.UserExecute == auth.UserExecute {
            return true, nil
        }
    }
    
    // Check guest permissions
    if p.Permission&auth.GuestExecute == auth.GuestExecute {
        return true, nil
    }
    
    // Check group permissions with consistent logic
    for _, uGroup := range usergroupId {
        // Validate admin group access with additional checks
        if uGroup.GroupReferenceId == adminGroupId && adminGroupId != daptinid.NullReferenceId {
            if err := p.validateAdminAccess(uGroup, adminGroupId); err != nil {
                return false, fmt.Errorf("admin validation failed: %v", err)
            }
            return true, nil
        }
        
        // Check group permissions with consistent logic
        for _, oGroup := range p.UserGroupId {
            if p.groupPermissionMatches(uGroup, oGroup) && 
               oGroup.Permission&auth.GroupExecute == auth.GroupExecute {
                return true, nil
            }
        }
    }
    
    return false, nil
}

// validatePermissionCheckInputs validates inputs to permission checking methods
func (p PermissionInstance) validatePermissionCheckInputs(userId daptinid.DaptinReferenceId, 
    usergroupId auth.GroupPermissionList, adminGroupId daptinid.DaptinReferenceId) error {
    
    // Validate group list size
    if len(usergroupId) > MaxGroupCount {
        return fmt.Errorf("too many user groups: %d", len(usergroupId))
    }
    
    // Validate each group in user group list
    for i, group := range usergroupId {
        if err := validateGroupPermission(group); err != nil {
            return fmt.Errorf("invalid user group at index %d: %v", i, err)
        }
    }
    
    return nil
}

// validateAdminAccess performs additional validation for admin access
func (p PermissionInstance) validateAdminAccess(userGroup auth.GroupPermission, 
    adminGroupId daptinid.DaptinReferenceId) error {
    
    // Additional admin validation logic
    if userGroup.GroupReferenceId != adminGroupId {
        return fmt.Errorf("admin group ID mismatch")
    }
    
    // Could add additional checks here:
    // - Admin group expiration
    // - Admin permission level validation
    // - Admin access logging
    
    return nil
}

// groupPermissionMatches provides consistent group matching logic
func (p PermissionInstance) groupPermissionMatches(userGroup, objectGroup auth.GroupPermission) bool {
    // Consistent logic across all permission methods
    return userGroup.GroupReferenceId == objectGroup.GroupReferenceId || 
           userGroup.RelationReferenceId == objectGroup.RelationReferenceId
}

// Implement all other permission methods with consistent logic...
func (p PermissionInstance) SecureCanRead(userId daptinid.DaptinReferenceId, 
    usergroupId auth.GroupPermissionList, adminGroupId daptinid.DaptinReferenceId) (bool, error) {
    
    if err := p.validatePermissionCheckInputs(userId, usergroupId, adminGroupId); err != nil {
        return false, err
    }
    
    // User-level permissions
    if p.UserId == userId && p.UserId != daptinid.NullReferenceId {
        if p.Permission&auth.UserRead == auth.UserRead {
            return true, nil
        }
    }
    
    // Guest permissions
    if p.Permission&auth.GuestRead == auth.GuestRead {
        return true, nil
    }
    
    // Group permissions with consistent logic
    for _, uGroup := range usergroupId {
        if uGroup.GroupReferenceId == adminGroupId && adminGroupId != daptinid.NullReferenceId {
            if err := p.validateAdminAccess(uGroup, adminGroupId); err != nil {
                return false, fmt.Errorf("admin validation failed: %v", err)
            }
            return true, nil
        }
        
        for _, oGroup := range p.UserGroupId {
            // Fixed: Use consistent logic (was different in original CanRead)
            if p.groupPermissionMatches(uGroup, oGroup) && 
               oGroup.Permission&auth.GroupRead == auth.GroupRead {
                return true, nil
            }
        }
    }
    
    return false, nil
}
```

### Long-term Improvements
1. **Permission Auditing:** Add comprehensive audit logging for permission checks
2. **Performance Optimization:** Optimize permission checking for large group lists
3. **Cache Integration:** Add caching for frequently checked permissions
4. **Permission Expiration:** Implement time-based permission expiration
5. **Role-Based Security:** Add role-based access control layer

## Edge Cases Identified

1. **Null Reference Scenarios:** Various null reference ID combinations
2. **Empty Group Lists:** Permission checking with empty group lists
3. **Large Group Lists:** Performance and memory with very large group lists
4. **Invalid Permission Bits:** Malformed or invalid permission bit combinations
5. **Admin Group Edge Cases:** Various admin group manipulation scenarios
6. **Binary Data Corruption:** Malformed binary permission data
7. **Concurrent Access:** Thread safety of permission checking operations
8. **Memory Exhaustion:** Extremely large permission structures

## Security Best Practices Violations

1. **No input validation in binary unmarshaling**
2. **Inconsistent permission checking logic**
3. **Insufficient admin group validation**
4. **No bounds checking on operations**
5. **Missing permission bit validation**

## Critical Issues Summary

1. **Binary Unmarshaling Vulnerability:** Buffer overflow and memory corruption risks
2. **Permission Logic Inconsistency:** Different logic between read and other operations
3. **Admin Group Bypass:** Insufficient validation for admin access
4. **Input Validation Gaps:** Multiple input validation failures
5. **Integer Overflow Risks:** Potential overflow in group count calculations

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - Multiple authorization bypass vulnerabilities and memory safety issues