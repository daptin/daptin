package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  "reflect"
  "gopkg.in/Masterminds/squirrel.v1"
  "time"
)

// Update an object
// Possible Responder status codes are:
// - 200 OK: Update successful, however some field(s) were changed, returns updates source
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Update was successful, no fields were changed by the server, return nothing
func (dr *DbResource) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {

  for _, bf := range dr.ms.BeforeUpdate {
    r, err := bf.InterceptBefore(dr, &req)
    if err != nil {
      log.Errorf("Error from before update middleware: %v", err)
      return nil, err
    }
    if r != nil {
      return r, err
    }
  }

  data := obj.(*api2go.Api2GoModel)
  log.Infof("Update object request: %v", data)
  id := data.GetID()

  attrs := data.GetAllAsAttributes()

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
    if !ok {
      continue
    }
    if col.IsForeignKey {
      log.Infof("Convert ref id to id %v[%v]", col.ForeignKeyData.TableName, val)
      uId, err := dr.GetReferenceIdToId(col.ForeignKeyData.TableName, val.(string))
      if err != nil {
        return nil, err
      }
      val = uId
    }

    if ok  {
      dataToInsert[col.ColumnName] = val
      colsList = append(colsList, col.ColumnName)
      valsList = append(valsList, val)
    }

  }

  colsList = append(colsList, "updated_at")
  valsList = append(valsList, time.Now())

  builder := squirrel.Update(dr.model.GetName())

  for i, _ := range colsList {
    log.Infof("cols to set: %v == %v", colsList[i], valsList[i])
    builder = builder.Set(colsList[i], valsList[i])
  }

  query, vals, err := builder.Where(squirrel.Eq{"reference_id": id}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    log.Errorf("Failed to create update query: %v", err)
    return NewResponse(nil, nil, 500, nil), err
  }

  log.Infof("Update query: %v", query)
  _, err = dr.db.Exec(query, vals...)
  if err != nil {
    log.Errorf("Failed to execute update query: %v", err)
    return NewResponse(nil, nil, 500, nil), err
  }

  query, vals, err = squirrel.Select("*").From(dr.model.GetName()).Where(squirrel.Eq{"reference_id": id}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    log.Errorf("Failed to create select query: %v", err)
    return nil, err
  }

  updatedResource, err := dr.GetReferenceIdToObject(dr.model.GetName(), id)
  if err != nil {
    log.Errorf("Failed to select the newly created entry: %v", err)
    return nil, err
  }
  //

  for _, bf := range dr.ms.AfterUpdate {
    results, err := bf.InterceptAfter(dr, &req, []map[string]interface{}{updatedResource})
    if len(results) != 0 {
      updatedResource = results[0]
    } else {
      updatedResource = nil
    }

    if err != nil {
      log.Errorf("Error from after create middleware: %v", err)
    }
  }

  for k, v := range updatedResource {
    k1 := reflect.TypeOf(v)
    //log.Infof("K: %v", k1)
    if v != nil && k1.Kind() == reflect.Slice {
      updatedResource[k] = string(v.([]uint8))
    }
  }

  //log.Infof("Create response: %v", m)

  return NewResponse(nil, api2go.NewApi2GoModelWithData(dr.model.GetName(), dr.model.GetColumns(), dr.model.GetDefaultPermission(), dr.model.GetRelations(), updatedResource), 200, nil), nil

}

