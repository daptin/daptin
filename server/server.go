package server

import (
  "github.com/artpar/api2go"
  "github.com/artpar/api2go-adapter/gingonic"
  _ "github.com/go-sql-driver/mysql"
  log "github.com/sirupsen/logrus"
  "gopkg.in/gin-gonic/gin.v1"
  //"gopkg.in/authboss.v1"
  _ "github.com/lib/pq"
  _ "github.com/mattn/go-sqlite3"
  //"io/ioutil"
  //"encoding/json"
  "github.com/artpar/goms/server/auth"
  "github.com/artpar/goms/server/resource"
  "github.com/jamiealquiza/envy"
  "github.com/jmoiron/sqlx"
  "net/http"
  "os"
  //"strings"
  "encoding/json"
  "fmt"
  "io/ioutil"
  "path/filepath"
  //"github.com/pkg/errors"
  "flag"
  "github.com/gorilla/context"
  uuid2 "github.com/satori/go.uuid"
  "gopkg.in/Masterminds/squirrel.v1"
  "strings"
)

var cruds = make(map[string]*resource.DbResource)

func loadConfigFiles() (resource.CmsConfig, []error) {

  var err error

  errs := make([]error, 0)
  var globalInitConfig resource.CmsConfig
  globalInitConfig = resource.CmsConfig{
    Tables:                   make([]resource.TableInfo, 0),
    Relations:                make([]api2go.TableRelation, 0),
    Actions:                  make([]resource.Action, 0),
    StateMachineDescriptions: make([]resource.LoopbookFsmDescription, 0),
  }

  globalInitConfig.Tables = append(globalInitConfig.Tables, resource.StandardTables...)
  globalInitConfig.Relations = append(globalInitConfig.Relations, resource.StandardRelations...)
  globalInitConfig.Actions = append(globalInitConfig.Actions, resource.SystemActions...)
  globalInitConfig.StateMachineDescriptions = append(globalInitConfig.StateMachineDescriptions, resource.SystemSmds...)
  globalInitConfig.ExchangeContracts = append(globalInitConfig.ExchangeContracts, resource.SystemExchanges...)

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
    var initConfig resource.CmsConfig
    err = json.Unmarshal(fileContents, &initConfig)
    if err != nil {
      errs = append(errs, err)
      continue
    }

    globalInitConfig.Tables = append(globalInitConfig.Tables, initConfig.Tables...)
    globalInitConfig.Relations = append(globalInitConfig.Relations, initConfig.Relations...)
    globalInitConfig.Actions = append(globalInitConfig.Actions, initConfig.Actions...)
    globalInitConfig.StateMachineDescriptions = append(globalInitConfig.StateMachineDescriptions, initConfig.StateMachineDescriptions...)
    globalInitConfig.ExchangeContracts = append(globalInitConfig.ExchangeContracts, initConfig.ExchangeContracts...)

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
  var connection_string = flag.String("db_connection_string", "test.db", "\n\tSQLite: test.db\n"+
      "\tMySql: <username>:<password>@tcp(<hostname>:<port>)/<db_name>\n"+
      "\tPostgres: host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable")

  var runtimeMode = flag.String("runtime", "debug", "Runtime for Gin: debug, test, release")

  envy.Parse("GOMS") // looks for GOMS_PORT
  flag.Parse()

  gin.SetMode(*runtimeMode)

  //configFile := "gocms_style.json"

  db, err := sqlx.Open(*db_type, *connection_string)

  //db, err := sqlx.Open("sqlite3", "test.db")
  //db, err = sqlx.Open("mysql", "root:parth123@tcp(localhost:3306)/example")
  if err != nil {
    panic(err)
  }

  /// Start system initialise

  log.Infof("Load config files")
  initConfig, errs := loadConfigFiles()
  if errs != nil {
    for _, err := range errs {
      log.Errorf("Failed to load config file: %v", err)
    }
  }

  existingTables, _ := GetTablesFromWorld(db)
  initConfig.Tables = append(initConfig.Tables, existingTables...)

  resource.CheckRelations(&initConfig, db)

  //AddStateMachines(&initConfig, db)

  log.Infof("After check relations")

  resource.CheckAllTableStatus(&initConfig, db)

  resource.CreateRelations(&initConfig, db)

  resource.CreateUniqueConstraints(&initConfig, db)
  resource.CreateIndexes(&initConfig, db)

  resource.UpdateWorldTable(&initConfig, db)
  resource.UpdateWorldColumnTable(&initConfig, db)
  resource.UpdateStateMachineDescriptions(&initConfig, db)
  resource.UpdateExchanges(&initConfig, db)

  err = resource.UpdateActionTable(&initConfig, db)
  resource.CheckErr(err, "Failed to update action table")

  CleanUpConfigFiles()

  ms := BuildMiddlewareSet()

  /// end system initialise

  r := gin.Default()
  r.Use(CorsMiddlewareFunc)

  r.StaticFS("/static", http.Dir("./gomsweb/dist/static"))
  r.StaticFile("", "./gomsweb/dist/index.html")
  r.StaticFile("/favicon.ico", "./gomsweb/dist/static/favicon.ico")

  configStore, err := resource.NewConfigStore(db)
  if err != nil {
    log.Errorf("Failed to create a config store: %v", err)
  }

  r.GET("/config", CreateConfigHandler(configStore))

  jwtSecret, err := configStore.GetConfigValueFor("jwt.secret", "backend")
  if err != nil {
    jwtSecret = uuid2.NewV4().String()
    err = configStore.SetConfigValueFor("jwt.secret", jwtSecret, "backend")
    resource.CheckErr(err, "Failed to store jwt secret")
  }

  authMiddleware := auth.NewAuthMiddlewareBuilder(db)
  auth.InitJwtMiddleware([]byte(jwtSecret))
  r.Use(authMiddleware.AuthCheckMiddleware)

  r.GET("/actions", resource.CreateGuestActionListHandler(&initConfig, cruds))

  api := api2go.NewAPIWithRouting(
    "api",
    api2go.NewStaticResolver("/"),
    gingonic.New(r),
  )
  cruds = AddResourcesToApi2Go(api, initConfig.Tables, db, &ms, configStore)

  encryptionSecret, err := configStore.GetConfigValueFor("encryption.secret", "backend")

  if err != nil || len(encryptionSecret) < 10 {

    newSecret := strings.Replace(uuid2.NewV4().String(), "-", "", -1)
    configStore.SetConfigValueFor("encryption.secret", newSecret, "backend")
  }

  authMiddleware.SetUserCrud(cruds["user"])
  authMiddleware.SetUserGroupCrud(cruds["usergroup"])
  authMiddleware.SetUserUserGroupCrud(cruds["user_user_id_has_usergroup_usergroup_id"])

  fsmManager := resource.NewFsmManager(db, cruds)

  r.GET("/ping", func(c *gin.Context) {
    c.String(200, "pong")
  })

  r.GET("/jsmodel/:typename", CreateJsModelHandler(&initConfig))
  r.OPTIONS("/jsmodel/:typename", CreateJsModelHandler(&initConfig))

  actionPerformers := GetActionPerformers(&initConfig, configStore)

  r.POST("/action/:typename/:actionName", resource.CreatePostActionHandler(&initConfig, configStore, cruds, actionPerformers))
  r.GET("/action/:typename/:actionName", resource.CreatePostActionHandler(&initConfig, configStore, cruds, actionPerformers))

  r.POST("/track/start/:stateMachineId", CreateEventStartHandler(fsmManager, cruds, db))
  r.POST("/track/event/:typename/:objectStateId/:eventName", CreateEventHandler(&initConfig, fsmManager, cruds, db))

  r.NoRoute(func(c *gin.Context) {
    c.File("./gomsweb/dist/index.html")
  })

  r.Run(fmt.Sprintf(":%v", *port))
}

