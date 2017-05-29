package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  "gopkg.in/Masterminds/squirrel.v1"
  "strconv"
  "fmt"
  "strings"
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
  log.Infof("Request [%v]: %v", dr.model.GetName(), req.QueryParams)

  pageNumber := uint64(0)
  if len(req.QueryParams["page[number]"]) > 0 {
    pageNumber, err = strconv.ParseUint(req.QueryParams["page[number]"][0], 10, 32)
    if err != nil {
      log.Errorf("Invalid parameter value: %v", req.QueryParams["page[number]"])
    }
    pageNumber -= 1
  }

  pageSize := uint64(10)
  if len(req.QueryParams["page[size]"]) > 0 {
    pageSize, err = strconv.ParseUint(req.QueryParams["page[size]"][0], 10, 32)
    if err != nil {
      log.Errorf("Invalid parameter value: %v", req.QueryParams["page[size]"])
    }
  }

  if pageSize == 0 {
    return uint(dr.GetTotalCount()), nil, nil
  }

  sortOrder := []string{}
  if len(req.QueryParams["sort"]) > 0 {
    sortOrder = strings.Split(req.QueryParams["sort"][0], ",")
  }

  if (pageNumber > 0) {
    pageNumber = pageNumber * pageSize
  }

  m := dr.model
  //log.Infof("Get all resource type: %v\n", m)

  cols := m.GetColumnNames()
  //log.Infof("Cols: %v", cols)
  queryBuilder := squirrel.Select(cols...).From(m.GetTableName()).Where(squirrel.Eq{"deleted_at": nil}).Offset(pageNumber).Limit(pageSize)

  for key, values := range req.QueryParams {
    log.Infof("Query [%v] == %v", key, values)
  }

  for _, so := range sortOrder {

    if len(so) < 1 {
      continue
    }
    //log.Infof("Sort order: %v", so)
    if so[0] == '-' {
      queryBuilder = queryBuilder.OrderBy(so[1:] + " desc")
    } else {
      queryBuilder = queryBuilder.OrderBy(so + " asc")
    }
  }

  sql1, args, err := queryBuilder.ToSql()
  if err != nil {
    log.Infof("Error: %v", err)
    return 0, nil, err
  }

  //log.Infof("Sql: %v\n", sql1)

  rows, err := dr.db.Query(sql1, args...)
  defer rows.Close()

  if err != nil {
    log.Infof("Error: %v", err)
    return 0, nil, err
  }

  results, includes, err := dr.ResultToArrayOfMap(rows)
  //log.Infof("Results: %v", results)

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

  includesNew := make([][]map[string]interface{}, 0)
  for _, bf := range dr.ms.AfterFindAll {

    for _, include := range includes {
      include, err = bf.InterceptAfter(dr, &req, include)
      if err != nil {
        log.Errorf("Error from findall paginated create middleware: %v", err)
      }
      includesNew = append(includesNew, include)
    }

  }

  result := make([]*api2go.Api2GoModel, 0)

  for i, res := range results {
    includes := includesNew[i]
    var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission(), dr.model.GetRelations())
    a.Data = res

    for _, include := range includes {
      perm, err := strconv.ParseInt(include["permission"].(string), 10, 64)
      if err != nil {
        log.Errorf("Failed to parse permission, skipping record: %v", err)
        continue
      }

      delete(include, "id")
      delete(include, "deleted_at")
      model := api2go.NewApi2GoModelWithData(include["__type"].(string), nil, int(perm), nil, include)

      a.Includes = append(a.Includes, model)
    }

    result = append(result, a)
  }

  total1 := dr.GetTotalCount()
  total := total1
  if total < pageSize {
    total = pageSize
  }
  if pageNumber < pageSize {
    pageNumber = pageSize
  }
  //log.Infof("Offset, limit: %v, %v", pageNumber, pageSize)

  return uint(dr.GetTotalCount()), NewResponse(nil, result, 200, &api2go.Pagination{
    Next:  map[string]string{"limit": fmt.Sprintf("%v", pageSize), "offset":  fmt.Sprintf("%v", pageSize + pageNumber)},
    Prev:  map[string]string{"limit":  fmt.Sprintf("%v", pageSize), "offset":  fmt.Sprintf("%v", pageNumber - pageSize)},
    First: map[string]string{},
    Last:  map[string]string{"limit":  fmt.Sprintf("%v", pageSize), "offset":  fmt.Sprintf("%v", total - pageSize)},
    Total: total1,
    PerPage: pageSize,
    CurrentPage: 1 + (pageNumber / pageSize),
    LastPage: 1 + (total1 / pageSize),
    From: pageNumber + 1,
    To: pageSize,
  }), nil

}

