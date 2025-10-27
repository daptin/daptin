# action_render_template.go

**File:** server/actions/action_render_template.go

## Code Summary

### Type: renderTemplateActionPerformer (lines 20-25)
**Fields:**
- `responseAttrs map[string]interface{}` - Response attributes (unused in implementation)
- `cruds map[string]*resource.DbResource` - Database resource access map
- `configStore *resource.ConfigStore` - Configuration storage (unused in implementation)
- `encryptionSecret []byte` - Encryption secret (unused in implementation)

### Function: Name() (lines 27-29)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"template.render"`

### Function: DoAction() (lines 31-161)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFieldMap map[string]interface{}` - Input parameters with template name and data
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Template Name Extraction (lines 35-38):**
- Line 35: Type assertion: `inFieldMap["template"].(string)`
- Lines 36-38: Returns error if template name not provided

**2. Template Retrieval (lines 39-59):**
- Lines 39-43: Gets template object: `actionPerformer.cruds["template"].GetObjectByWhereClauseWithTransaction("template", "name", template_name, transaction)`
- Line 44: Type assertion: `templateInstance["content"].(string)`
- Line 45: Type assertion: `templateInstance["mime_type"].(string)`
- Line 46: Type assertion: `templateInstance["headers"].(string)`
- Lines 48-56: Parses headers JSON if present:
  - Line 50: **COMPILATION ERROR** - `json.Unmarshal()` used without importing `encoding/json`
- Lines 57-59: Validates template content exists

**3. Base64 Decoding Attempt (lines 61-68):**
- Line 62: Attempts base64 decoding: `base64.StdEncoding.DecodeString(templateContent)`
- Lines 63-66: Uses decoded content if successful
- Line 68: Comment indicates fallback to original content if decoding fails

**4. Subsite File Resolution (lines 69-100):**
**If template content starts with "subsite://":**
- Line 72: Parses subsite path: `strings.SplitN(strings.TrimPrefix(templateContent, "subsite://"), "/", 2)`
- Lines 73-75: Validates path format
- Lines 77-78: Extracts site reference ID and file path
- Line 81: Converts reference ID: `daptinid.InterfaceToDIR(siteReferenceIdStr)`
- Lines 82-84: Validates reference ID
- Line 87: Gets subsite folder cache: `actionPerformer.cruds["template"].SubsiteFolderCache(siteReferenceId)`
- Lines 88-90: Validates subsite exists
- Line 93: Loads file content: `loadFileFromSubsite(assetFolderCache, filePath)`
- Lines 94-96: Error handling for file loading
- Line 99: Uses file content as template

**5. Site File Resolution (lines 102-133):**
**If template content starts with "site://" (duplicate logic):**
- Lines 102-133: Identical logic to subsite file resolution

**6. Template Processing (lines 135-148):**
- Line 135: Creates function map: `soha.CreateFuncMap()`
- Line 137: Parses template: `template.New(template_name).Funcs(sohaFuncMap).Parse(templateContent)`
- Lines 138-141: Error handling for template parsing
- Line 144: Executes template: `tmpl.Execute(&buf, inFieldMap)`
- Lines 145-148: Error handling for template execution

**7. Response Creation (lines 150-160):**
- Lines 150-158: Creates API response with base64 encoded content, mime type, and headers
- Line 154: Base64 encodes output: `resource.Btoa([]byte(buf.String()))`

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with rendered template

**Edge Cases:**
- **Lines 35, 44, 45, 46:** Type assertions can panic if fields missing or wrong type
- **Line 50:** **COMPILATION ERROR** - `json` package not imported
- **Template injection:** User-controlled template content executed with user data (potential code execution)
- **Path traversal:** File paths not validated in subsite/site resolution (lines 78, 111)
- **Duplicate logic:** Site and subsite resolution logic is identical (lines 69-100 vs 102-133)
- **No access control:** No validation of file access permissions
- **Template execution:** User data passed directly to template execution without sanitization

