# action_cloudstore_folder_create.go

**File:** server/actions/action_cloudstore_folder_create.go

## Code Summary

### Type: cloudStoreFolderCreateActionPerformer (lines 23-25)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 27-29)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"cloudstore.folder.create"`

### Function: DoAction() (lines 31-106)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with folder path and credentials
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Temporary Directory Setup (lines 35-42):**
- Line 35: Generates UUID v7 (ignores error with `_`)
- Line 36: Creates directory name: `"upload-" + u.String()[0:8]`
- Line 37: Creates temp directory: `os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)`
- Line 42: Checks error with `resource.CheckErr()`
- **Note:** Line 40 commented out - temp directory cleanup happens in goroutine

**2. Input Parameter Extraction (lines 43-45):**
- Line 43: Gets `path` from `inFields["path"]` with type assertion, ignores error: `atPath, _ := inFields["path"].(string)`
- Line 44: Gets `name` from `inFields["name"]` with type assertion, ignores error: `folderName, _ := inFields["name"].(string)`
- Line 45: Gets `root_path` from `inFields["root_path"]` with type assertion: `rootPath := inFields["root_path"].(string)`

**3. Path Construction (lines 47-56):**
- Lines 47-49: Ensures `atPath` ends with "/" if not empty
- Line 51: Constructs folder path: `folderPath := atPath + folderName`
- Lines 52-54: Creates args array with `rootPath`
- Line 56: Extracts store name: `storeName := strings.Split(rootPath, ":")[0]`

**4. Credential Setup (lines 58-67):**
- Gets `credential_name` from `inFields["credential_name"]`
- If credential exists and not empty:
  - Line 60: Retrieves credential: `d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)`
  - Line 61: Checks error with `resource.CheckErr()`
  - Lines 62-66: Iterates through credential data map and sets config values:
    - `config.Data().SetValue(storeName, key, fmt.Sprintf("%s", val))`

**5. Folder Creation (lines 69-96):**
- Line 69: Creates filesystem source: `cmd.NewFsSrc(args)`
- Lines 70-78: Sets up cobra command and context with filter
- Lines 80-96: Executes folder creation in goroutine:
  - Lines 81-84: Validates source filesystem
  - Line 86: Creates folder: `operations.Mkdir(ctx, fsrc, folderPath)`
  - Lines 87-90: Handles folder creation error
  - Lines 91-95: Removes temp directory: `os.RemoveAll(tempDirectoryPath)`

**6. Response (lines 98-103):**
- Creates success notification with message "Cloud storage file upload queued"

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with success notification

**Edge Cases:**
- **Line 45:** Type assertion `inFields["root_path"].(string)` can panic if field missing or wrong type
- **Line 60:** Type assertion `credentialName.(string)` can panic if wrong type
- **Lines 43-44:** Silent type assertion failures result in empty strings (could cause invalid paths)
- **Line 56:** `strings.Split(rootPath, ":")[0]` could panic if `rootPath` is empty string
- **Missing folderName:** Would create folder with empty name
- **Path traversal:** No validation on `folderPath` construction
- **Temp directory:** Created but only used for command setup, not actual folder creation
- **Success response:** Always returned even if mkdir operation fails
- **Goroutine execution:** No error propagation back to caller

### Function: NewCloudStoreFolderCreateActionPerformer() (lines 108-116)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**
1. Creates `cloudStoreFolderCreateActionPerformer` struct with `cruds` field
2. Returns pointer to the struct

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always `nil`

**Side Effects:**
- **File system operations:** Creates temporary directory (unnecessary for this operation)
- **Goroutine execution:** Folder creation runs asynchronously
- **Config modification:** Sets rclone configuration from credentials
- **Cleanup:** Removes temporary directory after operation