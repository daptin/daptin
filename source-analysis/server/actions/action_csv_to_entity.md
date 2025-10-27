# action_csv_to_entity.go

**File:** server/actions/action_csv_to_entity.go

## Code Summary

### Type: uploadCsvFileToEntityPerformer (lines 22-26)
**Fields:**
- `responseAttrs map[string]interface{}` - Response attributes (unused in code)
- `cruds map[string]*resource.DbResource` - Database resource access map
- `cmsConfig *resource.CmsConfig` - CMS configuration (unused in code)

### Function: Name() (lines 28-30)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"__upload_csv_file_to_entity"`

### Function: DoAction() (lines 32-219)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with CSV files and entity configuration
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Input Parameter Extraction (lines 37-47):**
- Line 37: Gets files array with type assertion: `inFields["data_csv_file"].([]interface{})`
- Line 39: Gets entity name with type assertion: `inFields["entity_name"].(string)`
- Lines 40-43: Gets `create_if_not_exists` flag with type assertion and defaults to `false`
- Lines 44-47: Gets `add_missing_columns` flag with type assertion and defaults to `false`

**2. Table Setup (lines 49-51):**
- Creates new `table_info.TableInfo` with entity name

**3. Entity Validation (lines 60-68):**
- If `create_if_not_exists` is false:
  - Lines 63-66: Checks if entity exists in `d.cruds[entityName]`
  - If not found: Returns error `[]error{fmt.Errorf("no such entity: %v", entityName)}`
  - Line 67: Gets existing entity table info

**4. File Processing Loop (lines 72-184):**
- **For each file in files array:**
  - Lines 73-76: Type asserts file to `map[string]interface{}`, continues if fails
  - Line 77: Gets file name with type assertion: `file["name"].(string)` and prepends "_uploaded_"
  - Line 78: Gets base64 content with type assertion: `file["file"].(string)`
  - Line 79: Decodes base64 after splitting by comma: `base64.StdEncoding.DecodeString(strings.Split(fileContentsBase64, ",")[1])`
  - Lines 82-85: Checks decode error and returns if failed
  - Line 87: Writes file to disk: `os.WriteFile(schemaFolderDefinedByEnv+string(os.PathSeparator)+fileName, fileBytes, 0644)`
  - Lines 92-104: CSV parsing:
    - Creates CSV reader: `csvmap.NewReader(bytes.NewReader(fileBytes))`
    - Reads header: `csvReader.ReadHeader()`
    - Reads all data: `csvReader.ReadAll()`

**5. Column Analysis (lines 111-174):**
- **For each column name:**
  - Lines 113-115: Skips empty column names
  - Lines 119-125: If `add_missing_columns` and entity exists, skips non-existent columns
  - Lines 127-150: Data analysis:
    - Creates data map for unique values
    - Iterates through up to 100,000 records
    - Tracks nullable values and unique data
  - Lines 152-159: Type detection:
    - Calls `fieldtypes.DetectType(datas)`
    - Maps to column and data types using `EntityTypeToColumnTypeMap` and `entityTypeToDataTypeMap`
    - Defaults to "label"/"varchar(100)" on error
  - Lines 161-167: Index and uniqueness detection:
    - Sets `IsIndexed = true` if unique values > 10% of records
    - Sets `IsUnique = true` if unique values equal record count
  - Lines 169-173: Sets column properties (nullable, name, snake_case name)

**6. Schema Generation (lines 186-217):**
- If processing completed:
  - Lines 188-192: Builds schema object with tables (if creating) and imports
  - Line 194: Marshals to JSON: `json.Marshal(allSt)` (**UNDEFINED** - `json` not imported)
  - Lines 200-204: Writes schema file: `schema_uploaded_{entityName}_daptin.json`
  - Lines 207-211: Conditionally imports data or triggers restart
  - Line 212: Fires cleanup trigger: `trigger.Fire("clean_up_uploaded_files")`
  - Line 214: Returns `successResponses` (**UNDEFINED** variable)
- Line 216: Returns `failedResponses` (**UNDEFINED** variable)

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with undefined response variables

**Edge Cases:**
- **Line 37:** Type assertion `inFields["data_csv_file"].([]interface{})` can panic if field missing or wrong type
- **Line 39:** Type assertion `inFields["entity_name"].(string)` can panic if field missing or wrong type
- **Line 77:** Type assertion `file["name"].(string)` can panic if field missing or wrong type
- **Line 78:** Type assertion `file["file"].(string)` can panic if field missing or wrong type
- **Line 79:** `strings.Split(..., ",")[1]` can panic if split results in less than 2 elements
- **Line 87:** Uses `schemaFolderDefinedByEnv` without null check - could panic if environment variable not set
- **Line 87:** Path construction could result in double separators or invalid paths
- **Line 194:** `json.Marshal()` call but `json` package not imported - **COMPILATION ERROR**
- **Line 214:** `successResponses` variable undefined - **COMPILATION ERROR**
- **Line 216:** `failedResponses` variable undefined - **COMPILATION ERROR**
- **Undefined global variables:** `EntityTypeToColumnTypeMap`, `entityTypeToDataTypeMap`, `SmallSnakeCaseText` used but not defined in file
- **File permissions:** Creates files with 0644 permissions which may be too permissive
- **Path traversal:** No validation on file names - could contain "../" sequences
- **Memory exhaustion:** Loads entire CSV into memory without size limits
- **Infinite loop potential:** Line 131 sets count to 100000 but only decrements inside loop - could hang on large datasets

### Function: NewUploadCsvFileToEntityPerformer() (lines 221-230)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**
1. Creates `uploadCsvFileToEntityPerformer` struct with cruds and config
2. Returns pointer to the struct

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always `nil`

**Side Effects:**
- **File system operations:** Writes uploaded CSV files and schema JSON to disk
- **Database operations:** Potentially creates tables and imports data
- **Memory usage:** Loads entire CSV files into memory for processing
- **Trigger execution:** Fires cleanup trigger event