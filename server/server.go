package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/artpar/api2go-adapter/gingonic"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/action_provider"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/cloud_store"
	"github.com/daptin/daptin/server/dbresourceinterface"
	"github.com/daptin/daptin/server/fsm"
	"github.com/daptin/daptin/server/hostswitch"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/daptin/daptin/server/table_info"
	"github.com/daptin/daptin/server/task"
	"github.com/daptin/daptin/server/task_scheduler"
	"github.com/emersion/go-webdav"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/golang-lru"
	"github.com/jmoiron/sqlx"
	"github.com/sadlil/go-trigger"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	//"sync"
	"time"

	"github.com/artpar/api2go"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-imap-idle"
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
	"github.com/icrowley/fake"
	rateLimit "github.com/yangxikun/gin-limit-by-key"
	"golang.org/x/time/rate"
	//"github.com/gin-gonic/gin"
	"io"
	"net/http"

	graphqlhandler "github.com/graphql-go/handler"
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

// PathExistsAndIsFolder checks if a path exists and is a folder
func PathExistsAndIsFolder(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false // Path does not exist
	}
	if err != nil {
		return false // Other errors
	}
	return info.IsDir() // Check if it's a directory
}

// DiskFileCache represents a cached file entry
type DiskFileCache struct {
	Data         []byte
	ContentType  string
	LastModified time.Time
	ETag         string
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

	fmt.Print(`                                                                           
                              
===================================
===================================

 ____    _    ____ _____ ___ _   _ 
|  _ \  / \  |  _ |_   _|_ _| \ | |
| | | |/ _ \ | |_) || |  | ||  \| |
| |_| / ___ \|  __/ | |  | || |\  |
|____/_/   \_|_|    |_| |___|_| \_|

===================================                                   
===================================


`)

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
		initialiseResources(&initConfig, db)
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

	defaultRouter.Use(NewCorsMiddleware().CorsMiddlewareFunc)
	defaultRouter.Use(limit.MaxAllowed(maxConnections))
	defaultRouter.Use(rateLimit.NewRateLimiter(func(c *gin.Context) string {
		requestPath := strings.Split(c.Request.RequestURI, "?")[0]
		return c.ClientIP() + requestPath // limit rate by client ip + url
	}, func(c *gin.Context) (*rate.Limiter, time.Duration) {
		requestPath := strings.Split(c.Request.RequestURI, "?")[0]
		ratePerSecond, ok := rateConfig.limits[requestPath]
		if !ok {
			ratePerSecond = 500
		}
		microSecondRateGap := int(1000000 / ratePerSecond)
		return rate.NewLimiter(rate.Every(time.Duration(microSecondRateGap)*time.Microsecond),
			ratePerSecond,
		), time.Minute // limit 10 qps/clientIp and permit bursts of at most 10 tokens, and the limiter liveness time duration is 1 hour
	}, func(c *gin.Context) {
		c.AbortWithStatus(429) // handle exceed rate limit request
	}))

	defaultRouter.GET("/statistics", CreateStatisticsHandler(db))

	defaultRouter.StaticFS("/static", NewSubPathFs(boxRoot, "/static"))
	defaultRouter.StaticFS("/statics", NewSubPathFs(boxRoot, "/statics"))
	defaultRouter.StaticFS("/js", NewSubPathFs(boxRoot, "/js"))
	defaultRouter.StaticFS("/css", NewSubPathFs(boxRoot, "/css"))
	defaultRouter.StaticFS("/fonts", NewSubPathFs(boxRoot, "/fonts"))

	// Handle both favicon.ico and favicon.png with aggressive caching
	defaultRouter.GET("/favicon.:format", func(c *gin.Context) {
		format := c.Param("format")
		if format != "ico" && format != "png" {
			c.AbortWithStatus(404)
			return
		}

		// Set aggressive caching headers
		c.Header("Cache-Control", "public, max-age=31536000, immutable") // 1 year
		c.Header("Pragma", "public")

		// Try to get file from primary location
		var file http.File
		var err error
		var contentType string

		if format == "ico" {
			file, err = boxRoot.Open("static/img/favicon.ico")
			contentType = "image/x-icon"
			if err != nil {
				// Try fallback location for .ico
				file, err = boxRoot.Open("favicon.ico")
				if err != nil {
					c.AbortWithStatus(404)
					return
				}
			}
		} else { // png
			file, err = boxRoot.Open("static/img/favicon.png")
			contentType = "image/png"
			if err != nil {
				c.AbortWithStatus(404)
				return
			}
		}

		// Read file content
		fileContents, err := io.ReadAll(file)
		if err != nil {
			c.AbortWithStatus(404)
			return
		}

		// Generate ETag for better caching
		fileInfo, _ := file.Stat()
		etag := generateETag(fileContents, fileInfo.ModTime())
		c.Header("ETag", etag)

		// Check if client has this version cached
		if match := c.Request.Header.Get("If-None-Match"); match != "" && match == etag {
			c.AbortWithStatus(http.StatusNotModified) // 304
			return
		}

		// Set content type based on format
		c.Header("Content-Type", contentType)

		// Set last modified
		c.Header("Last-Modified", time.Now().UTC().Format(http.TimeFormat))

		// Write response
		_, err = c.Writer.Write(fileContents)
		resource.CheckErr(err, "Failed to write favicon."+format)
	})

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
		log.Infof("YJS endpoint is enabled in config")
		yjs_temp_directory, err := configStore.GetConfigValueFor("yjs.storage.path", "backend", transaction)
		//if err != nil {
		yjs_temp_directory = localStoragePath + "/yjs-documents"
		configStore.SetConfigValueFor("yjs.storage.path", yjs_temp_directory, "backend", transaction)
		//}

		if !PathExistsAndIsFolder(yjs_temp_directory) {
			err = os.Mkdir(yjs_temp_directory, 0777)
			if err != nil {
				resource.CheckErr(err, "Failed to create yjs storage directory")
			}
		}

		documentProvider = ydb.NewDiskDocumentProvider(yjs_temp_directory, 10000, ydb.DocumentListener{
			GetDocumentInitialContent: func(documentPath string, transaction *sqlx.Tx) []byte {
				log.Debugf("Get initial content for document: %v", documentPath)
				pathParts := strings.Split(documentPath, ".")
				typeName := pathParts[0]
				referenceId := pathParts[1]
				columnName := pathParts[2]
				if transaction == nil {
					log.Tracef("start transaction for GetDocumentInitialContent")
					transaction, err = cruds[typeName].Connection().Beginx()
					if err != nil {
						return nil
					}
					defer transaction.Rollback()
				}

				object, _, _ := cruds[typeName].GetSingleRowByReferenceIdWithTransaction(typeName,
					daptinid.DaptinReferenceId(uuid.MustParse(referenceId)), map[string]bool{
						columnName: true,
					}, transaction)
				log.Tracef("Completed NewDiskDocumentProvider GetSingleRowByReferenceIdWithTransaction")

				originalFile := object[columnName]
				if originalFile == nil {
					return []byte{}
				}
				columnValueArray := originalFile.([]map[string]interface{})

				fileContentsJson := []byte{}
				for _, file := range columnValueArray {
					if file["type"] != "x-crdt/yjs" {
						continue
					}

					fileContentsJson, _ = base64.StdEncoding.DecodeString(file["contents"].(string))

				}

				log.Debugf("Completed get initial content for document: %v", documentPath)
				return fileContentsJson
			},
			SetDocumentInitialContent: nil,
		})
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
	go func(topicScubscription *redis.PubSub) {
		channel := topicScubscription.Channel()
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
	api.UseMiddleware(func(contexter api2go.APIContexter, writer http.ResponseWriter, request *http.Request) {

	})

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
		imapListenInterface, err := configStore.GetConfigValueFor("imap.listen_interface", "backend", transaction)
		if err != nil {
			err = configStore.SetConfigValueFor("imap.listen_interface", ":1143", "backend", transaction)
			resource.CheckErr(err, "Failed to store default imap listen interface in config")
			imapListenInterface = ":1143"
		}

		hostname, err := configStore.GetConfigValueFor("hostname", "backend", transaction)
		hostname = "imap." + hostname
		imapBackend := resource.NewImapServer(cruds)

		// Create a new server
		imapServer = server.New(imapBackend)
		imapServer.Addr = imapListenInterface
		imapServer.Debug = nil
		imapServer.AllowInsecureAuth = false
		imapServer.Enable(idle.NewExtension())
		//imapServer.Debug = os.Stdout
		//imapServer.EnableAuth("CRAM-MD5", func(conn server.Conn) sasl.Server {
		//
		//	return &Crammd5{
		//		dbResource:  cruds["mail"],
		//		conn:        conn,
		//		imapBackend: imapBackend,
		//	}
		//})

		cert, err := certificateManager.GetTLSConfig(hostname, true, transaction)
		resource.CheckErr(err, "Failed to get certificate for IMAP [%v]", hostname)
		imapServer.TLSConfig = cert.TLSConfig

		log.Printf("Starting IMAP server at %s: %v", imapListenInterface, hostname)

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
		log.Tracef("Process caldav")

		//caldavStorage, err := resource.NewCaldavStorage(cruds, certificateManager)
		caldavHandler := webdav.Handler{
			FileSystem: webdav.LocalFileSystem("./storage"),
		}
		caldavHttpHandler := func(c *gin.Context) {
			ok, abort, modifiedRequest := authMiddleware.AuthCheckMiddlewareWithHttp(c.Request, c.Writer, true)
			if !ok || abort {
				c.Header("WWW-Authenticate", "Basic realm='caldav'")
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			caldavHandler.ServeHTTP(c.Writer, modifiedRequest)
		}
		defaultRouter.Handle("OPTIONS", "/caldav/*path", caldavHttpHandler)
		defaultRouter.Handle("HEAD", "/caldav/*path", caldavHttpHandler)
		defaultRouter.Handle("GET", "/caldav/*path", caldavHttpHandler)
		defaultRouter.Handle("POST", "/caldav/*path", caldavHttpHandler)
		defaultRouter.Handle("PUT", "/caldav/*path", caldavHttpHandler)
		defaultRouter.Handle("PATCH", "/caldav/*path", caldavHttpHandler)
		defaultRouter.Handle("PROPFIND", "/caldav/*path", caldavHttpHandler)
		defaultRouter.Handle("DELETE", "/caldav/*path", caldavHttpHandler)
		defaultRouter.Handle("COPY", "/caldav/*path", caldavHttpHandler)
		defaultRouter.Handle("MOVE", "/caldav/*path", caldavHttpHandler)
		defaultRouter.Handle("MKCOL", "/caldav/*path", caldavHttpHandler)
		defaultRouter.Handle("PROPPATCH", "/caldav/*path", caldavHttpHandler)

		defaultRouter.Handle("OPTIONS", "/carddav/*path", caldavHttpHandler)
		defaultRouter.Handle("HEAD", "/carddav/*path", caldavHttpHandler)
		defaultRouter.Handle("GET", "/carddav/*path", caldavHttpHandler)
		defaultRouter.Handle("POST", "/carddav/*path", caldavHttpHandler)
		defaultRouter.Handle("PUT", "/carddav/*path", caldavHttpHandler)
		defaultRouter.Handle("PATCH", "/carddav/*path", caldavHttpHandler)
		defaultRouter.Handle("PROPFIND", "/carddav/*path", caldavHttpHandler)
		defaultRouter.Handle("DELETE", "/carddav/*path", caldavHttpHandler)
		defaultRouter.Handle("COPY", "/carddav/*path", caldavHttpHandler)
		defaultRouter.Handle("MOVE", "/carddav/*path", caldavHttpHandler)
		defaultRouter.Handle("MKCOL", "/carddav/*path", caldavHttpHandler)
		defaultRouter.Handle("PROPPATCH", "/carddav/*path", caldavHttpHandler)

		//hostSwitch.HandlerMap["calendar"] = caldavHandler
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
	actionHandlerMap := actionPerformersListToMap(actionPerformers)
	for k := range cruds {
		cruds[k].ActionHandlerMap = actionHandlerMap
		cruds[k].EncryptionSecret = []byte(encryptionSecret)
	}

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [634]")
	}

	_ = subsite.CreateTemplateHooks(transaction, crudsInterface, hostSwitch)
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

		ftp_interface, err := configStore.GetConfigValueFor("ftp.listen_interface", "backend", transaction)
		if err != nil {
			ftp_interface = "0.0.0.0:2121"
			err = configStore.SetConfigValueFor("ftp.listen_interface", ftp_interface, "backend", transaction)
			resource.CheckErr(err, "Failed to store default value for ftp.listen_interface")
		}
		// ftpListener, err := net.Listen("tcp", ftp_interface)
		// resource.CheckErr(err, "Failed to create listener for FTP")
		ftpServer, err = CreateFtpServers(cruds, crudsInterface, certificateManager, ftp_interface, transaction)
		auth.CheckErr(err, "Failed to creat FTP server")
		go func() {
			log.Printf("FTP server started at %v", ftp_interface)
			err = ftpServer.ListenAndServe()
			resource.CheckErr(err, "Failed to listen at ftp interface")
		}()
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
	metaHandler := CreateMetaHandler(&initConfig)
	blueprintHandler := CreateApiBlueprintHandler(&initConfig, cruds)
	statsHandler := CreateStatsHandler(&initConfig, cruds)
	resource.InitialiseColumnManager()

	dbAssetHandler := CreateDbAssetHandler(cruds, olricDb)
	defaultRouter.GET("/asset/:typename/:resource_id/:columnname", dbAssetHandler)

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
			Schema:     graphqlSchema,
			Pretty:     true,
			Playground: true,
			GraphiQL:   true,
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

	//loader := CreateSubSiteContentHandler(&initConfig, cruds, db)
	//defaultRouter.POST("/site/content/load", loader)
	//defaultRouter.GET("/site/content/load", loader)
	//defaultRouter.POST("/site/content/store", CreateSubSiteSaveContentHandler(&initConfig, cruds, db))

	//TODO: make websockets functional at /live

	websocketServer := websockets.NewServer("/live", &dtopicMap, cruds)

	if enableYjs == "true" {
		//var sessionFetcher *YjsConnectionSessionFetcher
		//sessionFetcher = &YjsConnectionSessionFetcher{}
		var ydbInstance = ydb.InitYdb(documentProvider)

		yjsConnectionHandler := ydb.YdbWsConnectionHandler(ydbInstance)

		defaultRouter.GET("/yjs/:documentName", func(ginContext *gin.Context) {

			sessionUser := ginContext.Request.Context().Value("user")
			if sessionUser == nil {
				ginContext.AbortWithStatus(403)
			}

			log.Tracef("Handle new YJS client")
			yjsConnectionHandler(ginContext.Writer, ginContext.Request)

		})

		for typename, crud := range cruds {

			for _, columnInfo := range crud.TableInfo().Columns {
				if !BeginsWithCheck(columnInfo.ColumnType, "file.") {
					continue
				}

				path := fmt.Sprintf("/live/%v/:referenceId/%v/yjs", typename, columnInfo.ColumnName)
				log.Printf("[%v] YJS websocket endpoint for %v[%v]", path, typename, columnInfo.ColumnName)
				defaultRouter.GET(path, func(typename string, columnInfo api2go.ColumnInfo) func(ginContext *gin.Context) {

					redisPubSub := dtopicMap[typename].Subscribe(context.Background(), typename)
					go func(rps *redis.PubSub) {
						channel := rps.Channel()
						for {
							msg := <-channel
							var eventMessage resource.EventMessage
							//log.Infof("Message received: %s", msg.Payload)
							err = ProcessEventMessage(eventMessage, msg, typename, cruds, columnInfo, documentProvider)
							CheckErr(err, "Failed to process message on OlricTopic[%v]", typename)

						}
					}(redisPubSub)

					return func(ginContext *gin.Context) {

						sessionUser := ginContext.Request.Context().Value("user")
						if sessionUser == nil {
							ginContext.AbortWithStatus(403)
							return
						}
						user := sessionUser.(*auth.SessionUser)

						referenceId := ginContext.Param("referenceId")

						tx, err := cruds[typename].Connection().Beginx()
						if err != nil {
							resource.CheckErr(err, "Failed to begin transaction [840]")
							return
						}

						object, _, err := cruds[typename].GetSingleRowByReferenceIdWithTransaction(typename,
							daptinid.DaptinReferenceId(uuid.MustParse(referenceId)), nil, tx)
						tx.Rollback()
						if err != nil {
							ginContext.AbortWithStatus(404)
							return
						}

						tx, err = cruds[typename].Connection().Beginx()
						objectPermission := cruds[typename].GetRowPermission(object, tx)
						tx.Rollback()
						if err != nil {
							ginContext.AbortWithStatus(500)
							return
						}

						if !objectPermission.CanUpdate(user.UserReferenceId, user.Groups, cruds[typename].AdministratorGroupId) {
							ginContext.AbortWithStatus(401)
							return
						}

						roomName := fmt.Sprintf("%v%v%v%v%v", typename, ".", referenceId, ".", columnInfo.ColumnName)
						ginContext.Request = ginContext.Request.WithContext(context.WithValue(ginContext.Request.Context(), "roomname", roomName))

						yjsConnectionHandler(ginContext.Writer, ginContext.Request)

					}
				}(typename, columnInfo))

			}

		}

	}

	go func() {
		websocketServer.Listen(defaultRouter)
	}()

	setupNoRouteRouter(boxRoot, defaultRouter)

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

func ProcessEventMessage(eventMessage resource.EventMessage, msg *redis.Message, typename string, cruds map[string]*resource.DbResource, columnInfo api2go.ColumnInfo, documentProvider ydb.DocumentProvider) error {
	var err error
	err = eventMessage.UnmarshalBinary([]byte(msg.Payload))
	if err != nil {
		resource.CheckErr(err, "Failed to read message on channel "+typename)
		return nil
	}
	if eventMessage.EventType == "update" && eventMessage.ObjectType == typename {
		eventDataMap := make(map[string]interface{})
		err := json.Unmarshal(eventMessage.EventData, &eventDataMap)
		resource.CheckErr(err, "Failed to unmarshal message ["+eventMessage.ObjectType+"]")
		referenceId := uuid.MustParse(eventDataMap["reference_id"].(string))

		transaction1, err := cruds[typename].Connection().Beginx()
		defer transaction1.Rollback()
		if err != nil {
			resource.CheckErr(err, "Failed to begin transaction [788]")
			return nil
		}

		object, _, _ := cruds[typename].GetSingleRowByReferenceIdWithTransaction(typename, daptinid.DaptinReferenceId(referenceId), map[string]bool{
			columnInfo.ColumnName: true,
		}, transaction1)
		log.Tracef("Completed dtopicMapListener GetSingleRowByReferenceIdWithTransaction")

		colValue := object[columnInfo.ColumnName]
		if colValue == nil {
			return nil
		}
		columnValueArray, ok := colValue.([]map[string]interface{})
		if !ok {
			log.Warnf("value is not of type array - %v", colValue)
			return nil
		}

		fileContentsJson := []byte{}
		for _, file := range columnValueArray {
			if file["type"] != "x-crdt/yjs" {
				continue
			}
			fileContentsJson, _ = base64.StdEncoding.DecodeString(file["contents"].(string))
		}

		documentName := fmt.Sprintf("%v.%v.%v", typename, referenceId, columnInfo.ColumnName)
		document := documentProvider.GetDocument(ydb.YjsRoomName(documentName), transaction1)
		if document != nil && len(fileContentsJson) > 0 {
			document.SetInitialContent(fileContentsJson)
		}

	}
	return err
}

func setupNoRouteRouter(boxRoot http.FileSystem, defaultRouter *gin.Engine) {

	indexFile, err := boxRoot.Open("index.html")

	resource.CheckErr(err, "Failed to open index.html file from dashboard directory %v")

	var indexFileContents = []byte("")
	if indexFile != nil && err == nil {
		indexFileContents, err = io.ReadAll(indexFile)
	}
	defaultRouter.GET("", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=60") // Short cache time for index.html
		c.Data(http.StatusOK, "text/html; charset=UTF-8", indexFileContents)
	})

	// Add cache middleware
	defaultRouter.Use(func(c *gin.Context) {
		// Skip non-GET requests
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		c.Next()
	})

	defaultRouter.NoRoute(func(c *gin.Context) {
		filePath := strings.TrimLeft(c.Request.URL.Path, "/")

		// Check if we have the file in our cache first
		if cached, found := diskFileCache.Get(filePath); found {
			cachedFile := cached.(*DiskFileCache)

			// Handle conditional requests
			ifModifiedSince := c.GetHeader("If-Modified-Since")
			ifNoneMatch := c.GetHeader("If-None-Match")

			// Check ETag first
			if ifNoneMatch != "" && ifNoneMatch == cachedFile.ETag {
				c.Status(http.StatusNotModified)
				return
			}

			// Then check Last-Modified
			if ifModifiedSince != "" {
				ifModifiedSinceTime, err := http.ParseTime(ifModifiedSince)
				if err == nil && !cachedFile.LastModified.After(ifModifiedSinceTime) {
					c.Status(http.StatusNotModified)
					return
				}
			}

			// Set cache headers
			setClientCacheHeaders(c, cachedFile)

			// Serve from cache
			c.Data(http.StatusOK, cachedFile.ContentType, cachedFile.Data)
			return
		}

		// File not in cache, try to open it
		file, err := boxRoot.Open(filePath)
		if err == nil && file != nil {
			defer file.Close()

			// For file system, get stats to determine last modified time
			stat, statErr := file.(interface{ Stat() (os.FileInfo, error) }).Stat()
			if statErr != nil {
				log.Printf("Error getting file stats: %v", statErr)
				c.FileFromFS(filePath, boxRoot)
				return
			}

			// Don't cache large files
			if stat.Size() > maxFileSizeToCache {
				// Still set client caching headers even if we don't cache it server-side
				setClientCacheHeadersForFile(c, stat.ModTime(), generateETagWithData(filePath, stat.ModTime(), stat.Size()))
				c.FileFromFS(filePath, boxRoot)
				return
			}

			// Read the file content
			content := make([]byte, stat.Size())
			_, readErr := file.Read(content)
			if readErr != nil {
				log.Printf("Error reading file: %v", readErr)
				c.FileFromFS(filePath, boxRoot)
				return
			}

			// Determine content type
			contentType := getContentType(filePath)
			lastModified := stat.ModTime()
			etag := generateETagWithData(filePath, lastModified, stat.Size())

			// Create cache entry
			cacheEntry := &DiskFileCache{
				Data:         content,
				ContentType:  contentType,
				LastModified: lastModified,
				ETag:         etag,
			}

			// Add to cache
			diskFileCache.Add(filePath, cacheEntry)

			// Set client cache headers
			setClientCacheHeaders(c, cacheEntry)

			// Serve the file
			c.Data(http.StatusOK, contentType, content)
			return
		}

		// Fallback to serving index.html
		if len(indexFileContents) > 0 {
			// Set minimal caching for index.html
			c.Header("Cache-Control", "public, max-age=60") // Short cache time for index.html
			c.Data(http.StatusOK, "text/html; charset=UTF-8", indexFileContents)
		}
	})
}

// Set HTTP cache headers based on the cached file
func setClientCacheHeaders(c *gin.Context, cachedFile *DiskFileCache) {
	// Set ETag
	c.Header("ETag", cachedFile.ETag)

	// Set Last-Modified
	c.Header("Last-Modified", cachedFile.LastModified.UTC().Format(http.TimeFormat))

	// Set Cache-Control with aggressive but sane settings
	c.Header("Cache-Control", fmt.Sprintf(
		"public, max-age=%d, stale-while-revalidate=%d, stale-if-error=%d",
		cacheMaxAge, cacheStaleRevalidate, cacheStaleIfError))

	// Add Expires header as a fallback for older clients
	expiresTime := time.Now().Add(time.Duration(cacheMaxAge) * time.Second)
	c.Header("Expires", expiresTime.UTC().Format(http.TimeFormat))

	// Set Content-Type
	c.Header("Content-Type", cachedFile.ContentType)
}

// Set HTTP cache headers for a file that isn't cached server-side
func setClientCacheHeadersForFile(c *gin.Context, lastModified time.Time, etag string) {
	c.Header("ETag", etag)
	c.Header("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	c.Header("Cache-Control", fmt.Sprintf(
		"public, max-age=%d, stale-while-revalidate=%d, stale-if-error=%d",
		cacheMaxAge, cacheStaleRevalidate, cacheStaleIfError))
	expiresTime := time.Now().Add(time.Duration(cacheMaxAge) * time.Second)
	c.Header("Expires", expiresTime.UTC().Format(http.TimeFormat))
}