func GetActionPerformers(initConfig *resource.CmsConfig, configStore *resource.ConfigStore) []resource.ActionPerformerInterface {
  performers := make([]resource.ActionPerformerInterface, 0)

  becomeAdminPerformer, err := resource.NewBecomeAdminPerformer(initConfig, cruds)
  resource.CheckErr(err, "Failed to create become admin performer")
  performers = append(performers, becomeAdminPerformer)

  downloadConfigPerformer, err := resource.NewDownloadCmsConfigPerformer(initConfig)
  resource.CheckErr(err, "Failed to create download config performer")
  performers = append(performers, downloadConfigPerformer)

  oauth2redirect, err := resource.NewOauthLoginBeginActionPerformer(initConfig, cruds, configStore)
  resource.CheckErr(err, "Failed to create oauth2 request performer")
  performers = append(performers, oauth2redirect)

  oauth2response, err := resource.NewOauthLoginResponseActionPerformer(initConfig, cruds, configStore)
  resource.CheckErr(err, "Failed to create oauth2 response handler")
  performers = append(performers, oauth2response)

  generateJwtPerformer, err := resource.NewGenerateJwtTokenPerformer(configStore, cruds)
  resource.CheckErr(err, "Failed to create generate jwt performer")
  performers = append(performers, generateJwtPerformer)

  restartPerformer, err := resource.NewRestarSystemPerformer(initConfig)
  resource.CheckErr(err, "Failed to create restart performer")
  performers = append(performers, restartPerformer)

  return performers
}

