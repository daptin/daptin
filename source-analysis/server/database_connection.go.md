# Security Analysis: server/database_connection.go

**File:** `server/database_connection.go`  
**Type:** Database connection management and configuration  
**Lines of Code:** 113  

## Overview
This file manages database connections for the Daptin server, supporting MySQL, PostgreSQL, and SQLite databases. It handles connection string manipulation, connection pool configuration through environment variables, and database driver initialization. The implementation includes automatic charset and collation configuration for MySQL connections.

## Key Components

### GetDbConnection function
**Lines:** 17-100  
**Purpose:** Creates and configures database connections with pool settings  

### Connection String Manipulation
- **MySQL charset handling:** Lines 19-25
- **MySQL collation handling:** Lines 27-33
- **Database driver selection:** Line 35

### Connection Pool Configuration
- **Environment variable reading:** Lines 41-59
- **Parameter parsing:** Lines 61-79
- **Pool settings application:** Lines 84-88

## Security Analysis

### 1. HIGH: Connection String Injection - HIGH RISK
**Severity:** HIGH  
**Lines:** 19-33, 35  
**Issue:** Connection string manipulation without validation enables SQL injection in connection parameters.

```go
if strings.Index(connectionString, "charset=") == -1 {
    if strings.Index(connectionString, "?") > -1 {
        connectionString = connectionString + "&charset=utf8mb4"  // String injection
    } else {
        connectionString = connectionString + "?charset=utf8mb4"  // String injection
    }
}
```

**Risk:**
- **Connection string injection** through malicious parameters
- **Database credential manipulation** via injected connection strings
- **Connection hijacking** through parameter injection
- **Authentication bypass** via connection string manipulation

### 2. HIGH: Environment Variable Injection - HIGH RISK
**Severity:** HIGH  
**Lines:** 41-59  
**Issue:** Environment variables used directly without validation for critical database settings.

```go
maxIdleConnections := os.Getenv("DAPTIN_MAX_IDLE_CONNECTIONS")      // No validation
maxOpenConnections := os.Getenv("DAPTIN_MAX_OPEN_CONNECTIONS")      // No validation
maxConnectionLifetimeMinString := os.Getenv("DAPTIN_MAX_CONNECTIONS_LIFETIME")  // No validation
```

**Risk:**
- **Resource exhaustion** through malicious environment values
- **Denial of service** via connection pool manipulation
- **Performance degradation** from invalid configuration
- **System instability** through extreme configuration values

### 3. MEDIUM: Missing Input Validation - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 17, 61-79  
**Issue:** Database type and connection parameters not validated for security.

```go
func GetDbConnection(dbType string, connectionString string) (*sqlx.DB, error) {
    // No validation of dbType or connectionString parameters
    db, e := sqlx.Open(dbType, connectionString)  // Direct use without validation
}
```

**Risk:**
- **Invalid database driver exploitation** through malicious dbType
- **Connection string manipulation** via crafted parameters
- **Resource consumption** from invalid connection attempts
- **Error information disclosure** through invalid connections

### 4. MEDIUM: Information Disclosure - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 89-97  
**Issue:** Detailed database configuration logged without sanitization.

```go
log.Infof("Database connection Params: "+
    "Max Idle Connections: [%v], "+
    "Max Open Connections: [%v] , "+
    "Max connection Life time: [%v] , "+
    "Max Idle connection life time: [%v] ",
    maxIdleConnectionsInt, maxOpenConnectionsInt,
    maxConnectionLifetimeMin, maxConnectionIdleTimeMin)
```

**Risk:**
- **Configuration disclosure** revealing system architecture
- **Performance fingerprinting** through configuration exposure
- **Attack surface mapping** via detailed logging
- **Operational security information** leaked through logs

### 5. MEDIUM: Error Handling Issues - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 62-79  
**Issue:** Parse errors handled silently with default values, masking potential attacks.

