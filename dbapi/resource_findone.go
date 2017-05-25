package dbapi

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  "gopkg.in/Masterminds/squirrel.v1"
  "database/sql"
  "strconv"
)

func (dr *DbResource) GetIdToReferenceId(typeName string, id int64) (string, error) {

  s, q, err := squirrel.Select("reference_id").From(typeName).Where(squirrel.Eq{"id": id}).ToSql()
  if err != nil {
    return "", err
  }

  var str string
  err = dr.db.QueryRowx(s, q...).Scan(&str)
  return str, err

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
      log.Infof("Key: [%v] == %v", key, val)

      if key == "reference_id" {
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
  m := m1[0]

  return m, err

}




// FindOne returns an object by its ID
// Possible Responder success status code 200
func (dr *DbResource) FindOne(referenceId string, req api2go.Request) (api2go.Responder, error) {

  data, err := dr.GetSingleRowByReferenceId(dr.model.GetName(), referenceId)

  infos := dr.model.GetColumns()
  var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission())
  a.Data = data

  return NewResponse(nil, a, 200), err
}

