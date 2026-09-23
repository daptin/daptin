package server

import (
	"context"
	"fmt"
	"github.com/artpar/api2go-adapter/gingonic"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/action_provider"
	"github.com/daptin/daptin/server/actions"
	"github.com/daptin/daptin/server/dbresourceinterface"
	"github.com/daptin/daptin/server/fsm"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/llm"
	"github.com/daptin/daptin/server/subsite"
	"github.com/daptin/daptin/server/table_info"
	"github.com/daptin/daptin/server/task"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/golang-lru"
	"os"
	"time"

	"github.com/artpar/api2go/v2"
	server2 "github.com/artpar/ftpserver/server"
	"github.com/artpar/go-imap/server"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/stats"
	"github.com/artpar/ydb"
	"github.com/aviddiviner/gin-limit"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/websockets"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"net/http"

	log "github.com/sirupsen/logrus"
)

var Stats = stats.New()

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

func NewRuntime(ctx context.Context, boxRoot http.FileSystem, db database.DatabaseConnection, localStoragePath string,
	olricDb *olric.EmbeddedClient, localOlricAddr string) (*Runtime, error) {

	PrintCliBanner()
	runtimeErrors := make(chan error, 8)

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
	if err := resource.RemoveObsoleteSystemActions(db); err != nil {
		log.Warnf("Failed to remove obsolete system actions: %v", err)
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
		gzipMiddleware := gzip.Gzip(gzip.DefaultCompression,
			gzip.WithExcludedExtensions([]string{".pdf", ".mp4", ".jpg", ".png", ".wav", ".gif", ".mp3"}),
			gzip.WithExcludedPaths([]string{"/asset/", "/live"}))
		defaultRouter.Use(func(c *gin.Context) {
			if !acceptsGzip(c.GetHeader("Accept-Encoding")) {
				c.Next()
				return
			}
			gzipMiddleware(c)
		})
	}

	defaultRouter.Use(func() gin.HandlerFunc {
		return func(c *gin.Context) {
			startedAt, recorder := Stats.Begin(c.Writer) // NOSONAR -- This call starts request timing and does not open a transaction.
			c.Next()
			Stats.End(startedAt, stats.WithRecorder(recorder))
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
		rateConfigJson = defaultRateConfigJSON
		err = configStore.SetConfigValueFor("limit.rate", rateConfigJson, "backend", transaction)
		resource.CheckErr(err, "Failed to store limit.rate default value in db")
	}

	rateConfig, rateConfigErr := ParseRateConfig(rateConfigJson)
	if rateConfigErr != nil {
		log.Errorf("Invalid backend limit.rate configuration; using defaults without overwriting the stored value: %v", rateConfigErr)
		rateConfig = defaultRateConfig
	}
	corsConfigJSON, corsConfigErr := configStore.GetConfigValueFor("cors.config", "backend", transaction)
	if corsConfigErr != nil {
		corsConfigJSON = defaultCorsConfigJSON
		if err := configStore.SetConfigValueFor("cors.config", corsConfigJSON, "backend", transaction); err != nil {
			resource.CheckErr(err, "Failed to store cors.config default value")
		}
	}
	corsConfig, corsParseErr := ParseCorsConfig(corsConfigJSON)
	if corsParseErr != nil {
		log.Errorf("Invalid backend cors.config; disabling cross-origin access without overwriting the stored value: %v", corsParseErr)
		corsConfig = defaultCorsConfig
	}
	_ = transaction.Commit()

	var rateLimiter = CreateRateLimiterMiddleware(rateConfig, olricDb)

	defaultRouter.Use(NewCorsMiddleware(corsConfig).CorsMiddlewareFunc)
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
	graphqlRequestBodyLimit := defaultGraphQLRequestBodyLimit
	configuredGraphQLRequestBodyLimit, err := configStore.GetConfigValueFor("graphql.max_request_bytes", "backend", transaction)
	if err != nil {
		err = configStore.SetConfigValueFor("graphql.max_request_bytes", graphqlRequestBodyLimit, "backend", transaction)
		resource.CheckErr(err, "Failed to set a default value for graphql.max_request_bytes")
	} else {
		parsedLimit, parseErr := parseGraphQLRequestBodyLimit(configuredGraphQLRequestBodyLimit)
		if parseErr != nil {
			log.Errorf("Invalid backend graphql.max_request_bytes configuration; using the 10 MiB default: %v", parseErr)
		} else {
			graphqlRequestBodyLimit = parsedLimit
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
	defaultRouter.Use(meteringPayloadMiddleware(&cruds, graphqlRequestBodyLimit))
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

	var store ydb.Store

	if enableYjs == "true" {
		store = CreateYjsStore(configStore, transaction, localStoragePath, cruds)
	} else {
		log.Infof("YJS endpoint is disabled in config")
	}
	transaction.Commit()

	ms := BuildMiddlewareSet(&initConfig, &cruds, &dtopicMap)
	log.Tracef("Created middleware set")
	AddResourcesToApi2Go(api, initConfig.Tables, db, &ms, configStore, olricDb, cruds)
	log.Tracef("Added ResourcesToApi2Go")
	tablesPubSub, err := cruds["world"].OlricDb.NewPubSub(olric.ToAddress(localOlricAddr))
	if err != nil {
		return nil, fmt.Errorf("failed to create Olric topic: %w", err)
	}
	log.Infof("Created PubSub pinned to local Olric: %s", localOlricAddr)

	tableTopicSubscription := tablesPubSub.Subscribe(ctx, "members")
	go func(topicSubscription *redis.PubSub) {
		channel := topicSubscription.Channel()
		for msg := range channel {
			log.Infof("[438] Received message on [%s]: [%v]", msg.Channel, msg.String())
		}
	}(tableTopicSubscription)

	for key, val := range cruds {
		dtopicMap[key] = tablesPubSub
		crudsInterface[key] = val
		val.PubSub = tablesPubSub
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
			return nil, fmt.Errorf("failed to start SMTP server: %w", err)
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
		return nil, fmt.Errorf("begin IMAP configuration transaction: %w", err)
	}

	enableImapServer, err := configStore.GetConfigValueFor("imap.enabled", "backend", transaction)
	if err == nil && enableImapServer == "true" {
		imapServer, err = InitializeImapResources(configStore, transaction, cruds, imapServer, certificateManager, runtimeErrors)
		if err != nil {
			_ = transaction.Rollback()
			imapServer = nil
			log.WithError(err).Error("IMAP is disabled for this runtime; correct its backend configuration and restart Daptin")
		}
	} else {
		if err != nil {
			err = configStore.SetConfigValueFor("imap.enabled", "false", "backend", transaction)
			if err != nil {
				_ = transaction.Rollback()
				return nil, fmt.Errorf("store default imap.enabled configuration: %w", err)
			}
		}
	}
	if err == nil {
		if err = transaction.Commit(); err != nil {
			return nil, fmt.Errorf("commit IMAP configuration transaction: %w", err)
		}
	}
	log.Tracef("Processed imps")

	transaction, err = db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("begin CalDAV configuration transaction: %w", err)
	}
	enableCaldav, err := configStore.GetConfigValueFor("caldav.enable", "backend", transaction)
	if err != nil {
		enableCaldav = "false"
		err = configStore.SetConfigValueFor("caldav.enable", enableCaldav, "backend", transaction)
		resource.CheckErr(err, "Failed to store caldav.enable in _config")
	}
	log.Printf("[CALDAV INIT] enableCaldav read from config: '%s', err: %v", enableCaldav, err)
	if err = transaction.Commit(); err != nil {
		return nil, fmt.Errorf("commit CalDAV configuration transaction: %w", err)
	}

	taskScheduler, err := resource.NewTaskScheduler(cruds)
	if err != nil {
		return nil, fmt.Errorf("initialize task scheduler: %w", err)
	}

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

	adminTaskUserReferenceId, taskUserErr := resolveTaskUserReference(transaction)
	if taskUserErr != nil {
		log.WithError(taskUserErr).Warn("scheduled system tasks have no administrator identity")
	}
	hostSwitch, subsiteCacheFolders := CreateSubSites(ctx, &initConfig, transaction, cruds, authMiddleware, rateConfig,
		maxConnections, olricDb, taskScheduler, adminTaskUserReferenceId, enableGzip == "true")
	transaction.Commit()

	log.Printf("[CALDAV INIT] Checking if CalDAV should be enabled: enableCaldav='%s'", enableCaldav)
	if enableCaldav == "true" {
		log.Printf("[CALDAV INIT] Initializing CalDAV resources...")
		InitializeCaldavResources(authMiddleware, cruds, defaultRouter)
		log.Printf("[CALDAV INIT] CalDAV initialization complete!")
	} else {
		log.Printf("[CALDAV INIT] CalDAV NOT enabled (value: '%s')", enableCaldav)
	}
	log.Tracef("Completed process caldav")

	for k := range cruds {
		cruds[k].SetSubsitesFolderCache(subsiteCacheFolders)
	}

	hostSwitch.HandlerMap["api"] = defaultRouter
	hostSwitch.HandlerMap["dashboard"] = defaultRouter

	llmGateway, err := llm.NewGateway(ctx, cruds, olricDb)
	if err != nil {
		return nil, fmt.Errorf("initialize LLM gateway: %w", err)
	}
	llmGatewayTransferred := false
	defer func() {
		if !llmGatewayTransferred {
			drainContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if drainErr := llmGateway.Drain(drainContext); drainErr != nil {
				log.WithError(drainErr).Error("failed to drain LLM gateway after runtime initialization failure")
			}
		}
	}()

	integrationRuntimeInstanceID := uuid.NewString()
	actionPerformers := action_provider.GetActionPerformers(&initConfig, configStore, cruds, mailDaemon, hostSwitch, certificateManager, integrationRuntimeInstanceID, llmGateway)
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
	integrationSubscription := actions.StartIntegrationRuntimeInstallSubscriber(ctx, cruds, configStore, integrationRuntimeInstanceID)

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [634]")
	}

	// Set the Olric client for template cache
	_ = subsite.CreateTemplateHooks(transaction, crudsInterface, hostSwitch, olricDb, enableGzip == "true")
	_ = transaction.Commit()

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [642]")
	}

	err = taskScheduler.AddTask(task.Task{
		EntityName:        "mail_server",
		ActionName:        "sync_mail_servers",
		Attributes:        map[string]interface{}{},
		AsUserReferenceId: adminTaskUserReferenceId,
		Schedule:          "@every 1h",
	})
	resource.CheckErr(err, "Failed to register mail server sync task")

	err = taskScheduler.AddTask(task.Task{
		EntityName:        "outbox",
		ActionName:        "process_outbox",
		Attributes:        map[string]interface{}{},
		AsUserReferenceId: adminTaskUserReferenceId,
		Schedule:          "@every 5m",
	})
	resource.CheckErr(err, "Failed to register outbox processing task")

	if adminTaskUserReferenceId == daptinid.NullReferenceId {
		_ = transaction.Rollback()
		log.Warn("data exchange processor task was not persisted because no administrator identity exists")
	} else {
		adminTaskUser, _, err := cruds[resource.USER_ACCOUNT_TABLE_NAME].GetSingleRowByReferenceIdWithTransaction(
			resource.USER_ACCOUNT_TABLE_NAME, adminTaskUserReferenceId, nil, transaction)
		if err != nil {
			_ = transaction.Rollback()
			return nil, fmt.Errorf("load data exchange task administrator: %w", err)
		}
		adminTaskUserEmail := resource.StringOrEmpty(adminTaskUser["email"])
		if adminTaskUserEmail == "" {
			_ = transaction.Rollback()
			return nil, fmt.Errorf("data exchange task administrator has no email")
		}
		exchangeTaskConfig := resource.CmsConfig{Tasks: []task.Task{{
			Name:        "process-data-exchange-executions",
			EntityName:  resource.EXCHANGE_RUN_TABLE_NAME,
			ActionName:  "process_data_exchange_executions",
			Attributes:  map[string]interface{}{},
			AsUserEmail: adminTaskUserEmail,
			Schedule:    "@every 1s",
			Active:      true,
			JobType:     "action",
		}}}
		err = resource.UpdateTasksData(&exchangeTaskConfig, transaction)
		if err != nil {
			_ = transaction.Rollback()
			return nil, fmt.Errorf("persist data exchange execution processing task: %w", err)
		}
		if err := transaction.Commit(); err != nil {
			return nil, fmt.Errorf("commit data exchange execution processing task: %w", err)
		}
	}

	taskScheduler.LoadPersistedTasks()

	transaction = db.MustBegin()
	assetColumnFolders := CreateAssetColumnSync(crudsInterface, transaction, taskScheduler, adminTaskUserReferenceId)
	transaction.Commit()
	for k := range cruds {
		cruds[k].AssetFolderCache = assetColumnFolders
	}
	llmGateway.StartBatchProcessing(ctx)
	taskScheduler.Start()

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
	var ftpConnections *ConnectionTracker
	if enableFtp == "true" {
		ftpServer, ftpConnections, err = InitializeFtpResources(ctx, configStore, transaction, cruds, crudsInterface, certificateManager, runtimeErrors)
		if err != nil {
			_ = transaction.Rollback()
			return nil, fmt.Errorf("failed to start FTP server: %w", err)
		}
	}

	RegisterLLMEndpoints(defaultRouter, llmGateway)

	resource.InitialiseColumnManager()
	jsModelHandler := CreateJsModelHandler(&initConfig, cruds, transaction)
	transaction.Commit()
	blueprintHandler := CreateApiBlueprintHandler(&initConfig, cruds)
	statsHandler := CreateStatsHandler(&initConfig, cruds)
	metaHandler := CreateMetaHandler(&initConfig)

	dbAssetHandler := CreateDbAssetHandler(cruds, olricDb)
	defaultRouter.GET("/asset/:typename/:resource_id/:columnname", dbAssetHandler)

	// Asset upload endpoints - properly organized
	assetUploadHandler := AssetUploadHandler(cruds)
	// Main upload endpoint - uses operation query param for different actions
	defaultRouter.POST("/asset/:typename/:resource_id/:columnname/upload", assetUploadHandler)
	defaultRouter.GET("/asset/:typename/:resource_id/:columnname/upload", assetUploadHandler)    // For get_part_url operation
	defaultRouter.DELETE("/asset/:typename/:resource_id/:columnname/upload", assetUploadHandler) // Delete attached asset

	defaultRouter.GET("/feed/:feedname", feedHandler)

	configHandler := CreateConfigHandler(&initConfig, cruds, configStore)
	defaultRouter.GET("/_config/:end/:key", configHandler)
	defaultRouter.GET("/_config", configHandler)
	defaultRouter.POST("/_config/:end/:key", configHandler)
	defaultRouter.PATCH("/_config/:end/:key", configHandler)
	defaultRouter.PUT("/_config/:end/:key", configHandler)
	defaultRouter.DELETE("/_config/:end/:key", configHandler)

	InitializeOAuthResources(cruds, configStore, defaultRouter)

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
	defaultRouter.POST("/action/:typename/*actionName", actionHandler)
	defaultRouter.PATCH("/action/:typename/*actionName", actionHandler)
	defaultRouter.PUT("/action/:typename/*actionName", actionHandler)
	defaultRouter.DELETE("/action/:typename/*actionName", actionHandler)
	defaultRouter.GET("/action/:typename/*actionName", actionHandler)
	defaultRouter.GET("/integration/:providerName/openapi.yaml", CreateIntegrationOpenAPIHandler(cruds))
	defaultRouter.GET("/integration/:providerName/operations", CreateIntegrationOperationsHandler(cruds))
	defaultRouter.GET("/integration/:providerName/operations/*operationName", CreateIntegrationOperationDescribeHandler(cruds))
	defaultRouter.POST("/integration/:providerName/*operationName", CreateIntegrationOperationHandler(cruds))

	defaultRouter.POST("/track/start/:stateMachineId", CreateEventStartHandler(fsmManager, cruds, db))
	defaultRouter.POST("/track/event/:typename/:objectStateId/:eventName", CreateEventHandler(&initConfig, fsmManager, cruds, db))

	websocketServer := websockets.NewServer("/live", &dtopicMap, cruds, tablesPubSub)

	var yjsRuntime *YjsRuntime
	if enableYjs == "true" {
		yjsRuntime, err = InitializeYjsResources(ctx, store, defaultRouter, cruds)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize YJS: %w", err)
		}
	}

	SetupNoRouteRouter(boxRoot, defaultRouter)
	websocketServer.Register(defaultRouter)
	go websocketServer.Listen()

	//defaultRouter.Run(fmt.Sprintf(":%v", *port))
	resource.CleanUpConfigFiles()
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

	llmGatewayTransferred = true
	return &Runtime{
		Handler:                 &hostSwitch,
		ConfigStore:             configStore,
		CertificateManager:      certificateManager,
		mailDaemon:              mailDaemon,
		scheduler:               taskScheduler,
		ftpServer:               ftpServer,
		ftpConnections:          ftpConnections,
		imapServer:              imapServer,
		websocketServer:         websocketServer,
		yjs:                     yjsRuntime,
		llmGateway:              llmGateway,
		tableSubscription:       tableTopicSubscription,
		integrationSubscription: integrationSubscription,
		errors:                  runtimeErrors,
	}, nil

}

func resolveTaskUserReference(transaction *sqlx.Tx) (daptinid.DaptinReferenceId, error) {
	administratorReferences := resource.GetUserMembersByGroupNameWithTransaction("administrators", transaction)
	if len(administratorReferences) == 0 {
		return daptinid.NullReferenceId, fmt.Errorf("administrators group has no members")
	}
	return administratorReferences[0], nil
}
