I'll create a comprehensive technical documentation of the authentication layer implementation that uses Linux-like permission bits for database row/table access control.

# Daptin Authorization System: Technical Documentation

## Overview

Daptin implements a sophisticated authorization system inspired by Linux-style permission bits but extended for database operations. This system enables fine-grained access control at both table and row levels across different user contexts (guest, authenticated user, and group).

The permission system controls six core operations: Peek, Read, Create, Update, Delete, and Execute, plus an additional Reference permission for relationship management.

## Permission Structure

### Permission Bits

The authorization system uses bitmasks to represent permissions efficiently:

```go
type AuthPermission int64

const None AuthPermission = iota

const (
    GuestPeek AuthPermission = 1 << iota
    GuestRead
    GuestCreate
    GuestUpdate
    GuestDelete
    GuestExecute
    GuestRefer
    UserPeek
    UserRead
    UserCreate
    UserUpdate
    UserDelete
    UserExecute
    UserRefer
    GroupPeek
    GroupRead
    GroupCreate
    GroupUpdate
    GroupDelete
    GroupExecute
    GroupRefer
)
```

Permission bits are organized into three categories:
- **Guest permissions**: Apply to unauthenticated users
- **User permissions**: Apply to object owners
- **Group permissions**: Apply to users who belong to groups with access

### Convenience Permission Sets

The system provides predefined permission sets for common use cases:

```go
const (
    GuestCRUD = GuestPeek | GuestRead | GuestCreate | GuestUpdate | GuestDelete | GuestRefer
    UserCRUD  = UserPeek | UserRead | UserCreate | UserUpdate | UserDelete | UserRefer
    GroupCRUD = GroupPeek | GroupRead | GroupCreate | GroupUpdate | GroupDelete | GroupRefer
)

const (
    DEFAULT_PERMISSION               = GuestPeek | GuestExecute | UserRead | UserExecute | GroupRead | GroupExecute
    DEFAULT_PERMISSION_WHEN_NO_ADMIN = GuestCRUD | GuestExecute | UserCRUD | UserExecute | GroupCRUD | GroupExecute
    ALLOW_ALL_PERMISSIONS            = GuestCRUD | GuestExecute | UserCRUD | UserExecute | GroupCRUD | GroupExecute
)
```

## Core Components

### 1. Permission Instance

The `PermissionInstance` struct holds ownership and permission data for a specific resource:

```go
type PermissionInstance struct {
    UserId      daptinid.DaptinReferenceId
    UserGroupId auth.GroupPermissionList
    Permission  auth.AuthPermission
}
```

- `UserId`: Resource owner's reference ID
- `UserGroupId`: List of groups with permissions to this resource
- `Permission`: Bitmask of authorized operations

### 2. Session User

The `SessionUser` struct represents an authenticated user's session:

```go
type SessionUser struct {
    UserId          int64
    UserReferenceId daptinid.DaptinReferenceId
    Groups          GroupPermissionList
}
```

- `UserId`: Database ID
- `UserReferenceId`: Unique reference ID
- `Groups`: List of groups the user belongs to

### 3. Group Permission

The `GroupPermission` struct represents a relationship between groups and resources:

```go
type GroupPermission struct {
    GroupReferenceId    daptinid.DaptinReferenceId
    ObjectReferenceId   daptinid.DaptinReferenceId
    RelationReferenceId daptinid.DaptinReferenceId
    Permission          AuthPermission
}
```

## Permission Check Methods

Each `PermissionInstance` provides methods to check if specific operations are allowed:

### CanPeek

Allows viewing the existence of a resource (similar to Unix `x` bit on directories):

```go
func (p PermissionInstance) CanPeek(userId daptinid.DaptinReferenceId, 
                                   usergroupId auth.GroupPermissionList,
                                   adminGroupId daptinid.DaptinReferenceId) bool
```

### CanRead

Allows reading resource content (similar to Unix `r` bit):

```go
func (p PermissionInstance) CanRead(userId daptinid.DaptinReferenceId, 
                                   usergroupId auth.GroupPermissionList,
                                   adminGroupId daptinid.DaptinReferenceId) bool
```

### CanCreate

Allows creating new resources:

```go
func (p PermissionInstance) CanCreate(userId daptinid.DaptinReferenceId, 
                                     usergroupId auth.GroupPermissionList,
                                     adminGroupId daptinid.DaptinReferenceId) bool
```

### CanUpdate

Allows modifying existing resources (similar to Unix `w` bit):

```go
func (p PermissionInstance) CanUpdate(userId daptinid.DaptinReferenceId, 
                                     usergroupId auth.GroupPermissionList,
                                     adminGroupId daptinid.DaptinReferenceId) bool
```

### CanDelete

Allows removing resources:

```go
func (p PermissionInstance) CanDelete(userId daptinid.DaptinReferenceId, 
                                     usergroupId auth.GroupPermissionList,
                                     adminGroupId daptinid.DaptinReferenceId) bool
```

### CanExecute

Allows performing actions on resources:

```go
func (p PermissionInstance) CanExecute(userId daptinid.DaptinReferenceId, 
                                      usergroupId auth.GroupPermissionList,
                                      adminGroupId daptinid.DaptinReferenceId) bool
```

### CanRefer

Allows establishing relationships between resources:

```go
func (p PermissionInstance) CanRefer(userId daptinid.DaptinReferenceId, 
                                    usergroupId auth.GroupPermissionList,
                                    adminGroupId daptinid.DaptinReferenceId) bool
```

## Authorization Flow

### 1. Authentication Middleware

The `AuthMiddleware` handles user authentication and session creation:

