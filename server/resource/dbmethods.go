package resource

import (
  "gopkg.in/Masterminds/squirrel.v1"
  log "github.com/Sirupsen/logrus"
  "strconv"
  "errors"
  "github.com/artpar/api2go"
  "github.com/artpar/gocms/server/auth"
  "fmt"
  "encoding/json"
)

func (dr *DbResource) IsUserActionAllowed(userReferenceId string, userGroups []auth.GroupPermission, typeName string, actionName string) bool {

  worldId, err := dr.GetIdByWhereClause("world", squirrel.Eq{"table_name": typeName})
  if err != nil {
    return false
  }
  permission, err := dr.GetActionPermissionByName(worldId[0], actionName)
  if err != nil {
    log.Errorf("Failed to get action permission [%v][%]: %v", typeName, actionName, err)
    return false
  }

  return permission.CanExecute(userReferenceId, userGroups)

}

func (dr *DbResource) GetActionByName(typeName string, actionName string) (Action, error) {
  var a ActionRow

  err := dr.db.QueryRowx("select a.action_name as name, w.table_name as ontype, a.label, in_fields as infields, out_fields as outfields, a.reference_id as referenceid from action a join world w on w.id = a.world_id where w.table_name = ? and a.action_name = ? and a.deleted_at is null limit 1", typeName, actionName).StructScan(&a)
  var action Action
  if err != nil {
    log.Errorf("Failed to scan action: %", err)
    return action, err
  }

  action.Name = a.Name
  action.Label = a.Name
  action.ReferenceId = a.ReferenceId
  action.OnType = a.OnType

  err = json.Unmarshal([]byte(a.InFields), &action.InFields)
  CheckError(err, "failed to unmarshal infields")
  err = json.Unmarshal([]byte(a.OutFields), &action.OutFields)
  CheckError(err, "failed to unmarshal outfields")

  return action, nil
}

func CheckError(err error, msg string) {
  if err != nil {
    log.Errorf(msg+" : %v", err)
  }
}

func (dr *DbResource) GetActionPermissionByName(worldId int64, actionName string) (Permission, error) {

  refId, err := dr.GetReferenceIdByWhereClause("action", squirrel.Eq{"action_name": actionName}, squirrel.Eq{"world_id": worldId})
  if err != nil {
    return Permission{}, err
  }

  if refId == nil || len(refId) < 1 {
    return Permission{}, errors.New(fmt.Sprintf("Failed to find action [%v] on [%v]", actionName, worldId))
  }
  permissions := dr.GetObjectPermission("action", refId[0])

  return permissions, nil
}

func (dr *DbResource) GetObjectPermission(objectType string, referenceId string) (Permission) {
  s, q, err := squirrel.Select("user_id", "permission", "id").From(objectType).Where(squirrel.Eq{"reference_id": referenceId}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    log.Errorf("Failed to create sql: %v", err)
    return Permission{
      "", []auth.GroupPermission{}, 0,
    }
  }

  m := make(map[string]interface{})
  err = dr.db.QueryRowx(s, q...).MapScan(m)
  if err != nil {
    log.Errorf("Failed to can permisison: %v", err)
  }
  //log.Infof("permi map: %v", m)
  var perm Permission
  if m["user_id"] != nil {

    user, err := dr.GetIdToReferenceId("user", m["user_id"].(int64))
    if err == nil {
      perm.UserId = user
    }

  }

  perm.UserGroupId = dr.GetObjectGroupsByObjectId(objectType, m["id"].(int64))

  perm.Permission = m["permission"].(int64)
  if err != nil {
    log.Errorf("Failed to scan permission: %v", err)
  }

  //log.Infof("Permission for [%v]: %v", typeName, perm)
  return perm
}

