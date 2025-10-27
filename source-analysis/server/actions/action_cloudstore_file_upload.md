# action_cloudstore_file_upload.go

**File:** server/actions/action_cloudstore_file_upload.go

## Code Summary

### Global Variables (lines 76-77)
- `cleanupmux = sync2.Mutex{}` - Mutex for cleanup path synchronization
- `cleanuppath = make(map[string]bool)` - Track cleanup paths to prevent duplicate cleanup

### Type: fileUploadActionPerformer (lines 31-33)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 35-37)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"cloudstore.file.upload"`

### Function: unzip() (lines 39-74)
**Inputs:**
- `archive string` - Path to zip file
- `target string` - Target directory for extraction

**Process:**
1. **Line 40:** Opens zip reader with `zip.OpenReader(archive)`
2. **Line 45:** Creates target directory with `os.MkdirAll(target, 0755)`
3. **Lines 49-71:** Iterates through zip files:
   - Line 50: Constructs path with `filepath.Join(target, file.Name)`
   - Lines 51-54: If directory, creates with `os.MkdirAll(path, file.Mode())`
   - Lines 56-60: Opens file reader
   - Lines 62-66: Creates target file with `os.OpenFile()`
   - Line 68: Copies content with `io.Copy(targetFile, fileReader)`

**Output:** `error` (nil on success)

**Edge Cases:**
- Zip file not found → Returns error from `zip.OpenReader()`
- Target directory creation fails → Returns error from `os.MkdirAll()`
- File extraction fails → Returns error from individual file operations
- **Deferred closures:** `fileReader.Close()` and `targetFile.Close()` in each iteration (potential resource leak on early returns)

### Function: DoAction() (lines 79-221)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with file data and paths
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Temporary Directory Setup (lines 83-90):**
- Line 83: Generates UUID v7
- Line 84: Creates directory name: `"upload-" + u.String()[0:8]`
- Line 85: Creates temp directory: `os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)`
- **Note:** Line 88 commented out - temp directory cleanup deferred to goroutine

**2. File Processing (lines 91-141):**
- Line 91: Gets `path` from `inFields["path"]` (type asserts to string, no error check)
- Line 92: Gets `file` from `inFields["file"]` (type asserts to `[]interface{}`)
- **If files array invalid:** Returns error `[]error{fmt.Errorf("improper file attachment, expected []interface{} got %v", inFields["file"])}`
- **For each file (lines 95-137):**
  - Line 96: Type asserts file to `map[string]interface{}`
  - Lines 97-101: Gets file name:
    - If missing: Logs error and continues to next file
  - Line 102: Constructs temp file path: `filepath.Join(tempDirectoryPath, fileName)`
  - Lines 104-110: Gets file contents:
    - First tries `file["file"]` then `file["contents"]`
    - If both missing: Continues to next file
  - Lines 111-115: Handles base64 data:
    - Splits by comma and takes last part (handles data URI format)
  - Line 116: Decodes base64: `base64.StdEncoding.DecodeString(encodedPart)`
  - Lines 120-124: Writes file:
    - Creates directory: `os.MkdirAll(fileDir, 0755)`
    - Writes file: `os.WriteFile(temproryFilePath, fileBytes, 0666)`
  - Lines 126-135: ZIP handling:
    - If filename ends with ".zip": Calls `unzip(temproryFilePath, tempDirectoryPath)`
    - Starts goroutine to delete zip file after 5 minutes

**3. Path Construction (lines 143-150):**
- Line 143: Gets `root_path` from `inFields["root_path"]` (type asserts to string, no error check)
- Lines 144-150: Builds full path:
  - If `atPath` not empty and path concatenation logic needed:
    - Adds "/" if `rootPath` doesn't end with "/" and `atPath` doesn't start with "/"
  - Concatenates: `rootPath = rootPath + atPath`

**4. Credential Setup (lines 157-167):**
- Gets `credential_name` from `inFields["credential_name"]`
- If credential exists:
  - Line 159: Retrieves credential: `actionPerformer.cruds["credential"].GetCredentialByName()`
  - Line 161: Extracts name from rootPath (splits by ":")
  - Lines 162-166: Sets config values from credential data

**5. Upload Execution (lines 169-211):**
- Line 169: Creates source and destination: `cmd.NewFsSrcDst(args)`
- Lines 170-179: Sets up context and configuration
- Lines 181-211: Executes upload in goroutine:
  - Lines 182-185: Validates source and destination
  - Lines 187-190: Configures sync and executes: `sync.CopyDir(ctx, fdst, fsrc, false)`
  - Lines 192-208: Cleanup goroutine:
    - Uses mutex to prevent duplicate cleanup
    - Waits 10 minutes then removes temp directory
    - Removes path from cleanup map

**6. Response (lines 213-218):**
- Creates success notification response

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with success notification

**Edge Cases:**
- Missing `path` parameter → Type assertion panic (not handled)
- Missing `root_path` parameter → Type assertion panic (not handled)
- Invalid `file` parameter → Returns error message
- Missing file name → Logs error, continues with next file
- Missing file contents → Continues with next file
- Type assertion failures on individual files → Would cause panic (not handled)
- Base64 decode failure → Logged but continues
- File write failure → Logged but continues
- ZIP extraction failure → Logged but continues
- Credential retrieval failure → Logged but continues
- Upload operation failure → Logged but still returns success
- Temp directory creation failure → Logged but continues
- **Memory leak potential:** Multiple goroutines created without cleanup tracking
- **Resource leak:** File handles in unzip function if early return occurs

### Function: NewFileUploadActionPerformer() (lines 223-231)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**
1. Creates `fileUploadActionPerformer` struct with `cruds` field
2. Returns pointer to the struct

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always `nil`

**Side Effects:**
- **File system operations:** Creates temp directories, writes files, extracts ZIP files
- **Goroutine execution:** Upload and cleanup operations run asynchronously
- **Config modification:** Sets rclone configuration from credentials
- **Global state:** Updates `cleanuppath` map for cleanup tracking