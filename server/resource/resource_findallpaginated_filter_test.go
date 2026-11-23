package resource

import (
	"database/sql"
	"net/http"
	"strings"
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

func TestAddFiltersRejectsUnknownOperators(t *testing.T) {
	columns := []api2go.ColumnInfo{
		{ColumnName: "name"},
		{ColumnName: "reference_id"},
		{ColumnName: "parent_id", IsForeignKey: true, ForeignKeyData: api2go.ForeignKeyData{Namespace: "parent"}},
	}
	resource := &DbResource{
		model:     api2go.NewApi2GoModel("filter_probe", columns, 0, nil),
		tableInfo: &table_info.TableInfo{TableName: "filter_probe", Columns: columns},
	}
	for _, operator := range []string{"#", "", "unknown", "is invalid", "not invalid", "EQ", " eq", "eq ", "eq; DROP TABLE filter_probe"} {
		for _, column := range []string{"name", "reference_id", "parent_id"} {
			for _, group := range []string{"", "group1"} {
				t.Run(operator+"/"+column+"/"+group, func(t *testing.T) {
					// A valid UUID with no transaction proves rejection precedes FK lookup.
					list, count, err := resource.addFilters(goqu.From("filter_probe"), goqu.From("filter_probe"),
						[]Query{{ColumnName: column, Operator: operator, Value: "12345678-1234-1234-1234-123456789abc", LogicalGroup: group}}, "filter_probe.", nil)
					httpError, ok := err.(api2go.HTTPError)
					if !ok || httpError.Status() != http.StatusBadRequest {
						t.Fatalf("expected HTTP 400, got %T: %v", err, err)
					}
					if list != nil || count != nil {
						t.Fatal("invalid operator returned query builders")
					}
				})
			}
		}
	}
}

func TestAddFiltersSupportedOperators(t *testing.T) {
	columns := []api2go.ColumnInfo{{ColumnName: "name"}}
	resource := &DbResource{
		model:     api2go.NewApi2GoModel("filter_probe", columns, 0, nil),
		tableInfo: &table_info.TableInfo{TableName: "filter_probe", Columns: columns},
	}
	for _, test := range []struct {
		operators []string
		value     interface{}
		predicate string
	}{
		{[]string{"=", "is", "eq"}, "value", `= 'value'`},
		{[]string{"not", "neq"}, "value", `!= 'value'`},
		{[]string{"is not"}, "value", `IS NOT 'value'`},
		{[]string{"gt", "more than", "after"}, 10, `> 10`},
		{[]string{"lt", "less than", "before"}, 10, `< 10`},
		{[]string{"contains", "like", "begins with", "ends with"}, "%value%", `LIKE '%value%'`},
		{[]string{"ilike"}, "%value%", `ILIKE '%value%'`},
		{[]string{"not contains", "not like"}, "%value%", `NOT LIKE '%value%'`},
		{[]string{"not ilike"}, "%value%", `NOT ILIKE '%value%'`},
		{[]string{"in", "any of"}, []interface{}{"a", "b"}, `IN ('a', 'b')`},
		{[]string{"none of"}, []interface{}{"a", "b"}, `NOT IN ('a', 'b')`},
		{[]string{"is true"}, "ignored", `IS TRUE`},
		{[]string{"is false"}, "ignored", `IS FALSE`},
		{[]string{"is empty"}, "ignored", `IS NULL`},
		{[]string{"is"}, nil, `IS NULL`},
		{[]string{"contains_literal"}, "value", `LIKE '%value%' ESCAPE '!'`},
	} {
		for _, operator := range test.operators {
			t.Run(operator+"/"+test.predicate, func(t *testing.T) {
				list, count, err := resource.addFilters(goqu.From("filter_probe"), goqu.From("filter_probe"),
					[]Query{{ColumnName: "name", Operator: operator, Value: test.value}}, "filter_probe.", nil)
				if err != nil {
					t.Fatal(err)
				}
				for _, builder := range []*goqu.SelectDataset{list, count} {
					statement, _, err := builder.ToSQL()
					if err != nil || !strings.Contains(statement, test.predicate) {
						t.Fatalf("operator %q: SQL = %s, error = %v; want %s", operator, statement, err, test.predicate)
					}
				}
			})
		}
	}
}
