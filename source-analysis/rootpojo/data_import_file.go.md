# Security Analysis: server/rootpojo/data_import_file.go

**File:** `server/rootpojo/data_import_file.go`  
**Type:** Data structure for file import operations  
**Lines of Code:** 14  

## Overview
This file defines the DataFileImport struct and its String() method, which represents file import operations in the Daptin system. It contains metadata about files to be imported including file path, entity mapping, and file type information.

## Key Components

### DataFileImport struct
**Lines:** 9-13  
**Purpose:** Data structure representing a file import operation with path, entity, and type information  

### String method
**Lines:** 5-7  
**Purpose:** Provides string representation of DataFileImport for logging and display  

## Security Analysis

### 1. MEDIUM: Information Disclosure in String Method - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 5-7  
**Issue:** String method exposes potentially sensitive file paths in logs and error messages.

```go
func (s DataFileImport) String() string {
    return fmt.Sprintf("[%v][%v]", s.FileType, s.FilePath)  // FilePath exposed
}
```

**Risk:**
- **File path disclosure** in application logs
- **Directory structure leakage** through error messages
- **Sensitive path information** exposed in debugging output
- **No sanitization** of file paths before display
- **Potential enumeration** of file system structure

**Impact:** Information disclosure that could aid attackers in understanding system file structure and locating sensitive files.

### 2. MEDIUM: Missing Input Validation - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 9-13  
**Issue:** No validation methods for input data fields.

```go
type DataFileImport struct {
    FilePath string  // No validation
    Entity   string  // No validation  
    FileType string  // No validation
}
```

**Risk:**
- **Path traversal** through unchecked FilePath values
- **Invalid entity names** that could cause processing errors
- **Malformed file types** leading to incorrect processing
- **No length limits** on string fields
- **No format validation** for any fields

### 3. LOW: Generic String Fields - LOW RISK
**Severity:** LOW  
**Lines:** 10-12  
**Issue:** All fields are generic strings without type safety or constraints.

**Risk:**
- **Type confusion** between different string contexts
- **No enumeration** for valid FileType values
- **No constraints** on Entity field format
- **Runtime errors** from unexpected field values

## Potential Attack Vectors

### Information Disclosure Attacks
1. **Log Analysis:** Extract file paths and system structure from application logs
2. **Error Message Enumeration:** Use error messages to map file system layout
3. **Debug Output Analysis:** Analyze debug information for sensitive paths

### Path Traversal Attacks
1. **Directory Traversal:** Use "../" sequences in FilePath to access unauthorized files
2. **Absolute Path Exploitation:** Use absolute paths to access system files
3. **Symbolic Link Exploitation:** Use symbolic links to bypass path restrictions

### Data Injection Attacks
1. **Entity Name Injection:** Inject malicious entity names to affect processing
2. **File Type Confusion:** Use unexpected file types to bypass processing logic
3. **Field Length Exploitation:** Use extremely long field values for DoS

## Recommendations

### Immediate Actions
1. **Sanitize String Output:** Remove or redact sensitive path information from String() method
2. **Add Input Validation:** Implement validation for all fields
3. **Add Path Security:** Validate and sanitize file paths
4. **Add Field Constraints:** Define valid values and length limits

### Enhanced Security Implementation

