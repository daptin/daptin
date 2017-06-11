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
    log.Infof("Invoke BeforeFindAll [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())
    r, err := bf.InterceptBefore(dr, &req)
    if err != nil {
      log.Infof("Error from BeforeFindAll middleware [%v]: %v", bf.String(), err)
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

  reqFieldMap := make(map[string]bool)
  requestedFields, hasRequestedFields := req.QueryParams["fields"]
  if hasRequestedFields {
    for _, f := range requestedFields {
      reqFieldMap[f] = true
    }
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
    sortOrder = req.QueryParams["sort"]
  }

  queries := []string{}

  if len(req.QueryParams["query"]) > 0 {
    queries = req.QueryParams["query"];
  }

  if (pageNumber > 0) {
    pageNumber = pageNumber * pageSize
  }

  m := dr.model
  //log.Infof("Get all resource type: %v\n", m)

  cols := m.GetColumns()
  finalCols := make([]string, 0)
  //log.Infof("Cols: %v", cols)

  prefix := dr.model.GetName() + "."
  if hasRequestedFields {

    for _, col := range cols {
      if !col.ExcludeFromApi && reqFieldMap[col.Name] {
        finalCols = append(finalCols, prefix+col.ColumnName)
      }
    }
  } else {
    finalCols := []string{}
    for _, col := range cols {
      if col.ExcludeFromApi {
        continue
      }
      finalCols = append(finalCols, prefix+col.ColumnName)
    }
  }

  queryBuilder := squirrel.Select(finalCols...).From(m.GetTableName()).Where(squirrel.Eq{prefix + "deleted_at": nil}).Offset(pageNumber).Limit(pageSize)

  infos := dr.model.GetColumns()

  // todo: fix search in findall operation. currently no way to do an " or " query
  if len(queries) > 0 && false {
    for _, col := range infos {
      if col.IsIndexed {
        queryBuilder = queryBuilder.Where(squirrel.Eq{col.ColumnName: queries})
      }
    }
  }

  for key, values := range req.QueryParams {
    log.Infof("Query [%v] == %v", key, values)
  }

  for _, rel := range dr.model.GetRelations() {
    log.Infof("TableRelation[%v] == [%v]", dr.model.GetName(), rel.String())
    if rel.GetSubject() == dr.model.GetName() {

      log.Infof("Forward Relation %v", rel.String())
      queries, ok := req.QueryParams[rel.GetObject()+"_id"]
      if !ok || len(queries) < 1 {
        continue
      }

      objectNameList, ok := req.QueryParams[rel.GetObject()+"Name"]

      var objectName string
      /**
      api2go give us two params for each relationship
      <entityName> -> the name of the column which is used to reference, usually <entity>_id but you name it something for special relations in the config
       */
      if !ok {
        objectName = rel.GetObjectName()
      } else {
        objectName = objectNameList[0];
        if objectName != rel.GetSubjectName() {
          continue
        }
      }
      ids, err := dr.GetSingleColumnValueByReferenceId(rel.GetObject(), "id", "reference_id", queries)
      log.Infof("Converted ids: %v", ids)
      if err != nil {
        log.Errorf("Failed to convert refids to ids 2: %v", err)
        continue
      }
      switch rel.Relation {
      case "has_one":
        if len(ids) < 1 {
          continue
        }
        queryBuilder = queryBuilder.Where(squirrel.Eq{rel.GetObjectName(): ids})
        break;

      case "belongs_to":
        queryBuilder = queryBuilder.Where(squirrel.Eq{rel.GetObjectName(): ids})
        break

      case "has_many":
        wh := squirrel.Eq{}
        wh[rel.GetObject()+".id"] = ids
        queryBuilder = queryBuilder.Join(rel.GetJoinString()).Where(wh)

      }

    } else if rel.GetObject() == dr.model.GetName() {

      subjectNameList, ok := req.QueryParams[rel.GetSubject()+"Name"]
      log.Infof("Reverse Relation %v", rel.String())

      var subjectName string
      /**
      api2go give us two params for each relationship
      <entityName> -> the name of the column which is used to reference, usually <entity>_id but you name it something for special relations in the config
       */
      if !ok {
        subjectName = rel.GetSubjectName()
      } else {
        subjectName = subjectNameList[0];
        if subjectName != rel.GetObjectName() {
          continue
        }
      }

      switch rel.Relation {
      case "has_one":

        subjectId := req.QueryParams[rel.GetSubject()+"_id"]
        if len(subjectId) < 1 {
          continue
        }
        queryBuilder = queryBuilder.Join(rel.GetReverseJoinString()).Where(squirrel.Eq{rel.Subject + ".reference_id": subjectId })
        break;

      case "belongs_to":

        queries, ok := req.QueryParams[rel.GetSubject()+"_id"]
        log.Infof("%d Values as RefIds for relation [%v]", len(queries), rel.String())
        if !ok || len(queries) < 1 {
          continue
        }
        ids, err := dr.GetSingleColumnValueByReferenceId(rel.GetSubject(), "id", "reference_id", queries)
        if err != nil {
          log.Errorf("Failed to convert refids to ids[%v]: %v", queries, err)
          continue
        }

        queryBuilder = queryBuilder.Join(rel.GetReverseJoinString()).Where(squirrel.Eq{rel.GetSubject() + ".id": ids})
        break
      case "has_many":
        subjectId := req.QueryParams[rel.GetSubject()+"_id"]
        if len(subjectId) < 1 {
          continue
        }
        log.Infof("Has many [%v] : [%v] === %v", dr.model.GetName(), subjectId, req.QueryParams)
        queryBuilder = queryBuilder.Join(rel.GetReverseJoinString()).Where(squirrel.Eq{rel.Subject + ".reference_id": subjectId })

      }

    }
  }

  for _, so := range sortOrder {

    if len(so) < 1 {
      continue
    }
    //log.Infof("Sort order: %v", so)
    if so[0] == '-' {
      queryBuilder = queryBuilder.OrderBy(prefix + so[1:] + " desc")
    } else {
      queryBuilder = queryBuilder.OrderBy(prefix + so + " asc")
    }
  }

  sql1, args, err := queryBuilder.ToSql()
  if err != nil {
    log.Infof("Error: %v", err)
    return 0, nil, err
  }

  log.Infof("Sql: %v\n", sql1)

  rows, err := dr.db.Queryx(sql1, args...)

  if err != nil {
    log.Infof("Error: %v", err)
    return 0, nil, err
  }
  defer rows.Close()

  results, includes, err := dr.ResultToArrayOfMap(rows)
  //log.Infof("Results: %v", results)

  if err != nil {
    return 0, nil, err
  }

  // todo: handle fetching of usergroups, because world permission
  for _, bf := range dr.ms.AfterFindAll {
    log.Infof("Invoke AfterFindAll [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())

    results, err = bf.InterceptAfter(dr, &req, results)
    if err != nil {
      log.Errorf("Error from findall paginated create middleware: %v", err)
    }
  }

  includesNew := make([][]map[string]interface{}, 0)
  for _, bf := range dr.ms.AfterFindAll {
    log.Infof("Invoke AfterFindAll Includes [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())

    for _, include := range includes {
      include, err = bf.InterceptAfter(dr, &req, include)
      if err != nil {
        log.Errorf("Error from AfterFindAll middleware: %v", err)
      }
      includesNew = append(includesNew, include)
    }

  }

  result := make([]*api2go.Api2GoModel, 0)

  for i, res := range results {
    delete(res, "id")
    includes := includesNew[i]
    var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission(), dr.model.GetRelations())
    a.Data = res

    for _, include := range includes {
      delete(include, "id")
      perm, ok := include["permission"].(int64)
      if !ok {
        log.Errorf("Failed to parse permission, skipping record: %v", err)
        continue
      }

      delete(include, "id")
      delete(include, "deleted_at")
      model := api2go.NewApi2GoModelWithData(include["__type"].(string), nil, int64(perm), nil, include)

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
    Next:        map[string]string{"limit": fmt.Sprintf("%v", pageSize), "offset": fmt.Sprintf("%v", pageSize+pageNumber)},
    Prev:        map[string]string{"limit": fmt.Sprintf("%v", pageSize), "offset": fmt.Sprintf("%v", pageNumber-pageSize)},
    First:       map[string]string{},
    Last:        map[string]string{"limit": fmt.Sprintf("%v", pageSize), "offset": fmt.Sprintf("%v", total-pageSize)},
    Total:       total1,
    PerPage:     pageSize,
    CurrentPage: 1 + (pageNumber / pageSize),
    LastPage:    1 + (total1 / pageSize),
    From:        pageNumber + 1,
    To:          pageSize,
  }), nil

}
