# action_integration_execute.go

**File:** server/actions/action_integration_execute.go

## Code Summary

### Constants: Mode (lines 26-33)
- `ModeRequest` - For request body generation (writes to server)
- `ModeResponse` - For response body generation (reads from server)

### Type: integrationActionPerformer (lines 40-48)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map
- `integration resource.Integration` - Integration configuration
- `router *openapi3.T` - OpenAPI 3.0 specification
- `commandMap map[string]*openapi3.Operation` - Maps operation IDs to OpenAPI operations
- `pathMap map[string]string` - Maps operation IDs to URL paths
- `methodMap map[string]string` - Maps operation IDs to HTTP methods
- `encryptionSecret []byte` - Secret for decrypting authentication data

### Function: Name() (lines 51-53)
**Inputs:** None (receiver method)
**Process:** Returns integration name
**Output:** `d.integration.Name`

### Function: DoAction() (lines 56-453)
**Inputs:**
- `request actionresponse.Outcome` - Action request details with method
- `inFieldMap map[string]interface{}` - Input parameters for API call
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Operation Lookup (lines 58-68):**
- Lines 58-61: Gets operation, method, and path from request method
- Lines 66-68: Returns error if operation not found

**2. Authentication Specification Processing (lines 72-83):**
- Line 72: Decrypts authentication spec: `resource.Decrypt(d.encryptionSecret, d.integration.AuthenticationSpecification)`
- Line 78: Unmarshals auth keys: `json.Unmarshal([]byte(decryptedSpec), &authKeys)` (**UNDEFINED** - `json` not imported)
- Lines 81-83: Merges auth keys into input field map

**3. Server URL Resolution (lines 85-105):**
- Lines 85-88: Validates servers exist in OpenAPI spec
- Line 90: Gets base URL from first server
- Lines 92-98: Prefers HTTPS URLs over HTTP
- Lines 99-105: Constructs full URL with path normalization

**4. URL Parameter Processing (lines 107-125):**
- Line 107: Extracts parameter names: `GetParametersNames(url)`
- Lines 113-121: Replaces URL parameters with values:
  - Line 114: Gets parameter value from input map
  - Line 120: Type assertion: `value.(string)`
- Lines 123-125: Evaluates URL with templates: `resource.EvaluateString(url, inFieldMap)`

**5. Request Body Creation (lines 130-167):**
- **For each media type in request body (lines 135-164):**
  - **JSON (lines 137-144):**
    - Line 139: Creates request body: `CreateRequestBody(ModeRequest, mediaType, "request", spec.Schema.Value, inFieldMap)`
    - Line 143: Adds JSON body: `req.BodyJSON(requestBody)`
  - **Form URL Encoded (lines 146-162):**
    - Line 147: Creates request body parameters
    - Lines 155-159: Adds as parameters: `req.Param(requestBody.(map[string]interface{}))`

**6. Authentication Processing (lines 169-374):**
- **OpenAPI Security (lines 171-285):**
  - **OAuth2 (lines 184-210):**
    - Line 186: Gets OAuth token ID: `daptinid.InterfaceToDIR(authKeys["oauth_token_id"])`
    - Line 189: Gets OAuth token: `d.cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId, transaction)`
    - Lines 196-204: Refreshes token if expired
    - Lines 206-208: Adds Bearer authorization header
  - **HTTP Basic/Bearer (lines 212-237):**
    - Lines 214-224: Basic auth with base64 encoding
    - Lines 226-235: Bearer token auth
  - **API Key (lines 239-272):**
    - Lines 242-250: Cookie-based API key
    - Lines 253-261: Header-based API key
    - Lines 264-271: Query parameter API key

- **Fallback Authentication (lines 287-374):**
  - Similar authentication methods as above for when OpenAPI security not specified

**7. Parameter Processing (lines 376-415):**
- Lines 377-413: Processes OpenAPI parameters:
  - Lines 379-380: Skips path parameters (already handled)
  - Lines 386-397: Header parameters
  - Lines 401-412: Query parameters

**8. HTTP Request Execution (lines 417-436):**
- Lines 417-432: Executes HTTP request based on method (POST, GET, DELETE, PATCH, PUT, OPTIONS)
- Lines 433-436: Returns error if request fails

**9. Response Processing (lines 438-452):**
- Line 439: Parses JSON response: `resp.ToJSON(&res)`
- Lines 441-447: Handles JSON parsing errors with fallback
- Lines 448-452: Creates response with status code and data

**10. Return (line 452):**
- Returns responder, action responses, and nil errors

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with API response

**Edge Cases:**
- **Line 78:** `json.Unmarshal()` called but `json` package not imported - **COMPILATION ERROR**
- **Line 120:** Type assertion `value.(string)` can panic if parameter value is not string
- **Line 125:** Type assertion `urlValue.(string)` can panic if evaluated URL is not string
- **Lines 159, 156:** Type assertion `requestBody.(map[string]interface{})` can panic if request body is not map
- **Lines 215-216:** Type assertions `authKeys["username"].(string)` and `authKeys["password"].(string)` can panic
- **Line 267:** Type assertion `authKeys[name].(string)` can panic if auth key is not string
- **Line 316:** Type assertion `authKeys["scheme"].(string)` can panic if scheme is not string
- **Multiple auth type assertions:** Lines 318, 319, 330, 345, 346, 356, 357, 365, 366 all have unhandled type assertions
- **OAuth token handling:** No validation that OAuth tokens are valid or unexpired before use
- **URL parameter injection:** No validation or sanitization of URL parameters
- **External API calls:** Makes HTTP requests to external services without rate limiting or timeout controls
- **Authentication token exposure:** OAuth and API tokens used without encryption in transit

