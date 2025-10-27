# utils.go

**File:** server/actions/utils.go

## Code Summary

This file contains utility functions used by the actions package, primarily for string manipulation and Excel data processing.

### Function: EndsWithCheck() (lines 11-24)
**Purpose:** Checks if a string ends with a specific suffix
**Inputs:** 
- `str string` - Source string to check
- `endsWith string` - Suffix to check for

**Process:**
- Lines 12-14: Returns false if suffix longer than string
- Lines 16-18: Returns false if equal length but different content
- Line 20: Extracts suffix from string: `str[len(str)-len(endsWith):]`
- Lines 21-22: Compares suffix with expected value

**Edge Cases:**
- **No bounds checking:** Assumes valid string inputs
- **Empty string handling:** Does not explicitly handle empty strings
- **Unicode issues:** May not handle Unicode characters correctly

### Function: BeginsWithCheck() (lines 26-40)
**Purpose:** Checks if a string begins with a specific prefix
**Inputs:**
- `str string` - Source string to check  
- `beginsWith string` - Prefix to check for

**Process:**
- Lines 27-29: Returns false if prefix longer than string
- Lines 31-33: Returns false if equal length but different content  
- Line 35: Extracts prefix from string: `str[:len(beginsWith)]`
- Lines 36-38: Compares prefix with expected value

**Edge Cases:**
- **No bounds checking:** Assumes valid string inputs
- **Empty string handling:** Does not explicitly handle empty strings
- **Unicode issues:** May not handle Unicode characters correctly

### Function: SmallSnakeCaseText() (lines 42-45)
**Purpose:** Converts string to lowercase snake_case format
**Inputs:** 
- `str string` - String to transform

**Process:**
- Line 43: Uses external `conform.TransformString()` with "lower,snake" transformation
- Line 44: Returns transformed string

**Dependencies:**
- **External library:** Relies on `github.com/artpar/conform` package
- **Transformation rules:** Depends on conform package implementation

### Function: GetDataArray() (lines 47-104)
**Purpose:** Extracts data from Excel sheet into array of maps
**Inputs:**
- `sheet *xlsx.Sheet` - Excel sheet to process

**Process:**
**1. Sheet Validation (lines 51-66):**
- Lines 51-52: Gets sheet dimensions
- Lines 57-60: Returns error if no columns
- Lines 62-65: Returns error if less than 2 rows

**2. Header Processing (lines 68-80):**
- Line 70: Gets first row as headers: `sheet.Row(0)`
- Lines 72-80: **DANGEROUS LOOP:** Processes headers without bounds checking
- Line 73: **POTENTIAL PANIC:** `headerRow.GetCell(i).Value` can panic if cell doesn't exist
- Lines 74-77: Breaks on empty column name
- Line 79: Converts header to snake_case

**3. Data Processing (lines 82-100):**
- Lines 82-100: **DANGEROUS LOOP:** Processes data rows
- Line 87: **POTENTIAL PANIC:** `sheet.Row(i)` can return nil
- Line 90: **POTENTIAL PANIC:** `currentRow.GetCell(j).Value` can panic
- Line 95: **INDEX OUT OF BOUNDS:** `properColumnNames[j]` accessed without bounds check

**Output:**
- Returns data array, column names, and error

**Edge Cases:**
- **Line 73:** Can panic if headerRow is nil or cell doesn't exist
- **Line 87:** Can panic if sheet.Row(i) returns nil
- **Line 90:** Can panic if currentRow is nil or cell doesn't exist
- **Line 95:** Index out of bounds if j >= len(properColumnNames)
- **Memory usage:** Loads entire sheet into memory
- **No data validation:** Cell values not validated before processing

### Function: EndsWith() (lines 106-120)
**Purpose:** Checks if string ends with suffix and returns prefix
**Inputs:**
- `str string` - Source string
- `endsWith string` - Suffix to check for

**Process:**
- Lines 107-109: Returns empty string and false if suffix longer
- Lines 111-113: Returns empty string and false if equal length but different
- Lines 115-118: Extracts prefix and suffix, compares suffix

**Edge Cases:**
- **No bounds checking:** Assumes valid string inputs
- **Duplicate logic:** Similar to EndsWithCheck() but returns prefix

## Critical Issues Found

### 🚨 Critical Runtime Safety Issues
1. **Line 73:** `headerRow.GetCell(i).Value` can panic if headerRow is nil or cell doesn't exist
2. **Line 87:** `sheet.Row(i)` can return nil causing panic on subsequent access
3. **Line 90:** `currentRow.GetCell(j).Value` can panic if currentRow is nil
4. **Line 95:** Index out of bounds access `properColumnNames[j]` without bounds checking
5. **No nil pointer checks:** Multiple locations assume non-nil pointers without validation

### ⚠️ Logic and Bounds Issues
6. **Loop bounds:** Loops use sheet.MaxRow/MaxCol without validating actual data bounds
7. **Empty cell handling:** Inconsistent handling of empty cells across functions
8. **Column count mismatch:** No validation that data rows have same column count as headers
9. **Memory exhaustion:** GetDataArray() loads entire sheet into memory without limits
10. **Resource usage:** No limits on sheet size or memory consumption

### 🔐 Input Validation Issues
11. **No parameter validation:** Functions don't validate input parameters for nil or invalid values
12. **No size limits:** No limits on string lengths or sheet dimensions
13. **No sanitization:** Cell values not sanitized before processing
14. **Unicode handling:** String functions may not handle Unicode correctly
15. **No encoding validation:** No validation of character encoding in cell values

### 🏗️ Design Issues
16. **Duplicate logic:** EndsWithCheck() and EndsWith() have similar logic
17. **External dependency:** SmallSnakeCaseText() depends on external conform package
18. **Inconsistent error handling:** Some functions return errors, others panic
19. **Mixed concerns:** Utils file mixes string utilities with Excel-specific functionality
20. **No documentation:** Functions lack proper documentation

### 📂 Data Processing Issues
21. **Type assumptions:** Assumes all cell values are strings
22. **No data type detection:** No attempt to detect or preserve data types
23. **Silent data loss:** Empty cells silently skipped without indication
24. **No metadata preservation:** Cell formatting, formulas, comments lost
25. **Row inconsistency:** Rows with different column counts handled inconsistently

### ⚙️ Performance Issues
26. **Inefficient string operations:** String slicing operations not optimized
27. **Memory duplication:** Data copied multiple times during processing
28. **No streaming:** Entire sheet processed in memory at once
29. **No caching:** Repeated cell access without caching results
30. **No parallel processing:** Sequential processing of large sheets

### 🔧 API Design Issues
31. **Inconsistent naming:** Function names don't follow Go conventions
32. **Mixed return types:** Some functions return tuples, others single values
33. **Error handling inconsistency:** Different error handling patterns across functions
34. **No context support:** Functions don't support cancellation or timeouts
35. **Limited reusability:** Excel-specific functions mixed with general utilities

### 💾 Memory Management Issues
36. **No memory limits:** No protection against large sheet memory consumption
37. **Potential memory leaks:** Large data structures not explicitly freed
38. **No garbage collection hints:** No hints to help garbage collector
39. **Unbounded growth:** Data arrays can grow without limits
40. **No streaming alternatives:** No streaming options for large datasets