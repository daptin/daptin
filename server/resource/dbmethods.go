package resource

import (
  "gopkg.in/Masterminds/squirrel.v1"
  log "github.com/Sirupsen/logrus"
  "strconv"
  "errors"
  "github.com/artpar/api2go"
)

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


