package server

import (
  "fmt"
  //sq  "gopkg.in/Masterminds/squirrel.v1"
  //"github.com/jmoiron/sqlx"
  log "github.com/Sirupsen/logrus"
  "database/sql"
  //"reflect"
  //"github.com/satori/go.uuid"
  //"github.com/artpar/reflect"
  //"time"
)




type StatusResponse struct {
  Message string
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
