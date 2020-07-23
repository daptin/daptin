package resource

import (
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
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

func InArray(val []string, ar string) (exists bool) {
	exists = false

	for _, v := range val {
		if v == ar {
			return true
		}
	}
	return false
}

func (dr *DbResource) DataStats(req AggregationRequest) (AggregateData, error) {

	sort.Strings(req.GroupBy)
	projections := req.ProjectColumn

	for i, project := range projections {
		if project == "count" {
			projections[i] = "count(*) as count"
		}
	}

	for _, group := range req.GroupBy {
		projections = append(projections, group)
	}

	if len(projections) == 0 {
		projections = append(projections, "count(*) as count")
	}

	selectBuilder := statementbuilder.Squirrel.Select(projections...)
	builder := selectBuilder.From(req.RootEntity)
	builder = builder.GroupBy(req.GroupBy...)

	builder = builder.OrderBy(req.Order...)

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

			function := builder.Where
			if len(req.GroupBy) > 0 {
				//function = builder.Having
			}

			switch functionName {
			case "eq":
				builder = function(squirrel.Eq{leftVal: rightVal})
			case "neq":
				builder = function(fmt.Sprintf("%s != %s", leftVal, rightVal))
			case "lt":
				builder = function(fmt.Sprintf("%s < %s", leftVal, rightVal))
			case "lte":
				builder = function(fmt.Sprintf("%s <= %s", leftVal, rightVal))
			case "gt":
				builder = function(fmt.Sprintf("%s > %s", leftVal, rightVal))
			case "gte":
				builder = function(fmt.Sprintf("%s >= %s", leftVal, rightVal))
			case "like":
				builder = function(fmt.Sprintf("%s LIKE %s", leftVal, rightVal))
			case "in":
				builder = function(squirrel.Eq{leftVal: strings.Split(rightVal, ",")})
			case "notin":
				builder = function(squirrel.Eq{leftVal: strings.Split(rightVal, ",")})
			}
		}
	}

	sql, args, err := builder.ToSql()
	CheckErr(err, "Failed to generate stats sql: [%v]")
	if err != nil {
		return AggregateData{}, err
	}

	log.Infof("Stats query: %v == %v", sql, args)
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
