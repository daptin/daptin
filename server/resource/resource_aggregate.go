package resource

import (
	"fmt"
	"github.com/artpar/api2go/v2"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"

	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
)

type TimeStamp string

type AggregationRequest struct {
	RootEntity    string    `json:"root_entity,omitempty"`
	Join          []string  `json:"join,omitempty"`
	GroupBy       []string  `json:"group,omitempty"`
	ProjectColumn []string  `json:"column,omitempty"`
	Query         []Query   `json:"query,omitempty"`
	Order         []string  `json:"order,omitempty"`
	Having        []string  `json:"having,omitempty"`
	Filter        []string  `json:"filter,omitempty"`
	TimeSample    TimeStamp `json:"timesample,omitempty"`
	TimeFrom      string    `json:"timefrom,omitempty"`
	TimeTo        string    `json:"timeto,omitempty"`
}

type AggregateRow struct {
	Type       string                 `json:"type"`
	Id         string                 `json:"id"`
	Attributes map[string]interface{} `json:"attributes"`
}

// PaginatedFindAll(req Request) (totalCount uint, response Responder, err error)
type AggregateData struct {
	Data []AggregateRow `json:"data"`
}

func InArray(val []interface{}, ar interface{}) (exists bool) {
	exists = false

	for _, v := range val {
		if v == ar {
			return true
		}
	}
	return false
}
func InStringArray(val []string, ar string) (exists bool) {
	exists = false

	for _, v := range val {
		if v == ar {
			return true
		}
	}
	return false
}

func ToInterfaceArray(s []string) []interface{} {
	r := make([]interface{}, len(s))
	for i, e := range s {
		r[i] = e
	}
	return r
}

func MapArrayToInterfaceArray(s []map[string]interface{}) []interface{} {
	r := make([]interface{}, len(s))
	for i, e := range s {
		r[i] = e
	}
	return r
}

func ColumnToInterfaceArray(s []column) []interface{} {
	r := make([]interface{}, len(s))
	for i, e := range s {
		r[i] = e.originalvalue
	}
	return r
}

// aggregateFuncs maps aggregate function names to their safe goqu typed constructors.
// Exact map key lookup — no pattern matching.
var aggregateFuncs = map[string]func(interface{}) exp.SQLFunctionExpression{
	"count": func(col interface{}) exp.SQLFunctionExpression { return goqu.COUNT(col) },
	"sum":   func(col interface{}) exp.SQLFunctionExpression { return goqu.SUM(col) },
	"min":   func(col interface{}) exp.SQLFunctionExpression { return goqu.MIN(col) },
	"max":   func(col interface{}) exp.SQLFunctionExpression { return goqu.MAX(col) },
	"avg":   func(col interface{}) exp.SQLFunctionExpression { return goqu.AVG(col) },
	"first": func(col interface{}) exp.SQLFunctionExpression { return goqu.FIRST(col) },
	"last":  func(col interface{}) exp.SQLFunctionExpression { return goqu.LAST(col) },
}

// scalarFuncs is the allowlist of safe data-transformation SQL functions.
// System functions, I/O functions, and anything that can read from other sources are excluded.
// Exact map key lookup — no pattern matching.
var scalarFuncs = map[string]bool{
	"date": true, "time": true, "datetime": true, "strftime": true, "julianday": true,
	"month": true, "year": true, "day": true,
	"upper": true, "lower": true, "length": true, "substr": true, "trim": true,
	"ltrim": true, "rtrim": true, "replace": true, "hex": true,
	"abs": true, "round": true,
	"coalesce": true, "ifnull": true, "nullif": true,
}

// AggregationValidationError identifies malformed or out-of-scope aggregate
// input. Callers may safely expose the stable error code, but not the detailed
// message, which can contain schema names.
type AggregationValidationError struct {
	Parameter string
	Err       error
}

func (e *AggregationValidationError) Error() string {
	return fmt.Sprintf("invalid aggregate %s: %v", e.Parameter, e.Err)
}

func (e *AggregationValidationError) Unwrap() error { return e.Err }

func invalidAggregation(parameter, format string, args ...interface{}) error {
	return &AggregationValidationError{Parameter: parameter, Err: fmt.Errorf(format, args...)}
}

