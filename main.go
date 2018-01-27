package main

import (
	"github.com/daptin/daptin/server"
	"github.com/gocraft/health"
	//"github.com/jpillora/overseer"
	"log"
	//"os"
	"fmt"
	//"sync"
	"github.com/GeertJohan/go.rice"
	"github.com/daptin/daptin/server/resource"
	"net/http"
	"os"
	"syscall"
	//"github.com/jpillora/overseer"
	"flag"
	"github.com/artpar/goagain"
	"github.com/gin-gonic/gin"
	"github.com/jamiealquiza/envy"
	"net"
	"sync"
)

// Save the stream as a global variable
var stream = health.NewStream()

func init() {
	goagain.Strategy = goagain.Double
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	log.SetPrefix(fmt.Sprintf("pid:%d ", syscall.Getpid()))
}

func main() {

	var db_type = flag.String("db_type", "sqlite3", "Database to use: sqlite3/mysql/postgres")
	var connection_string = flag.String("db_connection_string", "daptin.db", "\n\tSQLite: test.db\n"+
		"\tMySql: <username>:<password>@tcp(<hostname>:<port>)/<db_name>\n"+
		"\tPostgres: host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable")

	var webDashboardSource = flag.String("dashboard", "daptinweb/dist", "path to dist folder for daptin web dashboard")
	var assetsSource = flag.String("assets", "assets", "path to folder for assets")
	var port = flag.String("port", "6336", "Daptin port")
	var runtimeMode = flag.String("runtime", "debug", "Runtime for Gin: debug, test, release")

	gin.SetMode(*runtimeMode)

	envy.Parse("DAPTIN") // looks for GOMS_PORT
	flag.Parse()

	stream.AddSink(&health.WriterSink{os.Stdout})
	assetsRoot, err := rice.FindBox("assets")
	resource.CheckErr(err, "Failed to open %s/static", assetsSource)
	boxRoot1, err := rice.FindBox("daptinweb/dist/")
	resource.CheckErr(err, "Failed to open %s", webDashboardSource)



	var assetsStatic, boxRoot http.FileSystem
	if err != nil {
		log.Printf("Try loading web dashboard from: %v", *webDashboardSource)
		assetsStatic = http.Dir(*webDashboardSource + "/static")
		boxRoot = http.Dir(*webDashboardSource)
	} else {
		assetsStatic = assetsRoot.HTTPBox()
		boxRoot = boxRoot1.HTTPBox()
	}
	db, err := server.GetDbConnection(*db_type, *connection_string)
	resource.CheckErr(err, "Failed to connect to database")
	log.Printf("Connection acquired from database")

	// Inherit a net.Listener from our parent process or listen anew.
	ch := make(chan struct{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	l, err := goagain.Listener()
	if nil != err {

		// Listen on a TCP or a UNIX domain socket (TCP here).
		l, err = net.Listen("tcp", fmt.Sprintf(":%v", *port))
		if nil != err {
			log.Printf("Failed to listen to port: %v", err)
		} else {
			log.Println("listening on", l.Addr())
			// Accept connections in a new goroutine.
			go server.Main(boxRoot, assetsStatic, db, wg, l, ch)

		}

	} else {

		// Resume listening and accepting connections in a new goroutine.
		log.Println("resuming listening on", l.Addr())
		go server.Main(boxRoot, assetsStatic, db, wg, l, ch)

		// If this is the child, send the parent SIGUSR2.  If this is the
		// parent, send the child SIGQUIT.
		if err := goagain.Kill(); nil != err {
			log.Fatalln(err)
		}

	}

	// Block the main goroutine awaiting signals.
	sig, err := goagain.Wait(l)
	if nil != err {
		log.Fatalln(err)
	}

	// Do whatever's necessary to ensure a graceful exit like waiting for
	// goroutines to terminate or a channel to become closed.
	//
	// In this case, we'll close the channel to signal the goroutine to stop
	// accepting connections and wait for the goroutine to exit.
	close(ch)
	wg.Wait()

	// If we received SIGUSR2, re-exec the parent process.
	log.Printf("Daptin main signal received: %v", sig)
	if goagain.SIGUSR2 == sig {
		if err := goagain.Exec(l); nil != err {
			log.Fatalln(err)
		}
	}
	log.Printf("Why end now ?")
}

//func CreateServerProgram(boxRoot, boxStatic http.FileSystem) (func(state overseer.State)) {
//	return func(state overseer.State) {
//		go server.Main(boxRoot, boxStatic)
//		http.Serve(state.Listener, nil)
//	}
//}
