# Security Analysis: server/config.go

**File:** `server/config.go`  
**Type:** Configuration file loader and schema parser  
**Lines of Code:** 191  

## Overview
This file handles loading and parsing of schema configuration files for the Daptin CMS system. It searches for files matching the pattern `schema_*.*` and supports JSON, YAML, and TOML formats. The configuration includes table definitions, relations, imports, actions, state machines, and other system components.

## Key Components

### LoadConfigFiles function
**Lines:** 22-190  
**Purpose:** Main configuration loader that searches and parses schema files  

### Configuration Processing
- **Environment path handling:** Lines 47-60
- **File discovery:** Lines 62-64
- **Format detection:** Lines 83-109
- **Schema validation:** Lines 120-133
- **Configuration merging:** Lines 134-152

## Security Analysis

### 1. CRITICAL: Path Traversal Vulnerability - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 47-60, 140-143  
**Issue:** Environment-controlled path manipulation without validation enables directory traversal attacks.

```go
schemaPath, specifiedSchemaPath := os.LookupEnv("DAPTIN_SCHEMA_FOLDER")
// No validation of schemaPath
files1, _ = filepath.Glob(schemaPath + "schema_*.*")

// Import path manipulation
if importPath.FilePath[0] != '/' {
    importPath.FilePath = schemaPath + importPath.FilePath  // Path injection
}
```

**Risk:**
- **Directory traversal** through malicious DAPTIN_SCHEMA_FOLDER values
- **Arbitrary file access** via path injection in import paths
- **Configuration tampering** by loading files from unintended locations
- **System compromise** through malicious schema file injection

### 2. HIGH: Unsafe File Loading - HIGH RISK
**Severity:** HIGH  
**Lines:** 74-78, 87-107  
**Issue:** Files loaded and parsed without security validation or size limits.

```go
fileBytes, err := os.ReadFile(fileName)  // No size limits or validation
err = json1.Unmarshal(fileBytes, &initConfig)  // Unsafe unmarshaling
err = toml.Unmarshal(fileBytes, &initConfig)   // Unsafe unmarshaling
```

**Risk:**
- **Memory exhaustion** from extremely large configuration files
- **Resource exhaustion** through malformed or complex configurations
- **Code injection** via unsafe unmarshaling of untrusted data
- **Denial of service** through malicious configuration content

### 3. HIGH: Environment Variable Injection - HIGH RISK
**Severity:** HIGH  
**Lines:** 47  
**Issue:** Environment variable directly used in file path operations without validation.

```go
schemaPath, specifiedSchemaPath := os.LookupEnv("DAPTIN_SCHEMA_FOLDER")
// Direct use without validation
```

**Risk:**
- **Configuration directory manipulation** through environment control
- **Unauthorized file access** via path manipulation
- **Schema pollution** by pointing to malicious configuration directories
- **Container escape** in containerized environments

### 4. MEDIUM: Unsafe String Operations - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 56-59, 140-143  
**Issue:** String manipulation without proper validation for path construction.

```go
if schemaPath[len(schemaPath)-1] != os.PathSeparator {
    schemaPath = schemaPath + string(os.PathSeparator)  // No bounds checking
}
```

**Risk:**
- **Index out of bounds** panic from empty schemaPath
- **Path injection** through malformed path separators
- **Application crash** from string manipulation edge cases
- **Resource exhaustion** from infinite path construction loops

### 5. MEDIUM: Error Information Disclosure - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 88, 94, 115, 155, 165, 176  
**Issue:** Detailed error information and system paths logged without sanitization.

```go
log.Debugf("YAML: %v: %v", string(jsonBytes), err)
log.Errorf("Failed to load config file: %v", err)
log.Printf("Error, column without name: %v", table)
```

**Risk:**
- **File system path disclosure** through error messages
- **Configuration structure exposure** via debug logging
- **System information leakage** through detailed error reporting
- **Attack surface mapping** through verbose logging

### 6. MEDIUM: Missing Input Validation - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 122-130, 160-168  
**Issue:** Configuration data processed without comprehensive validation.

```go
table.TableName = flect.Underscore(table.TableName)  // No validation before processing
table.Columns[j].ColumnName = flect.Underscore(col.ColumnName)  // No validation
```

**Risk:**
- **SQL injection** through malicious table/column names
- **Configuration corruption** via invalid naming patterns
- **System instability** from malformed configuration structures
- **Database schema pollution** through invalid identifiers

### 7. LOW: Resource Management Issues - LOW RISK
**Severity:** LOW  
**Lines:** 74-78, 87-107  
**Issue:** No resource limits on file operations and memory allocation.