var aggregateConditionOperators = map[string]bool{
	"eq": true, "not": true, "lt": true, "lte": true,
	"gt": true, "gte": true, "in": true, "notin": true, "is": true,
}

type aggregateCondition struct {
	Operator string
	Left     string
	Right    string
}

// splitTopLevelComma splits at the first comma outside nested parentheses and
// quoted strings. It is used for conditions such as gt(sum(total),100).
func splitTopLevelComma(input string) (string, string, bool) {
	depth := 0
	var quote rune
	for i, r := range input {
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return "", "", false
			}
			depth--
		case ',':
			if depth == 0 {
				return strings.TrimSpace(input[:i]), strings.TrimSpace(input[i+1:]), true
			}
		}
	}
	return "", "", false
}

// parseAggregateCondition parses the entire condition. Prefix/suffix matches
// are deliberately rejected so trailing SQL cannot be ignored by the parser.
func parseAggregateCondition(raw string) (aggregateCondition, error) {
	raw = strings.TrimSpace(raw)
	open := strings.IndexByte(raw, '(')
	if open <= 0 || !strings.HasSuffix(raw, ")") {
		return aggregateCondition{}, fmt.Errorf("expected operator(left,right)")
	}
	operator := strings.TrimSpace(raw[:open])
	if !isSimpleIdentifier(operator) || !aggregateConditionOperators[operator] {
		return aggregateCondition{}, fmt.Errorf("unsupported operator %q", operator)
	}
	left, right, ok := splitTopLevelComma(raw[open+1 : len(raw)-1])
	if !ok || left == "" || right == "" {
		return aggregateCondition{}, fmt.Errorf("condition requires non-empty left and right operands")
	}
	return aggregateCondition{Operator: operator, Left: left, Right: right}, nil
}

func parseAggregateValue(raw string) interface{} {
	raw = strings.TrimSpace(raw)
	if value, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return value
	}
	if value, err := strconv.ParseFloat(raw, 64); err == nil {
		return value
	}
	if strings.EqualFold(raw, "true") {
		return true
	}
	if strings.EqualFold(raw, "false") {
		return false
	}
	if strings.EqualFold(raw, "null") {
		return nil
	}
	return raw
}

func buildAggregateComparison(left interface {
	exp.Comparable
	exp.Inable
	exp.Isable
}, operator string, right interface{}) (exp.BooleanExpression, error) {
	switch operator {
	case "eq":
		return left.Eq(right), nil
	case "not":
		if right == nil || right == true || right == false {
			return left.IsNot(right), nil
		}
		return left.Neq(right), nil
	case "lt":
		return left.Lt(right), nil
	case "lte":
		return left.Lte(right), nil
	case "gt":
		return left.Gt(right), nil
	case "gte":
		return left.Gte(right), nil
	case "is":
		if right != nil && right != true && right != false {
			return nil, fmt.Errorf("is only accepts null, true, or false")
		}
		return left.Is(right), nil
	default:
		return nil, fmt.Errorf("operator %q requires a list value", operator)
	}
}

// isSimpleIdentifier returns true if s is a valid SQL identifier:
// only letters, digits, underscores; first character not a digit.
// Used to validate alias names. Character loop — no regex.
func isSimpleIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 && c >= '0' && c <= '9' {
			return false
		}
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// splitFuncArgs splits a comma-separated argument string while respecting single-quoted strings.
// E.g.: "'%Y-%m', created_at" → ["'%Y-%m'", "created_at"]
func splitFuncArgs(argsStr string) []string {
	var args []string
	depth, start := 0, 0
	inQuote := false
	for i, c := range argsStr {
		switch {
		case c == '\'' && depth == 0:
			inQuote = !inQuote
		case !inQuote && c == '(':
			depth++
		case !inQuote && c == ')':
			depth--
		case !inQuote && c == ',' && depth == 0:
			args = append(args, strings.TrimSpace(argsStr[start:i]))
			start = i + 1
		}
	}
	return append(args, strings.TrimSpace(argsStr[start:]))
}

