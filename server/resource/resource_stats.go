package resource

import (
	"gopkg.in/Masterminds/squirrel.v1"
	"regexp"
	"github.com/pkg/errors"
	"fmt"
	"strings"
	"sort"
	"reflect"
)

type TimeStamp string

type AggregationRequest struct {
	RootEntity    string
	Join          []string
	GroupBy       []string
	ProjectColumn []string
	Filter        []string
	Order         []string
	TimeSample    TimeStamp
	TimeFrom      string
	TimeTo        string
}

// PaginatedFindAll(req Request) (totalCount uint, response Responder, err error)
type AggregateData struct {
	Data []map[string]interface{} `json:"data"`
}

func InArray(val interface{}, array interface{}) (exists bool) {
	exists = false

	switch reflect.TypeOf(array).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(array)

		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(val, s.Index(i).Interface()) == true {
				exists = true
				return
			}
		}
	}

	return
}

func (dr *DbResource) DataStats(req AggregationRequest) (AggregateData, error) {

	sort.Strings(req.GroupBy)
	projections := req.ProjectColumn
	for _, group := range req.GroupBy {
		projections = append(projections, group)
	}
	selectBuilder := squirrel.Select(projections...)
	builder := selectBuilder.From(req.RootEntity)
	builder = builder.GroupBy(req.GroupBy...)

	builder = builder.OrderBy(req.Order...)

	// functionName(param1, param2)
	querySyntax, err := regexp.Compile("([a-zA-Z0-9]+)\\( *([^,]+) *, *([^)]+) *\\)")
	CheckErr(err, "Failed to build query regex")
	for _, filter := range req.Filter {

		if !querySyntax.MatchString(filter) {
			CheckErr(errors.New("Invalid filter syntax"), "Failed to parse query [%v]", filter)
		} else {

			parts := querySyntax.FindStringSubmatch(filter)

			functionName := parts[1]
			leftVal := strings.TrimSpace(parts[2])
			rightVal := strings.TrimSpace(parts[3])

			function := builder.Where
			if len(req.GroupBy) > 0 {
				function = builder.Having
			}

			switch functionName {
			case "eq":
				builder = function(squirrel.Eq{leftVal: rightVal})
			case "neq":
				builder = function(fmt.Sprintf("%s != %s", leftVal, rightVal))
			case "le":
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

	//log.Infof("Stats query: %v == %v", sql, args)
	res, err := dr.db.Queryx(sql, args...)
	CheckErr(err, "Failed to query stats: %v", err)
	if err != nil {
		return AggregateData{}, err
	}

	rows, err := RowsToMap(res, "aggregate_"+req.RootEntity)
	CheckErr(err, "Failed to scan ")

	return AggregateData{
		Data: rows,
	}, err

}
