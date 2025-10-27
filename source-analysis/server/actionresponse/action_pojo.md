# action_pojo.go

**File:** server/actionresponse/action_pojo.go

## Code Summary

This file contains only type definitions - no executable code.

### Type: ActionResponse (lines 10-13)
**Fields:**
- `ResponseType string` - identifies the type of response
- `Attributes interface{}` - holds the actual response data

### Type: ActionRequest (lines 15-21)
**Fields:**
- `Type string` - entity name the action operates on
- `Action string` - name of action to execute
- `Attributes map[string]interface{}` - input parameters for the action
- `RawBodyBytes []byte` - raw request body as bytes
- `RawBodyString string` - raw request body as string

### Interface: ActionPerformerInterface (lines 23-26)
**Required Methods:**
- `DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error)`
  - Input: Outcome struct, input fields map, database transaction
  - Output: API responder, array of action responses, array of errors
- `Name() string`
  - Output: string name identifying the performer

### Type: Outcome (lines 37-46)
**Fields:**
- `Type string` - entity type to operate on
- `Method string` - operation method (GET/PUT/POST/DELETE/UPDATE/PATCH/EXECUTE/INTEGRATION)
- `Reference string` - name for referencing this outcome in later steps
- `LogToConsole bool` - whether to log execution to console
- `SkipInResponse bool` - whether to exclude from HTTP response
- `Condition string` - JavaScript condition string for conditional execution
- `Attributes map[string]interface{}` - parameters for outcome execution
- `ContinueOnError bool` - whether to continue processing if this outcome fails

### Type: Action (lines 51-62)
**Fields:**
- `Name string` - action identifier
- `Label string` - human-readable description
- `OnType string` - entity type this action operates on
- `InstanceOptional bool` - whether reference_id parameter is required
- `RequestSubjectRelations []string` - relations to fetch with subject entity
- `ReferenceId daptinid.DaptinReferenceId` - UUID identifier for this action
- `InFields []api2go.ColumnInfo` - input field definitions
- `OutFields []Outcome` - array of outcomes to execute sequentially
- `Validations []columns.ColumnTag` - validation rules for inputs
- `Conformations []columns.ColumnTag` - confirmation rules

**Data Flow:**
- Actions contain array of Outcomes in `OutFields`
- Each Outcome can reference previous outcomes by `Reference` name
- JavaScript evaluation supported in `Condition` and `Attributes`
- Actions are stored in database `action` table