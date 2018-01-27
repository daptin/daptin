package server

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	//"github.com/casbin/xorm-adapter"
	//"github.com/casbin/casbin"
)

func GetDbConnection(dbType string, connectionString string) (*sqlx.DB, error) {
	return sqlx.Open(dbType, connectionString)
}
//
//func GetCasbinAdapter(dbType string, connectionString string) (*xormadapter.Adapter) {
//	a := xormadapter.NewAdapter(dbType, connectionString) // Your driver and data source.
//	return a
//}
//
//func GetCasbinEnforcer(a xormadapter.Adapter) (casbin.Enforcer) {
//	e := casbin.NewEnforcer("../examples/rbac_model.conf", a)
//
//	return e
//}


