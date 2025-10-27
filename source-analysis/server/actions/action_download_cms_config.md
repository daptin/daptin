# action_download_cms_config.go

**File:** server/actions/action_download_cms_config.go

## Code Summary

### Type: downloadCmsConfigActionPerformer (lines 12-14)
**Fields:**
- `responseAttrs map[string]interface{}` - Pre-configured response attributes for file download

### Function: Name() (lines 16-18)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"__download_cms_config"`

### Function: DoAction() (lines 20-30)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused in implementation)
- `inFields map[string]interface{}` - Input parameters (unused in implementation)
- `transaction *sqlx.Tx` - Database transaction (unused in implementation)

**Process:**

**1. Response Array Initialization (line 23):**
- Line 23: Creates empty response slice: `make([]actionresponse.ActionResponse, 0)`

**2. Action Response Creation (line 25):**
- Line 25: Creates download action response: `resource.NewActionResponse("client.file.download", d.responseAttrs)`
- Uses pre-configured response attributes from struct field

**3. Response Assembly (line 27):**
- Line 27: Appends action response to responses array

**4. Return (line 29):**
- Returns nil responder, responses array, and nil errors

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with file download response

**Edge Cases:**
- **No validation:** All input parameters ignored - action always succeeds regardless of request context
- **Static response:** Response content is determined at initialization time, not at execution time
- **No error handling:** Cannot fail at runtime since no processing occurs

### Function: NewDownloadCmsConfigPerformer() (lines 32-52)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration to be downloaded

**Process:**

**1. JSON Serialization (lines 34-38):**
- Line 34: Marshals config to indented JSON: `json.MarshalIndent(*initConfig, "", "  ")` (**UNDEFINED** - `json` not imported)
- Lines 35-38: Returns error if marshaling fails

**2. Response Attributes Setup (lines 40-44):**
- Line 40: Creates response attributes map
- Line 41: Base64 encodes JSON content: `base64.StdEncoding.EncodeToString(js)`
- Line 42: Sets filename: `"schema.json"`
- Line 43: Sets content type: `"application/json"`
- Line 44: Sets download message: `"Downloading system schema"`

**3. Performer Creation (lines 46-48):**
- Lines 46-48: Creates performer struct with response attributes

**4. Return (line 50):**
- Returns pointer to performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is nil on success

**Edge Cases:**
- **Line 34:** `json.MarshalIndent()` called but `json` package not imported - **COMPILATION ERROR**
- **Config exposure:** Entire CMS configuration serialized and made downloadable - potential information disclosure
- **No access control:** No validation of who can download the configuration
- **Sensitive data exposure:** Configuration may contain database credentials, API keys, or other sensitive information
- **Large config handling:** No size limits on configuration data being encoded

**Side Effects:**
- **Information disclosure:** Exposes complete system configuration including potentially sensitive settings
- **Base64 encoding overhead:** Configuration data size increased by ~33% due to base64 encoding
- **Memory usage:** Full configuration loaded into memory for serialization