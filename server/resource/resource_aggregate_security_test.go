package resource

import (
	"errors"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/table_info"
)

func aggregateSecurityTestResource() *DbResource {
	rootColumns := []api2go.ColumnInfo{
		{Name: "id", ColumnName: "id", ColumnType: "id", ExcludeFromApi: true},
		{Name: "email", ColumnName: "email"},
		{Name: "total", ColumnName: "total"},
		{Name: "customer_id", ColumnName: "customer_id"},
		{Name: "password", ColumnName: "password", ColumnType: "password"},
		{Name: "notes", ColumnName: "notes", ColumnType: "content"},
		{Name: "auth_version", ColumnName: "auth_version", ColumnType: "measurement", ExcludeFromApi: true},
		{Name: "shared", ColumnName: "shared", ColumnType: "label"},
	}
	joinedColumns := []api2go.ColumnInfo{
		{Name: "id", ColumnName: "id", ColumnType: "id", ExcludeFromApi: true},
		{Name: "name", ColumnName: "name"},
		{Name: "secret", ColumnName: "secret", ColumnType: "encrypted"},
		{Name: "shared", ColumnName: "shared", ColumnType: "password"},
	}
	root := &DbResource{tableInfo: &table_info.TableInfo{TableName: "orders", Columns: rootColumns}}
	joined := &DbResource{tableInfo: &table_info.TableInfo{TableName: "customer", Columns: joinedColumns}}
	root.Cruds = map[string]*DbResource{"orders": root, "customer": joined}
	joined.Cruds = root.Cruds
	return root
}

func TestAggregateRejectsRestrictedColumnTypes(t *testing.T) {
	resource := aggregateSecurityTestResource()
	blockedTypes := []string{
		"password", "bcrypt", "md5", "md5-bcrypt", "encrypted", "hidden",
		"content", "html", "markdown", "json", "gzip", "file", "file.csv", "image", "video",
	}
	for _, columnType := range blockedTypes {
		resource.tableInfo.Columns = append(resource.tableInfo.Columns, api2go.ColumnInfo{
			Name: "restricted", ColumnName: "restricted", ColumnType: columnType,
		})
		for _, ref := range []string{"restricted", "orders.restricted"} {
			if err := resource.validateColumnRef(ref, []string{"orders"}); err == nil {
				t.Errorf("%s column %q was accepted", columnType, ref)
			}
		}
		resource.tableInfo.Columns = resource.tableInfo.Columns[:len(resource.tableInfo.Columns)-1]
	}
	for _, ref := range []string{"id", "customer.id", "auth_version", "orders.auth_version", "customer.secret", "shared"} {
		if err := resource.validateColumnRef(ref, []string{"orders", "customer"}); err == nil {
			t.Errorf("restricted column %q was accepted", ref)
		}
	}
	for _, ref := range []string{"orders.id", "customer.id"} {
		if err := resource.validateJoinColumnRef(ref, []string{"orders", "customer"}); err != nil {
			t.Errorf("join key %q was rejected: %v", ref, err)
		}
	}
	for _, ref := range []string{"customer.secret", "orders.auth_version"} {
		if err := resource.validateJoinColumnRef(ref, []string{"orders", "customer"}); err == nil {
			t.Errorf("restricted join column %q was accepted", ref)
		}
	}
	for _, ref := range []string{"email", "total", "customer.name", "orders.shared"} {
		if err := resource.validateColumnRef(ref, []string{"orders", "customer"}); err != nil {
			t.Errorf("ordinary column %q was rejected: %v", ref, err)
		}
	}
}

func TestAggregateRejectsRestrictedColumnsInEveryClause(t *testing.T) {
	resource := aggregateSecurityTestResource()
	requests := []struct {
		name string
		req  AggregationRequest
	}{
		{"column", AggregationRequest{ProjectColumn: []string{"password"}}},
		{"aggregate function", AggregationRequest{ProjectColumn: []string{"max(password)"}}},
		{"scalar function", AggregationRequest{ProjectColumn: []string{"upper(password) as secret"}}},
		{"group", AggregationRequest{GroupBy: []string{"password"}}},
		{"order", AggregationRequest{ProjectColumn: []string{"count"}, Order: []string{"password"}}},
		{"filter", AggregationRequest{ProjectColumn: []string{"count"}, Filter: []string{"eq(password,guess)"}}},
		{"having", AggregationRequest{ProjectColumn: []string{"count"}, Having: []string{"gt(max(password),guess)"}}},
		{"joined column", AggregationRequest{Join: []string{"customer@eq(customer_id,customer.id)"}, ProjectColumn: []string{"customer.secret"}}},
		{"join left", AggregationRequest{ProjectColumn: []string{"count"}, Join: []string{"customer@eq(customer.secret,customer_id)"}}},
		{"join right", AggregationRequest{ProjectColumn: []string{"count"}, Join: []string{"customer@eq(customer_id,customer.secret)"}}},
	}
	for _, test := range requests {
		t.Run(test.name, func(t *testing.T) {
			test.req.RootEntity = "orders"
			_, err := resource.DataStats(test.req, nil)
			var validationError *AggregationValidationError
			if !errors.As(err, &validationError) {
				t.Fatalf("restricted aggregate returned %v, want validation error", err)
			}
		})
	}
	if _, err := resource.parseAggExpr("count(*)", []string{"orders"}, true); err != nil {
		t.Fatalf("count(*) was rejected: %v", err)
	}
}

