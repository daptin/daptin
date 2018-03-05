package resource

import (
	"database/sql"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"strings"
	"testing"
)

func GetDb() *InMemoryTestDatabase {

	db, err := sqlx.Open("sqlite3", "test.db")
	if err != nil {
		panic(err)
	}

	wrapper := NewInMemoryTestDatabase(db)
	return wrapper

}

func GetResource() (*InMemoryTestDatabase, *DbResource) {
	wrapper := GetDb()

	configStore, _ := NewConfigStore(wrapper)

	initConfig, _ := server.LoadConfigFiles()

	existingTables, _ := server.GetTablesFromWorld(wrapper)
	//initConfig.Tables = append(initConfig.Tables, existingTables...)

	allTables := server.MergeTables(existingTables, initConfig.Tables)

	initConfig.Tables = allTables

	cruds := make(map[string]*DbResource)

	ms := server.BuildMiddlewareSet(&initConfig, cruds)
	for _, table := range initConfig.Tables {
		model := api2go.NewApi2GoModel(table.TableName, table.Columns, table.DefaultPermission, table.Relations)
		res := NewDbResource(model, wrapper, &ms, cruds, configStore, &table)
		cruds[table.TableName] = res
	}

	CheckRelations(&initConfig)
	CheckAuditTables(&initConfig)
	//AddStateMachines(&initConfig, wrapper)
	tx, errb := wrapper.Beginx()
	//_, errb := db.Exec("begin")
	CheckErr(errb, "Failed to begin transaction")

	CheckAllTableStatus(&initConfig, wrapper, tx)
	CreateRelations(&initConfig, tx)
	CreateUniqueConstraints(&initConfig, tx)
	CreateIndexes(&initConfig, tx)
	UpdateWorldTable(&initConfig, tx)
	UpdateWorldColumnTable(&initConfig, tx)
	errc := tx.Commit()
	CheckErr(errc, "Failed to commit transaction")

	UpdateStateMachineDescriptions(&initConfig, wrapper)
	UpdateExchanges(&initConfig, wrapper)
	UpdateStreams(&initConfig, wrapper)
	UpdateMarketplaces(&initConfig, wrapper)
	UpdateStandardData(&initConfig, wrapper)

	err := UpdateActionTable(&initConfig, wrapper)
	CheckErr(err, "Failed to update action table")

	dbResource := NewDbResource(nil, wrapper, &ms, cruds, configStore, &TableInfo{})
	return wrapper, dbResource
}
func GetResourceWithName(name string) (*InMemoryTestDatabase, *DbResource) {
	wrapper := GetDb()

	cols := []api2go.ColumnInfo{}
	model := api2go.NewApi2GoModel(name, cols, 0, nil)
	tableInfo := &TableInfo{
		TableName: name,
	}
	dbResource := NewDbResource(model, wrapper, nil, nil, nil, tableInfo)
	return wrapper, dbResource
}

func TestGetReferenceIdToObject(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()
	dbResource.GetReferenceIdToObject("todo", "refId")

	if !wrapper.HasExecuted("SELECT * FROM todo WHERE reference_id = ?") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestUserGroupNameToId(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()
	dbResource.UserGroupNameToId("groupname")

	if !wrapper.HasExecuted("SELECT id FROM usergroup WHERE name = ?") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestStoreToken(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()
	wrapper.ResetQueries()
	token := oauth2.Token{}
	dbResource.StoreToken(&token, "type", "ref_id")

	if !wrapper.HasExecuted("SELECT * FROM oauth_connect WHERE reference_id = ?") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

	if !wrapper.HasExecuted("SELECT value FROM _config WHERE name = ? AND configstate = ? AND configenv = ? AND configtype = ?") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestGetIdToObject(t *testing.T) {

	wrapper, dbResource := GetResourceWithName("todo")
	defer wrapper.db.Close()
	dbResource.GetIdToObject("todo", 1)

	if !wrapper.HasExecuted("SELECT * FROM todo WHERE id = ?") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestGetActionsByType(t *testing.T) {

	wrapper, dbResource := GetResource()
	defer wrapper.db.Close()
	dbResource.GetActionsByType("todo")

	if !wrapper.HasExecuted("select a.action_name as name, w.table_name as ontype, a.label, action_schema as action_schema, a.instance_optional as instance_optional, a.reference_id as referenceid from action a join world w on w.id = a.world_id where w.table_name = ?") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestPaginatedFindAllWithoutFilters(t *testing.T) {

	wrapper, dbResource := GetResourceWithName("todo")
	defer wrapper.db.Close()
	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "GET",
		},
		QueryParams: map[string][]string{},
	}

	dbResource.PaginatedFindAllWithoutFilters(req)

	if !wrapper.HasExecuted("SELECT todo.permission, todo.reference_id FROM todo LIMIT 10 OFFSET 0") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func TestCreateWithoutFilter(t *testing.T) {

	wrapper, dbResource := GetResourceWithName("todo")
	defer wrapper.db.Close()
	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "GET",
		},
		QueryParams: map[string][]string{},
	}

	data := map[string]interface{}{}
	obj := api2go.NewApi2GoModelWithData("todo", nil, 0, nil, data)
	dbResource.CreateWithoutFilter(obj, req)

	if !wrapper.HasExecuted("INSERT INTO todo (reference_id,permission,created_at) VALUES (?,?,?)") {
		t.Errorf("Expected query not fired")
		t.Fail()
	}

}

func (imtd *InMemoryTestDatabase) HasExecuted(query string) bool {
	query = strings.TrimSpace(query)

	for _, qu := range imtd.queries {
		if strings.TrimSpace(qu) == query {
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
