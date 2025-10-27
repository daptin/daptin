# action_site_sync_storage.go

**File:** server/actions/action_site_sync_storage.go

## Code Summary

### Type: syncSiteStorageActionPerformer (lines 23-25)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 27-29)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"site.storage.sync"`

### Function: DoAction() (lines 31-129)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with cloud store, site, and path information
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Parameter Extraction (lines 35-41):**
- Line 35: Reference ID conversion: `daptinid.InterfaceToDIR(inFields["cloud_store_id"])`
- Line 36: Reference ID conversion: `daptinid.InterfaceToDIR(inFields["site_id"])`
- Line 37: Type assertion: `inFields["path"].(string)`
- Line 38: Gets cloud store: `d.cruds["cloud_store"].GetCloudStoreByReferenceId(cloudStoreId, transaction)`
- Lines 39-41: Error handling for cloud store retrieval

**2. Site Cache Validation (lines 43-47):**
- Line 43: Gets site cache folder: `d.cruds["cloud_store"].SubsiteFolderCache(siteId)` (error ignored)
- Lines 44-47: Returns error if site cache not found

**3. Configuration Setup (lines 49-61):**
- Line 49: Uses cloud store name for config
- Lines 50-52: Extracts config name from root path if contains ":"
- Lines 53-61: Sets up rclone configuration from credentials:
  - Line 55: Uses `resource.CheckErr()` (may panic instead of returning error)
  - Lines 57-59: Sets credential values in rclone config: `config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))`

**4. Path Resolution (lines 63-66):**
- Lines 63-66: Uses provided path or defaults to site cache local sync path

**5. Site Type Detection (lines 68-72):**
- Line 68: Gets site object: `d.cruds["site"].GetSingleRowByReferenceIdWithTransaction("site", siteId, nil, transaction)`
- Lines 69-71: Error handling for site retrieval
- Line 72: Checks if site type is "hugo"

**6. Sync Arguments Setup (lines 74-84):**
- Line 74: Trims keyname: `strings.Trim(siteCacheFolder.Keyname, "/")`
- Lines 75-78: Creates sync arguments array with source and destination paths
- Line 80: Creates rclone filesystem objects: `cmd.NewFsSrcFileDst(args)`
- Lines 82-84: Creates cobra command for rclone

**7. Asynchronous Sync Operation (lines 86-119):**
**Runs in goroutine:**
- Lines 87-90: Validates source and destination filesystems
- Lines 92-96: Additional validation with different error message
- Lines 99-105: Configures rclone sync settings with debug logging and auto-confirm
- Lines 106-110: Performs sync or file copy operation
- Lines 112-116: **Hugo build execution for hugo sites:**
  - Line 114: Executes Hugo command: `hugoCommand.Execute([]string{"--source", tempDirectoryPath, "--destination", tempDirectoryPath + "/" + "public", "--verbose", "--verboseLog"})`

**8. Success Response (lines 121-128):**
- Lines 121-126: Creates success notification response
- Line 128: Returns response immediately (before sync completes)

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with sync status

**Edge Cases:**
- **Line 37:** Type assertion `inFields["path"].(string)` can panic if field missing or wrong type
- **Line 43:** Site cache folder error silently ignored
- **Line 55:** `resource.CheckErr()` may panic instead of returning error
- **Line 114:** **POTENTIAL CODE EXECUTION** - Hugo command executed with user-controlled source directory
- **Asynchronous operation:** Returns success before actual sync completes
- **No validation:** Source and destination paths not validated for security
- **Credential exposure:** Cloud credentials set in global rclone config
- **Path injection:** User-controlled paths used directly in filesystem operations

### Function: NewSyncSiteStorageActionPerformer() (lines 131-139)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Handler Creation (lines 133-135):**
- Creates performer with cruds

**2. Return (line 137):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **No validation:** cruds parameter not validated for nil

**Side Effects:**
- **File system synchronization:** Syncs files between cloud storage and local filesystem
- **External command execution:** Executes Hugo static site generator
- **Global configuration modification:** Modifies rclone global configuration
- **Asynchronous operations:** Starts background goroutines for sync operations
- **Network operations:** Connects to external cloud storage services

## Critical Issues Found

### 🚨 Critical Security Vulnerabilities
1. **Potential code execution** (line 114): Hugo command executed with user-controlled source directory
2. **Path injection:** User-controlled paths used directly in filesystem operations without validation
3. **No input validation:** Source and destination paths not validated for security
4. **Credential exposure:** Cloud credentials set in global rclone config accessible to all operations

### ⚠️ Runtime Safety Issues
5. **Type assertion panic** (line 37): `inFields["path"].(string)` can panic if field missing or wrong type
6. **Silent error ignoring** (line 43): Site cache folder error silently ignored
7. **CheckErr panic risk** (line 55): `resource.CheckErr()` may panic instead of returning error

### 🔐 Command Execution Security Issues
8. **Hugo command injection:** Hugo executed with user-controlled directory paths
9. **No command validation:** No validation of Hugo command parameters
10. **Process execution:** External processes executed without sandboxing
11. **No resource limits:** No limits on Hugo build time or resource usage

### 🌐 Network Security Issues
12. **External cloud access:** Connects to arbitrary cloud storage services
13. **Credential management:** Stores cloud credentials in global configuration
14. **No network validation:** No validation of cloud storage endpoints
15. **Uncontrolled sync:** Can sync arbitrary data from cloud storage

### 🏗️ Design Issues
16. **Asynchronous operation:** Returns success before actual sync completes
17. **Global state modification:** Modifies global rclone configuration
18. **No error propagation:** Sync errors not propagated to user
19. **Resource management:** No cleanup of temporary files or configurations

### 📂 File System Security Issues
20. **Directory traversal:** No validation of sync paths for traversal attacks
21. **Overwrite protection:** No protection against overwriting critical system files
22. **Symlink attacks:** No protection against symbolic link attacks
23. **File permissions:** No validation of file permissions after sync

### ⚙️ Operational Issues
24. **No sync status:** No way to track sync operation progress or completion
25. **No rollback:** No mechanism to rollback failed syncs
26. **Concurrent access:** No protection against concurrent sync operations
27. **Hugo dependency:** Requires Hugo binary to be installed and accessible

### 🔒 Access Control Issues
28. **No authorization:** No checks for user permissions to sync specific sites
29. **No audit logging:** Sync operations not logged for security monitoring
30. **Site boundary bypass:** No validation that user has access to specific sites or cloud stores