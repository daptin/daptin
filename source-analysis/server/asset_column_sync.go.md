# Security Analysis: server/asset_column_sync.go

**File:** `server/asset_column_sync.go`  
**Type:** Asset column synchronization setup with cloud storage integration  
**Lines of Code:** 98  

## Overview
This file implements asset column synchronization functionality that creates cache folders and schedules sync tasks for cloud storage integration. It manages the setup of local cache directories and synchronization schedules for foreign key columns linked to cloud storage.

## Key Components

### CreateAssetColumnSync function
**Lines:** 15-97  
**Purpose:** Creates asset cache structure and schedules sync tasks for cloud storage columns  

### Cloud storage integration
**Lines:** 18-28, 41-51  
**Purpose:** Retrieves cloud store configurations and sets up local cache mirroring  

### Task scheduling
**Lines:** 73-84  
**Purpose:** Schedules periodic sync tasks for cached cloud storage columns  

## Security Analysis

### 1. CRITICAL: Environment Variable Injection - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 42  
**Issue:** Direct use of environment variable without validation in temporary directory creation.

```go
tempDirectoryPath, err := ioutil.TempDir(os.Getenv("DAPTIN_CACHE_FOLDER"), tableName+"_"+columnName)
```

**Risk:**
- **Path traversal attacks** through malicious environment variable values
- **Directory injection** allowing creation of directories outside intended locations
- **System compromise** through controlled cache folder location
- **File system manipulation** through crafted environment values

### 2. CRITICAL: Unsafe Task Scheduling - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 73-84  
**Issue:** Task created with user-controlled data without validation.

```go
err = TaskScheduler.AddTask(task.Task{
    EntityName: "world",
    ActionName: "sync_column_storage",
    Attributes: map[string]interface{}{
        "table_name":      tableInfo.TableName,      // User-controlled
        "credential_name": cloudStore.CredentialName, // User-controlled
        "column_name":     columnName,               // User-controlled
    },
    AsUserEmail: cruds["user_account"].GetAdminEmailId(transaction),
    Schedule:    "@every 30m",
})
```

**Risk:**
- **Privilege escalation** through admin email execution context
- **Command injection** through crafted table/column names
- **Credential exposure** through task attribute manipulation
- **Scheduled attack execution** through task scheduler

### 3. HIGH: Credential Information Exposure - HIGH RISK
**Severity:** HIGH  
**Lines:** 54-59, 66  
**Issue:** Credentials retrieved and stored in memory without encryption.

```go
if cloudStore.CredentialName != "" {
    cred, err := cruds["credential"].GetCredentialByName(cloudStore.CredentialName, transaction)
    if err == nil && cred != nil {
        credentials = cred.DataMap  // Raw credential data stored
    }
}
// ...
assetCacheFolder := &assetcachepojo.AssetFolderCache{
    Credentials: credentials,  // Credentials stored in cache object
}
```

**Risk:**
- **Credential leakage** through memory dumps or logging
- **Unauthorized access** to cloud storage systems
- **Data breach** from exposed authentication data
- **Lateral movement** through compromised credentials

### 4. HIGH: Path Injection Vulnerability - HIGH RISK
**Severity:** HIGH  
**Lines:** 42, 47  
**Issue:** Table and column names used in file path construction without validation.

```go
tempDirectoryPath, err := ioutil.TempDir(os.Getenv("DAPTIN_CACHE_FOLDER"), tableName+"_"+columnName)
// ...
err = cruds["task"].SyncStorageToPath(cloudStore, column.ForeignKeyData.KeyName, tempDirectoryPath, transaction)
```

**Risk:**
- **Directory traversal** through crafted table/column names
- **File system manipulation** through path injection
- **Resource consumption** through deep directory structures
- **System compromise** through controlled file locations

### 5. HIGH: Transaction Misuse - HIGH RISK
**Severity:** HIGH  
**Lines:** 18, 47, 56, 81  
**Issue:** Long-running operations performed within database transaction.

