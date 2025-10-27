# Security Analysis: server/csvmap/csvmap.go

**File:** `server/csvmap/csvmap.go`  
**Type:** CSV parsing utility with column mapping  
**Lines of Code:** 64  

## Overview
This file provides a wrapper around Go's standard CSV reader that maps CSV columns to named fields, creating map-based record access instead of position-based access. It includes functionality for reading headers, individual records, and entire CSV files.

## Key Components

### Reader struct
**Lines:** 10-13  
**Purpose:** Wraps csv.Reader with column name mapping functionality  

### NewReader(r io.Reader) *Reader
**Lines:** 16-20  
**Purpose:** Creates new CSV reader with column mapping capability  

### Read() (record map[string]string, err error)
**Lines:** 31-44  
**Purpose:** Reads single CSV record and maps to column names  

### ReadAll() (records []map[string]string, err error)
**Lines:** 47-56  
**Purpose:** Reads entire CSV file and returns array of mapped records  

## Security Analysis

### 1. CSV Injection Vulnerability - CRITICAL
**Severity:** HIGH  
**Lines:** 31-44  
**Issue:** No validation or sanitization of CSV field content.

**Risk:**
- CSV injection attacks through formula injection (=, +, -, @)
- When exported to Excel/Calc, formulas could execute arbitrary commands
- Potential for data exfiltration or system compromise

**Example Attack:**
```csv
name,command
user,=cmd|'/c calc'!A0
```

**Impact:** Code execution when CSV is opened in spreadsheet applications.

### 2. Resource Exhaustion Vulnerabilities
**Severity:** HIGH  
**Lines:** 47-56  
**Issue:** No limits on memory consumption during CSV processing.

```go
func (r *Reader) ReadAll() (records []map[string]string, err error) {
    var record map[string]string
    for record, err = r.Read(); err == nil; record, err = r.Read() {
        records = append(records, record)  // Unbounded memory growth
    }
    // ...
}
```

**Risk:**
- Memory exhaustion attacks via large CSV files
- DoS attacks through resource consumption
- No protection against maliciously crafted large CSV files

### 3. Column Name Injection
**Severity:** MEDIUM  
**Lines:** 37-40  
**Issue:** Column names not validated or sanitized.

```go
column := r.Columns[index]
if _, exists := record[column]; exists {
    return nil, fmt.Errorf("Multiple indices with the same name '%s'", column)
}
record[column] = rawRecord[index]
```

**Risk:**
- Malicious column names could cause issues in downstream processing
- Special characters in column names could break serialization
- Very long column names could cause memory issues

### 4. Error Information Disclosure
**Severity:** LOW  
**Lines:** 39  
**Issue:** Error messages include user-controlled column names.

```go
return nil, fmt.Errorf("Multiple indices with the same name '%s'", column)
```

**Risk:**
- Potential log injection if column names contain special characters
- Information disclosure of column structure

### 5. Input Validation Gaps
**Severity:** MEDIUM  
**Lines:** Throughout  
**Issue:** No validation of CSV structure or content.

**Missing Validations:**
- No column count limits
- No field length limits  
- No character encoding validation
- No header validation

### 6. Duplicate Column Handling Vulnerability
**Severity:** MEDIUM  
**Lines:** 38-40  
**Issue:** Duplicate column detection happens during read, not header parsing.

**Risk:**
- Duplicate columns only detected when reading records
- Potential for confusing behavior with duplicate headers
- Could mask data corruption issues

## Potential Attack Vectors

### CSV Injection Attacks
1. **Formula Injection:** Insert formulas starting with =, +, -, @ to execute commands
2. **DDE Injection:** Use Dynamic Data Exchange to execute system commands
3. **Hyperlink Injection:** Insert malicious hyperlinks in CSV fields

### Resource Exhaustion Attacks
1. **Large File Attacks:** Submit extremely large CSV files to exhaust memory
2. **Wide CSV Attacks:** Submit CSV with thousands of columns
3. **Long Field Attacks:** Submit CSV with extremely long field values