func (dr *DbResource) GetObjectPermissionByWhereClause(objectType string, colName string, colValue string) (Permission) {
  s, q, err := squirrel.Select("user_id", "permission", "id").From(objectType).Where(squirrel.Eq{colName: colValue}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    log.Errorf("Failed to create sql: %v", err)
    return Permission{
      "", []auth.GroupPermission{}, 0,
    }
  }

  m := make(map[string]interface{})
  err = dr.db.QueryRowx(s, q...).MapScan(m)
  //log.Infof("permi map: %v", m)
  var perm Permission
  if m["user_id"] != nil {

    user, err := dr.GetIdToReferenceId("user", m["user_id"].(int64))
    if err == nil {
      perm.UserId = user
    }

  }

  perm.UserGroupId = dr.GetObjectGroupsByObjectId(objectType, m["id"].(int64))

  perm.Permission = m["permission"].(int64)
  if err != nil {
    log.Errorf("Failed to scan permission: %v", err)
  }

  //log.Infof("Permission for [%v]: %v", typeName, perm)
  return perm
}

//func (dr *DbResource) GetTablePermission(typeName string) (Permission) {
//
//  s, q, err := squirrel.Select("user_id", "permission", "id").From("world").Where(squirrel.Eq{"table_name": typeName}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
//  if err != nil {
//    log.Errorf("Failed to create sql: %v", err)
//    return Permission{
//      "", []auth.GroupPermission{}, 0,
//    }
//  }
//
//  m := make(map[string]interface{})
//  err = dr.db.QueryRowx(s, q...).MapScan(m)
//  //log.Infof("permi map: %v", m)
//  var perm Permission
//  if m["user_id"] != nil {
//
//    user, err := dr.GetIdToReferenceId("user", m["user_id"].(int64))
//    if err == nil {
//      perm.UserId = user
//    }
//
//  }
//
//  perm.UserGroupId = dr.GetObjectGroups("world", m["id"])
//
//  perm.Permission = m["permission"].(int64)
//  if err != nil {
//    log.Errorf("Failed to scan permission: %v", err)
//  }
//
//  //log.Infof("Permission for [%v]: %v", typeName, perm)
//  return perm
//}

func (dr *DbResource) GetObjectGroupsByWhere(objType string, colName string, colvalue string) ([]auth.GroupPermission) {

  s := make([]auth.GroupPermission, 0)

  rel := api2go.TableRelation{}
  rel.Subject = objType
  rel.SubjectName = objType + "_id"
  rel.Object = "usergroup"
  rel.ObjectName = "usergroup_id"
  rel.Relation = "has_many_and_belongs_to_many"

  log.Infof("Join string: %v: ", rel.GetJoinString())

  sql := fmt.Sprintf("select usergroup.reference_id as referenceid, j1.permission from %s join %s  where %s.%s = ?", rel.Subject, rel.GetJoinString(), rel.Subject, colName)
  log.Infof("Group select sql: %v", sql)
  res, err := dr.db.Queryx(sql, colvalue)
  if err != nil {
    log.Errorf("Failed to get object groups by where clause: %v", err)
    return s
  }

  for ; res.Next(); {
    var g auth.GroupPermission
    err = res.StructScan(&g)
    if err != nil {
      log.Errorf("Failed to scan group permisison : %v", err)
    }
    s = append(s, g)
  }
  return s

}
func (dr *DbResource) GetObjectGroupsByObjectId(objType string, objectId int64) ([]auth.GroupPermission) {

  s := make([]auth.GroupPermission, 0)

  res, err := dr.db.Queryx(fmt.Sprintf("select ug.reference_id as referenceid, uug.permission from usergroup ug join %s_has_usergroup uug on uug.usergroup_id = ug.id and uug.%s_id = ?", objType, objType), objectId)
  if err != nil {
    return s
  }

  for ; res.Next(); {
    var g auth.GroupPermission
    err = res.StructScan(&g)
    if err != nil {
      log.Errorf("Failed to scan group permisison : %v", err)
    }
    s = append(s, g)
  }
  return s

}

func (dr *DbResource) GetRowPermission(row map[string]interface{}) (Permission) {
  var perm Permission

  if row["user_id"] != nil {
    perm.UserId = row["user_id"].(string)
  }

  if dr.model.HasMany("usergroup") {
    refId, ok := row["reference_id"]
    if !ok {
      refId = row["id"]
    }
    perm.UserGroupId = dr.GetObjectGroupsByWhere(row["__type"].(string), "reference_id", refId.(string))
  }
  if row["permission"] != nil {

    var err error
    i64, ok := row["permission"].(int64)
    if !ok {
      i64, err = strconv.ParseInt(row["permission"].(string), 10, 64)
      //p, err := int64(row["permission"].(int))
      if err != nil {
        log.Errorf("Invalid cast :%v", err)
      }

    }

    perm.Permission = i64
  }
  //log.Infof("Row permission: %v  ---------------- %v", perm, row)
  return perm
}