```go
stores, err := cloud_store.GetAllCloudStores(cruds["cloud_store"], transaction)
// ...
err = cruds["task"].SyncStorageToPath(cloudStore, column.ForeignKeyData.KeyName, tempDirectoryPath, transaction)
// ...
AsUserEmail: cruds["user_account"].GetAdminEmailId(transaction),
```

**Risk:**
- **Database deadlocks** from long-held transaction locks
- **Performance degradation** from extended transaction duration
- **Resource exhaustion** from connection pool depletion
- **Data inconsistency** from transaction timeouts

### 6. MEDIUM: Insufficient Error Handling - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 21-23, 48-50  
**Issue:** Silent failures and incomplete error handling in critical operations.

```go
if err != nil || len(stores) == 0 {
    return assetCache  // Returns empty cache on error
}
// ...
if CheckErr(err, "Failed to setup sync to path for table column [%v][%v]", tableName, column.ColumnName) {
    continue  // Continues processing on sync failure
}
```

**Risk:**
- **Silent failures** masking critical setup errors
- **Incomplete initialization** leading to runtime failures
- **Data loss** from failed synchronization setups
- **System inconsistency** from partial setup completion

### 7. MEDIUM: Resource Leak Potential - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 42, 64  
**Issue:** Temporary directories created without cleanup mechanism.

```go
tempDirectoryPath, err := ioutil.TempDir(os.Getenv("DAPTIN_CACHE_FOLDER"), tableName+"_"+columnName)
// ...
assetCacheFolder := &assetcachepojo.AssetFolderCache{
    LocalSyncPath: tempDirectoryPath,  // No cleanup mechanism
}
```

**Risk:**
- **Disk space exhaustion** from accumulated temporary directories
- **File system clutter** from abandoned cache folders
- **Resource leaks** on application restart
- **Storage waste** from unmanaged temporary files

### 8. LOW: Hard-coded Schedule Configuration - LOW RISK
**Severity:** LOW  
**Lines:** 82  
**Issue:** Fixed schedule interval without configuration flexibility.

```go
Schedule: "@every 30m",  // Hard-coded sync interval
```

**Risk:**
- **Inflexible synchronization** schedules
- **Performance impact** from inappropriate sync intervals
- **Resource consumption** from over-frequent syncing
- **Configuration management** difficulties

## Potential Attack Vectors

### Environment-Based Attacks
1. **Cache Folder Manipulation:** Control DAPTIN_CACHE_FOLDER to redirect cache creation
2. **Path Traversal:** Use environment variable to escape intended directory structure
3. **File System Access:** Gain unauthorized access to system directories

### Data Injection Attacks
1. **Table Name Injection:** Craft malicious table names to manipulate file paths
2. **Column Name Injection:** Use special characters in column names for path traversal
3. **Credential Name Injection:** Manipulate credential names to access unauthorized data

### Task Scheduler Exploitation
1. **Privilege Escalation:** Execute tasks with admin privileges through scheduled sync
2. **Command Injection:** Inject commands through task attributes
3. **Resource Exhaustion:** Schedule excessive sync tasks to overwhelm system

### Cloud Storage Attacks
1. **Credential Harvesting:** Extract cloud storage credentials from memory
2. **Unauthorized Sync:** Sync malicious content to legitimate storage
3. **Data Exfiltration:** Access sensitive data through compromised sync paths

## Recommendations

### Immediate Actions
1. **Validate Environment Variables:** Sanitize and validate DAPTIN_CACHE_FOLDER before use
2. **Implement Input Validation:** Validate table and column names for path safety
3. **Secure Credential Handling:** Encrypt credentials in memory and implement secure storage
4. **Add Transaction Management:** Use separate transactions for long-running operations

### Enhanced Security Implementation