### Function: GetParametersNames() (lines 454-467)
**Inputs:**
- `s string` - URL string with parameter placeholders

**Process:**
- Line 456: Compiles regex for parameter extraction: `\{([^}]+)\}`
- Line 461: Finds all parameter matches
- Lines 463-465: Extracts parameter names from matches

**Output:**
- Returns `([]string, error)` with parameter names

### Function: CreateRequestBody() (lines 472-679)
**Inputs:**
- `mode Mode` - Request or response mode
- `mediaType string` - Content type
- `name string` - Field name
- `schema *openapi3.Schema` - OpenAPI schema
- `values map[string]interface{}` - Input values

**Process:**
**Complex schema-to-value conversion logic with multiple type handlers:**

**1. Boolean Handling (lines 474-495):**
- Lines 476-480: Gets boolean value from input map
- Lines 482-487: Handles string-to-boolean conversion
- Lines 489-492: Handles native boolean values

**2. Number/Integer Handling (lines 496-522):**
- Lines 507-510: String-to-integer conversion: `strconv.ParseInt(value.(string), 10, 64)`
- Lines 512-516: Type conversions for numeric values
- Lines 518-521: Integer vs float return handling

**3. String Handling (lines 523-530):**
- Lines 524-526: Gets string value with nil check
- Line 529: Type assertion: `str.(string)`

**4. Array Handling (lines 531-580):**
- Lines 533-538: Gets array value from input
- Lines 541-562: Complex array type conversion logic
- Lines 568-577: Recursive processing of array items

**5. Object Handling (lines 581-664):**
- Lines 590-633: Recursive object property processing
- Lines 637-649: Additional properties handling
- Lines 651-658: Dynamic property addition

**6. AnyOf Schema Handling (lines 666-675):**
- Lines 667-673: Tries multiple schema options

**Output:**
- Returns `(interface{}, error)` with converted value

**Edge Cases:**
- **Multiple type assertions throughout:** Lines 529, 508, 512, etc. can panic on type mismatches
- **Recursive processing:** Deep nesting could cause stack overflow
- **Schema validation:** No validation that input data matches schema constraints
- **Complex object conversion:** Form URL encoding logic (lines 604-630) is complex and error-prone

### Function: excludeFromMode() (lines 683-695)
**Inputs:**
- `mode Mode` - Request or response mode
- `schema *openapi3.Schema` - Schema to check

**Process:**
- Lines 684-686: Returns true if schema is nil
- Lines 688-691: Excludes read-only fields from requests, write-only fields from responses

**Output:**
- Returns `bool` indicating if field should be excluded

### Function: NewIntegrationActionPerformer() (lines 698-788)
**Inputs:**
- `integration resource.Integration` - Integration configuration
- `initConfig *resource.CmsConfig` - System configuration (unused)
- `cruds map[string]*resource.DbResource` - Database resource map
- `configStore *resource.ConfigStore` - Configuration storage
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Specification Parsing (lines 701-740):**
- Lines 704-722: OpenAPI v2 handling with JSON/YAML support
- Lines 723-739: OpenAPI v3 handling with JSON/YAML support
- Line 708: JSON unmarshal: `json.Unmarshal(yamlBytes, &openapiv2Spec)` (**UNDEFINED** - `json` not imported)
- Line 726: JSON unmarshal: `json.Unmarshal(yamlBytes, &router)` (**UNDEFINED** - `json` not imported)

**2. Reference Resolution (lines 742-747):**
- Line 742: Resolves OpenAPI references: `openapi3.NewLoader().ResolveRefsIn(router, nil)`

**3. Operation Mapping (lines 752-768):**
- Lines 756-767: Maps all operations to action names and creates lookup tables

**4. Handler Creation (lines 771-784):**
- Line 771: Gets encryption secret from config
- Lines 776-783: Creates performer with all mappings

**5. Return (line 786):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Lines 708, 726:** `json.Unmarshal()` called but `json` package not imported - **COMPILATION ERROR**
- **Specification validation:** No validation that OpenAPI specifications are valid or secure
- **Error handling:** Specification parsing errors logged but may not prevent performer creation
- **Unused parameter:** initConfig parameter completely ignored

**Side Effects:**
- **External API integration:** Executes HTTP requests to external APIs based on OpenAPI specifications
- **Authentication handling:** Manages OAuth tokens, API keys, and other authentication methods
- **Dynamic request construction:** Builds HTTP requests dynamically from OpenAPI schemas
- **Token refresh:** Automatically refreshes expired OAuth tokens
- **Database interactions:** Reads OAuth tokens and credentials from database
- **Configuration decryption:** Decrypts stored authentication specifications