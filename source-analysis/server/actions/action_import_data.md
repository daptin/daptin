# action_import_data.go

**File:** server/actions/action_import_data.go

## Code Summary

### Type: importDataPerformer (lines 18-21)
**Fields:**
- `cmsConfig *resource.CmsConfig` - System configuration (unused in implementation)
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 24-26)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"__data_import"`

### Function: DoAction() (lines 29-220)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with import configuration
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Parameter Initialization (lines 30-47):**
- Lines 34-35: Gets optional table_name parameter
- Lines 37-47: Processes user information:
  - Line 40: Type assertion: `user.(map[string]interface{})`
  - Line 41: Converts user reference ID: `daptinid.InterfaceToDIR(userMap["reference_id"])`
  - Line 43: Gets user ID: `d.cruds[resource.USER_ACCOUNT_TABLE_NAME].GetReferenceIdToId(...)`

**2. Import Options Processing (lines 49-62):**
- Lines 50-55: Gets truncate_before_insert option with type assertion: `val.(bool)`
- Lines 57-62: Gets batch_size option with validation: `val.(int)` and `intVal > 0`

**3. File Validation (lines 64-70):**
- Line 65: Type assertion: `inFields["dump_file"].([]interface{})`
- Lines 66-70: Returns error if no files provided

**4. Processing Statistics Setup (lines 72-76):**
- Initializes counters for tracking import progress

**5. File Processing Loop (lines 78-206):**
- **For each file (lines 78-206):**
  - **File Validation (lines 79-95):**
    - Line 79: Type assertion: `fileInterface.(map[string]interface{})`
    - Line 85: Type assertion: `file["name"].(string)`
    - Line 91: Type assertion: `file["file"].(string)`
  
  - **Base64 Decoding (lines 97-112):**
    - Line 98: Splits content on comma: `strings.Split(fileContentsBase64, ",")`
    - Lines 102-106: Decodes base64 content: `base64.StdEncoding.DecodeString(...)`
  
  - **Parser Creation (lines 117-123):**
    - Line 117: Detects file format: `DetectFileFormat(fileBytes, fileName)` (**UNDEFINED** - function not imported)
    - Line 118: Creates parser: `CreateStreamingImportParser(format)` (**UNDEFINED** - function not imported)
  
  - **Parser Initialization (lines 125-132):**
    - Line 125: Type assertion: `tableName.(string)`
    - Line 127: Initializes parser: `parser.Initialize(fileBytes, tableNameString)`
  
  - **Table Processing (lines 134-205):**
    - Lines 135-140: Gets table names: `parser.GetTableNames()`
    - Lines 143-147: Filters to specific table if requested
    - **For each table (lines 150-204):**
      - Lines 152-155: Skips inaccessible tables
      - **Table Truncation (lines 158-172):** If truncate_before_insert enabled:
        - Line 165: Truncates table: `instance.TruncateTable(currentTable, false, transaction)`
      - **Row Processing (lines 178-196):**
        - Line 178: Parses rows in batches: `parser.ParseRows(currentTable, batchSize, callback)`
        - Lines 179-195: For each row:
          - Lines 181-183: Adds user reference if present
          - Line 185: Inserts row: `d.cruds[currentTable].DirectInsert(currentTable, row, transaction)`
          - Lines 186-193: Tracks success/failure counts

**6. Response Generation (lines 208-217):**
- Lines 209-214: Creates summary with import statistics
- Lines 216-217: Creates client notification response

**7. Return (line 219):**
- Returns nil responder, responses array, and errors array

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, []error)` with import results

**Edge Cases:**
- **Line 40:** Type assertion `user.(map[string]interface{})` can panic if user is not map type
- **Line 65:** Type assertion `inFields["dump_file"].([]interface{})` can panic if dump_file is not array type
- **Line 79:** Type assertion `fileInterface.(map[string]interface{})` can panic if file is not map type
- **Line 85:** Type assertion `file["name"].(string)` can panic if name is not string type
- **Line 91:** Type assertion `file["file"].(string)` can panic if file content is not string type
- **Line 98:** `strings.Split(fileContentsBase64, ",")` assumes specific base64 format
- **Line 117:** `DetectFileFormat()` function not imported - **COMPILATION ERROR**
- **Line 118:** `CreateStreamingImportParser()` function not imported - **COMPILATION ERROR**
- **Line 125:** Type assertion `tableName.(string)` can panic if tableName is not string
- **Line 144:** Duplicate type assertion `tableName.(string)` already done on line 125
- **Transaction scope:** Long-running import operations within database transaction
- **Memory usage:** Entire file contents loaded into memory for processing
- **Error continuation:** Individual row insertion errors don't stop overall import
- **No input validation:** File content not validated before processing
- **Destructive operation:** Table truncation with minimal safeguards

### Function: NewImportDataPerformer() (lines 223-230)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Handler Creation (lines 224-227):**
- Creates performer with configuration and cruds

**2. Return (line 229):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Unused field:** cmsConfig stored but never used in DoAction

**Side Effects:**
- **Data import:** Imports data from various file formats into database tables
- **Table truncation:** Optionally truncates tables before import (destructive operation)
- **Batch processing:** Processes data in configurable batch sizes
- **User context:** Associates imported data with specific users when provided
- **Transaction usage:** Performs multiple database operations within provided transaction
- **Memory consumption:** Loads entire import files into memory during processing