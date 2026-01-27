# User Feature Documentation Gaps

**Purpose**: Identifies user-configurable features that exist in code but aren't documented in the wiki.

**Generated**: 2026-01-27
**Method**: Systematic code exploration of user-facing capabilities

---

## Documentation Gap Summary

### ✅ Well-Documented Features (No Action Needed)

| Feature | Files | Status |
|---------|-------|--------|
| **Column Types** | Column-Types.md, Column-Type-Reference.md | ✅ All 41 types complete with examples |
| **Schema Properties** | Schema-Reference-Complete.md, Schema-Examples.md | ✅ All 18 TableInfo properties documented |
| **State Machines** | State-Machines.md | ✅ Comprehensive with testing notes |
| **Custom Actions Basics** | Custom-Actions.md | ✅ InFields and OutFields structure covered |
| **Built-in Actions** | Action-Reference.md, multiple guides | ✅ 50+ system actions with examples |
| **Column Properties** | Column-Type-Reference.md | ✅ All flags and options explained |
| **Relationships** | Relationships.md | ✅ Complete relationship types guide |

---

## ❌ Missing User Documentation

### HIGH Priority - Users Need These Now

#### 1. Validation Tags Reference ✅ COMPLETE

**What**: List of validation rules users can apply to columns and actions

**Documented**: wiki/Validation-Reference.md (2026-01-27)

**Code Location**:
- `server/resource/middleware_datavalidation.go` (lines 45-68)
- Uses `gopkg.in/go-playground/validator.v9`

**Found in Code**:
```go
// Examples from column_types.go:
email     - Validates email format (line 227)
required  - Field must not be empty
min=X     - Minimum value (line 190: "min=1,max=12")
max=X     - Maximum value (line 198: "min=100,max=2100")
gt=X      - Greater than
gte=X     - Greater than or equal
lt=X      - Less than
lte=X     - Less than or equal
latitude  - Validates latitude coordinate
longitude - Validates longitude coordinate
iscolor   - Validates color format
url       - Validates URL format
base64    - Validates base64 encoding
len=X     - Exact length
eq=X      - Equals value
ne=X      - Not equals
oneof=X Y - One of specified values
```

**Where Users Apply**:
```yaml
Tables:
  - TableName: product
    Columns:
      - Name: price
        ColumnType: measurement
    Validations:
      - ColumnName: price
        Tags: "required,gt=0"  # <-- Users need to know available tags
```

**Should Document**:
- Complete list of validator.v9 tags
- Syntax for combining multiple tags
- Examples for each common tag
- How tags work with different column types

**Suggested File**: `wiki/Validation-Reference.md`

---

#### 2. Conformation Tags Reference ✅ COMPLETE

**What**: Data transformation rules users can apply before validation

**Documented**: wiki/Conformation-Reference.md (2026-01-27)

**Code Location**:
- `server/resource/middleware_datavalidation.go` (lines 71-83)
- Uses `github.com/artpar/conform`

**Found in Code**:
```go
// Examples from columns.go and utils.go:
email     - Normalizes email format (line 1134)
name      - Normalizes name format (line 1138)
trim      - Removes whitespace (line 1142)
lower     - Converts to lowercase
upper     - Converts to uppercase
snake     - Converts to snake_case (line 371)
```

**Where Users Apply**:
```yaml
Tables:
  - TableName: user_account
    Conformations:
      - ColumnName: email
        Tags: "email"  # <-- Users need to know available tags
      - ColumnName: name
        Tags: "trim"
```

**Should Document**:
- All available conform tags
- Order of operations (conform → validate)
- Difference between Validations vs Conformations
- Examples showing transformation results

**Suggested File**: `wiki/Conformation-Reference.md`

---

#### 3. OutFields Performer Type Reference ⚠️ PARTIAL

**What**: Complete list of action types users can use in custom action OutFields

**Code Location**: 46 performer files in `server/actions/action_*.go`

**Currently Documented**: ~15 types with examples in Custom-Actions.md

**Missing from Docs** (examples found in code):
```yaml
# System performers
__become_admin
__data_export
__data_import
__csv_data_export
__download_cms_config
__upload_xlsx_file_to_entity
__upload_csv_file_to_entity

# Mail performers
mail.send
aws.mail.send

# Cloud performers
cloudstore.file.upload
cloudstore.file.delete
cloudstore.folder.create
cloudstore.path.move
site.file.list
site.file.get
site.file.delete
site.storage.sync
column.storage.sync

# OAuth performers
oauth.client.redirect
oauth.login.response
oauth.profile.exchange

# OTP performers
otp.generate
otp.login.verify

# TLS performers
acme.tls.generate
self.tls.generate

# Client-side performers
client.notify
client.redirect
client.file.download
client.store.set

# Data performers
random.generate
world.column.delete
world.column.rename
world.delete

# Integration performers
integration.install
cloud_store.files.import
```

