# auth.go

**File:** server/auth/auth.go

## Code Summary

This file implements the core authentication and authorization middleware for Daptin, handling JWT tokens, basic auth, user sessions, and permission management. It's a critical security component with complex authentication flows.

### Authentication Constants and Types (lines 30-70)
**AuthPermission:** Bitmask-based permission system with Guest/User/Group levels
**Permission Constants:**
- Individual permissions: Peek, Read, Create, Update, Delete, Execute, Refer
- Combined permissions: CRUD (multiple operations), DEFAULT_PERMISSION, ALLOW_ALL_PERMISSIONS
- **Line 68:** `DEFAULT_PERMISSION_WHEN_NO_ADMIN` grants excessive permissions when no admin exists

### Type: AuthMiddleware (lines 81-88)
**Purpose:** Core authentication middleware managing JWT and user sessions
**Fields:**
- `db database.DatabaseConnection` - Database connection
- `userCrud ResourceAdapter` - User CRUD operations
- `userGroupCrud ResourceAdapter` - User group management
- `userUserGroupCrud ResourceAdapter` - User-group relationship management
- `issuer string` - JWT token issuer
- `olricDb *olric.EmbeddedClient` - Distributed cache

### Function: InitJwtMiddleware() (lines 112-142)
**Purpose:** Initializes global JWT middleware
**Inputs:**
- `secret []byte` - JWT signing secret
- `issuer string` - Token issuer
- `db *olric.EmbeddedClient` - Cache database

**Security Issues:**
- **Global state:** Uses global `jwtMiddleware` variable
- **Token extraction:** Supports multiple token sources (header, parameter, cookie)
- **Algorithm specification:** Correctly specifies HS256 to prevent algorithm substitution attacks

### Function: BasicAuthCheckMiddlewareWithHttp() (lines 157-199)
**Purpose:** Validates HTTP Basic Authentication
**Inputs:**
- `req *http.Request` - HTTP request
- `writer http.ResponseWriter` - HTTP response writer

**Process:**
1. **Header Parsing (lines 159-165):**
   - Extracts Authorization header
   - Splits Bearer/Basic token

2. **Base64 Decoding (lines 166-175):**
   - Decodes base64 credentials
   - Parses username:password format

3. **Database Verification (lines 176-186):**
   - Gets user password hash from database
   - Validates against provided password

**Security Issues:**
- **Line 171:** No bounds checking on `tokenValueParts[0]` - can panic
- **Line 182:** Database query for password hash on every request
- **Line 188:** Password comparison using bcrypt (secure)
- **Line 191:** **INFORMATION LEAKAGE:** Uses email address for username in JWT claims
- **No rate limiting:** Vulnerable to brute force attacks
- **Transaction handling:** Creates and rolls back transaction for read-only operation

### Function: CheckErr() (lines 206-216)
**Purpose:** Error logging utility
**Security Issues:**
- **Line 208:** Type assertion `message[0].(string)` can panic
- **Information disclosure:** May log sensitive error information

### Function: AuthCheckMiddlewareWithHttp() (lines 251-498)
**Purpose:** Main authentication middleware processing HTTP requests
**This is the core authentication function with complex logic:**

**1. Static Resource Bypass (lines 257-260):**
- Allows unauthenticated access to `/static` and `/favicon.ico`

**2. JWT Token Validation (lines 269-288):**
- Validates JWT tokens using global middleware
- Falls back to Basic Auth if JWT fails

**3. User Session Management (lines 290-495):**
- **Complex caching logic using both local and distributed cache**
- **User creation for OAuth users (lines 353-401)**
- **User group permission loading (lines 404-442)**

**Critical Security Issues:**

**User Session Caching (lines 319-482):**
- **Line 301:** Type assertion `userToken.Claims.(jwt.MapClaims)["email"].(string)` can panic
- **Line 302:** Type assertion `userToken.Claims.(jwt.MapClaims)["name"].(string)` can panic
- **Lines 353-401:** **AUTOMATIC USER CREATION** for OAuth users without validation
- **Line 360:** Uses `DEFAULT_PERMISSION` which may be overly permissive
- **Line 376:** **CRITICAL:** Type assertion on user ID can fail

**Database Operations:**
- **Lines 326-333:** Raw SQL construction for user lookup
- **Lines 404-441:** Complex user group permission queries
- **No prepared statement reuse:** Creates new prepared statements on each request

