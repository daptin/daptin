package server

import (
	"github.com/daptin/daptin/server/actionresponse"
	"os"
	"path/filepath"
	"strings"

	"github.com/artpar/api2go"
	"github.com/artpar/conform"
	"github.com/artpar/ydb"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/daptin/daptin/server/table_info"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func EndsWithCheck(str string, endsWith string) bool {
	if len(endsWith) > len(str) {
		return false
	}

	if len(endsWith) == len(str) && endsWith != str {
		return false
	}

	suffix := str[len(str)-len(endsWith):]
	i := suffix == endsWith
	return i

}

func BeginsWithCheck(str string, beginsWith string) bool {
	if len(beginsWith) > len(str) {
		return false
	}

	if len(beginsWith) == len(str) && beginsWith != str {
		return false
	}

	prefix := str[:len(beginsWith)]
	i := prefix == beginsWith
	//log.Printf("Check [%v] begins with [%v]: %v", str, beginsWith, i)
	return i

}

func CheckErr(err error, message ...interface{}) bool {

	if err != nil {
		fmtString := message[0].(string)
		args := make([]interface{}, 0)
		if len(message) > 1 {
			args = message[1:]
		}
		args = append(args, err)
		log.Errorf(fmtString+": %v", args...)
		return true
	}
	return false
}

func CheckSystemSecrets(store *resource.ConfigStore, transaction *sqlx.Tx) error {
	jwtSecret, err := store.GetConfigValueFor("jwt.secret", "backend", transaction)
	if err != nil {
		u, _ := uuid.NewV7()
		jwtSecret = u.String()
		err = store.SetConfigValueFor("jwt.secret", jwtSecret, "backend", transaction)
		CheckErr(err, "Failed to store jwt secret")
	}

	encryptionSecret, err := store.GetConfigValueFor("encryption.secret", "backend", transaction)

	if err != nil || len(encryptionSecret) < 10 {
		u, _ := uuid.NewV7()
		newSecret := strings.Replace(u.String(), "-", "", -1)
		err = store.SetConfigValueFor("encryption.secret", newSecret, "backend", transaction)
	}
	return err

}

func AddResourcesToApi2Go(api *api2go.API, tables []table_info.TableInfo, db database.DatabaseConnection, ms *resource.MiddlewareSet, configStore *resource.ConfigStore, olricDb *olric.EmbeddedClient, cruds map[string]*resource.DbResource) {
	for _, table := range tables {

		if table.TableName == "" {
			log.Errorf("Table name is empty, not adding to JSON API, as it will create conflict: %v", table)
			continue
		}

		model := api2go.NewApi2GoModel(table.TableName, table.Columns, int64(table.DefaultPermission), table.Relations)

		res, err := resource.NewDbResource(model, db, ms, cruds, configStore, olricDb, table)
		if err != nil {
			panic(err)
		}

		cruds[table.TableName] = res

		//if table.IsJoinTable {
		//	we do expose join table as web api
		//continue
		//}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Recovered in adding routes for table [%v]", table.TableName)
					log.Errorf("Error was: %v", r)
				}
			}()
			api.AddResource(model, res)
		}()
	}

}

func GetTablesFromWorld(db database.DatabaseConnection) ([]table_info.TableInfo, error) {

	ts := make([]table_info.TableInfo, 0)

	sql, args, err := statementbuilder.Squirrel.
		Select("table_name", "permission", "default_permission",
			"world_schema_json", "is_top_level", "is_hidden", "is_state_tracking_enabled", "default_order", "icon",
		).Prepared(true).
		From("world").
		Where(goqu.Ex{
			"table_name": goqu.Op{
				"notlike": "%_has_%",
			},
		}).
		Where(goqu.Ex{
			"table_name": goqu.Op{
				"notlike": "%_audit",
			},
		}).
		Where(goqu.Ex{
			"table_name": goqu.Op{
				"notin": []string{"world", "action", "usergroup"},
			},
		}).
		ToSQL()
	if err != nil {
		return nil, err
	}

	stmt1, err := db.Preparex(sql)
	if err != nil {
		log.Errorf("[106] failed to prepare statment: %v", err)
		return nil, err
	}

	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	res, err := stmt1.Queryx(args...)
	if err != nil {
		log.Printf("Failed to select from world table: %v", err)
		return ts, err
	}
	defer res.Close()
	for res.Next() {
		var table_name string
		var permission int64
		var default_permission int64
		var world_schema_json string
		var default_order string
		var icon string
		var is_top_level bool
		var is_hidden bool
		var is_state_tracking_enabled bool

		err = res.Scan(&table_name, &permission, &default_permission, &world_schema_json, &is_top_level, &is_hidden, &is_state_tracking_enabled, &default_order, &icon)
		if err != nil {
			log.Errorf("Failed to scan json schema from world: %v", err)
			continue
		}

		var t table_info.TableInfo

		err = json.Unmarshal([]byte(world_schema_json), &t)

		for i, col := range t.Columns {
			if col.Name == "" && col.ColumnName != "" {
				col.Name = col.ColumnName
			} else if col.Name != "" && col.ColumnName == "" {
				col.ColumnName = col.Name
			} else if col.Name == "" && col.ColumnName == "" {
				log.Printf("Error, column without name in existing tables: %v", t)
			}
			t.Columns[i] = col
		}

		if err != nil {
			log.Errorf("Failed to unmarshal json schema: %v", err)
			continue
		}

		t.TableName = table_name
		t.Permission = auth.AuthPermission(permission)
		t.DefaultPermission = auth.AuthPermission(default_permission)
		t.IsHidden = is_hidden
		t.IsTopLevel = is_top_level
		t.Icon = icon
		t.IsStateTrackingEnabled = is_state_tracking_enabled
		t.DefaultOrder = default_order
		ts = append(ts, t)

	}

	log.Printf("Loaded %d tables from world table", len(ts))

	return ts, nil

}

