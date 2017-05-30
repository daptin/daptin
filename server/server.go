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
  "github.com/artpar/gocms/server/resource"
  "time"
  "github.com/artpar/gocms/server/auth"
  "net/http"
)

type CmsConfig struct {
  Tables    []datastore.TableInfo
  Relations []api2go.TableRelation
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

type User struct {
  Name       string
  Email      string
  Password   string
  Id         uint64 `gorm:"PRIMARY KEY"`
  CreatedAt  time.Time `gorm:"not null"`
  UpdatedAt  time.Time
  Permission int
  Status     string
  DeletedAt  *time.Time `sql:"index"`
}

var cruds = make(map[string]*resource.DbResource)

func Main(configFile string) {

  db, err := sqlx.Open("mysql", "root:parth123@tcp(localhost:3306)/example")
  if err != nil {
    panic(err)
  }

  r := gin.Default()

  //r.StaticFile("/static", "/opt/gocms")
  r.StaticFS("/static", http.Dir("/opt/gocms/static"))
  r.StaticFile("", "/opt/gocms/index.html")

  r.Use(CorsMiddlewareFunc)

  authMiddleware := auth.NewAuthMiddlewareBuilder(db)

  r.Use(authMiddleware.AuthCheckMiddleware)

  api := api2go.NewAPIWithRouting(
    "api",
    api2go.NewStaticResolver("/"),
    gingonic.New(r),
  )

  //r.Use(cors.Default())
  //r.Use()

  contents, err := ioutil.ReadFile(configFile)
  if err != nil {
    log.Errorf("Failed to read config file: %v", err)
    return
  }

  var initConfig CmsConfig
  json.Unmarshal([]byte(contents), &initConfig)
  //log.Infof("Config: %v", initConfig)


  initConfig.Tables = append(initConfig.Tables, datastore.StandardTables...)

  for _, table := range initConfig.Tables {
    log.Infof("Table: %v: %v", table.TableName, table.Relations)
  }

  initConfig.Relations = append(initConfig.Relations, datastore.StandardRelations...)

  CheckRelations(&initConfig, db)
  CheckAllTableStatus(&initConfig, db)
  CreateRelations(&initConfig, db)

  log.Infof("table relations: %v", initConfig.Tables)

  CreateUniqueConstraints(&initConfig, db)
  CreateIndexes(&initConfig, db)

  UpdateWorldTable(&initConfig, db)
  //log.Infof("Tables: %v", tables)
  UpdateWorldColumnTable(&initConfig, db)

  //log.Infof("content: %v", initConfig)

  var ms resource.MiddlewareSet

  permissionChecker := &resource.TableAccessPermissionChecker{}

  findOneHandler := resource.NewFindOneEventHandler()
  createHandler := resource.NewCreateEventHandler()
  updateHandler := resource.NewUpdateEventHandler()
  deleteHandler := resource.NewDeleteEventHandler()

  ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{permissionChecker, }
  ms.AfterFindAll = []resource.DatabaseRequestInterceptor{permissionChecker, }

  ms.BeforeCreate = []resource.DatabaseRequestInterceptor{permissionChecker, createHandler, }
  ms.AfterCreate = []resource.DatabaseRequestInterceptor{permissionChecker, createHandler, }

  ms.BeforeDelete = []resource.DatabaseRequestInterceptor{permissionChecker, deleteHandler, }
  ms.AfterDelete = []resource.DatabaseRequestInterceptor{permissionChecker, deleteHandler, }

  ms.BeforeUpdate = []resource.DatabaseRequestInterceptor{permissionChecker, updateHandler, }
  ms.AfterUpdate = []resource.DatabaseRequestInterceptor{permissionChecker, updateHandler, }

  ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{permissionChecker, findOneHandler, }
  ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{permissionChecker, findOneHandler, }

  cruds = AddAllTablesToApi2Go(api, initConfig.Tables, db, &ms)

  authMiddleware.SetUserCrud(cruds["user"])
  authMiddleware.SetUserGroupCrud(cruds["usergroup"])
  authMiddleware.SetUserUserGroupCrud(cruds["user_has_usergroup"])

  r.GET("/ping", func(c *gin.Context) {
    c.String(200, "pong")
  })

  r.GET("/jsmodel/:typename", CreateJsModelHandler(&initConfig))
  r.OPTIONS("/jsmodel/:typename", CreateJsModelHandler(&initConfig))

  r.GET("/downloadSchema", CreateJsModelHandler(&initConfig))

  r.Run(":6336")

}

func CreateJsModelHandler(initConfig *CmsConfig) func(*gin.Context) {

  return func(c *gin.Context) {
    typeName := strings.Split(c.Param("typename"), ".")[0]
    //resource := resources[typeName]
    var selectedTable *datastore.TableInfo

    for _, t := range initConfig.Tables {
      if t.TableName == typeName {
        selectedTable = &t
        break
      }
    }

    if selectedTable == nil {
      c.AbortWithStatus(404)
      return
    }

    log.Infof("data: %v", selectedTable.Relations)

    if selectedTable == nil {
      c.AbortWithError(404, errors.New("Invalid type"))
      return
    }
    cols := selectedTable.Columns

    res := map[string]interface{}{}

    for _, col := range cols {
      if col.ColumnName == "deleted_at" {
        continue
      }
      if col.ColumnName == "id" {
        continue
      }

      _, ok := api2go.EndsWith(col.ColumnName, "_id")
      if ok && col.ColumnName != "reference_id" {
        log.Infof("Column [%v] is relation ", col.ColumnName)
        //res[typeOfOtherEntity] = NewJsonApiRelation(typeOfOtherEntity, "hasOne", "entity")
      } else {
        //res[col.ColumnName] = NewJsonApiRelation("", "", col.ColumnType)
        res[col.ColumnName] = col.ColumnType
      }

    }

    for _, rel := range selectedTable.Relations {

      if rel.GetSubject() == selectedTable.TableName {
        r := "hasMany"
        if rel.GetRelation() == "belongs_to" {
          r = "hasOne"
        }
        res[rel.GetObjectName()] = NewJsonApiRelation(rel.GetObject(), r, "entity")
      } else {
        if (rel.GetRelation() == "belongs_to") {
          res[rel.GetSubjectName()] = NewJsonApiRelation(rel.GetObject(), "hasMany", "entity")
        } else {

        }
      }
    }

    //res["__type"] = "string"
    c.JSON(200, res)
    if true {
      return
    }

    //j, _ := json.Marshal(res)

    //c.String(200, "jsonApi.define('%v', %v)", typeName, string(j))

  }
}

func NewJsonApiRelation(name string, relationType string, columnType string) JsonApiRelation {

  return JsonApiRelation{
    Type: name,
    JsonApi: relationType,
    ColumnType: columnType,
  }

}

type JsonApiRelation struct {
  JsonApi    string `json:"jsonApi,omitempty"`
  ColumnType string `json:"columnType"`
  Type       string `json:"type,omitempty"`
}

func AuthenticationFilter(c *gin.Context) {

}

func CorsMiddlewareFunc(c *gin.Context) {
  //log.Infof("middleware ")

  c.Header("Access-Control-Allow-Origin", "*")
  c.Header("Access-Control-Allow-Methods", "POST,GET,DELETE,PUT,OPTIONS,PATCH")
  c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")

  if c.Request.Method == "OPTIONS" {
    c.AbortWithStatus(200)
  }

  return
}

func AddAllTablesToApi2Go(api *api2go.API, tables []datastore.TableInfo, db *sqlx.DB, ms *resource.MiddlewareSet) map[string]*resource.DbResource {
  m := make(map[string]*resource.DbResource)
  for _, table := range tables {
    log.Infof("Table [%v] Relations: %v", table.TableName, table.Relations)
    model := api2go.NewApi2GoModel(table.TableName, table.Columns, table.DefaultPermission, table.Relations)

    res := resource.NewDbResource(model, db, ms)

    m[table.TableName] = res
    api.AddResource(model, res)
  }
  return m
}

