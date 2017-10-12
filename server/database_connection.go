package server

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func GetDbConnection(dbType string, connectionString string) (*sqlx.DB, error) {
	return sqlx.Open(dbType, connectionString)
}