```go
fileBytes, err := os.ReadFile(fileName)  // No size limits
// Multiple unmarshaling operations without memory limits
```

**Risk:**
- **Memory exhaustion** from large configuration files
- **Disk I/O exhaustion** from excessive file operations
- **Performance degradation** under resource pressure
- **Service disruption** through resource consumption

## Potential Attack Vectors

### Path Traversal Attacks
1. **Environment Manipulation:** Set DAPTIN_SCHEMA_FOLDER to "../../../etc/" to access system files
2. **Import Path Injection:** Craft relative paths in import configurations to escape schema directory
3. **Symlink Following:** Use symbolic links to redirect file access to sensitive locations
4. **Schema Directory Poisoning:** Point to directories containing malicious schema files

### Configuration Injection Attacks
1. **Malicious Schema Files:** Place crafted schema files in accessible directories
2. **YAML/JSON Bomb:** Create deeply nested or extremely large configuration files
3. **Import Chain Exploitation:** Create circular or deep import chains to exhaust resources
4. **Table/Column Injection:** Inject malicious names that become SQL identifiers

### Information Disclosure Attacks
1. **Error Message Harvesting:** Trigger errors to extract file system information
2. **Debug Information Extraction:** Enable debug logging to expose internal data
3. **Configuration Enumeration:** Use error responses to map configuration structure
4. **Path Discovery:** Use path-related errors to map file system structure

### Resource Exhaustion Attacks
1. **Memory Bomb:** Create extremely large configuration files to exhaust memory
2. **CPU Exhaustion:** Craft complex configurations that consume excessive CPU during parsing
3. **Disk I/O Flooding:** Generate numerous file operations to exhaust I/O capacity
4. **Connection Pool Exhaustion:** Create configurations that exhaust database connections

## Recommendations

### Immediate Actions
1. **Validate Environment Variables:** Add validation for DAPTIN_SCHEMA_FOLDER path
2. **Implement Path Sanitization:** Sanitize all file paths before use
3. **Add File Size Limits:** Implement maximum file size restrictions
4. **Sanitize Logging:** Remove sensitive information from log messages

### Enhanced Security Implementation

```go
package server

import (
    json1 "encoding/json"
    "fmt"
    "path/filepath"
    "regexp"
    "strings"
    "os"
    
    "github.com/artpar/api2go/v2"
    "github.com/daptin/daptin/server/actionresponse"
    "github.com/daptin/daptin/server/fsm"
    "github.com/daptin/daptin/server/resource"
    "github.com/daptin/daptin/server/rootpojo"
    "github.com/daptin/daptin/server/table_info"
    yaml2 "github.com/ghodss/yaml"
    "github.com/gobuffalo/flect"
    "github.com/naoina/toml"
    log "github.com/sirupsen/logrus"
)

const (
    maxConfigFileSize     = 10 * 1024 * 1024  // 10MB max per file
    maxTotalConfigSize    = 50 * 1024 * 1024  // 50MB total
    maxConfigFiles        = 100               // Maximum number of config files
    maxTableNameLength    = 64               // Maximum table name length
    maxColumnNameLength   = 64               // Maximum column name length
    maxImportDepth        = 10               // Maximum import nesting depth
)

var (
    // Safe path pattern for schema directories
    safePathPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
    
    // Safe identifier pattern for tables and columns
    safeIdentifierPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
    
    // Dangerous path components to reject
    dangerousPathComponents = []string{
        "..", "~", "$", "`", ";", "|", "&", 
        "etc", "root", "proc", "sys", "dev",
    }
)

// sanitizePath safely validates and sanitizes file paths
func sanitizePath(path string) (string, error) {
    if path == "" {
        return "", fmt.Errorf("path cannot be empty")
    }
    
    // Clean the path
    cleaned := filepath.Clean(path)
    
    // Check for dangerous patterns
    if !safePathPattern.MatchString(cleaned) {
        return "", fmt.Errorf("path contains invalid characters")
    }
    
    // Check for dangerous components
    for _, component := range dangerousPathComponents {
        if strings.Contains(strings.ToLower(cleaned), component) {
            return "", fmt.Errorf("path contains dangerous component: %s", component)
        }
    }
    
    // Ensure path is absolute for security
    if !filepath.IsAbs(cleaned) {
        wd, err := os.Getwd()
        if err != nil {
            return "", fmt.Errorf("failed to get working directory: %v", err)
        }
        cleaned = filepath.Join(wd, cleaned)
    }
    
    return cleaned, nil
}

