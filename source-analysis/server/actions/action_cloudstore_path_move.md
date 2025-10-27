# action_cloudstore_path_move.go

**File:** server/actions/action_cloudstore_path_move.go

## Code Summary

### Type: cloudStorePathMoveActionPerformer (lines 24-26)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 28-30)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"cloudstore.path.move"`

### Function: DoAction() (lines 32-111)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with source/destination paths and credentials
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Temporary Directory Setup (lines 36-43):**
- Line 36: Generates UUID v7 (ignores error with `_`)
- Line 37: Creates directory name: `"upload-" + u.String()[0:8]`
- Line 38: Creates temp directory: `os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)`
- Line 43: Checks error with `resource.CheckErr()`
- **Note:** Line 41 commented out - temp directory cleanup happens conditionally in operation

**2. Input Parameter Extraction (lines 44-46):**
- Line 44: Gets `source` from `inFields["source"]` with type assertion, ignores error: `sourcePath, _ := inFields["source"].(string)`
- Line 45: Gets `destination` from `inFields["destination"]` with type assertion, ignores error: `destinationPath, _ := inFields["destination"].(string)`
- Line 46: Gets `root_path` from `inFields["root_path"]` with type assertion: `rootPath := inFields["root_path"].(string)`

**3. Path Normalization (lines 48-59):**
- Lines 48-50: Ensures `sourcePath` starts with "/" if not empty
- Lines 52-54: Ensures `destinationPath` starts with "/" if not empty
- Lines 56-59: Creates args array:
  - `args[0] = rootPath + sourcePath` (source)
  - `args[1] = rootPath + destinationPath` (destination)

**4. Store Name and Credential Setup (lines 62-72):**
- Line 62: Extracts store name: `storeName := strings.Split(rootPath, ":")[0]`
- Gets `credential_name` from `inFields["credential_name"]`
- If credential exists and not empty:
  - Line 65: Retrieves credential: `d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)`
  - Line 66: Checks error with `resource.CheckErr()`
  - Lines 67-71: Iterates through credential data map and sets config values

**5. Move Operation Setup (lines 74-84):**
- Line 74: Creates filesystem source: `cmd.NewFsSrc(args)` (Note: This only uses first arg, ignoring destination)
- Lines 75-83: Sets up cobra command and context with filter and config

**6. Move Execution (lines 85-101):**
- Line 85: **Re-creates** filesystem objects: `fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)`
- Lines 86-101: Executes move operation **synchronously** (no `go` keyword):
  - Lines 88-92: Conditional move logic:
    - If `srcFileName == ""`: Directory move with `sync.MoveDir(ctx, fdst, fsrc, false, true)`
    - Else: File move with `operations.MoveFile(ctx, fdst, fsrc, srcFileName, srcFileName)`
  - Lines 94-98: Error handling:
    - Logs error with `resource.InfoErr()`
    - Removes temp directory: `os.RemoveAll(tempDirectoryPath)`
    - Returns `nil` (suppresses error)
  - Line 100: Returns error on success

**7. Response (lines 103-108):**
- Creates success notification with message "Cloud storage path moved"

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with success notification

**Edge Cases:**
- **Line 46:** Type assertion `inFields["root_path"].(string)` can panic if field missing or wrong type
- **Line 65:** Type assertion `credentialName.(string)` can panic if wrong type
- **Lines 44-45:** Silent type assertion failures result in empty strings (could cause invalid paths)
- **Line 62:** `strings.Split(rootPath, ":")[0]` could panic if `rootPath` is empty string
- **Path construction:** No validation on constructed paths - could result in invalid or dangerous paths
- **Empty paths:** No validation prevents moving to/from empty paths
- **Same source/destination:** No check prevents moving path to itself
- **Line 74:** First `cmd.NewFsSrc(args)` call is redundant and only uses first arg
- **Line 98:** Returns `nil` instead of error, masking move failures
- **Synchronous execution:** Unlike other actions, this doesn't run in goroutine, blocking the response
- **Temp directory:** Created but never used for actual operation

### Function: NewCloudStorePathMoveActionPerformer() (lines 113-121)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**
1. Creates `cloudStorePathMoveActionPerformer` struct with `cruds` field
2. Returns pointer to the struct

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always `nil`

**Side Effects:**
- **File system operations:** Creates temporary directory (unused)
- **Synchronous execution:** Move operation blocks until completion
- **Config modification:** Sets rclone configuration from credentials
- **Conditional cleanup:** Only removes temp directory on error