# action_become_admin.go

**File:** server/actions/action_become_admin.go

## Code Summary

### Type: becomeAdminActionPerformer (lines 13-15)
**Fields:**
- `cruds map[string]*resource.DbResource` - Database resource access map

### Function: Name() (lines 18-20)
**Inputs:** None (receiver method)
**Process:** Returns hardcoded string
**Output:** `"__become_admin"`

### Function: DoAction() (lines 24-56)
**Inputs:**
- `request actionresponse.Outcome` - Action request details (unused in code)
- `inFieldMap map[string]interface{}` - Input parameters containing user data
- `transaction *sqlx.Tx` - Database transaction

**Process:**
1. **Line 26:** Calls `d.cruds["world"].CanBecomeAdmin(transaction)` to check authorization
   - If `false`: Returns error `[]error{errors.New("Unauthorized")}`
2. **Line 29:** Gets `user` from `inFieldMap["user"]`
   - If `nil`: Returns error `[]error{errors.New("Unauthorized")}`
3. **Line 33:** Type asserts user to `map[string]interface{}`
4. **Line 35:** Creates empty `responseAttrs` map
5. **Line 38:** Calls `d.cruds["world"].BecomeAdmin(user["id"].(int64), transaction)`
   - **If BecomeAdmin returns true:**
     - Line 39: Commits transaction with `transaction.Commit()`
     - Line 40: Checks commit error with `resource.CheckErr()`
     - Lines 41-43: Sets response attributes:
       - `responseAttrs["location"] = "/"`
       - `responseAttrs["window"] = "self"`
       - `responseAttrs["delay"] = 7000`
     - Line 45: Creates redirect action response: `resource.NewActionResponse("client.redirect", responseAttrs)`
     - Line 46: Destroys cache: `resource.OlricCache.Destroy(context.Background())`
   - **If BecomeAdmin returns false:**
     - Line 48: Rolls back transaction with `transaction.Rollback()`
     - Line 49: Checks rollback error with `resource.CheckErr()`

**Output:**
- Returns `(nil, []actionresponse.ActionResponse, nil)` where ActionResponse array contains:
  1. The redirect response (if successful) or empty ActionResponse (if failed)
  2. Restart response: `{ResponseType: "Restart", Attributes: nil}`

**Edge Cases:**
- User not provided in `inFieldMap` → Returns "Unauthorized" error
- `CanBecomeAdmin()` returns false → Returns "Unauthorized" error  
- Transaction commit/rollback failures → Logged via `resource.CheckErr()` but doesn't stop execution
- Type assertion failure on `user["id"].(int64)` → Would cause panic (not handled)

### Function: NewBecomeAdminPerformer() (lines 59-67)
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration (unused in code)
- `cruds map[string]*resource.DbResource` - Database resource map

**Process:**
1. **Lines 61-63:** Creates `becomeAdminActionPerformer` struct with `cruds` field
2. **Line 65:** Returns pointer to the struct

**Output:**
- Returns `(actionresponse.ActionPerformerInterface, error)` where error is always `nil`

**Side Effects:**
- **Success case:** Commits database transaction, destroys cache, triggers system restart
- **Failure case:** Rolls back database transaction