// validateSchemaPath validates the schema directory path
func validateSchemaPath(schemaPath string) (string, error) {
    if schemaPath == "" {
        schemaPath = "."
    }
    
    sanitized, err := sanitizePath(schemaPath)
    if err != nil {
        return "", fmt.Errorf("invalid schema path: %v", err)
    }
    
    // Verify directory exists and is accessible
    info, err := os.Stat(sanitized)
    if err != nil {
        return "", fmt.Errorf("schema directory not accessible: %v", err)
    }
    
    if !info.IsDir() {
        return "", fmt.Errorf("schema path is not a directory")
    }
    
    return sanitized, nil
}

// validateConfigFile validates configuration file before processing
func validateConfigFile(filePath string) error {
    // Check file size
    info, err := os.Stat(filePath)
    if err != nil {
        return fmt.Errorf("cannot access file: %v", err)
    }
    
    if info.Size() > maxConfigFileSize {
        return fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), maxConfigFileSize)
    }
    
    // Validate file extension
    ext := strings.ToLower(filepath.Ext(filePath))
    validExtensions := map[string]bool{
        ".json": true,
        ".yaml": true,
        ".yml":  true,
        ".toml": true,
    }
    
    if !validExtensions[ext] {
        return fmt.Errorf("unsupported file extension: %s", ext)
    }
    
    return nil
}

// validateIdentifier validates table and column names
func validateIdentifier(name, context string) error {
    if name == "" {
        return fmt.Errorf("%s name cannot be empty", context)
    }
    
    maxLength := maxTableNameLength
    if context == "column" {
        maxLength = maxColumnNameLength
    }
    
    if len(name) > maxLength {
        return fmt.Errorf("%s name too long: max %d characters", context, maxLength)
    }
    
    if !safeIdentifierPattern.MatchString(name) {
        return fmt.Errorf("%s name contains invalid characters: %s", context, name)
    }
    
    // Check for SQL reserved words
    reservedWords := []string{
        "select", "insert", "update", "delete", "drop", "create",
        "alter", "table", "database", "schema", "user", "password",
        "admin", "root", "sys", "system",
    }
    
    lowerName := strings.ToLower(name)
    for _, reserved := range reservedWords {
        if lowerName == reserved {
            return fmt.Errorf("%s name is reserved: %s", context, name)
        }
    }
    
    return nil
}

// secureUnmarshal safely unmarshals configuration data
func secureUnmarshal(data []byte, format string, config *resource.CmsConfig) error {
    if len(data) > maxConfigFileSize {
        return fmt.Errorf("data too large for unmarshaling")
    }
    
    // Check for potential bomb patterns
    if strings.Count(string(data), "{") > 10000 || strings.Count(string(data), "[") > 10000 {
        return fmt.Errorf("configuration structure too complex")
    }
    
    switch format {
    case "json":
        return json1.Unmarshal(data, config)
    case "yaml", "yml":
        jsonBytes, err := yaml2.YAMLToJSON(data)
        if err != nil {
            return fmt.Errorf("YAML to JSON conversion failed: %v", err)
        }
        return json1.Unmarshal(jsonBytes, config)
    case "toml":
        return toml.Unmarshal(data, config)
    default:
        return fmt.Errorf("unsupported format: %s", format)
    }
}

