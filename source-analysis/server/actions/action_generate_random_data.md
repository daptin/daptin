# action_generate_random_data.go

**File:** server/actions/action_generate_random_data.go

## Code Summary

### Type: randomDataGeneratePerformer (lines 19-23)
**Fields:**
- `cmsConfig *resource.CmsConfig` - System configuration with table definitions
- `cruds map[string]*resource.DbResource` - Database resource access map
- `tableMap map[string][]api2go.ColumnInfo` - Mapping of table names to column information

### Function: Name() (lines 25-27)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"generate.random.data"`

### Function: DoAction() (lines 29-115)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused)
- `inFields map[string]interface{}` - Input parameters with table_name, count, user information
- `transaction *sqlx.Tx` - Database transaction

**Process:**

**1. Parameter Extraction (lines 33-58):**
- Line 33: Logs table name: `inFields["table_name"]`
- Lines 39-41: Gets user reference ID: `daptinid.InterfaceToDIR(inFields["user_reference_id"])`
- Line 43: Type assertion and conversion: `strconv.ParseInt(inFields[resource.USER_ACCOUNT_ID_COLUMN].(string), 10, 32)`
- Line 49: Type assertion: `inFields["table_name"].(string)`
- Lines 51-55: Gets table resource and validates existence
- Line 57: Type assertion: `inFields["count"].(float64)`

**2. Random Data Generation Loop (lines 59-80):**
- **For each row to generate (lines 60-79):**
  - Line 61: Gets column definitions: `actionPerformer.tableMap[tableName]`
  - Line 62: Generates fake row: `GetFakeRow(columns)`
  - **Foreign Key Processing (lines 63-74):**
    - Lines 64-73: For foreign key columns, gets random existing row: `actionPerformer.cruds[column.ForeignKeyData.Namespace].GetRandomRow(...)`
    - Line 71: Sets foreign key value: `daptinid.InterfaceToDIR(foreignRow[0]["reference_id"]).String()`
  - Lines 76-78: Sets reference_id (UUID) and default permissions
  - Line 79: Appends to rows array

**3. Request Context Setup (lines 81-97):**
- Line 81: Creates URL: `url.Parse("/" + tableName)`
- Lines 83-86: Creates HTTP POST request
- Lines 88-93: Creates session user with extracted user ID and reference ID
- Lines 95-97: Creates API2GO request wrapper

**4. Data Insertion (lines 99-106):**
- **For each generated row (lines 99-106):**
  - Line 101: Creates database record: `tableResource.CreateWithTransaction(api2go.NewApi2GoModelWithData(tableName, nil, 0, nil, row), req, transaction)`
  - Lines 102-105: Logs errors but continues processing

**5. Response Creation (lines 107-114):**
- Lines 107-113: Creates success response with message "Random data generated"

**6. Return (line 114):**
- Returns responder, empty responses array, and nil errors

**Output:**
- Returns `(api2go.Responder, []actionresponse.ActionResponse, []error)` with success response

**Edge Cases:**
- **Line 43:** Type assertion `inFields[resource.USER_ACCOUNT_ID_COLUMN].(string)` can panic if field is not string
- **Line 49:** Type assertion `inFields["table_name"].(string)` can panic if field is not string
- **Line 57:** Type assertion `inFields["count"].(float64)` can panic if field is not float64
- **Line 71:** Array access `foreignRow[0]` can panic if foreignRow is empty (despite length check on line 67)
- **No input validation:** Count parameter not validated for reasonable bounds
- **Resource exhaustion:** Large count values could cause memory/database issues
- **Transaction scope:** Long-running operation within transaction may cause locks
- **Error continuation:** Individual row insertion errors logged but don't stop overall process
- **Permission handling:** All generated rows get `auth.DEFAULT_PERMISSION` regardless of user context
- **Foreign key integrity:** No verification that foreign key references are valid after generation

### Function: GetFakeRow() (lines 117-147)
**Inputs:**
- `columns []api2go.ColumnInfo` - Column definitions for the table

**Process:**

**1. Row Initialization (line 119):**
- Creates empty row map

**2. Column Processing Loop (lines 121-143):**
- **For each column (lines 122-142):**
  - Lines 123-125: Skips foreign key columns
  - Lines 127-136: Skips standard system columns by comparing with `resource.StandardColumns`
  - Line 138: Generates fake data: `resource.ColumnManager.GetFakeData(col.ColumnType)`
  - Line 141: Sets fake value in row

**3. Return (line 145):**
- Returns row map with fake data

**Output:**
- Returns `map[string]interface{}` with generated fake data

**Edge Cases:**
- **Column type handling:** Depends on `resource.ColumnManager.GetFakeData()` supporting all column types
- **Standard column detection:** Linear search through standard columns for each column
- **Data type consistency:** No validation that generated fake data matches expected column constraints
- **Null handling:** No consideration for nullable vs non-nullable columns

### Function: NewRandomDataGeneratePerformer() (lines 149-164)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Table Map Creation (lines 151-154):**
- Lines 151-154: Creates mapping of table names to column information

**2. Handler Creation (lines 156-160):**
- Creates performer with configuration and table mapping

**3. Return (line 162):**
- Returns performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)`

**Side Effects:**
- **Database population:** Inserts generated random/fake data into specified tables
- **Foreign key dependencies:** Creates relationships between tables using existing data
- **Permission assignment:** All generated records get default permissions
- **Transaction usage:** Performs multiple database insertions within provided transaction
- **Resource consumption:** Can generate large amounts of data affecting database size and performance