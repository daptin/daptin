package server

import (
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/graphql-go/graphql"
)

func TestActionGraphQLArgumentsUseInstalledActionFields(t *testing.T) {
	resource.InitialiseColumnManager()
	action := actionresponse.Action{InFields: []api2go.ColumnInfo{
		{ColumnName: "credential_id", ColumnType: "hidden", IsNullable: false},
		{ColumnName: "required-value", ColumnType: "label", IsNullable: false},
		{ColumnName: "optional_value", ColumnType: "label", IsNullable: true},
	}}

	args, inputArgs := actionGraphQLArguments(action)
	if len(args) != 3 || len(inputArgs) != 3 {
		t.Fatalf("action arguments = %d/%d, want 3/3", len(args), len(inputArgs))
	}
	if _, ok := args["credential_id"].Type.(*graphql.NonNull); !ok {
		t.Fatalf("credential_id type = %T, want non-null", args["credential_id"].Type)
	}
	if _, ok := args["required_value"].Type.(*graphql.NonNull); !ok {
		t.Fatalf("required_value type = %T, want non-null", args["required_value"].Type)
	}
	if _, ok := args["optional_value"].Type.(*graphql.NonNull); ok {
		t.Fatal("optional_value unexpectedly became non-null")
	}

	input := actionGraphQLInput(map[string]interface{}{
		"credential_id":  "credential-reference",
		"required_value": "value",
	}, inputArgs)
	if input["credential_id"] != "credential-reference" || input["required-value"] != "value" {
		t.Fatalf("mapped action input = %#v", input)
	}
}