```go
package rootpojo

import (
    "fmt"
    "path/filepath"
    "regexp"
    "strings"
)

const (
    MaxFilePathLength = 4096
    MaxEntityNameLength = 255
    MaxFileTypeLength = 50
)

var (
    validEntityNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
    validFileTypePattern = regexp.MustCompile(`^[a-zA-Z0-9]+$`)
    
    // Define allowed file types
    AllowedFileTypes = map[string]bool{
        "csv":  true,
        "json": true,
        "xml":  true,
        "xlsx": true,
        "xls":  true,
        "txt":  true,
    }
)

// DataFileImport represents a file import operation with validation
type DataFileImport struct {
    FilePath string `json:"file_path" validate:"required,filepath"`
    Entity   string `json:"entity" validate:"required,entity_name"`
    FileType string `json:"file_type" validate:"required,file_type"`
}

// Validate validates all fields of DataFileImport
func (s *DataFileImport) Validate() error {
    if err := s.validateFilePath(); err != nil {
        return fmt.Errorf("invalid file path: %v", err)
    }
    
    if err := s.validateEntity(); err != nil {
        return fmt.Errorf("invalid entity: %v", err)
    }
    
    if err := s.validateFileType(); err != nil {
        return fmt.Errorf("invalid file type: %v", err)
    }
    
    return nil
}

// validateFilePath validates and sanitizes the file path
func (s *DataFileImport) validateFilePath() error {
    if len(s.FilePath) == 0 {
        return fmt.Errorf("file path cannot be empty")
    }
    
    if len(s.FilePath) > MaxFilePathLength {
        return fmt.Errorf("file path too long: %d characters", len(s.FilePath))
    }
    
    // Check for path traversal attempts
    cleanPath := filepath.Clean(s.FilePath)
    if strings.Contains(cleanPath, "..") {
        return fmt.Errorf("path traversal detected in file path")
    }
    
    // Ensure path is relative (no absolute paths)
    if filepath.IsAbs(s.FilePath) {
        return fmt.Errorf("absolute paths not allowed")
    }
    
    // Check for null bytes and other dangerous characters
    if strings.ContainsAny(s.FilePath, "\x00\r\n") {
        return fmt.Errorf("invalid characters in file path")
    }
    
    return nil
}

// validateEntity validates the entity name
func (s *DataFileImport) validateEntity() error {
    if len(s.Entity) == 0 {
        return fmt.Errorf("entity name cannot be empty")
    }
    
    if len(s.Entity) > MaxEntityNameLength {
        return fmt.Errorf("entity name too long: %d characters", len(s.Entity))
    }
    
    if !validEntityNamePattern.MatchString(s.Entity) {
        return fmt.Errorf("invalid entity name format")
    }
    
    return nil
}

// validateFileType validates the file type
func (s *DataFileImport) validateFileType() error {
    if len(s.FileType) == 0 {
        return fmt.Errorf("file type cannot be empty")
    }
    
    if len(s.FileType) > MaxFileTypeLength {
        return fmt.Errorf("file type too long: %d characters", len(s.FileType))
    }
    
    if !validFileTypePattern.MatchString(s.FileType) {
        return fmt.Errorf("invalid file type format")
    }
    
    // Check against allowed file types
    if !AllowedFileTypes[strings.ToLower(s.FileType)] {
        return fmt.Errorf("file type not allowed: %s", s.FileType)
    }
    
    return nil
}

// SanitizedString provides a string representation without sensitive information
func (s DataFileImport) SanitizedString() string {
    // Only show filename, not full path
    filename := filepath.Base(s.FilePath)
    if filename == "." || filename == "/" {
        filename = "[HIDDEN]"
    }
    
    return fmt.Sprintf("[%s][%s]", s.FileType, filename)
}

// String method with path sanitization
func (s DataFileImport) String() string {
    return s.SanitizedString()
}

// SecureString provides minimal information for logging
func (s DataFileImport) SecureString() string {
    return fmt.Sprintf("DataFileImport{Entity: %s, Type: %s}", s.Entity, s.FileType)
}

// GetSanitizedFilePath returns a sanitized version of the file path for logging
func (s *DataFileImport) GetSanitizedFilePath() string {
    if len(s.FilePath) == 0 {
        return "[EMPTY]"
    }
    
    // Show only the filename for security
    return filepath.Base(s.FilePath)
}

// IsValid performs complete validation and returns true if all fields are valid
func (s *DataFileImport) IsValid() bool {
    return s.Validate() == nil
}

// SafeFilePath returns the file path with basic safety checks applied
func (s *DataFileImport) SafeFilePath() (string, error) {
    if err := s.validateFilePath(); err != nil {
        return "", err
    }
    
    // Return cleaned path
    return filepath.Clean(s.FilePath), nil
}
```

### Long-term Improvements
1. **File Type Registry:** Implement a formal file type registry with validation
2. **Path Whitelisting:** Implement allowed path patterns or directories
3. **Entity Validation:** Integrate with entity schema validation
4. **Audit Logging:** Log all file import operations securely
5. **Content Validation:** Add file content validation beyond just metadata

## Edge Cases Identified

1. **Empty Fields:** Handling of empty or missing field values
2. **Unicode Paths:** File paths with unicode characters
3. **Long Paths:** Very long file path handling
4. **Special Characters:** File paths with special characters or spaces
5. **Case Sensitivity:** File type case sensitivity handling
6. **Path Separators:** Different path separator handling across platforms
7. **Symbolic Links:** Handling of symbolic links in file paths
8. **Network Paths:** UNC paths or network file paths
9. **Reserved Names:** Operating system reserved file names
10. **Encoding Issues:** File path encoding problems

## Security Best Practices Violations

1. **No input validation for critical path fields**
2. **Information disclosure through String() method**
3. **Missing path traversal protection**
4. **No file type whitelisting**
5. **No length limits on input fields**
6. **No sanitization for logging output**

## Critical Issues Summary

1. **Information Disclosure:** File paths exposed in String() method
2. **Path Traversal Risk:** No validation of file paths for traversal attacks
3. **Input Validation Missing:** No validation for any input fields
4. **No Security Controls:** Missing basic security controls for file operations

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** MEDIUM - Information disclosure and path traversal risks