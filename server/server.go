package server

import (
  "github.com/artpar/api2go"
  "github.com/artpar/api2go-adapter/gingonic"
  log "github.com/sirupsen/logrus"
  "gopkg.in/gin-gonic/gin.v1"
  "github.com/artpar/goms/server/auth"
  "github.com/artpar/goms/server/resource"
  "github.com/jamiealquiza/envy"
  "github.com/jmoiron/sqlx"
  "net/http"
  //"strings"
  "encoding/json"
  "fmt"
  "io/ioutil"
  "path/filepath"
  //"github.com/pkg/errors"
  "flag"
  uuid2 "github.com/satori/go.uuid"
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

func Main(boxRoot, boxStatic http.FileSystem) {

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

  db, err := GetDbConnection(*db_type, *connection_string)
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

  /// end system initialise

  r := gin.Default()
  r.Use(CorsMiddlewareFunc)

  //r.StaticFS("/static", http.Dir("./gomsweb/dist/static"))
  //boxStatic := rice.MustFindBox("./gomsweb/dist/static").HTTPBox()
  r.StaticFS("/static", boxStatic)
  //r.StaticFile("", "./gomsweb/dist/index.html")

  r.GET("/favicon.ico", func(c *gin.Context) {

    file, err := boxRoot.Open("index.html")
    fileContents, err := ioutil.ReadAll(file)
    _, err = c.Writer.Write(fileContents)
    resource.CheckErr(err, "Failed to write favico")
  })
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
  ms := BuildMiddlewareSet(&initConfig)
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
    file, err := boxRoot.Open("index.html")
    fileContents, err := ioutil.ReadAll(file)
    _, err = c.Writer.Write(fileContents)
    resource.CheckErr(err, "Failed to write index html")
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

func CreateConfigHandler(configStore *resource.ConfigStore) func(context *gin.Context) {

  return func(c *gin.Context) {
    webConfig := configStore.GetWebConfig()
    c.JSON(200, webConfig)
  }
}


func AddResourcesToApi2Go(api *api2go.API, tables []resource.TableInfo, db *sqlx.DB, ms *resource.MiddlewareSet, configStore *resource.ConfigStore) map[string]*resource.DbResource {
  cruds = make(map[string]*resource.DbResource)
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
