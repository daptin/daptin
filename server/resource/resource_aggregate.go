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

func (dbResource *DbResource) DataStats(req AggregationRequest, transaction *sqlx.Tx) (*AggregateData, error) {

	requestedGroupBys := req.GroupBy

	projections := req.ProjectColumn

	joinedTables := make([]string, 0)

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

	for i, project := range projections {
		if project == "count" {
			projections[i] = "count(*) as count"
			projectionsAdded = append(projectionsAdded, goqu.L("count(*)").As("count"))
		} else {
			if strings.Index(project, " as ") > -1 {
				parts := strings.Split(project, " as ")
				projectionsAdded = append(projectionsAdded, goqu.L(parts[0]).As(parts[1]))
			} else {
				projectionsAdded = append(projectionsAdded, goqu.L(project))
			}
		}
	}

	groupBysAdded := make([]interface{}, 0)
	for _, group := range requestedGroupBys {
		projections = append(projections, group)
		projectionsAdded = append(projectionsAdded, goqu.L(group))
		groupBysAdded = append(groupBysAdded, goqu.L(group))
	}

	if len(projections) == 0 {
		projectionsAdded = append(projectionsAdded, goqu.L("count(*)").As("count"))
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
				entityReferenceId := uuid.MustParse(rightValParts[1])
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
					return nil, fmt.Errorf("invalid function name in having clause - %s", leftValParts[0])
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
						entityReferenceId := uuid.MustParse(rightValParts[1])
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
