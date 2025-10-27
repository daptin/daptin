# streaming_import_parsers.go

**File:** server/actions/streaming_import_parsers.go

## Code Summary

This file implements streaming import parsers for multiple formats (JSON, CSV, XLSX) that can handle large datasets by processing them in batches.

### Interface: StreamingImportParser (lines 26-42)
**Purpose:** Defines contract for import parsers
**Methods:**
- `Initialize()`: Prepares parser with file content
- `GetTableNames()`: Returns table names found in file
- `GetColumnsForTable()`: Returns column names for specific table
- `ParseRows()`: Processes rows in batches with handler function
- `GetFormat()`: Returns parser format

### StreamingJSONParser (lines 44-140)
**Purpose:** Parses JSON files for data import
**Key Methods:**

**Initialize() (lines 51-87):**
- Line 55: **DANGEROUS:** Uses global `json.Unmarshal()` without validation
- Lines 67-70: Type assertion `tableData.([]interface{})` can panic
- Lines 75-78: Type assertion `rowData.(map[string]interface{})` can panic
- Loads entire JSON file into memory

**GetColumnsForTable() (lines 98-112):**
- Lines 99-102: No validation of table existence
- Line 105: Assumes first row exists without validation

**ParseRows() (lines 115-135):**
- Lines 116-119: Basic table existence check
- Lines 122-132: Processes data in batches (good for memory)

**Security Issues:**
- **JSON bomb:** No protection against large JSON files consuming excessive memory
- **Type confusion:** Multiple unsafe type assertions can panic
- **No input validation:** JSON structure not validated before processing

### StreamingCSVParser (lines 142-270)
**Purpose:** Parses CSV files for data import
**Key Methods:**

**Initialize() (lines 153-204):**
- Line 159: Reads entire CSV into memory: `reader.ReadAll()`
- Lines 177-187: **LOGIC BUG:** Table name extraction broken (line 185 always uses original tableName)
- Lines 188-196: Inconsistent header handling

**ParseRows() (lines 223-265):**
- Lines 250-254: **INDEX OUT OF BOUNDS:** No validation that k < len(row) before accessing headers
- No validation of column count consistency

**Security Issues:**
- **CSV injection:** No protection against CSV formula injection
- **Memory exhaustion:** Loads entire CSV into memory
- **Data inconsistency:** Inconsistent row lengths not properly handled

### StreamingXLSXParser (lines 272-398)
**Purpose:** Parses Excel files for data import
**Key Methods:**

**Initialize() (lines 280-323):**
- Line 285: **DANGEROUS:** Uses `xlsx.OpenBinary()` without size limits
- Lines 303-315: **PERFORMANCE ISSUE:** Nested loops process entire sheet into memory
- Lines 304-307: Skips validation of row/sheet corruption

**ParseRows() (lines 351-393):**
- Lines 378-382: Better bounds checking than CSV parser
- Line 379: Validates header is not empty

**Security Issues:**
- **Excel bomb:** No protection against large Excel files
- **Memory exhaustion:** Loads entire workbook into memory
- **Zip bomb:** Excel files are ZIP archives, vulnerable to zip bomb attacks

### Helper Functions (lines 400-453)

**DetectFileFormat() (lines 406-439):**
- Lines 409-415: Basic file extension detection
- Lines 420-434: Simple content-based detection
- **SECURITY ISSUE:** File type detection based on content can be spoofed

**CreateStreamingImportParser() (lines 442-453):**
- Simple factory function for creating parsers

## Critical Issues Found

### 🚨 Critical Security Vulnerabilities
1. **JSON bomb attacks:** No limits on JSON file size or structure depth
2. **Excel zip bomb:** XLSX files can contain zip bombs causing resource exhaustion
3. **CSV injection:** No protection against CSV formula injection attacks
4. **Type assertion panics:** Multiple unsafe type assertions can crash application
5. **Memory exhaustion:** All parsers load entire files into memory

### ⚠️ Runtime Safety Issues
6. **Line 67:** Type assertion `tableData.([]interface{})` can panic if not array
7. **Line 75:** Type assertion `rowData.(map[string]interface{})` can panic if not object
8. **Line 251:** Index out of bounds access when k >= len(row) in CSV parser
9. **Line 304-307:** XLSX parser skips validation of corrupted rows/sheets
10. **Missing error handling:** Many operations lack proper error handling

### 🔐 Input Validation Issues
11. **No file size limits:** Parsers accept arbitrarily large files
12. **No structure validation:** JSON/CSV structure not validated before processing
13. **No content sanitization:** File contents not sanitized before parsing
14. **File type spoofing:** Content-based detection can be spoofed
15. **No schema validation:** No validation that imported data matches expected schema

### 📂 Memory Management Issues
16. **Memory exhaustion:** All formats load entire files into memory
17. **No streaming:** Despite "streaming" name, parsers are not truly streaming
18. **Resource leaks:** XLSX files may leave file handles open
19. **Unbounded growth:** No limits on number of tables/columns/rows
20. **No garbage collection hints:** Large data structures not explicitly freed

### 🏗️ Logic and Design Issues
21. **Line 185:** CSV table name extraction always uses original tableName (bug)
22. **Inconsistent behavior:** Different parsers handle empty data differently
23. **No batch size validation:** batchSize parameter not validated
24. **Poor error messages:** Generic error messages don't help with debugging
25. **No progress tracking:** No way to track parsing progress

### 🌐 Format-Specific Issues
26. **JSON structure assumptions:** Assumes specific JSON structure without validation
27. **CSV delimiter:** Hard-coded comma delimiter, doesn't support other delimiters
28. **Excel compatibility:** May not handle all Excel features correctly
29. **Encoding issues:** No explicit handling of character encoding
30. **Date/time parsing:** No special handling for date/time formats

### ⚙️ Operational Issues
31. **No cancellation:** No way to cancel long-running parsing operations
32. **No timeout:** Parsing operations can run indefinitely
33. **No metrics:** No monitoring of parsing performance or failures
34. **Error recovery:** No mechanism to recover from partial parsing failures
35. **No resumability:** Cannot resume interrupted parsing operations

### 🔒 Data Security Issues
36. **Information disclosure:** Error messages may reveal file structure
37. **Data injection:** Imported data not validated for malicious content
38. **No access control:** No validation of user permissions to import data
39. **Audit trail:** Import operations not logged for security monitoring
40. **Data integrity:** No validation of data consistency or integrity

### 💾 Performance Issues
41. **Synchronous processing:** All parsing is synchronous and blocking
42. **Memory inefficient:** Duplicates data during parsing process
43. **CPU intensive:** Large file parsing can consume significant CPU
44. **No caching:** Parsed data not cached for repeated access
45. **No compression:** Parsed data structures not compressed in memory

### 🔧 API Design Issues
46. **Handler function signature:** Handler cannot return parsed data, only errors
47. **Batch size control:** Caller must determine appropriate batch size
48. **Table iteration:** No easy way to iterate over all tables
49. **Column metadata:** No way to get column types or metadata
50. **Error context:** Errors lack sufficient context about failure location