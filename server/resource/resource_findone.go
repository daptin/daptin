package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  "gopkg.in/Masterminds/squirrel.v1"
  "database/sql"
  "strconv"
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

  m, err := dr.RowsToMap(row, cols, typeName)

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

  m, err := dr.RowsToMap(row, cols, typeName)
  return m[0], err
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

func (dr *DbResource) RowsToMap(rows *sql.Rows, columns []string, typeName string) ([]map[string]interface{}, error) {

  responseArray := make([]map[string]interface{}, 0)

  for ; rows.Next(); {

    rc := NewMapStringScan(columns)
    err := rc.Update(rows)
    if err != nil {
      return responseArray, err
    }

    dbRow := rc.Get()
    dbRow["__type"] = typeName

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

  //finalArray := make([]map[string]interface{}, 0)

  responseArray, err := dr.RowsToMap(rows, columns, dr.model.GetName())
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
          continue
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
      delete(responseArray[i], "id")
      delete(responseArray[i], "deleted_at")

    }

    includes = append(includes, localInclude)
    //finalArray = append(finalArray, row)

  }

  return responseArray, includes, nil
}
