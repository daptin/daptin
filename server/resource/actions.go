package resource

import (
	"bytes"
	"encoding/binary"
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
	Type            string
	Method          string // method name
	Reference       string
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
	ReferenceId             string
	InFields                []api2go.ColumnInfo
	OutFields               []Outcome
	Validations             []ColumnTag
	Conformations           []ColumnTag
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

// MarshalBinary encodes the struct into binary format manually
func (e ActionRow) MarshalBinary() (data []byte, err error) {
	buffer := new(bytes.Buffer)

	// Encode Name
	if err := encodeString(buffer, e.Name); err != nil {
		return nil, err
	}

	// Encode Label
	if err := encodeString(buffer, e.Label); err != nil {
		return nil, err
	}

	// Encode OnType
	if err := encodeString(buffer, e.OnType); err != nil {
		return nil, err
	}

	// Encode InstanceOptional
	if err := binary.Write(buffer, binary.BigEndian, e.InstanceOptional); err != nil {
		return nil, err
	}

	// Encode ReferenceId
	if err := encodeString(buffer, e.ReferenceId); err != nil {
		return nil, err
	}

	// Encode ActionSchema
	if err := encodeString(buffer, e.ActionSchema); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// UnmarshalBinary decodes the data into the struct using manual binary decoding
func (e ActionRow) UnmarshalBinary(data []byte) error {
	buffer := bytes.NewBuffer(data)

	// Decode Name
	if name, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.Name = name
	}

	// Decode Label
	if label, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.Label = label
	}

	// Decode OnType
	if onType, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.OnType = onType
	}

	// Decode InstanceOptional
	if err := binary.Read(buffer, binary.BigEndian, &e.InstanceOptional); err != nil {
		return err
	}

	// Decode ReferenceId
	if referenceId, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.ReferenceId = referenceId
	}

	// Decode ActionSchema
	if actionSchema, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.ActionSchema = actionSchema
	}

	return nil
}

type ActionRequest struct {
	Type       string
	Action     string
	Attributes map[string]interface{}
}
