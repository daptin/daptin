package server

import (
  "gopkg.in/gin-gonic/gin.v1"
  "github.com/artpar/api2go"
  "github.com/artpar/api2go-adapter/gingonic"
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
  //log.Infof("Tables: %v", tables)
  UpdateWorldColumnTable(&initConfig, db)

  //log.Infof("content: %v", initConfig)

  var ms MiddlewareSet

  tpc := &TableAccessPermissionChecker{}
  ms.BeforeFindAll = []DatabaseRequestInterceptor{
    tpc,
  }
  ms.AfterFindAll = []DatabaseRequestInterceptor{
    tpc,
  }

  AddAllTablesToApi2Go(api, tables, db, &ms)

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
      if col.ColumnType == "deleted_at" {
        continue
      }
      if col.ColumnType == "id" {
        continue
      }

      res[col.ColumnName] = col.ColumnType
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

type DatabaseRequestInterceptor interface {
  InterceptBefore(*DbResource, *api2go.Request) (api2go.Responder, error)
  InterceptAfter(*DbResource, *api2go.Request, []map[string]interface{}) ([]map[string]interface{}, error)
}

type MiddlewareSet struct {
  BeforeCreate  []DatabaseRequestInterceptor
  BeforeFindAll []DatabaseRequestInterceptor
  BeforeFindOne []DatabaseRequestInterceptor
  BeforeUpdate  []DatabaseRequestInterceptor
  BeforeDelete  []DatabaseRequestInterceptor

  AfterCreate   []DatabaseRequestInterceptor
  AfterFindAll  []DatabaseRequestInterceptor
  AfterFindOne  []DatabaseRequestInterceptor
  AfterUpdate   []DatabaseRequestInterceptor
  AfterDelete   []DatabaseRequestInterceptor
}

type DbResource struct {
  model        *api2go.Api2GoModel
  db           *sqlx.DB
  ms           *MiddlewareSet
  contextCache map[string]interface{}
}

func NewDbResource(model *api2go.Api2GoModel, db *sqlx.DB, ms *MiddlewareSet) *DbResource {
  cols := model.GetColumns()
  model.SetColumns(&cols)
  log.Infof("Columns [%v]: %v\n", model.GetName(), model.GetColumnNames())
  return &DbResource{
    model: model,
    db: db,
    ms: ms,
    contextCache: make(map[string]interface{}),
  }
}

func (dr *DbResource) PutContext(key string, val interface{}) {
  dr.contextCache[key] = val
}

func (dr *DbResource) GetContext(key string) interface{} {
  return dr.contextCache[key]
}

func AddAllTablesToApi2Go(api *api2go.API, tables []datastore.TableInfo, db *sqlx.DB, ms *MiddlewareSet) map[string]*DbResource {
  m := make(map[string]*DbResource)
  for _, table := range tables {
    //log.Infof("Table [%v] DF: %v", table.TableName, table.DefaultPermission)
    model := api2go.NewApi2GoModel(table.TableName, table.Columns, table.DefaultPermission)

    res := NewDbResource(model, db, ms)

    m[table.TableName] = res
    api.AddResource(model, res)
  }
  return m
}

