# Security Analysis: server/subsite/subsite_staticfs_server.go

**File:** `server/subsite/subsite_staticfs_server.go`  
**Type:** Static file system server with fallback mechanism  
**Lines of Code:** 19  

## Overview
This file implements a custom HTTP file system that provides a fallback mechanism for serving static files. When a requested file is not found, it serves a default page (typically for SPA applications). This is commonly used for single-page applications that need to serve index.html for client-side routing.

## Key Components

### StaticFsWithDefaultIndex struct
**Lines:** 5-8  
**Purpose:** Wrapper around http.FileSystem that provides 404 fallback functionality  

### Open method
**Lines:** 10-18  
**Purpose:** Opens files with fallback to default page when file not found  

## Security Analysis

### 1. HIGH: Path Traversal Vulnerability - HIGH RISK
**Severity:** HIGH  
**Lines:** 13, 15  
**Issue:** No validation of file paths, allowing potential directory traversal attacks.

```go
f, err := spf.system.Open(name)  // No path validation
if err != nil {
    return spf.system.Open(spf.pageOn404)  // No validation of fallback path
}
```

**Risk:**
- **Directory traversal** through "../" sequences in `name` parameter
- **Unauthorized file access** outside intended directory
- **System file exposure** through crafted path requests
- **No input sanitization** of file paths
- **Potential access to sensitive files** like /etc/passwd, config files

**Impact:** Complete file system access outside the intended web root directory.

### 2. MEDIUM: Fallback Page Path Injection - MEDIUM RISK
**Severity:** MEDIUM  
**Line:** 15  
**Issue:** pageOn404 field not validated, could be manipulated to serve unintended files.

```go
return spf.system.Open(spf.pageOn404)  // No validation of fallback path
```

**Risk:**
- **Arbitrary file serving** if pageOn404 is controlled by attacker
- **Configuration injection** through manipulated fallback paths
- **Information disclosure** through crafted fallback file paths
- **No bounds checking** on fallback file access

### 3. MEDIUM: Missing Error Handling for Fallback - MEDIUM RISK
**Severity:** MEDIUM  
**Line:** 15  
**Issue:** No error handling when fallback page also fails to open.

```go
return spf.system.Open(spf.pageOn404)  // Error not handled
```

**Risk:**
- **Application crashes** if fallback page doesn't exist
- **Infinite recursion** potential in error conditions
- **Resource leaks** from unclosed file handles
- **No graceful degradation** when both primary and fallback fail

### 4. LOW: Information Disclosure Through Error Patterns - LOW RISK
**Severity:** LOW  
**Lines:** 13-16  
**Issue:** Error handling pattern may leak information about file system structure.

**Risk:**
- **File existence enumeration** through error response differences
- **Directory structure disclosure** through error patterns
- **File system fingerprinting** based on error behaviors

## Potential Attack Vectors

### Path Traversal Attacks
1. **Directory Traversal:** Use "../" sequences to access files outside web root
2. **Absolute Path Access:** Use absolute paths to access system files
3. **Symbolic Link Exploitation:** Exploit symbolic links to bypass path restrictions
4. **URL Encoding Bypass:** Use URL encoding to bypass basic path filters

### Information Disclosure Attacks
1. **File Enumeration:** Enumerate files and directories through error responses
2. **System File Access:** Access sensitive system files like /etc/passwd
3. **Configuration File Access:** Access application configuration files
4. **Source Code Access:** Access application source code files

### Denial of Service Attacks
1. **Resource Exhaustion:** Request large files to exhaust server resources
2. **File Handle Exhaustion:** Open many files without closing to exhaust handles
3. **Fallback Loop:** Create conditions where fallback repeatedly fails

## Recommendations

### Immediate Actions
1. **Add Path Validation:** Validate and sanitize all file paths
2. **Implement Path Restrictions:** Restrict access to specific directories
3. **Add Fallback Error Handling:** Handle errors when fallback file fails
4. **Add Path Sanitization:** Clean and normalize all file paths

