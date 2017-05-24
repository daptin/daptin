package server

import (
  "gopkg.in/gin-gonic/gin.v1"
  "github.com/artpar/api2go"
  "github.com/artpar/api2go-adapter/gingonic"
  "github.com/artpar/gocms/dbapi"
  log "github.com/Sirupsen/logrus"
  _ "github.com/go-sql-driver/mysql"
  //"github.com/itsjamie/gin-cors"
  "io/ioutil"
  "encoding/json"
  "github.com/jmoiron/sqlx"
  "github.com/artpar/gocms/datastore"
)

type CmsConfig struct {
  Tables    []datastore.TableInfo
  Relations []datastore.TableRelation
}

func Main() {
  r := gin.Default()
  r.Use(CorsMiddlewareFunc)

  api := api2go.NewAPIWithRouting(
    "api",
    api2go.NewStaticResolver("/"),
    gingonic.New(r),
  )

  //r.Use(cors.Default())
  //r.Use()

  configFile := "gocms.json"
  contents, err := ioutil.ReadFile(configFile)
  if err != nil {
    log.Errorf("Failed to read config file: %v", err)
    return
  }

  var initConfig CmsConfig
  json.Unmarshal([]byte(contents), &initConfig)
  //log.Infof("Config: %v", initConfig)

  db, err := sqlx.Open("mysql", "root:parth123@tcp(localhost:3306)/example")
  if err != nil {
    panic(err)
  }

  initConfig.Tables = append(initConfig.Tables, datastore.StandardTables...)
  CheckRelations(&initConfig, db)
  tables := CheckAllTableStatus(&initConfig, db)
  CreateRelations(&initConfig, db)

  CreateUniqueConstraints(&initConfig, db)
  CreateIndexes(&initConfig, db)

  UpdateWorldTable(&initConfig, db)

  AddAllTablesToApi2Go(api, tables, db)

  r.GET("/ping", func(c *gin.Context) {
    c.String(200, "pong")
  })


  r.Run(":6336")

}

func CorsMiddlewareFunc(c *gin.Context) {
  log.Infof("middleware ")
  c.Header("Access-Control-Allow-Origin", "*")

}

func AddAllTablesToApi2Go(api *api2go.API, tables []datastore.TableInfo, db *sqlx.DB) {
  for _, table := range tables {
    model := api2go.NewApi2GoModel(table.TableName, table.Columns, table.DefaultPermission)
    api.AddResource(model, dbapi.NewDbResource(model, db))
  }
}

