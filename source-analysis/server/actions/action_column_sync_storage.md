# action_column_sync_storage.go

**File:** server/actions/action_column_sync_storage.go

## Code Summary

### Type: syncColumnStorageActionPerformer (lines 19-21)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 23-25)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"column.storage.sync"`

### Function: DoAction() (lines 27-100)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with column/table names and credentials
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Input Validation (lines 31-39):**
- Lines 31-35: Gets `column_name` from `inFields["column_name"]`:
  - Type asserts to string
  - If assertion fails: Returns error `[]error{errors.New("missing column name")}`
- Lines 36-39: Gets `table_name` from `inFields["table_name"]`:
  - Type asserts to string
  - If assertion fails: Returns error `[]error{errors.New("missing table name")}`

**2. Cache Folder Lookup (lines 41-45):**
- Line 41: Looks up cache folder: `d.cruds["world"].AssetFolderCache[tableName][columnName]`
- Lines 42-44: Validates cache folder exists:
  - If not found: Returns error `[]error{errors.New("not a synced folder")}`
- Line 45: Gets cloud store: `cloudStore := cacheFolder.CloudStore`

**3. Credential Setup (lines 47-60):**
- Line 47: Gets `credential_name` from `inFields["credential_name"]` (no type assertion)
- Line 48: Sets config name to `cloudStore.Name`
- Lines 49-51: Checks if `cloudStore.RootPath` contains ":" and extracts prefix:
  - `configSetName = strings.Split(cloudStore.RootPath, ":")[0]`
- Lines 52-60: If credential provided:
  - Line 53: Type asserts credential name: `credentialName.(string)`
  - Line 53: Retrieves credential: `d.cruds["credential"].GetCredentialByName()`
  - Lines 55-59: Sets config values from credential data

**4. Path Construction (lines 62-69):**
- Lines 62-65: Creates args array:
  - `args[0] = cloudStore.RootPath` (source)
  - `args[1] = cacheFolder.LocalSyncPath` (destination)
- Lines 67-69: If keyname exists, appends to source path:
  - `args[0] = args[0] + "/" + cacheFolder.Keyname`

**5. Sync Execution (lines 71-90):**
- Line 71: Creates source and destination: `cmd.NewFsSrcDst(args)`
- Lines 77-90: Executes sync in goroutine:
  - Lines 78-81: First null check with custom error message
  - Lines 84-87: **Duplicate null check** (redundant)
  - Line 88: Executes sync: `sync.CopyDir(ctx, fdst, fsrc, true)`
  - Line 89: Returns sync result

**6. Response (lines 92-97):**
- Creates success notification with message "Cloud storage sync queued"

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with success notification

**Edge Cases:**
- **Missing column_name:** Properly validated and returns error
- **Missing table_name:** Properly validated and returns error
- **Line 53:** Type assertion `credentialName.(string)` can panic if credential_name is not string
- **Line 50:** `strings.Split(cloudStore.RootPath, ":")[0]` could panic if RootPath is empty
- **Non-existent cache folder:** Properly validated and returns error
- **Nil cloudStore:** Could cause panic when accessing `cloudStore.Name` or `cloudStore.RootPath`
- **Path injection:** No validation on `cacheFolder.Keyname` - could contain path traversal sequences
- **Asset folder cache access:** No bounds checking on nested map access `AssetFolderCache[tableName][columnName]`
- **Redundant null checks:** Lines 78-81 and 84-87 check the same conditions
- **Success response:** Always returned even if sync operation fails
- **No error propagation:** Goroutine errors don't affect the returned response

### Function: NewSyncColumnStorageActionPerformer() (lines 102-110)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**
1. Creates `syncColumnStorageActionPerformer` struct with `cruds` field
2. Returns pointer to the struct

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always `nil`

**Side Effects:**
- **Goroutine execution:** Sync operation runs asynchronously
- **Config modification:** Sets rclone configuration from credentials
- **File synchronization:** Syncs cloud storage to local cache folder