# action_export_csv_data.go

**File:** server/actions/action_export_csv_data.go

## Code Summary

### Type: exportCsvDataPerformer (lines 17-20)
**Fields:**
- `cmsConfig *resource.CmsConfig` - System configuration with table definitions
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 22-24)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"__csv_data_export"`

### Function: DoAction() (lines 26-117)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused in implementation)
- `inFields map[string]interface{}` - Input parameters, optionally containing table_name
- `transaction *sqlx.Tx` - Database transaction for data access

**Process:**

**1. Input Processing (lines 30-38):**
- Line 30: Checks for table_name parameter: `inFields["table_name"]`
- Line 32: Sets default filename prefix: `"complete"`
- Lines 36-38: If table specified, gets table name with type assertion: `tableName.(string)`

**2. Data Collection (lines 41-59):**
- **Single Table Export (lines 41-47):**
  - Line 41: Gets all objects for specified table: `d.cruds[tableNameStr].GetAllRawObjectsWithTransaction(tableNameStr, transaction)`
  - Lines 42-44: Logs errors but continues processing
  - Lines 46-47: Stores data and updates filename
- **Full Database Export (lines 50-57):**
  - Lines 50-57: Iterates through all configured tables
  - Line 51: Gets all objects for each table
  - Lines 52-54: Logs errors and continues to next table
  - Line 56: Stores data for each table

**3. CSV File Creation (lines 61-96):**
- Lines 61-63: Creates temporary file with timestamp prefix: `os.CreateTemp("", prefix)`
- Line 64: Defers file close
- **For each table in results (lines 66-96):**
  - Lines 68-70: Writes table name header if single table export
  - Line 71: Creates CSV writer: `csv.NewWriter(csvFile)`
  - Line 72: Type asserts content: `contents.([]map[string]interface{})`
  - Lines 74-76: Handles empty datasets
  - Lines 78-84: Extracts column names from first row
  - Line 86: Writes CSV header row
  - Lines 88-94: Writes data rows with format conversion: `fmt.Sprintf("%v", row[colName])`
  - Line 95: Adds newline separator

**4. File Processing (lines 98-104):**
- Line 98: Gets temporary file name
- Line 99: Reads file contents: `os.ReadFile(csvFileName)`
- Lines 100-104: Handles file read errors with user notification

**5. Response Generation (lines 106-114):**
- Lines 106-110: Creates download response attributes with base64 encoded content
- Line 108: Sets filename: `fmt.Sprintf("daptin_dump_%v.csv", finalName)`
- Lines 112-114: Creates and appends download action response

**6. Return (line 116):**
- Returns nil responder, responses array, and nil errors

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with file download response

**Edge Cases:**
- **Line 38:** Type assertion `tableName.(string)` can panic if table_name is not string type
- **Line 72:** Type assertion `contents.([]map[string]interface{})` can panic if data structure unexpected
- **Line 80:** Array access `contentArray[0]` can panic if contentArray is empty (despite line 74 check)
- **No table validation:** Requested table name not validated against configured tables
- **Temporary file cleanup:** File closed via defer but not explicitly deleted - may accumulate in temp directory
- **Memory usage:** Large datasets loaded entirely into memory before CSV conversion
- **Transaction scope:** Long-running operation within transaction may cause locks
- **Column ordering:** Column order not deterministic due to map iteration
- **Data type handling:** All values converted to string using fmt.Sprintf - may lose precision
- **Error continuation:** Database errors logged but export continues with partial data
- **File permission:** Temporary files created with default permissions

### Function: NewExportCsvDataPerformer() (lines 119-128)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Performer Creation (lines 121-124):**
- Lines 121-124: Creates performer struct with config and cruds

**2. Return (line 126):**
- Returns pointer to performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always nil

**Side Effects:**
- **Database data exposure:** Exports all or specified table data to downloadable CSV
- **Temporary file creation:** Creates files in system temp directory
- **Memory consumption:** Loads entire datasets into memory during processing
- **Potential data leak:** All table data made downloadable without field-level access control