### Enhanced Security Implementation

```go
package subsite

import (
    "fmt"
    "net/http"
    "path/filepath"
    "strings"
    
    log "github.com/sirupsen/logrus"
)

const (
    MaxPathLength = 4096
    MaxDepth = 10
)

type StaticFsWithDefaultIndex struct {
    system      http.FileSystem
    pageOn404   string
    allowedRoot string  // Root directory restriction
}

// NewStaticFsWithDefaultIndex creates a new secure static file system
func NewStaticFsWithDefaultIndex(system http.FileSystem, pageOn404, allowedRoot string) (*StaticFsWithDefaultIndex, error) {
    // Validate fallback page
    if err := validatePath(pageOn404); err != nil {
        return nil, fmt.Errorf("invalid fallback page path: %v", err)
    }
    
    // Validate allowed root
    if err := validatePath(allowedRoot); err != nil {
        return nil, fmt.Errorf("invalid allowed root path: %v", err)
    }
    
    // Clean paths
    cleanPageOn404 := filepath.Clean(pageOn404)
    cleanAllowedRoot := filepath.Clean(allowedRoot)
    
    return &StaticFsWithDefaultIndex{
        system:      system,
        pageOn404:   cleanPageOn404,
        allowedRoot: cleanAllowedRoot,
    }, nil
}

// validatePath validates file paths for security
func validatePath(path string) error {
    if len(path) == 0 {
        return fmt.Errorf("path cannot be empty")
    }
    
    if len(path) > MaxPathLength {
        return fmt.Errorf("path too long: %d characters", len(path))
    }
    
    // Check for null bytes
    if strings.Contains(path, "\x00") {
        return fmt.Errorf("path contains null bytes")
    }
    
    // Check for directory traversal
    if strings.Contains(path, "..") {
        return fmt.Errorf("path contains directory traversal")
    }
    
    // Check for absolute paths (depending on requirements)
    if filepath.IsAbs(path) {
        return fmt.Errorf("absolute paths not allowed")
    }
    
    // Count path depth
    parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
    if len(parts) > MaxDepth {
        return fmt.Errorf("path too deep: %d levels", len(parts))
    }
    
    // Check for dangerous file names
    for _, part := range parts {
        if isDangerousFileName(part) {
            return fmt.Errorf("dangerous file name detected: %s", part)
        }
    }
    
    return nil
}

// isDangerousFileName checks for potentially dangerous file names
func isDangerousFileName(name string) bool {
    dangerousNames := []string{
        "passwd", "shadow", "hosts", ".env", "config",
        "web.config", ".htaccess", ".git", ".svn",
    }
    
    lowerName := strings.ToLower(name)
    for _, dangerous := range dangerousNames {
        if lowerName == dangerous || strings.Contains(lowerName, dangerous) {
            return true
        }
    }
    
    return false
}

// isWithinAllowedRoot checks if the resolved path is within the allowed root
func (spf *StaticFsWithDefaultIndex) isWithinAllowedRoot(path string) bool {
    if spf.allowedRoot == "" {
        return true  // No restriction if allowedRoot not set
    }
    
    cleanPath := filepath.Clean(path)
    allowedPath := filepath.Clean(spf.allowedRoot)
    
    // Check if path starts with allowed root
    rel, err := filepath.Rel(allowedPath, cleanPath)
    if err != nil {
        return false
    }
    
    // Path should not start with ".." (outside allowed root)
    return !strings.HasPrefix(rel, "..")
}

// Open securely opens files with validation and fallback
func (spf *StaticFsWithDefaultIndex) Open(name string) (http.File, error) {
    // Validate input path
    if err := validatePath(name); err != nil {
        log.Warnf("Invalid path requested: %s, error: %v", name, err)
        return spf.openFallback()
    }
    
    // Clean the path
    cleanName := filepath.Clean(name)
    
    // Check if path is within allowed root
    if !spf.isWithinAllowedRoot(cleanName) {
        log.Warnf("Path outside allowed root: %s", cleanName)
        return spf.openFallback()
    }
    
    // Log file access attempt
    log.Debugf("Attempting to serve file: %s", cleanName)
    
    // Try to open the requested file
    f, err := spf.system.Open(cleanName)
    if err != nil {
        log.Debugf("File not found: %s, serving fallback", cleanName)
        return spf.openFallback()
    }
    
    log.Debugf("Successfully serving file: %s", cleanName)
    return f, nil
}

// openFallback securely opens the fallback page with error handling
func (spf *StaticFsWithDefaultIndex) openFallback() (http.File, error) {
    // Validate fallback path before opening
    if err := validatePath(spf.pageOn404); err != nil {
        log.Errorf("Invalid fallback page path: %v", err)
        return nil, fmt.Errorf("invalid fallback configuration")
    }
    
    // Check if fallback is within allowed root
    if !spf.isWithinAllowedRoot(spf.pageOn404) {
        log.Errorf("Fallback page outside allowed root: %s", spf.pageOn404)
        return nil, fmt.Errorf("fallback configuration error")
    }
    
    // Try to open fallback page
    f, err := spf.system.Open(spf.pageOn404)
    if err != nil {
        log.Errorf("Failed to open fallback page: %s, error: %v", spf.pageOn404, err)
        return nil, fmt.Errorf("fallback page unavailable")
    }
    
    log.Debugf("Serving fallback page: %s", spf.pageOn404)
    return f, nil
}

// SetFallbackPage securely updates the fallback page with validation
func (spf *StaticFsWithDefaultIndex) SetFallbackPage(newPageOn404 string) error {
    if err := validatePath(newPageOn404); err != nil {
        return fmt.Errorf("invalid fallback page: %v", err)
    }
    
    cleanPath := filepath.Clean(newPageOn404)
    if !spf.isWithinAllowedRoot(cleanPath) {
        return fmt.Errorf("fallback page outside allowed root")
    }
    
    spf.pageOn404 = cleanPath
    log.Infof("Fallback page updated to: %s", cleanPath)
    return nil
}

// GetStats returns usage statistics (for monitoring)
func (spf *StaticFsWithDefaultIndex) GetStats() map[string]interface{} {
    return map[string]interface{}{
        "fallback_page": spf.pageOn404,
        "allowed_root":  spf.allowedRoot,
    }
}
```

