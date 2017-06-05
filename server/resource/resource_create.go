package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  "gopkg.in/Masterminds/squirrel.v1"
  //"reflect"
  "github.com/satori/go.uuid"
  "github.com/gorilla/context"
  //"strconv"
)

// Create a new object. Newly created object/struct must be in Responder.
// Possible Responder status codes are:
// - 201 Created: Resource was created and needs to be returned
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Resource created with a client generated ID, and no fields were modified by
//   the server

func (dr *DbResource) Create(obj interface{}, req api2go.Request) (api2go.Responder, error) {

  for _, bf := range dr.ms.BeforeCreate {
    r, err := bf.InterceptBefore(dr, &req)
    if err != nil {
      log.Errorf("Error from before create middleware: %v", err)
      return nil, err
    }
    if r != nil {
      return r, err
    }
  }

  data := obj.(*api2go.Api2GoModel)
  log.Infof("Create object request: %v", data)

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

    if col.ColumnName == "permission" {
      continue
    }

    if col.ColumnName == "user_id" && dr.model.GetName() != "user_user_id_has_usergroup_usergroup_id" {
      continue
    }

    //log.Infof("Check column: %v", col.ColumnName)

    val, ok := attrs[col.ColumnName]

    if !ok || val == nil {
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

    dataToInsert[col.ColumnName] = val
    colsList = append(colsList, col.ColumnName)
    valsList = append(valsList, val)
  }

  //for _, rel := range dr.model.GetRelations() {
  //  if rel.Relation == "belongs_to" || rel.Relation == "has_one" {
  //
  //    log.Infof("Relations : %v == %v", rel.Object, attrs)
  //    val, ok := attrs[rel.Object + "_id"]
  //    if ok {
  //      colsList = append(colsList, rel.Object + "_id")
  //      valsList = append(valsList, val)
  //    }
  //
  //  }
  //}

  newUuid := uuid.NewV4().String()

  colsList = append(colsList, "reference_id")
  valsList = append(valsList, newUuid)

  colsList = append(colsList, "permission")
  valsList = append(valsList, dr.model.GetDefaultPermission())

  var userId uint64
  userIdInt := context.Get(req.PlainRequest, "user_id_integer")
  if userIdInt != nil {
    userId = uint64(userIdInt.(int64))
  }

  if userId != 0 && dr.model.GetName() != "user_user_id_has_usergroup_usergroup_id" && dr.model.HasColumn("user_id") {

    colsList = append(colsList, "user_id")
    valsList = append(valsList, userId)
  }

  query, vals, err := squirrel.Insert(dr.model.GetName()).Columns(colsList...).Values(valsList...).ToSql()
  if err != nil {
    log.Errorf("Failed to create insert query: %v", err)
    return NewResponse(nil, nil, 500, nil), err
  }

  log.Infof("Insert query: %v", query)
  _, err = dr.db.Exec(query, vals...)
  if err != nil {
    log.Errorf("Failed to execute insert query: %v", err)
    return NewResponse(nil, nil, 500, nil), err
  }

  createdResource, err := dr.GetReferenceIdToObject(dr.model.GetName(), newUuid)
  if err != nil {
    log.Errorf("Failed to select the newly created entry: %v", err)
    return nil, err
  }
  //

  log.Infof("Crated entry: %v", createdResource)

  userGroupId := dr.GetUserGroupIdByUserId(userId)

  if userGroupId != 0 && dr.model.HasMany("usergroup") {
    log.Infof("Associate new entity with usergroup: %v", userGroupId)
    nuuid := uuid.NewV4().String()

    belogsToUserGroupSql, q, err := squirrel.
    Insert(dr.model.GetName() + "_" + dr.model.GetName() + "_id" + "_has_usergroup_usergroup_id").
      Columns(dr.model.GetName()+"_id", "usergroup_id", "reference_id", "permission").
      Values(createdResource["id"], userGroupId, nuuid, "755").ToSql()

    log.Infof("Query: %v", belogsToUserGroupSql)
    _, err = dr.db.Exec(belogsToUserGroupSql, q...)

    if err != nil {
      log.Errorf("Failed to insert add user group relation for [%v]: %v", dr.model.GetName(), err)
    }
  }

  for _, bf := range dr.ms.AfterCreate {
    results, err := bf.InterceptAfter(dr, &req, []map[string]interface{}{createdResource})
    if err != nil {
      log.Errorf("Error from after create middleware: %v", err)
    }
    if len(results) < 1 {
      createdResource = nil
    } else {
      createdResource = results[0]
    }
  }

  //for k, v := range createdResource {
  //  k1 := reflect.TypeOf(v)
  //  //log.Infof("K: %v", k1)
  //  if v != nil && k1.Kind() == reflect.Slice {
  //    createdResource[k] = string(v.([]uint8))
  //  }
  //}

  delete(createdResource, "id")
  delete(createdResource, "deleted_at")
  log.Infof("Create response: %v", createdResource)

  n1 := dr.model.GetName()
  c1 := dr.model.GetColumns()
  p1 := dr.model.GetDefaultPermission()
  r1 := dr.model.GetRelations()
  return NewResponse(nil,
    api2go.NewApi2GoModelWithData(
      n1,
      c1,
      p1,
      r1, createdResource,
    ),
    201, nil,
  ), nil

}