// Generate ETag based on file path, modification time, and size
func generateETagWithData(path string, modTime time.Time, size int64) string {
	etag := fmt.Sprintf("\"%x-%x-%x\"", size, modTime.UnixNano(), hash(path))
	return etag
}

// Simple hash function for ETag generation
func hash(s string) uint32 {
	h := uint32(0)
	for i := 0; i < len(s); i++ {
		h = h*31 + uint32(s[i])
	}
	return h
}

// Get content type based on file extension
func getContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=UTF-8"
	case ".css":
		return "text/css; charset=UTF-8"
	case ".js":
		return "application/javascript; charset=UTF-8"
	case ".json":
		return "application/json; charset=UTF-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain; charset=UTF-8"
	case ".xml":
		return "application/xml; charset=UTF-8"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	default:
		return "application/octet-stream"
	}
}

// loadIndexFile loads the index.html file into memory
func loadIndexFile(boxRoot http.FileSystem) error {
	file, err := boxRoot.Open("index.html")
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.(interface{ Stat() (os.FileInfo, error) }).Stat()
	if err != nil {
		return err
	}

	indexFileContents = make([]byte, stat.Size())
	_, err = file.Read(indexFileContents)
	return err
}

func CreateFtpServers(resources map[string]*resource.DbResource, resourcesInterfaces map[string]dbresourceinterface.DbResourceInterface, certManager *resource.CertificateManager, ftp_interface string, transaction *sqlx.Tx) (*server2.FtpServer, error) {

	subsites, err := subsite.GetAllSites(resourcesInterfaces["site"], transaction)
	if err != nil {
		return nil, err
	}
	cloudStores, err := cloud_store.GetAllCloudStores(resourcesInterfaces["cloud_store"], transaction)

	if err != nil {
		return nil, err
	}
	cloudStoreMap := make(map[uuid.UUID]rootpojo.CloudStore)
	for _, cloudStore := range cloudStores {
		re, _ := uuid.FromBytes(cloudStore.ReferenceId[:])
		cloudStoreMap[re] = cloudStore
	}
	var driver *DaptinFtpDriver

	sites := make([]SubSiteAssetCache, 0)
	for _, ftpServer := range subsites {

		if !ftpServer.FtpEnabled {
			continue
		}

		assetCacheFolder, ok := resourcesInterfaces["site"].SubsiteFolderCache(ftpServer.ReferenceId)
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
	subsite.SubSite
	*assetcachepojo.AssetFolderCache
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

	//resource.CreateRelations(initConfig, db)
	//resource.CheckErr(errc, "Failed to commit transaction after creating relations")

	transaction, err := db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [1017]")
		return
	}

	if transaction != nil {
		resource.CreateUniqueConstraints(initConfig, transaction)
		errc = transaction.Commit()
		resource.CheckErr(errc, "Failed to commit transaction after creating unique constrains")
	}

	resource.CreateIndexes(initConfig, db)

	var errb error
	transaction, err = db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction [1031]")

	if transaction != nil {
		errb = resource.UpdateWorldTable(initConfig, transaction)
		resource.CheckErr(errb, "Failed to update world tables")
		errc := transaction.Commit()
		resource.CheckErr(errc, "Failed to commit transaction after updating world tables")
	}

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [1042]")
		return
	}

	resource.UpdateExchanges(initConfig, transaction)
	//go func() {
	resource.UpdateStateMachineDescriptions(initConfig, transaction)
	resource.UpdateStreams(initConfig, transaction)
	//resource.UpdateMarketplaces(initConfig, db)
	err = resource.UpdateTasksData(initConfig, transaction)
	resource.CheckErr(err, "[870] Failed to update cron jobs")
	err = resource.UpdateActionTable(initConfig, transaction)
	resource.CheckErr(err, "Failed to update action table")
	if err == nil {
		transaction.Commit()
	} else {
		transaction.Rollback()
	}
	//}()

}

