package resource

import (
	"fmt"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"regexp"
	"sort"
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

// PaginatedFindAll(req Request) (totalCount uint, response Responder, err error)
type AggregateData struct {
	Data []map[string]interface{} `json:"data"`
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
	r := make([]exp.OrderedExpression, len(s))
	for i, e := range s {
		if e[0] == '-' {
			r[i] = goqu.C(e[1:]).Desc()
		} else {
			r[i] = goqu.C(e).Asc()
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

func (dr *DbResource) DataStats(req AggregationRequest) (*AggregateData, error) {

	sort.Strings(req.GroupBy)
	projections := req.ProjectColumn

	projectionsAdded := make([]interface{}, 0)
	for i, project := range projections {
		if project == "count" {
			projections[i] = "count(*) as count"
			projectionsAdded = append(projectionsAdded, goqu.L("count(*)").As("count"))
		} else {
			projectionsAdded = append(projectionsAdded, goqu.I(project))
		}
	}

	for _, group := range req.GroupBy {
		projections = append(projections, group)
		projectionsAdded = append(projectionsAdded, goqu.C(group))
	}

	if len(projections) == 0 {
		projectionsAdded = append(projectionsAdded, goqu.L("count(*)").As("count"))
	}

	selectBuilder := statementbuilder.Squirrel.Select(projectionsAdded...)
	builder := selectBuilder.From(req.RootEntity)

	builder = builder.GroupBy(ToInterfaceArray(req.GroupBy)...)

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

			functionName := strings.TrimSpace(parts[1])
			leftVal := strings.TrimSpace(parts[2])
			rightVal := strings.TrimSpace(parts[3])

			//function := builder.Where

			var rightValInterface interface{}
			rightValInterface = rightVal
			if rightValInterface == "null" {
				rightValInterface = nil

				switch functionName {
				case "is":
					whereExpressions = append(whereExpressions, goqu.C(leftVal).IsNull())

				case "not":
					whereExpressions = append(whereExpressions, goqu.C(leftVal).IsNotNull())

				default:
					return nil, fmt.Errorf("invalid function name for null rhs - " + functionName)

				}
				continue

			}
			if rightValInterface == "true" {
				rightValInterface = nil

				switch functionName {
				case "is":
					whereExpressions = append(whereExpressions, goqu.C(leftVal).IsTrue())

				case "not":
					whereExpressions = append(whereExpressions, goqu.C(leftVal).IsNotTrue())

				default:
					return nil, fmt.Errorf("invalid function name for true rhs - " + functionName)

				}
				continue

			}
			if rightValInterface == "false" {
				rightValInterface = nil

				switch functionName {
				case "is":
					whereExpressions = append(whereExpressions, goqu.C(leftVal).IsFalse())

				case "not":
					whereExpressions = append(whereExpressions, goqu.C(leftVal).IsNotFalse())

				default:
					return nil, fmt.Errorf("invalid function name for false rhs - " + functionName)

				}
				continue

			}

			if functionName == "in" || functionName == "notin" {
				rightValInterface = strings.Split(rightVal, ",")
			}

			if functionName == "in" || functionName == "notin" {
				rightValInterface = strings.Split(rightVal, ",")
				whereExpressions = append(whereExpressions, goqu.Ex{
					leftVal: rightValInterface,
				})
			} else if functionName == "=" {
				whereExpressions = append(whereExpressions, goqu.Ex{
					leftVal: rightValInterface,
				})
			} else {
				whereExpressions = append(whereExpressions, goqu.Ex{
					leftVal: goqu.Op{
						functionName: goqu.V(rightValInterface),
					},
				})
			}

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
				colName := strings.Split(leftValParts[1], ")")[0]
				var expr exp.SQLFunctionExpression
				var finalExpr exp.Expression

				switch leftValParts[0] {
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
					return nil, fmt.Errorf("invalid function name in having clause - " + leftValParts[0])
				}

				switch functionName {
				case "lt":
					finalExpr = expr.Lt(rightVal)
				case "lte":
					finalExpr = expr.Lte(rightVal)
				case "gt":
					finalExpr = expr.Gt(rightVal)
				case "gte":
					finalExpr = expr.Gte(rightVal)
				case "eq":
					finalExpr = expr.Eq(rightVal)
				}

				havingExpressions = append(havingExpressions, finalExpr)

			}

		}
	}
	builder = builder.Having(havingExpressions...)

	sql, args, err := builder.ToSQL()
	CheckErr(err, "Failed to generate stats sql: [%v]")
	if err != nil {
		return nil, err
	}

	log.Infof("Aggregation query: %v", sql)
	res, err := dr.db.Queryx(sql, args...)
	CheckErr(err, "Failed to query stats: %v", sql)
	if err != nil {
		return nil, err
	}
	defer func(res *sqlx.Rows) {
		err := res.Close()
		if err != nil {
			log.Errorf("failed to close aggregate query result - {}", err)
		}
	}(res)

	rows, err := RowsToMap(res, "aggregate_"+req.RootEntity)
	CheckErr(err, "Failed to scan ")

	for _, groupedColumn := range req.GroupBy {
		columnInfo, ok := dr.tableInfo.GetColumnByName(groupedColumn)
		if !ok {
			continue
		}
		if columnInfo.IsForeignKey && columnInfo.ForeignKeyData.DataSource == "self" {
			entityName := columnInfo.ForeignKeyData.Namespace
			idsToConvert := make([]int64, 0)
			for _, row := range rows {
				idsToConvert = append(idsToConvert, row[groupedColumn].(int64))
			}
			referenceIds, err := dr.Cruds[entityName].GetIdListToReferenceIdList(entityName, idsToConvert)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				row[groupedColumn] = referenceIds[row[groupedColumn].(int64)]
			}
		}
	}

	return &AggregateData{
		Data: rows,
	}, err

}
