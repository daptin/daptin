package resource

import (
  "gopkg.in/Masterminds/squirrel.v1"
  log "github.com/Sirupsen/logrus"
  "strconv"
  "errors"
  "github.com/artpar/api2go"
  "github.com/artpar/goms/server/auth"
  "fmt"
  "encoding/json"
  "github.com/jmoiron/sqlx"
  "reflect"
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

func (dr *DbResource) GetObjectUserGroupsByWhere(objType string, colName string, colvalue string) ([]auth.GroupPermission) {

  s := make([]auth.GroupPermission, 0)

  rel := api2go.TableRelation{}
  rel.Subject = objType
  rel.SubjectName = objType + "_id"
  rel.Object = "usergroup"
  rel.ObjectName = "usergroup_id"
  rel.Relation = "has_many_and_belongs_to_many"

  //log.Infof("Join string: %v: ", rel.GetJoinString())

  sql := fmt.Sprintf("select usergroup.reference_id as referenceid, j1.permission from %s join %s  where %s.%s = ?", rel.Subject, rel.GetJoinString(), rel.Subject, colName)
  //log.Infof("Group select sql: %v", sql)
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
    uid, ok := row["user_id"].(string)

    if !ok {

    }
    perm.UserId = uid
  }

  if dr.model.HasMany("usergroup") {
    refId, ok := row["reference_id"]
    if !ok {
      refId = row["id"]
    }
    perm.UserGroupId = dr.GetObjectUserGroupsByWhere(row["__type"].(string), "reference_id", refId.(string))
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
  rows, err := dr.db.Queryx(s, q...)
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

  rows, err := dr.db.Queryx(s, q...)
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

func (dr *DbResource) GetIdToObject(typeName string, id int64) (map[string]interface{}, error) {
  s, q, err := squirrel.Select("*").From(typeName).Where(squirrel.Eq{"id": id}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    return nil, err
  }

  row, err := dr.db.Queryx(s, q...)

  if err != nil {
    return nil, err
  }

  m, err := RowsToMap(row, typeName)

  return m[0], err
}

func (dr *DbResource) GetReferenceIdToObject(typeName string, referenceId string) (map[string]interface{}, error) {
  s, q, err := squirrel.Select("*").From(typeName).Where(squirrel.Eq{"reference_id": referenceId}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    return nil, err
  }

  row, err := dr.db.Queryx(s, q...)

  if err != nil {
    return nil, err
  }

  //cols, err := row.Columns()
  //if err != nil {
  //  return nil, err
  //}

  results, _, err := dr.ResultToArrayOfMap(row)
  if err != nil {
    return nil, err
  }

  return results[0], err
}

func (dr *DbResource) GetReferenceIdByWhereClause(typeName string, queries ...squirrel.Eq) ([]string, error) {
  builder := squirrel.Select("reference_id").From(typeName).Where(squirrel.Eq{"deleted_at": nil})

  for _, qu := range queries {
    builder = builder.Where(qu)
  }

  s, q, err := builder.ToSql()
  log.Debugf("reference id by where query: %v", s)

  if err != nil {
    return nil, err
  }

  res, err := dr.db.Queryx(s, q...)

  if err != nil {
    return nil, err
  }

  ret := make([]string, 0)
  for ; res.Next(); {
    var s string
    res.Scan(&s)
    ret = append(ret, s)
  }

  return ret, err

}

func (dr *DbResource) GetIdByWhereClause(typeName string, queries ...squirrel.Eq) ([]int64, error) {
  builder := squirrel.Select("id").From(typeName).Where(squirrel.Eq{"deleted_at": nil})

  for _, qu := range queries {
    builder = builder.Where(qu)
  }

  s, q, err := builder.ToSql()
  log.Debugf("reference id by where query: %v", s)

  if err != nil {
    return nil, err
  }

  res, err := dr.db.Queryx(s, q...)

  if err != nil {
    return nil, err
  }

  ret := make([]int64, 0)
  for ; res.Next(); {
    var s int64
    res.Scan(&s)
    ret = append(ret, s)
  }

  return ret, err

}

func (dr *DbResource) GetIdToReferenceId(typeName string, id int64) (string, error) {

  s, q, err := squirrel.Select("reference_id").From(typeName).Where(squirrel.Eq{"id": id}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    return "", err
  }

  var str string
  err = dr.db.QueryRowx(s, q...).Scan(&str)
  return str, err

}

func (dr *DbResource) GetReferenceIdToId(typeName string, referenceId string) (uint64, error) {

  var id uint64
  s, q, err := squirrel.Select("id").From(typeName).Where(squirrel.Eq{"reference_id": referenceId}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    return 0, err
  }

  err = dr.db.QueryRowx(s, q...).Scan(&id)
  return id, err

}

func (dr *DbResource) GetSingleColumnValueByReferenceId(typeName string, selectColumn, matchColumn string, values []string) ([]interface{}, error) {

  s, q, err := squirrel.Select(selectColumn).From(typeName).Where(squirrel.Eq{matchColumn: values}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    return nil, err
  }

  return dr.db.QueryRowx(s, q...).SliceScan()
}

func RowsToMap(rows *sqlx.Rows, typeName string) ([]map[string]interface{}, error) {

  columns, err := rows.Columns()
  if err != nil {
    return nil, err
  }
  responseArray := make([]map[string]interface{}, 0)

  for ; rows.Next(); {

    rc := NewMapStringScan(columns)
    err := rc.Update(rows)
    if err != nil {
      return responseArray, err
    }

    dbRow := rc.Get()
    dbRow["__type"] = typeName
    //log.Infof("Scanned row: %v", dbRow)

    //id := dbRow["id"]
    //deletedAt := dbRow["deleted_at"]

    //delete(dbRow, "id")
    //delete(dbRow, "deleted_at")

    responseArray = append(responseArray, dbRow)
  }

  return responseArray, nil

}

func (dr *DbResource) ResultToArrayOfMap(rows *sqlx.Rows) ([]map[string]interface{}, [][]map[string]interface{}, error) {

  //finalArray := make([]map[string]interface{}, 0)

  responseArray, err := RowsToMap(rows, dr.model.GetName())
  if err != nil {
    return responseArray, nil, err
  }

  includes := make([][]map[string]interface{}, 0)

  for i, row := range responseArray {
    localInclude := make([]map[string]interface{}, 0)

    for key, val := range row {
      //log.Infof("Key: [%v] == %v", key, val)

      if key == "reference_id" {
        continue
      }

      if val == "" || val == nil {
        continue
      }

      typeName, ok := api2go.EndsWith(key, "_id")
      if ok {
        i, ok := val.(int64)
        if !ok {

          si, ok := val.(string)
          if ok {
            i, err = strconv.ParseInt(si, 10, 64)
            if err != nil {
              log.Errorf("Failed to convert [%v] to int", si)
              continue
            }
          } else {
            log.Errorf("Id should have been integer [%v]: %v", val, reflect.TypeOf(val))
            continue
          }
        }

        refId, err := dr.GetIdToReferenceId(typeName, i)

        row[key] = refId
        if err != nil {
          log.Errorf("Failed to get ref id for [%v][%v]: %v", typeName, val, err)
          continue
        }
        obj, err := dr.GetIdToObject(typeName, i)
        obj["__type"] = typeName

        if err != nil {
          log.Errorf("Failed to get ref object for [%v][%v]: %v", typeName, val, err)
        } else {
          localInclude = append(localInclude, obj)
        }

      }
      //delete(responseArray[i], "id")
      delete(responseArray[i], "deleted_at")

    }

    includes = append(includes, localInclude)
    //finalArray = append(finalArray, row)

  }

  return responseArray, includes, nil
}
