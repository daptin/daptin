package resource

import (
	"gopkg.in/Masterminds/squirrel.v1"
	"regexp"
	"github.com/pkg/errors"
	"fmt"
	//log "github.com/sirupsen/logrus"
	"strings"
)

type TimeStamp string

type AggregationRequest struct {
	RootEntity    string
	Join          []string
	GroupBy       []string
	ProjectColumn []string
	Filter        []string
	TimeSample    TimeStamp
	TimeFrom      string
	TimeTo        string
}

// PaginatedFindAll(req Request) (totalCount uint, response Responder, err error)
type AggregateData struct {
	Data []map[string]interface{}
}

func (dr *DbResource) DataStats(req AggregationRequest) (AggregateData, error) {

	builder := squirrel.Select(req.ProjectColumn...).From(req.RootEntity)
	builder = builder.GroupBy(req.GroupBy...)

	querySyntax, err := regexp.Compile("([a-zA-Z0-9]+)\\( *([^, ]) *, *([^ ]) *\\)")
	CheckErr(err, "Failed to build query regex")
	for _, filter := range req.Filter {

		if !querySyntax.MatchString(filter) {
			CheckErr(errors.New("Invalid filter syntax"), "Failed to parse query [%v]", filter)
		} else {

			parts := querySyntax.FindStringSubmatch(filter)

			functionName := parts[1]
			leftVal := parts[2]
			rightVal := parts[3]

			switch functionName {
			case "eq":
				builder = builder.Where(squirrel.Eq{leftVal: rightVal})
			case "neq":
				builder = builder.Where("? != ?", leftVal, rightVal)
			case "le":
				builder = builder.Where("? < ?", leftVal, rightVal)
			case "lte":
				builder = builder.Where("? <= ?", leftVal, rightVal)
			case "gt":
				builder = builder.Where("? > ?", leftVal, rightVal)
			case "gte":
				builder = builder.Where("? >= ?", leftVal, rightVal)
			case "like":
				builder = builder.Where("? LIKE ?", leftVal, fmt.Sprint("%", rightVal, "%"))
			case "in":
				builder = builder.Where(squirrel.Eq{leftVal: strings.Split(rightVal, ",")})
			case "notin":
				builder = builder.Where(squirrel.Eq{leftVal: strings.Split(rightVal, ",")})
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
