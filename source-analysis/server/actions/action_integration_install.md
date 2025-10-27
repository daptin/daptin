# action_integration_install.go

**File:** server/actions/action_integration_install.go

## Code Summary

### Type: integrationInstallationPerformer (lines 24-32)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map
- `integration resource.Integration` - Integration configuration (unused in implementation)
- `router *openapi3.T` - OpenAPI 3.0 specification (unused in implementation)
- `commandMap map[string]*openapi3.Operation` - Maps operation IDs to operations (unused in implementation)
- `pathMap map[string]string` - Maps operation IDs to paths (unused in implementation) 
- `methodMap map[string]string` - Maps operation IDs to methods (unused in implementation)
- `encryptionSecret []byte` - Secret for decrypting authentication specifications

### Function: Name() (lines 35-37)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"integration.install"`

### Function: DoAction() (lines 41-239)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFieldMap map[string]interface{}` - Input parameters with reference_id
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Integration Lookup (lines 43-49):**
- Line 43: Converts reference ID: `daptinid.InterfaceToDIR(inFieldMap["reference_id"])`
- Line 44: Gets integration by reference ID: `d.cruds["integration"].GetSingleRowByReferenceIdWithTransaction(...)`
- Lines 46-49: Validates specification exists, returns error if missing

**2. Authentication Specification Processing (lines 51-62):**
- Line 51: Type assertion: `spec.(string)`
- Line 53: Type assertion: `integration["authentication_specification"].(string)`
- Line 55: Decrypts auth spec: `resource.Decrypt(d.encryptionSecret, authSpec)`
- Line 59: Unmarshals auth data: `json.Unmarshal([]byte(decryptedSpec), &authDataMap)` (**COMPILATION ERROR** - `json` not imported)

**3. Specification Format Conversion (lines 64-73):**
- Lines 64-72: If YAML format, converts to JSON: `yaml.YAMLToJSON(specBytes)`

**4. OpenAPI Specification Parsing (lines 75-104):**
- **OpenAPI v2 Processing (lines 77-95):**
  - Line 81: Unmarshals v2 spec: `json.Unmarshal(specBytes, &openapiv2Spec)` (**COMPILATION ERROR** - `json` not imported)
  - Line 88: Converts to v3: `openapi2conv.ToV3(&openapiv2Spec)`
- **OpenAPI v3 Processing (lines 101-104):**
  - Line 103: Loads v3 spec: `openapi3.NewLoader().LoadFromData(specBytes)`

**5. Operation Mapping (lines 106-116):**
- Lines 109-115: Maps all operations to command/path/method lookup tables
- Line 111: Logs operation registration

**6. Server Configuration (lines 118-120):**
- Line 120: Gets host from first server: `router.Servers[0].URL`

**7. Global Security Attributes (lines 122-138):**
- Lines 124-137: Processes security schemes and creates global attributes map
- Lines 130-137: Maps security scheme locations (header, query, path) to attribute names

**8. Action Generation Loop (lines 140-232):**
- **For each command (lines 140-231):**
  - **URL Parameter Processing (lines 144-169):**
    - Line 144: Gets parameter names: `GetParametersNames(host + path)`
    - Lines 157-169: Creates columns for URL parameters not in auth data
  
  - **OpenAPI Parameter Processing (lines 171-183):**
    - Lines 171-182: Creates columns for OpenAPI parameters not in auth data
  
  - **Request Body Parameter Processing (lines 185-212):**
    - Line 189: Gets JSON media type: `contents.Get("application/json")`
    - Line 192: Gets body parameter names: `GetBodyParameterNames(ModeRequest, "", jsonMedia.Schema.Value)`
    - Lines 199-211: Creates columns for body parameters not in auth data
  
  - **Action Creation (lines 214-230):**
    - Line 214: Type assertion: `integration["name"].(string)`
    - Lines 215-230: Creates action with columns and outcome configuration

**9. Action Table Update (lines 234-236):**
- Line 234: Updates action table: `resource.UpdateActionTable(&resource.CmsConfig{Actions: actions}, transaction)`

