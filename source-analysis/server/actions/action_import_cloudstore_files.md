# action_import_cloudstore_files.go

**File:** server/actions/action_import_cloudstore_files.go

## Code Summary

### Type: importCloudStoreFilesPerformer (lines 23-25)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 28-30)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"cloud_store.files.import"`

### Function: DoAction() (lines 33-141)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with table_name
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Table Validation (lines 35-42):**
- Line 35: Type assertion: `inFields["table_name"].(string)`
- Lines 39-42: Validates table exists in cruds map, returns error if not found

**2. Cloud Store Column Detection (lines 44-64):**
- Lines 44-61: Iterates through table columns to find cloud store foreign keys:
  - Line 49: Checks if column type contains "." and is cloud_store foreign key
  - Lines 52-60: Processes default values and required columns
- Lines 62-64: Copies default values (redundant operation)

**3. File Import Loop (lines 66-136):**
- **For each cloud store column (lines 68-135):**
  - **Cache Folder Setup (lines 70-77):**
    - Line 70: Gets cache folder: `d.cruds[tableName].AssetFolderCache[tableName][colName]`
    - Lines 72-77: Sets default values (version, created_at, permission, user_account_id)
  
  - **Credential Configuration (lines 79-91):**
    - Lines 79-82: Extracts config set name from root path
    - Lines 83-91: If credentials exist, applies them to rclone config:
      - Line 84: Gets credential: `d.cruds["credential"].GetCredentialByName(...)`
      - Lines 87-89: Sets credential values: `config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))`
  
  - **RClone File System Setup (lines 93-99):**
    - Line 93: Creates file system: `cmd.NewFsDir([]string{cacheFolder.CloudStore.RootPath + "/" + colFkdata.KeyName})`
    - Lines 94-98: Sets up cobra command and logging configuration
  
  - **File Listing and Import (lines 100-135):**
    - Lines 100-135: Executes rclone operation in wrapped command:
      - Lines 108-131: Lists files using `operations.ListJSON()` with callback:
        - Lines 114-118: Creates JSON file metadata: `json.Marshal([]map[string]string{{"name": item.Name}})`
        - Lines 119-121: Generates UUID and sets reference_id
        - Line 123: Inserts file record: `d.cruds[tableName].DirectInsert(tableName, defaltValues, transaction)`
        - Lines 125-129: Tracks success/failure counts

**4. Response Generation (lines 138-140):**
- Returns success notification with import statistics

**5. Return (line 140):**
- Returns nil responder, notification response, and nil errors

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with import results

**Edge Cases:**
- **Line 35:** Type assertion `inFields["table_name"].(string)` can panic if table_name is not string
- **Line 114:** `json.Marshal()` called but `json` package not imported - **COMPILATION ERROR**
- **Line 70:** Multiple nested map access `AssetFolderCache[tableName][colName]` can panic if any level is nil
- **Line 81:** `strings.Split(cacheFolder.CloudStore.RootPath, ":")[0]` can panic if no ":" found
- **Line 120:** `u[:]` converts UUID to byte slice instead of string - likely incorrect for reference_id
- **Lines 62-64:** Redundant loop copying defaltValues to itself
- **Error handling:** Individual file import errors logged but don't stop overall import
- **Transaction scope:** Long-running operation with external file system access within database transaction
- **Credential exposure:** Credentials applied globally to rclone config without cleanup
- **File metadata:** Only stores file name, ignoring other metadata like size, modification time, etc.

### Function: NewImportCloudStoreFilesPerformer() (lines 144-152)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration (unused)
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Handler Creation (lines 146-148):**
- Creates performer with cruds map

**2. Return (line 150):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Unused parameter:** initConfig parameter completely ignored

**Side Effects:**
- **File metadata import:** Imports file listings from cloud storage into database
- **Credential configuration:** Applies cloud store credentials to global rclone configuration
- **Database population:** Creates records for each file found in cloud storage
- **External system dependency:** Relies on rclone and cloud storage connectivity
- **Transaction usage:** Performs multiple database insertions within provided transaction