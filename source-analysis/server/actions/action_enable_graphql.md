# action_enable_graphql.go

**File:** server/actions/action_enable_graphql.go

## Code Summary

### Type: graphqlEnableActionPerformer (lines 15-17)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map for configuration management

### Function: Name() (lines 20-22)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"__enable_graphql"`

### Function: DoAction() (lines 26-39)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused in implementation)
- `inFieldMap map[string]interface{}` - Input parameters (unused in implementation)
- `transaction *sqlx.Tx` - Database transaction for configuration update

**Process:**

**1. Configuration Update (line 28):**
- Line 28: Sets GraphQL enable configuration: `d.cruds["world"].ConfigStore.SetConfigValueForWithTransaction("graphql.enable", "true", "backend", transaction)`
- Stores configuration change in database within the provided transaction

**2. Result Processing (lines 30-38):**
- **Lines 30-34: Success Path (Logic Error):**
  - Condition: `if err != nil` - executes when there IS an error
  - Line 31: Commented out restart: `//go Restart()`
  - Lines 33-34: Returns SUCCESS notification: "Restarting with graphql enabled"
- **Lines 35-37: Failure Path (Logic Error):**
  - Condition: `else` - executes when there is NO error
  - Lines 36-37: Returns FAILURE notification: "Failed to update config: "+err.Error()
  - **Critical Bug:** Calls `err.Error()` when `err` is nil, will cause panic

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` with client notification
- **Logic is inverted:** Success treated as failure and vice versa

**Edge Cases:**
- **Line 37:** `err.Error()` called when `err` is nil - **RUNTIME PANIC GUARANTEED**
- **Inverted logic:** Success/failure conditions are backwards
- **Missing restart:** Line 31 restart call is commented out, so GraphQL enabling may not take effect
- **No input validation:** All input parameters ignored
- **No authorization check:** Any user can enable GraphQL without permission validation

### Function: NewGraphqlEnablePerformer() (lines 42-50)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration (unused in implementation)
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**

**1. Performer Creation (lines 44-46):**
- Lines 44-46: Creates performer struct with cruds map
- initConfig parameter is completely ignored

**2. Return (line 48):**
- Returns pointer to performer and nil error

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always nil

**Edge Cases:**
- **Unused parameter:** `initConfig` parameter accepted but never used
- **No validation:** No checks on cruds map validity

**Side Effects:**
- **System configuration change:** Permanently enables GraphQL functionality in system configuration
- **Potential service restart required:** GraphQL enabling may require system restart to take effect (restart call is disabled)
- **Security impact:** Enables GraphQL endpoint which exposes database schema and data access