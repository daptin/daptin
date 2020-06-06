package server

import (
	"fmt"
	"io"
	"os"
	"strings"
	//"sync"
	"time"

	server2 "github.com/fclairamb/ftpserver/server"

	"github.com/artpar/api2go"
	"github.com/artpar/api2go-adapter/gingonic"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-imap-idle"
	"github.com/artpar/go-imap/server"
	"github.com/artpar/go.uuid"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/stats"
	"github.com/aviddiviner/gin-limit"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/websockets"
	"github.com/gin-gonic/gin"
	"github.com/icrowley/fake"
	rateLimit "github.com/yangxikun/gin-limit-by-key"
	"golang.org/x/time/rate"
	//"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"

	graphqlhandler "github.com/graphql-go/handler"
	log "github.com/sirupsen/logrus"
)

var TaskScheduler resource.TaskScheduler
var Stats = stats.New()

func Main(boxRoot http.FileSystem, db database.DatabaseConnection) (HostSwitch, *guerrilla.Daemon,
	resource.TaskScheduler, *resource.ConfigStore, *resource.CertificateManager, *server2.FtpServer, *server.Server) {

	/// Start system initialise
	log.Infof("Load config files")
	initConfig, errs := LoadConfigFiles()
	if errs != nil {
		for _, err := range errs {
			log.Errorf("Failed to load config indexFile: %v", err)
		}
	}

	existingTables, _ := GetTablesFromWorld(db)

	allTables := MergeTables(existingTables, initConfig.Tables)

	initConfig.Tables = allTables

	// rclone config load
	config.LoadConfig()
	fs.Config.DryRun = false
	fs.Config.LogLevel = 200
	fs.Config.StatsLogLevel = 200

	initialiseResources(&initConfig, db)

	/// end system initialise

	defaultRouter := gin.Default()

	defaultRouter.Use(func() gin.HandlerFunc {
		return func(c *gin.Context) {
			beginning, recorder := Stats.Begin(c.Writer)
			defer Stats.End(beginning, stats.WithRecorder(recorder))
			c.Next()
		}
	}())

	defaultRouter.GET("/statistics", func(c *gin.Context) {
		c.JSON(http.StatusOK, Stats.Data())
	})

	// 6 UID FETCH 1:2 (UID)
	defaultRouter.Use(NewCorsMiddleware().CorsMiddlewareFunc)
	defaultRouter.StaticFS("/statics", NewSubPathFs(boxRoot, "/statics"))
	defaultRouter.StaticFS("/js", NewSubPathFs(boxRoot, "/js"))
	defaultRouter.StaticFS("/css", NewSubPathFs(boxRoot, "/css"))
	defaultRouter.StaticFS("/fonts", NewSubPathFs(boxRoot, "/fonts"))

	defaultRouter.GET("/favicon.ico", func(c *gin.Context) {

		file, err := boxRoot.Open("static/img/favicon.png")
		if err != nil {
			c.AbortWithStatus(404)
			return
		}

		fileContents, err := ioutil.ReadAll(file)
		if err != nil {
			c.AbortWithStatus(404)
			return
		}
		_, err = c.Writer.Write(fileContents)
		resource.CheckErr(err, "Failed to write favico")
	})

	defaultRouter.GET("/favicon.png", func(c *gin.Context) {

		file, err := boxRoot.Open("static/img/favicon.png")
		if err != nil {
			c.AbortWithStatus(404)
			return
		}

		fileContents, err := ioutil.ReadAll(file)
		if err != nil {
			c.AbortWithStatus(404)
			return
		}
		_, err = c.Writer.Write(fileContents)
		resource.CheckErr(err, "Failed to write favicon")
	})

	configStore, err := resource.NewConfigStore(db)
	resource.CheckErr(err, "Failed to get config store")
	defaultRouter.Use(NewLanguageMiddleware(configStore).LanguageMiddlewareFunc)

	maxConnections, err := configStore.GetConfigIntValueFor("limit.max_connectioins", "backend")
	if err != nil {
		maxConnections = 100
		err = configStore.SetConfigValueFor("limit.max_connections", maxConnections, "backend")
		resource.CheckErr(err, "Failed to store limit.max_connections default value in db")
	}
	defaultRouter.Use(limit.MaxAllowed(maxConnections))

	rate1, err := configStore.GetConfigIntValueFor("limit.rate", "backend")
	if err != nil {
		rate1 = 100
		err = configStore.SetConfigValueFor("limit.rate", rate1, "backend")
		resource.CheckErr(err, "Failed to store limit.rate default value in db")
	}
	defaultRouter.Use(rateLimit.NewRateLimiter(func(c *gin.Context) string {
		return c.ClientIP() // limit rate by client ip
	}, func(c *gin.Context) (*rate.Limiter, time.Duration) {
		return rate.NewLimiter(rate.Every(100*time.Millisecond), rate1), time.Hour // limit 10 qps/clientIp and permit bursts of at most 10 tokens, and the limiter liveness time duration is 1 hour
	}, func(c *gin.Context) {
		c.AbortWithStatus(429) // handle exceed rate limit request
	}))

	hostname, err := configStore.GetConfigValueFor("hostname", "backend")
	if err != nil {
		name, e := os.Hostname()
		if e != nil {
			name = "localhost"
		}
		hostname = name
		err = configStore.SetConfigValueFor("hostname", hostname, "backend")
		resource.CheckErr(err, "Failed to store hostname in _config")
	}

	initConfig.Hostname = hostname

	jwtSecret, err := configStore.GetConfigValueFor("jwt.secret", "backend")
	if err != nil {
		u, _ := uuid.NewV4()
		newSecret := u.String()
		err = configStore.SetConfigValueFor("jwt.secret", newSecret, "backend")
		resource.CheckErr(err, "Failed to store secret in database")
		jwtSecret = newSecret
	}

	enablelogs, err := configStore.GetConfigValueFor("logs.enable", "backend")
	if err != nil {
		err = configStore.SetConfigValueFor("logs.enable", "false", "backend")
		resource.CheckErr(err, "Failed to store a default value for logs.enable")
	}

	var ok bool
	LogFileLocation, ok := os.LookupEnv("DAPTIN_LOG_LOCATION")
	if !ok || LogFileLocation == "" {
		LogFileLocation = "daptin.log"
	}

	go func() {

		for {

			fileInfo, err := os.Stat(LogFileLocation)
			if err != nil {
				log.Errorf("Failed to stat log file: %v", err)
				time.Sleep(30 * time.Minute)
				continue
			}

			fileMbs := fileInfo.Size() / (1024 * 1024)
			//log.Printf("Current log size: %d MB", fileMbs)
			if fileMbs > 100 {
				err = os.Remove(LogFileLocation)
				resource.CheckErr(err, "Failed to remove log file [%v]", LogFileLocation)
				_, err = os.Create(LogFileLocation)
				resource.CheckErr(err, "Failed to create new log file after cleanup")
				f, e := os.OpenFile(LogFileLocation, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
				if e != nil {
					log.Errorf("Failed to open logfile %v", e)
				}

				mwriter := io.MultiWriter(f, os.Stdout)

				log.SetOutput(mwriter)
				log.Infof("Truncated log file, cleaned %d MB", fileMbs)

			}
			time.Sleep(30 * time.Minute)

		}
	}()

	if enablelogs == "true" {

		//defaultRouter.GET("/__logs", func(c *gin.Context) {
		//	webtail.ViewLog(c.Writer, c.Request, []httprouter.Param{
		//		{
		//			Key:   "name",
		//			Value: "daptin.log",
		//		},
		//	})
		//})
	}

	enableGraphql, err := configStore.GetConfigValueFor("graphql.enable", "backend")
	if err != nil {
		err = configStore.SetConfigValueFor("graphql.enable", fmt.Sprintf("%v", initConfig.EnableGraphQL), "backend")
		resource.CheckErr(err, "Failed to set a default value for graphql.enable")
	} else {
		if enableGraphql == "true" {
			initConfig.EnableGraphQL = true
		} else {
			initConfig.EnableGraphQL = false
		}
	}

	err = CheckSystemSecrets(configStore)
	resource.CheckErr(err, "Failed to initialise system secrets")

	jwtTokenIssuer, err := configStore.GetConfigValueFor("jwt.token.issuer", "backend")
	resource.CheckErr(err, "No default jwt token issuer set")
	if err != nil {
		uid, _ := uuid.NewV4()
		jwtTokenIssuer = "daptin-" + uid.String()[0:6]
		err = configStore.SetConfigValueFor("jwt.token.issuer", jwtTokenIssuer, "backend")
	}
	authMiddleware := auth.NewAuthMiddlewareBuilder(db, jwtTokenIssuer)
	auth.InitJwtMiddleware([]byte(jwtSecret), jwtTokenIssuer)
	defaultRouter.Use(authMiddleware.AuthCheckMiddleware)

	cruds := make(map[string]*resource.DbResource)
	defaultRouter.GET("/actions", resource.CreateGuestActionListHandler(&initConfig))

	api := api2go.NewAPIWithRouting(
		"api",
		api2go.NewStaticResolver("/"),
		gingonic.New(defaultRouter),
	)

	ms := BuildMiddlewareSet(&initConfig, &cruds)
	cruds = AddResourcesToApi2Go(api, initConfig.Tables, db, &ms, configStore, cruds)

	rcloneRetries, err := configStore.GetConfigIntValueFor("rclone.retries", "backend")
	if err != nil {
		rcloneRetries = 5
		configStore.SetConfigIntValueFor("rclone.retries", rcloneRetries, "backend")
	}

	certificateManager, err := resource.NewCertificateManager(cruds, configStore)
	resource.CheckErr(err, "Failed to create certificate manager")

	streamProcessors := GetStreamProcessors(&initConfig, configStore, cruds)

	mailDaemon, err := StartSMTPMailServer(cruds["mail"], certificateManager, hostname)

	if err == nil {
		err = mailDaemon.Start()

		if err != nil {
			log.Errorf("Failed to mail daemon start: %s", err)
		} else {
			log.Infof("Started mail server")
		}
	} else {
		log.Errorf("Failed to start mail daemon: %s", err)
	}

	var imapServer *server.Server
	imapServer = nil
	// Create a memory backend
	enableImapServer, err := configStore.GetConfigValueFor("imap.enabled", "backend")
	if err == nil && enableImapServer == "true" {
		imapListenInterface, err := configStore.GetConfigValueFor("imap.listen_interface", "backend")
		if err != nil {
			err = configStore.SetConfigValueFor("imap.listen_interface", ":1143", "backend")
			resource.CheckErr(err, "Failed to store default imap listen interface in config")
			imapListenInterface = ":1143"
		}

		hostname, err := configStore.GetConfigValueFor("hostname", "backend")
		hostname = "imap." + hostname
		imapBackend := resource.NewImapServer(cruds)

		// Create a new server
		imapServer = server.New(imapBackend)
		imapServer.Addr = imapListenInterface
		imapServer.Debug = nil
		imapServer.AllowInsecureAuth = false
		imapServer.Enable(idle.NewExtension())
		imapServer.Debug = os.Stdout
		//imapServer.EnableAuth("CRAM-MD5", func(conn server.Conn) sasl.Server {
		//
		//	return &Crammd5{
		//		dbResource:  cruds["mail"],
		//		conn:        conn,
		//		imapBackend: imapBackend,
		//	}
		//})

		tlsConfig, _, _, _, _, err := certificateManager.GetTLSConfig(hostname, true)
		resource.CheckErr(err, "Failed to get certificate for IMAP [%v]", hostname)
		imapServer.TLSConfig = tlsConfig

		log.Printf("Starting IMAP server at %s: %v\n", imapListenInterface, hostname)

		go func() {
			if EndsWithCheck(imapListenInterface, ":993") {
				if err := imapServer.ListenAndServeTLS(); err != nil {
					resource.CheckErr(err, "Imap server is not listening anymore 1")
				}
			} else {
				if err := imapServer.ListenAndServe(); err != nil {
					resource.CheckErr(err, "Imap server is not listening anymore 2")
				}
			}
		}()

	} else {
		if err != nil {
			configStore.SetConfigValueFor("imap.enabled", "false", "backend")
		}
	}
	TaskScheduler = resource.NewTaskScheduler(&initConfig, cruds, configStore)

	hostSwitch, subsiteCacheFolders := CreateSubSites(&initConfig, db, cruds, authMiddleware, configStore)

	for k := range cruds {
		cruds[k].SubsiteFolderCache = subsiteCacheFolders
	}

	hostSwitch.handlerMap["api"] = defaultRouter
	hostSwitch.handlerMap["dashboard"] = defaultRouter

	actionPerformers := GetActionPerformers(&initConfig, configStore, cruds, mailDaemon, hostSwitch, certificateManager)
	initConfig.ActionPerformers = actionPerformers

	AddStreamsToApi2Go(api, streamProcessors, db, &ms, configStore)

	// todo : move this somewhere and make it part of something
	actionHandlerMap := actionPerformersListToMap(actionPerformers)
	for k := range cruds {
		cruds[k].ActionHandlerMap = actionHandlerMap
	}

	resource.ImportDataFiles(initConfig.Imports, db, cruds)

	err = TaskScheduler.AddTask(resource.Task{
		EntityName:  "mail_server",
		ActionName:  "sync_mail_servers",
		Attributes:  map[string]interface{}{},
		AsUserEmail: cruds[resource.USER_ACCOUNT_TABLE_NAME].GetAdminEmailId(),
		Schedule:    "@every 1h",
	})

	TaskScheduler.StartTasks()

	assetColumnFolders := CreateAssetColumnSync(cruds)
	for k := range cruds {
		cruds[k].AssetFolderCache = assetColumnFolders
	}

	authMiddleware.SetUserCrud(cruds[resource.USER_ACCOUNT_TABLE_NAME])
	authMiddleware.SetUserGroupCrud(cruds["usergroup"])
	authMiddleware.SetUserUserGroupCrud(cruds["user_account_user_account_id_has_usergroup_usergroup_id"])

	fsmManager := resource.NewFsmManager(db, cruds)

	enableFtp, err := configStore.GetConfigValueFor("ftp.enable", "backend")
	if err != nil {
		enableFtp = "false"
		err = configStore.SetConfigValueFor("ftp.enable", enableFtp, "backend")
		auth.CheckErr(err, "Failed to store default valuel for ftp.enable")
	}

	var ftpServer *server2.FtpServer
	if enableFtp == "true" {

		ftp_interface, err := configStore.GetConfigValueFor("ftp.listen_interface", "backend")
		if err != nil {
			ftp_interface = "0.0.0.0:2121"
			err = configStore.SetConfigValueFor("ftp.listen_interface", ftp_interface, "backend")
			resource.CheckErr(err, "Failed to store default value for ftp.listen_interface")
		}
		// ftpListener, err := net.Listen("tcp", ftp_interface)
		// resource.CheckErr(err, "Failed to create listener for FTP")
		ftpServer, err = CreateFtpServers(cruds, certificateManager, ftp_interface)
		auth.CheckErr(err, "Failed to creat FTP server")
		go func() {
			log.Printf("FTP server started at %v", ftp_interface)
			err = ftpServer.ListenAndServe()
			resource.CheckErr(err, "Failed to listen at ftp interface")
		}()
	}

	defaultRouter.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	handler := CreateJsModelHandler(&initConfig, cruds)
	metaHandler := CreateMetaHandler(&initConfig)
	blueprintHandler := CreateApiBlueprintHandler(&initConfig, cruds)
	modelHandler := CreateReclineModelHandler()
	statsHandler := CreateStatsHandler(&initConfig, cruds)
	resource.InitialiseColumnManager()

	dbAssetHandler := CreateDbAssetHandler(cruds)
	defaultRouter.GET("/asset/:typename/:resource_id/:columnname", dbAssetHandler)

	feedHandler := CreateFeedHandler(cruds, streamProcessors)
	defaultRouter.GET("/feed/:feedname", feedHandler)

	configHandler := CreateConfigHandler(&initConfig, cruds, configStore)
	defaultRouter.GET("/_config/:end/:key", configHandler)
	defaultRouter.GET("/_config", configHandler)
	defaultRouter.POST("/_config/:end/:key", configHandler)
	defaultRouter.PATCH("/_config/:end/:key", configHandler)
	defaultRouter.PUT("/_config/:end/:key", configHandler)
	defaultRouter.DELETE("/_config/:end/:key", configHandler)

	resource.RegisterTranslations()

	if initConfig.EnableGraphQL {

		// TODO: add state machine change api available as graphql
		graphqlSchema := MakeGraphqlSchema(&initConfig, cruds)

		graphqlHttpHandler := graphqlhandler.New(&graphqlhandler.Config{
			Schema:   graphqlSchema,
			Pretty:   true,
			GraphiQL: true,
		})

		// serve HTTP
		defaultRouter.Handle("GET", "/graphql", func(c *gin.Context) {
			graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)
		})
		// serve HTTP
		defaultRouter.Handle("POST", "/graphql", func(c *gin.Context) {
			graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)
		})
		// serve HTTP
		defaultRouter.Handle("PUT", "/graphql", func(c *gin.Context) {
			graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)
		})
		// serve HTTP
		defaultRouter.Handle("PATCH", "/graphql", func(c *gin.Context) {
			graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)
		})
		// serve HTTP
		defaultRouter.Handle("DELETE", "/graphql", func(c *gin.Context) {
			graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)
		})
	}

	defaultRouter.GET("/jsmodel/:typename", handler)
	defaultRouter.GET("/stats/:typename", statsHandler)
	defaultRouter.GET("/meta", metaHandler)
	defaultRouter.GET("/openapi.yaml", blueprintHandler)
	defaultRouter.GET("/recline_model", modelHandler)
	defaultRouter.OPTIONS("/jsmodel/:typename", handler)
	defaultRouter.OPTIONS("/openapi.yaml", blueprintHandler)
	defaultRouter.OPTIONS("/recline_model", modelHandler)
	defaultRouter.GET("/system", func(c *gin.Context) {
		c.AbortWithStatusJSON(200, Stats.Data())
	})

	actionHandler := resource.CreatePostActionHandler(&initConfig, configStore, cruds, actionPerformers)
	defaultRouter.POST("/action/:typename/:actionName", actionHandler)
	defaultRouter.GET("/action/:typename/:actionName", actionHandler)

	defaultRouter.POST("/track/start/:stateMachineId", CreateEventStartHandler(fsmManager, cruds, db))
	defaultRouter.POST("/track/event/:typename/:objectStateId/:eventName", CreateEventHandler(&initConfig, fsmManager, cruds, db))

	//loader := CreateSubSiteContentHandler(&initConfig, cruds, db)
	//defaultRouter.POST("/site/content/load", loader)
	//defaultRouter.GET("/site/content/load", loader)
	//defaultRouter.POST("/site/content/store", CreateSubSiteSaveContentHandler(&initConfig, cruds, db))

	// TODO: make websockets functional at /live
	//webSocketConnectionHandler := WebSocketConnectionHandlerImpl{}
	//websocketServer := websockets.NewServer("/live", &webSocketConnectionHandler)

	//go websocketServer.Listen(defaultRouter)

	indexFile, err := boxRoot.Open("index.html")

	resource.CheckErr(err, "Failed to open index.html file from dashboard directory %v")

	var indexFileContents = []byte("")
	if indexFile != nil && err == nil {
		indexFileContents, err = ioutil.ReadAll(indexFile)
	}

	defaultRouter.NoRoute(func(c *gin.Context) {
		resource.CheckErr(err, "Failed to open index.html")
		if err != nil {
			c.AbortWithStatus(500)
			return
		}
		_, err = c.Writer.Write(indexFileContents)
		resource.CheckErr(err, "Failed to write index html")
	})

	defaultRouter.GET("", func(c *gin.Context) {
		_, err = c.Writer.Write(indexFileContents)
		resource.CheckErr(err, "Failed to write index html")
	})

	//defaultRouter.Run(fmt.Sprintf(":%v", *port))
	CleanUpConfigFiles()
	adminEmail := cruds[resource.USER_ACCOUNT_TABLE_NAME].GetAdminEmailId()
	if adminEmail == "" {
		adminEmail = "No one"
	}
	log.Printf("Our admin is [%v]", adminEmail)

	return hostSwitch, mailDaemon, TaskScheduler, configStore, certificateManager, ftpServer, imapServer

}