func TestParseAggregateConditionRejectsPartialAndInjectedInput(t *testing.T) {
	malicious := []string{
		"eq(id,1) trailing",
		"eq(id`,1)",
		"eq(id) OR 1=1 --,1)",
		"unknown(id,1)",
		"eq(id,)",
		"eq(,1)",
		"eq(id,1",
	}
	for _, input := range malicious {
		t.Run(input, func(t *testing.T) {
			condition, err := parseAggregateCondition(input)
			if err == nil {
				// The structural parser may accept hostile text as a left operand;
				// schema validation must then reject it before SQL construction.
				if validateErr := aggregateSecurityTestResource().validateColumnRef(condition.Left, []string{"orders"}); validateErr == nil {
					t.Fatalf("malicious condition was accepted: %#v", condition)
				}
			}
		})
	}
}

func TestAggregateConditionSupportedGrammar(t *testing.T) {
	tests := []struct {
		input, operator, left, right string
	}{
		{"eq(email,user@example.test)", "eq", "email", "user@example.test"},
		{"in(id,1,2,3)", "in", "id", "1,2,3"},
		{"gt(sum(total),100.5)", "gt", "sum(total)", "100.5"},
	}
	for _, test := range tests {
		condition, err := parseAggregateCondition(test.input)
		if err != nil {
			t.Fatalf("parseAggregateCondition(%q): %v", test.input, err)
		}
		if condition.Operator != test.operator || condition.Left != test.left || condition.Right != test.right {
			t.Fatalf("parseAggregateCondition(%q) = %#v", test.input, condition)
		}
	}
}

func TestAggregateOrderValidatesIdentifiersAndAliases(t *testing.T) {
	resource := aggregateSecurityTestResource()
	tables := []string{"orders", "customer"}
	valid := [][]string{{"email"}, {"-customer.name"}, {"-total_sum"}, {"sum(total)"}}
	for _, order := range valid {
		if _, err := resource.buildAggregateOrder(order, []string{"sum(total) as total_sum"}, tables); err != nil {
			t.Errorf("valid order %q rejected: %v", order, err)
		}
	}

	malicious := [][]string{
		{"id` LIMIT (CASE WHEN 1=1 THEN 1 ELSE 0 END) -- "},
		{"id\" LIMIT 1 -- "},
		{"-"},
		{"unknown"},
		{"sqlite_version()"},
		{"sum(password)"},
	}
	for _, order := range malicious {
		_, err := resource.buildAggregateOrder(order, nil, tables)
		var validationError *AggregationValidationError
		if err == nil || !errors.As(err, &validationError) {
			t.Errorf("unsafe order %q was not rejected as validation error: %v", order, err)
		}
	}
}

func TestProjectionAliasesPreserveFunctionArgumentCommas(t *testing.T) {
	aliases := projectionAliases([]string{"strftime('%Y-%m', created_at) as month,count"})
	if !aliases["month"] || !aliases["count"] {
		t.Fatalf("projection aliases were not parsed at top level: %#v", aliases)
	}
}

func TestAggregateHavingRequiresValidatedAggregate(t *testing.T) {
	resource := aggregateSecurityTestResource()
	tables := []string{"orders"}
	valid := []string{"count", "count(*)", "sum(total)", "avg(total)"}
	for _, expression := range valid {
		if _, err := resource.parseHavingExpression(expression, tables); err != nil {
			t.Errorf("valid HAVING expression %q rejected: %v", expression, err)
		}
	}
	invalid := []string{"email", "sum(password)", "sum(total`)", "sqlite_version()", "sum(total) as leaked"}
	for _, expression := range invalid {
		if _, err := resource.parseHavingExpression(expression, tables); err == nil {
			t.Errorf("unsafe HAVING expression %q accepted", expression)
		}
	}
}

func TestAggregationJoinTablesRejectsMalformedAndUnknownTables(t *testing.T) {
	resource := aggregateSecurityTestResource()
	tests := []AggregationRequest{
		{Join: []string{"customer"}},
		{Join: []string{"customer@"}},
		{Join: []string{"customer`@eq(id,customer.id)"}},
		{Join: []string{"user_account@eq(id,user_account.id)"}},
	}
	for _, request := range tests {
		if _, err := resource.AggregationJoinTables(request); err == nil {
			t.Errorf("unsafe join %q accepted", request.Join)
		}
	}
	joins, err := resource.AggregationJoinTables(AggregationRequest{Join: []string{"customer@eq(customer_id,customer.id)"}})
	if err != nil || len(joins) != 1 || joins[0] != "customer" {
		t.Fatalf("valid join rejected: joins=%q err=%v", joins, err)
	}
}
