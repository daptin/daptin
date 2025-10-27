# action_cloudstore_file_delete.go

**File:** server/actions/action_cloudstore_file_delete.go

## Code Summary

### Type: cloudStoreFileDeleteActionPerformer (lines 21-23)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 25-27)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"cloudstore.file.delete"`

### Function: DoAction() (lines 29-97)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused in code)
- `inFields map[string]interface{}` - Input parameters with path and credential info
- `transaction *sqlx.Tx` - Database transaction

**Process:**
1. **Line 31:** Initializes empty `responses` slice
2. **Lines 34-37:** Gets `path` from `inFields["path"]`
   - Type asserts to string
   - If assertion fails or missing: Returns error `[]error{errors.New("path is missing")}`
3. **Line 39:** Gets `root_path` from `inFields["root_path"]` (type asserts to string, no error check)
4. **Lines 40-46:** Constructs full path:
   - If `atPath` is not empty:
     - Checks if `rootPath` doesn't end with "/" AND `atPath` doesn't start with "/"
     - If both true: Appends "/" to `rootPath`
     - Concatenates: `rootPath = rootPath + atPath`
5. **Lines 47-50:** Creates args array with `rootPath` and logs delete target
6. **Lines 52-62:** Credential handling:
   - Gets `credential_name` from `inFields["credential_name"]`
   - If credential exists and not empty:
     - Line 54: Calls `d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)`
     - Line 55: Checks error with `resource.CheckErr()`
     - Line 56: Extracts name from `rootPath` (splits by ":" and takes first part)
     - Lines 57-61: Iterates through `cred.DataMap` and sets config values:
       - `config.Data().SetValue(name, key, fmt.Sprintf("%s", val))`
7. **Lines 64-73:** Sets up rclone operation context:
   - Creates filesystem source: `cmd.NewFsSrc(args)`
   - Creates cobra command with description
   - Creates context and filter
   - Sets up default config with `LogLevel = fs.LogLevelNotice`
8. **Lines 74-87:** Executes delete operation in goroutine:
   - Runs `cmd.Run(true, false, cobraCommand, func()...)`
   - If `fsrc` is nil: Logs error and returns nil
   - Line 80: Attempts `operations.Delete(ctx, fsrc)`
   - Lines 81-83: If delete fails, attempts `operations.Purge(ctx, fsrc, "")`
   - Line 85: Logs result with `resource.InfoErr()`
9. **Lines 89-94:** Creates success response:
   - Sets attributes: `type="success"`, `message="Cloud storage path deleted"`, `title="Success"`
   - Creates action response: `resource.NewActionResponse("client.notify", restartAttrs)`

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` where ActionResponse contains success notification

**Edge Cases:**
- Missing `path` parameter → Returns "path is missing" error
- Missing `root_path` parameter → Would cause panic on type assertion (not handled)
- Missing `credential_name` → Skips credential setup (handled gracefully)
- Type assertion failure on `credential_name` → Would cause panic (not handled)
- Nil `fsrc` → Logs error but continues execution
- Delete operation failure → Attempts purge operation as fallback
- Both delete and purge fail → Logs error but still returns success response

### Function: NewCloudStoreFileDeleteActionPerformer() (lines 99-107)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**
1. **Lines 101-103:** Creates `cloudStoreFileDeleteActionPerformer` struct with `cruds` field
2. **Line 105:** Returns pointer to the struct

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always `nil`

**Side Effects:**
- **Executes in goroutine:** Deletes files/directories from cloud storage using rclone
- **Config modification:** Sets rclone configuration values from credential data
- **Logging:** Logs delete target path and operation results