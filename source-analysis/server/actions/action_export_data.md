# action_export_data.go

**File:** server/actions/action_export_data.go

## Code Summary

### Constants: ExportFormat (lines 15-29)
**Format Types:**
- `FormatJSON` - "json" format
- `FormatCSV` - "csv" format  
- `FormatXLSX` - "xlsx" format
- `FormatPDF` - "pdf" format
- `FormatHTML` - "html" format

### Type: exportDataPerformer (lines 32-35)
**Fields:**
- `cmsConfig *resource.CmsConfig` - System configuration with table definitions
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 38-40)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"__data_export"`

### Function: DoAction() (lines 43-255)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused in implementation)
- `inFields map[string]interface{}` - Input parameters with export configuration
- `transaction *sqlx.Tx` - Database transaction for data access

**Process:**

**1. Format Processing (lines 49-53):**
- Line 49: Gets format parameter: `inFields["format"]`
- Line 50: Defaults to JSON format
- Lines 51-53: If format specified, converts to lowercase and casts: `ExportFormat(strings.ToLower(formatStr.(string)))`

**2. Table Selection (lines 56-57):**
- Line 56: Gets table_name parameter: `inFields["table_name"]`
- Line 57: Sets default filename: `"complete"`

**3. Export Options Processing (lines 60-71):**
- **Headers Option (lines 60-63):**
  - Line 60: Defaults to include headers: `true`
  - Lines 61-63: Gets include_headers parameter with type assertion: `includeHeadersVal.(bool)`
- **Page Size Option (lines 66-71):**
  - Line 66: Defaults to 1000 page size
  - Lines 67-71: Gets page_size parameter with validation

**4. Column Selection Processing (lines 74-107):**
- Line 74: Creates column mapping: `make(map[string][]string)`
- Lines 75-95: **Complex column parsing with multiple code paths:**
  - **Array format (lines 79-84):** Processes `[]interface{}` array
  - **String format (lines 85-95):** JSON unmarshals string (**UNDEFINED** - `json` not imported)
- Lines 98-106: Maps columns to tables

**5. Writer Creation (lines 110-114):**
- Line 110: Creates streaming writer: `CreateStreamingExportWriter(format)` (**UNDEFINED** - function not imported)
- Lines 111-114: Returns error if writer creation fails

**6. Table Determination (lines 117-126):**
- **Single table export (lines 118-121):**
  - Line 119: Type assertion: `tableName.(string)`
  - Lines 120-121: Sets single table and filename
- **All tables export (lines 123-125):**
  - Lines 123-125: Iterates through configured tables

**7. Writer Initialization (lines 129-134):**
- Line 130: Initializes writer: `writer.Initialize(tablesToExport, includeHeaders, selectedColumnsMap)`
- Lines 131-134: Returns error if initialization fails

**8. Table Processing Loop (lines 137-209):**
- **For each table in tablesToExport:**
  - **Access Check (lines 139-142):** Skips if table not accessible in cruds map
  - **Table Header (lines 145-149):** Writes table header via `writer.WriteTable(currentTable)`
  - **Column Determination (lines 152-181):**
    - Uses selected columns if available
    - Otherwise queries first row to determine columns: `GetAllRawObjectsWithPaginationAndTransaction(..., 1, ...)`
    - Lines 174-176: Iterates over first row keys to get column names
  - **Header Writing (lines 184-193):** Writes column headers via `writer.WriteHeaders(currentTable, columns)`
  - **Data Streaming (lines 196-208):** Streams data in batches via `GetAllRawObjectsWithPaginationAndTransaction` with callback

**9. Export Finalization (lines 212-217):**
- Line 212: Finalizes export: `writer.Finalize()`
- Lines 213-217: Handles finalization errors with fallback to empty content

**10. Content Type Mapping (lines 220-242):**
- Lines 223-242: Maps export format to MIME type and file extension

**11. Response Generation (lines 245-252):**
- Lines 245-252: Creates download response with base64 encoded content
- Line 247: Sets filename: `fmt.Sprintf("daptin_export_%v.%s", finalName, fileExtension)`

**12. Return (line 254):**
- Returns nil responder, responses array, and nil errors

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with file download response

**Edge Cases:**
- **Line 52:** Type assertion `formatStr.(string)` can panic if format is not string type
- **Line 62:** Type assertion `includeHeadersVal.(bool)` ignores conversion errors using `_` pattern
- **Line 87:** `json.Unmarshal()` called but `json` package not imported - **COMPILATION ERROR**
- **Line 99:** Type assertion `tableName.(string)` can panic if table_name is not string type
- **Line 110:** `CreateStreamingExportWriter()` called but function not defined - **COMPILATION ERROR**
- **Line 119:** Duplicate type assertion `tableName.(string)` - already done on line 99
- **Lines 158-166:** Complex pagination callback with potential for callback errors
- **Line 174:** Map iteration order not deterministic - column order varies between exports
- **Format validation:** No validation that format is supported - unsupported formats default to JSON
- **Memory usage:** Despite streaming approach, entire content loaded into memory for base64 encoding
- **Transaction scope:** Long-running operation within transaction may cause database locks
- **Error handling:** Individual table errors logged but don't fail overall export
- **No authorization:** No access control checks on table or column level
- **Column filtering:** No validation that requested columns exist in tables

### Function: NewExportDataPerformer() (lines 258-265)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Performer Creation (lines 259-262):**
- Lines 259-262: Creates performer struct with config and cruds

**2. Return (line 264):**
- Returns pointer to performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always nil

**Side Effects:**
- **Database data exposure:** Exports table data in multiple formats without authorization checks
- **Resource consumption:** Streaming approach but final content loaded into memory for encoding
- **Format flexibility:** Supports multiple export formats (JSON, CSV, XLSX, PDF, HTML)
- **Potential data leak:** All accessible table data exportable without field-level access control