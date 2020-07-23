package resource

import (
	"github.com/artpar/api2go"
)

// Outcome is call to a internal function with attributes as parameters
// Outcome has a particular `type`, it can be one of the data entities already defined
// Method is the type of outcome: GET/PUT/POST/DELETE/UPDATE/PATCH/EXECUTE/INTEGRATION
// Condition can be specified in JS to be checked, false condition will skip processing the outcome
// set SkipInResponse to true to not include action outcome in the http response of the action call
// reference is a name you can assign to the outcome of, which can be used in furthur chained outcomes
// Attributes is a map of string to interface{} which will be used by the action
// The attributes are evaluated to generate the actual data to be sent to execution
// JS scripting can be used to reference existing outcomes by reference names
type Outcome struct {
	Type           string
	Method         string // method name
	Reference      string
	SkipInResponse bool
	Condition      string
	Attributes     map[string]interface{}
}

// Action is a set of `Outcome` based on set of Input values on a particular data type
// New actions can be defined and added using JSON or YAML files
// Actions are stored and reloaded from the `action` table of the storage
type Action struct {
	Name             string // Name of the action
	Label            string
	OnType           string
	InstanceOptional bool
	ReferenceId      string
	InFields         []api2go.ColumnInfo
	OutFields        []Outcome
	Validations      []ColumnTag
	Conformations    []ColumnTag
}

// ActionRow represents an action instance on the database
// Can be retrieved using GetActionByName
type ActionRow struct {
	Name             string
	Label            string
	OnType           string
	InstanceOptional bool `db:"instance_optional"`
	ReferenceId      string
	ActionSchema     string `db:"action_schema"`
}

type ActionRequest struct {
	Type       string
	Action     string
	Attributes map[string]interface{}
}
