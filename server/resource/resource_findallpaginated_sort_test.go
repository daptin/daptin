package resource

import (
	"reflect"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/table_info"
)

func TestValidateSortOrder(t *testing.T) {
	columns := []api2go.ColumnInfo{
		{Name: "created_at", ColumnName: "created_at"},
		{Name: "display_name", ColumnName: "name"},
		{Name: "reference_id", ColumnName: "reference_id"},
	}
	model := api2go.NewApi2GoModel("world", columns, 0, nil)
	dbResource := &DbResource{
		model: model,
		tableInfo: &table_info.TableInfo{
			TableName: "world",
			Columns:   columns,
		},
	}

	tests := []struct {
		name      string
		sortOrder []string
		want      []string
		wantError bool
	}{
		{name: "column", sortOrder: []string{"created_at"}, want: []string{"created_at"}},
		{name: "ascending column", sortOrder: []string{"+created_at"}, want: []string{"+created_at"}},
		{name: "descending column", sortOrder: []string{"-created_at"}, want: []string{"-created_at"}},
		{name: "multiple columns", sortOrder: []string{"reference_id", "-created_at"}, want: []string{"reference_id", "-created_at"}},
		{name: "api column name", sortOrder: []string{"display_name"}, want: []string{"name"}},
		{name: "descending api column name", sortOrder: []string{"-display_name"}, want: []string{"-name"}},
		{name: "physical column name", sortOrder: []string{"name"}, want: []string{"name"}},
		{name: "unknown column", sortOrder: []string{"password"}, wantError: true},
		{name: "empty column", sortOrder: []string{""}, want: []string{}},
		{name: "empty and valid column", sortOrder: []string{"", "created_at"}, want: []string{"created_at"}},
		{name: "ascending direction only", sortOrder: []string{"+"}, wantError: true},
		{name: "descending direction only", sortOrder: []string{"-"}, wantError: true},
		{name: "backtick breakout", sortOrder: []string{"id`,(select password from user_account limit 1)"}, wantError: true},
		{name: "double quote breakout", sortOrder: []string{"email\" --"}, wantError: true},
		{name: "sql comment", sortOrder: []string{"created_at --"}, wantError: true},
		{name: "function", sortOrder: []string{"random()"}, wantError: true},
		{name: "qualified column", sortOrder: []string{"user_account.password"}, wantError: true},
		{name: "leading whitespace", sortOrder: []string{" created_at"}, wantError: true},
		{name: "trailing whitespace", sortOrder: []string{"created_at "}, wantError: true},
		{name: "one invalid column", sortOrder: []string{"created_at", "password"}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := dbResource.validateSortOrder(test.sortOrder)
			if test.wantError {
				if err == nil {
					t.Fatalf("validateSortOrder(%q) returned no error", test.sortOrder)
				}
				if got != nil {
					t.Fatalf("validateSortOrder(%q) = %q, want nil", test.sortOrder, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("validateSortOrder(%q): %v", test.sortOrder, err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("validateSortOrder(%q) = %q, want %q", test.sortOrder, got, test.want)
			}
		})
	}
}
