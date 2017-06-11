package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  //"reflect"
  "gopkg.in/Masterminds/squirrel.v1"
  "time"
  "errors"
  "net/http"
  "github.com/artpar/goms/server/auth"
)

// Update an object
// Possible Responder status codes are:
// - 200 OK: Update successful, however some field(s) were changed, returns updates source
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Update was successful, no fields were changed by the server, return nothing
func (dr *DbResource) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {

  for _, bf := range dr.ms.BeforeUpdate {
    log.Infof("Invoke BeforeUpdate [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())

    r, err := bf.InterceptBefore(dr, &req)
    if err != nil {
      log.Errorf("Error from BeforeUpdate middleware: %v", err)
      return nil, err
    }
    if r != nil {
      return r, err
    }
  }

  data, ok := obj.(*api2go.Api2GoModel)
  if !ok {
    log.Errorf("Request data is not api2go model: %v", data)
    return nil, errors.New("Invalid request");
  }
  log.Infof("Update object request: %v", data.Data)
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
    if !ok || val == nil {
      continue
    }
    if col.IsForeignKey {
      log.Infof("Convert ref id to id %v[%v]", col.ForeignKeyData.TableName, val)

      valString := val.(string)
      var uId interface{}
      var err error
      if valString == "" {
        uId = nil
      } else {
        uId, err = dr.GetReferenceIdToId(col.ForeignKeyData.TableName, valString)
      }
      if err != nil {
        return nil, err
      }
      val = uId
    }

    if ok {
      dataToInsert[col.ColumnName] = val
      colsList = append(colsList, col.ColumnName)
      valsList = append(valsList, val)
    }

  }

  colsList = append(colsList, "updated_at")
  valsList = append(valsList, time.Now())

  builder := squirrel.Update(dr.model.GetName())

  for i, _ := range colsList {
    //log.Infof("cols to set: %v == %v", colsList[i], valsList[i])
    builder = builder.Set(colsList[i], valsList[i])
  }

  query, vals, err := builder.Where(squirrel.Eq{"reference_id": id}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    log.Errorf("Failed to create update query: %v", err)
    return NewResponse(nil, nil, 500, nil), err
  }

  //log.Infof("Update query: %v", query)
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

  for _, rel := range dr.model.GetRelations() {
    relationName := rel.GetRelation()
    log.Infof("Check relation in Update: %v", rel.String())
    if rel.GetSubject() == dr.model.GetName() {

      if relationName == "belongs_to" || relationName == "has_one" {
        continue
      }

      val11, ok := attrs[rel.GetObjectName()]
      if !ok || len(val11.([]map[string]interface{})) < 1 {
        continue
      }
      log.Infof("Update object for relation on [%v] : [%v]", rel.GetObjectName(), val11)

      valueList := val11.([]map[string]interface{})
      switch relationName {
      case "has_one":
      case "belongs_to":
        break;

      case "has_many_and_belongs_to_many":
      case "has_many":

        for _, item := range valueList {
          obj := make(map[string]interface{})
          obj[rel.GetObjectName()] = item[rel.GetObjectName()]
          obj[rel.GetSubjectName()] = updatedResource["reference_id"]

          modl := api2go.NewApi2GoModelWithData(rel.GetJoinTableName(), nil, auth.DEFAULT_PERMISSION, nil, obj)
          req := api2go.Request{
            PlainRequest: &http.Request{
              Method: "POST",
            },
          }
          _, err := dr.cruds[rel.GetJoinTableName()].Create(modl, req)
          if err != nil {
            log.Errorf("Failed to insert join table data [%v] : %v", rel.GetJoinTableName(), err)
            continue
          }

        }

        break;

      default:
        log.Errorf("Unknown relation: %v", relationName)
      }

    } else {

      val, ok := attrs[rel.GetSubjectName()]
      if !ok {
        continue
      }
      log.Infof("Update %v on: %v", rel.String(), val)

      var relUpdateQuery string
      var vars []interface{}
      switch relationName {
      case "has_one":
        intId, err := dr.GetReferenceIdToId(rel.GetObject(), id)
        if err != nil {
          log.Errorf("Subject not found to update: %v", err)
          continue
        }

        log.Infof("Converted ids for [%v]: %v", rel.GetObject(), intId)
        relUpdateQuery, vars, err = squirrel.Update(rel.GetSubject()).
            Set(rel.GetObjectName(), intId).Where(squirrel.Eq{"reference_id": val}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
        if err != nil {
          log.Errorf("Failed to make update query: %v", err)
          continue
        }
        log.Infof("Relation update query params: %v", vars)

        break;
      case "belongs_to":
        relUpdateQuery, vars, err = squirrel.Update(rel.GetSubject()).
            Set(rel.GetObjectName(), val).Where(squirrel.Eq{"reference_id": id}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
        if err != nil {
          log.Errorf("Failed to make update query: %v", err)
          continue
        }

        break;

      case "has_many":
        obj := make(map[string]interface{})
        obj[rel.GetSubjectName()] = val
        obj[rel.GetObjectName()] = updatedResource["id"]

        req := api2go.Request{
          PlainRequest: &http.Request{
            Method: "POST",
          },
        }
        _, err := dr.Create(obj, req)
        if err != nil {
          log.Errorf("Failed to insert join table data [%v] : %v", rel.GetJoinTableName(), err)
          continue
        }
      case "has_many_and_belongs_to_many":

        obj := make(map[string]interface{})
        obj[rel.GetSubjectName()] = val
        obj[rel.GetObjectName()] = updatedResource["id"]

        req := api2go.Request{
          PlainRequest: &http.Request{
            Method: "POST",
          },
        }
        _, err := dr.Create(obj, req)
        if err != nil {
          log.Errorf("Failed to insert join table data [%v] : %v", rel.GetJoinTableName(), err)
          continue
        }

        //relUpdateQuery, vars, err = squirrel.Insert(rel.GetJoinTableName()).Columns(rel.GetSubjectName(), rel.GetObjectName(), "reference_id").Values(val, updatedResource["id"], uuid.NewV4().String()).ToSql()
        //if err != nil {
        //  log.Errorf("Failed to make update query: %v", err)
        //  continue
        //}

        break;

      default:
        log.Errorf("Unknown relation: %v", relationName)
      }

      _, err = dr.db.Exec(relUpdateQuery, vars...)
      if err != nil {
        log.Errorf("Failed to execute update query for relation: %v", err)
      }

    }
  }
  //

  for _, bf := range dr.ms.AfterUpdate {
    log.Infof("Invoke AfterUpdate [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())

    results, err := bf.InterceptAfter(dr, &req, []map[string]interface{}{updatedResource})
    if len(results) != 0 {
      updatedResource = results[0]
    } else {
      updatedResource = nil
    }

    if err != nil {
      log.Errorf("Error from AfterUpdate middleware: %v", err)
    }
  }
  delete(updatedResource, "id")

  //for k, v := range updatedResource {
  //  k1 := reflect.TypeOf(v)
  //  //log.Infof("K: %v", k1)
  //  if v != nil && k1.Kind() == reflect.Slice {
  //    updatedResource[k] = string(v.([]uint8))
  //  }
  //}

  //log.Infof("Create response: %v", m)

  return NewResponse(nil, api2go.NewApi2GoModelWithData(dr.model.GetName(), dr.model.GetColumns(), dr.model.GetDefaultPermission(), dr.model.GetRelations(), updatedResource), 200, nil), nil

}
