package resource

import (
	"net/http"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/daptin/daptin/server/table_info"
)

func TestAddFiltersRejectsUnknownColumns(t *testing.T) {
	columns := []api2go.ColumnInfo{
		{Name: "name", ColumnName: "name"},
		{Name: "reference_id", ColumnName: "reference_id"},
	}
	model := api2go.NewApi2GoModel("filter_probe", columns, 0, nil)
	dbResource := &DbResource{
		model: model,
		tableInfo: &table_info.TableInfo{
			TableName: "filter_probe",
			Columns:   columns,
		},
	}

	tests := []struct {
		name    string
		queries []Query
	}{
		{
			name:    "ordinary filter",
			queries: []Query{{ColumnName: "does_not_exist", Operator: "is", Value: "value"}},
		},
		{
			name:    "logical group filter",
			queries: []Query{{ColumnName: "does_not_exist", Operator: "is", Value: "value", LogicalGroup: "group1"}},
		},
		{
			name:    "fuzzy filter",
			queries: []Query{{ColumnName: "does_not_exist", Operator: "fuzzy", Value: "value"}},
		},
		{
			name:    "mixed fuzzy columns",
			queries: []Query{{ColumnName: "name,does_not_exist", Operator: "fuzzy", Value: "value"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queryBuilder := statementbuilder.Squirrel.Select("id").From("filter_probe")
			countQueryBuilder := statementbuilder.Squirrel.Select("id").From("filter_probe")
			queryBuilder, countQueryBuilder, err := dbResource.addFilters(
				queryBuilder, countQueryBuilder, test.queries, "filter_probe.", nil,
			)
			if err == nil {
				t.Fatal("addFilters returned no error")
			}
			if queryBuilder != nil || countQueryBuilder != nil {
				t.Fatalf("addFilters returned builders for invalid input: %#v, %#v", queryBuilder, countQueryBuilder)
			}

			httpError, ok := err.(api2go.HTTPError)
			if !ok {
				t.Fatalf("addFilters error type = %T, want api2go.HTTPError", err)
			}
			if httpError.Status() != http.StatusBadRequest {
				t.Fatalf("addFilters status = %d, want %d", httpError.Status(), http.StatusBadRequest)
			}
		})
	}
}
