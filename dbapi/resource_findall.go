package dbapi

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  "gopkg.in/Masterminds/squirrel.v1"
)

func (dr *DbResource) FindAll(req api2go.Request) (api2go.Responder, error) {
  m := dr.model
  log.Infof("Get all resource type: %v\n", m)

  cols := m.GetColumnNames()
  queryBuilder := squirrel.Select(cols...).From(m.GetTableName()).Where(squirrel.Eq{"deleted_at": nil})
  sql1, args, err := queryBuilder.ToSql()
  if err != nil {
    log.Infof("Error: %v", err)
    return nil, err
  }

  log.Infof("Sql: %v\n", sql1)

  rows, err := dr.db.Query(sql1, args...)

  if err != nil {
    log.Infof("Error: %v", err)
    return nil, err
  }

  result := make([]*api2go.Api2GoModel, 0)

  results, err := dr.ResultToArrayOfMap(rows)
  if err != nil {
    return nil, err
  }

  infos := dr.model.GetColumns()

  for _, res := range results {
    var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission())
    a.Data = res
    result = append(result, a)
  }

  return NewResponse(nil, result, 200), nil

}

