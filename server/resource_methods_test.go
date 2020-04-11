package server

import (
	"database/sql"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"strings"
	"testing"
)

func GetDb() *InMemoryTestDatabase {
	db, err := sqlx.Open("sqlite3", "daptin_test.db")
	if err != nil {
		panic(err)
	}

	wrapper := NewInMemoryTestDatabase(db)
	return wrapper

}

func GetResource() (*InMemoryTestDatabase, *resource.DbResource) {
	wrapper := GetDb()

	configStore, _ := resource.NewConfigStore(wrapper)

	initConfig, _ := LoadConfigFiles()

	existingTables, _ := GetTablesFromWorld(wrapper)
	//initConfig.Tables = append(initConfig.Tables, existingTables...)

	allTables := MergeTables(existingTables, initConfig.Tables)

	initConfig.Tables = allTables

	cruds := make(map[string]*resource.DbResource)

	ms := BuildMiddlewareSet(&initConfig, &cruds)
	for _, table := range initConfig.Tables {
		model := api2go.NewApi2GoModel(table.TableName, table.Columns, int64(table.DefaultPermission), table.Relations)
		res := resource.NewDbResource(model, wrapper, &ms, cruds, configStore, table)
		cruds[table.TableName] = res
	}

	resource.CheckRelations(&initConfig)
	resource.CheckAuditTables(&initConfig)
	//AddStateMachines(&initConfig, wrapper)

	tx, errb := wrapper.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction")
	resource.CheckAllTableStatus(&initConfig, wrapper, tx)
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

	err := resource.UpdateActionTable(&initConfig, wrapper)
	resource.CheckErr(err, "Failed to update action table")

	for _, table := range initConfig.Tables {
		model := api2go.NewApi2GoModel(table.TableName, table.Columns, int64(table.DefaultPermission), table.Relations)
		res := resource.NewDbResource(model, wrapper, &ms, cruds, configStore, table)
		cruds[table.TableName] = res
	}

	dbResource := resource.NewDbResource(nil, wrapper, &ms, cruds, configStore, resource.TableInfo{})
	return wrapper, dbResource
}

func GetResourceWithName(name string) (*InMemoryTestDatabase, *resource.DbResource) {
	wrapper := GetDb()

	var cols []api2go.ColumnInfo
	model := api2go.NewApi2GoModel(name, cols, 0, nil)
	tableInfo := resource.TableInfo{
		TableName: name,
	}

	cruds := make(map[string]*resource.DbResource)
	dbResource := resource.NewDbResource(model, wrapper, nil, cruds, nil, tableInfo)
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

func (imtd *InMemoryTestDatabase) HasExecuted(query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))

	for _, qu := range imtd.queries {
		q := strings.ToLower(qu)
		if BeginsWithCheck(q, query) {
			return true
		}
	}

	log.Printf("%v", strings.Join(imtd.queries, "\n"))

	return false
}

func (imtd *InMemoryTestDatabase) HasExecutedAll(queries ...string) bool {

	executedAll := true

	for _, query := range queries {
		found := false
		query = strings.TrimSpace(query)

		for _, qu := range imtd.queries {
			if strings.TrimSpace(qu) == query {
				found = true
				break
			}
		}

		if !found {
			log.Printf("%v", strings.Join(imtd.queries, "\n"))
			executedAll = false
			break
		}

	}

	return executedAll
}

func NewInMemoryTestDatabase(db *sqlx.DB) *InMemoryTestDatabase {
	return &InMemoryTestDatabase{
		db: db,
	}
}

func (imtd *InMemoryTestDatabase) GetQueries() []string {
	return imtd.queries
}

type InMemoryTestDatabase struct {
	db      *sqlx.DB
	rowx    *sqlx.Row
	row     *sql.Row
	stmt    *sqlx.Stmt
	tx      *sqlx.Tx
	result  sql.Result
	queries []string
}

func (imtd *InMemoryTestDatabase) DriverName() string {
	return "sqlite3"
}

func (imtd *InMemoryTestDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {

	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	res, err := imtd.db.Exec(query, args...)
	return &InMemoryTestDatabase{
		result: res,
	}, err
}
func (imtd *InMemoryTestDatabase) LastInsertId() (int64, error) {
	return imtd.result.LastInsertId()
}
func (imtd *InMemoryTestDatabase) RowsAffected() (int64, error) {
	return imtd.result.RowsAffected()
}

func (imtd *InMemoryTestDatabase) Prepare(query string) (*sql.Stmt, error) {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Prepare(query)

}

func (imtd *InMemoryTestDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Query(query, args...)
}
func (imtd *InMemoryTestDatabase) Queryx(query string, args ...interface{}) (*sqlx.Rows, error) {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)

	return imtd.db.Queryx(query, args...)

}
func (imtd *InMemoryTestDatabase) QueryRowx(query string, args ...interface{}) *sqlx.Row {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.QueryRowx(query, args...)
}

func (imtd *InMemoryTestDatabase) Rebind(query string) string {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Rebind(query)
}
func (imtd *InMemoryTestDatabase) BindNamed(query string, args interface{}) (string, []interface{}, error) {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.BindNamed(query, args)

}

func (imtd *InMemoryTestDatabase) Select(dest interface{}, query string, args ...interface{}) error {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Select(dest, query, args...)

}

func (imtd *InMemoryTestDatabase) Get(dest interface{}, query string, args ...interface{}) error {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Get(dest, query, args...)

}

func (imtd *InMemoryTestDatabase) MustBegin() *sqlx.Tx {
	return imtd.db.MustBegin()

}

func (imtd *InMemoryTestDatabase) Preparex(query string) (*sqlx.Stmt, error) {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Preparex(query)

}

func (imtd *InMemoryTestDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.QueryRow(query, args...)
}

func (imtd *InMemoryTestDatabase) Beginx() (*sqlx.Tx, error) {
	return imtd.db.Beginx()
}
func (database *InMemoryTestDatabase) ResetQueries() {
	database.queries = make([]string, 0)
}
