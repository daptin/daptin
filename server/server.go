package server

import (
	"github.com/artpar/api2go"
	"github.com/artpar/api2go-adapter/gingonic"
	log "github.com/sirupsen/logrus"
	"gopkg.in/gin-gonic/gin.v1"
	"github.com/artpar/goms/server/auth"
	"github.com/artpar/goms/server/resource"
	"github.com/jamiealquiza/envy"
	"net/http"
	"fmt"
	"io/ioutil"
	"flag"
	"github.com/jmoiron/sqlx"
	"github.com/julienschmidt/httprouter"
	"context"
	"github.com/artpar/rclone/fs"
	"strings"
	"github.com/gin-gonic/gin/json"
	"github.com/artpar/rclone/cmd"
	"github.com/satori/go.uuid"
)

var cruds = make(map[string]*resource.DbResource)

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
	//initConfig.Tables = append(initConfig.Tables, existingTables...)
	existingTablesMap := make(map[string]bool)

	allTables := make([]resource.TableInfo, 0)

	for j, existableTable := range existingTables {
		existingTablesMap[existableTable.TableName] = true
		var isBeingModified = false
		var indexBeingModified = -1

		for i, newTable := range initConfig.Tables {
			if newTable.TableName == existableTable.TableName {
				isBeingModified = true
				indexBeingModified = i
				break
			}
		}

		if isBeingModified {
			log.Infof("Table %s is being modified", existableTable.TableName)
			tableBeingModified := initConfig.Tables[indexBeingModified]

			if len(tableBeingModified.Columns) > 0 {

				for _, newColumnDef := range tableBeingModified.Columns {
					columnAlreadyExist := false
					for _, existingColumn := range existableTable.Columns {
						if existingColumn.ColumnName == newColumnDef.ColumnName {
							columnAlreadyExist = true
							break
						}
					}
					if columnAlreadyExist {
						log.Infof("Modifying existing columns[%v][%v] is not supported at present. not sure what would break. and alter query isnt being run currently.", existableTable.TableName, newColumnDef.Name);
					} else {
						existableTable.Columns = append(existableTable.Columns, newColumnDef)
					}

				}

			}
			if len(tableBeingModified.Relations) > 0 {
				existableTable.Relations = append(existableTable.Relations, tableBeingModified.Relations...)
			}
			existingTables[j] = existableTable
		}
		allTables = append(allTables, existableTable)
	}

	for _, newTable := range initConfig.Tables {
		if existingTablesMap[newTable.TableName] {
			continue
		}
		allTables = append(allTables, newTable)

	}
	initConfig.Tables = allTables
	fs.LoadConfig()
	fs.Config.DryRun = false
	fs.Config.LogLevel = 200
	fs.Config.StatsLogLevel = 200

	resource.CheckRelations(&initConfig, db)
	resource.CheckAuditTables(&initConfig, db)

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
	r.StaticFS("/static", boxStatic)

	r.GET("/favicon.ico", func(c *gin.Context) {

		file, err := boxRoot.Open("index.html")
		fileContents, err := ioutil.ReadAll(file)
		_, err = c.Writer.Write(fileContents)
		resource.CheckErr(err, "Failed to write favico")
	})

	configStore, err := resource.NewConfigStore(db)
	jwtSecret, err := configStore.GetConfigValueFor("jwt.secret", "backend")


	if err != nil {
		newSecret := uuid.NewV4().String()
		configStore.SetConfigValueFor("jwt.secret", newSecret, "backend")
		jwtSecret = newSecret
	}


	resource.CheckError(err, "Failed to get config store")
	err = CheckSystemSecrets(configStore)
	resource.CheckErr(err, "Failed to initialise system secrets")

	r.GET("/config", CreateConfigHandler(configStore))

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
	hostSwitch := CreateSubSites(&initConfig, db, cruds)

	hostSwitch.handlerMap["default"] = r

	authMiddleware.SetUserCrud(cruds["user"])
	authMiddleware.SetUserGroupCrud(cruds["usergroup"])
	authMiddleware.SetUserUserGroupCrud(cruds["user_user_id_has_usergroup_usergroup_id"])

	fsmManager := resource.NewFsmManager(db, cruds)

	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	r.GET("/jsmodel/:typename", CreateJsModelHandler(&initConfig))
	r.GET("/apiblueprint.json", CreateApiBlueprintHandler(&initConfig, cruds))
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

	resource.InitialiseColumnManager()

	//r.Run(fmt.Sprintf(":%v", *port))

	http.ListenAndServe(fmt.Sprintf(":%v", *port), hostSwitch)
}

