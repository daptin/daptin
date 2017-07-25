package resource

import (
	"github.com/artpar/api2go"
)

type Outcome struct {
	Type       string `json:"type"`
	Method     string `json:"method"`
	Reference  string
	Attributes map[string]interface{} `json:"attributes"`
}

type Action struct {
	Name             string              `json:"name"`
	Label            string              `json:"label"`
	OnType           string              `json:"onType"`
	InstanceOptional bool              `json:"instanceOptional"`
	ReferenceId      string              `json:"reference_id"`
	InFields         []api2go.ColumnInfo `json:"fields"`
	OutFields        []Outcome           `json:"outcomes"`
	Validations      []ColumnTag         `json:"validations"`
	Conformations    []ColumnTag         `json:"conformations"`
}

type ActionRow struct {
	Name             string `json:"name"`
	Label            string `json:"label"`
	OnType           string `json:"onType"`
	InstanceOptional bool   `db:"instance_optional",json:"instance_optional"`
	ReferenceId      string `json:"reference_id"`
	ActionSchema     string `db:"action_schema"`
}

type ActionRequest struct {
	Type       string
	Action     string
	Attributes map[string]interface{}
}