func (dr *DbResource) GetRowsByWhereClause(typeName string, where squirrel.Eq) ([]map[string]interface{}, [][]map[string]interface{}, error) {

  s, q, err := squirrel.Select("*").From(typeName).Where(where).Where(squirrel.Eq{"deleted_at": nil}).ToSql()

  //log.Infof("Select query: %v == [%v]", s, q)
  rows, err := dr.db.Query(s, q...)
  defer rows.Close()
  m1, include, err := dr.ResultToArrayOfMap(rows)
  if err != nil {
    return nil, nil, err
  }

  return m1, include, nil

}

func (dr *DbResource) GetUserGroupIdByUserId(userId uint64) (uint64) {

  s, q, err := squirrel.Select("usergroup_id").From("user_user_id_has_usergroup_usergroup_id").Where(squirrel.Eq{"deleted_at": nil}).Where(squirrel.Eq{"user_id": userId}).OrderBy("created_at").Limit(1).ToSql()
  if err != nil {
    log.Errorf("Failed to create sql query: ", err)
    return 0
  }

  var refId uint64

  err = dr.db.QueryRowx(s, q...).Scan(&refId)
  if err != nil {
    log.Errorf("Failed to scan user group id from the result: %v", err)
  }

  return refId

}

func (dr *DbResource) GetSingleRowByReferenceId(typeName string, referenceId string) (map[string]interface{}, []map[string]interface{}, error) {

  s, q, err := squirrel.Select("*").From(typeName).Where(squirrel.Eq{"reference_id": referenceId}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    log.Errorf("Failed to create select query by ref id: %v", referenceId)
    return nil, nil, err
  }

  rows, err := dr.db.Query(s, q...)
  defer rows.Close()
  resultRows, includeRows, err := dr.ResultToArrayOfMap(rows)
  if err != nil {
    return nil, nil, err
  }

  if len(resultRows) < 1 {
    return nil, nil, errors.New("No such entity")
  }

  m := resultRows[0]
  n := includeRows[0]

  return m, n, err

}

// FindOne returns an object by its ID
// Possible Responder success status code 200
func (dr *DbResource) FindOne(referenceId string, req api2go.Request) (api2go.Responder, error) {

  for _, bf := range dr.ms.BeforeFindOne {
    r, err := bf.InterceptBefore(dr, &req)
    if err != nil {
      log.Errorf("Error from before findone middleware: %v", err)
      return nil, err
    }
    if r != nil {
      return r, err
    }
  }
  log.Infof("Find [%s] by id [%s]", dr.model.GetName(), referenceId)

  data, include, err := dr.GetSingleRowByReferenceId(dr.model.GetName(), referenceId)

  for _, bf := range dr.ms.AfterFindOne {
    results, err := bf.InterceptAfter(dr, &req, []map[string]interface{}{data})
    if len(results) != 0 {
      data = results[0]
    } else {
      data = nil
    }
    if err != nil {
      log.Errorf("Error from after create middleware: %v", err)
    }
    include, err = bf.InterceptAfter(dr, &req, include)

    if err != nil {
      log.Errorf("Error from after create middleware: %v", err)
    }
  }

  infos := dr.model.GetColumns()
  var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission(), dr.model.GetRelations())
  a.Data = data

  for _, inc := range include {
    p, err := strconv.ParseInt(inc["permission"].(string), 10, 32)
    if err != nil {
      log.Errorf("Failed to convert [%v] to permission: %v", err)
      continue
    }
    a.Includes = append(a.Includes, api2go.NewApi2GoModelWithData(inc["__type"].(string), nil, int(p), nil, inc))
  }

  return NewResponse(nil, a, 200, nil), err
}
