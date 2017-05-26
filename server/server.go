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

func Main() {

  db, err := sqlx.Open("mysql", "root:parth123@tcp(localhost:3306)/example")
  if err != nil {
    panic(err)
  }

  r := gin.Default()
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

  configFile := "gocms.json"
  contents, err := ioutil.ReadFile(configFile)
  if err != nil {
    log.Errorf("Failed to read config file: %v", err)
    return
  }

  var initConfig CmsConfig
  json.Unmarshal([]byte(contents), &initConfig)
  //log.Infof("Config: %v", initConfig)


  initConfig.Tables = append(initConfig.Tables, datastore.StandardTables...)
  initConfig.Relations = append(initConfig.Relations, datastore.StandardRelations...)
  CheckRelations(&initConfig, db)
  CheckAllTableStatus(&initConfig, db)
  CreateRelations(&initConfig, db)

  log.Infof("table relations: %v", initConfig.Tables[0])

  CreateUniqueConstraints(&initConfig, db)
  CreateIndexes(&initConfig, db)

  UpdateWorldTable(&initConfig, db)
  //log.Infof("Tables: %v", tables)
  UpdateWorldColumnTable(&initConfig, db)

  //log.Infof("content: %v", initConfig)

  var ms resource.MiddlewareSet

  tpc := &resource.TableAccessPermissionChecker{}

  ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{
    tpc,
  }
  ms.AfterFindAll = []resource.DatabaseRequestInterceptor{
    tpc,
  }

  cruds = AddAllTablesToApi2Go(api, initConfig.Tables, db, &ms)

  authMiddleware.SetUserCrud(cruds["user"])
  authMiddleware.SetUserGroupCrud(cruds["usergroup"])
  authMiddleware.SetUserUserGroupCrud(cruds["user_has_usergroup"])

  r.GET("/ping", func(c *gin.Context) {
    c.String(200, "pong")
  })

  r.GET("/jsmodel/:typename", CreateJsModelHandler(&initConfig))
  r.OPTIONS("/jsmodel/:typename", CreateJsModelHandler(&initConfig))

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

    //log.Infof("data: %v", selectedTable)

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

      suffix, ok := api2go.EndsWith(col.ColumnName, "_id")
      if ok && col.ColumnName != "reference_id" {
        res[col.ColumnName] = NewJsonApiRelation(suffix, "belongs_to")
      } else {
        res[col.ColumnName] = col.ColumnType
      }

    }

    for _, rel := range selectedTable.Relations {

      if rel.Subject == selectedTable.TableName {
        res[rel.Object] = NewJsonApiRelation(rel.Object, rel.Relation)
      } else {
        if (rel.Relation == "belongs_to") {
          res[rel.Subject] = NewJsonApiRelation(rel.Object, "has_many")
        } else {

        }
      }

    }

    c.JSON(200, res)
    if true {
      return
    }

    //j, _ := json.Marshal(res)

    //c.String(200, "jsonApi.define('%v', %v)", typeName, string(j))

  }
}

func NewJsonApiRelation(name string, relationType string) JsonApiRelation {

  if relationType == "belongs_to" {
    return JsonApiRelation{
      JsonApi: "hasOne",
      Type: name,
    }
  } else if relationType == "has_many" {
    return JsonApiRelation{
      Type: name,
      JsonApi: "hasMany",
    }
  } else {
    return JsonApiRelation{
      Type: name,
      JsonApi: "hasMany",
    }
  }

}

type JsonApiRelation struct {
  JsonApi string `json:"jsonApi"`
  Type    string `json:"type"`
}

func AuthenticationFilter(c *gin.Context) {

}

func CorsMiddlewareFunc(c *gin.Context) {
  //log.Infof("middleware ")

  c.Header("Access-Control-Allow-Origin", "*")
  c.Header("Access-Control-Allow-Methods", "POST,GET,DELETE,PUT,OPTIONS,PATCH")
  c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
  c.Next()
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