**10. Return (line 238):**
- Returns nil responder, empty responses, and error from action update

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, []error)` with action installation result

**Edge Cases:**
- **Line 51:** Type assertion `spec.(string)` can panic if specification is not string type
- **Line 53:** Type assertion `integration["authentication_specification"].(string)` can panic if field is not string
- **Line 59:** `json.Unmarshal()` called but `json` package not imported - **COMPILATION ERROR**
- **Line 81:** `json.Unmarshal()` called but `json` package not imported - **COMPILATION ERROR**
- **Line 120:** Array access `router.Servers[0]` can panic if no servers defined
- **Line 214:** Type assertion `integration["name"].(string)` can panic if name is not string
- **Multiple unused struct fields:** integration, router, commandMap, pathMap, methodMap stored but never used
- **No error handling:** OpenAPI parsing errors may not be properly handled
- **No validation:** Integration specifications not validated for security or correctness
- **Parameter name collisions:** No handling of duplicate parameter names across different sources

### Function: NewIntegrationInstallationPerformer() (lines 242-255)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration (unused)
- `cruds map[string]*resource.DbResource` - Database resource map
- `configStore *resource.ConfigStore` - Configuration storage
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Encryption Secret Retrieval (lines 244-247):**
- Line 244: Gets encryption secret: `configStore.GetConfigValueFor("encryption.secret", "backend", transaction)`

**2. Handler Creation (lines 248-251):**
- Creates performer with cruds and encryption secret only

**3. Return (line 253):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Unused parameters:** initConfig parameter completely ignored
- **Incomplete initialization:** Many struct fields left uninitialized despite being declared

### Function: GetBodyParameterNames() (lines 257-318)
**Inputs:**
- `mode Mode` - Request or response mode
- `name string` - Parameter name context
- `schema *openapi3.Schema` - OpenAPI schema to analyze

**Process:**

**1. Schema Type Processing (lines 259-315):**
- **Primitive Types (lines 260-265):** Returns empty arrays for boolean, number, integer, string
- **Array Type (lines 266-280):** Recursively processes array item schemas
- **Object Type (lines 281-314):**
  - Lines 284-299: Processes object properties recursively
  - Lines 301-312: Handles additional properties

**2. Error Return (line 317):**
- Returns error for unrecognized schema types

**Output:**
- Returns `([]string, error)` with parameter names

**Edge Cases:**
- **Recursive processing:** Deep schema nesting could cause stack overflow
- **Missing Mode definition:** ModeRequest constant referenced but not defined in this file
- **Missing excludeFromMode function:** Function called but not defined in this file
- **Error handling:** Individual parameter extraction errors logged but don't stop overall processing

**Side Effects:**
- **Action registration:** Creates new actions in the system action table based on OpenAPI specifications
- **Parameter extraction:** Analyzes OpenAPI schemas to determine required input parameters
- **Authentication integration:** Excludes parameters that are handled by authentication mechanisms
- **Database modification:** Updates action configuration in database transaction

## Critical Issues Found

### 🚨 Compilation Errors
1. **Missing json import** (lines 59, 81): `json.Unmarshal()` called without importing `encoding/json`
2. **Undefined constants/functions**: ModeRequest and excludeFromMode referenced but not defined in this file

### ⚠️ Runtime Safety Issues
3. **Panic-prone type assertions** (lines 51, 53, 120, 214): No error handling for type conversions
4. **Array bounds access** (line 120): `router.Servers[0]` without checking if servers exist
5. **Unused struct fields**: Most integrationInstallationPerformer fields are never used after initialization

### 🔐 Security Concerns
6. **No input validation**: OpenAPI specifications processed without security validation
7. **Parameter injection**: URL parameters used without sanitization
8. **Authentication exposure**: Decrypted authentication data handled without proper validation

### 🏗️ Design Issues
9. **Incomplete initialization**: Many struct fields declared but never set in constructor
10. **Error handling gaps**: OpenAPI parsing errors may not prevent action creation
11. **Resource management**: No cleanup or validation of external specifications