func CreateEventStartHandler(fsmManager resource.FsmManager, cruds map[string]*resource.DbResource, db *sqlx.DB) func(context *gin.Context) {

  return func(gincontext *gin.Context) {

    uId := context.Get(gincontext.Request, "user_id")
    var currentUserReferenceId string
    currentUsergroups := make([]auth.GroupPermission, 0)

    if uId != nil {
      currentUserReferenceId = uId.(string)
    }
    ugId := context.Get(gincontext.Request, "usergroup_id")
    if ugId != nil {
      currentUsergroups = ugId.([]auth.GroupPermission)
    }

    jsBytes, err := ioutil.ReadAll(gincontext.Request.Body)
    if err != nil {
      log.Errorf("Failed to read post body: %v", err)
      gincontext.AbortWithError(400, err)
      return
    }

    m := make(map[string]interface{})
    json.Unmarshal(jsBytes, &m)

    typename := m["typeName"].(string)
    refId := m["referenceId"].(string)
    stateMachineId := gincontext.Param("stateMachineId")

    pr := &http.Request{}
    pr.Method = "GET"
    req := api2go.Request{
      PlainRequest: pr,
      QueryParams:  map[string][]string{},
    }

    context.Set(pr, "user_id", currentUserReferenceId)
    context.Set(pr, "usergroup_id", currentUsergroups)

    response, err := cruds["smd"].FindOne(stateMachineId, req)
    if err != nil {
      gincontext.AbortWithError(400, err)
      return
    }

    stateMachineInstance := response.Result().(*api2go.Api2GoModel)
    stateMachineInstanceProperties := stateMachineInstance.GetAttributes()
    stateMachinePermission := cruds["smd"].GetRowPermission(stateMachineInstance.GetAllAsAttributes())

    if !stateMachinePermission.CanExecute(currentUserReferenceId, currentUsergroups) {
      gincontext.AbortWithStatus(403)
      return
    }

    subjectInstanceResponse, err := cruds[typename].FindOne(refId, req)
    if err != nil {
      gincontext.AbortWithError(400, err)
      return
    }
    subjectInstanceModel := subjectInstanceResponse.Result().(*api2go.Api2GoModel).GetAttributes()

    newStateMachine := make(map[string]interface{})

    newStateMachine["current_state"] = stateMachineInstanceProperties["initial_state"]
    newStateMachine[typename+"_smd"] = stateMachineInstanceProperties["reference_id"]
    newStateMachine["is_state_of_"+typename] = subjectInstanceModel["reference_id"]
    newStateMachine["permission"] = "750"

    req.PlainRequest.Method = "POST"

    resp, err := cruds[typename+"_state"].Create(api2go.NewApi2GoModelWithData(typename+"_state", nil, 0, nil, newStateMachine), req)

    //s, v, err := squirrel.Insert(typename + "_state").SetMap(newStateMachine).ToSql()
    //if err != nil {
    //  log.Errorf("Failed to create state insert query: %v", err)
    //  gincontext.AbortWithError(500, err)
    //}

    //_, err = db.Exec(s, v...)
    if err != nil {
      log.Errorf("Failed to execute state insert query: %v", err)
      gincontext.AbortWithError(500, err)
      return
    }

    gincontext.JSON(200, resp)

  }

}