func CreateFtpServers(resources map[string]*resource.DbResource, certManager *resource.CertificateManager, ftp_interface string) (*server2.FtpServer, error) {

	subsites, err := resources["site"].GetAllSites()
	if err != nil {
		return nil, err
	}
	cloudStores, err := resources["cloud_store"].GetAllCloudStores()

	if err != nil {
		return nil, err
	}
	cloudStoreMap := make(map[string]resource.CloudStore)
	for _, cloudStore := range cloudStores {
		cloudStoreMap[cloudStore.ReferenceId] = cloudStore
	}
	var driver *DaptinFtpDriver

	sites := make([]SubSiteAssetCache, 0)
	for _, ftpServer := range subsites {

		if !ftpServer.FtpEnabled {
			continue
		}

		assetCacheFolder, ok := resources["site"].SubsiteFolderCache[ftpServer.ReferenceId]
		if !ok {
			continue
		}
		site := SubSiteAssetCache{
			SubSite:          ftpServer,
			AssetFolderCache: assetCacheFolder,
		}
		sites = append(sites, site)

	}

	driver, err = NewDaptinFtpDriver(resources, certManager, ftp_interface, sites)
	ftpS := server2.NewFtpServer(driver)
	resource.CheckErr(err, "Failed to create daptin ftp driver [%v]", driver)
	return ftpS, err

}