// validateColumnRef checks that col (a simple identifier or "table.col") exists in the schema
// of one of the listed tables. It accepts only plain identifiers — no expressions, no operators.
// For qualified names (table.col), the table must be in the tables allowlist (root entity or
// an explicitly joined table) — not just any entity in the system.
func (dbResource *DbResource) validateColumnRef(col string, tables []string) error {
	if strings.Contains(col, ".") {
		parts := strings.SplitN(col, ".", 2)
		tbl, field := parts[0], parts[1]
		inScope := false
		for _, t := range tables {
			if t == tbl {
				inScope = true
				break
			}
		}
		if !inScope {
			return fmt.Errorf("table %q is not in scope (must be the root entity or a joined table)", tbl)
		}
		crud := dbResource.Cruds[tbl]
		if crud == nil {
			return fmt.Errorf("unknown table %q", tbl)
		}
		if _, ok := crud.TableInfo().GetColumnByName(field); !ok {
			return fmt.Errorf("unknown column %q in table %q", field, tbl)
		}
		return nil
	}
	for _, tbl := range tables {
		if crud := dbResource.Cruds[tbl]; crud != nil {
			if _, ok := crud.TableInfo().GetColumnByName(col); ok {
				return nil
			}
		}
	}
	return fmt.Errorf("unknown column: %q", col)
}

// buildFuncArgs converts raw argument strings into safe goqu expressions.
// Each argument is classified as: "*" (star, count only), 'quoted string' (passed as Go value,
// parameterized by goqu), or a column reference (schema-validated, wrapped in goqu.I).
func (dbResource *DbResource) buildFuncArgs(funcName string, rawArgs []string, tables []string) ([]interface{}, error) {
	built := make([]interface{}, 0, len(rawArgs))
	for _, arg := range rawArgs {
		arg = strings.TrimSpace(arg)
		if arg == "*" {
			if funcName != "count" {
				return nil, fmt.Errorf("'*' only valid in count()")
			}
			built = append(built, goqu.Star())
			continue
		}
		if strings.HasPrefix(arg, "'") && strings.HasSuffix(arg, "'") && len(arg) >= 2 {
			// Quoted string literal: strip quotes, pass as Go string.
			// goqu will parameterize it in prepared-statement mode — no injection possible.
			literal := arg[1 : len(arg)-1]
			if strings.Contains(literal, "'") {
				return nil, fmt.Errorf("invalid string literal in %s()", funcName)
			}
			built = append(built, literal)
			continue
		}
		// Column reference: must exist in schema.
		if err := dbResource.validateColumnRef(arg, tables); err != nil {
			return nil, fmt.Errorf("arg %q in %s(): %w", arg, funcName, err)
		}
		built = append(built, goqu.I(arg))
	}
	return built, nil
}