func CreateEventHandler(initConfig *resource.CmsConfig, fsmManager resource.FsmManager, cruds map[string]*resource.DbResource, db *sqlx.DB) func(context *gin.Context) {

  return func(gincontext *gin.Context) {

    currentUserReferenceId := context.Get(gincontext.Request, "user_id").(string)
    currentUsergroups := context.Get(gincontext.Request, "usergroup_id").([]auth.GroupPermission)

    pr := &http.Request{}
    pr.Method = "GET"
    req := api2go.Request{
      PlainRequest: pr,
      QueryParams:  map[string][]string{},
    }

    context.Set(pr, "user_id", currentUserReferenceId)
    context.Set(pr, "usergroup_id", currentUsergroups)

    objectStateMachineId := gincontext.Param("objectStateId")
    typename := gincontext.Param("typename")

    objectStateMachineResponse, err := cruds[typename+"_state"].FindOne(objectStateMachineId, req)
    if err != nil {
      log.Errorf("Failed to get object state machine: %v", err)
      gincontext.AbortWithError(400, err)
      return
    }

    objectStateMachine := objectStateMachineResponse.Result().(*api2go.Api2GoModel)

    stateObject := objectStateMachine.Data

    var subjectInstanceModel *api2go.Api2GoModel
    var stateMachineDescriptionInstance *api2go.Api2GoModel

    for _, included := range objectStateMachine.Includes {
      casted := included.(*api2go.Api2GoModel)
      if casted.GetTableName() == typename {
        subjectInstanceModel = casted
      } else if casted.GetTableName() == "smd" {
        stateMachineDescriptionInstance = casted
      }

    }

    stateMachineId := objectStateMachine.GetID()
    eventName := gincontext.Param("eventName")

    stateMachinePermission := cruds["smd"].GetRowPermission(stateMachineDescriptionInstance.GetAllAsAttributes())

    if !stateMachinePermission.CanExecute(currentUserReferenceId, currentUsergroups) {
      gincontext.AbortWithStatus(403)
      return
    }

    nextState, err := fsmManager.ApplyEvent(subjectInstanceModel.GetAllAsAttributes(), NewStateMachineEvent(stateMachineId, eventName))
    if err != nil {
      gincontext.AbortWithError(400, err)
      return
    }

    stateObject["current_state"] = nextState

    s, v, err := squirrel.Update(typename + "_state").Set("current_state", nextState).Where(squirrel.Eq{"reference_id": stateMachineId}).ToSql()

    _, err = db.Exec(s, v...)
    if err != nil {
      gincontext.AbortWithError(500, err)
      return
    }

    gincontext.AbortWithStatus(200)

  }

}

type simpleStateMachinEvent struct {
  machineReferenceId string
  eventName          string
}

func NewStateMachineEvent(machineId string, eventName string) resource.StateMachineEvent {
  return &simpleStateMachinEvent{
    machineReferenceId: machineId,
    eventName:          eventName,
  }
}

func (f *simpleStateMachinEvent) GetStateMachineInstanceId() string {
  return f.machineReferenceId
}
func (f *simpleStateMachinEvent) GetEventName() string {
  return f.eventName
}

func CreateConfigHandler(configStore *resource.ConfigStore) func(context *gin.Context) {

  return func(c *gin.Context) {
    webConfig := configStore.GetWebConfig()
    c.JSON(200, webConfig)
  }
}

func CleanUpConfigFiles() {

  files, _ := filepath.Glob("schema_*_gocms.json")
  log.Infof("Found files to load: %v", files)

  for _, fileName := range files {
    os.Remove(fileName)

  }

}

func GetTablesFromWorld(db *sqlx.DB) ([]resource.TableInfo, error) {

  ts := make([]resource.TableInfo, 0)

  res, err := db.Queryx("select table_name, permission, default_permission, schema_json, is_top_level, is_hidden" +
      " from world where deleted_at is null and table_name not like '%_has_%' and table_name not in ('world', 'world_column', 'action', 'user', 'usergroup')")
  if err != nil {
    log.Infof("Failed to select from world table: %v", err)
    return ts, err
  }

  for res.Next() {
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

    var t resource.TableInfo

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

//func GetActionList(typename string, initConfig *CmsConfig) []resource.Action {
//
//  actions := make([]resource.Action, 0)
//
//  for _, a := range initConfig.Actions {
//    if a.OnType == typename {
//      actions = append(actions, a)
//    }
//  }
//  return actions
//}

type JsModel struct {
  ColumnModel   map[string]interface{}
  Actions       []resource.Action
  StateMachines []map[string]interface{}
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

func AddResourcesToApi2Go(api *api2go.API, tables []resource.TableInfo, db *sqlx.DB, ms *resource.MiddlewareSet, configStore *resource.ConfigStore) map[string]*resource.DbResource {
  cruds := make(map[string]*resource.DbResource)
  for _, table := range tables {
    log.Infof("Table [%v] Relations: %v", table.TableName)
    for _, r := range table.Relations {
      log.Infof("Relation :: %v", r.String())
    }
    model := api2go.NewApi2GoModel(table.TableName, table.Columns, table.DefaultPermission, table.Relations)

    res := resource.NewDbResource(model, db, ms, cruds, configStore)

    cruds[table.TableName] = res
    api.AddResource(model, res)
  }
  return cruds
}