func actionPerformersListToMap(interfaces []actionresponse.ActionPerformerInterface) map[string]actionresponse.ActionPerformerInterface {
	m := make(map[string]actionresponse.ActionPerformerInterface)

	for _, api := range interfaces {
		if api == nil {
			continue
		}
		m[api.Name()] = api
	}
	return m
}

func MergeTables(existingTables []table_info.TableInfo, initConfigTables []table_info.TableInfo) []table_info.TableInfo {
	allTables := make([]table_info.TableInfo, 0)
	existingTablesMap := make(map[string]bool)

	newTableMap := make(map[string]table_info.TableInfo)
	for _, newTable := range initConfigTables {
		newTableMap[newTable.TableName] = newTable
	}

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
			log.Infof("Table from initial configuration:          %-20s", existableTable.TableName)
			tableBeingModified := initConfigTables[indexBeingModified]

			if len(tableBeingModified.Columns) > 0 {

				for _, newColumnDef := range tableBeingModified.Columns {
					columnAlreadyExist := false
					colIndex := -1
					for i, existingColumn := range existableTable.Columns {
						//log.Printf("Table column old/new [%v][%v] == [%v][%v] @ %v", tableBeingModified.TableName, newColumnDef.Name, existableTable.TableName, existingColumn.Name, i)
						if existingColumn.ColumnName == newColumnDef.ColumnName {
							columnAlreadyExist = true
							colIndex = i
							break
						}
					}
					//log.Printf("Decide for table column [%v][%v] @ index: %v [%v]", tableBeingModified.TableName, newColumnDef.Name, colIndex, columnAlreadyExist)
					if columnAlreadyExist {
						//log.Printf("Modifying existing columns[%v][%v] is not supported at present. not sure what would break. and alter query isnt being run currently.", existableTable.Columns[colIndex], newColumnDef);

						existableTable.Columns[colIndex].DefaultValue = newColumnDef.DefaultValue
						existableTable.Columns[colIndex].ExcludeFromApi = newColumnDef.ExcludeFromApi
						existableTable.Columns[colIndex].IsIndexed = newColumnDef.IsIndexed
						existableTable.Columns[colIndex].IsNullable = newColumnDef.IsNullable
						existableTable.Columns[colIndex].IsUnique = newColumnDef.IsUnique
						existableTable.Columns[colIndex].ColumnType = newColumnDef.ColumnType
						existableTable.Columns[colIndex].Options = newColumnDef.Options
						existableTable.Columns[colIndex].DataType = newColumnDef.DataType
						existableTable.Columns[colIndex].ColumnDescription = newColumnDef.ColumnDescription
						existableTable.Columns[colIndex].ForeignKeyData = newColumnDef.ForeignKeyData
						existableTable.Columns[colIndex].IsForeignKey = newColumnDef.IsForeignKey
						existableTable.Columns[colIndex].IsPrimaryKey = newColumnDef.IsPrimaryKey

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
			existableTable.DefaultRelations = tableBeingModified.DefaultRelations
			existableTable.DefaultOrder = tableBeingModified.DefaultOrder
			existableTable.Conformations = tableBeingModified.Conformations
			existableTable.Validations = tableBeingModified.Validations
			existableTable.CompositeKeys = tableBeingModified.CompositeKeys
			existableTable.Icon = tableBeingModified.Icon
			existingTables[j] = existableTable
		} else {
			log.Tracef("Table %s is not being modified", existableTable.TableName)
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
	//log.Printf("Service file from static path: %s/%s", spf.subPath, name)
	return spf.system.Open(spf.subPath + name)
}

func AddStreamsToApi2Go(api *api2go.API, processors []*resource.StreamProcessor, db database.DatabaseConnection,
	middlewareSet *resource.MiddlewareSet, configStore *resource.ConfigStore) {

	for _, processor := range processors {

		contract := processor.GetContract()
		model := api2go.NewApi2GoModel(contract.StreamName, contract.Columns, 0, nil)
		api.AddResource(model, processor)

	}

}

func GetStreamProcessors(config *resource.CmsConfig, store *resource.ConfigStore,
	cruds map[string]*resource.DbResource) []*resource.StreamProcessor {

	allProcessors := make([]*resource.StreamProcessor, 0)

	for _, streamContract := range config.Streams {

		streamProcessor := resource.NewStreamProcessor(streamContract, cruds)
		allProcessors = append(allProcessors, streamProcessor)

	}

	return allProcessors

}

func CreateAssetColumnSync(cruds map[string]dbresourceinterface.DbResourceInterface, transaction *sqlx.Tx) map[string]map[string]*assetcachepojo.AssetFolderCache {
	log.Tracef("CreateAssetColumnSync")

	stores, err := cloud_store.GetAllCloudStores(cruds["cloud_store"], transaction)
	assetCache := make(map[string]map[string]*assetcachepojo.AssetFolderCache)

	if err != nil || len(stores) == 0 {
		return assetCache
	}
	cloudStoreMap := make(map[string]rootpojo.CloudStore)

	for _, store := range stores {
		cloudStoreMap[store.Name] = store
	}

	for tableName, tableCrud := range cruds {

		colCache := make(map[string]*assetcachepojo.AssetFolderCache)

		tableInfo := tableCrud.TableInfo()
		for _, column := range tableInfo.Columns {

			if column.IsForeignKey && column.ForeignKeyData.DataSource == "cloud_store" {

				columnName := column.ColumnName

				cloudStore := cloudStoreMap[column.ForeignKeyData.Namespace]
				tempDirectoryPath, err := ioutil.TempDir(os.Getenv("DAPTIN_CACHE_FOLDER"), tableName+"_"+columnName)

				if cloudStore.StoreProvider != "local" {
					err = cruds["task"].SyncStorageToPath(cloudStore, column.ForeignKeyData.KeyName, tempDirectoryPath, transaction)
					if CheckErr(err, "Failed to setup sync to path for table column [%v][%v]", tableName, column.ColumnName) {
						continue
					}
				} else {
					tempDirectoryPath = cloudStore.RootPath + "/" + column.ForeignKeyData.KeyName
				}

				assetCacheFolder := &assetcachepojo.AssetFolderCache{
					CloudStore:    cloudStore,
					LocalSyncPath: tempDirectoryPath,
					Keyname:       column.ForeignKeyData.KeyName,
				}

				colCache[columnName] = assetCacheFolder
				log.Infof("Sync table column [%v][%v] at %v", tableName, columnName, tempDirectoryPath)

				if cloudStore.StoreProvider != "local" {
					err = TaskScheduler.AddTask(task.Task{
						EntityName: "world",
						ActionName: "sync_column_storage",
						Attributes: map[string]interface{}{
							"table_name":  tableInfo.TableName,
							"column_name": columnName,
						},
						AsUserEmail: cruds["user_account"].GetAdminEmailId(transaction),
						Schedule:    "@every 30m",
					})
				}

			}

		}

		assetCache[tableName] = colCache

	}
	log.Tracef("Completed CreateAssetColumnSync")

	return assetCache

}
