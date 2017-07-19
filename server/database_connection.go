package server

import (
  _ "github.com/go-sql-driver/mysql"
  _ "github.com/lib/pq"
  _ "github.com/mattn/go-sqlite3"
  "github.com/jmoiron/sqlx"
)

func GetDbConnection(dbType string, connectionString string) (*sqlx.DB, error) {
  return sqlx.Open(dbType, connectionString)
}
