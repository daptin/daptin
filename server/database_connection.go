package server

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
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

	maxIdleConnections := os.Getenv("DAPTIN_MAX_IDLE_CONNECTIONS")
	if maxIdleConnections == "" {
		maxIdleConnections = "10"
	}
	maxOpenConnections := os.Getenv("DAPTIN_MAX_OPEN_CONNECTIONS")
	if maxOpenConnections == "" {
		maxOpenConnections = "50"
	}
	if strings.Index(dbType, "sqlite") > -1 {
		maxOpenConnections = "1"
	}
	maxConnectionLifetimeMinString := os.Getenv("DAPTIN_MAX_CONNECTIONS_LIFETIME")
	if maxConnectionLifetimeMinString == "" {
		maxConnectionLifetimeMinString = "1"
	}
	maxConnectionIdleTimeMinString := os.Getenv("DAPTIN_MAX_IDLE_CONNECTIONS_TIME")
	if maxConnectionIdleTimeMinString == "" {
		maxConnectionIdleTimeMinString = "1"
	}

	maxIdleConnectionsInt, err := strconv.ParseInt(maxIdleConnections, 10, 64)
	if err != nil {
		maxIdleConnectionsInt = 10
	}

	maxOpenConnectionsInt, err := strconv.ParseInt(maxOpenConnections, 10, 64)
	if err != nil {
		maxOpenConnectionsInt = 50
	}

	maxConnectionLifetimeMinInt, err := strconv.ParseInt(maxConnectionLifetimeMinString, 10, 64)
	if err != nil {
		maxConnectionLifetimeMinInt = 5
	}

	maxConnectionIdleTimeMinInt, err := strconv.ParseInt(maxConnectionIdleTimeMinString, 10, 64)
	if err != nil {
		maxConnectionIdleTimeMinInt = 5
	}

	maxConnectionLifetimeMin := time.Duration(maxConnectionLifetimeMinInt) * time.Minute
	maxConnectionIdleTimeMin := time.Duration(maxConnectionIdleTimeMinInt) * time.Minute

	db.SetMaxIdleConns(int(maxIdleConnectionsInt))
	db.SetMaxOpenConns(int(maxOpenConnectionsInt))

	db.SetConnMaxLifetime(maxConnectionLifetimeMin)
	db.SetConnMaxIdleTime(maxConnectionIdleTimeMin)
	log.Infof("Database Connection Params: "+
		"Max Idle Connections: [%v], "+
		"Max Open Connections: [%v] , "+
		"Max Connection Life time: [%v] , "+
		"Max Idle Connection life time: [%v] ",
		maxIdleConnectionsInt,
		maxOpenConnectionsInt,
		maxConnectionLifetimeMin,
		maxConnectionIdleTimeMin)

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
