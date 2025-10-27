# action_delete_table.go

**File:** server/actions/action_delete_table.go

## Code Summary

### Type: deleteWorldPerformer (lines 19-22)
**Fields:**
- `cmsConfig *resource.CmsConfig` - CMS configuration (unused in code)
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 24-26)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"world.delete"`

### Function: DoAction() (lines 28-177)
**Inputs:**
- `request actionresponse.Outcome` - Action request details with user context
- `inFields map[string]interface{}` - Input parameters with world ID
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Input Validation (lines 30-33):**
- Line 30: Converts world ID: `daptinid.InterfaceToDIR(inFields["world_id"])`
- Lines 31-33: Validates ID is not null reference:
  - If null: Returns error `[]error{fmt.Errorf("world id is a null reference")}`

**2. User Context Setup (lines 35-44):**
- Line 35: Gets session user from request attributes
- Lines 37-44: Creates HTTP request context with user

**3. Table Lookup (lines 46-63):**
- Line 46: Finds table by ID: `d.cruds["world"].FindOneWithTransaction(worldIdAsDir, *req, transaction)`
- Lines 47-49: Returns error if table not found
- Lines 51-55: Type asserts result to `api2go.Api2GoModel`
- Line 57: Gets schema JSON: `tableData.GetAttributes()["world_schema_json"]`
- Line 60: Unmarshals schema: `json.Unmarshal([]byte(schemaJson.(string)), &tableSchema)` (**UNDEFINED** - `json` not imported)
- Line 64: Gets relations from table schema

**4. Relation Cleanup (lines 66-155):**
- **For each relation in table:**
  - **Lines 72-81: "belongs_to" relation:**
    - If subject is current table: No action
    - Else: **SQL injection vulnerability** - Line 77: `"alter table " + relation.Subject + " drop column " + relation.ObjectName`
  - **Lines 82-91: "has_one" relation:**
    - Same logic as belongs_to with **SQL injection** on line 87
  - **Lines 93-107: "has_many" and "has_many_and_belongs_to_many" relations:**
    - Line 95: **SQL injection vulnerability** - `"drop table " + relation.GetJoinTableName()`
    - Lines 99-106: Gets join table reference ID for deletion
  - **Lines 110-154: Update related table schemas:**
    - Lines 115-119: Gets other table data
    - Line 122: Unmarshals other table schema: `json.Unmarshal()` (**UNDEFINED** - `json` not imported)
    - Lines 127-136: Filters out relations matching current table
    - Line 138: Marshals updated schema: `json.Marshal()` (**UNDEFINED** - `json` not imported)
    - Lines 144-153: Updates other table with new schema

**5. Main Table Deletion (lines 157-164):**
- Line 157: Parses table ID: `uuid.MustParse(tableData.GetID())`
- Line 158: Adds to removal list
- Line 160: **SQL injection vulnerability** - `"drop table " + tableData.GetAttributes()["table_name"].(string)`
- Lines 161-164: Handles SQL execution errors

**6. World Record Cleanup (lines 166-171):**
- Lines 166-171: Deletes world records for all removed tables

**7. Response (lines 175-176):**
- Returns success notification: "Table deleted" with any accumulated errors

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, []error)` with success notification and error list

**Edge Cases:**
- **Lines 60, 122, 138:** `json.Unmarshal()` and `json.Marshal()` called but `json` package not imported - **COMPILATION ERROR**
- **Line 57:** No type checking on `tableData.GetAttributes()["world_schema_json"]` - could cause panic
- **Line 60:** Type assertion `schemaJson.(string)` can panic if field is not string
- **Line 122:** Type assertion `otherTableData["world_schema_json"].(string)` can panic
- **Line 160:** Type assertion `tableData.GetAttributes()["table_name"].(string)` can panic
- **Lines 77, 87, 95, 160:** **Multiple SQL injection vulnerabilities** from direct string concatenation:
  - `relation.Subject`, `relation.ObjectName`, `relation.GetJoinTableName()`, `table_name` all used without escaping
- **Line 157:** `uuid.MustParse()` will panic if ID is not valid UUID format
- **Transaction coordination:** Multiple SQL DDL operations not properly coordinated - partial failures could leave database in inconsistent state
- **Cascade deletion complexity:** Handles relations but may miss complex dependency chains
- **No authorization checks:** Only gets user but doesn't validate table deletion permissions
- **Destructive operation:** Irreversible table deletion with all data loss
- **Error accumulation:** Continues processing even after SQL errors, could compound issues

### Function: NewDeleteWorldPerformer() (lines 179-188)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**
1. Creates `deleteWorldPerformer` struct with cruds and config
2. Returns pointer to the struct

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always `nil`

**Side Effects:**
- **Database schema destruction:** Permanently removes tables and all related data
- **Cascade relation cleanup:** Modifies other tables to remove references
- **Metadata cleanup:** Removes world records for deleted tables
- **Irreversible data loss:** All data in deleted tables is permanently lost