```go
maxIdleConnectionsInt, err := strconv.ParseInt(maxIdleConnections, 10, 64)
if err != nil {
    maxIdleConnectionsInt = 10  // Silent fallback, no logging
}
```

**Risk:**
- **Silent failure** masking malicious input attempts
- **Configuration drift** from expected vs actual values
- **Attack attempt concealment** through silent error handling
- **Debugging complexity** from undocumented fallbacks

### 6. LOW: Resource Management Configuration - LOW RISK
**Severity:** LOW  
**Lines:** 49-51, 84-88  
**Issue:** Hardcoded SQLite connection limit and potential resource misconfiguration.

```go
if strings.Index(dbType, "sqlite") > -1 {
    maxOpenConnections = "1"  // Hardcoded value
}
```

**Risk:**
- **Performance bottlenecks** from hardcoded SQLite limits
- **Resource contention** in high-concurrency scenarios
- **Database locking** issues with SQLite
- **Scalability limitations** from fixed configurations

### 7. LOW: String Parsing Vulnerabilities - LOW RISK
**Severity:** LOW  
**Lines:** 19-33  
**Issue:** Simple string operations for connection string manipulation without proper parsing.

```go
if strings.Index(connectionString, "charset=") == -1 {
    // Simple string operations without URL parsing
}
```

**Risk:**
- **Malformed connection strings** from improper manipulation
- **Parameter corruption** through string concatenation
- **Connection failures** from invalid string operations
- **Encoding issues** from charset manipulation

## Potential Attack Vectors

### Connection String Injection Attacks
1. **Parameter Injection:** Inject malicious parameters into connection strings
2. **Credential Manipulation:** Modify database credentials through string injection
3. **Connection Hijacking:** Redirect connections to malicious databases
4. **Authentication Bypass:** Bypass authentication through connection string manipulation

### Environment Variable Attacks
1. **Resource Exhaustion:** Set extreme values for connection pool parameters
2. **Denial of Service:** Configure impossible connection settings
3. **Performance Degradation:** Use values that degrade system performance
4. **Configuration Corruption:** Set invalid values to break database connectivity

### Database Driver Exploitation
1. **Driver Confusion:** Use unsupported or malicious database types
2. **Version Exploitation:** Exploit specific driver vulnerabilities
3. **Connection Abuse:** Create excessive connections through driver manipulation
4. **Error Harvesting:** Extract system information through driver errors

### Information Disclosure Attacks
1. **Configuration Harvesting:** Extract database configuration from logs
2. **Performance Profiling:** Profile system performance through configuration
3. **Architecture Mapping:** Map system architecture through connection details
4. **Operational Intelligence:** Gather operational details through verbose logging

## Recommendations

### Immediate Actions
1. **Validate Input Parameters:** Add comprehensive validation for all parameters
2. **Sanitize Connection Strings:** Implement proper URL parsing and validation
3. **Validate Environment Variables:** Add bounds checking and validation
4. **Sanitize Logging:** Remove sensitive information from logs

### Enhanced Security Implementation