### Data Structure Attacks
1. **Column Name Pollution:** Use special characters or extremely long column names
2. **Unicode Attacks:** Use Unicode characters to break processing
3. **Encoding Attacks:** Mix different character encodings

## Recommendations

### Immediate Actions
1. **Add CSV Injection Protection:** Sanitize field values starting with dangerous characters
2. **Implement Resource Limits:** Add limits for file size, column count, field length
3. **Validate Column Names:** Sanitize and validate column names
4. **Improve Error Handling:** Avoid exposing user data in error messages

### Example Security Improvements

```go
const (
    MaxCSVSize = 10 * 1024 * 1024  // 10MB
    MaxColumns = 1000
    MaxFieldLength = 10 * 1024     // 10KB
    MaxColumnNameLength = 100
)

// SanitizeCSVField removes dangerous CSV injection patterns
func SanitizeCSVField(field string) string {
    if len(field) == 0 {
        return field
    }
    
    // Remove dangerous formula prefixes
    dangerous := []string{"=", "+", "-", "@", "\t", "\r"}
    for _, prefix := range dangerous {
        if strings.HasPrefix(field, prefix) {
            return "'" + field  // Prefix with quote to neutralize
        }
    }
    
    return field
}

// ValidateColumnName ensures column names are safe
func ValidateColumnName(name string) error {
    if len(name) > MaxColumnNameLength {
        return fmt.Errorf("column name too long: %d characters", len(name))
    }
    
    if strings.ContainsAny(name, "\n\r\t") {
        return fmt.Errorf("column name contains invalid characters")
    }
    
    return nil
}

// Enhanced Read method with security checks
func (r *Reader) Read() (record map[string]string, err error) {
    var rawRecord []string
    rawRecord, err = r.Reader.Read()
    if err != nil {
        return nil, err
    }
    
    // Check field count limits
    if len(rawRecord) > MaxColumns {
        return nil, fmt.Errorf("too many columns: %d", len(rawRecord))
    }
    
    length := min(len(rawRecord), len(r.Columns))
    record = make(map[string]string)
    
    for index := 0; index < length; index++ {
        column := r.Columns[index]
        
        // Validate column name
        if err := ValidateColumnName(column); err != nil {
            return nil, fmt.Errorf("invalid column name at index %d: %v", index, err)
        }
        
        if _, exists := record[column]; exists {
            return nil, fmt.Errorf("duplicate column detected at index %d", index)
        }
        
        // Check field length
        if len(rawRecord[index]) > MaxFieldLength {
            return nil, fmt.Errorf("field too long at column %d", index)
        }
        
        // Sanitize field value
        record[column] = SanitizeCSVField(rawRecord[index])
    }
    
    return record, nil
}
```

### Long-term Improvements
1. **Streaming Processing:** Implement streaming for large files
2. **Configuration:** Make limits configurable
3. **Audit Logging:** Log suspicious CSV processing attempts
4. **Content Validation:** Add schema validation for expected CSV structure

## Edge Cases Identified

1. **Empty CSV Files:** Files with no content or only headers
2. **Mismatched Column Counts:** Rows with different numbers of columns
3. **Unicode Content:** CSV files with Unicode characters
4. **Large Fields:** Individual fields exceeding memory limits
5. **Special Characters:** Column names or values with control characters
6. **Encoding Issues:** Mixed character encodings in single file
7. **Quote Handling:** Complex quote and escape sequences
8. **Line Ending Variations:** Different line ending formats (CRLF, LF)

## Security Best Practices Violations

1. **No input sanitization**
2. **No resource limits**
3. **No injection protection**
4. **Information disclosure in errors**
5. **No validation framework**

## Files Requiring Further Review

1. **Callers of csvmap functions** - Check how CSV data is used downstream
2. **CSV export functionality** - Verify output sanitization
3. **File upload handlers** - Check CSV file validation
4. **Database import processes** - Verify CSV data validation before DB insertion

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - CSV injection vulnerability and resource exhaustion require immediate attention