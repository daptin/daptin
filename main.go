package main

import (
	"flag"
	"fmt"
	"github.com/GeertJohan/go.rice"
	//"github.com/artpar/goagain"
	"github.com/daptin/daptin/server"
	"github.com/gin-gonic/gin"
	"github.com/gocraft/health"
	"github.com/jamiealquiza/envy"
	"log"
	"net/http"
	"github.com/sadlil/go-trigger"
	"os"
	"sync"
	"syscall"
	"os/signal"
	//"github.com/artpar/goagain"
)

// Save the stream as a global variable
var stream = health.NewStream()

func init() {
	//goagain.Strategy = goagain.Double
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	log.SetPrefix(fmt.Sprintf("pid:%d ", syscall.Getpid()))
}

func main() {
	//eventEmitter := &emitter.Emitter{}

	var db_type = flag.String("db_type", "sqlite3", "Database to use: sqlite3/mysql/postgres")
	var connection_string = flag.String("db_connection_string", "daptin.db", "\n\tSQLite: test.db\n"+
		"\tMySql: <username>:<password>@tcp(<hostname>:<port>)/<db_name>\n"+
		"\tPostgres: host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable")

	var webDashboardSource = flag.String("dashboard", "daptinweb/dist", "path to dist folder for daptin web dashboard")
	//var assetsSource = flag.String("assets", "assets", "path to folder for assets")
	var port = flag.String("port", ":6336", "Daptin port")
	var runtimeMode = flag.String("runtime", "debug", "Runtime for Gin: debug, test, release")

	gin.SetMode(*runtimeMode)

	envy.Parse("DAPTIN") // looks for DAPTIN_PORT
	flag.Parse()

	stream.AddSink(&health.WriterSink{os.Stdout})
	//assetsRoot, err := rice.FindBox("assets")
	//resource.CheckErr(err, "Failed to open %s/static", assetsSource)
	boxRoot1, err := rice.FindBox("daptinweb/dist/")
	if err != nil {
		panic(err)
	}

	var boxRoot http.FileSystem
	if err != nil {
		log.Printf("Try loading web dashboard from: %v", *webDashboardSource)
		//assetsStatic = http.Dir(*webDashboardSource + "/static")
		boxRoot = http.Dir(*webDashboardSource)
	} else {
		//assetsStatic = assetsRoot.HTTPBox()
		boxRoot = boxRoot1.HTTPBox()
	}
	db, err := server.GetDbConnection(*db_type, *connection_string)
	if err != nil {
		panic(err)
	}
	log.Printf("Connection acquired from database")

	// Inherit a net.Listener from our parent process or listen anew.
	//ch := make(chan struct{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	//l, err := goagain.Listener()

	var hostSwitch server.HostSwitch

	hostSwitch = server.Main(boxRoot, db)
	rhs := RestartHandlerServer{
		HostSwitch: &hostSwitch,
	}

	trigger.On("restart", func() {
		// Do Some Task Here.
		log.Printf("Trigger restart")
		hostSwitch = server.Main(boxRoot, db)
		rhs.HostSwitch = &hostSwitch
	})

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Signal(syscall.SIGUSR2))
	go func() {
		for sig := range c {
			switch sig.String() {
			case "user defined signal 2":
				log.Printf("Got signal: %v, updatd host switch", sig)
				hostSwitch = server.Main(boxRoot, db)
				rhs.HostSwitch = &hostSwitch

			}
			// sig is a ^C, handle it
		}
	}()

	//log.Printf("Listening at: %v", l.Addr().String())
	//go func() {
	err = http.ListenAndServe(*port, &rhs)
	if err != nil {
		panic(err)
	}
	//}()

	log.Printf("Why end now ?")
}

type RestartHandlerServer struct {
	HostSwitch *server.HostSwitch
}

func (rhs *RestartHandlerServer) ServeHTTP(rew http.ResponseWriter, req *http.Request) {
	rhs.HostSwitch.ServeHTTP(rew, req)
}