**Should Document**:
- All 46+ performer types
- Required Attributes for each type
- Method types (EXECUTE, ACTIONRESPONSE, GET, POST, PATCH, DELETE)
- Examples for each performer
- When to use SkipInResponse, LogToConsole, ContinueOnError

**Suggested File**: `wiki/OutFields-Performers.md`

---

#### 4. Data Exchange Complete Reference ⚠️ PARTIAL

**What**: All options for configuring data exchanges

**Code Location**: `server/resource/exchange.go`

**Currently Documented**: Basic structure in Data-Exchange.md

**Missing from Docs**:
```yaml
# Target types found in code:
target_type: "action"       # Execute action
target_type: "rest"         # REST API call
target_type: "gsheet-append" # Google Sheets append
target_type: "self"         # Internal Daptin table

# Attributes for each type:
# action type:
target_attributes:
  action_name: "action_to_execute"
  entity_name: "table_name"

# gsheet-append type:
target_attributes:
  sheetUrl: "https://content-sheets.googleapis.com/v4/spreadsheets/{id}/values/A1:append"
  appKey: "api_key"
options:
  hasHeader: true
```

**Should Document**:
- All source_type options
- All target_type options with required attributes
- Complete attribute structure per type
- Column mapping syntax
- Options available for each exchange type

**Suggested File**: Expand `wiki/Data-Exchange.md` or create `wiki/Data-Exchange-Reference.md`

---

### MEDIUM Priority - Advanced Features

#### 5. Condition Syntax for Actions ❌

**What**: Syntax for conditional OutFields execution

**Code Location**: `actionresponse.Outcome.Condition` field

**Example from Code** (columns.go line 1163):
```yaml
OutFields:
  - Type: "otp.generate"
    Condition: "!mobile != null && mobile != undefined && mobile != ''"
    # Only executes if mobile number is provided
```

**Should Document**:
- JavaScript-like expression syntax
- Available variables ($user, $subject, ~fieldname, !expression)
- Boolean operators (&&, ||, !)
- Comparison operators (==, !=, <, >, <=, >=)
- Examples for common patterns

**Suggested File**: Add section to `wiki/Custom-Actions.md`

---

#### 6. RequestSubjectRelations Usage ⚠️ MINIMAL

**What**: How to preload related records when action executes

**Code Location**: `actionresponse.Action.RequestSubjectRelations` field

**Should Document**:
- Syntax for specifying relations to fetch
- Performance implications
- Examples showing when to use

**Suggested File**: Add section to `wiki/Custom-Actions.md`

---

## Recommendations

### Immediate Actions (Week 1)

1. **Create `wiki/Validation-Reference.md`**
   - List all validator.v9 tags with examples
   - Show syntax for combining tags
   - Provide common validation patterns

2. **Create `wiki/Conformation-Reference.md`**
   - List all conform tags with transformation examples
   - Explain conform → validate order
   - Show use cases for each tag

3. **Create `wiki/OutFields-Performers.md`**
   - Complete catalog of all 46+ performer types
   - Required attributes for each
   - Examples for common patterns

### Short-term Actions (Week 2-3)

4. **Expand `wiki/Data-Exchange.md`**
   - Add complete attribute reference per target_type
   - Document all available source/target types
   - Provide examples for each exchange type

5. **Add to `wiki/Custom-Actions.md`**
   - Condition syntax section with examples
   - RequestSubjectRelations usage guide

---

## Documentation Completeness After Fixes

Current: **~75%** of user-configurable features documented

After addressing gaps: **~95%** coverage

**Remaining 5%**: Edge cases and advanced combinations users discover through experimentation

---

## Testing Approach for New Docs

Per Documentation-TODO.md guidelines:

1. **Start with fresh database**: `rm daptin.db && go run main.go`
2. **Test each validation tag**: Create schema with tag, verify behavior
3. **Test each conformation tag**: Verify transformation before validation
4. **Test each performer type**: Create action using performer, execute, verify result
5. **Test exchange types**: Create exchange for each type, execute, verify data transfer
6. **Test condition syntax**: Create conditional OutFields, test true/false paths

**Document only what you've tested and verified.**