**Cache Operations:**
- **Lines 462-466:** User session stored in distributed cache for 10 minutes
- **No cache invalidation:** User permissions cached without invalidation mechanism

### Function: AuthCheckMiddleware() (lines 500-512)
**Purpose:** Gin middleware wrapper for HTTP auth middleware
**Simple wrapper that converts HTTP auth to Gin context**

### Type: SessionUser (lines 514-518)
**Purpose:** Represents authenticated user session with permissions
**Binary Serialization Methods (lines 520-559):**
- Custom binary marshaling for cache storage
- **Line 539:** Buffer overflow protection with minimum size check

### Type: GroupPermission (lines 561-566)
**Purpose:** Represents user's permission within a group
**Binary Serialization Methods (lines 606-636):**
- Fixed-size binary representation (56 bytes)
- UUID-based reference IDs

## Critical Issues Found

### 🚨 Critical Security Vulnerabilities
1. **Lines 301-302:** Type assertions on JWT claims can panic if malformed token
2. **Line 171:** Array bounds access without validation can cause panic  
3. **Line 376:** Type assertion on user ID creation can fail silently
4. **Line 360:** Automatic user creation uses potentially excessive DEFAULT_PERMISSION
5. **Lines 353-401:** OAuth users automatically created without proper validation

### ⚠️ Authentication Security Issues
6. **No rate limiting:** Basic auth vulnerable to brute force attacks
7. **Information leakage:** Email addresses used in JWT claims and error messages
8. **Weak permission model:** DEFAULT_PERMISSION_WHEN_NO_ADMIN grants excessive access
9. **Global state:** JWT middleware stored in global variable creating race conditions
10. **Token extraction:** Multiple token sources increase attack surface

### 🔐 Session Management Issues
11. **Cache poisoning:** User sessions cached without proper validation
12. **No session invalidation:** Cached sessions persist even after password changes
13. **Permission persistence:** User permissions cached for 10 minutes without updates
14. **Distributed cache:** No encryption or integrity protection for cached user data
15. **Memory leaks:** Local cache maps grow without cleanup

### 📂 Database Security Issues
16. **SQL injection potential:** Dynamic query construction in user lookup
17. **Transaction abuse:** Read-only operations use unnecessary transactions
18. **No prepared statement caching:** Performance and resource issues
19. **Password hash exposure:** Password hashes retrieved on every authentication
20. **Connection pool exhaustion:** No connection pool management

### 🏗️ Design Issues
21. **Complex authentication flow:** Multiple authentication paths increase complexity
22. **Mixed responsibilities:** Single middleware handles JWT, Basic auth, and session management
23. **Global variables:** Multiple global variables create testing and concurrency issues
24. **No abstraction:** Direct database access without abstraction layer
25. **Hard-coded timeouts:** Cache timeouts and permissions hard-coded

### ⚙️ Runtime Safety Issues
26. **Multiple type assertions:** Numerous unsafe type assertions throughout codebase
27. **Array bounds access:** No validation of array/slice access
28. **Nil pointer dereferences:** Insufficient nil checking
29. **Resource leaks:** Database connections and prepared statements may leak
30. **Concurrent access:** Shared cache access without proper synchronization

### 🌐 Authorization Issues
31. **Permission escalation:** Users can potentially gain unauthorized permissions through caching
32. **No audit logging:** Authentication and authorization events not logged
33. **Weak default permissions:** System defaults may grant excessive access
34. **Group permission complexity:** Complex group permission model hard to audit
35. **No permission validation:** Permissions not validated when applied

### 💾 Cache Security Issues
36. **Cache corruption:** No validation of cached user data integrity
37. **Distributed cache:** Sensitive user data stored in distributed cache without encryption
38. **Cache timing:** Cache expiration not synchronized with security events
39. **No cache eviction:** Manual cache clearing not implemented
40. **Memory disclosure:** User data may persist in memory beyond intended lifetime

## Security Architecture Issues

This authentication system has several architectural security concerns:

1. **Excessive Complexity:** The authentication flow is overly complex with multiple fallback mechanisms
2. **Default Permissions:** The system grants broad permissions by default, especially when no admin exists
3. **Automatic User Creation:** OAuth users are automatically created with significant permissions
4. **Cache Dependencies:** Heavy reliance on caching creates security consistency issues
5. **Global State:** Use of global variables creates race conditions and testing difficulties

The authentication system would benefit from simplification, stronger default permissions, and better separation of concerns.