```go
package server

import (
    "fmt"
    "net/url"
    "regexp"
    "strconv"
    "strings"
    "time"
    
    _ "github.com/go-sql-driver/mysql"
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
    _ "github.com/mattn/go-sqlite3"
    log "github.com/sirupsen/logrus"
)

const (
    // Connection pool limits
    minIdleConnections    = 1
    maxIdleConnections    = 100
    minOpenConnections    = 1
    maxOpenConnections    = 1000
    minLifetimeMinutes    = 1
    maxLifetimeMinutes    = 1440  // 24 hours
    minIdleTimeMinutes    = 1
    maxIdleTimeMinutes    = 60    // 1 hour
    
    // Default values
    defaultIdleConnections = 10
    defaultOpenConnections = 50
    defaultLifetimeMinutes = 5
    defaultIdleTimeMinutes = 5
)

var (
    // Supported database types
    supportedDbTypes = map[string]bool{
        "mysql":    true,
        "postgres": true,
        "sqlite3":  true,
        "sqlite":   true,
    }
    
    // Safe parameter pattern for connection strings
    safeParamPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
)

// validateDbType validates the database type parameter
func validateDbType(dbType string) error {
    if dbType == "" {
        return fmt.Errorf("database type cannot be empty")
    }
    
    if !supportedDbTypes[dbType] {
        return fmt.Errorf("unsupported database type: %s", dbType)
    }
    
    return nil
}

// validateConnectionString validates and sanitizes connection string
func validateConnectionString(dbType, connectionString string) (string, error) {
    if connectionString == "" {
        return "", fmt.Errorf("connection string cannot be empty")
    }
    
    // Limit connection string length
    if len(connectionString) > 2048 {
        return "", fmt.Errorf("connection string too long: %d characters", len(connectionString))
    }
    
    // Parse connection string based on database type
    switch dbType {
    case "mysql":
        return validateMySQLConnectionString(connectionString)
    case "postgres":
        return validatePostgreSQLConnectionString(connectionString)
    case "sqlite3", "sqlite":
        return validateSQLiteConnectionString(connectionString)
    default:
        return "", fmt.Errorf("unsupported database type for validation: %s", dbType)
    }
}

// validateMySQLConnectionString validates MySQL connection string
func validateMySQLConnectionString(connectionString string) (string, error) {
    // Parse MySQL DSN format: [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
    
    // Check for dangerous patterns
    dangerousPatterns := []string{
        "allowAllFiles=true", "allowLoadLocalInfile=true",
        "autocommit=false", "sql_mode=", "init_command=",
    }
    
    lowerConn := strings.ToLower(connectionString)
    for _, pattern := range dangerousPatterns {
        if strings.Contains(lowerConn, pattern) {
            return "", fmt.Errorf("connection string contains dangerous parameter: %s", pattern)
        }
    }
    
    // Add required charset and collation safely
    parsedConn := connectionString
    
    if !strings.Contains(lowerConn, "charset=") {
        if strings.Contains(parsedConn, "?") {
            parsedConn += "&charset=utf8mb4"
        } else {
            parsedConn += "?charset=utf8mb4"
        }
    }
    
    if !strings.Contains(lowerConn, "collation=") {
        parsedConn += "&collation=utf8mb4_unicode_ci"
    }
    
    // Add security parameters
    securityParams := []string{
        "tls=true",
        "allowNativePasswords=false",
        "allowCleartextPasswords=false",
        "allowOldPasswords=false",
    }
    
    for _, param := range securityParams {
        if !strings.Contains(lowerConn, strings.Split(param, "=")[0]+"=") {
            parsedConn += "&" + param
        }
    }
    
    return parsedConn, nil
}

// validatePostgreSQLConnectionString validates PostgreSQL connection string
func validatePostgreSQLConnectionString(connectionString string) (string, error) {
    // Parse PostgreSQL connection string
    
    // Check for dangerous parameters
    dangerousPatterns := []string{
        "sslmode=disable", "sslmode=allow",
        "application_name=", "search_path=",
    }
    
    lowerConn := strings.ToLower(connectionString)
    for _, pattern := range dangerousPatterns {
        if strings.Contains(lowerConn, pattern) {
            log.Warnf("Potentially dangerous PostgreSQL parameter: %s", pattern)
        }
    }
    
    // Ensure SSL is required if not explicitly set
    if !strings.Contains(lowerConn, "sslmode=") {
        if strings.Contains(connectionString, "?") {
            connectionString += "&sslmode=require"
        } else {
            connectionString += "?sslmode=require"
        }
    }
    
    return connectionString, nil
}

// validateSQLiteConnectionString validates SQLite connection string
func validateSQLiteConnectionString(connectionString string) (string, error) {
    // SQLite connection strings are typically file paths
    
    // Check for dangerous patterns
    dangerousPatterns := []string{
        "../", "~", "/etc/", "/proc/", "/sys/",
        "\\", "|", ";", "&", "`",
    }
    
    for _, pattern := range dangerousPatterns {
        if strings.Contains(connectionString, pattern) {
            return "", fmt.Errorf("SQLite path contains dangerous pattern: %s", pattern)
        }
    }
    
    // Add security parameters
    if !strings.Contains(connectionString, "?") {
        connectionString += "?_journal_mode=WAL&_synchronous=NORMAL&_foreign_keys=ON"
    }
    
    return connectionString, nil
}

