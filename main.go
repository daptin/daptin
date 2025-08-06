package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/buraksezer/olric"
	olricConfig "github.com/buraksezer/olric/config"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/hostswitch"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/task_scheduler"
	server2 "github.com/fclairamb/ftpserver/server"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/memberlist"
	jsoniter "github.com/json-iterator/go"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"net/url"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GeertJohan/go.rice"
	"github.com/artpar/go-guerrilla"
	imapServer "github.com/artpar/go-imap/server"
	"github.com/daptin/daptin/server"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/gin-gonic/gin"
	"github.com/gocraft/health"
	"github.com/jamiealquiza/envy"
	"github.com/sadlil/go-trigger"
	log "github.com/sirupsen/logrus"

	//"io"
	"net/http"
	"os"
	"syscall"
)

// Save the stream as a global variable
var stream = health.NewStream()

func init() {

	goMaxProcs, ok := os.LookupEnv("DAPTIN_GOMAXPROCS")
	if !ok || goMaxProcs == "" {
		goMaxProcs = "0"
	}
	goMaxProcsInt, err := strconv.ParseInt(goMaxProcs, 10, 64)
	if err != nil {
		goMaxProcsInt = 0
	}

	log.SetFormatter(&log.TextFormatter{
		ForceColors:               true,
		DisableColors:             false,
		ForceQuote:                false,
		DisableQuote:              false,
		EnvironmentOverrideColors: false,
		DisableTimestamp:          false,
		FullTimestamp:             true,
		TimestampFormat:           "2006-01-02 15:04:05",
		DisableSorting:            false,
		SortingFunc:               nil,
		DisableLevelTruncation:    false,
		PadLevelText:              false,
		QuoteEmptyFields:          false,
		FieldMap:                  nil,
		CallerPrettyfier:          nil,
	})

	jsoniter.RegisterTypeEncoder("daptinid.DaptinReferenceId", &daptinid.DaptinReferenceEncoder{})

	log.Debugf("Go Max Procs: %v, Default value was [%v]", goMaxProcsInt, runtime.NumCPU())

	runtime.GOMAXPROCS(int(goMaxProcsInt))
	// manually set time zone
	if tz := os.Getenv("TZ"); tz != "" {
		var err error
		log.Infof("Setting timezone: %v", tz)
		time.Local, err = time.LoadLocation(tz)
		if err != nil {
			log.Printf("error loading timezone location '%s': %v\n", tz, err)
		}
	} else {
		log.Infof("Setting timezone to UTC since no TZ env variable set")
	}

	logFileLocation, ok := os.LookupEnv("DAPTIN_LOG_LOCATION")
	if !ok || logFileLocation == "" {
		return
	}

	hostname, _ := os.Hostname()
	processId := fmt.Sprintf("%v", os.Getpid())
	logFileLocation = strings.ReplaceAll(logFileLocation, "${HOSTNAME}", hostname)
	logFileLocation = strings.ReplaceAll(logFileLocation, "${PID}", processId)

	maxLogFileSize, ok := os.LookupEnv("DAPTIN_LOG_MAX_SIZE")
	if !ok {
		maxLogFileSize = "10"
	}
	maxLogFileBackups, ok := os.LookupEnv("DAPTIN_LOG_MAX_BACKUPS")
	if !ok {
		maxLogFileBackups = "10"
	}
	maxLogFileAge, ok := os.LookupEnv("DAPTIN_LOG_MAX_AGE")
	if !ok {
		maxLogFileAge = "7"
	}

	maxLogFileSizeInt, err := strconv.ParseInt(maxLogFileSize, 10, strconv.IntSize)
	if err != nil {
		log.Fatalf("invalid max log file size: %v => %v", maxLogFileSize, err)
	}
	maxLogFileBackupsInt, err := strconv.ParseInt(maxLogFileBackups, 10, strconv.IntSize)
	if err != nil {
		log.Fatalf("invalid max log file backups: %v => %v", maxLogFileBackups, err)
	}
	maxLogFileAgeInt, err := strconv.ParseInt(maxLogFileAge, 10, strconv.IntSize)
	if err != nil {
		log.Fatalf("invalid max log file age: %v => %v", maxLogFileAge, err)
	}

	lumberjackLogger := &lumberjack.Logger{
		// Log file absolute path, os agnostic
		Filename:   filepath.ToSlash(logFileLocation),
		MaxSize:    int(maxLogFileSizeInt), // MB
		MaxBackups: int(maxLogFileBackupsInt),
		MaxAge:     int(maxLogFileAgeInt), // days
		LocalTime:  true,
		Compress:   false, // disabled by default
	}

	mwriter := io.MultiWriter(lumberjackLogger, os.Stdout)
	log.SetOutput(mwriter)
	gin.DefaultWriter = mwriter
	gin.DefaultErrorWriter = mwriter
}

