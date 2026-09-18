package resource

import (
	"reflect"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/graphql-go/graphql/language/ast"
)

func TestJSONColumnStorageValueCanonicalizesObjectsAndArrays(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{name: "object", value: map[string]interface{}{"source": "test", "rank": float64(1)}, want: `{"rank":1,"source":"test"}`},
		{name: "array", value: []interface{}{map[string]interface{}{"id": float64(1)}, "two"}, want: `[{"id":1},"two"]`},
		{name: "encoded object", value: ` { "source": "test" } `, want: `{"source":"test"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := jsonColumnStorageValue(test.value)
			if err != nil {
				t.Fatalf("jsonColumnStorageValue() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("jsonColumnStorageValue() = %v, want %s", got, test.want)
			}
		})
	}
}

func TestJSONColumnStorageValueRejectsInvalidAndScalarValues(t *testing.T) {
	for _, value := range []interface{}{`{"unterminated":`, `"scalar"`, 42, true} {
		if _, err := jsonColumnStorageValue(value); err == nil {
			t.Fatalf("jsonColumnStorageValue(%#v) succeeded, want error", value)
		}
	}
}

func TestPublicJSONColumnDataDecodesWithoutMutatingDatabaseRow(t *testing.T) {
	row := map[string]interface{}{
		"metadata": `{"source":"test","nested":[1,true]}`,
		"name":     "probe",
	}
	columns := []api2go.ColumnInfo{
		{ColumnName: "metadata", ColumnType: "json"},
		{ColumnName: "name", ColumnType: "label"},
	}

	got, err := publicJSONColumnData(row, columns)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"metadata": map[string]interface{}{"source": "test", "nested": []interface{}{float64(1), true}},
		"name":     "probe",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("publicJSONColumnData() = %#v, want %#v", got, want)
	}
	if row["metadata"] != `{"source":"test","nested":[1,true]}` {
		t.Fatalf("database row was mutated: %#v", row)
	}
}

func TestGraphQLJSONLiteralPreservesStructure(t *testing.T) {
	literal := ast.NewObjectValue(&ast.ObjectValue{Fields: []*ast.ObjectField{
		ast.NewObjectField(&ast.ObjectField{
			Name: ast.NewName(&ast.Name{Value: "items"}),
			Value: ast.NewListValue(&ast.ListValue{Values: []ast.Value{
				ast.NewIntValue(&ast.IntValue{Value: "1"}),
				ast.NewBooleanValue(&ast.BooleanValue{Value: true}),
			}}),
		}),
	}})

	want := map[string]interface{}{"items": []interface{}{int64(1), true}}
	if got := graphqlJSONLiteral(literal); !reflect.DeepEqual(got, want) {
		t.Fatalf("graphqlJSONLiteral() = %#v, want %#v", got, want)
	}
}