// validateEnvInt validates environment variable integer with bounds
func validateEnvInt(envVar, defaultValue string, min, max int64) (int64, error) {
    value := defaultValue
    if envVar != "" {
        value = envVar
    }
    
    parsed, err := strconv.ParseInt(value, 10, 64)
    if err != nil {
        return 0, fmt.Errorf("invalid integer value: %s", value)
    }
    
    if parsed < min || parsed > max {
        return 0, fmt.Errorf("value out of bounds: %d (min: %d, max: %d)", parsed, min, max)
    }
    
    return parsed, nil
}

// getSecureEnvValue safely retrieves and validates environment variables
func getSecureEnvValue(key, defaultValue string, min, max int64) int64 {
    envValue := os.Getenv(key)
    
    validated, err := validateEnvInt(envValue, defaultValue, min, max)
    if err != nil {
        log.Warnf("Invalid environment variable %s: %v, using default: %s", key, err, defaultValue)
        defaultParsed, _ := strconv.ParseInt(defaultValue, 10, 64)
        return defaultParsed
    }
    
    return validated
}

// GetSecureDbConnection creates a secure database connection with validation
func GetSecureDbConnection(dbType string, connectionString string) (*sqlx.DB, error) {
    // Validate database type
    if err := validateDbType(dbType); err != nil {
        return nil, fmt.Errorf("invalid database type: %v", err)
    }
    
    // Validate and sanitize connection string
    validatedConnectionString, err := validateConnectionString(dbType, connectionString)
    if err != nil {
        return nil, fmt.Errorf("invalid connection string: %v", err)
    }
    
    // Create database connection
    db, err := sqlx.Open(dbType, validatedConnectionString)
    if err != nil {
        return nil, fmt.Errorf("failed to open database connection: %v", err)
    }
    
    // Configure connection pool with validated parameters
    maxIdleConns := getSecureEnvValue("DAPTIN_MAX_IDLE_CONNECTIONS", 
        strconv.Itoa(defaultIdleConnections), minIdleConnections, maxIdleConnections)
    
    maxOpenConns := getSecureEnvValue("DAPTIN_MAX_OPEN_CONNECTIONS", 
        strconv.Itoa(defaultOpenConnections), minOpenConnections, maxOpenConnections)
    
    // SQLite special handling
    if strings.Contains(dbType, "sqlite") {
        maxOpenConns = 1
        log.Infof("SQLite detected, setting max open connections to 1")
    }
    
    maxLifetimeMin := getSecureEnvValue("DAPTIN_MAX_CONNECTIONS_LIFETIME", 
        strconv.Itoa(defaultLifetimeMinutes), minLifetimeMinutes, maxLifetimeMinutes)
    
    maxIdleTimeMin := getSecureEnvValue("DAPTIN_MAX_IDLE_CONNECTIONS_TIME", 
        strconv.Itoa(defaultIdleTimeMinutes), minIdleTimeMinutes, maxIdleTimeMinutes)
    
    // Apply connection pool settings
    db.SetMaxIdleConns(int(maxIdleConns))
    db.SetMaxOpenConns(int(maxOpenConns))
    db.SetConnMaxLifetime(time.Duration(maxLifetimeMin) * time.Minute)
    db.SetConnMaxIdleTime(time.Duration(maxIdleTimeMin) * time.Minute)
    
    // Test connection
    if err := db.Ping(); err != nil {
        db.Close()
        return nil, fmt.Errorf("database connection test failed: %v", err)
    }
    
    // Log configuration (sanitized)
    log.Infof("Database connection established: type=%s, maxIdle=%d, maxOpen=%d, maxLifetime=%dm, maxIdleTime=%dm",
        dbType, maxIdleConns, maxOpenConns, maxLifetimeMin, maxIdleTimeMin)
    
    return db, nil
}