### Function: loadFileFromSubsite() (lines 164-186)
**Inputs:**
- `assetFolderCache *assetcachepojo.AssetFolderCache` - Asset folder cache with local sync path
- `filePath string` - Relative file path

**Process:**

**1. Path Construction (line 166):**
- Line 166: Constructs full path: `assetFolderCache.LocalSyncPath + string(os.PathSeparator) + filePath`

**2. File Validation (lines 168-177):**
- Line 169: Checks file existence: `os.Stat(fullPath)`
- Lines 170-172: Error handling for file not found
- Lines 175-177: Validates path is not a directory

**3. File Reading (lines 179-185):**
- Line 180: Reads file: `ioutil.ReadFile(fullPath)` (deprecated function)
- Lines 181-183: Error handling for read failure
- Line 185: Returns file content as string

**Output:**
- Returns `(string, error)` with file content

**Edge Cases:**
- **Path traversal vulnerability:** No validation of filePath for ".." or absolute paths
- **No access control:** No validation of file permissions or access rights
- **Deprecated function:** Uses `ioutil.ReadFile()` instead of `os.ReadFile()`
- **No file size limits:** Could read extremely large files into memory
- **No file type validation:** Could read any file type without restrictions

### Function: NewRenderTemplateActionPerformer() (lines 188-200)
**Inputs:**
- `cruds map[string]*resource.DbResource` - Database resource map
- `configStore *resource.ConfigStore` - Configuration storage
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Encryption Secret Retrieval (line 190):**
- Line 190: Gets encryption secret (error ignored): `configStore.GetConfigValueFor("encryption.secret", "backend", transaction)`

**2. Handler Creation (lines 192-196):**
- Creates performer with cruds, encryption secret, and config store
- Does not set responseAttrs field

**3. Return (line 198):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Error ignored:** Encryption secret retrieval error silently ignored
- **Unused fields:** responseAttrs, configStore, and encryptionSecret stored but never used

**Side Effects:**
- **Template execution:** Executes user-controlled templates with user data
- **File system access:** Reads files from local file system based on user input
- **HTML/content generation:** Generates dynamic content based on templates

## Critical Issues Found

### 🚨 Compilation Errors
1. **Line 50:** `json.Unmarshal()` used without importing `encoding/json` package

### 🚨 Critical Security Vulnerabilities
2. **Template injection** (line 137): User-controlled template content executed - **POTENTIAL REMOTE CODE EXECUTION**
3. **Path traversal** (lines 78, 111, 166): File paths not validated for ".." or absolute paths
4. **Arbitrary file access** (lines 93, 126): Can read any file accessible to the application

### ⚠️ Runtime Safety Issues
5. **Multiple type assertion panics** (lines 35, 44, 45, 46): Can panic if database fields missing or wrong type
6. **Error ignored** (line 190): Encryption secret retrieval error silently ignored

### 🔐 File System Security Issues
7. **No access control:** No validation of file permissions or access rights for subsite/site files
8. **No file size limits:** Could read extremely large files causing memory exhaustion
9. **No file type validation:** Could read any file type including sensitive system files
10. **Deprecated function:** Uses `ioutil.ReadFile()` instead of `os.ReadFile()`

### 🏗️ Design Issues
11. **Duplicate code:** Site and subsite resolution logic identical (lines 69-100 vs 102-133)
12. **Unused struct fields:** responseAttrs, configStore, and encryptionSecret stored but never used
13. **Complex template resolution:** Multiple template source types with similar logic
14. **No template caching:** Templates parsed and executed on every request

### 🎨 Template Security Issues
15. **User data injection:** User input passed directly to template execution without sanitization
16. **Template function exposure:** Exposes soha function map without restrictions
17. **No template validation:** No validation of template syntax or safety before execution
18. **Template size limits:** No limits on template size or complexity

### 📂 Path Handling Issues
19. **Cross-platform path issues:** Manual path separator construction instead of using filepath.Join
20. **Path validation gaps:** No validation of relative vs absolute paths
21. **Reference ID conversion:** No validation of reference ID format or existence