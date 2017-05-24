package dbapi

import (
  "fmt"
  sq  "gopkg.in/Masterminds/squirrel.v1"
  "github.com/artpar/api2go"
  "github.com/jmoiron/sqlx"
  log "github.com/Sirupsen/logrus"
  "database/sql"
  "reflect"
  "github.com/satori/go.uuid"
  //"github.com/artpar/reflect"
)

func NewDbResource(model *api2go.Api2GoModel, db *sqlx.DB) *DbResource {
  cols := model.GetColumns()
  model.SetColumns(&cols)
  log.Infof("Columns [%v]: %v\n", model.GetName(), model.GetColumnNames())
  return &DbResource{
    model: model,
    db: db,
  }
}

// FindOne returns an object by its ID
// Possible Responder success status code 200
func (dr *DbResource) FindAll(req api2go.Request) (api2go.Responder, error) {
  m := dr.model
  log.Infof("Get all resource type: %v\n", m)

  cols := m.GetColumnNames()
  queryBuilder := sq.Select(cols...).From(m.GetTableName())
  sql1, args, err := queryBuilder.ToSql()
  if err != nil {
    log.Infof("Error: %v", err)
    return nil, err
  }

  log.Infof("Sql: %v\n", sql1)

  rows, err := dr.db.Query(sql1, args...)
  columns, _ := rows.Columns()

  if err != nil {
    log.Infof("Error: %v", err)
    return NewResponse(nil, ErrorResponse{"Failed"}, 500), err
  }

  result := make([]*api2go.Api2GoModel, 0)

  for rows.Next() {
    rc := NewMapStringScan(columns)
    err := rc.Update(rows)
    if err != nil {
      log.Fatal(err)
    }
    infos := dr.model.GetColumns()
    var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission())
    a.Data = rc.Get()
    result = append(result, a)
  }

  return NewResponse(nil, result, 200), nil

}
func (dr *DbResource) FindOne(ID string, req api2go.Request) (api2go.Responder, error) {
  return NewResponse(nil, api2go.Api2GoModel{}, 200), nil

}

//func (dr *DbResource) GetTableDefaultPermission(tableName string) int {
//  return dr.model.GetDefaultPermission()
//}

// Create a new object. Newly created object/struct must be in Responder.
// Possible Responder status codes are:
// - 201 Created: Resource was created and needs to be returned
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Resource created with a client generated ID, and no fields were modified by
//   the server
func (dr *DbResource) Create(obj interface{}, req api2go.Request) (api2go.Responder, error) {
  data := obj.(*api2go.Api2GoModel)
  log.Infof("Create object request: %v", data)

  attrs := data.GetAttributes()

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

    //log.Infof("Check column: %v", col.ColumnName)

    val, ok := attrs[col.ColumnName]

    if ok {
      dataToInsert[col.ColumnName] = val
      colsList = append(colsList, col.ColumnName)
      valsList = append(valsList, val)
    }

  }

  newUuid := uuid.NewV4().String()

  colsList = append(colsList, "reference_id")
  valsList = append(valsList, newUuid)

  colsList = append(colsList, "permission")
  valsList = append(valsList, dr.model.GetDefaultPermission())

  query, vals, err := sq.Insert(dr.model.GetName()).Columns(colsList...).Values(valsList...).ToSql()
  if err != nil {
    log.Errorf("Failed to create insert query: %v", err)
    return NewResponse(nil, nil, 500), err
  }

  log.Infof("Insert query: %v", query)
  _, err = dr.db.Exec(query, vals...)
  if err != nil {
    log.Errorf("Failed to execute insert query: %v", err)
    return NewResponse(nil, nil, 500), err
  }

  query, vals, err = sq.Select("*").From(dr.model.GetName()).Where(sq.Eq{"reference_id": newUuid}).ToSql()
  if err != nil {
    log.Errorf("Failed to create select query: %v", err)
    return nil, err
  }

  m := make(map[string]interface{})
  dr.db.QueryRowx(query, vals...).MapScan(m)

  for k, v := range m {
    k1 := reflect.TypeOf(v)
    //log.Infof("K: %v", k1)
    if v != nil && k1.Kind() == reflect.Slice {
      m[k] = string(v.([]uint8))
    }
  }

  //log.Infof("Create response: %v", m)

  return NewResponse(nil, api2go.NewApi2GoModelWithData(dr.model.GetName(), dr.model.GetColumns(), dr.model.GetDefaultPermission(), m), 201), nil

}