### Long-term Improvements
1. **Access Logging:** Implement comprehensive access logging for security monitoring
2. **Rate Limiting:** Add rate limiting to prevent abuse
3. **File Type Restrictions:** Restrict serving to specific file types only
4. **Content Security:** Add content-type validation and headers
5. **Performance Monitoring:** Monitor file access patterns for security anomalies

## Edge Cases Identified

1. **Empty File Names:** Handling of empty or whitespace-only file names
2. **Unicode File Names:** File names with unicode characters
3. **Long File Paths:** Very long file path handling
4. **Special Characters:** File names with special characters
5. **Case Sensitivity:** File name case sensitivity across different file systems
6. **Symbolic Links:** Handling of symbolic links in file paths
7. **Device Files:** Attempting to access device files
8. **Large Files:** Memory usage with very large files
9. **Concurrent Access:** Multiple simultaneous access to same files
10. **File System Errors:** Various file system error conditions

## Security Best Practices Violations

1. **No path validation or sanitization**
2. **No directory traversal protection**
3. **Missing error handling for fallback failures**
4. **No access logging or monitoring**
5. **No file type restrictions**
6. **No rate limiting or abuse protection**

## Critical Issues Summary

1. **Path Traversal Vulnerability:** Complete lack of path validation allows directory traversal
2. **Fallback Path Injection:** Unvalidated fallback path could serve arbitrary files
3. **Missing Error Handling:** No error handling when fallback also fails
4. **Information Disclosure:** Error patterns may reveal file system information

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** HIGH - Critical path traversal vulnerability allowing arbitrary file access