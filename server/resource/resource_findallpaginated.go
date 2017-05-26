package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  "gopkg.in/Masterminds/squirrel.v1"
  "strconv"
  "fmt"
)

func (dr *DbResource) GetTotalCount() uint64 {
  s, v, err := squirrel.Select("count(*)").From(dr.model.GetName()).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    log.Errorf("Failed to generate count query for %v: %v", dr.model.GetName(), err)
    return 0
  }

  var count uint64
  dr.db.QueryRowx(s, v...).Scan(&count)
  //log.Infof("Count: [%v] %v", dr.model.GetTableName(), count)
  return count
}

// PaginatedFindAll(req Request) (totalCount uint, response Responder, err error)
func (dr *DbResource) PaginatedFindAll(req api2go.Request) (totalCount uint, response api2go.Responder, err error) {


  for _, bf := range dr.ms.BeforeFindAll {
    r, err := bf.InterceptBefore(dr, &req)
    if err != nil {
      log.Errorf("Error from before findall paginated middleware: %v", err)
      return 0, nil, err
    }
    if r != nil {
      return 0, r, err
    }
  }

  offset := uint64(0)
  if len(req.QueryParams["page[offset]"]) > 0 {
    offset, err = strconv.ParseUint(req.QueryParams["page[offset]"][0], 10, 32)
    if err != nil {
      log.Errorf("Invalid parameter value: %v", req.QueryParams["page[offset]"])
    }
  }

  limit := uint64(50)
  if len(req.QueryParams["page[limit]"]) > 0 {
    limit, err = strconv.ParseUint(req.QueryParams["page[limit]"][0], 10, 32)
    if err != nil {
      log.Errorf("Invalid parameter value: %v", req.QueryParams["page[limit]"])
    }
  }

  m := dr.model
  //log.Infof("Get all resource type: %v\n", m)

  cols := m.GetColumnNames()
  queryBuilder := squirrel.Select(cols...).From(m.GetTableName()).Where(squirrel.Eq{"deleted_at": nil}).Offset(offset).Limit(limit)
  sql1, args, err := queryBuilder.ToSql()
  if err != nil {
    log.Infof("Error: %v", err)
    return 0, nil, err
  }

  log.Infof("Sql: %v\n", sql1)

  rows, err := dr.db.Query(sql1, args...)

  if err != nil {
    log.Infof("Error: %v", err)
    return 0, nil, err
  }

  result := make([]*api2go.Api2GoModel, 0)

  results, err := dr.ResultToArrayOfMap(rows)
  if err != nil {
    return 0, nil, err
  }

  infos := dr.model.GetColumns()

  for _, bf := range dr.ms.AfterFindAll {
    results, err = bf.InterceptAfter(dr, &req, results)
    if err != nil {
      log.Errorf("Error from findall paginated create middleware: %v", err)
    }
  }

  for _, res := range results {
    var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission(), dr.model.GetRelations())
    a.Data = res
    result = append(result, a)
  }

  total := dr.GetTotalCount()
  if total < limit {
    total = limit
  }
  if offset < limit {
    offset = limit
  }
  log.Infof("Offset, limit: %v, %v", offset, limit)

  return uint(dr.GetTotalCount()), NewResponse(nil, result, 200, &api2go.Pagination{
    Next:  map[string]string{"limit": fmt.Sprintf("%v", limit), "offset":  fmt.Sprintf("%v", limit + offset)},
    Prev:  map[string]string{"limit":  fmt.Sprintf("%v", limit), "offset":  fmt.Sprintf("%v", offset - limit)},
    First: map[string]string{},
    Last:  map[string]string{"limit":  fmt.Sprintf("%v", limit), "offset":  fmt.Sprintf("%v", total - limit)},
  }), nil

}

