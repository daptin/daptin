//+build test

package server

import (
	"github.com/artpar/api2go"
	"github.com/artpar/ydb"
	"github.com/buraksezer/olric"
	olricConfig "github.com/buraksezer/olric/config"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"net/http"
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
	//initConfig.Tables = append(initConfig.Tables, existingTables...)

	allTables := MergeTables(existingTables, initConfig.Tables)

	initConfig.Tables = allTables

	cruds := make(map[string]*resource.DbResource)

	olricDb, _ := olric.New(olricConfig.New("local"))

	dtopicMap := make(map[string]*olric.DTopic)

	documentProvider := ydb.NewDiskDocumentProvider("/tmp", 10000, ydb.DocumentListener{
		GetDocumentInitialContent: func(string) []byte {
			return []byte{}
		},
		SetDocumentInitialContent: func(string, []byte) {},
	})
	ms := BuildMiddlewareSet(&initConfig, &cruds, documentProvider, &dtopicMap)
	for _, table := range initConfig.Tables {
		model := api2go.NewApi2GoModel(table.TableName, table.Columns, int64(table.DefaultPermission), table.Relations)
		res := resource.NewDbResource(model, wrapper, &ms, cruds, configStore, olricDb, table)
		cruds[table.TableName] = res
	}

	var err error
	for key, crud := range cruds {
		dtopicMap[key], err = crud.OlricDb.NewDTopic(key, 4, 1)
		resource.CheckErr(err, "Failed to create topic for table: %v", key)
		err = nil
	}

	resource.CheckRelations(&initConfig)
	resource.CheckAuditTables(&initConfig)
	//AddStateMachines(&initConfig, wrapper)

	tx, errb := wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	resource.CheckAllTableStatus(&initConfig, wrapper)
	errc := tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating tables")

	tx, errb = wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	resource.CreateRelations(&initConfig, tx)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating relations")

	tx, errb = wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	resource.CreateUniqueConstraints(&initConfig, tx)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating unique constrains")

	tx, errb = wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	resource.CreateIndexes(&initConfig, wrapper)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after creating indexes")

	tx, errb = wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	resource.UpdateWorldTable(&initConfig, tx)
	errc = tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction after updating world tables")

	resource.UpdateStateMachineDescriptions(&initConfig, wrapper)
	resource.UpdateExchanges(&initConfig, wrapper)
	resource.UpdateStreams(&initConfig, wrapper)
	//resource.UpdateMarketplaces(&initConfig, wrapper)
	resource.UpdateStandardData(&initConfig, wrapper)

	err = resource.UpdateActionTable(&initConfig, wrapper)
	resource.CheckErr(err, "Failed to update action table")

	//for _, table := range initConfig.Tables {
	//	model := api2go.NewApi2GoModel(table.TableName, table.Columns, int64(table.DefaultPermission), table.Relations)
	//	res := resource.NewDbResource(model, wrapper, &ms, cruds, configStore, nil, table)
	//	cruds[table.TableName] = res
	//}

	dbResource := resource.NewDbResource(nil, wrapper, &ms, cruds, configStore, nil, resource.TableInfo{})
	return wrapper, dbResource
}

func GetResourceWithName(name string) (*InMemoryTestDatabase, *resource.DbResource) {
	wrapper := GetInMemoryDbForTest()

	var cols []api2go.ColumnInfo
	model := api2go.NewApi2GoModel(name, cols, 0, nil)
	tableInfo := resource.TableInfo{
		TableName: name,
	}

	cruds := make(map[string]*resource.DbResource)
	dbResource := resource.NewDbResource(model, wrapper, nil, cruds, nil, nil, tableInfo)
	cruds[name] = dbResource
	return wrapper, dbResource
}

func TestGetReferenceIdToObject(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()
	dbResource.Cruds["world"].GetReferenceIdToObject("world", "refId")

	if !wrapper.HasExecuted("SELECT * FROM world WHERE reference_id =") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestUserGroupNameToId(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()
	dbResource.UserGroupNameToId("groupname")

	if !wrapper.HasExecuted("SELECT id FROM usergroup WHERE name =") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestStoreToken(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()
	wrapper.ResetQueries()
	token := oauth2.Token{}

	//newUser := map[string]interface{}{
	//	"email":    "test@gmail.com",
	//	"password": "test",
	//	"name":     "test",
	//}

	//userModel := api2go.NewApi2GoModelWithData("user_account", nil, 0, nil, newUser)
	//httpRequest := &http.Request{
	//
	//}

	//ctx := context.Background()
	//sessionUser := &auth.SessionUser{
	//
	//}
	//httpRequest = httpRequest.WithContext(context.WithValue(ctx, "user", sessionUser))
	//apiRequest := api2go.Request{
	//	PlainRequest: httpRequest,
	//}

	//userResponse, err := dbResource.Cruds["user_account"].CreateWithoutFilter(userModel, apiRequest)
	//log.Printf("New user: %v", userResponse)

	users, err := dbResource.Cruds["user_account"].GetAllRawObjects("user_account")
	if err != nil {
		t.Errorf("Failed to get users: %v", err)
		t.Fail()
		return
	}
	user := users[0]
	err = dbResource.StoreToken(&token, "type", "ref_id", user["reference_id"].(string))

	if !wrapper.HasExecuted("SELECT * FROM oauth_connect WHERE reference_id =") {
		t.Errorf("Expected query not fired: %v", err)
		t.Fail()
	}

	//if !wrapper.HasExecuted("SELECT value FROM _config WHERE name = ? AND configstate = ? AND configenv = ? AND configtype = ?") {
	//	t.Errorf("Expected query not fired")
	//	t.Fail()
	//}

}

func TestGetIdToObject(t *testing.T) {

	wrapper, dbResource := GetResourceWithName("world")
	defer wrapper.db.Close()
	dbResource.GetIdToObject("world", 1)

	if !wrapper.HasExecuted("SELECT * FROM world WHERE id =") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestGetActionsByType(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()
	dbResource.GetActionsByType("world")

	if !wrapper.HasExecuted("select a.action_name as name, w.table_name as ontype, a.label, action_schema as action_schema, a.instance_optional as instance_optional, a.reference_id as referenceid from action a join world w on w.id = a.world_id where w.table_name =") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestPaginatedFindAllWithoutFilters(t *testing.T) {

	wrapper, dbResource := GetResourceWithName("world")
	defer wrapper.db.Close()
	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "GET",
		},
		QueryParams: map[string][]string{},
	}

	dbResource.PaginatedFindAllWithoutFilters(req)

	if !wrapper.HasExecuted("SELECT distinct(world.id) from world left join ") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestPaginatedFindAllWithoutFilter(t *testing.T) {

	wrapper, dbResource := GetResourceWithName("world")
	defer wrapper.db.Close()
	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "GET",
		},
		QueryParams: map[string][]string{},
	}
	dbResource.PaginatedFindAllWithoutFilters(req)

	if !wrapper.HasExecuted("SELECT distinct(world.id) FROM world LEFT JOIN world_world_id_") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestDeleteWithoutFilter(t *testing.T) {

	wrapper, dbResource := GetResourceWithName("world")
	defer wrapper.db.Close()
	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "GET",
		},
		QueryParams: map[string][]string{},
	}

	worlds, _ := dbResource.GetAllRawObjects("world")
	log.Printf("%v", worlds[0]["reference_id"])

	dbResource.DeleteWithoutFilters(worlds[0]["reference_id"].(string), req)

	if !wrapper.HasExecuted("DELETE FROM world WHERE reference_id =") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}
