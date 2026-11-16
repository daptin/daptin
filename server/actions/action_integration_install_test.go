package actions

import (
	stdjson "encoding/json"
	"reflect"
	"testing"

	"github.com/daptin/daptin/server/resource"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestGetBodyParameterNamesHandlesRootOneOf(t *testing.T) {
	schema := &openapi3.Schema{
		OneOf: openapi3.SchemaRefs{
			{Value: openapi3.NewObjectSchema().WithProperty("labels", openapi3.NewArraySchema().WithItems(openapi3.NewStringSchema()))},
			{Value: openapi3.NewArraySchema().WithItems(openapi3.NewStringSchema())},
		},
	}

	names, err := GetBodyParameterNames(ModeRequest, "", schema)
	if err != nil {
		t.Fatalf("GetBodyParameterNames returned error: %v", err)
	}

	assertSameStrings(t, names, []string{"labels", "body"})
}

func TestGetBodyParameterNamesHandlesNamedOneOfFreeFormProperty(t *testing.T) {
	schema := openapi3.NewObjectSchema().WithProperty("payload", &openapi3.Schema{
		OneOf: openapi3.SchemaRefs{
			{Value: openapi3.NewObjectSchema()},
			{Value: openapi3.NewStringSchema()},
		},
	})

	names, err := GetBodyParameterNames(ModeRequest, "", schema)
	if err != nil {
		t.Fatalf("GetBodyParameterNames returned error: %v", err)
	}

	assertSameStrings(t, names, []string{"payload"})
}

func TestGetBodyParameterNamesHandlesRootAnyOf(t *testing.T) {
	schema := &openapi3.Schema{
		AnyOf: openapi3.SchemaRefs{
			{Value: openapi3.NewObjectSchema().WithProperty("environment", openapi3.NewStringSchema())},
			{Value: openapi3.NewObjectSchema().WithProperty("state", openapi3.NewStringSchema())},
		},
	}

	names, err := GetBodyParameterNames(ModeRequest, "", schema)
	if err != nil {
		t.Fatalf("GetBodyParameterNames returned error: %v", err)
	}

	assertSameStrings(t, names, []string{"environment", "state"})
}

func TestGetBodyParameterNamesHandlesAllOfMerge(t *testing.T) {
	schema := &openapi3.Schema{
		AllOf: openapi3.SchemaRefs{
			{Value: openapi3.NewObjectSchema().WithProperty("title", openapi3.NewStringSchema())},
			{Value: openapi3.NewObjectSchema().WithProperty("head", openapi3.NewStringSchema())},
		},
	}

	names, err := GetBodyParameterNames(ModeRequest, "", schema)
	if err != nil {
		t.Fatalf("GetBodyParameterNames returned error: %v", err)
	}

	assertSameStrings(t, names, []string{"title", "head"})
}

func TestGetBodyParameterNamesUsesBodyForFreeFormRoot(t *testing.T) {
	schema := openapi3.NewObjectSchema()
	names, err := GetBodyParameterNames(ModeRequest, "", schema)
	if err != nil {
		t.Fatalf("GetBodyParameterNames returned error: %v", err)
	}

	assertSameStrings(t, names, []string{"body"})
}

func TestGetBodyParameterNamesFromSchemaRefIgnoresOptionalMissingSchema(t *testing.T) {
	names, err := GetBodyParameterNamesFromSchemaRef(ModeRequest, false, nil)
	if err != nil {
		t.Fatalf("GetBodyParameterNamesFromSchemaRef returned error: %v", err)
	}

	assertSameStrings(t, names, []string{})
}

func TestGetBodyParameterNamesFromSchemaRefUsesBodyForRequiredMissingSchema(t *testing.T) {
	names, err := GetBodyParameterNamesFromSchemaRef(ModeRequest, true, nil)
	if err != nil {
		t.Fatalf("GetBodyParameterNamesFromSchemaRef returned error: %v", err)
	}

	assertSameStrings(t, names, []string{"body"})
}

func TestGetBodyParameterNamesFromSchemaRefRejectsUnresolvedRef(t *testing.T) {
	_, err := GetBodyParameterNamesFromSchemaRef(ModeRequest, false, &openapi3.SchemaRef{Ref: "#/components/schemas/Missing"})
	if err == nil {
		t.Fatalf("expected unresolved schema ref error")
	}
}

func TestRequiredIntegrationBodyParametersUsesOpenAPIRequiredFields(t *testing.T) {
	schema := openapi3.NewObjectSchema().
		WithProperty("required_value", openapi3.NewStringSchema()).
		WithProperty("optional_value", openapi3.NewStringSchema())
	schema.Required = []string{"required_value"}
	required := requiredIntegrationBodyParameters(true, schema.NewRef())
	if !required["required_value"] || required["optional_value"] || required["body"] {
		t.Fatalf("unexpected required body fields: %#v", required)
	}
	if optional := requiredIntegrationBodyParameters(false, schema.NewRef()); len(optional) != 0 {
		t.Fatalf("optional request body produced required fields: %#v", optional)
	}
}

func TestGetBodyParameterNamesRejectsInvalidCompositionBranch(t *testing.T) {
	schema := &openapi3.Schema{
		OneOf: openapi3.SchemaRefs{nil},
	}

	_, err := GetBodyParameterNames(ModeRequest, "", schema)
	if err == nil {
		t.Fatalf("expected invalid composition branch error")
	}
}

func assertSameStrings(t *testing.T, actual []string, expected []string) {
	t.Helper()
	if !reflect.DeepEqual(stringSet(actual), stringSet(expected)) {
		t.Fatalf("unexpected names: got %#v want %#v", actual, expected)
	}
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func TestIntegrationOperationMembershipPolicy(t *testing.T) {
	for _, test := range []struct {
		name                    string
		value                   interface{}
		present, guard, invalid bool
	}{
		{name: "absent"},
		{name: "false", value: false, present: true},
		{name: "true", value: true, present: true, guard: true},
		{name: "raw true", value: stdjson.RawMessage(`true`), present: true, guard: true},
		{name: "bytes false", value: []byte(`false`), present: true},
		{name: "string", value: "true", present: true, invalid: true},
		{name: "encoded string", value: stdjson.RawMessage(`"true"`), present: true, invalid: true},
		{name: "null", present: true, invalid: true},
		{name: "number", value: 1, present: true, invalid: true},
		{name: "object", value: map[string]interface{}{}, present: true, invalid: true},
		{name: "malformed", value: stdjson.RawMessage(`tru`), present: true, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			operation := &openapi3.Operation{}
			if test.present {
				operation.Extensions = map[string]interface{}{"x-daptin-require-membership": test.value}
			}
			attrs := map[string]interface{}{"body": "~body"}
			outcomes, err := integrationOperationOutcomes("provider", "invoke", attrs, operation)
			if test.invalid {
				if err == nil {
					t.Fatal("invalid policy accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			count := 1
			if test.guard {
				count = 2
			}
			if len(outcomes) != count {
				t.Fatalf("outcomes=%#v", outcomes)
			}
			if test.guard {
				guard := outcomes[0]
				if guard.Type != resource.RequireCurrentMembershipActionName || guard.Method != "EXECUTE" || !guard.SkipInResponse || guard.Condition != "" || len(guard.Attributes) != 0 {
					t.Fatalf("unexpected guard: %#v", guard)
				}
			}
			provider := outcomes[len(outcomes)-1]
			if provider.Type != "provider" || provider.Method != "invoke" || !reflect.DeepEqual(provider.Attributes, attrs) {
				t.Fatalf("provider changed: %#v", provider)
			}
		})
	}
}
