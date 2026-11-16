package resource

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/daptin/daptin/server/table_info"
	"github.com/doug-martin/goqu/v9"
	_ "github.com/mattn/go-sqlite3"
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

func TestLiteralSubstringFilterSharesScopeAndCount(t *testing.T) {
	columns := []api2go.ColumnInfo{{ColumnName: "subject"}, {ColumnName: "box"}}
	resource := &DbResource{
		model:     api2go.NewApi2GoModel("filter_probe", columns, 0, nil),
		tableInfo: &table_info.TableInfo{TableName: "filter_probe", Columns: columns},
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE filter_probe (subject TEXT, box TEXT)"); err != nil {
		t.Fatal(err)
	}
	term := "50%_![Q1] \\ 日本語 ' OR 1=1 --"
	match := "Budget " + term
	for _, row := range [][2]string{{match, "inbox"}, {match, "archive"}, {"Budget 500X!", "inbox"}} {
		if _, err := db.Exec("INSERT INTO filter_probe VALUES (?, ?)", row[0], row[1]); err != nil {
			t.Fatal(err)
		}
	}
	dialect := goqu.Dialect("sqlite3")
	for _, filter := range []struct {
		operator, value string
		want            int
	}{
		{"contains_literal", term, 1},
		{"contains_literal", "%_", 1},
		{"contains", "%50%_%", 2},
	} {
		t.Run(filter.operator+filter.value, func(t *testing.T) {
			list, count, err := resource.addFilters(
				dialect.From("filter_probe").Prepared(true),
				dialect.From("filter_probe").Select(goqu.COUNT("*")).Prepared(true),
				[]Query{{ColumnName: "box", Operator: "is", Value: "inbox"},
					{ColumnName: "subject", Operator: filter.operator, Value: filter.value}}, "filter_probe.", nil)
			if err != nil {
				t.Fatal(err)
			}
			statement, args, err := count.ToSQL()
			if err != nil {
				t.Fatal(err)
			}
			var total int
			if err := db.QueryRow(statement, args...).Scan(&total); err != nil || total != filter.want {
				t.Fatalf("count=%d, need %d: %v", total, filter.want, err)
			}
			statement, args, err = list.Select("subject").Order(goqu.I("subject").Asc()).ToSQL()
			if err != nil {
				t.Fatal(err)
			}
			var subject string
			if err := db.QueryRow(statement, args...).Scan(&subject); err != nil || subject != match {
				t.Fatalf("subject=%q, need %q: %v", subject, match, err)
			}
		})
	}
}
