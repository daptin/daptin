package server

import (
	"context"
	"fmt"
	"github.com/artpar/api2go-adapter/gingonic"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/action_provider"
	"github.com/daptin/daptin/server/dbresourceinterface"
	"github.com/daptin/daptin/server/fsm"
	"github.com/daptin/daptin/server/hostswitch"
	"github.com/daptin/daptin/server/subsite"
	"github.com/daptin/daptin/server/table_info"
	"github.com/daptin/daptin/server/task"
	"github.com/daptin/daptin/server/task_scheduler"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/golang-lru"
	"github.com/sadlil/go-trigger"
	"os"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-imap/server"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/stats"
	"github.com/artpar/ydb"
	"github.com/aviddiviner/gin-limit"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/websockets"
	server2 "github.com/fclairamb/ftpserver/server"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"

	log "github.com/sirupsen/logrus"
)

var TaskScheduler task_scheduler.TaskScheduler
var Stats = stats.New()

type RateConfig struct {
	version string
	limits  map[string]int
}

var defaultRateConfig = RateConfig{
	version: "default",
	limits:  map[string]int{},
}

type YjsConnectionSessionFetcher struct {
}

func (y *YjsConnectionSessionFetcher) GetSessionId(r *http.Request, roomname string) uint64 {

	return 0
}

// Configure these values based on your requirements
const (
	// Server-side cache settings
	cacheSize          = 1000             // Number of files to cache
	maxFileSizeToCache = 10 * 1024 * 1024 // 10MB max file size to cache

	// Client-side cache settings
	cacheMaxAge          = 86400     // 1 day in seconds
	cacheStaleIfError    = 86400 * 7 // 7 days in seconds
	cacheStaleRevalidate = 43200     // 12 hours in seconds
)

var (
	diskFileCache     *lru.Cache
	indexFileContents []byte
)

