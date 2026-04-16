package resource

import (
	"fmt"
	"github.com/artpar/api2go/v2"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strconv"
	"strings"

	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/pkg/errors"
)

type TimeStamp string

type AggregationRequest struct {
	RootEntity    string
	Join          []string
	GroupBy       []string
	ProjectColumn []string
	Query         []Query
	Order         []string
	Having        []string
	Filter        []string
	TimeSample    TimeStamp
	TimeFrom      string
	TimeTo        string
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

func ToOrderedExpressionArray(s []string) []exp.OrderedExpression {
	r := make([]exp.OrderedExpression, 0, len(s))
	for _, e := range s {
		if e == "" {
			continue
		}
		if e[0] == '-' {
			r = append(r, goqu.C(e[1:]).Desc())
		} else {
			r = append(r, goqu.C(e).Asc())
		}
	}
	return r
}

func ToExpressionArray(s []string) []exp.Expression {
	r := make([]exp.Expression, len(s))
	for i, e := range s {
		r[i] = goqu.C(e).Asc()
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

func (dbResource *DbResource) DataStats(req AggregationRequest, transaction *sqlx.Tx) (*AggregateData, error) {

	requestedGroupBys := req.GroupBy
	projections := req.ProjectColumn
	joinedTables := make([]string, 0)

	// Pre-pass: validate join table names and build allowedTables.
	// Projections and group-by are validated against this set so cross-table
	// column references (e.g. "customer.name") can be schema-checked.
	allowedTables := []string{req.RootEntity}
	for _, join := range req.Join {
		joinParts := strings.Split(join, "@")
		joinTable := joinParts[0]
		if dbResource.Cruds[joinTable] == nil {
			return nil, fmt.Errorf("unknown join table: %q", joinTable)
		}
		allowedTables = append(allowedTables, joinTable)
	}

	// Parse and validate projections (column param).
	// Top-level comma-splitting is preserved for the "column=a,b,c" shorthand.
	projectionsAdded := make([]interface{}, 0)
	updatedProjections := make([]string, 0)
	for _, project := range projections {
		if strings.Index(project, ",") > -1 {
			parts := strings.Split(project, ",")
			updatedProjections = append(updatedProjections, parts...)
		} else {
			updatedProjections = append(updatedProjections, project)
		}
	}
	projections = updatedProjections

	for _, project := range projections {
		project = strings.TrimSpace(project)
		expr, err := dbResource.parseAggExpr(project, allowedTables, true)
		if err != nil {
			return nil, fmt.Errorf("invalid column %q: %w", project, err)
		}
		projectionsAdded = append(projectionsAdded, expr)
	}

	// Parse and validate group-by expressions.
	// Aggregate functions are not permitted in GROUP BY (allowAggregate=false).
	groupBysAdded := make([]interface{}, 0)
	for _, group := range requestedGroupBys {
		expr, err := dbResource.parseAggExpr(group, allowedTables, false)
		if err != nil {
			return nil, fmt.Errorf("invalid group-by %q: %w", group, err)
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

	builder = builder.Order(ToOrderedExpressionArray(req.Order)...)

	// functionName(param1, param2)
	querySyntax, err := regexp.Compile("([a-zA-Z0-9=<>]+)\\(([^,]+?),(.+)\\)")
	CheckErr(err, "Failed to build query regex")
	whereExpressions := make([]goqu.Expression, 0)
	for _, filter := range req.Filter {

		if !querySyntax.MatchString(filter) {
			CheckErr(errors.New("Invalid filter syntax"), "Failed to parse query [%v]", filter)
		} else {

			parts := querySyntax.FindStringSubmatch(filter)

			var rightVal interface{}
			functionName := strings.TrimSpace(parts[1])
			leftVal := strings.TrimSpace(parts[2])
			rightVal = strings.TrimSpace(parts[3])

			if strings.Index(rightVal.(string), "@") > -1 {
				rightValParts := strings.Split(rightVal.(string), "@")
				entityName := rightValParts[0]
				entityReferenceId, err := uuid.Parse(rightValParts[1])
				if err != nil {
					return nil, fmt.Errorf("invalid reference id in where clause - [%v][%v]: %v", entityName, rightValParts[1], err)
				}
				entityId, err := GetReferenceIdToIdWithTransaction(entityName, daptinid.DaptinReferenceId(entityReferenceId), transaction)
				if err != nil {
					return nil, fmt.Errorf("referenced entity in where clause not found - [%v][%v] -%v", entityName, entityReferenceId, err)
				}
				rightVal = entityId

			}

			//function := builder.Where
			whereClause, err := BuildWhereClause(functionName, leftVal, rightVal)
			if err != nil {
				return nil, err
			}
			whereExpressions = append(whereExpressions, whereClause)

		}
	}
	builder = builder.Where(whereExpressions...)

	havingExpressions := make([]goqu.Expression, 0)
	for _, filter := range req.Having {

		if !querySyntax.MatchString(filter) {
			CheckErr(errors.New("Invalid filter syntax"), "Failed to parse query [%v]", filter)
		} else {

			parts := querySyntax.FindStringSubmatch(filter)

			functionName := strings.TrimSpace(parts[1])
			leftVal := strings.TrimSpace(parts[2])
			rightVal := strings.TrimSpace(parts[3])

			//function := builder.Where

			var rightValInterface interface{}
			rightValInterface = rightVal

			if functionName == "in" || functionName == "notin" {
				rightValInterface = strings.Split(rightVal, ",")
				havingExpressions = append(havingExpressions, goqu.Ex{
					leftVal: rightValInterface,
				})
			} else {
				leftValParts := strings.Split(leftVal, "(")
				var colName string
				var aggregateFunc string

				// Handle both "count" and "count(column)" syntax
				if len(leftValParts) > 1 {
					// Complex form: sum(price), count(id), etc.
					colName = strings.Split(leftValParts[1], ")")[0]
					aggregateFunc = leftValParts[0]
				} else {
					// Simple form: count - use "*" for COUNT(*)
					aggregateFunc = leftVal
					if aggregateFunc == "count" {
						colName = "*"
					} else {
						return nil, fmt.Errorf("aggregate function %s requires a column name in having clause", aggregateFunc)
					}
				}

				var expr exp.SQLFunctionExpression
				var finalExpr exp.Expression

				switch aggregateFunc {
				case "count":
					expr = goqu.COUNT(colName)
				case "sum":
					expr = goqu.SUM(colName)
				case "min":
					expr = goqu.MIN(colName)
				case "max":
					expr = goqu.MAX(colName)
				case "avg":
					expr = goqu.AVG(colName)
				case "first":
					expr = goqu.FIRST(colName)
				case "last":
					expr = goqu.LAST(colName)
				default:
					return nil, fmt.Errorf("invalid function name in having clause - %s", aggregateFunc)
				}

				// Convert rightVal to appropriate type (try int, then float, fallback to string)
				var rightValTyped interface{}
				if intVal, err := strconv.ParseInt(rightVal, 10, 64); err == nil {
					rightValTyped = intVal
				} else if floatVal, err := strconv.ParseFloat(rightVal, 64); err == nil {
					rightValTyped = floatVal
				} else {
					rightValTyped = rightVal
				}

				switch functionName {
				case "lt":
					finalExpr = expr.Lt(rightValTyped)
				case "lte":
					finalExpr = expr.Lte(rightValTyped)
				case "gt":
					finalExpr = expr.Gt(rightValTyped)
				case "gte":
					finalExpr = expr.Gte(rightValTyped)
				case "eq":
					finalExpr = expr.Eq(rightValTyped)
				}

				havingExpressions = append(havingExpressions, finalExpr)

			}
		}
	}
	builder = builder.Having(havingExpressions...)

	for _, join := range req.Join {
		joinParts := strings.Split(join, "@")

		joinTable := joinParts[0]
		joinClause := strings.Join(joinParts[1:], "@")
		joinClauseList := strings.Split(joinClause, "&")

		joinedTables = append(joinedTables, joinTable)

		joinWhereList := make([]goqu.Expression, 0)
		for _, joinClause := range joinClauseList {

			if !querySyntax.MatchString(joinClause) {
				return nil, fmt.Errorf("invalid join condition format: %v", joinClause)
			} else {
				parts := querySyntax.FindStringSubmatch(joinClause)

				var rightValue interface{}
				if BeginsWith(parts[3], "\"") || BeginsWith(parts[3], "'") {
					rightValue, _ = strconv.Unquote(parts[3])
				} else {
					if strings.Index(parts[3], "@") > -1 {
						rightValParts := strings.Split(parts[3], "@")
						entityName := rightValParts[0]
						entityReferenceId, err := uuid.Parse(rightValParts[1])
						if err != nil {
							return nil, fmt.Errorf("invalid reference id in join clause - [%v][%v]: %v", entityName, rightValParts[1], err)
						}
						entityId, err := GetReferenceIdToIdWithTransaction(entityName, daptinid.DaptinReferenceId(entityReferenceId), transaction)
						if err != nil {
							return nil, fmt.Errorf("referenced entity in join clause not found - [%v][%v] -%v", entityName, entityReferenceId, err)
						}
						rightValue = entityId
					} else {
						rightValue = goqu.I(parts[3])
					}
				}

				joinWhere, err := BuildWhereClause(parts[1], parts[2], rightValue)
				if err != nil {
					return nil, err
				}
				joinWhereList = append(joinWhereList, joinWhere)
			}

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

func BuildWhereClause(functionName string, leftVal string, rightVal interface{}) (goqu.Expression, error) {

	var rightValInterface interface{}
	rightValInterface = rightVal
	if rightValInterface == "null" {
		rightValInterface = nil

		switch functionName {
		case "is":
			return goqu.C(leftVal).IsNull(), nil

		case "not":
			return goqu.C(leftVal).IsNotNull(), nil

		default:
			return nil, fmt.Errorf("invalid function name for null rhs - %v", functionName)

		}

	}
	if rightValInterface == "true" {
		rightValInterface = nil

		switch functionName {
		case "is":
			return goqu.C(leftVal).IsTrue(), nil

		case "not":
			return goqu.C(leftVal).IsNotTrue(), nil

		default:
			return nil, fmt.Errorf("invalid function name for true rhs - %v", functionName)

		}

	}
	if rightValInterface == "false" {
		rightValInterface = nil

		switch functionName {
		case "is":
			return goqu.C(leftVal).IsFalse(), nil

		case "not":
			return goqu.C(leftVal).IsNotFalse(), nil

		default:
			return nil, fmt.Errorf("invalid function name for false rhs - %v", functionName)

		}

	}

	if functionName == "in" || functionName == "notin" {
		rightValInterface = strings.Split(rightVal.(string), ",")
	}

	switch functionName {
	case "in":
		fallthrough
	case "notin":
		rightValInterface = strings.Split(rightVal.(string), ",")
		return goqu.Ex{
			leftVal: rightValInterface,
		}, nil
	case "=":
		return goqu.Ex{
			leftVal: rightValInterface,
		}, nil
	case "not":
		return goqu.Ex{
			leftVal: goqu.Op{
				"neq": rightValInterface,
			},
		}, nil
	default:
		return goqu.Ex{
			leftVal: goqu.Op{
				functionName: rightValInterface,
			},
		}, nil

	}
}
