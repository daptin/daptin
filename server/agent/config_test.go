package agent

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/llmgateway/contract"
)

func TestToolsetRejectsUnknownOperationsAndSchemaKeywords(t *testing.T) {
	for _, config := range []Config{
		{Model: "test", Toolset: Toolset{Operations: []string{"sql"}}},
		{Model: "test", Toolset: Toolset{Tools: []Binding{{Name: "send", Entity: "item", Action: "send", Parameters: json.RawMessage(`{"type":"object","$ref":"https://example.test/schema"}`)}}}},
		{Model: "test", Toolset: Toolset{Tools: []Binding{{Name: "daptin_create", Entity: "item", Action: "send", Parameters: json.RawMessage(`{"type":"object"}`)}}}},
	} {
		if err := config.validate(); err == nil {
			t.Fatalf("accepted invalid configuration %#v", config)
		}
	}
}
func TestToolBoundaryValidatesEntireDeclaredShape(t *testing.T) {
	config := Config{Model: "test", Toolset: Toolset{Resources: []string{"*"}, Operations: []string{"read", "update"}}}
	if err := config.validate(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, args string
		valid      bool
	}{
		{"daptin_read", `{"entity":"item","reference":"00000000-0000-4000-8000-000000000001"}`, true},
		{"daptin_read", `{"entity":"item"}`, false},
		{"daptin_read", `{"entity":"item","reference":42}`, false},
		{"daptin_read", `{"entity":"item","reference":"00000000-0000-4000-8000-000000000001","sessionUser":{}}`, false},
		{"daptin_delete", `{"entity":"item","reference":"00000000-0000-4000-8000-000000000001"}`, false},
		{"daptin_read", `[]`, false},
		{"daptin_read", `{"entity":"item","reference":"01a217d4-fb28-7776-8876-887a-4ebef164e8b3"}`, false},
		{"daptin_update", `{"entity":"item","reference":"00000000-0000-4000-8000-000000000001","attributes":{"version":"1"}}`, false},
		{"daptin_update", `{"entity":"item","reference":"00000000-0000-4000-8000-000000000001","attributes":{"version":1}}`, true},
		{"daptin_update", `{"entity":"item","reference":"00000000-0000-4000-8000-000000000001","attributes":{"version":1.5}}`, false},
		{"daptin_update", `{"entity":"item","reference":"00000000-0000-4000-8000-000000000001","attributes":{"version":0}}`, false},
	} {
		call := contract.ToolCall{ID: "call-1", Type: "function", Function: contract.FunctionCall{Name: test.name, Arguments: test.args}}
		_, err := validateCall(config, call)
		if (err == nil) != test.valid {
			t.Errorf("%s %s: %v", test.name, test.args, err)
		}
	}
}

func TestOptionalToolInputsUseProviderValidSchema(t *testing.T) {
	config := Config{Toolset: Toolset{Resources: []string{"artifact"}}}
	for _, tool := range tools(config) {
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.Function.Parameters, &schema); err != nil {
			t.Fatal(err)
		}
		if _, ok := schema["required"].([]interface{}); !ok {
			t.Fatalf("%s required must be an array, got %v", tool.Function.Name, schema["required"])
		}
	}
}
func TestMutationIsNotRepeatableByDefault(t *testing.T) {
	config := Config{Toolset: Toolset{Tools: []Binding{{Name: "search", Repeatable: true}, {Name: "send", IdempotencyInput: "operation"}}}}
	for _, name := range []string{"daptin_read", "search", "send"} {
		if !repeatable(config, name) {
			t.Errorf("%s should be repeatable", name)
		}
	}
	for _, name := range []string{"daptin_create", "daptin_update", "daptin_delete", "daptin_action"} {
		if repeatable(config, name) {
			t.Errorf("%s must require reconciliation", name)
		}
	}
}

func TestResourcePaginationBoundsBothNativeStyles(t *testing.T) {
	for _, raw := range []string{"0", "101", "-1", "1.5", "9999999999999999999999"} {
		if _, err := boundedPageSize(raw); err == nil {
			t.Errorf("accepted page size %s", raw)
		}
	}
	for _, raw := range []string{"", "1", "100"} {
		if _, err := boundedPageSize(raw); err != nil {
			t.Errorf("rejected page size %s: %v", raw, err)
		}
	}
}

func TestDuplicateOperationsAndPagedTools(t *testing.T) {
	config := Config{Model: "test", Toolset: Toolset{Resources: []string{"*"}, Operations: []string{"read", "read"}}}
	if err := config.validate(); err == nil {
		t.Fatal("duplicate operations accepted")
	}
	config.Toolset.Operations = []string{"relations"}
	for _, name := range []string{"daptin_discover", "daptin_relations"} {
		arguments := `{"page":2}`
		if name == "daptin_relations" {
			arguments = `{"entity":"item","reference":"00000000-0000-4000-8000-000000000001","relation":"usergroup","page":2}`
		}
		args, err := validateCall(config, contract.ToolCall{ID: "page", Type: "function", Function: contract.FunctionCall{Name: name, Arguments: arguments}})
		if err != nil || toolPage(args) != 2 {
			t.Fatalf("%s pagination: %v %v", name, args, err)
		}
	}
}
func TestResultEncodingRetainsLargeValuesAndRejectsInvalidJSON(t *testing.T) {
	large := strings.Repeat("x", 5<<20)
	body, err := encode(map[string]interface{}{"value": large})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := decode(body, &result); err != nil || result["value"] != large {
		t.Fatal("large result was not retained", err)
	}
	if _, err := encode(math.NaN()); err == nil {
		t.Fatal("invalid JSON accepted")
	}
	cycle := map[string]interface{}{}
	cycle["self"] = cycle
	if _, err := encode(cycle); err == nil {
		t.Fatal("cyclic result accepted")
	}
}

func TestExecutionContainsPanics(t *testing.T) {
	service := &Service{}
	_, err := service.execute(context.Background(), nil, &runState{}, intent{Kind: "model"})
	if err == nil || !strings.Contains(err.Error(), "execution panic") {
		t.Fatalf("panic escaped execution boundary: %v", err)
	}
}

func TestRelationshipResultsRetainResourceAttributes(t *testing.T) {
	model := api2go.NewApi2GoModelWithData("item", nil, 0, nil, map[string]interface{}{"name": "related", "reference_id": "00000000-0000-4000-8000-000000000001"})
	result := publicResult(api2go.Response{Res: []api2go.Api2GoModel{model}}).([]map[string]interface{})
	if len(result) != 1 || result[0]["name"] != "related" {
		t.Fatalf("resource attributes lost: %#v", result)
	}
}