// GetDbConnection maintains backward compatibility
func GetDbConnection(dbType string, connectionString string) (*sqlx.DB, error) {
    return GetSecureDbConnection(dbType, connectionString)
}

// ConnectionHealthCheck performs health check on database connection
func ConnectionHealthCheck(db *sqlx.DB) error {
    if db == nil {
        return fmt.Errorf("database connection is nil")
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    return db.PingContext(ctx)
}

// GetConnectionStats returns connection pool statistics
func GetConnectionStats(db *sqlx.DB) map[string]interface{} {
    if db == nil {
        return map[string]interface{}{"error": "database connection is nil"}
    }
    
    stats := db.Stats()
    return map[string]interface{}{
        "max_open_connections":     stats.MaxOpenConnections,
        "open_connections":         stats.OpenConnections,
        "in_use":                  stats.InUse,
        "idle":                    stats.Idle,
        "wait_count":              stats.WaitCount,
        "wait_duration":           stats.WaitDuration,
        "max_idle_closed":         stats.MaxIdleClosed,
        "max_idle_time_closed":    stats.MaxIdleTimeClosed,
        "max_lifetime_closed":     stats.MaxLifetimeClosed,
    }
}
```

### Long-term Improvements
1. **Connection Pool Monitoring:** Implement comprehensive monitoring of connection pool health
2. **Dynamic Configuration:** Support runtime configuration changes for connection settings
3. **Connection Encryption:** Ensure all database connections use encryption in transit
4. **Certificate Management:** Implement proper SSL/TLS certificate validation
5. **Audit Logging:** Add audit logging for all database connection events

## Edge Cases Identified

1. **SQLite Concurrent Access:** Multiple connections to SQLite database causing locks
2. **Connection String Encoding:** Special characters in connection strings causing parsing issues
3. **Environment Variable Precedence:** Multiple sources of configuration with conflicting values
4. **Database Driver Conflicts:** Multiple database drivers loaded causing conflicts
5. **Connection Pool Exhaustion:** All connections in use causing application blocking
6. **Network Connectivity Issues:** Intermittent network causing connection failures
7. **Database Server Restart:** Database server restart while connections are active
8. **SSL Certificate Expiry:** Database SSL certificates expiring causing connection failures
9. **Memory Pressure:** High memory usage affecting connection pool performance
10. **Container Resource Limits:** Container memory/CPU limits affecting database connections

## Security Best Practices Violations

1. **Connection string injection** through unvalidated parameter manipulation
2. **Environment variable injection** without bounds checking or validation
3. **Missing input validation** for critical database parameters
4. **Information disclosure** through detailed logging of configuration
5. **Silent error handling** masking potential security issues
6. **Hardcoded configuration** limiting security flexibility
7. **No connection string parsing** enabling injection attacks
8. **Missing encryption enforcement** for database connections
9. **No certificate validation** for SSL/TLS connections
10. **Lack of connection monitoring** for security events

## Positive Security Aspects

1. **Connection pooling** with configurable limits
2. **Multiple database support** with appropriate drivers
3. **UTF-8 encoding enforcement** for MySQL
4. **Timeout configuration** for connection management
5. **Error handling** for connection failures

## Critical Issues Summary

1. **Connection String Injection:** Unvalidated connection string manipulation enables injection attacks
2. **Environment Variable Injection:** Environment variables used without validation for critical settings
3. **Missing Input Validation:** Database parameters not validated for security
4. **Information Disclosure:** Database configuration details logged without sanitization
5. **Error Handling Issues:** Silent error handling masking potential attacks
6. **Resource Management Configuration:** Hardcoded limits and potential misconfiguration
7. **String Parsing Vulnerabilities:** Simple string operations without proper parsing

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** HIGH - Database connection management with injection vulnerabilities and missing validation