// Following variables will be statically linked at the time of compiling

// GitCommit holds short commit hash of source tree
var GitCommit string

// GitBranch holds current branch name the code is built off
var GitBranch string

// GitState shows whether there are uncommitted changes
var GitState string

// GitSummary holds output of git describe --tags --dirty --always
var GitSummary string

// BuildDate holds RFC3339 formatted UTC date (build time)
var BuildDate string

// Version holds contents of ./VERSION file, if exists, or the value passed via the -version option
var Version string

func printVersion() {
	fmt.Printf(`
   GitCommit: %s
   GitBranch: %s
    GitState: %s
  GitSummary: %s
   BuildDate: %s
     Version: %s
	`, GitCommit, GitBranch, GitState, GitSummary, BuildDate, Version)
}

func main() {

	restartLock := sync.Mutex{}
	//eventEmitter := &emitter.Emitter{}

	var dbType = flag.String("db_type", "sqlite3", "Database to use: sqlite3/mysql/postgres")
	var localStoragePath = flag.String("local_storage_path", "./storage", "Path where blob column assets will be stored, set to ; to disable")
	var connectionString = flag.String("db_connection_string", "daptin.db", "\n\tSQLite: test.db\n"+
		"\tMySql: <username>:<password>@tcp(<hostname>:<port>)/<db_name>\n"+
		"\tPostgres: host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable")

	var webDashboardSource = flag.String("dashboard", "daptinweb", "path to dist folder for daptin web dashboard")
	//var assetsSource = flag.String("assets", "assets", "path to folder for assets")
	var port_variable = flag.String("port_variable", "DAPTIN_PORT", "ENV port variable name to look for port")
	var database_url_variable = flag.String("database_url_variable", "DAPTIN_DB_CONNECTION_STRING", "ENV port variable name to look for connection string")
	var port = flag.String("port", ":6336", "daptin port")
	var httpsPort = flag.String("https_port", ":6443", "daptin https port")
	var runtimeMode = flag.String("runtime", "release", "Runtime for Gin: profile, debug, test, release")
	var logLevel = flag.String("log_level", "info", "log level : debug, trace, info, warn, error, fatal")
	var profileDumpPath = flag.String("profile_dump_path", "./", "location for dumping cpu/heap data in profile mode")
	var profileDumpPeriod = flag.Int("profile_dump_period", 5, "time period in minutes for triggering profile dump")
	var olricPeers = flag.String("olric_peers", "", "list of olric peers, comma separated in ip:port")
	var olricBindPort = flag.Int("olric_bind_port", 0, "port for olric server")
	var olricMembershipPort = flag.Int("olric_membership_port", 0, "port for olric membership")
	var olricConfigEnv = flag.String("olric_env", "local", "env value for olric: local/lan/wan, default: lan")

	envy.Parse("DAPTIN") // looks for DAPTIN_PORT, DAPTIN_DASHBOARD, DAPTIN_DB_TYPE, DAPTIN_RUNTIME
	flag.Parse()

	printVersion()

	logLevelParsed, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Errorf("invalid log level: %s, setting to info", *logLevel)
		logLevelParsed = log.InfoLevel
	}
	log.Infof("Runtime is [%s], logLevel: [%s]", *runtimeMode, *logLevel)
	log.SetLevel(logLevelParsed)
	profileDumpCount := 0
	if *runtimeMode == "profile" {
		gin.SetMode("release")
		log.Infof("Dumping CPU/Heap Profile at %s every %v Minutes", *profileDumpPath, *profileDumpPeriod)

		hostname, _ := os.Hostname()
		cpuprofile := fmt.Sprintf("%sdaptin_%s_profile_cpu.%v", *profileDumpPath, hostname, profileDumpCount)
		heapprofile := fmt.Sprintf("%sdaptin_%s_profile_heap.%v", *profileDumpPath, hostname, profileDumpCount)
		cpuFile, err1 := os.Create(cpuprofile)
		heapFile, err2 := os.Create(heapprofile)
		if err1 != nil || err2 != nil {
			log.Errorf("Failed to create file for profile dump: %v - %v", err1, err2)
		} else {
			var err error
			err = pprof.StartCPUProfile(cpuFile)
			auth.CheckErr(err, "Failed to start CPU profile: %v", err)
			err = pprof.WriteHeapProfile(heapFile)
			auth.CheckErr(err, "Failed to start HEAP profile: %v", err)
		}
	} else {
		gin.SetMode(*runtimeMode)
	}

	stream.AddSink(&health.WriterSink{
		Writer: os.Stdout,
	})

	boxRoot1, err := rice.FindBox("daptinweb")

	var boxRoot http.FileSystem
	if err != nil || (webDashboardSource != nil && *webDashboardSource != "daptinweb") {
		log.Tracef("Dashboard not loading from default path: %v == %v", err, boxRoot1)
		log.Infof("Try loading web dashboard from: %v", *webDashboardSource)
		boxRoot = http.Dir(*webDashboardSource)
	} else {
		boxRoot = boxRoot1.HTTPBox()
	}

	if database_url_variable != nil && *database_url_variable != "DAPTIN_DB_CONNECTION_STRING" {

		databaseUrlValue, ok := os.LookupEnv(*database_url_variable)
		if ok && len(databaseUrlValue) > 0 {
			if strings.Index(databaseUrlValue, "://") > -1 {
				log.Printf("connection URL found for database in env variable [%v]", *database_url_variable)
				databaseUrlParsed, err := url.Parse(databaseUrlValue)

				if err != nil {
					log.Printf("Unable to parse database variable value as url, passing it as it is")
					connectionString = &databaseUrlValue
				} else {
					password, _ := databaseUrlParsed.User.Password()
					databaseName := strings.Split(databaseUrlParsed.Path, "/")[1]
					switch databaseUrlParsed.Scheme {
					case "postgresql":
						fallthrough
					case "postgres":
						x := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
							databaseUrlParsed.Hostname(), databaseUrlParsed.Port(), databaseUrlParsed.User.Username(), password, databaseName)
						connectionString = &x
					case "mysql":
						x := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
							databaseUrlParsed.User.Username(), password, databaseUrlParsed.Hostname(), databaseUrlParsed.Port(), databaseName)
						connectionString = &x
					default:
						connectionString = &databaseUrlValue
					}
				}
			} else {
				connectionString = &databaseUrlValue
			}

			if *dbType == "" {
				if strings.Index(*connectionString, "dbname=") > -1 {
					*dbType = "postgres"
				} else if strings.Index(*connectionString, "@tcp") > -1 {
					*dbType = "mysql"
				}
			}
		}

	}

	statementbuilder.InitialiseStatementBuilder(*dbType)
	auth.PrepareAuthQueries()
	log.Printf("Database connection using: [%v] [%v]", *dbType, *connectionString)

	db, err := server.GetDbConnection(*dbType, *connectionString)
	if err != nil {
		panic(err)
	}
	db.Stats()
	transaction := db.MustBegin()
	_ = transaction.Rollback()
	log.Printf("connection acquired from database [%s]", *dbType)

	portValue := *port
	portInt := int64(6336)
	if strings.Index(portValue, ".") > -1 {
		// port has ip and nothing to do
		// get the port from the value, ip:port
		portInt, err = strconv.ParseInt(strings.Split(portValue, ":")[1], 10, 32)
		if err != nil {
			portInt = 6336
		}
	} else if portValue[0] != ':' {
		// port is missing :
		portInt, err = strconv.ParseInt(portValue, 10, 32)
		portValue = ":" + portValue
	}

	var hostSwitch hostswitch.HostSwitch
	var mailDaemon *guerrilla.Daemon
	var taskScheduler task_scheduler.TaskScheduler
	var certManager *resource.CertificateManager
	var configStore *resource.ConfigStore
	var ftpServer *server2.FtpServer
	var imapServerInstance *imapServer.Server
	var olricDb *olric.EmbeddedClient

	if localStoragePath != nil && *localStoragePath != "" {
		if _, err := os.Stat(*localStoragePath); err == os.ErrNotExist {
			_ = os.Mkdir(*localStoragePath, 0644)
		}
	}
	olricBindPortValue := *olricBindPort
	if olricBindPortValue == 0 {
		if portInt > 2000 {
			olricBindPortValue = int(portInt - 1000)
		} else {
			olricBindPortValue = int(portInt + 1000)
		}
	}

	log.Infof("Olric bind port is: %v", olricBindPortValue)
	olricConfigEnvValue := *olricConfigEnv

	olricConfig1 := olricConfig.New(olricConfigEnvValue)
	err = olricConfig1.SetupNetworkConfig()

	olricConfig1.BindPort = olricBindPortValue

	olricConfig1.MemberlistConfig = memberlist.DefaultLocalConfig()
	olricConfig1.MemberlistConfig.Name = fmt.Sprintf("%v:%v", olricConfig1.MemberlistConfig.BindAddr, olricConfig1.BindPort)

	olricConfig1.LogLevel = "INFO"
	olricConfig1.LogVerbosity = 1
	if len(*olricPeers) > 0 {
		olricConfig1.Peers = strings.Split(*olricPeers, ",")
	}
	log.Infof("olric peers: %v", olricConfig1.Peers)
	olricConfig1.LogOutput = os.Stdout

	olricMembershipPortValue := *olricMembershipPort
	if olricMembershipPortValue == 0 {
		olricMembershipPortValue = olricBindPortValue + 1
	}

	olricConfig1.MemberlistConfig.BindPort = olricMembershipPortValue
	log.Infof("Olric member name: %v => %v", olricConfig1.MemberlistConfig.Name, olricConfig1.BindPort)
	log.Infof("Olric membership address: %v", olricConfig1.MemberlistConfig.Name)

	emb, err := olric.New(olricConfig1)
	if err != nil {
		fmt.Printf("Failed to create olric cache: %v", err)
	}

	go func() {
		err = emb.Start()
		resource.CheckErr(err, "failed to start cache server")
	}()
	olricDb = emb.NewEmbeddedClient()

	var membersTopic *olric.PubSub

	go func() {

		time.Sleep(5 * time.Second)
		membersTopic, err := olricDb.NewPubSub()
		resource.CheckErr(err, "failed to listen to _members topic")

		sub := membersTopic.Subscribe(context.Background(), "members")
		go func(pubsub *redis.PubSub) {
			channel := pubsub.Channel()
			for {
				msg := <-channel
				log.Printf("[402] [%s] Member says: [%v]", msg.Channel, msg.String())
			}
		}(sub)
		resource.CheckErr(err, "Failed to add listener to _members topic")

		_, err = membersTopic.Publish(context.Background(), "members",
			fmt.Sprintf("Joining from %v", olricConfig1.MemberlistConfig.Name))

		resource.CheckErr(err, "Failed to publish message at _members topic")

	}()

	hostSwitch, mailDaemon, taskScheduler, configStore, certManager,
		ftpServer, imapServerInstance, olricDb = server.Main(boxRoot, db, *localStoragePath, olricDb)
	rhs := RestartHandlerServer{
		HostSwitch: &hostSwitch,
	}

	if *runtimeMode == "profile" {

		go func() {

			for {
				time.Sleep(time.Duration(*profileDumpPeriod) * time.Minute)
				log.Infof("Dumping cpu and heap profile at %s", *profileDumpPath)
				profileDumpCount += 1
				pprof.StopCPUProfile()

				hostname, _ := os.Hostname()
				cpuprofile := fmt.Sprintf("%sdaptin_%s_profile_cpu.%v", *profileDumpPath, hostname, profileDumpCount)
				heapprofile := fmt.Sprintf("%sdaptin_%s_profile_heap.%v", *profileDumpPath, hostname, profileDumpCount)

				cpuFile, err := os.Create(cpuprofile)
				heapFile, err := os.Create(heapprofile)
				if err != nil {
					log.Errorf("Failed to create file [%v] for profile dump: %v", cpuprofile, err)
				} else {
					err = pprof.StartCPUProfile(cpuFile)
					auth.CheckErr(err, "Failed to start CPU profile: %v", err)
					err = pprof.WriteHeapProfile(heapFile)
					auth.CheckErr(err, "Failed to start HEAP profile: %v", err)
				}
			}

		}()

	}

	err = trigger.On("restart", func() {
		log.Printf("Trigger restart")
		restartLock.Lock()
		defer restartLock.Unlock()

		if membersTopic != nil {
			membersTopic.Publish(context.Background(), "members",
				fmt.Sprintf("I need to restart: %v", olricConfig1.MemberlistConfig.Name))
		}

		startTime := time.Now()

		log.Printf("Close down services and db connection")
		taskScheduler.StopTasks()
		if ftpServer != nil {
			ftpServer.Stop()
		}

		if mailDaemon != nil {
			mailDaemon.Shutdown()
		}

		if imapServerInstance != nil {
			err = imapServerInstance.Close()
			if err != nil {
				log.Printf("Failed to close imap server connections: %v", err)
			}
		}

		//err = db.Close()
		//resource.CheckErr(err, "Failed to close database connection")

		log.Printf("All connections closed")
		log.Printf("Create new connections")
		//db1, err := server.GetDbConnection(*dbType, *connectionString)
		//auth.CheckErr(err, "Failed to create new db connection")
		//if err != nil {
		//	return
		//}

		//transaction := db1.MustBegin()
		//_ = transaction.Rollback()
		log.Printf("connection acquired from database [%s]", *dbType)

		hostSwitch, mailDaemon, taskScheduler, configStore, certManager,
			ftpServer, imapServerInstance, olricDb = server.Main(boxRoot, db, *localStoragePath, olricDb)
		rhs.HostSwitch = &hostSwitch

		secondsToRestart := float64(time.Now().UnixNano()-startTime.UnixNano()) / float64(1000000000)
		log.Printf("Restart complete, took %f seconds", secondsToRestart)
		if membersTopic != nil {
			membersTopic.Publish(context.Background(), "members",
				fmt.Sprintf("I am back: %v, took me [%v] seconds to restart", olricConfig1.MemberlistConfig.Name, secondsToRestart))
		}

	})

	resource.CheckErr(err, "Error while adding restart trigger function")

	if port_variable != nil && *port_variable != "DAPTIN_PORT" {
		portVarString := *port_variable

		portVarStringValue, ok := os.LookupEnv(portVarString)
		log.Infof("Looking up variable [%s] for port: %v", portVarString, portVarStringValue)
		if ok && len(portVarStringValue) > 0 {
			if portVarStringValue[0] != ':' {
				log.Infof("Port value picked from  env is missing colon: %v", portVarStringValue)
				portVarStringValue = ":" + portVarStringValue
			}
			portValue = portVarStringValue
		}
	}

	log.Infof("[ProcessId=%v] Listening at port: %v", syscall.Getpid(), portValue)

	transaction, err = db.Beginx()
	resource.CheckErr(err, "Failed to begin transaction")
	enableHttps, err := configStore.GetConfigValueFor("enable_https", "backend", transaction)
	if err != nil {
		enableHttps = "false"
		_ = configStore.SetConfigValueFor("enable_https", enableHttps, "backend", transaction)
	}

	hostname, err := configStore.GetConfigValueFor("hostname", "backend", transaction)

	backendHostnameCertificate, err := certManager.GetTLSConfig(hostname, true, transaction)
	subsitesCertificateMap, err := certManager.GetTLSForEnabledSubsites(transaction)
	transaction.Commit()

	if err == nil && enableHttps == "true" {
		go func() {

			keyPairMap := make(map[string]tls.Certificate)

			backendCertKeyPair, err := CreateKeyPairFromTLSCertificate(backendHostnameCertificate)
			if err != nil {
				panic(err)
			}

			keyPairMap[hostname] = backendCertKeyPair

			for subsiteHostname, subsiteCertificate := range subsitesCertificateMap {
				certKeyPair, err := CreateKeyPairFromTLSCertificate(subsiteCertificate)
				if err != nil {
					log.Errorf("Failed to create key pair for subsite [%v]: %v", subsiteHostname, err)
					continue
				}
				keyPairMap[subsiteHostname] = certKeyPair
			}

			tlsServer := &http.Server{
				Addr:    *httpsPort,
				Handler: &rhs,
				TLSConfig: &tls.Config{
					GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
						cert, ok := keyPairMap[info.ServerName]
						if ok {
							return &cert, nil
						}
						return nil, fmt.Errorf("certificate not found for hostname [%v]", info.ServerName)
					},
				},
			}

			log.Infof("TLS server listening on port %v", *httpsPort)
			err1 := tlsServer.ListenAndServeTLS("", "")
			if err1 != nil {
				log.Errorf("Failed to start TLS server: %v", err1)
			}
		}()
	} else {
		log.Warnf("Not starting HTTPS server: %v: %v", hostname, err)
	}

	log.Infof("Listening at: [%v]", portValue)
	err = http.ListenAndServe(portValue, &rhs)
	if err != nil {
		panic(err)
	}
	if membersTopic != nil {
		membersTopic.Publish(context.Background(), "members",
			fmt.Sprintf("I am shutting down: %v", olricConfig1.MemberlistConfig.Name))
	}

	log.Printf("Why quit now ?")
}

func CreateKeyPairFromTLSCertificate(backendHostnameCertificate *resource.TLSCertificate) (tls.Certificate, error) {
	certPem := append(backendHostnameCertificate.CertPEM, '\n')
	certPem = append(certPem, backendHostnameCertificate.RootCert...)
	keyPem := backendHostnameCertificate.PrivatePEMDecrypted

	backendCertKeyPair, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		return tls.Certificate{}, err
	}
	return backendCertKeyPair, nil
}

// RestartHandlerServer helps in switching the new router with old router with restart is triggered
type RestartHandlerServer struct {
	HostSwitch *hostswitch.HostSwitch
}

func (rhs *RestartHandlerServer) ServeHTTP(rew http.ResponseWriter, req *http.Request) {
	rhs.HostSwitch.ServeHTTP(rew, req)
}
