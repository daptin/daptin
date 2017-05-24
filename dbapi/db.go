package dbapi

import (
  "github.com/jmoiron/sqlx"
  "github.com/artpar/api2go"
)

type DbConnection struct {
  db *sqlx.DB
}

func NewDbConnection(db1 *sqlx.DB) *DbConnection {
  return &DbConnection{
    db: db1,
  }
}

type DbResource struct {
  model *api2go.Api2GoModel
  db    *sqlx.DB
}
