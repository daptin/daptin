package actionresponse

import (
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/columns"
	"github.com/daptin/daptin/server/id"
	"github.com/jmoiron/sqlx"
)

type ActionResponse struct {
	ResponseType string
	Attributes   interface{}
}

type ActionRequest struct {
	Type          string
	Action        string
	Attributes    map[string]interface{}
	RawBodyBytes  []byte
	RawBodyString string
}

type ActionPerformerInterface interface {
	DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error)
	Name() string
}

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
	Type            string
	Method          string // method name
	Reference       string
	LogToConsole    bool
	SkipInResponse  bool
	Condition       string
	Attributes      map[string]interface{}
	ContinueOnError bool
}

// Action is a set of `Outcome` based on set of Input values on a particular data type
// New actions can be defined and added using JSON or YAML files
// Actions are stored and reloaded from the `action` table of the storage
type Action struct {
	Name                    string // Name of the action
	Label                   string
	OnType                  string
	InstanceOptional        bool
	RequestSubjectRelations []string
	ReferenceId             daptinid.DaptinReferenceId
	InFields                []api2go.ColumnInfo
	OutFields               []Outcome
	Validations             []columns.ColumnTag
	Conformations           []columns.ColumnTag
}
