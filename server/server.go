package server

import (
  "gopkg.in/gin-gonic/gin.v1"
  "github.com/artpar/api2go"
  "github.com/artpar/api2go-adapter/gingonic"
  log "github.com/Sirupsen/logrus"
  _ "github.com/go-sql-driver/mysql"
  _ "github.com/mattn/go-sqlite3"
  //"io/ioutil"
  //"encoding/json"
  "github.com/jmoiron/sqlx"
  "github.com/artpar/goms/datastore"
  "time"
  "github.com/artpar/goms/server/auth"
  "net/http"
  "github.com/artpar/goms/server/resource"
  "os"
  //"strings"
  "fmt"
  "path/filepath"
  "io/ioutil"
  "encoding/json"
  "github.com/pkg/errors"
)

type CmsConfig struct {
  Tables    []datastore.TableInfo
  Relations []api2go.TableRelation
  Actions   []resource.Action `json:"actions"`
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
  Id         uint64
  CreatedAt  time.Time
  UpdatedAt  time.Time
  Permission int
  Status     string
  DeletedAt  *time.Time `sql:"index"`
}

var cruds = make(map[string]*resource.DbResource)

func getenvironment(data []string, getkeyval func(item string) (key, val string)) map[string]string {
  items := make(map[string]string)
  for _, item := range data {
    key, val := getkeyval(item)
    items[key] = val
  }
  return items
}

func loadConfigFiles() (CmsConfig, []error) {

  var err error

  errs := make([]error, 0)
  var globalInitConfig CmsConfig
  globalInitConfig = CmsConfig{
    Tables:    make([]datastore.TableInfo, 0),
    Relations: make([]api2go.TableRelation, 0),
    Actions:   make([]resource.Action, 0),
  }

  globalInitConfig.Tables = append(globalInitConfig.Tables, datastore.StandardTables...)
  globalInitConfig.Relations = append(globalInitConfig.Relations, datastore.StandardRelations...)
  globalInitConfig.Actions = append(globalInitConfig.Actions, datastore.StandardActions...)

  files, err := filepath.Glob("schema_*_gocms.json")

  if err != nil {
    errs = append(errs, err)
    return globalInitConfig, errs
  }

  for _, fileName := range files {

    fileContents, err := ioutil.ReadFile(fileName)
    if err != nil {
      errs = append(errs, err)
      continue
    }
    var initConfig CmsConfig
    err = json.Unmarshal(fileContents, &initConfig)
    if err != nil {
      errs = append(errs, err)
      continue
    }

    globalInitConfig.Tables = append(globalInitConfig.Tables, initConfig.Tables...)
    globalInitConfig.Relations = append(globalInitConfig.Relations, initConfig.Relations...)
    globalInitConfig.Actions = append(globalInitConfig.Actions, initConfig.Actions...)

    //for _, table := range initConfig.Tables {
    //log.Infof("Table: %v: %v", table.TableName, table.Relations)
    //}

    err = os.Remove(fileName)
    if err != nil {
      errs = append(errs, errors.New(fmt.Sprintf("Failed to delete config file: %v", fileName)))
      errs = append(errs, err)
    }

  }

  return globalInitConfig, errs

}

func Main() {
  //configFile := "gocms_style.json"

  //environment := getenvironment(os.Environ(), func(item string) (key, val string) {
  //  splits := strings.Split(item, "=")
  //  key = splits[0]
  //  val = splits[1]
  //  return
  //})

  //db, err := sqlx.Open("mysql", "root:parth123@tcp(localhost:3306)/example")
  db, err := sqlx.Open("sqlite3", "test.db")
  if err != nil {
    panic(err)
  }

  r := gin.Default()

  r.StaticFS("/static", http.Dir("./gocms/static"))
  r.StaticFile("", "./gocms/index.html")

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

  initConfig, _ := loadConfigFiles()

  CheckRelations(&initConfig, db)
  CheckAllTableStatus(&initConfig, db)
  CreateRelations(&initConfig, db)

  //log.Infof("table relations: %v", initConfig.Tables)

  CreateUniqueConstraints(&initConfig, db)
  CreateIndexes(&initConfig, db)

  UpdateWorldTable(&initConfig, db)
  UpdateWorldColumnTable(&initConfig, db)

  //for _, t := range initConfig.Tables {
  //for _, c := range t.Columns {
  //log.Infof("Default values [%v][%v] : [%v]", t.TableName, c.ColumnName, c.DefaultValue)
  //}
  //}

  err = UpdateActionTable(&initConfig, db)
  CheckErr(err, "Failed to update action table")

  ms := GetMiddlewareSet()

  cruds = AddAllTablesToApi2Go(api, initConfig.Tables, db, &ms)

  authMiddleware.SetUserCrud(cruds["user"])
  authMiddleware.SetUserGroupCrud(cruds["usergroup"])
  authMiddleware.SetUserUserGroupCrud(cruds["user_user_id_has_usergroup_usergroup_id"])

  r.GET("/ping", func(c *gin.Context) {
    c.String(200, "pong")
  })

  r.GET("/jsmodel/:typename", CreateJsModelHandler(&initConfig))
  r.OPTIONS("/jsmodel/:typename", CreateJsModelHandler(&initConfig))

  r.GET("/downloadSchema", CreateJsModelHandler(&initConfig))
  //r.GET("/actions/:typename", CreateActionListHandler(&initConfig))

  r.POST("/action/:actionName", CreateActionEventHandler(&initConfig, cruds))

  port := os.Getenv("PORT")
  if port == "" {
    port = "6336"
  }

  r.Run(fmt.Sprintf(":%v", port))

}

func GetMiddlewareSet() resource.MiddlewareSet {

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
  return ms
}

type ManualResponse struct {
  Data interface{}
}

func GetActionList(typename string, initConfig *CmsConfig) []resource.Action {

  actions := make([]resource.Action, 0)

  for _, a := range initConfig.Actions {
    if a.OnType == typename {
      actions = append(actions, a)
    }
  }
  return actions
}

type JsModel struct {
  ColumnModel map[string]interface{}
  Actions     []resource.Action
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
  cruds := make(map[string]*resource.DbResource)
  for _, table := range tables {
    log.Infof("Table [%v] Relations: %v", table.TableName, table.Relations)
    model := api2go.NewApi2GoModel(table.TableName, table.Columns, table.DefaultPermission, table.Relations)

    res := resource.NewDbResource(model, db, ms, cruds)

    cruds[table.TableName] = res
    api.AddResource(model, res)
  }
  return cruds
}
