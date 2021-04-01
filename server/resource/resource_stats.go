package resource

import (
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/pkg/errors"
	"regexp"
	"sort"
	"strings"
)

type TimeStamp string

type AggregationRequest struct {
	RootEntity    string
	Join          []string
	GroupBy       []string
	ProjectColumn []string
	Filter        []string
	Query         []Query
	Order         []string
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

func ToInterfaceArray(s []string) []interface{} {
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

func (dr *DbResource) DataStats(req AggregationRequest) (AggregateData, error) {

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
		projectionsAdded = append(projectionsAdded, goqu.I(group))
	}

	if len(projections) == 0 {
		projectionsAdded = append(projectionsAdded, goqu.L("count(*)").As("count"))
	}

	selectBuilder := statementbuilder.Squirrel.Select(projectionsAdded...)
	builder := selectBuilder.From(req.RootEntity)

	for _, group := range req.GroupBy {
		builder = builder.GroupBy(group)
	}

	for _, order := range req.Order {

		builder = builder.Order(goqu.C(order).Asc())
	}

	// functionName(param1, param2)
	querySyntax, err := regexp.Compile("([a-zA-Z0-9]+)\\(([^,]+?),(.+)\\)")
	CheckErr(err, "Failed to build query regex")
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

			if functionName == "in" || functionName == "notin" {
				rightValInterface = strings.Split(rightVal, ",")
			}

			builder = builder.Where(goqu.Ex{leftVal: goqu.Op{functionName: rightValInterface}})
		}
	}

	sql, args, err := builder.ToSQL()
	CheckErr(err, "Failed to generate stats sql: [%v]")
	if err != nil {
		return AggregateData{}, err
	}

	//log.Infof("Stats query: %v == %v", sql, args)
	res, err := dr.db.Queryx(sql, args...)
	CheckErr(err, "Failed to query stats: %v", err)
	if err != nil {
		return AggregateData{}, err
	}
	defer res.Close()

	rows, err := RowsToMap(res, "aggregate_"+req.RootEntity)
	CheckErr(err, "Failed to scan ")

	return AggregateData{
		Data: rows,
	}, err

}