type HostSwitch struct {
	handlerMap map[string]http.Handler
	siteMap    map[string]resource.SubSite
}

// Implement the ServerHTTP method on our new type
func (hs HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if a http.Handler is registered for the given host.
	// If yes, use it to handle the request.
	if handler := hs.handlerMap[r.Host]; handler != nil {
		handler.ServeHTTP(w, r)
	} else {
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) > 1 {

			firstSubFolder := pathParts[1]
			subSite, isSubSite := hs.siteMap[firstSubFolder]
			if isSubSite {
				r.URL.Path = "/" + strings.Join(pathParts[2:], "/")
				handler := hs.handlerMap[subSite.Hostname]
				handler.ServeHTTP(w, r)
				return
			}
		}

		handler := hs.handlerMap["default"]
		handler.ServeHTTP(w, r)

		// Handle host names for wich no handler is registered
		//http.Error(w, "Forbidden", 403) // Or Redirect?
	}
}

func CreateSubSites(config *resource.CmsConfig, db *sqlx.DB, cruds map[string]*resource.DbResource) (HostSwitch) {

	router := httprouter.New()
	router.ServeFiles("/*filepath", http.Dir("./scripts"))

	hs := HostSwitch{}
	hs.handlerMap = make(map[string]http.Handler)
	hs.siteMap = make(map[string]resource.SubSite)

	sites, err := cruds["site"].GetAllSites()
	stores, err := cruds["cloud_store"].GetAllCloudStores()
	cloudStoreMap := make(map[int64]resource.CloudStore)

	for _, store := range stores {
		cloudStoreMap[store.Id] = store
	}

	if err != nil {
		log.Errorf("Failed to load sites from database: %v", err)
		return hs
	}

	for _, site := range sites {
		hs.siteMap[site.Path] = site
		log.Infof("Site to subhost: %v", site)

		cloudStore, ok := cloudStoreMap[site.CloudStoreId]
		storeProvider := cloudStore.StoreProvider
		if !ok {
			log.Infof("Site [%v] does not have a associated storage", site.Name)
			continue
		}

		oauthTokenId := cloudStore.OAutoTokenId

		token, err := cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId)
		oauthConf, err := cruds["oauth_token"].GetOauthDescriptionByTokenReferenceId(oauthTokenId)
		if err != nil {
			log.Errorf("Failed to get oauth token for store sync: %v", err)
			continue
		}

		if !token.Valid() {
			ctx := context.Background()
			tokenSource := oauthConf.TokenSource(ctx, token)
			token, err = tokenSource.Token()
			resource.CheckErr(err, "Failed to get new access token")
			err = cruds["oauth_token"].UpdateAccessTokenByTokenReferenceId(oauthTokenId, token.AccessToken, token.Expiry.Unix())
			resource.CheckErr(err, "failed to update access token")
		}

		sourceDirectoryName := uuid.NewV4().String()
		tempDirectoryPath, err := ioutil.TempDir("", sourceDirectoryName)

		hostRouter := httprouter.New()

		jsonToken, err := json.Marshal(token)
		resource.CheckErr(err, "Failed to convert token to json")
		fs.ConfigFileSet(storeProvider, "client_id", oauthConf.ClientID)
		fs.ConfigFileSet(storeProvider, "type", storeProvider)
		fs.ConfigFileSet(storeProvider, "client_secret", oauthConf.ClientSecret)
		fs.ConfigFileSet(storeProvider, "token", string(jsonToken))
		fs.ConfigFileSet(storeProvider, "client_scopes", strings.Join(oauthConf.Scopes, ","))
		fs.ConfigFileSet(storeProvider, "redirect_url", oauthConf.RedirectURL)

		args := []string{
			cloudStore.RootPath,
			tempDirectoryPath,
		}

		fsrc, fdst := cmd.NewFsSrcDst(args)
		log.Infof("Temp dir for site [%v] ==> %v", site.Name, tempDirectoryPath)
		go cmd.Run(true, true, nil, func() error {
			log.Infof("Starting to copy drive for site base from [%v] to [%v]", fsrc.String(), fdst.String())
			if fsrc == nil || fdst == nil {
				log.Errorf("Source or destination is null")
				return nil
			}
			dir := fs.CopyDir(fdst, fsrc)
			return dir
		})
		hostRouter.ServeFiles("/*filepath", http.Dir(tempDirectoryPath))

		hs.handlerMap[site.Hostname] = hostRouter
	}

	return hs
}
