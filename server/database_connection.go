package server

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"strings"
	"time"
	//"github.com/casbin/xorm-adapter"
	//"github.com/casbin/casbin"
)

func GetDbConnection(dbType string, connectionString string) (*sqlx.DB, error) {

	if dbType == "mysql" && strings.Index(connectionString, "charset=") == -1 {
		if strings.Index(connectionString, "?") > -1 {
			connectionString = connectionString + "&charset=utf8mb4"
		} else {
			connectionString = connectionString + "?charset=utf8mb4"
		}
	}

	if dbType == "mysql" && strings.Index(connectionString, "collation=") == -1 {
		if strings.Index(connectionString, "?") > -1 {
			connectionString = connectionString + "&collation=utf8mb4_unicode_ci"
		} else {
			connectionString = connectionString + "?collation=utf8mb4_unicode_ci"
		}
	}

	db, e := sqlx.Open(dbType, connectionString)
	if e != nil {
		return nil, e
	}
	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(50)
	db.SetConnMaxLifetime(20 * time.Second)
	return db, e
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