1. Checks for JWT tokens in request headers, cookies, or parameters
2. Validates credentials against stored password hashes (bcrypt)
3. Builds a `SessionUser` object with user data and group memberships
4. Stores the session in request context for downstream handlers

### 2. Table-Level Access Control

The `TableAccessPermissionChecker` enforces permissions at the entity level:

1. Retrieves table ownership and permission records
2. Checks if the requested operation is allowed for the current user
3. Returns 403 Forbidden if access is denied

### 3. Row-Level Access Control

The `ObjectAccessPermissionChecker` enforces permissions at the individual record level:

1. Filters result sets based on permission checks for each row
2. Removes unauthorized records from results

## Permission Resolution Logic

For each permission check:

1. If the user is the resource owner with appropriate permission bit set, access is granted
2. If the operation's guest permission bit is set, access is granted
3. If the user belongs to the administrator group, access is granted
4. If the user belongs to any group with appropriate permission bit set, access is granted
5. Otherwise, access is denied

## Binary Serialization

Permission data is serialized for efficient caching:

```go
// MarshalBinary implements encoding.BinaryMarshaler interface
func (p PermissionInstance) MarshalBinary() (data []byte, err error) { ... }

// UnmarshalBinary implements encoding.BinaryUnmarshaler interface
func (p *PermissionInstance) UnmarshalBinary(data []byte) error { ... }
```

## Caching

To improve performance, authentication data is cached:

1. User session data is cached in an Olric distributed map
2. An in-memory cache with expiry times reduces database lookups

## UI Design Considerations

When designing the permission management UI, consider the following:

### Permission Visualization

1. **Visual Metaphor**: Create a UI representation of permission bits similar to Linux file permissions, but with expanded categories.

2. **Three-Tier Display**: Group permissions into Guest, User, and Group sections.

3. **Permission Matrix**: Create a grid with:
    - Rows: Guest, User, Group
    - Columns: Peek, Read, Create, Update, Delete, Execute, Refer

### Editing Interface

1. **Permission Toggle**: Allow toggling individual permission bits with checkboxes or switches.

2. **Permission Templates**: Provide preset permission combinations:
    - Public (GuestCRUD)
    - Private (UserCRUD)
    - Group-Only (GroupCRUD)
    - Read-Only (GuestPeek | GuestRead | UserPeek | UserRead | GroupPeek | GroupRead)

3. **Numeric View**: Include a numeric representation of the permission value for advanced users.

4. **Permission Calculator**: Allow users to build permissions through checkboxes that update the numeric value.

### Group Management

1. **Group Assignment**: Provide interfaces to assign users to groups.

2. **Group Permission Management**: Create interfaces to set default permissions for groups.

3. **Special Handling for Admin Group**: Clearly indicate when a user belongs to the administrator group.

### Permission Inheritance and Override

1. **Inheritance Display**: Show the effective permissions including those inherited from groups.

2. **Override Controls**: Allow explicit overriding of inherited permissions.

### Conflict Resolution

1. **Permission Conflict Indicator**: Highlight when user and group permissions may create unexpected access patterns.

2. **Permission Analyzer**: Help identify permission gaps or excessive permissions.

## Example Permission Configurations

The following examples demonstrate common permission patterns:

### Public Read-Only Resource
```
GuestPeek | GuestRead | UserCRUD | GroupRead
```

### Owner-Only Resource
```
UserCRUD | UserExecute
```

### Team-Accessible Resource
```
UserCRUD | UserExecute | GroupRead | GroupExecute
```

### Public Contribution Resource
```
GuestPeek | GuestRead | GuestCreate | UserCRUD | UserExecute | GroupCRUD
```

## API Reference

### Permission Checking
```go
// Check if user can view an object (table-level)
tableOwnership.CanPeek(sessionUser.UserReferenceId, sessionUser.Groups, dr.AdministratorGroupId)

// Check if user can modify an object (row-level)
permission.CanUpdate(sessionUser.UserReferenceId, sessionUser.Groups, dr.AdministratorGroupId)

// Check if user can establish a reference to another object
foreignObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId)

// Check if user can perform an action on an object
permission.CanExecute(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId)
```

## Error Messages

When access is denied, a standard error format is used:
```
[%v] [%v] access not allowed for action [%v] to user [%v]
```

For example:
```
[object] [user_profile] access not allowed for action [GET] to user [550e8400-e29b-41d4-a716-446655440000]
```

## Implementation Examples

### Checking Action Permission
```go
if !permission.CanExecute(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId) {
    log.Warnf("user[%v] not allowed action on this object: %v - %v", sessionUser, actionRequest.Action, subjectInstanceReferenceString)
    return nil, api2go.NewHTTPError(errors.New("forbidden"), "forbidden", 403)
}
```

### Checking Reference Permission
```go
foreignObjectPermission := GetObjectPermissionByReferenceIdWithTransaction(col.ForeignKeyData.Namespace, dir, createTransaction)

if isAdmin || foreignObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId) {
    uId = foreignObjectReferenceId
} else {
    log.Printf("[137] User cannot refer this object [%v][%v]", col.ForeignKeyData.Namespace, columnValue)
    return nil, fmt.Errorf("refer object not allowed [%v][%v]", col.ForeignKeyData.Namespace, columnValue)
}
```

## Conclusion

Daptin's permission system provides a flexible, fine-grained access control mechanism that extends Linux-style permissions to database operations. By leveraging bitmap operations for permission checks, the system remains efficient while providing extensive control over access patterns.

The UI team should focus on creating intuitive interfaces that represent these complex permissions in a user-friendly manner, allowing administrators to easily configure and manage access to system resources.