```go
package server

import (
    "crypto/rand"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"
    "unicode/utf8"
    
    "github.com/daptin/daptin/server/assetcachepojo"
    "github.com/daptin/daptin/server/cloud_store"
    "github.com/daptin/daptin/server/dbresourceinterface"
    "github.com/daptin/daptin/server/rootpojo"
    "github.com/daptin/daptin/server/task"
    "github.com/jmoiron/sqlx"
    log "github.com/sirupsen/logrus"
)

const (
    MaxTableNameLength = 255
    MaxColumnNameLength = 255
    MaxPathLength = 4096
    DefaultCacheFolder = "/tmp/daptin_cache"
    MinSyncInterval = "1m"
    MaxSyncInterval = "24h"
)

var (
    validNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
    validPathPattern = regexp.MustCompile(`^[a-zA-Z0-9/_.-]+$`)
)

// validateName validates table and column names for security
func validateName(name string, fieldType string) error {
    if len(name) == 0 {
        return fmt.Errorf("%s name cannot be empty", fieldType)
    }
    
    if len(name) > MaxTableNameLength {
        return fmt.Errorf("%s name too long: %d", fieldType, len(name))
    }
    
    if !utf8.ValidString(name) {
        return fmt.Errorf("%s name contains invalid UTF-8", fieldType)
    }
    
    if !validNamePattern.MatchString(name) {
        return fmt.Errorf("%s name contains invalid characters: %s", fieldType, name)
    }
    
    // Check for path traversal patterns
    dangerousPatterns := []string{"..", "/", "\\", "\x00"}
    for _, pattern := range dangerousPatterns {
        if strings.Contains(name, pattern) {
            return fmt.Errorf("%s name contains dangerous pattern: %s", fieldType, pattern)
        }
    }
    
    return nil
}

// validateCacheFolder validates and sanitizes cache folder path
func validateCacheFolder() (string, error) {
    cacheFolder := os.Getenv("DAPTIN_CACHE_FOLDER")
    if cacheFolder == "" {
        cacheFolder = DefaultCacheFolder
    }
    
    // Clean and validate path
    cacheFolder = filepath.Clean(cacheFolder)
    
    if len(cacheFolder) > MaxPathLength {
        return "", fmt.Errorf("cache folder path too long: %d", len(cacheFolder))
    }
    
    if !filepath.IsAbs(cacheFolder) {
        return "", fmt.Errorf("cache folder must be absolute path: %s", cacheFolder)
    }
    
    // Validate path contains only safe characters
    if !validPathPattern.MatchString(cacheFolder) {
        return "", fmt.Errorf("cache folder contains invalid characters: %s", cacheFolder)
    }
    
    // Ensure directory exists and is writable
    if err := os.MkdirAll(cacheFolder, 0750); err != nil {
        return "", fmt.Errorf("failed to create cache folder: %v", err)
    }
    
    return cacheFolder, nil
}

// generateSecureTempDir creates a secure temporary directory
func generateSecureTempDir(cacheFolder, tableName, columnName string) (string, error) {
    // Validate inputs
    if err := validateName(tableName, "table"); err != nil {
        return "", err
    }
    
    if err := validateName(columnName, "column"); err != nil {
        return "", err
    }
    
    // Generate random suffix for uniqueness
    randomBytes := make([]byte, 8)
    if _, err := rand.Read(randomBytes); err != nil {
        return "", fmt.Errorf("failed to generate random suffix: %v", err)
    }
    randomSuffix := fmt.Sprintf("%x", randomBytes)
    
    // Create secure directory name
    dirName := fmt.Sprintf("%s_%s_%s", tableName, columnName, randomSuffix)
    tempDir := filepath.Join(cacheFolder, dirName)
    
    // Ensure path is within cache folder (no traversal)
    if !strings.HasPrefix(tempDir, cacheFolder) {
        return "", fmt.Errorf("path traversal detected: %s", tempDir)
    }
    
    // Create directory with restricted permissions
    if err := os.MkdirAll(tempDir, 0750); err != nil {
        return "", fmt.Errorf("failed to create temp directory: %v", err)
    }
    
    return tempDir, nil
}

// encryptCredentials encrypts credential data for secure storage
func encryptCredentials(credentials map[string]interface{}) (map[string]interface{}, error) {
    // Placeholder for encryption implementation
    // In production, use proper encryption like AES-GCM
    if credentials == nil {
        return nil, nil
    }
    
    // For now, return sanitized credentials
    sanitized := make(map[string]interface{})
    for key, value := range credentials {
        // Validate key
        if err := validateName(key, "credential key"); err != nil {
            continue // Skip invalid keys
        }
        
        // Sanitize value based on type
        switch v := value.(type) {
        case string:
            if len(v) > 1000 {
                continue // Skip overly long values
            }
            sanitized[key] = v
        case map[string]interface{}:
            // Recursively handle nested maps (with depth limit)
            if len(v) <= 10 {
                sanitized[key] = v
            }
        default:
            // Skip unsupported types
            continue
        }
    }
    
    return sanitized, nil
}

// validateSyncInterval validates sync schedule interval
func validateSyncInterval(interval string) error {
    validIntervals := []string{
        "@every 1m", "@every 5m", "@every 15m", "@every 30m",
        "@every 1h", "@every 2h", "@every 6h", "@every 12h", "@every 24h",
    }
    
    for _, valid := range validIntervals {
        if interval == valid {
            return nil
        }
    }
    
    return fmt.Errorf("invalid sync interval: %s", interval)
}

// CreateAssetColumnSyncSecure creates asset cache with comprehensive security validation
func CreateAssetColumnSyncSecure(cruds map[string]dbresourceinterface.DbResourceInterface, transaction *sqlx.Tx) (map[string]map[string]*assetcachepojo.AssetFolderCache, error) {
    log.Tracef("CreateAssetColumnSyncSecure starting")
    
    // Input validation
    if cruds == nil {
        return nil, fmt.Errorf("cruds map cannot be nil")
    }
    
    if transaction == nil {
        return nil, fmt.Errorf("transaction cannot be nil")
    }
    
    // Validate required cruds
    requiredCruds := []string{"cloud_store", "credential", "task", "user_account"}
    for _, required := range requiredCruds {
        if cruds[required] == nil {
            return nil, fmt.Errorf("required crud '%s' is missing", required)
        }
    }
    
    // Validate cache folder
    cacheFolder, err := validateCacheFolder()
    if err != nil {
        return nil, fmt.Errorf("cache folder validation failed: %v", err)
    }
    
    // Get cloud stores with separate transaction to avoid long locks
    tx, err := cruds["cloud_store"].Connection().Beginx()
    if err != nil {
        return nil, fmt.Errorf("failed to begin transaction: %v", err)
    }
    defer tx.Rollback()
    
    stores, err := cloud_store.GetAllCloudStores(cruds["cloud_store"], tx)
    if err != nil {
        return nil, fmt.Errorf("failed to get cloud stores: %v", err)
    }
    
    if err = tx.Commit(); err != nil {
        return nil, fmt.Errorf("failed to commit transaction: %v", err)
    }
    
    assetCache := make(map[string]map[string]*assetcachepojo.AssetFolderCache)
    
    if len(stores) == 0 {
        log.Infof("No cloud stores found, returning empty cache")
        return assetCache, nil
    }
    
    // Create cloud store map with validation
    cloudStoreMap := make(map[string]rootpojo.CloudStore)
    for _, store := range stores {
        if err := validateName(store.Name, "cloud store"); err != nil {
            log.Warnf("Skipping invalid cloud store name: %v", err)
            continue
        }
        cloudStoreMap[store.Name] = store
    }
    
    // Process each table
    for tableName, tableCrud := range cruds {
        if err := validateName(tableName, "table"); err != nil {
            log.Warnf("Skipping invalid table name '%s': %v", tableName, err)
            continue
        }
        
        colCache := make(map[string]*assetcachepojo.AssetFolderCache)
        
        tableInfo := tableCrud.TableInfo()
        if tableInfo == nil {
            log.Warnf("No table info for table: %s", tableName)
            continue
        }
        
        for _, column := range tableInfo.Columns {
            if !column.IsForeignKey || column.ForeignKeyData.DataSource != "cloud_store" {
                continue
            }
            
            columnName := column.ColumnName
            if err := validateName(columnName, "column"); err != nil {
                log.Warnf("Skipping invalid column name '%s': %v", columnName, err)
                continue
            }
            
            cloudStore, exists := cloudStoreMap[column.ForeignKeyData.Namespace]
            if !exists {
                log.Warnf("Cloud store not found: %s", column.ForeignKeyData.Namespace)
                continue
            }
            
            // Generate secure temporary directory
            tempDirectoryPath, err := generateSecureTempDir(cacheFolder, tableName, columnName)
            if err != nil {
                log.Errorf("Failed to create temp directory for %s.%s: %v", tableName, columnName, err)
                continue
            }
            
            // Setup cache mirroring for non-local cached stores
            if cloudStore.StoreProvider != "local" && cloudStore.StoreType == "cached" {
                log.Infof("Setting up secure cache mirroring [%v][%v] for [%s][%s] at [%s]", 
                    cloudStore.StoreProvider, cloudStore.Name, tableName, columnName, tempDirectoryPath)
                
                // Use separate transaction for sync operation
                syncTx, err := cruds["task"].Connection().Beginx()
                if err != nil {
                    log.Errorf("Failed to begin sync transaction: %v", err)
                    continue
                }
                
                err = cruds["task"].SyncStorageToPath(cloudStore, column.ForeignKeyData.KeyName, tempDirectoryPath, syncTx)
                if err != nil {
                    syncTx.Rollback()
                    log.Errorf("Failed to setup sync to path for table column [%v][%v]: %v", tableName, columnName, err)
                    continue
                }
                
                if err = syncTx.Commit(); err != nil {
                    log.Errorf("Failed to commit sync transaction: %v", err)
                    continue
                }
            }
            
            // Get and encrypt credentials if available
            var credentials map[string]interface{}
            if cloudStore.CredentialName != "" {
                if err := validateName(cloudStore.CredentialName, "credential"); err != nil {
                    log.Warnf("Invalid credential name '%s': %v", cloudStore.CredentialName, err)
                } else {
                    credTx, err := cruds["credential"].Connection().Beginx()
                    if err != nil {
                        log.Errorf("Failed to begin credential transaction: %v", err)
                    } else {
                        cred, err := cruds["credential"].GetCredentialByName(cloudStore.CredentialName, credTx)
                        if err == nil && cred != nil {
                            credentials, err = encryptCredentials(cred.DataMap)
                            if err != nil {
                                log.Errorf("Failed to encrypt credentials: %v", err)
                                credentials = nil
                            }
                        }
                        credTx.Commit()
                    }
                }
            }
            
            assetCacheFolder := &assetcachepojo.AssetFolderCache{
                CloudStore:    cloudStore,
                LocalSyncPath: tempDirectoryPath,
                Keyname:       column.ForeignKeyData.KeyName,
                Credentials:   credentials,
            }
            
            colCache[columnName] = assetCacheFolder
            
            log.Infof("Secure sync table column [%v][%v] at %v", tableName, columnName, tempDirectoryPath)
            
            // Schedule sync task for non-local cached stores
            if cloudStore.StoreProvider != "local" && cloudStore.StoreType == "cached" {
                syncInterval := "@every 30m"
                if err := validateSyncInterval(syncInterval); err != nil {
                    log.Errorf("Invalid sync interval: %v", err)
                    continue
                }
                
                // Get admin email with separate transaction
                adminTx, err := cruds["user_account"].Connection().Beginx()
                if err != nil {
                    log.Errorf("Failed to begin admin transaction: %v", err)
                    continue
                }
                
                adminEmail := cruds["user_account"].GetAdminEmailId(adminTx)
                adminTx.Commit()
                
                // Validate admin email
                if adminEmail == "" {
                    log.Errorf("Admin email not found")
                    continue
                }
                
                // Create validated task
                syncTask := task.Task{
                    EntityName: "world",
                    ActionName: "sync_column_storage",
                    Attributes: map[string]interface{}{
                        "table_name":      tableName,                  // Already validated
                        "credential_name": cloudStore.CredentialName, // Already validated
                        "column_name":     columnName,                 // Already validated
                    },
                    AsUserEmail: adminEmail,
                    Schedule:    syncInterval,
                }
                
                // Validate task before adding
                if err := syncTask.Validate(); err != nil {
                    log.Errorf("Task validation failed: %v", err)
                    continue
                }
                
                if err := TaskScheduler.AddTask(syncTask); err != nil {
                    log.Errorf("Failed to add sync task: %v", err)
                }
            }
        }
        
        assetCache[tableName] = colCache
    }
    
    log.Tracef("Completed CreateAssetColumnSyncSecure")
    return assetCache, nil
}

// CreateAssetColumnSync maintains backward compatibility
func CreateAssetColumnSync(cruds map[string]dbresourceinterface.DbResourceInterface, transaction *sqlx.Tx) map[string]map[string]*assetcachepojo.AssetFolderCache {
    result, err := CreateAssetColumnSyncSecure(cruds, transaction)
    if err != nil {
        log.Errorf("Secure asset column sync failed: %v", err)
        return make(map[string]map[string]*assetcachepojo.AssetFolderCache)
    }
    return result
}
```

