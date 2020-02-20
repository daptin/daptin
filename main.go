package main

import (
	"flag"
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
	"io"
	"io/ioutil"
	"strings"

	//"io"
	"net/http"
	"os"
	"syscall"
)

// Save the stream as a global variable
var stream = health.NewStream()

func init() {
	//goagain.Strategy = goagain.Double
	//log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	//log.SetPrefix(fmt.Sprintf("Daptin Process ID: %d ", syscall.Getpid()))

	logFileLocation, ok := os.LookupEnv("DAPTIN_LOG_LOCATION")
	if !ok || logFileLocation == "" {
		logFileLocation = "daptin.log"
	}
	f, e := os.OpenFile(logFileLocation, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if e != nil {
		log.Errorf("Failed to open logfile %v", e)
	}

	mwriter := io.MultiWriter(f, os.Stdout)

	log.SetOutput(mwriter)
}

func main() {
	//eventEmitter := &emitter.Emitter{}

	var db_type = flag.String("db_type", "sqlite3", "Database to use: sqlite3/mysql/postgres")
	var connection_string = flag.String("db_connection_string", "daptin.db", "\n\tSQLite: test.db\n"+
		"\tMySql: <username>:<password>@tcp(<hostname>:<port>)/<db_name>\n"+
		"\tPostgres: host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable")

	var webDashboardSource = flag.String("dashboard", "daptinweb/dist", "path to dist folder for daptin web dashboard")
	//var assetsSource = flag.String("assets", "assets", "path to folder for assets")
	var port = flag.String("port", ":6336", "daptin port")
	var https_port = flag.String("https_port", ":6443", "daptin https port")
	var runtimeMode = flag.String("runtime", "release", "Runtime for Gin: debug, test, release")

	envy.Parse("DAPTIN") // looks for DAPTIN_PORT, DAPTIN_DASHBOARD, DAPTIN_DB_TYPE, DAPTIN_RUNTIME
	flag.Parse()

	gin.SetMode(*runtimeMode)
	stream.AddSink(&health.WriterSink{
		Writer: os.Stdout,
	})
	boxRoot1, err := rice.FindBox("daptinweb/dist/")

	var boxRoot http.FileSystem
	if err != nil {
		log.Printf("Try loading web dashboard from: %v", *webDashboardSource)
		boxRoot = http.Dir(*webDashboardSource)
	} else {
		boxRoot = boxRoot1.HTTPBox()
	}
	statementbuilder.InitialiseStatementBuilder(*db_type)

	db, err := server.GetDbConnection(*db_type, *connection_string)
	if err != nil {
		panic(err)
	}
	log.Printf("Connection acquired from database")

	var hostSwitch server.HostSwitch
	var mailDaemon *guerrilla.Daemon
	var taskScheduler resource.TaskScheduler
	var certManager *resource.CertificateManager
	var configStore *resource.ConfigStore
	var imapServerInstance *imapServer.Server

	hostSwitch, mailDaemon, taskScheduler, configStore, certManager, imapServerInstance = server.Main(boxRoot, db)
	rhs := RestartHandlerServer{
		HostSwitch: &hostSwitch,
	}

	err = trigger.On("restart", func() {
		log.Printf("Trigger restart")

		log.Printf("Close down services and db connection")
		taskScheduler.StopTasks()

		if mailDaemon != nil {
			mailDaemon.Shutdown()
		}

		if imapServerInstance != nil {
			err = imapServerInstance.Close()
		}

		if err != nil {
			log.Printf("Failed to close DB connections: %v", err)
		}
		err = db.Close()
		if err != nil {
			log.Printf("Failed to close DB connections: %v", err)
		}

		log.Printf("All connections closed")
		log.Printf("Create new connections")
		db, err = server.GetDbConnection(*db_type, *connection_string)

		hostSwitch, mailDaemon, taskScheduler, configStore, certManager, imapServerInstance = server.Main(boxRoot, db)
		rhs.HostSwitch = &hostSwitch
		log.Printf("Restart complete")
	})
	resource.CheckErr(err, "Error while adding restart trigger function")

	portValue := *port
	if strings.Index(portValue, ".") > -1 {
		// port has ip and nothing to do
	} else if portValue[0] != ':' {
		// port is missing :
		portValue = ":" + portValue
	}
	log.Printf("[%v] Listening at port: %v", syscall.Getpid(), portValue)

	hostname, err := configStore.GetConfigValueFor("hostname", "backend")
	_, certBytes, privateBytes, _, err := certManager.GetTLSConfig(hostname, true)

	if err == nil {
		go func() {

			certTempDir := os.TempDir()
			certFile := certTempDir + "/" + hostname + ".crt"
			keyFile := certTempDir + "/" + hostname + ".key"
			log.Printf("Temp dir for certificates: %v", certTempDir)
			err = ioutil.WriteFile(certFile, certBytes, 0600)
			resource.CheckErr(err, "Failed to write cert file")
			err = ioutil.WriteFile(keyFile, privateBytes, 0600)
			resource.CheckErr(err, "Failed to write private key file")

			err1 := http.ListenAndServeTLS(*https_port, certFile, keyFile, &rhs)
			if err1 != nil {
				log.Errorf("Failed to start TLS server: %v", err1)
			}
		}()
	} else {
		log.Errorf("No Certificate available for: %v: %v", hostname, err)
	}

	err = http.ListenAndServe(*port, &rhs)
	if err != nil {
		panic(err)
	}

	log.Printf("Why quit now ?")
}

type RestartHandlerServer struct {
	HostSwitch *server.HostSwitch
}

func (rhs *RestartHandlerServer) ServeHTTP(rew http.ResponseWriter, req *http.Request) {
	rhs.HostSwitch.ServeHTTP(rew, req)
}