// parseAggExpr converts a user-supplied aggregation expression string into a safe goqu expression.
// Supported forms:
//   - "count"                          → COUNT(*)
//   - "col" / "table.col"              → identifier (schema-validated)
//   - "agg_func(col)"                  → aggregate function (allowlist + schema)
//   - "scalar_func(col)"               → scalar function (allowlist + schema)
//   - "scalar_func('lit', col)"        → scalar with string literal + column
//   - any of the above + " as alias"   → with alias (simple identifier check)
//
// allowAggregate controls whether aggregate functions are permitted (false for GROUP BY).
// goqu.L() is never used — all output uses goqu's typed safe constructors.
func (dbResource *DbResource) parseAggExpr(expr string, tables []string, allowAggregate bool) (interface{}, error) {
	expr = strings.TrimSpace(expr)

	// Special shorthand: bare "count" → COUNT(*)
	if expr == "count" && allowAggregate {
		return goqu.COUNT(goqu.Star()).As("count"), nil
	}

	// Peel off trailing " as alias" if present
	var alias string
	if idx := strings.LastIndex(expr, " as "); idx > 0 {
		maybeAlias := strings.TrimSpace(expr[idx+4:])
		if !isSimpleIdentifier(maybeAlias) {
			return nil, fmt.Errorf("invalid alias: %q", maybeAlias)
		}
		alias = maybeAlias
		expr = strings.TrimSpace(expr[:idx])
	}

	// Function call: funcname(args...)
	if openParen := strings.Index(expr, "("); openParen > 0 {
		if expr[len(expr)-1] != ')' {
			return nil, fmt.Errorf("malformed function call: %q", expr)
		}
		funcName := strings.TrimSpace(expr[:openParen])
		argsStr := expr[openParen+1 : len(expr)-1]
		rawArgs := splitFuncArgs(argsStr)

		// Aggregate function path
		if aggBuilder, ok := aggregateFuncs[funcName]; ok {
			if !allowAggregate {
				return nil, fmt.Errorf("aggregate function %q not allowed in group-by", funcName)
			}
			if len(rawArgs) != 1 {
				return nil, fmt.Errorf("%s() takes exactly one argument", funcName)
			}
			builtArgs, err := dbResource.buildFuncArgs(funcName, rawArgs, tables)
			if err != nil {
				return nil, err
			}
			result := aggBuilder(builtArgs[0])
			if alias != "" {
				return result.As(alias), nil
			}
			return result, nil
		}

		// Scalar function path
		if !scalarFuncs[funcName] {
			return nil, fmt.Errorf("unsupported function: %q", funcName)
		}
		// Scalar functions must reference at least one schema column.
		// This blocks zero-argument system functions (e.g. sqlite_version()).
		hasColumnArg := false
		for _, raw := range rawArgs {
			raw = strings.TrimSpace(raw)
			if raw != "*" && !(strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'")) {
				hasColumnArg = true
				break
			}
		}
		if !hasColumnArg {
			return nil, fmt.Errorf("scalar function %q requires at least one column argument", funcName)
		}
		builtArgs, err := dbResource.buildFuncArgs(funcName, rawArgs, tables)
		if err != nil {
			return nil, err
		}
		result := goqu.Func(funcName, builtArgs...)
		if alias != "" {
			return result.As(alias), nil
		}
		return result, nil
	}

	// Plain column reference — no parentheses permitted
	if strings.ContainsAny(expr, "(),") {
		return nil, fmt.Errorf("invalid expression: %q", expr)
	}
	if err := dbResource.validateColumnRef(expr, tables); err != nil {
		return nil, err
	}
	col := goqu.I(expr)
	if alias != "" {
		return col.As(alias), nil
	}
	return col, nil
}

// AggregationJoinTables validates the outer structure and table name of every
// requested join. It is shared by authorization and query construction so a
// joined table cannot bypass either layer.
func (dbResource *DbResource) AggregationJoinTables(req AggregationRequest) ([]string, error) {
	tables := make([]string, 0, len(req.Join))
	seen := make(map[string]bool)
	for _, rawJoin := range req.Join {
		parts := strings.SplitN(strings.TrimSpace(rawJoin), "@", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, invalidAggregation("join", "expected table@condition")
		}
		table := parts[0]
		if !isSimpleIdentifier(table) || dbResource.Cruds[table] == nil {
			return nil, invalidAggregation("join", "unknown join table %q", table)
		}
		if !seen[table] {
			tables = append(tables, table)
			seen[table] = true
		}
	}
	return tables, nil
}

func projectionAliases(projections []string) map[string]bool {
	aliases := make(map[string]bool)
	if len(projections) == 0 {
		aliases["count"] = true
	}
	for _, projection := range projections {
		for _, item := range splitFuncArgs(projection) {
			item = strings.TrimSpace(item)
			if item == "count" {
				aliases["count"] = true
			}
			if idx := strings.LastIndex(item, " as "); idx > 0 {
				alias := strings.TrimSpace(item[idx+4:])
				if isSimpleIdentifier(alias) {
					aliases[alias] = true
				}
			}
		}
	}
	return aliases
}

func (dbResource *DbResource) buildAggregateOrder(rawOrder []string, projections []string, tables []string) ([]exp.OrderedExpression, error) {
	aliases := projectionAliases(projections)
	result := make([]exp.OrderedExpression, 0, len(rawOrder))
	for _, raw := range rawOrder {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, invalidAggregation("order", "empty order expression")
		}
		descending := strings.HasPrefix(raw, "-")
		if descending {
			raw = strings.TrimSpace(raw[1:])
			if raw == "" {
				return nil, invalidAggregation("order", "missing expression after '-'")
			}
		}

		var orderable exp.Orderable
		if aliases[raw] {
			orderable = goqu.I(raw)
		} else {
			parsed, err := dbResource.parseAggExpr(raw, tables, true)
			if err != nil {
				return nil, invalidAggregation("order", "%v", err)
			}
			var ok bool
			orderable, ok = parsed.(exp.Orderable)
			if !ok {
				return nil, invalidAggregation("order", "expression %q cannot be ordered", raw)
			}
		}
		if descending {
			result = append(result, orderable.Desc())
		} else {
			result = append(result, orderable.Asc())
		}
	}
	return result, nil
}

