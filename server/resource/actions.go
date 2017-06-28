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
  Name        string              `json:"name"`
  Label       string              `json:"label"`
  OnType      string              `json:"onType"`
  ReferenceId string              `json:"reference_id"`
  InFields    []api2go.ColumnInfo `json:"fields"`
  OutFields   []Outcome           `json:"outcomes"`
}

type ActionRow struct {
  Name        string `json:"name"`
  Label       string `json:"label"`
  OnType      string `json:"onType"`
  ReferenceId string `json:"reference_id"`
  InFields    string `json:"fields"`
  OutFields   string `json:"outcomes"`
}

type ActionRequest struct {
  Type       string
  Action     string
  Attributes map[string]interface{}
}