func BuildMiddlewareSet(cmsConfig *resource.CmsConfig,
	cruds *map[string]*resource.DbResource,
	documentProvider ydb.DocumentProvider,
	dtopicMap *map[string]*olric.PubSub) resource.MiddlewareSet {

	var ms resource.MiddlewareSet

	exchangeMiddleware := resource.NewExchangeMiddleware(cmsConfig, cruds)

	tablePermissionChecker := &resource.TableAccessPermissionChecker{}
	objectPermissionChecker := &resource.ObjectAccessPermissionChecker{}
	dataValidationMiddleware := resource.NewDataValidationMiddleware(cmsConfig, cruds)

	createEventHandler := resource.NewCreateEventHandler(cruds, dtopicMap)
	updateEventHandler := resource.NewUpdateEventHandler(cruds, dtopicMap)
	deleteEventHandler := resource.NewDeleteEventHandler(cruds, dtopicMap)

	var yhsHandler resource.DatabaseRequestInterceptor
	yhsHandler = nil

	if documentProvider != nil {
		yhsHandler = resource.NewYJSHandlerMiddleware(documentProvider)
	}

	ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		exchangeMiddleware,
		objectPermissionChecker,
	}

	ms.AfterFindAll = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		exchangeMiddleware,
		objectPermissionChecker,
	}

	ms.BeforeCreate = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		dataValidationMiddleware,
		createEventHandler,
		exchangeMiddleware,
	}
	ms.AfterCreate = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		createEventHandler,
		exchangeMiddleware,
	}

	ms.BeforeDelete = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		deleteEventHandler,
		exchangeMiddleware,
	}
	ms.AfterDelete = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		deleteEventHandler,
		exchangeMiddleware,
	}

	if yhsHandler != nil {
		ms.BeforeUpdate = []resource.DatabaseRequestInterceptor{
			tablePermissionChecker,
			objectPermissionChecker,
			dataValidationMiddleware,
			yhsHandler,
			updateEventHandler,
			exchangeMiddleware,
		}
	} else {
		ms.BeforeUpdate = []resource.DatabaseRequestInterceptor{
			tablePermissionChecker,
			objectPermissionChecker,
			dataValidationMiddleware,
			updateEventHandler,
			exchangeMiddleware,
		}
	}

	ms.AfterUpdate = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		updateEventHandler,
		exchangeMiddleware,
	}

	ms.BeforeFindOne = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		exchangeMiddleware,
	}
	ms.AfterFindOne = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		exchangeMiddleware,
	}
	return ms
}

func CleanUpConfigFiles() {

	files, _ := filepath.Glob("*_uploaded_*")
	log.Debugf("Clean up uploaded config files: %v", files)

	for _, fileName := range files {
		err := os.Remove(fileName)
		CheckErr(err, "Failed to delete uploaded schema file: %s", fileName)
	}

	schemaFolderDefinedByEnv, _ := os.LookupEnv("DAPTIN_SCHEMA_FOLDER")
	files, _ = filepath.Glob(schemaFolderDefinedByEnv + string(os.PathSeparator) + "*_uploaded_*")

	for _, fileName := range files {
		err := os.Remove(fileName)
		log.Infof("Deleted config files: %v", fileName)
		CheckErr(err, "Failed to delete uploaded schema file: %s", fileName)
	}

}

func EndsWith(str string, endsWith string) (string, bool) {
	if len(endsWith) > len(str) {
		return "", false
	}

	if len(endsWith) == len(str) && endsWith != str {
		return "", false
	}

	suffix := str[len(str)-len(endsWith):]
	prefix := str[:len(str)-len(endsWith)]
	i := suffix == endsWith
	return prefix, i

}

func SmallSnakeCaseText(str string) string {
	transformed := conform.TransformString(str, "lower,snake")
	return transformed
}

func ActionPerformersListToMap(interfaces []actionresponse.ActionPerformerInterface) map[string]actionresponse.ActionPerformerInterface {
	m := make(map[string]actionresponse.ActionPerformerInterface)

	for _, api := range interfaces {
		if api == nil {
			continue
		}
		m[api.Name()] = api
	}
	return m
}

func AddStreamsToApi2Go(api *api2go.API, processors []*resource.StreamProcessor, db database.DatabaseConnection,
	middlewareSet *resource.MiddlewareSet, configStore *resource.ConfigStore) {

	for _, processor := range processors {

		contract := processor.GetContract()
		model := api2go.NewApi2GoModel(contract.StreamName, contract.Columns, 0, nil)
		api.AddResource(model, processor)

	}

}

func GetStreamProcessors(config *resource.CmsConfig, store *resource.ConfigStore,
	cruds map[string]*resource.DbResource) []*resource.StreamProcessor {

	allProcessors := make([]*resource.StreamProcessor, 0)

	for _, streamContract := range config.Streams {

		streamProcessor := resource.NewStreamProcessor(streamContract, cruds)
		allProcessors = append(allProcessors, streamProcessor)

	}

	return allProcessors

}