func buildAggregatePredicate(left interface {
	exp.Comparable
	exp.Inable
	exp.Isable
}, condition aggregateCondition) (exp.BooleanExpression, error) {
	if condition.Operator == "in" || condition.Operator == "notin" {
		rawValues := strings.Split(condition.Right, ",")
		values := make([]interface{}, 0, len(rawValues))
		for _, raw := range rawValues {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return nil, fmt.Errorf("list values cannot be empty")
			}
			values = append(values, parseAggregateValue(raw))
		}
		if condition.Operator == "in" {
			return left.In(values...), nil
		}
		return left.NotIn(values...), nil
	}
	return buildAggregateComparison(left, condition.Operator, parseAggregateValue(condition.Right))
}

func (dbResource *DbResource) parseHavingExpression(raw string, tables []string) (exp.SQLFunctionExpression, error) {
	raw = strings.TrimSpace(raw)
	if raw == "count" {
		return goqu.COUNT(goqu.Star()), nil
	}
	open := strings.IndexByte(raw, '(')
	if open <= 0 || !strings.HasSuffix(raw, ")") {
		return nil, fmt.Errorf("having requires an aggregate expression")
	}
	name := strings.TrimSpace(raw[:open])
	builder, ok := aggregateFuncs[name]
	if !ok {
		return nil, fmt.Errorf("unsupported aggregate function %q", name)
	}
	args := splitFuncArgs(raw[open+1 : len(raw)-1])
	if len(args) != 1 {
		return nil, fmt.Errorf("%s() takes exactly one argument", name)
	}
	built, err := dbResource.buildFuncArgs(name, args, tables)
	if err != nil {
		return nil, err
	}
	return builder(built[0]), nil
}