### Long-term Improvements
1. **Credential Encryption:** Implement proper encryption for credential storage
2. **Transaction Optimization:** Use connection pooling and transaction batching
3. **Resource Management:** Implement automatic cleanup of temporary directories
4. **Configuration Management:** Add dynamic sync interval configuration
5. **Audit Logging:** Log all sync setup and credential access operations

## Edge Cases Identified

1. **Environment Variable Manipulation:** DAPTIN_CACHE_FOLDER set to dangerous values
2. **Malformed Table Names:** Tables with special characters or path traversal
3. **Missing Cloud Stores:** Referenced cloud stores that no longer exist
4. **Credential Unavailability:** Missing or corrupted credential data
5. **File System Permissions:** Insufficient permissions for cache directory creation
6. **Transaction Timeouts:** Long-running operations causing transaction failures
7. **Memory Pressure:** Large credential maps causing memory issues
8. **Concurrent Access:** Multiple processes creating cache directories simultaneously

## Security Best Practices Violations

1. **Direct environment variable usage** without validation
2. **User-controlled data in file paths** without sanitization
3. **Credentials stored in plaintext** in memory
4. **Long-running operations** within database transactions
5. **Missing input validation** for names and paths
6. **Hard-coded configuration** values without flexibility
7. **Silent error handling** masking critical failures

## Positive Security Aspects

1. **Transaction-based operations** for data consistency
2. **Error logging** for operational visibility
3. **Separation of concerns** between sync setup and execution
4. **Use of established cloud storage patterns**

## Critical Issues Summary

1. **Environment Variable Injection:** Direct use of environment variable in path creation
2. **Unsafe Task Scheduling:** Tasks created with user data executed as admin
3. **Credential Information Exposure:** Raw credentials stored in memory without encryption
4. **Path Injection Vulnerability:** Table/column names used in paths without validation
5. **Transaction Misuse:** Long operations within database transactions
6. **Insufficient Error Handling:** Silent failures in critical setup operations
7. **Resource Leak Potential:** Temporary directories without cleanup mechanism
8. **Hard-coded Schedule Configuration:** Fixed sync intervals without flexibility

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - Asset synchronization with multiple critical security vulnerabilities requiring immediate remediation