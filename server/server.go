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
  "github.com/pkg/errors"
  "strings"
)

type CmsConfig struct {
  Tables    []datastore.TableInfo
  Relations []datastore.TableRelation
}

var ColumnTypes = []string{
  "Id",
  "Alias",
  "Date",
  "Time",
  "Day",
  "Month",
  "Year",
  "Minute",
  "Hour",
  "Email",
  "Name",
  "Value",
  "TrueFalse",
  "DateTime",
  "Location (lat/long)",
  "Color",
  "Measurement",
  "Label",
  "Content",
  "File",
  "Url",
  "Image",
}

var CollectionTypes = []string{
  "Pair",
  "Triplet",
  "Set",
  "OrderedSet",
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
  initConfig.Relations = append(initConfig.Relations, datastore.StandardRelations...)
  CheckRelations(&initConfig, db)
  tables := CheckAllTableStatus(&initConfig, db)
  initConfig.Tables = tables
  CreateRelations(&initConfig, db)

  CreateUniqueConstraints(&initConfig, db)
  CreateIndexes(&initConfig, db)

  UpdateWorldTable(&initConfig, db)
  UpdateWorldColumnTable(&initConfig, db)

  log.Infof("content: %v", initConfig)

  AddAllTablesToApi2Go(api, tables, db)

  r.GET("/ping", func(c *gin.Context) {
    c.String(200, "pong")
  })

  r.GET("/jsmodel/:typename", func(c *gin.Context) {
    typeName := strings.Split(c.Param("typename"), ".")[0]
    //resource := resources[typeName]
    var selectedTable *datastore.TableInfo
    for _, t := range initConfig.Tables {
      if t.TableName == typeName {
        selectedTable = &t
        break
      }
    }
    log.Infof("data: %v", selectedTable)

    if selectedTable == nil {
      c.AbortWithError(404, errors.New("Invalid type"))
      return
    }
    cols := selectedTable.Columns

    res := map[string]interface{}{}

    for _, col := range cols {
      res[col.ColumnName] = ""
    }
    c.JSON(200, res)
    if true {
      return
    }

    //j, _ := json.Marshal(res)

    //c.String(200, "jsonApi.define('%v', %v)", typeName, string(j))

  })

  r.Run(":6336")

}

func CorsMiddlewareFunc(c *gin.Context) {
  log.Infof("middleware ")
  c.Header("Access-Control-Allow-Origin", "*")
  c.Header("Access-Control-Allow-Methods", "POST,GET,DELETE,PUT,OPTIONS")
  c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
}

func AddAllTablesToApi2Go(api *api2go.API, tables []datastore.TableInfo, db *sqlx.DB) map[string]*dbapi.DbResource {
  m := make(map[string]*dbapi.DbResource)
  for _, table := range tables {
    model := api2go.NewApi2GoModel(table.TableName, table.Columns, table.DefaultPermission)
    res := dbapi.NewDbResource(model, db)
    m[table.TableName] = res
    api.AddResource(model, res)
  }
  return m
}