type SubSiteAssetCache struct {
	resource.SubSite
	*resource.AssetFolderCache
}

type Crammd5 struct {
	dbResource  *resource.DbResource
	conn        server.Conn
	challenge   string
	imapBackend *resource.DaptinImapBackend
}

// Begins or continues challenge-response authentication. If the client
// supplies an initial response, response is non-nil.
//
// If the authentication is finished, done is set to true. If the
// authentication has failed, an error is returned.
func (c *Crammd5) Next(response []byte) (challenge []byte, done bool, err error) {

	log.Printf(""+
		"Client sent: %v", string(response))

	if string(response) == "" {
		newChallenge := fmt.Sprintf("<%v.%v.%v>", fake.DigitsN(8), time.Now().UnixNano(), "daptin")
		c.challenge = newChallenge
		return []byte(c.challenge), false, nil
	}

	parts := strings.SplitN(string(response), " ", 2)

	_, err = c.imapBackend.LoginMd5(c.conn.Info(), parts[0], c.challenge, parts[1])
	if err != nil {
		return []byte("OK"), true, err
	}

	return []byte("OK"), false, nil
}

func initialiseResources(initConfig *resource.CmsConfig, db database.DatabaseConnection) {
	resource.CheckRelations(initConfig)
	resource.CheckAuditTables(initConfig)
	resource.CheckTranslationTables(initConfig)
	//lock := new(sync.Mutex)
	//AddStateMachines(&initConfig, db)

	var errc error

	resource.CheckAllTableStatus(initConfig, db)
	resource.CheckErr(errc, "Failed to commit transaction after creating tables")

	tx, errb := db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")

	tx, errb = db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")

	if tx != nil {

		resource.CreateRelations(initConfig, tx)
		errc = tx.Commit()
		resource.CheckErr(errc, "Failed to commit transaction after creating relations")
	}

	tx, errb = db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	if tx != nil {
		resource.CreateUniqueConstraints(initConfig, tx)
		errc = tx.Commit()
		resource.CheckErr(errc, "Failed to commit transaction after creating unique constrains")
	}

	tx, errb = db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction for creating indexes")
	if tx != nil {
		resource.CreateIndexes(initConfig, db)
		errc = tx.Commit()
		resource.CheckErr(errc, "Failed to commit transaction after creating indexes")
	}

	tx, errb = db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")

	if tx != nil {
		errb = resource.UpdateWorldTable(initConfig, tx)
		resource.CheckErr(errb, "Failed to update world tables")
		errc := tx.Commit()
		resource.CheckErr(errc, "Failed to commit transaction after updating world tables")
	}

	go func() {
		resource.UpdateStateMachineDescriptions(initConfig, db)
		resource.UpdateExchanges(initConfig, db)
		resource.UpdateStreams(initConfig, db)
		//resource.UpdateMarketplaces(initConfig, db)
		err := resource.UpdateTasksData(initConfig, db)
		resource.CheckErr(err, "Failed to  update cron jobs")
		resource.UpdateStandardData(initConfig, db)

		err = resource.UpdateActionTable(initConfig, db)
		resource.CheckErr(err, "Failed to update action table")
	}()

}

