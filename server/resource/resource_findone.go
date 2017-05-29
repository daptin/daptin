package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  "gopkg.in/Masterminds/squirrel.v1"
  "database/sql"
  "strconv"
  "github.com/pkg/errors"
  //"github.com/artpar/api2go/jsonapi"
  //"github.com/jmoiron/sqlx"
)

func (dr *DbResource) GetIdToObject(typeName string, id int64) (map[string]interface{}, error) {
  s, q, err := squirrel.Select("*").From(typeName).Where(squirrel.Eq{"id": id}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    return nil, err
  }

  row, err := dr.db.Query(s, q...)

  if err != nil {
    return nil, err
  }

  cols, err := row.Columns()
  if err != nil {
    return nil, err
  }

  m, err := dr.RowsToMap(row, cols)

  return m[0], err
}

func (dr *DbResource) GetReferenceIdToObject(typeName string, referenceId string) (map[string]interface{}, error) {
  s, q, err := squirrel.Select("*").From(typeName).Where(squirrel.Eq{"reference_id": referenceId}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    return nil, err
  }

  row, err := dr.db.Query(s, q...)

  if err != nil {
    return nil, err
  }

  cols, err := row.Columns()
  if err != nil {
    return nil, err
  }

  m, err := dr.RowsToMap(row, cols)

  return m[0], err
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

func (dr *DbResource) RowsToMap(rows *sql.Rows, columns []string) ([]map[string]interface{}, error) {

  responseArray := make([]map[string]interface{}, 0)

  for ; rows.Next(); {

    rc := NewMapStringScan(columns)
    err := rc.Update(rows)
    if err != nil {
      return responseArray, err
    }

    dbRow := rc.Get()

    //id := dbRow["id"]
    //deletedAt := dbRow["deleted_at"]

    //delete(dbRow, "id")
    //delete(dbRow, "deleted_at")

    responseArray = append(responseArray, dbRow)
  }

  return responseArray, nil

}

func (dr *DbResource) ResultToArrayOfMap(rows *sql.Rows) ([]map[string]interface{}, [][]map[string]interface{}, error) {

  columns, _ := rows.Columns()

  responseArray, err := dr.RowsToMap(rows, columns)
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

      if val == "" {
        continue
      }

      typeName, ok := api2go.EndsWith(key, "_id")
      if ok {
        i, err := strconv.ParseInt(val.(string), 10, 32)
        if err != nil {
          log.Errorf("Id should have been integer [%v]: %v", val, err)
          return responseArray, includes, err
        }

        refId, err := dr.GetIdToReferenceId(typeName, i)

        row[key] = refId
        if err != nil {
          log.Errorf("Failed to get ref id for [%v][%v]", typeName, val)
          return responseArray, includes, err
        }
        obj, err := dr.GetIdToObject(typeName, i)
        obj["__type"] = typeName

        if err != nil {
          log.Errorf("Failed to get ref object for [%v][%v]: %v", typeName, val, err)
        } else {
          localInclude = append(localInclude, obj)
        }

      }
      delete(responseArray[i], "id")
      delete(responseArray[i], "deleted_at")

    }

    includes = append(includes, localInclude)

  }

  return responseArray, includes, nil
}

type Permission struct {
  UserId      string `json:"user_id"`
  UserGroupId []string `json:"usergroup_id"`
  Permission  int64 `json:"permission"`
}

func (p Permission) CanExecute(userId string, usergroupId []string) bool {
  return p.CheckBit(userId, usergroupId, 8)
}

func (p Permission) CanRead(userId string, usergroupId []string) bool {
  return p.CheckBit(userId, usergroupId, 4)
}

func (p Permission) CanWrite(userId string, usergroupId []string) bool {
  return p.CheckBit(userId, usergroupId, 2)
}

func (p Permission) CanView(userId string, usergroupId []string) bool {
  return p.CheckBit(userId, usergroupId, 1)
}

func (p1 Permission) CheckBit(userId string, usergroupId []string, bit int64) bool {
  if userId == p1.UserId {
    p := p1.Permission / 100
    log.Infof("Check against user: %v", p)
    return (p & bit) == bit
  }

  for _, uid := range usergroupId {

    for _, gid := range p1.UserGroupId {
      if uid == gid {
        p := p1.Permission / 10
        p = p % 10
        log.Infof("Check against group: %v", p)
        return (p & bit) == bit
      }
    }
  }

  p := p1.Permission % 10
  //log.Infof("Check against world: %v == %v", p, (p & bit) == bit)
  return (p & bit) == bit
}

func (dr *DbResource) GetTablePermission(typeName string) (Permission) {

  s, q, err := squirrel.Select("user_id", "permission").From("world").Where(squirrel.Eq{"table_name": typeName}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    log.Errorf("Failed to create sql: %v", err)
    return Permission{
      "", []string{}, 0,
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

  perm.UserGroupId = dr.GetUserGroups(perm.UserId)

  perm.Permission = m["permission"].(int64)
  if err != nil {
    log.Errorf("Failed to scan permission: %v", err)
  }

  //log.Infof("Permission for [%v]: %v", typeName, perm)
  return perm
}

func (dr *DbResource) GetUserGroups(userRefId string) ([]string) {

  s := make([]string, 0)

  res, err := dr.db.Queryx("select ug.reference_id from usergroup ug join user_has_usergroup uug on uug.usergroup_id = ug.id join user u on uug.user_id = u.id where u.reference_id = ?", userRefId)
  if err != nil {
    return s
  }

  for ; res.Next(); {
    var t string
    res.Scan(&t)
    s = append(s, t)
  }
  return s

}

func (dr *DbResource) GetRowPermission(row map[string]interface{}) (Permission) {
  var perm Permission

  if row["user_id"] != nil {
    perm.UserId = row["user_id"].(string)
  }
  if row["usergroup_id"] != nil {
    perm.UserGroupId = dr.GetUserGroups(perm.UserId)
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

  s, q, err := squirrel.Select("usergroup_id").From("user_has_usergroup").Where(squirrel.Eq{"deleted_at": nil}).Where(squirrel.Eq{"user_id": userId}).OrderBy("created_at").Limit(1).ToSql()
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
  m1, includes, err := dr.ResultToArrayOfMap(rows)
  if err != nil {
    return nil, nil, err
  }

  if len(m1) < 1 {
    return nil, nil, errors.New("No such entity")
  }

  m := m1[0]
  n := includes[0]

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
  log.Infof("Find [%d] by id [%s]", dr.model.GetName(), referenceId)

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

