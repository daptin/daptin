package dbapi

import (
  "github.com/jmoiron/sqlx"
)

type DbConnection struct {
  db *sqlx.DB
}

func NewDbConnection(db1 *sqlx.DB) *DbConnection {
  return &DbConnection{
    db: db1,
  }
}

