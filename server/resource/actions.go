package resource

import (
	"github.com/artpar/api2go"
)

type Outcome struct {
	Type           string
	Method         string
	Reference      string
	SkipInResponse bool
	Condition      string
	Attributes     map[string]interface{}
}

type Action struct {
	Name             string
	Label            string
	OnType           string
	InstanceOptional bool
	ReferenceId      string
	InFields         []api2go.ColumnInfo
	OutFields        []Outcome
	Validations      []ColumnTag
	Conformations    []ColumnTag
}

type ActionRow struct {
	Name             string
	Label            string
	OnType           string
	InstanceOptional bool   `db:"instance_optional"`
	ReferenceId      string
	ActionSchema     string `db:"action_schema"`
}

type ActionRequest struct {
	Type       string
	Action     string
	Attributes map[string]interface{}
}
