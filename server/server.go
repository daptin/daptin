package server

import (
	"crypto/tls"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/api2go-adapter/gingonic"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/server"
	"github.com/artpar/go.uuid"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/stats"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/websockets"
	"github.com/emersion/go-sasl"
	"github.com/gin-gonic/gin"
	"github.com/hpcloud/tail"
	"github.com/icrowley/fake"
	"io"
	"os"
	"strings"
	"time"
	//"github.com/gin-gonic/gin"
	graphqlhandler "github.com/graphql-go/handler"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

var TaskScheduler resource.TaskScheduler
var Stats = stats.New()

func Main(boxRoot http.FileSystem, db database.DatabaseConnection) (HostSwitch, *guerrilla.Daemon, resource.TaskScheduler, *resource.ConfigStore) {

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
	defaultRouter.Use(CorsMiddlewareFunc)
	defaultRouter.StaticFS("/static", NewSubPathFs(boxRoot, "/static"))

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

	//logTail, err := tail.TailFile("daptin.log", tail.Config{Follow: true})

	//last10Lines := make([]string, 0)

	//go func() {
	//	for line := range logTail.Lines {
	//		last10Lines = append(last10Lines, line.Text)
	//		if len(last10Lines) > 10 {
	//			last10Lines = last10Lines[1:]
	//		}
	//		fmt.Println(line.Text)
	//	}
	//}()

	configStore, err := resource.NewConfigStore(db)
	resource.CheckErr(err, "Failed to get config store")

	hostname, err := configStore.GetConfigValueFor("hostname", "backend")
	if err != nil {
		name, e := os.Hostname()
		if e != nil {
			name = "localhost"
		}
		hostname = name
		configStore.SetConfigValueFor("hostname", hostname, "backend")
	}

	initConfig.Hostname = hostname

	jwtSecret, err := configStore.GetConfigValueFor("jwt.secret", "backend")
	if err != nil {
		u, _ := uuid.NewV4()
		newSecret := u.String()
		configStore.SetConfigValueFor("jwt.secret", newSecret, "backend")
		jwtSecret = newSecret
	}

	enablelogs, err := configStore.GetConfigValueFor("logs.enable", "backend")
	if err != nil {
		configStore.SetConfigValueFor("logs.enable", "false", "backend")
	}

	go func() {

		for {

			fileInfo, err := os.Stat("daptin.log")
			if err != nil {
				log.Errorf("Failed to stat log file: %v", err)
			}

			fileMbs := fileInfo.Size() / (1024 * 1024)
			//log.Printf("Current log size: %d MB", fileMbs)
			if fileMbs > 100 {
				logFile := "daptin.log"
				os.Remove(logFile)
				os.Create(logFile)
				f, e := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
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

		defaultRouter.GET("/__logs", func(c *gin.Context) {
			logTail, err := tail.TailFile("daptin.log", tail.Config{
				Follow: true,
				ReOpen: true,
				Location: &tail.SeekInfo{
					Offset: 0,
					Whence: 2,
				},
			})
			if err != nil {
				c.AbortWithError(500, err)
				return
			}

			for line := range logTail.Lines {
				c.Writer.WriteString(line.Text + "\n")
				c.Writer.Flush()
			}

		})
	}

	enableGraphql, err := configStore.GetConfigValueFor("graphql.enable", "backend")
	if err != nil {
		configStore.SetConfigValueFor("graphql.enable", fmt.Sprintf("%v", initConfig.EnableGraphQL), "backend")
	} else {
		if enableGraphql == "true" {
			initConfig.EnableGraphQL = true
		} else {
			initConfig.EnableGraphQL = false
		}
	}

	err = CheckSystemSecrets(configStore)
	resource.CheckErr(err, "Failed to initialise system secrets")

	defaultRouter.GET("/config", CreateConfigHandler(configStore))

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

	streamProcessors := GetStreamProcessors(&initConfig, configStore, cruds)

	mailDaemon, err := StartSMTPMailServer(cruds["mail"])

	if err == nil {
		err = mailDaemon.Start()
		if err != nil {
			log.Errorf("Failed to start mail daemon: %s", err)
		} else {
			log.Infof("Started mail server")
		}
	} else {
		log.Errorf("Failed to start mail daemon: %s", err)
	}

	// Create a memory backend
	enableImapServer, err := configStore.GetConfigValueFor("imap.enabled", "backend")
	if err == nil && enableImapServer == "true" {
		imapListenInterface, err := configStore.GetConfigValueFor("imap.listen_interface", "backend")
		if err != nil {
			configStore.SetConfigValueFor("imap.listen_interface", ":1143", "backend")
			imapListenInterface = ":1143"
		}
		imapBackend := resource.NewImapServer(cruds)

		// Create a new server
		s := server.New(imapBackend)
		s.Addr = imapListenInterface

		//s.Debug = os.Stdout

		s.EnableAuth(sasl.Login, func(conn server.Conn) sasl.Server {
			return sasl.NewLoginServer(func(username, password string) error {
				user, err := conn.Server().Backend.Login(conn.Info(), username, password)
				if err != nil {
					return err
				}

				ctx := conn.Context()
				ctx.State = imap.AuthenticatedState
				ctx.User = user
				return nil
			})
		})

		s.EnableAuth("CRAM-MD5", func(conn server.Conn) sasl.Server {

			return &Crammd5{
				dbResource:  cruds["mail"],
				conn:        conn,
				imapBackend: imapBackend,
			}
		})
		var LocalhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIETzCCAregAwIBAgIQH/X44kGApj052IIhHfJrszANBgkqhkiG9w0BAQsFADBh
MR4wHAYDVQQKExVta2NlcnQgZGV2ZWxvcG1lbnQgQ0ExGzAZBgNVBAsMEmFydHBh
ckBhYmJhZC5sb2NhbDEiMCAGA1UEAwwZbWtjZXJ0IGFydHBhckBhYmJhZC5sb2Nh
bDAeFw0xOTA2MTAwNDU3MjdaFw0yOTA2MTAwNDU3MjdaMEQxJzAlBgNVBAoTHm1r
Y2VydCBkZXZlbG9wbWVudCBjZXJ0aWZpY2F0ZTEZMBcGA1UECwwQcm9vdEBhYmJh
ZC5sb2NhbDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAN2pi0zl9EJH
qtKBdlaEXoOU4YwHdzwxyfExcBeCMjyAkHPRPnWZKEJfgDc5WcF7wr/kxHCTrUhI
RwRz/o5BIIjZmMrswFdCxZ74lKgHpYZl5s2+VAKgmAUhFOV33t/uL9tK2HevqfXi
FaseVIENIjKkdVHOwOfRcRbd2rmB3QV+b+sb82etqnnkPxIdVE9cHMaHVFIBiGe9
95LMPwrnq4aeHdATapVC4R6T6CK/dl2lzH95P61QsBa7t/awJ4EzcAMpUbEutEEw
BZy24heKgKaw+mOHuyCcuOff8EJ/hsf6VqoPzNJz0sZcbiQlXdRK7Ibmu5hZjQTc
bp3Ql+Bj2Q8CAwEAAaOBnzCBnDAOBgNVHQ8BAf8EBAMCBaAwEwYDVR0lBAwwCgYI
KwYBBQUHAwEwDAYDVR0TAQH/BAIwADAfBgNVHSMEGDAWgBRs0GlUpxTRlDw0D+eF
O8+2CRuc2TBGBgNVHREEPzA9ggpkYXB0aW4uY29tggwqLmRhcHRpbi5jb22CCWxv
Y2FsaG9zdIcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATANBgkqhkiG9w0BAQsFAAOC
AYEARMqj64kOfU7WHVgiOvsvTAxsyc9b7tmez5tid0o6VFBzH7XHq6uUJmMy21nm
3h8K2O/ovRlzXIMtfUjWYirSAoy0frkMXe6A7oZojpgFOjDJ699N7MDKiQ6ijzRc
gA9qVgK/ATrmVCtd9HlHgSbcXhaf3YUR++icvT5+3osvyNkGrNWRI+TyfogXj81e
7UCqnZjFOJidA6RZbMnCgoOS2+2P2CnUVkMazibd9Hc158dI9Hg9VyntnTOl4nWw
8rDfauWbg0AOus/q8qnPcZDR6DahC6piEgWeyC1P6dBDFDYMmCMIegGfgcZmA70T
S50eo3+VlsDuRmtg5Dfh4zC6Bb/rx0CBx9KJid8f1eBUQ4EFYaZDPQ3XBWDvduVe
TjjoGFkD5QRz/601bFVP6/DqDcjDbxbjyarWfxTu1nHukKkxemb265zLhtVwAbfd
SoGwaheGW0/zeaGZGQwnL4hCkJokHagmsSg3ZynqoinNVrJJBKfisucmzUlC365P
uBww
-----END CERTIFICATE-----`)

		// LocalhostKey is the private key for localhostCert.
		var LocalhostKey = []byte(`-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDdqYtM5fRCR6rS
gXZWhF6DlOGMB3c8McnxMXAXgjI8gJBz0T51mShCX4A3OVnBe8K/5MRwk61ISEcE
c/6OQSCI2ZjK7MBXQsWe+JSoB6WGZebNvlQCoJgFIRTld97f7i/bSth3r6n14hWr
HlSBDSIypHVRzsDn0XEW3dq5gd0Ffm/rG/Nnrap55D8SHVRPXBzGh1RSAYhnvfeS
zD8K56uGnh3QE2qVQuEek+giv3Zdpcx/eT+tULAWu7f2sCeBM3ADKVGxLrRBMAWc
tuIXioCmsPpjh7sgnLjn3/BCf4bH+laqD8zSc9LGXG4kJV3USuyG5ruYWY0E3G6d
0JfgY9kPAgMBAAECggEAQ4XuRVKXgclLJC0D238fO34S5xEvJUsVdT/WIZMrsnqH
hoBrQm+RcAafjDMQQHxu6v3JSXHzC13ZJGYhWTxFqOqAPPC59tsEUFTxE+6gYbyQ
/oPIG7TIGmflcbF+V0C7m1XFc1Azug9RAnuOynExxbOLeYw9/2AxzwFuK6x/o7g7
N1pTKS2BLUyC4trn19llo1Vf8tX9mSKjlluQy/ewF2LP/zhZSw50Qts3KMcuBXGx
BF4YjGjHSOELmZh+sM7ZZasfKxJ9QnCF2cqXSvVydDo20TwOwZKvcsJDmwpVcLqX
H4cHJLFfO4A3aBBOhu+Cs1uHjs3VXRSvv/mo3G+oQQKBgQDnp4TSQ/C2thhQTOO/
GXAznBXZo4FLdzk6YodOvfLYC7/e6xL7QYz1U4BxrGEVy3EMKmJ1798e8OctHIk7
AV+wI4AL2QBx5sMNFAjb9+CXl/Gqhbk+U4fjnmc+bsNy9vR/qvjlIt/Jp9y85zeF
QSO6/xTlLLHpe1J293OUv9nstwKBgQD09TInZ1U1fFMjC6BWjCDGLNC8//rFFKAR
jTjLEYCY0DuN6PJfywUS6ZoHKHoXKe60qaFRHqAk83T6iHExAlirwqylu6QSyxeQ
vWcSnoAJqHY7j84uC+Vf6WEEwAN4XImcmT8in9MtbEPZI8ytjN6K5XNsHOc9ORYW
XY+6xy5OaQKBgGc8Gk7yBBYItHEksuH43i3Bw2MIIJiW+yPvwMjwkYaCRfF75Suf
nMe/fKAr5+Akl66KPPK+ATrytLM/4lAvXotKZsfg3vfjlM0BPql4n9gu2H3bth/2
bbqcXvpNtkBHmdJDSUQj9IMTkaWFjRKPYvL0tkUjU+3vDWMDB7kkfmOlAoGAbWJE
iCXzfdPLiB279onSZMxEVfF0uKbSJ6RJVRy2sQZjYaZA/Re6Z0ybNFEV29wktNX+
rCuh1X5FoU5mRT1H/UMMN2HIDYBVQJPjQAQ5JpbsXQKFTjiPr7mWUjmwEwI3jQ89
iyeVdHYhAgijcGg0RA/b784kUEl6nHghI4WoHukCgYEAgZMXR1uCqsvmTEe2jkPX
sS6Lb/Elxq5uRlEays/t19zzc7S4kU+VT02oUYHUQm7VSQmHLvqD7lrgKjVS4ihc
aMgXlWWI/k15RrrmdE3HMkV0HfPnZVrRsilZTYF4mTCvEehQVMGhDGHgrxGY4nTE
fagus7nZFuPIRAU1dz5Ni1g=
-----END PRIVATE KEY-----`)

		cert, err := tls.X509KeyPair(LocalhostCert, LocalhostKey)
		if err != nil {
			log.Printf("Failed to load cert: %v", err)
		}

		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			Certificates:       []tls.Certificate{cert},
		}

		// Since we will use this server for testing only, we can allow plain text
		// authentication over unencrypted connections
		s.AllowInsecureAuth = false

		if err != nil {
			log.Fatal(err)
		}

		s.TLSConfig = tlsConfig
		//idleExt := idle.NewExtension()
		//s.Enable(idleExt)

		log.Printf("Starting IMAP server at %s\n", imapListenInterface)

		go func() {
			if err := s.ListenAndServe(); err != nil {
				log.Fatal(err)
			}
		}()

	} else {
		if err != nil {
			configStore.SetConfigValueFor("imap.enabled", "false", "backend")
		}
	}

	actionPerformers := GetActionPerformers(&initConfig, configStore, cruds, mailDaemon)
	initConfig.ActionPerformers = actionPerformers

	AddStreamsToApi2Go(api, streamProcessors, db, &ms, configStore)

	// todo : move this somewhere and make it part of something
	actionHandlerMap := actionPerformersListToMap(actionPerformers)
	for k := range cruds {
		cruds[k].ActionHandlerMap = actionHandlerMap
	}

	resource.ImportDataFiles(initConfig.Imports, db, cruds)

	TaskScheduler = resource.NewTaskScheduler(&initConfig, cruds, configStore)

	err = TaskScheduler.AddTask(resource.Task{
		EntityName:  "mail_server",
		ActionName:  "sync_mail_servers",
		Attributes:  map[string]interface{}{},
		AsUserEmail: cruds[resource.USER_ACCOUNT_TABLE_NAME].GetAdminEmailId(),
		Schedule:    "@every 1h",
	})

	TaskScheduler.StartTasks()

	hostSwitch := CreateSubSites(&initConfig, db, cruds, authMiddleware)
	assetColumnFolders := CreateAssetColumnSync(&initConfig, db, cruds, authMiddleware)
	for k := range cruds {
		cruds[k].AssetFolderCache = assetColumnFolders
	}

	hostSwitch.handlerMap["api"] = defaultRouter
	hostSwitch.handlerMap["dashboard"] = defaultRouter

	authMiddleware.SetUserCrud(cruds[resource.USER_ACCOUNT_TABLE_NAME])
	authMiddleware.SetUserGroupCrud(cruds["usergroup"])
	authMiddleware.SetUserUserGroupCrud(cruds["user_account_user_account_id_has_usergroup_usergroup_id"])

	fsmManager := resource.NewFsmManager(db, cruds)

	defaultRouter.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	handler := CreateJsModelHandler(&initConfig, cruds)
	metaHandler := CreateMetaHandler(&initConfig)
	blueprintHandler := CreateApiBlueprintHandler(&initConfig, cruds)
	modelHandler := CreateReclineModelHandler()
	statsHandler := CreateStatsHandler(&initConfig, cruds)
	resource.InitialiseColumnManager()

	dbAssetHandler := CreateDbAssetHandler(&initConfig, cruds)
	defaultRouter.GET("/asset/:typename/:resource_id/:columnname", dbAssetHandler)

	resource.RegisterTranslations()

	if initConfig.EnableGraphQL {

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

	loader := CreateSubSiteContentHandler(&initConfig, cruds, db)
	defaultRouter.POST("/site/content/load", loader)
	defaultRouter.GET("/site/content/load", loader)
	defaultRouter.POST("/site/content/store", CreateSubSiteSaveContentHandler(&initConfig, cruds, db))

	//webSocketConnectionHandler := WebSocketConnectionHandlerImpl{}
	//websocketServer := websockets.NewServer("/live", &webSocketConnectionHandler)

	//go websocketServer.Listen(defaultRouter)

	indexFile, err := boxRoot.Open("index.html")

	var indexFileContents = []byte("")
	if indexFile != nil {

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

	return hostSwitch, mailDaemon, TaskScheduler, configStore

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

	log.Printf("Client sent: %v", string(response))

	if string(response) == "" {
		newChallenge := fmt.Sprintf("<%v.%v.%v>", fake.DigitsN(8), time.Now().UnixNano(), "daptin")
		c.challenge = newChallenge
		return []byte(""), false, nil
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
	//AddStateMachines(&initConfig, db)

	tx, errb := db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	resource.CheckAllTableStatus(initConfig, db, tx)
	errc := tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating tables")

	tx, errb = db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	resource.CreateRelations(initConfig, tx)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating relations")

	tx, errb = db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	resource.CreateUniqueConstraints(initConfig, tx)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating unique constrains")

	tx, errb = db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	resource.CreateIndexes(initConfig, tx)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating indexes")

	tx, errb = db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	resource.UpdateWorldTable(initConfig, tx)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after updating world tables")

	resource.UpdateStateMachineDescriptions(initConfig, db)
	resource.UpdateExchanges(initConfig, db)
	resource.UpdateStreams(initConfig, db)
	resource.UpdateMarketplaces(initConfig, db)
	err := resource.UpdateTasksData(initConfig, db)
	resource.CheckErr(err, "Failed to  update cron jobs")
	resource.UpdateStandardData(initConfig, db)

	err = resource.UpdateActionTable(initConfig, db)
	resource.CheckErr(err, "Failed to update action table")

}

func actionPerformersListToMap(interfaces []resource.ActionPerformerInterface) map[string]resource.ActionPerformerInterface {
	m := make(map[string]resource.ActionPerformerInterface)

	for _, api := range interfaces {
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
			log.Printf("Table %s is being modified", existableTable.TableName)
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
