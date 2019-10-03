package main

import (
	"errors"
	"flag"
	"github.com/GeertJohan/go.rice"
	"github.com/artpar/go-guerrilla"
	"github.com/daptin/daptin/server"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/gin-gonic/gin"
	"github.com/gocraft/health"
	"github.com/imroc/req"
	"github.com/jamiealquiza/envy"
	"github.com/jmoiron/sqlx"
	"github.com/sadlil/go-trigger"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const testSchemas = `Tables:
  - TableName: gallery_image
    Columns:
      - Name: title
        DataType: varchar(500)
        ColumnType: label
      - Name: file
        DataType: text
        IsNullable: true
        ColumnType: image.png|jpg|jpeg|gif|tiff
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: local-store
          KeyName: images`

func TestServer(t *testing.T) {

	tempDir := os.TempDir() + "daptintest/"

	os.Mkdir(tempDir, 0777)

	schema := strings.Replace(testSchemas, "${imagePath}", tempDir, -1)

	ioutil.WriteFile(tempDir+"schema_test_daptin.yaml", []byte(schema), os.ModePerm)

	os.Setenv("DAPTIN_SCHEMA_FOLDER", tempDir)

	err := os.Remove("daptin_test.db")
	os.Remove("daptin_test.db")
	log.Printf("Failed to delete existing file %v", err)

	var db_type = flag.String("db_type", "sqlite3", "Database to use: sqlite3/mysql/postgres")
	var connection_string = flag.String("db_connection_string", "daptin_test.db", "\n\tSQLite: test.db\n"+
		"\tMySql: <username>:<password>@tcp(<hostname>:<port>)/<db_name>\n"+
		"\tPostgres: host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable")

	var webDashboardSource = flag.String("dashboard", "daptinweb/dist", "path to dist folder for daptin web dashboard")
	//var assetsSource = flag.String("assets", "assets", "path to folder for assets")
	var port = flag.String("port", ":6337", "Daptin port")
	var runtimeMode = flag.String("runtime", "debug", "Runtime for Gin: debug, test, release")

	gin.SetMode(*runtimeMode)

	envy.Parse("DAPTIN") // looks for DAPTIN_PORT, DAPTIN_DASHBOARD, DAPTIN_DB_TYPE, DAPTIN_RUNTIME
	flag.Parse()

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
	var configStore *resource.ConfigStore

	configStore, _ = resource.NewConfigStore(db)
	configStore.SetConfigValueFor("graphql.enable", "true", "backend")
	configStore.SetConfigValueFor("imap.enabled", "true", "backend")
	configStore.SetConfigValueFor("imap.listen_interface", ":8743", "backend")
	configStore.SetConfigValueFor("logs.enable", "true", "backend")

	hostSwitch, mailDaemon, taskScheduler, configStore = server.Main(boxRoot, db)

	rhs := TestRestartHandlerServer{
		HostSwitch: &hostSwitch,
	}

	trigger.On("restart", func() {
		log.Printf("Trigger restart")

		taskScheduler.StartTasks()
		mailDaemon.Shutdown()
		err = db.Close()
		if err != nil {
			log.Printf("Failed to close DB connections: %v", err)
		}

		db, err = server.GetDbConnection(*db_type, *connection_string)

		hostSwitch, mailDaemon, taskScheduler, configStore = server.Main(boxRoot, db)
		rhs.HostSwitch = &hostSwitch
	})

	log.Printf("Listening at port: %v", *port)

	srv := &http.Server{Addr: *port, Handler: rhs.HostSwitch}

	go func() {
		srv.ListenAndServe()
	}()
	time.Sleep(1 * time.Second)

	err = RunTests(t, hostSwitch, mailDaemon, db, taskScheduler, configStore)
	if err != nil {
		t.Errorf("test failed %v", err)
	}

	log.Printf("Shutdown now")
	//
	//shutDown := make(chan bool)
	//
	//srv.RegisterOnShutdown(func() {
	//	shutDown <- true
	//})
	//err = srv.Shutdown(context.Background())
	//if err != nil {
	//	log.Printf("Failed to shut down server")
	//}
	//
	//<-shutDown
	//log.Printf("Shut down complete")
	//
	//err = os.Remove("daptin_test.db")
	//if err != nil {
	//	log.Printf("Failed to delete test database file")
	//}

}

func RunTests(t *testing.T, hostSwitch server.HostSwitch, daemon *guerrilla.Daemon, db *sqlx.DB, scheduler resource.TaskScheduler, configStore *resource.ConfigStore) error {

	const baseAddress = "http://localhost:6337"

	r := req.New()

	responseMap := make(map[string]interface{})

	resp, err := r.Get(baseAddress + "/api/world")
	if err != nil {
		return err
	}

	resp.ToJSON(&responseMap)

	data := responseMap["data"].([]interface{})
	firstRow := data[0].(map[string]interface{})

	if firstRow["type"] != "world" {
		t.Errorf("world type mismatch")
	}

	resp, err = r.Get(baseAddress + "/actions")

	if err != nil {
		return err
	}

	actionMap := make(map[string]interface{})
	resp.ToJSON(&actionMap)

	signInAction, ok := actionMap["user:signin"].(map[string]interface{})
	if !ok {
		t.Errorf("signin action not found")
	}

	if signInAction["OnType"] != "user_account" {
		t.Errorf("Unexpected on type")
	}

	signUpAction, ok := actionMap["user:signup"].(map[string]interface{})
	if !ok {
		t.Errorf("signin action not found")
	}
	if signUpAction["OnType"] != "user_account" {
		t.Errorf("Unexpected on type")
	}

	resp, err = r.Get(baseAddress + "/meta?query=column_types")
	if err != nil {
		return err
	}

	cols := make(map[string]interface{})
	resp.ToJSON(&cols)

	if cols["label"] == nil {
		t.Errorf("label not found")
	}

	resp, err = r.Post(baseAddress+"/action/user_account/signup", req.BodyJSON(map[string]interface{}{
		"attributes": map[string]interface{}{
			"email":           "test@gmail.com",
			"name":            "name",
			"password":        "tester123",
			"passwordConfirm": "tester123",
		},
	}))

	if err != nil {
		return err
	}
	var signUpResponse interface{}

	resp.ToJSON(&signUpResponse)

	if signUpResponse.([]interface{})[0].(map[string]interface{})["ResponseType"] != "client.notify" {
		t.Errorf("Unexpected response type from sign up")
	}

	resp, err = r.Post(baseAddress+"/action/user_account/signin", req.BodyJSON(map[string]interface{}{
		"attributes": map[string]interface{}{
			"email":    "test@gmail.com",
			"password": "tester123",
		},
	}))

	if err != nil {
		return err
	}

	var token string
	var signInResponse interface{}

	resp.ToJSON(&signInResponse)

	responseAttr := signInResponse.([]interface{})[0].(map[string]interface{})
	if responseAttr["ResponseType"] != "client.store.set" {
		t.Errorf("Unexpected response type from sign up")
	}

	token = responseAttr["Attributes"].(map[string]interface{})["value"].(string)

	t.Logf("Token: %v", token)

	resp, err = r.Get(baseAddress + "/recline_model")
	if err != nil {
		return err
	}
	reclineModelMap := make(map[string]interface{})
	err = resp.ToJSON(&reclineModelMap)
	if err != nil {
		return err
	}

	if reclineModelMap["alias"] != "string" {
		return errors.New("unexpected recline model response")
	}

	resp, err = r.Get(baseAddress + "/jsmodel/world.js")
	if err != nil {
		return err
	}
	jsModelMap := make(map[string]interface{})
	err = resp.ToJSON(&jsModelMap)
	if err != nil {
		return err
	}

	if jsModelMap["ColumnModel"] == nil {
		return errors.New("unexpected model map response")
	}

	_, err = r.Get(baseAddress + "/favicon.ico")
	if err != nil {
		return err
	}

	_, err = r.Get(baseAddress + "/favicon.png")
	if err != nil {
		return err
	}

	_, err = r.Get(baseAddress + "/statistics")
	if err != nil {
		return err
	}

	_, err = r.Get(baseAddress + "/openapi.yaml")
	if err != nil {
		return err
	}

	// check user flow
	resp, err = r.Get(baseAddress+"/api/world", req.Header{
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		return err
	}

	resp.ToJSON(&responseMap)

	data = responseMap["data"].([]interface{})
	firstRow = data[0].(map[string]interface{})

	if firstRow["type"] != "world" {
		t.Errorf("world type mismatch")
	}

	return nil

}

type TestRestartHandlerServer struct {
	HostSwitch *server.HostSwitch
}

func (rhs *TestRestartHandlerServer) ServeHTTP(rew http.ResponseWriter, req *http.Request) {
	rhs.HostSwitch.ServeHTTP(rew, req)
}