type StatusResponse struct {
  Message string
}

// Delete an object
// Possible Responder status codes are:
// - 200 OK: Deletion was a success, returns meta information, currently not implemented! Do not use this
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Deletion was successful, return nothing
func (dr *DbResource) Delete(id string, req api2go.Request) (api2go.Responder, error) {
  return NewResponse(nil, nil, 200), nil
}

// Update an object
// Possible Responder status codes are:
// - 200 OK: Update successful, however some field(s) were changed, returns updates source
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Update was successful, no fields were changed by the server, return nothing
func (dr *DbResource) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {
  return NewResponse(nil, StatusResponse{"ok"}, 200), nil
}



/**
  using a map
*/
type mapStringScan struct {
  // cp are the column pointers
  cp       []interface{}
  // row contains the final result
  row      map[string]interface{}
  colCount int
  colNames []string
}

func NewMapStringScan(columnNames []string) *mapStringScan {
  lenCN := len(columnNames)
  s := &mapStringScan{
    cp:       make([]interface{}, lenCN),
    row:      make(map[string]interface{}, lenCN),
    colCount: lenCN,
    colNames: columnNames,
  }
  for i := 0; i < lenCN; i++ {
    s.cp[i] = new(sql.RawBytes)
  }
  return s
}

func (s *mapStringScan) Update(rows *sql.Rows) error {
  if err := rows.Scan(s.cp...); err != nil {
    return err
  }

  for i := 0; i < s.colCount; i++ {
    if rb, ok := s.cp[i].(*sql.RawBytes); ok {
      s.row[s.colNames[i]] = string(*rb)
      *rb = nil // reset pointer to discard current value to avoid a bug
    } else {
      return fmt.Errorf("Cannot convert index %d column %s to type *sql.RawBytes", i, s.colNames[i])
    }
  }
  return nil
}

func (s *mapStringScan) Get() map[string]interface{} {
  return s.row
}

/**
  using a string slice
*/
type stringStringScan struct {
  // cp are the column pointers
  cp       []interface{}
  // row contains the final result
  row      []string
  colCount int
  colNames []string
}

func NewStringStringScan(columnNames []string) *stringStringScan {
  lenCN := len(columnNames)
  s := &stringStringScan{
    cp:       make([]interface{}, lenCN),
    row:      make([]string, lenCN * 2),
    colCount: lenCN,
    colNames: columnNames,
  }
  j := 0
  for i := 0; i < lenCN; i++ {
    s.cp[i] = new(sql.RawBytes)
    s.row[j] = s.colNames[i]
    j = j + 2
  }
  return s
}

func (s *stringStringScan) Update(rows *sql.Rows) error {
  if err := rows.Scan(s.cp...); err != nil {
    return err
  }
  j := 0
  for i := 0; i < s.colCount; i++ {
    if rb, ok := s.cp[i].(*sql.RawBytes); ok {
      s.row[j + 1] = string(*rb)
      *rb = nil // reset pointer to discard current value to avoid a bug
    } else {
      return fmt.Errorf("Cannot convert index %d column %s to type *sql.RawBytes", i, s.colNames[i])
    }
    j = j + 2
  }
  return nil
}

func (s *stringStringScan) Get() []string {
  return s.row
}

// rowMapString was the first implementation but it creates for each row a new
// map and pointers and is considered as slow. see benchmark
func rowMapString(columnNames []string, rows *sql.Rows) (map[string]string, error) {
  lenCN := len(columnNames)
  ret := make(map[string]string, lenCN)

  columnPointers := make([]interface{}, lenCN)
  for i := 0; i < lenCN; i++ {
    columnPointers[i] = new(sql.RawBytes)
  }

  if err := rows.Scan(columnPointers...); err != nil {
    return nil, err
  }

  for i := 0; i < lenCN; i++ {
    if rb, ok := columnPointers[i].(*sql.RawBytes); ok {
      ret[columnNames[i]] = string(*rb)
    } else {
      return nil, fmt.Errorf("Cannot convert index %d column %s to type *sql.RawBytes", i, columnNames[i])
    }
  }

  return ret, nil
}

func fck(err error) {
  if err != nil {
    log.Fatal(err)
  }
}