func Main(boxRoot http.FileSystem, db database.DatabaseConnection, localStoragePath string, olricDb *olric.EmbeddedClient) (
	hostswitch.HostSwitch, *guerrilla.Daemon, task_scheduler.TaskScheduler, *resource.ConfigStore, *resource.CertificateManager,
	*server2.FtpServer, *server.Server, *olric.EmbeddedClient) {

	PrintCliBanner()

	/// Start system initialise
	log.Printf("Load config files")
	initConfig, errs := LoadConfigFiles()
	if errs != nil {
		for _, err := range errs {
			log.Errorf("Failed to load config indexFile: %v", err)
		}
	}

	skipDbConfig, skipValueFound := os.LookupEnv("DAPTIN_SKIP_CONFIG_FROM_DATABASE")

	var existingTables []table_info.TableInfo
	if skipValueFound && skipDbConfig == "true" {
		log.Printf("ENV[DAPTIN_SKIP_CONFIG_FROM_DATABASE] skip loading existing tables config from database")
	} else {
		log.Printf("ENV[DAPTIN_SKIP_CONFIG_FROM_DATABASE] loading existing tables config from database")
		existingTables, _ = GetTablesFromWorld(db)
		allTables := MergeTables(existingTables, initConfig.Tables)
		initConfig.Tables = allTables
	}

	// rclone config load
	//configfile.Install()
	defaultConfig := fs.GetConfig(nil)
	defaultConfig.DryRun = false
	defaultConfig.LogLevel = fs.LogLevelDebug
	defaultConfig.StatsLogLevel = fs.LogLevelDebug

	skipResourceInitialise, ok := os.LookupEnv("DAPTIN_SKIP_INITIALISE_RESOURCES")
	if ok && skipResourceInitialise == "true" {
		log.Infof("Skipping db resource initialise: %v", skipResourceInitialise)
	} else {
		log.Infof("ENV[DAPTIN_SKIP_INITIALISE_RESOURCES] value: %v", skipResourceInitialise)
		InitialiseServerResources(&initConfig, db)
	}

	configStore, err := resource.NewConfigStore(db)
	resource.CheckErr(err, "Failed to get config store")
	diskFileCache, err = lru.New(cacheSize)

	transaction, err := db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [122]")
		panic(err)
	}

	hostname, err := configStore.GetConfigValueFor("hostname", "backend", transaction)
	if err != nil {
		name, e := os.Hostname()
		if e != nil {
			name = "localhost"
		}
		hostname = name
		err = configStore.SetConfigValueFor("hostname", hostname, "backend", transaction)
		resource.CheckErr(err, "Failed to store hostname in _config")
	}

	initConfig.Hostname = hostname

	defaultRouter := gin.Default()

	enableGzip, err := configStore.GetConfigValueFor("gzip.enable", "backend", transaction)
	if err != nil {
		enableGzip = "true"
		err = configStore.SetConfigValueFor("gzip.enable", enableGzip, "backend", transaction)
		resource.CheckErr(err, "Failed to store gzip.enable in _config")
	}
	transaction.Commit()

	if enableGzip == "true" {
		defaultRouter.Use(gzip.Gzip(gzip.DefaultCompression,
			gzip.WithExcludedExtensions([]string{".pdf", ".mp4", ".jpg", ".png", ".wav", ".gif", ".mp3"}),
			gzip.WithExcludedPaths([]string{"/asset/"})),
		)
	}

	defaultRouter.Use(func() gin.HandlerFunc {
		return func(c *gin.Context) {
			beginning, recorder := Stats.Begin(c.Writer)
			c.Next()
			Stats.End(beginning, stats.WithRecorder(recorder))
		}
	}())

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [214]")
	}

	languageMiddleware := NewLanguageMiddleware(configStore, transaction)

	maxConnections, err := configStore.GetConfigIntValueFor("limit.max_connections", "backend", transaction)
	if err != nil {
		maxConnections = 100
		err = configStore.SetConfigValueFor("limit.max_connections", maxConnections, "backend", transaction)
		resource.CheckErr(err, "Failed to store limit.max_connections default value in db")
	}
	log.Printf("Limiting max connections per IP: %v", maxConnections)
	rateConfigJson, err := configStore.GetConfigValueFor("limit.rate", "backend", transaction)
	if err != nil {
		rateConfigJson = "{\"version\":\"default\"}"
		err = configStore.SetConfigValueFor("limit.rate", rateConfigJson, "backend", transaction)
		resource.CheckErr(err, "Failed to store limit.rate default value in db")
	}

	var rateConfig RateConfig
	err = json.Unmarshal([]byte(rateConfigJson), rateConfig)
	if err != nil || rateConfig.version == "" {
		rateConfig = defaultRateConfig
		rateConfigJson = "{\"version\":\"default\"}"
		err = configStore.SetConfigValueFor("limit.rate", rateConfigJson, "backend", transaction)
		resource.CheckErr(err, "Failed to store limit.rate default value in db")
	}
	_ = transaction.Commit()

	var rateLimiter = CreateRateLimiterMiddleware(rateConfig)

	defaultRouter.Use(NewCorsMiddleware().CorsMiddlewareFunc)
	defaultRouter.Use(limit.MaxAllowed(maxConnections))
	defaultRouter.Use(rateLimiter)

	defaultRouter.GET("/statistics", CreateStatisticsHandler(db))

	defaultRouter.StaticFS("/static", NewSubPathFs(boxRoot, "/static"))
	defaultRouter.StaticFS("/statics", NewSubPathFs(boxRoot, "/statics"))
	defaultRouter.StaticFS("/js", NewSubPathFs(boxRoot, "/js"))
	defaultRouter.StaticFS("/css", NewSubPathFs(boxRoot, "/css"))
	defaultRouter.StaticFS("/fonts", NewSubPathFs(boxRoot, "/fonts"))

	// Handle both favicon.ico and favicon.png with aggressive caching
	defaultRouter.GET("/favicon.:format", CreateFaviconEndpoint(boxRoot))

	defaultRouter.Use(languageMiddleware.LanguageMiddlewareFunc)

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [264]")
	}

	jwtSecret, err := configStore.GetConfigValueFor("jwt.secret", "backend", transaction)
	if err != nil {
		u, _ := uuid.NewV7()
		newSecret := u.String()
		err = configStore.SetConfigValueFor("jwt.secret", newSecret, "backend", transaction)
		resource.CheckErr(err, "Failed to store secret in database")
		jwtSecret = newSecret
	}

	enableGraphql, err := configStore.GetConfigValueFor("graphql.enable", "backend", transaction)
	if err != nil {
		err = configStore.SetConfigValueFor("graphql.enable", fmt.Sprintf("%v", initConfig.EnableGraphQL), "backend", transaction)
		resource.CheckErr(err, "Failed to set a default value for graphql.enable")
	} else {
		if enableGraphql == "true" {
			initConfig.EnableGraphQL = true
		} else {
			initConfig.EnableGraphQL = false
		}
	}

	err = CheckSystemSecrets(configStore, transaction)
	resource.CheckErr(err, "Failed to initialise system secrets")
	transaction.Commit()

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [294]")
	}

	jwtTokenIssuer, err := configStore.GetConfigValueFor("jwt.token.issuer", "backend", transaction)
	resource.CheckErr(err, "No default jwt token issuer set")
	if err != nil {
		uid, _ := uuid.NewV7()
		jwtTokenIssuer = "daptin-" + uid.String()[0:6]
		err = configStore.SetConfigValueFor("jwt.token.issuer", jwtTokenIssuer, "backend", transaction)
	}
	transaction.Commit()
	authMiddleware := auth.NewAuthMiddlewareBuilder(db, jwtTokenIssuer, olricDb)
	auth.InitJwtMiddleware([]byte(jwtSecret), jwtTokenIssuer, olricDb)
	defaultRouter.Use(authMiddleware.AuthCheckMiddleware)

	cruds := make(map[string]*resource.DbResource)
	crudsInterface := make(map[string]dbresourceinterface.DbResourceInterface)
	defaultRouter.GET("/actions", resource.CreateGuestActionListHandler(&initConfig))

	api := api2go.NewAPIWithRouting(
		"api",
		api2go.NewStaticResolver("/"),
		gingonic.New(defaultRouter),
	)

	dtopicMap := make(map[string]*olric.PubSub)

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [322]")
	}

	enableYjs, err := configStore.GetConfigValueFor("yjs.enabled", "backend", transaction)
	if err != nil || enableYjs == "" {
		enableYjs = "true"
		err = configStore.SetConfigValueFor("yjs.enabled", enableYjs, "backend", transaction)
		resource.CheckErr(err, "failed to store default value for yjs.enabled [true]")
	}

	var documentProvider ydb.DocumentProvider
	documentProvider = nil

	if enableYjs == "true" {
		documentProvider = CreateYjsDocumentProvider(configStore, transaction, localStoragePath, documentProvider, cruds)
	} else {
		log.Infof("YJS endpoint is disabled in config")
	}
	transaction.Commit()

	ms := BuildMiddlewareSet(&initConfig, &cruds, documentProvider, &dtopicMap)
	log.Tracef("Created middleware set")
	AddResourcesToApi2Go(api, initConfig.Tables, db, &ms, configStore, olricDb, cruds)
	log.Tracef("Added ResourcesToApi2Go")
	tablesPubSub, err := cruds["world"].OlricDb.NewPubSub()
	resource.CheckErr(err, "Failed to create topic")
	if err != nil {
		log.Fatalf("failed to create olric topic - %v", err)
	}

	tableTopicSubscription := tablesPubSub.Subscribe(context.Background(), "members")
	go func(topicSubscription *redis.PubSub) {
		channel := topicSubscription.Channel()
		for {
			msg := <-channel
			log.Infof("[438] Received message on [%s]: [%v]", msg.Channel, msg.String())
		}
	}(tableTopicSubscription)

	for key, val := range cruds {
		dtopicMap[key] = tablesPubSub
		crudsInterface[key] = val
	}
	log.Tracef("Crated olric topics")

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [396]")
	}

	rcloneRetries, err := configStore.GetConfigIntValueFor("rclone.retries", "backend", transaction)
	if err != nil {
		rcloneRetries = 5
		_ = configStore.SetConfigIntValueFor("rclone.retries", rcloneRetries, "backend", transaction)
	}

	certificateManager, err := resource.NewCertificateManager(cruds, configStore, transaction)
	resource.CheckErr(err, "Failed to create certificate manager")
	if err != nil {
		panic(err)
	}

	streamProcessors := GetStreamProcessors(&initConfig, configStore, cruds)
	AddStreamsToApi2Go(api, streamProcessors, db, &ms, configStore)
	feedHandler := CreateFeedHandler(cruds, streamProcessors, transaction)

	mailDaemon, err := StartSMTPMailServer(cruds["mail"], certificateManager, hostname, transaction)
	transaction.Commit()

	if err == nil {
		disableSmtp := os.Getenv("DAPTIN_DISABLE_SMTP")
		if disableSmtp != "true" && len(mailDaemon.Config.Servers) > 0 {
			log.Infof("Starting SMTP server at port: [%v], set DAPTIN_DISABLE_SMTP=true in environment to disable SMTP server",
				mailDaemon.Config.Servers)
			err = mailDaemon.Start()
		} else {
			log.Infof("SMTP server is disabled since DAPTIN_DISABLE_SMTP=true or no servers configured")
		}

		if err != nil {
			log.Errorf("Failed to mail daemon start: %s", err)
		} else {
			log.Printf("Started mail server")
		}
	} else {
		log.Errorf("Failed to start mail daemon: %s", err)
	}

	var imapServer *server.Server
	imapServer = nil
	// Create a memory backend
	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [442]")
	}

	enableImapServer, err := configStore.GetConfigValueFor("imap.enabled", "backend", transaction)
	if err == nil && enableImapServer == "true" {
		imapServer = InitializeImapResources(configStore, transaction, cruds, imapServer, certificateManager)
	} else {
		if err != nil {
			err = configStore.SetConfigValueFor("imap.enabled", "false", "backend", transaction)
			resource.CheckErr(err, "Failed to set default value for imap.enabled")
		}
	}
	log.Tracef("Processed imps")

	enableCaldav, err := configStore.GetConfigValueFor("caldav.enable", "backend", transaction)
	if err != nil {
		enableCaldav = "false"
		err = configStore.SetConfigValueFor("caldav.enable", enableCaldav, "backend", transaction)
		resource.CheckErr(err, "Failed to store caldav.enable in _config")
	}
	transaction.Commit()

	TaskScheduler = resource.NewTaskScheduler(&initConfig, cruds, configStore)

	skipImportData, skipImportValFound := os.LookupEnv("DAPTIN_SKIP_IMPORT_DATA")
	if skipImportValFound && skipImportData == "true" {
		log.Info("ENV[DAPTIN_SKIP_IMPORT_DATA] skipping importing data from files")
	} else {
		log.Info("ENV[DAPTIN_SKIP_IMPORT_DATA] importing data from files")
		transaction, err = db.Beginx()
		if err != nil {
			resource.CheckErr(err, "Failed to begin transaction [587]")
		}

		resource.ImportDataFiles(initConfig.Imports, transaction, cruds)
		transaction.Commit()
	}

	if localStoragePath != ";" {
		transaction, err = db.Beginx()
		err = resource.CreateDefaultLocalStorage(transaction, localStoragePath)
		if err != nil {
			log.Errorf("Failed to create default local storage: [%v]", err)
			transaction.Rollback()
		} else {
			transaction.Commit()
		}
		resource.CheckErr(err, "Failed to create default local storage at %v", localStoragePath)
	} else {
		log.Tracef("Not creating default local storage")
	}

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [559]")
	}

	hostSwitch, subsiteCacheFolders := CreateSubSites(&initConfig, transaction, cruds, authMiddleware, rateConfig, maxConnections, olricDb)
	transaction.Commit()

	if enableCaldav == "true" {
		InitializeCaldavResources(authMiddleware, defaultRouter)

	}
	log.Tracef("Completed process caldav")

	for k := range cruds {
		cruds[k].SetSubsitesFolderCache(subsiteCacheFolders)
	}

	hostSwitch.HandlerMap["api"] = defaultRouter
	hostSwitch.HandlerMap["dashboard"] = defaultRouter

	actionPerformers := action_provider.GetActionPerformers(&initConfig, configStore, cruds, mailDaemon, hostSwitch, certificateManager)
	initConfig.ActionPerformers = actionPerformers
	transaction, err = db.Beginx()
	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)
	_ = transaction.Rollback()

	// todo : move this somewhere and make it part of something
	actionHandlerMap := ActionPerformersListToMap(actionPerformers)
	for k := range cruds {
		cruds[k].ActionHandlerMap = actionHandlerMap
		cruds[k].EncryptionSecret = []byte(encryptionSecret)
	}

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [634]")
	}

	// Set the Olric client for template cache
	_ = subsite.CreateTemplateHooks(transaction, crudsInterface, hostSwitch, olricDb)
	_ = transaction.Commit()

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [642]")
	}

	err = TaskScheduler.AddTask(task.Task{
		EntityName:  "mail_server",
		ActionName:  "sync_mail_servers",
		Attributes:  map[string]interface{}{},
		AsUserEmail: cruds[resource.USER_ACCOUNT_TABLE_NAME].GetAdminEmailId(transaction),
		Schedule:    "@every 1h",
	})
	transaction.Rollback()

	TaskScheduler.StartTasks()

	transaction = db.MustBegin()
	assetColumnFolders := CreateAssetColumnSync(crudsInterface, transaction)
	transaction.Commit()
	for k := range cruds {
		cruds[k].AssetFolderCache = assetColumnFolders
	}

	authMiddleware.SetUserCrud(cruds[resource.USER_ACCOUNT_TABLE_NAME])
	authMiddleware.SetUserGroupCrud(cruds["usergroup"])
	authMiddleware.SetUserUserGroupCrud(cruds["user_account_user_account_id_has_usergroup_usergroup_id"])

	fsmManager := fsm.NewFsmManager(db)

	transaction = db.MustBegin()
	enableFtp, err := configStore.GetConfigValueFor("ftp.enable", "backend", transaction)
	if err != nil {
		enableFtp = "false"
		err = configStore.SetConfigValueFor("ftp.enable", enableFtp, "backend", transaction)
		auth.CheckErr(err, "Failed to store default valuel for ftp.enable")
	}

	var ftpServer *server2.FtpServer
	if enableFtp == "true" {

		ftpServer = InitializeFtpResources(configStore, transaction, ftpServer, cruds, crudsInterface, certificateManager)
	}

	defaultRouter.GET("/ping", func(c *gin.Context) {
		transaction, err := cruds["world"].Connection().Beginx()
		if err != nil {
			resource.CheckErr(err, "Failed to begin transaction [665]")
			c.String(500, fmt.Sprintf("%v", err))
		}
		//_, err := cruds["world"].GetObjectByWhereClause("world", "table_name", "world")
		_ = transaction.Rollback()
		c.String(200, "pong")
	})

	jsModelHandler := CreateJsModelHandler(&initConfig, cruds, transaction)
	transaction.Commit()
	blueprintHandler := CreateApiBlueprintHandler(&initConfig, cruds)
	statsHandler := CreateStatsHandler(&initConfig, cruds)
	resource.InitialiseColumnManager()
	metaHandler := CreateMetaHandler(&initConfig)

	dbAssetHandler := CreateDbAssetHandler(cruds, olricDb)
	defaultRouter.GET("/asset/:typename/:resource_id/:columnname", dbAssetHandler)

	// Asset upload endpoints - properly organized
	assetUploadHandler := AssetUploadHandler(cruds)
	// Main upload endpoint - uses operation query param for different actions
	defaultRouter.POST("/asset/:typename/:resource_id/:columnname/upload", assetUploadHandler)
	defaultRouter.GET("/asset/:typename/:resource_id/:columnname/upload", assetUploadHandler)    // For get_part_url operation
	defaultRouter.DELETE("/asset/:typename/:resource_id/:columnname/upload", assetUploadHandler) // For abort operation

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
		InitializeGraphqlResource(initConfig, cruds, defaultRouter)
	}

	defaultRouter.GET("/jsmodel/:typename", jsModelHandler)
	defaultRouter.GET("/aggregate/:typename", statsHandler)
	defaultRouter.POST("/aggregate/:typename", statsHandler)
	defaultRouter.GET("/meta", metaHandler)
	defaultRouter.GET("/openapi.yaml", blueprintHandler)
	defaultRouter.OPTIONS("/jsmodel/:typename", jsModelHandler)
	defaultRouter.OPTIONS("/openapi.yaml", blueprintHandler)

	actionHandler := resource.CreatePostActionHandler(&initConfig, cruds, actionPerformers)
	defaultRouter.POST("/action/:typename/:actionName", actionHandler)
	defaultRouter.PATCH("/action/:typename/:actionName", actionHandler)
	defaultRouter.PUT("/action/:typename/:actionName", actionHandler)
	defaultRouter.DELETE("/action/:typename/:actionName", actionHandler)
	defaultRouter.GET("/action/:typename/:actionName", actionHandler)

	defaultRouter.POST("/track/start/:stateMachineId", CreateEventStartHandler(fsmManager, cruds, db))
	defaultRouter.POST("/track/event/:typename/:objectStateId/:eventName", CreateEventHandler(&initConfig, fsmManager, cruds, db))

	websocketServer := websockets.NewServer("/live", &dtopicMap, cruds)

	if enableYjs == "true" {
		err = InitializeYjsResources(documentProvider, defaultRouter, cruds, dtopicMap)
	}

	go func() {
		websocketServer.Listen(defaultRouter)
	}()

	SetupNoRouteRouter(boxRoot, defaultRouter)

	//defaultRouter.Run(fmt.Sprintf(":%v", *port))
	CleanUpConfigFiles()

	trigger.On("clean_up_uploaded_files", func() {
		CleanUpConfigFiles()
	})
	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [906]")
	}
	adminEmail := cruds[resource.USER_ACCOUNT_TABLE_NAME].GetAdminEmailId(transaction)
	transaction.Rollback()
	if adminEmail == "" {
		adminEmail = "No one"
	}
	log.Printf("Our admin is [%v]", adminEmail)

	return hostSwitch, mailDaemon, TaskScheduler, configStore, certificateManager, ftpServer, imapServer, olricDb

}
