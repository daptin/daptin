package server

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  "reflect"
  "gopkg.in/Masterminds/squirrel.v1"
)

// Update an object
// Possible Responder status codes are:
// - 200 OK: Update successful, however some field(s) were changed, returns updates source
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Update was successful, no fields were changed by the server, return nothing
func (dr *DbResource) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {

  data := obj.(*api2go.Api2GoModel)
  log.Infof("Update object request: %v", data)
  id := data.GetID()

  attrs := data.GetAttributes()

  allColumns := dr.model.GetColumns()

  dataToInsert := make(map[string]interface{})

  colsList := []string{}
  valsList := []interface{}{}
  for _, col := range allColumns {

    //log.Infof("Add column: %v", col.ColumnName)
    if col.IsAutoIncrement {
      continue
    }

    if col.ColumnName == "created_at" {
      continue
    }

    if col.ColumnName == "deleted_at" {
      continue
    }

    if col.ColumnName == "reference_id" {
      continue
    }

    if col.ColumnName == "updated_at" {
      continue
    }

    //log.Infof("Check column: %v", col.ColumnName)

    val, ok := attrs[col.ColumnName]

    if ok {
      dataToInsert[col.ColumnName] = val
      colsList = append(colsList, col.ColumnName)
      valsList = append(valsList, val)
    }

  }

  builder := squirrel.Update(dr.model.GetName())

  for i, _ := range colsList {
    builder.Set(colsList[i], valsList[i])
  }

  query, vals, err := builder.Where(squirrel.Eq{"reference_id": id}).ToSql()
  if err != nil {
    log.Errorf("Failed to create update query: %v", err)
    return NewResponse(nil, nil, 500), err
  }

  log.Infof("Update query: %v", query)
  _, err = dr.db.Exec(query, vals...)
  if err != nil {
    log.Errorf("Failed to execute update query: %v", err)
    return NewResponse(nil, nil, 500), err
  }

  query, vals, err = squirrel.Select("*").From(dr.model.GetName()).Where(squirrel.Eq{"reference_id": id}).ToSql()
  if err != nil {
    log.Errorf("Failed to create select query: %v", err)
    return nil, err
  }

  m := make(map[string]interface{})
  dr.db.QueryRowx(query, vals...).MapScan(m)

  for k, v := range m {
    k1 := reflect.TypeOf(v)
    //log.Infof("K: %v", k1)
    if v != nil && k1.Kind() == reflect.Slice {
      m[k] = string(v.([]uint8))
    }
  }

  //log.Infof("Create response: %v", m)

  return NewResponse(nil, api2go.NewApi2GoModelWithData(dr.model.GetName(), dr.model.GetColumns(), dr.model.GetDefaultPermission(), m), 201), nil

  return NewResponse(nil, StatusResponse{"ok"}, 200), nil
}

