package server

import (
  "gopkg.in/gin-gonic/gin.v1"
  "github.com/artpar/api2go"
  "github.com/artpar/api2go-adapter/gingonic"
  log "github.com/Sirupsen/logrus"
  _ "github.com/go-sql-driver/mysql"
  "gopkg.in/authboss.v1"
  _ "github.com/lib/pq"
  _ "github.com/mattn/go-sqlite3"
  //"io/ioutil"
  //"encoding/json"
  "github.com/jmoiron/sqlx"
  "github.com/artpar/goms/datastore"
  "time"
  "github.com/jamiealquiza/envy"
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
  "flag"
)

type CmsConfig struct {
  Tables    []datastore.TableInfo
  Relations []api2go.TableRelation
  Actions   []resource.Action `json:"actions"`
}

var ColumnTypes = []string{
  "id",
  "alias",
  "date",
  "time",
  "day",
  "month",
  "year",
  "minute",
  "hour",
  "email",
  "name",
  "value",
  "truefalse",
  "datetime",
  "timestamp",
  "location.latitude",
  "location.longitude",
  "location.altitude",
  "color",
  "measurement",
  "label",
  "content",
  "file",
  "url",
  "image",
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
  globalInitConfig.Actions = append(globalInitConfig.Actions, datastore.SystemActions...)

  files, err := filepath.Glob("schema_*_gocms.json")
  log.Infof("Found files to load: %v", files)

  if err != nil {
    errs = append(errs, err)
    return globalInitConfig, errs
  }

  for _, fileName := range files {
    log.Infof("Process file: %v", fileName)

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

    log.Infof("File added to config, deleting %v", fileName)

  }

  return globalInitConfig, errs

}

func Main() {

  var port = flag.String("port", "6336", "GoMS port")
  var db_type = flag.String("db_type", "sqlite3", "Database to use: sqlite3/mysql/postgres")
  var connection_string = flag.String("db_connection_string", "test.db", "[test.db] is default for sqlite3. Specify for mysql/postgres\n"+
      "<username>:<password>@tcp(<hostname>:<port>)/<db_name> for mysql\n"+
      "host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable")

  envy.Parse("GOMS") // looks for GOMS_PORT
  flag.Parse()

  //configFile := "gocms_style.json"

  db, err := sqlx.Open(*db_type, *connection_string)

  //db, err := sqlx.Open("sqlite3", "test.db")
  //db, err = sqlx.Open("mysql", "root:parth123@tcp(localhost:3306)/example")
  if err != nil {
    panic(err)
  }

  ab := authboss.New() // Usually store this globally
  ab.MountPath = "/authentication"
  ab.LogWriter = os.Stdout

  if err := ab.Init(); err != nil {
    // Handle error, don't let program continue to run
    log.Fatalln(err)
  }

  r := gin.Default()

  r.StaticFS("/static", http.Dir("./gomsweb/dist/static"))
  r.StaticFile("", "./gomsweb/dist/index.html")
  r.StaticFile("/favicon.ico", "./gomsweb/dist/static/favicon.ico")

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

  log.Infof("Load config files")
  initConfig, errs := loadConfigFiles()
  if errs != nil {
    for _, err := range errs {
      log.Errorf("Failed to load config file: %v", err)
    }
  }

  //relations := GetRelations(db)
  //log.Infof("Loaded %d relations from database", len(relations))

  //initConfig.Relations = append(initConfig.Relations, relations...)

  //existingTableMap, err := GetWorldTableMapBy("table_name", db)
  //if err != nil {
  //  log.Infof("No existing tables in the world")
  //} else {
  //  for _, table := range existingTableMap {
  //    initConfig.Tables = append(initConfig.Tables, table...)
  //
  //  }
  //}

  log.Infof("Before existing tables")
  for _, table := range initConfig.Tables {
    for _, col := range table.Relations {
      log.Infof("Table [%v] Column [%v]", table.TableName, col.String())
    }
  }

  existingTables, _ := GetTablesFromWorld(db)
  initConfig.Tables = append(initConfig.Tables, existingTables...)

  log.Infof("before check relations")
  for _, table := range initConfig.Tables {
    for _, col := range table.Relations {
      log.Infof("Table [%v] Column [%v]", table.TableName, col.String())
    }
  }

  CheckRelations(&initConfig, db)

  log.Infof("After check relations")
  for _, table := range initConfig.Tables {
    for _, col := range table.Relations {
      log.Infof("Table [%v] Column [%v]", table.TableName, col.String())
    }
  }

  CheckAllTableStatus(&initConfig, db)
  log.Infof("After check all tables")
  for _, table := range initConfig.Tables {
    for _, col := range table.Relations {
      log.Infof("Table [%v] Column [%v]", table.TableName, col.String())
    }
  }

  CreateRelations(&initConfig, db)
  log.Infof("After create relations")
  for _, table := range initConfig.Tables {
    for _, col := range table.Relations {
      log.Infof("Table [%v] Column [%v]", table.TableName, col.String())
    }
  }
  CreateUniqueConstraints(&initConfig, db)
  CreateIndexes(&initConfig, db)

  UpdateWorldTable(&initConfig, db)
  UpdateWorldColumnTable(&initConfig, db)

  err = UpdateActionTable(&initConfig, db)
  CheckErr(err, "Failed to update action table")

  CleanUpConfigFiles()

  ms := BuildMiddlewareSet()

  cruds = AddResourcesToApi2Go(api, initConfig.Tables, db, &ms)

  authMiddleware.SetUserCrud(cruds["user"])
  authMiddleware.SetUserGroupCrud(cruds["usergroup"])
  authMiddleware.SetUserUserGroupCrud(cruds["user_user_id_has_usergroup_usergroup_id"])

  r.GET("/ping", func(c *gin.Context) {
    c.String(200, "pong")
  })

  r.GET("/jsmodel/:typename", CreateJsModelHandler(&initConfig))
  r.OPTIONS("/jsmodel/:typename", CreateJsModelHandler(&initConfig))

  r.POST("/action/:actionName", CreateActionEventHandler(&initConfig, cruds))

  r.Run(fmt.Sprintf(":%v", *port))

}
func CleanUpConfigFiles() {

  files, _ := filepath.Glob("schema_*_gocms.json")
  log.Infof("Found files to load: %v", files)

  for _, fileName := range files {
    os.Remove(fileName)

  }

}

