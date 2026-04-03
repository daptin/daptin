//go:build test
// +build test

package server

import (
	"github.com/artpar/api2go/v2"
	"github.com/artpar/ydb"
	"github.com/buraksezer/olric"
	olricConfig "github.com/buraksezer/olric/config"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"testing"
)

func GetInMemoryDbForTest() *InMemoryTestDatabase {
	db, err := sqlx.Open("sqlite3", "daptin_test.db")
	if err != nil {
		panic(err)
	}

	wrapper := NewInMemoryTestDatabase(db)
	return wrapper

}

func GetResource() (*InMemoryTestDatabase, *resource.DbResource) {
	wrapper := GetInMemoryDbForTest()

	configStore, _ := resource.NewConfigStore(wrapper)

	initConfig, _ := LoadConfigFiles()

	existingTables, _ := GetTablesFromWorld(wrapper)

	allTables := MergeTables(existingTables, initConfig.Tables)

	initConfig.Tables = allTables

	cruds := make(map[string]*resource.DbResource)

	olricDb1, _ := olric.New(olricConfig.New("local"))
	olricDb := olricDb1.NewEmbeddedClient()

	dtopicMap := make(map[string]*olric.PubSub)

	store := ydb.NewDiskStore("/tmp")
	ms := BuildMiddlewareSet(&initConfig, &cruds, store, &dtopicMap)
	for _, table := range initConfig.Tables {
		model := api2go.NewApi2GoModel(table.TableName, table.Columns, int64(table.DefaultPermission), table.Relations)
		res, _ := resource.NewDbResource(model, wrapper, &ms, cruds, configStore, olricDb, table)
		cruds[table.TableName] = res
	}

	for key := range cruds {
		pubSub, err := olricDb.NewPubSub()
		if err != nil {
			resource.CheckErr(err, "Failed to create pubsub for table: %v", key)
			continue
		}
		dtopicMap[key] = pubSub
	}

	resource.CheckRelations(&initConfig)
	resource.CheckAuditTables(&initConfig)

	tx, errb := wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction [76]")
	resource.CheckAllTableStatus(&initConfig, wrapper)
	errc := tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating tables")

	tx, errb = wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction [82]")
	resource.CreateRelations(&initConfig, wrapper)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating relations")

	tx, errb = wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction [88]")
	resource.CreateUniqueConstraints(&initConfig, tx)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating unique constrains")

	tx, errb = wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction [94]")
	resource.CreateIndexes(&initConfig, wrapper)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating indexes")

	tx, errb = wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction [100]")
	resource.UpdateWorldTable(&initConfig, tx)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after updating world tables")

	tx, errb = wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction [106]")
	resource.UpdateStateMachineDescriptions(&initConfig, tx)
	resource.UpdateExchanges(&initConfig, tx)
	resource.UpdateStreams(&initConfig, tx)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after updates")

	tx, errb = wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction [113]")
	err := resource.UpdateActionTable(&initConfig, tx)
	resource.CheckErr(err, "Failed to update action table")
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after updating action table")

	model := api2go.NewApi2GoModel("", nil, 0, nil)
	dbResource, _ := resource.NewDbResource(model, wrapper, &ms, cruds, configStore, nil, table_info.TableInfo{})
	return wrapper, dbResource
}

func GetResourceWithName(name string) (*InMemoryTestDatabase, *resource.DbResource) {
	wrapper := GetInMemoryDbForTest()

	var cols []api2go.ColumnInfo
	model := api2go.NewApi2GoModel(name, cols, 0, nil)
	tableInfo := table_info.TableInfo{
		TableName: name,
	}

	cruds := make(map[string]*resource.DbResource)
	dbResource, _ := resource.NewDbResource(model, wrapper, nil, cruds, nil, nil, tableInfo)
	cruds[name] = dbResource
	return wrapper, dbResource
}

func TestGetReferenceIdToObject(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()

	tx, _ := wrapper.Beginx()
	defer tx.Rollback()
	_, _ = dbResource.Cruds["world"].GetReferenceIdToObjectWithTransaction("world", daptinid.DaptinReferenceId(uuid.New()), tx)

	if !wrapper.HasExecuted("SELECT * FROM world WHERE reference_id =") {
		t.Errorf("Expected query not fired")
		t.FailNow()
	}

}

func TestUserGroupNameToId(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()
	dbResource.UserGroupNameToId("groupname")

	if !wrapper.HasExecuted("SELECT id FROM usergroup WHERE name =") {
		t.Errorf("Expected query not fired")
		t.FailNow()
	}

}

