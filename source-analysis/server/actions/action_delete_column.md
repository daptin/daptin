# action_delete_column.go

**File:** server/actions/action_delete_column.go

## Code Summary

### Type: deleteWorldColumnPerformer (lines 15-18)
**Fields:**
- `cmsConfig *resource.CmsConfig` - CMS configuration (unused in code)
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 20-22)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"world.column.delete"`

### Function: DoAction() (lines 24-94)
**Inputs:**
- `request actionresponse.Outcome` - Action request details with user context
- `inFields map[string]interface{}` - Input parameters with world and column names
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Input Parameter Extraction (lines 26-29):**
- Line 26: Gets world name with type assertion: `inFields["world_name"].(string)`
- Line 27: Gets column name with type assertion: `inFields["column_name"].(string)`
- Line 29: Gets session user from request attributes: `request.Attributes["user"]`

**2. World Lookup (lines 31-38):**
- Line 31: Gets world object by table name: `d.cruds["world"].GetObjectByWhereClauseWithTransaction("world", "table_name", worldName, transaction)`
- Lines 32-34: Returns error if world not found
- Line 36: Assigns table data
- Line 38: Gets schema JSON: `tableData["world_schema_json"]`

**3. Schema Parsing (lines 40-44):**
- Line 41: Unmarshals schema JSON: `json.Unmarshal([]byte(schemaJson.(string)), &tableSchema)` (**UNDEFINED** - `json` not imported)
- Lines 42-44: Returns error if unmarshaling fails

**4. HTTP Request Setup (lines 46-56):**
- Line 46: Parses URL: `url.Parse("/world")`
- Lines 48-53: Creates HTTP request with user context
- Lines 54-56: Creates API2GO request wrapper

**5. Column Removal (lines 58-71):**
- Lines 60-66: Iterates through columns to find and remove target column:
  - If column name matches: Sets `indexToDelete` and continues
  - Otherwise: Adds to `newColumns` array
- Lines 68-70: Validates column was found:
  - If `indexToDelete == -1`: Returns error `[]error{errors.New("no such column")}`
- Line 71: Updates table schema with new columns

**6. Database Operations (lines 73-89):**
- Line 73: Marshals updated schema: `json.Marshal(tableSchema)` (**UNDEFINED** - `json` not imported)
- Line 75: **Executes raw SQL**: `transaction.Exec("alter table " + tableSchema.TableName + " drop column " + columnToDelete)`
- Lines 76-78: Returns error if SQL execution fails
- Lines 80-85: Updates world record with new schema JSON
- Lines 87-89: Returns error if update fails

**7. Response (lines 93):**
- Returns success notification: "Column deleted"

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with success notification

**Edge Cases:**
- **Line 26:** Type assertion `inFields["world_name"].(string)` can panic if field missing or wrong type
- **Line 27:** Type assertion `inFields["column_name"].(string)` can panic if field missing or wrong type
- **Line 38:** No type checking on `tableData["world_schema_json"]` - could be nil or wrong type
- **Line 41:** Type assertion `schemaJson.(string)` can panic if field is not string
- **Lines 41, 73:** `json.Unmarshal()` and `json.Marshal()` called but `json` package not imported - **COMPILATION ERROR**
- **Line 75:** **SQL injection vulnerability** - Direct string concatenation in SQL query:
  - `tableSchema.TableName` and `columnToDelete` inserted without escaping
  - Could allow arbitrary SQL execution if values contain malicious content
- **No authorization check:** Only gets user from context but doesn't validate permissions
- **Destructive operation:** Column deletion is irreversible but has minimal validation
- **Transaction scope:** Schema update and SQL DDL not properly coordinated - partial failures could leave inconsistent state
- **No cascade handling:** Doesn't check for foreign key relationships or indexes that depend on the column

### Function: NewDeleteWorldColumnPerformer() (lines 96-105)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**
1. Creates `deleteWorldColumnPerformer` struct with cruds and config
2. Returns pointer to the struct

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always `nil`

**Side Effects:**
- **Database schema modification:** Permanently removes column from table
- **Metadata update:** Updates world schema JSON in database
- **Data loss:** All data in the deleted column is permanently lost