// LoadSecureConfigFiles loads configuration files with security validation
func LoadSecureConfigFiles() (resource.CmsConfig, []error) {
    errs := make([]error, 0)
    var globalInitConfig resource.CmsConfig
    
    // Initialize with standard components
    globalInitConfig = resource.CmsConfig{
        Tables:                   make([]table_info.TableInfo, 0),
        Relations:                make([]api2go.TableRelation, 0),
        Imports:                  make([]rootpojo.DataFileImport, 0),
        EnableGraphQL:            false,
        Actions:                  make([]actionresponse.Action, 0),
        StateMachineDescriptions: make([]fsm.LoopbookFsmDescription, 0),
        Streams:                  make([]resource.StreamContract, 0),
    }
    
    // Add standard components
    globalInitConfig.Tables = append(globalInitConfig.Tables, resource.StandardTables...)
    globalInitConfig.Tasks = append(globalInitConfig.Tasks, resource.StandardTasks...)
    globalInitConfig.Actions = append(globalInitConfig.Actions, resource.SystemActions...)
    globalInitConfig.Streams = append(globalInitConfig.Streams, resource.StandardStreams...)
    globalInitConfig.StateMachineDescriptions = append(globalInitConfig.StateMachineDescriptions, resource.SystemSmds...)
    globalInitConfig.ExchangeContracts = append(globalInitConfig.ExchangeContracts, resource.SystemExchanges...)
    
    // Validate and sanitize schema path
    schemaPathEnv, specifiedSchemaPath := os.LookupEnv("DAPTIN_SCHEMA_FOLDER")
    var schemaPath string
    var err error
    
    if specifiedSchemaPath {
        schemaPath, err = validateSchemaPath(schemaPathEnv)
        if err != nil {
            log.Errorf("Invalid schema path from environment: %v", err)
            errs = append(errs, err)
            schemaPath = "." // Fallback to current directory
        }
    } else {
        schemaPath = "."
    }
    
    // Find configuration files securely
    var files []string
    
    // Search in current directory
    currentFiles, err := filepath.Glob("schema_*.*")
    if err != nil {
        errs = append(errs, fmt.Errorf("failed to search current directory: %v", err))
    } else {
        files = append(files, currentFiles...)
    }
    
    // Search in schema path if different from current
    if schemaPath != "." {
        schemaFiles, err := filepath.Glob(filepath.Join(schemaPath, "schema_*.*"))
        if err != nil {
            errs = append(errs, fmt.Errorf("failed to search schema directory: %v", err))
        } else {
            files = append(files, schemaFiles...)
        }
    }
    
    // Limit number of files
    if len(files) > maxConfigFiles {
        log.Warnf("Too many config files found (%d), limiting to %d", len(files), maxConfigFiles)
        files = files[:maxConfigFiles]
    }
    
    log.Infof("Found %d configuration files to process", len(files))
    
    totalSize := int64(0)
    processedFiles := 0
    
    for _, fileName := range files {
        // Validate file
        if err := validateConfigFile(fileName); err != nil {
            log.Warnf("Skipping invalid config file %s: %v", fileName, err)
            continue
        }
        
        // Check total size limit
        info, _ := os.Stat(fileName)
        if totalSize+info.Size() > maxTotalConfigSize {
            log.Warnf("Total config size limit exceeded, skipping remaining files")
            break
        }
        totalSize += info.Size()
        
        log.Infof("Processing configuration file: %s", fileName)
        
        // Read file securely
        fileBytes, err := os.ReadFile(fileName)
        if err != nil {
            log.Errorf("Failed to read config file %s: %v", fileName, err)
            errs = append(errs, err)
            continue
        }
        
        // Parse configuration securely
        initConfig := resource.CmsConfig{}
        ext := strings.ToLower(filepath.Ext(fileName))
        format := strings.TrimPrefix(ext, ".")
        
        err = secureUnmarshal(fileBytes, format, &initConfig)
        if err != nil {
            log.Errorf("Failed to parse config file %s: %v", fileName, err)
            errs = append(errs, err)
            continue
        }
        
        // Validate and process tables
        validTables := make([]table_info.TableInfo, 0)
        for _, table := range initConfig.Tables {
            // Validate table name
            tableName := flect.Underscore(table.TableName)
            if err := validateIdentifier(tableName, "table"); err != nil {
                log.Warnf("Skipping invalid table %s: %v", table.TableName, err)
                continue
            }
            table.TableName = tableName
            
            // Validate columns
            validColumns := make([]table_info.ColumnInfo, 0)
            for _, col := range table.Columns {
                columnName := flect.Underscore(col.ColumnName)
                if err := validateIdentifier(columnName, "column"); err != nil {
                    log.Warnf("Skipping invalid column %s.%s: %v", table.TableName, col.ColumnName, err)
                    continue
                }
                col.ColumnName = columnName
                
                // Ensure Name and ColumnName consistency
                if col.Name == "" && col.ColumnName != "" {
                    col.Name = col.ColumnName
                } else if col.Name != "" && col.ColumnName == "" {
                    col.ColumnName = col.Name
                } else if col.Name == "" && col.ColumnName == "" {
                    log.Warnf("Column without name in table %s, skipping", table.TableName)
                    continue
                }
                
                validColumns = append(validColumns, col)
            }
            table.Columns = validColumns
            
            if len(validColumns) > 0 {
                validTables = append(validTables, table)
            }
        }
        initConfig.Tables = validTables
        
        // Validate and process imports
        validImports := make([]rootpojo.DataFileImport, 0)
        for _, importPath := range initConfig.Imports {
            if importPath.FilePath == "" {
                continue
            }
            
            // Secure import path handling
            if !filepath.IsAbs(importPath.FilePath) {
                fullPath := filepath.Join(schemaPath, importPath.FilePath)
                sanitizedPath, err := sanitizePath(fullPath)
                if err != nil {
                    log.Warnf("Skipping invalid import path %s: %v", importPath.FilePath, err)
                    continue
                }
                importPath.FilePath = sanitizedPath
            } else {
                sanitizedPath, err := sanitizePath(importPath.FilePath)
                if err != nil {
                    log.Warnf("Skipping invalid import path %s: %v", importPath.FilePath, err)
                    continue
                }
                importPath.FilePath = sanitizedPath
            }
            
            validImports = append(validImports, importPath)
        }
        initConfig.Imports = validImports
        
        // Merge configurations
        globalInitConfig.Tables = append(globalInitConfig.Tables, initConfig.Tables...)
        globalInitConfig.AddRelations(initConfig.Relations...)
        globalInitConfig.Imports = append(globalInitConfig.Imports, initConfig.Imports...)
        globalInitConfig.Streams = append(globalInitConfig.Streams, initConfig.Streams...)
        globalInitConfig.Tasks = append(globalInitConfig.Tasks, initConfig.Tasks...)
        globalInitConfig.Actions = append(globalInitConfig.Actions, initConfig.Actions...)
        globalInitConfig.StateMachineDescriptions = append(globalInitConfig.StateMachineDescriptions, initConfig.StateMachineDescriptions...)
        globalInitConfig.ExchangeContracts = append(globalInitConfig.ExchangeContracts, initConfig.ExchangeContracts...)
        
        if initConfig.EnableGraphQL {
            log.Infof("GraphQL enabled by configuration file: %s", fileName)
            globalInitConfig.EnableGraphQL = true
        }
        
        processedFiles++
        log.Infof("Successfully processed configuration file: %s", fileName)
    }
    
    log.Infof("Configuration loading complete: %d files processed, %d bytes total", processedFiles, totalSize)
    
    return globalInitConfig, errs
}

