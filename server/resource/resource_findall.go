package resource
//
//import (
//  "github.com/artpar/api2go"
//  log "github.com/Sirupsen/logrus"
//  "gopkg.in/Masterminds/squirrel.v1"
//  "strconv"
//  "fmt"
//)
//
//// PaginatedFindAll(req Request) (totalCount uint, response Responder, err error)
//func (dr *DbResource) FindAll(req api2go.Request) (response api2go.Responder, err error) {
//
//  for _, bf := range dr.ms.BeforeFindAll {
//    r, err := bf.InterceptBefore(dr, &req)
//    if err != nil {
//      log.Errorf("Error from before findall paginated middleware: %v", err)
//      return nil, err
//    }
//    if r != nil {
//      return r, err
//    }
//  }
//
//  pageNumber := uint64(0)
//  if len(req.QueryParams["page[number]"]) > 0 {
//    pageNumber, err = strconv.ParseUint(req.QueryParams["page[number]"][0], 10, 32)
//    if err != nil {
//      log.Errorf("Invalid parameter value: %v", req.QueryParams["page[number]"])
//    }
//    pageNumber -= 1
//  }
//
//  pageSize := uint64(10)
//  if len(req.QueryParams["page[size]"]) > 0 {
//    pageSize, err = strconv.ParseUint(req.QueryParams["page[size]"][0], 10, 32)
//    if err != nil {
//      log.Errorf("Invalid parameter value: %v", req.QueryParams["page[size]"])
//    }
//  }
//
//  if (pageNumber > 0) {
//    pageNumber = pageNumber * pageSize
//  }
//
//  m := dr.model
//  //log.Infof("Get all resource type: %v\n", m)
//
//  cols := m.GetColumnNames()
//  queryBuilder := squirrel.Select(cols...).From(m.GetTableName()).Where(squirrel.Eq{"deleted_at": nil}).Offset(pageNumber).Limit(pageSize)
//  sql1, args, err := queryBuilder.ToSql()
//  if err != nil {
//    log.Infof("Error: %v", err)
//    return nil, err
//  }
//
//  log.Infof("Sql: %v\n", sql1)
//
//  rows, err := dr.db.Query(sql1, args...)
//  defer rows.Close()
//
//  if err != nil {
//    log.Infof("Error: %v", err)
//    return nil, err
//  }
//
//  result := make([]*api2go.Api2GoModel, 0)
//
//  results, err := dr.ResultToArrayOfMap(rows)
//
//  if err != nil {
//    return nil, err
//  }
//
//  infos := dr.model.GetColumns()
//
//  for _, bf := range dr.ms.AfterFindAll {
//    results, err = bf.InterceptAfter(dr, &req, results)
//    if err != nil {
//      log.Errorf("Error from findall paginated create middleware: %v", err)
//    }
//  }
//
//  for _, res := range results {
//    var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission(), dr.model.GetRelations())
//    a.Data = res
//    result = append(result, a)
//  }
//
//  total1 := dr.GetTotalCount()
//  total := total1
//  if total < pageSize {
//    total = pageSize
//  }
//  if pageNumber < pageSize {
//    pageNumber = pageSize
//  }
//  log.Infof("Offset, limit: %v, %v", pageNumber, pageSize)
//
//  return NewResponse(nil, result, 200, &api2go.Pagination{
//    Next:  map[string]string{"limit": fmt.Sprintf("%v", pageSize), "offset":  fmt.Sprintf("%v", pageSize + pageNumber)},
//    Prev:  map[string]string{"limit":  fmt.Sprintf("%v", pageSize), "offset":  fmt.Sprintf("%v", pageNumber - pageSize)},
//    First: map[string]string{},
//    Last:  map[string]string{"limit":  fmt.Sprintf("%v", pageSize), "offset":  fmt.Sprintf("%v", total - pageSize)},
//    Total: total1,
//    PerPage: pageSize,
//    CurrentPage: 1 + (pageNumber / pageSize),
//    LastPage: 1 + (total1 / pageSize),
//    From: pageNumber + 1,
//    To: pageSize,
//  }), nil
//
//}
//