func GetTablesFromWorld(db *sqlx.DB) ([]datastore.TableInfo, error) {

  ts := make([]datastore.TableInfo, 0)

  res, err := db.Queryx("select table_name, permission, default_permission, schema_json, is_top_level, is_hidden" +
      " from world where deleted_at is null and table_name not like '%_has_%' and table_name not in ('world', 'world_column', 'action', 'user', 'usergroup')")
  if err != nil {
    log.Infof("Failed to select from world table: %v", err)
    return ts, err
  }

  for ; res.Next(); {
    var table_name string
    var permission int64
    var default_permission int64
    var schema_json string
    var is_top_level bool
    var is_hidden bool

    err = res.Scan(&table_name, &permission, &default_permission, &schema_json, &is_top_level, &is_hidden)
    if err != nil {
      log.Errorf("Failed to scan json schema from world: %v", err)
      continue
    }

    var t datastore.TableInfo

    err = json.Unmarshal([]byte(schema_json), &t)
    if err != nil {
      log.Errorf("Failed to unmarshal json schema: %v", err)
      continue
    }

    t.TableName = table_name
    t.Permission = permission
    t.DefaultPermission = default_permission
    t.IsHidden = is_hidden
    t.IsTopLevel = is_top_level
    ts = append(ts, t)

  }

  log.Infof("Loaded %d tables from world table", len(ts))

  return ts, nil

}

func GetRelations(db *sqlx.DB, tableName string) (r []api2go.TableRelation, err error) {

  res, err := db.Queryx("select schema_json from world where deleted_at is null and table_name = ?", tableName)
  if err != nil {
    log.Infof("Failed to load existing relations: %v", err)
    return
  }

  for ; res.Next(); {
    var schema string
    err = res.Scan(&schema)
    if err != nil {
      return
    }

    var t datastore.TableInfo

    json.Unmarshal([]byte(schema), &t)

    for _, rel := range t.Relations {

      if rel.Relation == "has_many" && rel.Object == "usergroup" {
        continue
      }

      if rel.Relation == "belongs_to" && rel.Object == "user" {
        continue
      }
      r = t.Relations
      return

    }

  }

  return

}

func BuildMiddlewareSet() resource.MiddlewareSet {

  var ms resource.MiddlewareSet

  permissionChecker := &resource.TableAccessPermissionChecker{}

  findOneHandler := resource.NewFindOneEventHandler()
  createHandler := resource.NewCreateEventHandler()
  updateHandler := resource.NewUpdateEventHandler()
  deleteHandler := resource.NewDeleteEventHandler()

  ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{
    permissionChecker,
  }

  ms.AfterFindAll = []resource.DatabaseRequestInterceptor{
    permissionChecker,
  }

  ms.BeforeCreate = []resource.DatabaseRequestInterceptor{
    permissionChecker,
    createHandler,
  }
  ms.AfterCreate = []resource.DatabaseRequestInterceptor{
    permissionChecker,
    createHandler,
  }

  ms.BeforeDelete = []resource.DatabaseRequestInterceptor{
    permissionChecker,
    deleteHandler,
  }
  ms.AfterDelete = []resource.DatabaseRequestInterceptor{
    permissionChecker,
    deleteHandler,
  }

  ms.BeforeUpdate = []resource.DatabaseRequestInterceptor{
    permissionChecker,
    updateHandler,
  }
  ms.AfterUpdate = []resource.DatabaseRequestInterceptor{
    permissionChecker,
    updateHandler,
  }

  ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{
    permissionChecker,
    findOneHandler,
  }
  ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{
    permissionChecker,
    findOneHandler,
  }
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

func AddResourcesToApi2Go(api *api2go.API, tables []datastore.TableInfo, db *sqlx.DB, ms *resource.MiddlewareSet) map[string]*resource.DbResource {
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
