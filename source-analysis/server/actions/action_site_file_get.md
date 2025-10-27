# action_site_file_get.go

**File:** server/actions/action_site_file_get.go

## Code Summary

### Type: cloudStoreFileGetActionPerformer (lines 16-18)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 20-22)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"site.file.get"`

### Function: DoAction() (lines 24-66)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with path and site_id
- `transaction *sqlx.Tx` - Database transaction (unused)

**Process:**

**1. Parameter Extraction (lines 28-30):**
- Line 28: Type assertion: `inFields["path"].(string)`
- Line 29: Reference ID conversion: `daptinid.InterfaceToDIR(inFields["site_id"])`
- Line 30: Gets site cache folder: `d.cruds["cloud_store"].SubsiteFolderCache(id)`

**2. Site Cache Validation (lines 31-41):**
- Lines 31-41: Returns error and client notification if site cache not found

**3. File Retrieval (lines 43-44):**
- Line 43: Gets file by name: `siteCacheFolder.GetFileByName(path)` (error ignored)
- Line 44: Defers file close

**4. File Reading with Size Limit (lines 46-56):**
- Line 47: Creates limited reader (10MB + 1 byte): `io.LimitReader(contents, 10*1024*1024+1)`
- Line 48: Reads all data: `io.ReadAll(limitedReader)`
- Lines 49-51: Error handling for read failure
- Lines 54-56: Validates file size doesn't exceed 10MB limit

**5. Response Creation (lines 57-65):**
- Line 57: Base64 encodes file data: `base64.StdEncoding.EncodeToString(data)`
- Lines 58-60: Creates file response with base64 data
- Lines 61-63: Creates action response with same data
- Line 65: Returns both responses

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with file data

**Edge Cases:**
- **Line 28:** Type assertion `inFields["path"].(string)` can panic if field missing or wrong type
- **Line 43:** File retrieval error silently ignored
- **Path traversal risk:** File path not validated for "../" or absolute paths
- **No access control:** No validation of file access permissions
- **Resource leak potential:** If early return occurs, deferred close may not execute properly
- **Memory usage:** Loads entire file into memory before size checking

### Function: NewCloudStoreFileGetActionPerformer() (lines 68-76)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Handler Creation (lines 70-72):**
- Creates performer with cruds

**2. Return (line 74):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **No validation:** cruds parameter not validated for nil

**Side Effects:**
- **File system access:** Reads files from site cache folders
- **Memory allocation:** Loads file content into memory for base64 encoding
- **Client notifications:** Sends error notifications on failure

## Critical Issues Found

### ⚠️ Runtime Safety Issues
1. **Type assertion panic** (line 28): `inFields["path"].(string)` can panic if field missing or wrong type
2. **Silent error ignoring** (line 43): File retrieval error silently ignored
3. **Resource leak potential:** Deferred close may not execute if early returns occur

### 🔐 Security Concerns
4. **Path traversal vulnerability:** File path not validated for "../" or absolute paths
5. **No access control:** No validation of file access permissions or user authorization
6. **Arbitrary file access:** Can potentially access any file in site cache folders
7. **No input sanitization:** File path used directly without validation

### 💾 Resource Management Issues
8. **Memory consumption:** Loads entire file into memory before size validation
9. **10MB limit bypass:** Uses LimitReader but reads all data first, defeating the purpose
10. **No file type restrictions:** Can read any file type without validation
11. **Base64 overhead:** Base64 encoding increases memory usage by ~33%

### 🏗️ Design Issues
12. **Inefficient size checking:** Reads entire file before checking size limit
13. **Duplicate response data:** Same base64 data included in both response types
14. **Unused parameters:** request and transaction parameters not used
15. **No file metadata:** No information about file type, size, or modification time

### 🔒 Access Control Issues
16. **No authentication:** No verification of user identity or permissions
17. **No authorization:** No checks for file access rights
18. **Site boundary bypass:** No validation that user has access to specific site
19. **No audit logging:** File access not logged for security monitoring

### 📂 File Handling Issues
20. **No file existence check:** Relies on GetFileByName without explicit existence validation
21. **No symlink protection:** No protection against symbolic link attacks
22. **No file locking:** No consideration of concurrent file access
23. **Error handling gaps:** Some file operations lack proper error handling