// LoadConfigFiles maintains backward compatibility
func LoadConfigFiles() (resource.CmsConfig, []error) {
    return LoadSecureConfigFiles()
}
```

### Long-term Improvements
1. **Configuration Schema Validation:** Implement strict JSON schema validation for all configuration files
2. **Digital Signatures:** Add cryptographic signatures for configuration file integrity
3. **Configuration Encryption:** Support for encrypted configuration files containing sensitive data
4. **Audit Logging:** Comprehensive logging of all configuration loading and validation events
5. **Hot Reloading:** Secure hot-reloading of configuration files with validation

## Edge Cases Identified

1. **Empty Schema Directory:** Schema folder exists but contains no valid files
2. **Circular Import Dependencies:** Configuration files that import each other creating loops
3. **Mixed File Formats:** Combination of JSON, YAML, and TOML files with conflicting data
4. **Unicode in Identifiers:** Non-ASCII characters in table and column names
5. **Extremely Deep Nesting:** Configuration files with very deep object/array structures
6. **Concurrent File Access:** Multiple processes loading configuration files simultaneously
7. **File System Permissions:** Configuration files with restricted read permissions
8. **Symbolic Link Following:** Schema files accessed through symbolic links
9. **Network File Systems:** Configuration files stored on NFS or other network filesystems
10. **Container Volume Mounts:** Configuration files in Docker volume mounts with special permissions

## Security Best Practices Violations

1. **Path traversal vulnerability** through unvalidated environment variables
2. **Unsafe file loading** without size limits or content validation
3. **Environment variable injection** without sanitization
4. **Missing input validation** for configuration data structures
5. **Information disclosure** through verbose error logging
6. **No resource limits** for file operations and memory allocation
7. **Unsafe string operations** leading to potential panics
8. **Missing file type validation** beyond extension checking
9. **No content sanitization** for loaded configuration data
10. **Lack of access controls** for configuration file access

## Positive Security Aspects

1. **Error collection and reporting** for troubleshooting
2. **Support for multiple configuration formats** providing flexibility
3. **Structured configuration loading** with clear separation of concerns
4. **Logging of configuration processing** for audit trails

## Critical Issues Summary

1. **Path Traversal Vulnerability:** Environment-controlled path manipulation enables directory traversal
2. **Unsafe File Loading:** Files loaded without security validation or size limits
3. **Environment Variable Injection:** Environment variables used directly in path operations
4. **Unsafe String Operations:** String manipulation without bounds checking
5. **Error Information Disclosure:** Detailed system information exposed in logs
6. **Missing Input Validation:** Configuration data processed without comprehensive validation
7. **Resource Management Issues:** No limits on file operations and memory allocation

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - Configuration loading with path traversal and environment injection vulnerabilities