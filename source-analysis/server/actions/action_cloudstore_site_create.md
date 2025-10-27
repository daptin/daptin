# action_cloudstore_site_create.go

**File:** server/actions/action_cloudstore_site_create.go

## Code Summary

### Type: cloudStoreSiteCreateActionPerformer (lines 26-28)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 30-32)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"cloudstore.site.create"`

### Function: DoAction() (lines 34-149)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with site configuration and credentials
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Temporary Directory Setup (lines 38-45):**
- Line 38: Generates UUID v7 (ignores error with `_`)
- Line 39: Creates directory name: `"upload-" + u.String()[0:8]`
- Line 40: Creates temp directory: `os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)`
- Line 45: Checks error with `resource.CheckErr()`

**2. Input Parameter Extraction (lines 46-50):**
- Line 46: Gets `site_type` with type assertion, ignores error: `site_type, _ := inFields["site_type"].(string)`
- Line 47: Gets `user_account_id` with type assertion and UUID parsing: `uuid.Parse(inFields["user_account_id"].(string))`
- Line 49: Gets `cloud_store_id` with type assertion and UUID parsing: `uuid.Parse(inFields["cloud_store_id"].(string))`
- **Error handling:** UUID parsing errors are ignored (assigned to `_`)

**3. Site Type Processing (lines 52-59):**
- **"hugo" case:**
  - Line 55: Executes Hugo command: `hugoCommand.Execute([]string{"new", "site", tempDirectoryPath})`
  - **Security risk:** Uses tempDirectoryPath directly in command execution
- **Default case:** No operation

**4. Path Construction (lines 61-73):**
- Line 61: Gets `root_path` with type assertion: `rootPath := inFields["root_path"].(string)`
- Lines 62-65: Gets `hostname` and validates it exists:
  - If missing: Returns error `[]error{errors.New("hostname is missing")}`
- Line 66: Gets `path` with type assertion: `path := inFields["path"].(string)`
- Lines 68-73: Path normalization:
  - If `path` not empty and needs separator, prepends "/"
  - Concatenates: `rootPath = rootPath + path`

**5. Site Database Creation (lines 80-106):**
- Lines 80-89: Creates HTTP request context with user:
  - Creates URL: `url.Parse("/site")`
  - Creates context with user ID: `context.WithValue(ctx, "user", &auth.SessionUser{...})`
- Lines 94-101: Prepares site data map with hostname, path, cloud_store_id, site_type, name
- Line 102: Creates site in database: `d.cruds["site"].CreateWithoutFilter(newSite, createRequest, transaction)`
- Lines 103-106: Error handling - returns if site creation fails

**6. Credential Setup (lines 110-121):**
- Line 110: Extracts store name: `storeName := strings.Split(rootPath, ":")[0]`
- Gets `credential_name` from `inFields["credential_name"]`
- If credential exists:
  - Line 114: Retrieves credential: `d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)`
  - Lines 116-120: Sets config values from credential data

**7. File Upload (lines 123-139):**
- Line 123: Creates source and destination: `cmd.NewFsSrcDst(args)`
- Lines 128-139: Executes upload in goroutine:
  - Lines 129-132: Validates source and destination
  - Line 134: Copies directory: `sync.CopyDir(ctx, fdst, fsrc, true)`
  - Lines 135-138: Cleanup and error logging

**8. Response (lines 141-146):**
- Creates success notification with message "Cloud storage file upload queued"

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with success notification

**Edge Cases:**
- **Line 47:** Type assertion `inFields["user_account_id"].(string)` can panic if field missing or wrong type
- **Line 49:** Type assertion `inFields["cloud_store_id"].(string)` can panic if field missing or wrong type  
- **Line 61:** Type assertion `inFields["root_path"].(string)` can panic if field missing or wrong type
- **Line 66:** Type assertion `inFields["path"].(string)` can panic if field missing or wrong type
- **Line 114:** Type assertion `credentialName.(string)` can panic if wrong type
- **Lines 47, 49:** UUID parsing errors ignored - could result in zero UUIDs
- **Line 110:** `strings.Split(rootPath, ":")[0]` could panic if `rootPath` is empty
- **Hugo command injection:** Line 55 passes `tempDirectoryPath` directly to external command without validation
- **Database creation failure:** If site creation fails after Hugo site generation, Hugo files remain in temp directory
- **Path validation:** No validation on constructed `rootPath` or path parameters
- **Duplicate hostnames:** No check for existing sites with same hostname
- **Silent type assertion failures:** Line 46 ignores error, could result in empty site_type

### Function: NewCloudStoreSiteCreateActionPerformer() (lines 151-159)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**
1. Creates `cloudStoreSiteCreateActionPerformer` struct with `cruds` field
2. Returns pointer to the struct

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always `nil`

**Side Effects:**
- **File system operations:** Creates temp directory, potentially runs Hugo site generation
- **Database operations:** Creates new site record in database
- **External command execution:** Executes Hugo commands for site generation
- **Goroutine execution:** File upload runs asynchronously
- **Config modification:** Sets rclone configuration from credentials