func actionPerformersListToMap(interfaces []resource.ActionPerformerInterface) map[string]resource.ActionPerformerInterface {
	m := make(map[string]resource.ActionPerformerInterface)

	for _, api := range interfaces {
		if api == nil {
			continue
		}
		m[api.Name()] = api
	}
	return m
}

func MergeTables(existingTables []resource.TableInfo, initConfigTables []resource.TableInfo) []resource.TableInfo {
	allTables := make([]resource.TableInfo, 0)
	existingTablesMap := make(map[string]bool)

	for j, existableTable := range existingTables {
		existingTablesMap[existableTable.TableName] = true
		var isBeingModified = false
		var indexBeingModified = -1

		for i, newTable := range initConfigTables {
			if newTable.TableName == existableTable.TableName {
				isBeingModified = true
				indexBeingModified = i
				break
			}
		}

		if isBeingModified {
			//log.Printf("Table %s is being modified", existableTable.TableName)
			tableBeingModified := initConfigTables[indexBeingModified]

			if len(tableBeingModified.Columns) > 0 {

				for _, newColumnDef := range tableBeingModified.Columns {
					columnAlreadyExist := false
					colIndex := -1
					for i, existingColumn := range existableTable.Columns {
						//log.Infof("Table column old/new [%v][%v] == [%v][%v] @ %v", tableBeingModified.TableName, newColumnDef.Name, existableTable.TableName, existingColumn.Name, i)
						if existingColumn.ColumnName == newColumnDef.ColumnName {
							columnAlreadyExist = true
							colIndex = i
							break
						}
					}
					//log.Infof("Decide for table column [%v][%v] @ index: %v [%v]", tableBeingModified.TableName, newColumnDef.Name, colIndex, columnAlreadyExist)
					if columnAlreadyExist {
						//log.Infof("Modifying existing columns[%v][%v] is not supported at present. not sure what would break. and alter query isnt being run currently.", existableTable.Columns[colIndex], newColumnDef);

						existableTable.Columns[colIndex].DefaultValue = newColumnDef.DefaultValue
						existableTable.Columns[colIndex].ExcludeFromApi = newColumnDef.ExcludeFromApi
						existableTable.Columns[colIndex].IsIndexed = newColumnDef.IsIndexed
						existableTable.Columns[colIndex].IsNullable = newColumnDef.IsNullable
						existableTable.Columns[colIndex].ColumnType = newColumnDef.ColumnType
						existableTable.Columns[colIndex].Options = newColumnDef.Options
						existableTable.Columns[colIndex].DataType = newColumnDef.DataType
						existableTable.Columns[colIndex].ColumnType = newColumnDef.ColumnType
						existableTable.Columns[colIndex].ForeignKeyData = newColumnDef.ForeignKeyData

					} else {
						existableTable.Columns = append(existableTable.Columns, newColumnDef)
					}
				}

			}
			if len(tableBeingModified.Relations) > 0 {

				existingRelations := existableTable.Relations
				relMap := make(map[string]bool)
				for _, rel := range existingRelations {
					relMap[rel.Hash()] = true
				}

				for _, newRel := range tableBeingModified.Relations {

					_, ok := relMap[newRel.Hash()]
					if !ok {
						existableTable.AddRelation(newRel)
					}
				}
			}
			existableTable.DefaultGroups = tableBeingModified.DefaultGroups
			existableTable.Conformations = tableBeingModified.Conformations
			existableTable.Validations = tableBeingModified.Validations
			existingTables[j] = existableTable
		} else {
			//log.Infof("Table %s is not being modified", existableTable.TableName)
		}
		allTables = append(allTables, existableTable)
	}

	for _, newTable := range initConfigTables {
		if existingTablesMap[newTable.TableName] {
			continue
		}
		allTables = append(allTables, newTable)
	}

	return allTables

}