func (dbResource *DbResource) DataStats(req AggregationRequest, transaction *sqlx.Tx) (*AggregateData, error) {
	if !isSimpleIdentifier(req.RootEntity) || dbResource.Cruds[req.RootEntity] == nil {
		return nil, invalidAggregation("entity", "unknown root entity %q", req.RootEntity)
	}

	requestedGroupBys := req.GroupBy
	projections := req.ProjectColumn
	joinedTables, err := dbResource.AggregationJoinTables(req)
	if err != nil {
		return nil, err
	}

	// Pre-pass: validate join table names and build allowedTables.
	// Projections and group-by are validated against this set so cross-table
	// column references (e.g. "customer.name") can be schema-checked.
	allowedTables := []string{req.RootEntity}
	allowedTables = append(allowedTables, joinedTables...)

	// Parse and validate projections (column param).
	// Top-level comma-splitting is preserved for the "column=a,b,c" shorthand.
	projectionsAdded := make([]interface{}, 0)
	updatedProjections := make([]string, 0)
	for _, project := range projections {
		updatedProjections = append(updatedProjections, splitFuncArgs(project)...)
	}
	projections = updatedProjections

	for _, project := range projections {
		project = strings.TrimSpace(project)
		expr, err := dbResource.parseAggExpr(project, allowedTables, true)
		if err != nil {
			return nil, invalidAggregation("column", "%v", err)
		}
		projectionsAdded = append(projectionsAdded, expr)
	}

	// Parse and validate group-by expressions.
	// Aggregate functions are not permitted in GROUP BY (allowAggregate=false).
	groupBysAdded := make([]interface{}, 0)
	for _, group := range requestedGroupBys {
		expr, err := dbResource.parseAggExpr(group, allowedTables, false)
		if err != nil {
			return nil, invalidAggregation("group", "%v", err)
		}
		projectionsAdded = append(projectionsAdded, expr)
		groupBysAdded = append(groupBysAdded, expr)
	}

	if len(projectionsAdded) == 0 {
		projectionsAdded = append(projectionsAdded, goqu.COUNT(goqu.Star()).As("count"))
	}

	selectBuilder := statementbuilder.Squirrel.Select(projectionsAdded...).Prepared(true)
	builder := selectBuilder.From(req.RootEntity)

	builder = builder.GroupBy(groupBysAdded...)

	orderExpressions, err := dbResource.buildAggregateOrder(req.Order, projections, allowedTables)
	if err != nil {
		return nil, err
	}
	builder = builder.Order(orderExpressions...)

	whereExpressions := make([]goqu.Expression, 0)
	for _, rawFilter := range req.Filter {
		condition, err := parseAggregateCondition(rawFilter)
		if err != nil {
			return nil, invalidAggregation("filter", "%v", err)
		}
		if err := dbResource.validateColumnRef(condition.Left, allowedTables); err != nil {
			return nil, invalidAggregation("filter", "%v", err)
		}
		if condition.Operator != "in" && condition.Operator != "notin" && strings.Count(condition.Right, "@") == 1 {
			reference := strings.SplitN(condition.Right, "@", 2)
			referenceID, referenceErr := uuid.Parse(reference[1])
			if referenceErr == nil && isSimpleIdentifier(reference[0]) && dbResource.Cruds[reference[0]] != nil {
				entityID, err := GetReferenceIdToIdWithTransaction(reference[0], daptinid.DaptinReferenceId(referenceID), transaction)
				if err != nil {
					return nil, invalidAggregation("filter", "referenced entity not found")
				}
				condition.Right = strconv.FormatInt(entityID, 10)
			}
		}
		whereClause, err := buildAggregatePredicate(goqu.I(condition.Left), condition)
		if err != nil {
			return nil, invalidAggregation("filter", "%v", err)
		}
		whereExpressions = append(whereExpressions, whereClause)
	}
	builder = builder.Where(whereExpressions...)

	havingExpressions := make([]goqu.Expression, 0)
	for _, rawHaving := range req.Having {
		condition, err := parseAggregateCondition(rawHaving)
		if err != nil {
			return nil, invalidAggregation("having", "%v", err)
		}
		left, err := dbResource.parseHavingExpression(condition.Left, allowedTables)
		if err != nil {
			return nil, invalidAggregation("having", "%v", err)
		}
		predicate, err := buildAggregatePredicate(left, condition)
		if err != nil {
			return nil, invalidAggregation("having", "%v", err)
		}
		havingExpressions = append(havingExpressions, predicate)
	}
	builder = builder.Having(havingExpressions...)

	for _, join := range req.Join {
		joinParts := strings.SplitN(join, "@", 2)
		joinTable := joinParts[0]
		joinClause := joinParts[1]
		joinClauseList := strings.Split(joinClause, "&")

		joinWhereList := make([]goqu.Expression, 0)
		for _, rawClause := range joinClauseList {
			condition, err := parseAggregateCondition(rawClause)
			if err != nil {
				return nil, invalidAggregation("join", "%v", err)
			}
			if condition.Operator == "in" || condition.Operator == "notin" || condition.Operator == "is" {
				return nil, invalidAggregation("join", "operator %q is not supported for joins", condition.Operator)
			}
			if err := dbResource.validateColumnRef(condition.Left, allowedTables); err != nil {
				return nil, invalidAggregation("join", "%v", err)
			}

			var rightValue interface{}
			rightRaw := strings.TrimSpace(condition.Right)
			if (strings.HasPrefix(rightRaw, "\"") && strings.HasSuffix(rightRaw, "\"")) ||
				(strings.HasPrefix(rightRaw, "'") && strings.HasSuffix(rightRaw, "'")) {
				if len(rightRaw) < 2 || strings.Contains(rightRaw[1:len(rightRaw)-1], rightRaw[:1]) {
					return nil, invalidAggregation("join", "invalid quoted literal")
				}
				rightValue = rightRaw[1 : len(rightRaw)-1]
			} else if strings.Count(rightRaw, "@") == 1 {
				reference := strings.SplitN(rightRaw, "@", 2)
				if !isSimpleIdentifier(reference[0]) || dbResource.Cruds[reference[0]] == nil {
					return nil, invalidAggregation("join", "unknown reference entity %q", reference[0])
				}
				referenceID, err := uuid.Parse(reference[1])
				if err != nil {
					return nil, invalidAggregation("join", "invalid reference id")
				}
				entityID, err := GetReferenceIdToIdWithTransaction(reference[0], daptinid.DaptinReferenceId(referenceID), transaction)
				if err != nil {
					return nil, invalidAggregation("join", "referenced entity not found")
				}
				rightValue = entityID
			} else {
				if err := dbResource.validateColumnRef(rightRaw, allowedTables); err != nil {
					return nil, invalidAggregation("join", "%v", err)
				}
				rightValue = goqu.I(rightRaw)
			}

			left := goqu.I(condition.Left)
			var joinWhere exp.BooleanExpression
			switch condition.Operator {
			case "eq":
				joinWhere = left.Eq(rightValue)
			case "not":
				joinWhere = left.Neq(rightValue)
			case "lt":
				joinWhere = left.Lt(rightValue)
			case "lte":
				joinWhere = left.Lte(rightValue)
			case "gt":
				joinWhere = left.Gt(rightValue)
			case "gte":
				joinWhere = left.Gte(rightValue)
			default:
				return nil, invalidAggregation("join", "unsupported join operator %q", condition.Operator)
			}
			joinWhereList = append(joinWhereList, joinWhere)
		}
		builder = builder.LeftJoin(goqu.T(joinTable), goqu.On(joinWhereList...))

	}

	sql, args, err := builder.ToSQL()
	CheckErr(err, "Failed to generate stats sql: [%v]")
	if err != nil {
		return nil, err
	}

	log.Infof("Aggregation query: %v", sql)

	stmt1, err := transaction.Preparex(sql)
	if err != nil {
		log.Errorf("[291] failed to prepare statment [%v]: %v", sql, err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	queryResult, err := stmt1.Queryx(args...)
	if err != nil {
		CheckErr(err, "Failed to query stats: %v", sql)
		return nil, err
	}
	defer func() {
		if err := queryResult.Close(); err != nil {
			log.Errorf("failed to close aggregate query result - %v", err)
		}
	}()

	returnModelName := "aggregate_" + req.RootEntity
	rows, err := RowsToMap(queryResult, returnModelName)
	CheckErr(err, "Failed to scan ")
	stmt1.Close()

	for _, groupedColumn := range requestedGroupBys {
		var columnInfo *api2go.ColumnInfo
		var ok bool

		if strings.Index(groupedColumn, ".") > -1 {
			groupedColumn = strings.Split(groupedColumn, ".")[1]
		}

		if dbResource.Cruds[req.RootEntity] != nil {
			columnInfo, ok = dbResource.Cruds[req.RootEntity].TableInfo().GetColumnByName(groupedColumn)
		}

		if columnInfo == nil {
			for _, tableName := range joinedTables {
				columnInfo, ok = dbResource.Cruds[tableName].TableInfo().GetColumnByName(groupedColumn)
				if !ok {
					continue
				} else {
					break
				}
			}
		}

		if columnInfo == nil {
			log.Warnf("[378] column info not found for %v", groupedColumn)
			continue
		}

		if columnInfo.IsForeignKey && columnInfo.ForeignKeyData.DataSource == "self" {
			entityName := columnInfo.ForeignKeyData.Namespace
			idsToConvert := make([]int64, 0)
			for _, row := range rows {
				value := row[groupedColumn]
				if value == nil {
					continue
				}
				idsToConvert = append(idsToConvert, row[groupedColumn].(int64))
			}
			if len(idsToConvert) == 0 {
				continue
			}
			referenceIds, err := dbResource.Cruds[entityName].GetIdListToReferenceIdList(entityName, idsToConvert, transaction)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				if row[groupedColumn] == nil {
					continue
				}
				row[groupedColumn] = referenceIds[row[groupedColumn].(int64)]
			}
		}
	}

	returnRows := make([]AggregateRow, 0)
	for _, row := range rows {
		newId, _ := uuid.NewV7()
		returnRows = append(returnRows, AggregateRow{
			Type:       returnModelName,
			Id:         newId.String(),
			Attributes: row,
		})
	}

	return &AggregateData{
		Data: returnRows,
	}, err

}
