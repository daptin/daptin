# action_rename_column.go

**File:** server/actions/action_rename_column.go

## Code Summary

### Type: renameWorldColumnPerformer (lines 15-18)
**Fields:**
- `cmsConfig *resource.CmsConfig` - CMS configuration (unused in implementation)
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 20-22)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"world.column.rename"`

### Function: DoAction() (lines 24-93)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with world and column names
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Parameter Extraction (lines 26-28):**
- Line 26: Type assertion: `inFields["world_name"].(string)`
- Line 27: Type assertion: `inFields["column_name"].(string)`
- Line 28: Type assertion: `inFields["new_column_name"].(string)`

**2. Column Name Processing (lines 30-37):**
- Line 30: Sanitizes new column name: `strings.ReplaceAll(columnToNew, " ", "_")`
- Lines 32-34: Early return if column names are identical
- Lines 35-37: Validates new column name isn't reserved word

**3. World Table Retrieval (lines 38-53):**
- Lines 38-45: Creates mock HTTP GET request for API operations
- Line 46: Gets world table object: `d.cruds["world"].GetObjectByWhereClause("world", "table_name", worldName, transaction)`
- Lines 47-49: Error handling for world retrieval
- Lines 50-52: Creates API model and extracts schema JSON
- Line 52: Type assertion: `schemaJson.(string)`

**4. Schema Parsing (lines 54-55):**
- Line 54: Declares table schema variable
- Line 55: **COMPILATION ERROR** - `json.Unmarshal()` used without importing `encoding/json`

**5. Column Renaming Logic (lines 57-73):**
- Lines 57-67: Iterates through columns to find and rename target column
- Lines 60-66: Updates column name and column name fields when match found
- Lines 69-71: Returns error if column not found
- Line 73: Updates table schema with new columns

**6. Schema Serialization (line 75):**
- Line 75: **COMPILATION ERROR** - `json.Marshal()` used without importing `encoding/json`

**7. Database Schema Update (lines 77-80):**
- Line 77: Executes ALTER TABLE SQL: `"alter table " + tableSchema.TableName + " rename column " + columnToRename + " to " + columnToNew`
- Lines 78-80: Error handling for SQL execution

**8. Configuration Update (lines 82-89):**
- Lines 82-84: Updates world schema JSON in API model
- Line 86: Updates world configuration: `d.cruds["world"].UpdateWithoutFilters(tableData, req, transaction)`
- Lines 87-89: Error handling for configuration update

**9. Success Response (lines 91-92):**
- Lines 91-92: Returns success notification

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with rename result

**Edge Cases:**
- **Lines 26, 27, 28:** Type assertions can panic if fields missing or wrong type
- **Line 52:** Type assertion `schemaJson.(string)` can panic if field wrong type
- **Lines 55, 75:** **COMPILATION ERROR** - `json` package not imported
- **Line 77:** SQL injection vulnerability - table and column names concatenated without validation
- **No transaction rollback:** If ALTER TABLE succeeds but configuration update fails, database and config become inconsistent
- **Column name validation:** Only checks reserved words, doesn't validate SQL identifier rules
- **Case sensitivity:** No handling of case sensitivity issues in column names

### Function: NewRenameWorldColumnPerformer() (lines 95-104)
**Inputs:**
- `initConfig *resource.CmsConfig` - CMS configuration
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Handler Creation (lines 97-100):**
- Creates performer with cruds and config

**2. Return (line 102):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Edge Cases:**
- **Unused field:** cmsConfig field stored but never used in DoAction

**Side Effects:**
- **Database schema modification:** Executes ALTER TABLE commands on production database
- **Configuration updates:** Modifies world schema configuration
- **Mock HTTP requests:** Creates artificial HTTP requests for internal API operations

## Critical Issues Found

### 🚨 Compilation Errors
1. **Lines 55, 75:** `json.Unmarshal()` and `json.Marshal()` used without importing `encoding/json` package

### 🚨 Critical Security Vulnerabilities
2. **SQL injection** (line 77): Table and column names concatenated directly into SQL without validation or escaping
3. **No input validation:** Column and table names not validated for SQL injection attacks

### ⚠️ Runtime Safety Issues
4. **Multiple type assertion panics** (lines 26, 27, 28, 52): Can panic if fields missing or wrong type
5. **Transaction consistency:** If ALTER TABLE succeeds but config update fails, creates inconsistent state

### 🗄️ Database Safety Issues
6. **No transaction rollback:** Database schema changes not properly rolled back on failure
7. **No backup validation:** No verification that column rename is safe or reversible
8. **Production schema modification:** Directly modifies production database schema without safeguards

### 🏗️ Design Issues
9. **Mock HTTP requests:** Creates artificial HTTP requests for internal API operations
10. **Unused struct field:** cmsConfig field stored but never used
11. **Inconsistent error handling:** Some operations have error handling, others don't
12. **Limited validation:** Only checks reserved words, not SQL identifier validity

### 📝 Data Integrity Issues
13. **No foreign key validation:** Doesn't check if column is referenced by foreign keys
14. **No index validation:** Doesn't handle indexes that reference the renamed column
15. **No constraint validation:** Doesn't handle constraints that reference the column
16. **Case sensitivity handling:** No consistent handling of case sensitivity in database operations

### 🔐 Access Control Issues
17. **No authorization checks:** No validation that user has permission to alter database schema
18. **No audit logging:** Schema changes not logged for audit purposes
19. **Unlimited access:** Can rename any column in any world table without restrictions