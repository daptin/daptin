# action_site_file_list.go

**File:** server/actions/action_site_file_list.go

## Code Summary

### Type: cloudStoreFileListActionPerformer (lines 12-14)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 16-18)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"site.file.list"`

### Function: DoAction() (lines 20-50)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with path and site_id
- `transaction *sqlx.Tx` - Database transaction (unused)

**Process:**

**1. Parameter Extraction (lines 24-26):**
- Line 24: Type assertion: `inFields["path"].(string)`
- Line 25: Reference ID conversion: `daptinid.InterfaceToDIR(inFields["site_id"])`
- Line 26: Gets site cache folder: `d.cruds["cloud_store"].SubsiteFolderCache(id)` (error ignored)

**2. Site Cache Validation (lines 28-38):**
- Lines 28-38: Returns error and client notification if site cache not found

**3. Directory Contents Retrieval (line 40):**
- Line 40: Gets path contents: `siteCacheFolder.GetPathContents(path)` (error ignored)

**4. Response Creation (lines 42-49):**
- Lines 42-44: Creates file response with contents under "files" key
- Lines 45-47: Creates action response with same contents under "list" key
- Line 49: Returns both responses

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with directory listing

**Edge Cases:**
- **Line 24:** Type assertion `inFields["path"].(string)` can panic if field missing or wrong type
- **Line 26:** Site cache folder retrieval error silently ignored
- **Line 40:** Directory contents retrieval error silently ignored
- **Path traversal risk:** Directory path not validated for "../" or absolute paths
- **No access control:** No validation of directory access permissions
- **Information disclosure:** Exposes directory structure without authorization

### Function: NewCloudStoreFileListActionPerformer() (lines 52-60)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Handler Creation (lines 54-56):**
- Creates performer with cruds

**2. Return (line 58):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **No validation:** cruds parameter not validated for nil

**Side Effects:**
- **Directory enumeration:** Lists contents of site cache directories
- **Information disclosure:** Reveals file and directory structure
- **Client notifications:** Sends error notifications on failure

## Critical Issues Found

### ⚠️ Runtime Safety Issues
1. **Type assertion panic** (line 24): `inFields["path"].(string)` can panic if field missing or wrong type
2. **Silent error ignoring** (lines 26, 40): Site cache and directory contents errors silently ignored

### 🔐 Security Concerns
3. **Path traversal vulnerability:** Directory path not validated for "../" or absolute paths
4. **No access control:** No validation of directory access permissions or user authorization
5. **Information disclosure:** Exposes directory structure and file listings without restrictions
6. **Directory enumeration:** Allows enumeration of site cache directory contents

### 🔒 Access Control Issues
7. **No authentication:** No verification of user identity or permissions
8. **No authorization:** No checks for directory access rights
9. **Site boundary bypass:** No validation that user has access to specific site
10. **No audit logging:** Directory access not logged for security monitoring

### 🏗️ Design Issues
11. **Unused parameters:** request and transaction parameters not used
12. **Inconsistent response keys:** Uses "files" in one response and "list" in another for same data
13. **Duplicate response data:** Same directory contents included in both response types
14. **No pagination:** No support for large directory listings

### 📂 Directory Handling Issues
15. **No directory validation:** No checks if path exists or is actually a directory
16. **No symlink protection:** No protection against symbolic link attacks
17. **No error handling:** Directory operations lack proper error handling
18. **No metadata:** No information about file types, sizes, or modification times

### 🌐 Information Security Issues
19. **Metadata exposure:** Directory structure reveals internal organization
20. **File enumeration:** Allows discovery of all files in site cache
21. **No filtering:** No filtering of sensitive files or hidden directories
22. **No rate limiting:** No protection against directory enumeration attacks

### 💾 Resource Issues
23. **No limits:** No limits on directory size or number of entries returned
24. **Memory usage:** Could consume significant memory for large directories
25. **No caching:** No caching of directory listings for performance