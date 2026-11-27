package agent

import (
	"encoding/json"
	"testing"

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