func TestStoreToken(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()
	wrapper.ResetQueries()
	token := oauth2.Token{}

	users, err := dbResource.Cruds["user_account"].GetAllRawObjects("user_account")
	if err != nil {
		t.Errorf("Failed to get users: %v", err)
		t.FailNow()
		return
	}
	user := users[0]
	refIdStr := user["reference_id"].(string)
	refId, _ := uuid.Parse(refIdStr)

	tx, _ := wrapper.Beginx()
	defer tx.Rollback()
	err = dbResource.StoreToken(&token, "type", daptinid.DaptinReferenceId(refId), &auth.SessionUser{
		UserId: daptinid.DaptinReferenceId(refId),
	}, tx)

	if !wrapper.HasExecuted("SELECT * FROM oauth_connect WHERE reference_id =") {
		t.Errorf("Expected query not fired: %v", err)
		t.FailNow()
	}

}

func TestGetIdToObject(t *testing.T) {

	wrapper, dbResource := GetResourceWithName("world")
	defer wrapper.db.Close()

	tx, _ := wrapper.Beginx()
	defer tx.Rollback()
	dbResource.GetIdToObject("world", 1, tx)

	if !wrapper.HasExecuted("SELECT * FROM world WHERE id =") {
		t.Errorf("Expected query not fired")
		t.FailNow()
	}

}

func TestGetActionsByType(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()
	dbResource.GetActionsByType("world", nil)

	if !wrapper.HasExecuted("select a.action_name as name, w.table_name as ontype, a.label, action_schema as action_schema, a.instance_optional as instance_optional, a.reference_id as referenceid from action a join world w on w.id = a.world_id where w.table_name =") {
		t.Errorf("Expected query not fired")
		t.FailNow()
	}

}

func TestPaginatedFindAllWithoutFilters(t *testing.T) {

	wrapper, dbResource := GetResourceWithName("world")
	defer wrapper.db.Close()
	ur, _ := url.Parse("/world/:referenceId")
	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "GET",
			URL:    ur,
		},
		QueryParams: map[string][]string{},
	}

	tx, _ := wrapper.Beginx()
	defer tx.Rollback()
	dbResource.PaginatedFindAllWithoutFilters(req, tx)

	if !wrapper.HasExecuted("SELECT distinct(world.id) from world left join ") {
		t.Errorf("Expected query not fired")
		t.FailNow()
	}

}

func TestPaginatedFindAllWithoutFilter(t *testing.T) {

	wrapper, dbResource := GetResourceWithName("world")
	ur, _ := url.Parse("/world/:referenceId")
	defer wrapper.db.Close()
	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "GET",
			URL:    ur,
		},
		QueryParams: map[string][]string{},
	}

	tx, _ := wrapper.Beginx()
	defer tx.Rollback()
	dbResource.PaginatedFindAllWithoutFilters(req, tx)

	if !wrapper.HasExecuted("SELECT distinct(world.id) FROM world LEFT JOIN world_world_id_") {
		t.Errorf("Expected query not fired")
		t.FailNow()
	}

}

func TestDeleteWithoutFilter(t *testing.T) {

	wrapper, dbResource := GetResourceWithName("world")
	defer wrapper.db.Close()
	ur, _ := url.Parse("/world/:referenceId")
	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "GET",
			URL:    ur,
		},
		QueryParams: map[string][]string{},
	}

	worlds, _ := dbResource.GetAllRawObjects("world")
	log.Printf("%v", worlds[0]["reference_id"])

	refIdStr := worlds[0]["reference_id"].(string)
	refId, _ := uuid.Parse(refIdStr)

	tx, _ := wrapper.Beginx()
	defer tx.Rollback()
	dbResource.DeleteWithoutFilters(daptinid.DaptinReferenceId(refId), req, tx)

	if !wrapper.HasExecuted("DELETE FROM world WHERE reference_id =") {
		t.Errorf("Expected query not fired")
		t.FailNow()
	}

}

func TestGetUserEmailByIdWithTransaction(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(
		"CREATE TABLE user_account (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT, name TEXT, reference_id BLOB, permission INTEGER)",
	)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	refId, _ := uuid.NewV7()
	_, err = db.Exec(
		"INSERT INTO user_account (email, name, reference_id, permission) VALUES (?, ?, ?, ?)",
		"testuser@example.com", "Test User", refId[:], 0,
	)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	// Create a minimal DbResource with just db access
	dbResource := &resource.DbResource{}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	email := dbResource.GetUserEmailByIdWithTransaction(1, tx)

	if email != "testuser@example.com" {
		t.Errorf("Expected testuser@example.com, got: %s", email)
	}
}

func TestGetUserEmailByIdWithTransaction_NonExistentUser(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(
		"CREATE TABLE user_account (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT, name TEXT, reference_id BLOB, permission INTEGER)",
	)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	dbResource := &resource.DbResource{}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	email := dbResource.GetUserEmailByIdWithTransaction(999999, tx)

	if email != "" {
		t.Errorf("Expected empty email for non-existent user, got: %s", email)
	}
}