func NewSubPathFs(system http.FileSystem, s string) http.FileSystem {
	return &SubPathFs{system: system, subPath: s}
}

type SubPathFs struct {
	system  http.FileSystem
	subPath string
}

func (spf *SubPathFs) Open(name string) (http.File, error) {
	//log.Infof("Service file from static path: %s/%s", spf.subPath, name)
	return spf.system.Open(spf.subPath + name)
}

type WebSocketConnectionHandlerImpl struct {
}

func (wsch *WebSocketConnectionHandlerImpl) MessageFromClient(message websockets.WebSocketPayload, request *http.Request) {
	// todo: complete implementation
}

func AddStreamsToApi2Go(api *api2go.API, processors []*resource.StreamProcessor, db database.DatabaseConnection, middlewareSet *resource.MiddlewareSet, configStore *resource.ConfigStore) {

	for _, processor := range processors {

		contract := processor.GetContract()
		model := api2go.NewApi2GoModel(contract.StreamName, contract.Columns, 0, nil)
		api.AddResource(model, processor)

	}

}

func GetStreamProcessors(config *resource.CmsConfig, store *resource.ConfigStore, cruds map[string]*resource.DbResource) []*resource.StreamProcessor {

	allProcessors := make([]*resource.StreamProcessor, 0)

	for _, streamContract := range config.Streams {

		streamProcessor := resource.NewStreamProcessor(streamContract, cruds)
		allProcessors = append(allProcessors, streamProcessor)

	}

	return allProcessors

}
