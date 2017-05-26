package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  "gopkg.in/Masterminds/squirrel.v1"
  "database/sql"
  "strconv"
  "github.com/pkg/errors"
  "fmt"
)

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

func (dr *DbResource) ResultToArrayOfMap(rows *sql.Rows) ([]map[string]interface{}, error) {

  responseArray := make([]map[string]interface{}, 0)

  columns, _ := rows.Columns()

  for ; rows.Next(); {
    rc := NewMapStringScan(columns)
    err := rc.Update(rows)
    if err != nil {
      return responseArray, err
    }

    dbRow := rc.Get()

    delete(dbRow, "id")
    delete(dbRow, "deleted_at")

    for key, val := range dbRow {
      //log.Infof("Key: [%v] == %v", key, val)

      if key == "reference_id" {
        continue
      }

      if val == "" {
        continue
      }

      if len(key) > 3 && key[len(key) - 3:] == "_id" {
        typeName := key[:len(key) - 3]
        i, err := strconv.ParseInt(val.(string), 10, 32)
        if err != nil {
          log.Errorf("Id should have been integer [%v]: %v", val, err)
          return responseArray, err
        }

        refId, err := dr.GetIdToReferenceId(typeName, i)
        dbRow[key] = refId
        if err != nil {
          log.Errorf("Failed to get ref id for [%v][%v]", typeName, val)
          return responseArray, err
        }
      }
    }

    responseArray = append(responseArray, dbRow)
  }

  return responseArray, nil
}

type Permission struct {
  UserId      string `json:"user_id"`
  UserGroupId string `json:"usergroup_id"`
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
    //log.Infof("Check against user: %v", p)
    return (p & bit) == bit
  }

  for _, uid := range usergroupId {
    if uid == p1.UserGroupId {
      p := p1.Permission / 10
      p = p % 10
      //log.Infof("Check against group: %v", p)
      return (p & bit) == bit
    }
  }

  p := p1.Permission % 10
  //log.Infof("Check against world: %v", p)
  return (p & bit) == bit
}

func (dr *DbResource) GetTablePermission(typeName string) (Permission) {

  s, q, err := squirrel.Select("user_id", "usergroup_id", "permission").From("world").Where(squirrel.Eq{"table_name": typeName}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    log.Errorf("Failed to create sql: %v", err)
    return Permission{
      "-1", "-1", 0,
    }
  }

  m := make(map[string]interface{})
  //log.Infof("Permission sql: %v", s)
  err = dr.db.QueryRowx(s, q...).MapScan(m)
  //log.Infof("permi map: %v", m)
  var perm Permission
  perm.UserId = fmt.Sprintf("%v", m["user_id"].(int64))
  perm.UserGroupId = fmt.Sprintf("%v", m["usergroup_id"].(int64))
  perm.Permission = m["permission"].(int64)
  if err != nil {
    log.Errorf("Failed to scan permission: %v", err)
  }

  //log.Infof("Permission for [%v]: %v", typeName, perm)
  return perm
}

func (dr *DbResource) GetRowPermission(row map[string]interface{}) (Permission) {
  var perm Permission

  if row["user_id"] != nil {
    perm.UserId = row["user_id"].(string)
  }
  if row["usergroup_id"] != nil {
    perm.UserGroupId = row["usergroup_id"].(string)
  }
  if row["permission"] != nil {
    p, _ := strconv.ParseInt(row["permission"].(string), 10, 64)
    perm.Permission = p
  }
  return perm
}

func (dr *DbResource) GetRowsByWhereClause(typeName string, where squirrel.Eq) ([]map[string]interface{}, error) {

  s, q, err := squirrel.Select("*").From(typeName).Where(where).Where(squirrel.Eq{"deleted_at": nil}).ToSql()

  //log.Infof("Select query: %v == [%v]", s, q)
  rows, err := dr.db.Query(s, q...)
  m1, err := dr.ResultToArrayOfMap(rows)
  if err != nil {
    return nil, err
  }

  return m1, nil

}

func (dr *DbResource) GetSingleRowByReferenceId(typeName string, referenceId string) (map[string]interface{}, error) {

  s, q, err := squirrel.Select("*").From(typeName).Where(squirrel.Eq{"reference_id": referenceId}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    log.Errorf("Failed to create select query by ref id: %v", referenceId)
    return nil, err
  }

  rows, err := dr.db.Query(s, q...)
  m1, err := dr.ResultToArrayOfMap(rows)
  if err != nil {
    return nil, err
  }

  if len(m1) < 1 {
    return nil, errors.New("No such entity")
  }

  m := m1[0]

  return m, err

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

  data, err := dr.GetSingleRowByReferenceId(dr.model.GetName(), referenceId)

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
  }

  infos := dr.model.GetColumns()
  var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission(), dr.model.GetRelations())
  a.Data = data

  return NewResponse(nil, a, 200, nil), err
}

