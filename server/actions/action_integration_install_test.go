package actions

import (
	"reflect"
	"testing"

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
