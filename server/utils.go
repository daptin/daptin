package server

import (
	"github.com/artpar/api2go"
	"github.com/artpar/go.uuid"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

func CheckSystemSecrets(store *resource.ConfigStore) error {
	jwtSecret, err := store.GetConfigValueFor("jwt.secret", "backend")
	if err != nil {
		u, _ := uuid.NewV4()
		jwtSecret = u.String()
		err = store.SetConfigValueFor("jwt.secret", jwtSecret, "backend")
		resource.CheckErr(err, "Failed to store jwt secret")
	}

	encryptionSecret, err := store.GetConfigValueFor("encryption.secret", "backend")

	if err != nil || len(encryptionSecret) < 10 {
		u, _ := uuid.NewV4()
		newSecret := strings.Replace(u.String(), "-", "", -1)
		err = store.SetConfigValueFor("encryption.secret", newSecret, "backend")
	}
	return err

}

func InArrayIndex(val interface{}, array interface{}) (index int) {
	index = -1

	switch reflect.TypeOf(array).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(array)

		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(val, s.Index(i).Interface()) == true {
				index = i
				return
			}
		}
	}

	return
}

func AddResourcesToApi2Go(api *api2go.API, tables []resource.TableInfo, db database.DatabaseConnection, ms *resource.MiddlewareSet, configStore *resource.ConfigStore, cruds map[string]*resource.DbResource) map[string]*resource.DbResource {
	for _, table := range tables {

		if table.TableName == "" {
			log.Errorf("Table name is empty, not adding to JSON API, as it will create conflict: %v", table)
			continue
		}

		model := api2go.NewApi2GoModel(table.TableName, table.Columns, int64(table.DefaultPermission), table.Relations)

		res := resource.NewDbResource(model, db, ms, cruds, configStore, table)

		cruds[table.TableName] = res

		if table.IsJoinTable {
			// we do not expose join table as web api
			continue
		}

		//if table.IsJoinTable {
		//	continue
		//}

		//log.Infof("Add Resources To Api2Go: %v", table.TableName)

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered in adding routes for table [%v]", table.TableName)
					log.Printf("Error was: %v", r)
				}
			}()
			api.AddResource(model, res)
		}()
	}

	return cruds
}

func GetTablesFromWorld(db database.DatabaseConnection) ([]resource.TableInfo, error) {

	ts := make([]resource.TableInfo, 0)

	sql, args, err := statementbuilder.Squirrel.Select("table_name", "permission", "default_permission",
		"world_schema_json", "is_top_level", "is_hidden", "is_state_tracking_enabled", "default_order",
	).From("world").Where("table_name not like '%_has_%'").Where("table_name not like '%_audit'").Where("table_name not in (?,?,?)",
		"world", "action", "usergroup").ToSql()
	if err != nil {
		return nil, err
	}

	res, err := db.Queryx(sql, args...)
	if err != nil {
		log.Infof("Failed to select from world table: %v", err)
		return ts, err
	}
	defer res.Close()

	for res.Next() {
		var table_name string
		var permission int64
		var default_permission int64
		var world_schema_json string
		var default_order *string
		var is_top_level bool
		var is_hidden bool
		var is_state_tracking_enabled bool

		err = res.Scan(&table_name, &permission, &default_permission, &world_schema_json, &is_top_level, &is_hidden, &is_state_tracking_enabled, &default_order)
		if err != nil {
			log.Errorf("Failed to scan json schema from world: %v", err)
			continue
		}

		var t resource.TableInfo

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
		t.IsStateTrackingEnabled = is_state_tracking_enabled
		if default_order != nil {
			t.DefaultOrder = *default_order
		}
		ts = append(ts, t)

	}

	log.Infof("Loaded %d tables from world table", len(ts))

	return ts, nil

}

func BuildMiddlewareSet(cmsConfig *resource.CmsConfig, cruds *map[string]*resource.DbResource) resource.MiddlewareSet {

	var ms resource.MiddlewareSet

	exchangeMiddleware := resource.NewExchangeMiddleware(cmsConfig, cruds)

	tablePermissionChecker := &resource.TableAccessPermissionChecker{}
	objectPermissionChecker := &resource.ObjectAccessPermissionChecker{}
	dataValidationMiddleware := resource.NewDataValidationMiddleware(cmsConfig, cruds)

	findOneHandler := resource.NewFindOneEventHandler()
	createEventHandler := resource.NewCreateEventHandler()
	updateEventHandler := resource.NewUpdateEventHandler()
	deleteEventHandler := resource.NewDeleteEventHandler()

	ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
	}

	ms.AfterFindAll = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
	}

	ms.BeforeCreate = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		dataValidationMiddleware,
		createEventHandler,
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
	}
	ms.AfterDelete = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		deleteEventHandler,
	}

	ms.BeforeUpdate = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		dataValidationMiddleware,
		updateEventHandler,
	}
	ms.AfterUpdate = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		updateEventHandler,
	}

	ms.BeforeFindOne = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		findOneHandler,
	}
	ms.AfterFindOne = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		findOneHandler,
	}
	return ms
}

func CleanUpConfigFiles() {

	files, _ := filepath.Glob("schema_uploaded_*_daptin.*")
	log.Infof("Clean up config files: %v", files)

	for _, fileName := range files {
		err := os.Remove(fileName)
		resource.CheckErr(err, "Failed to delete uploaded schema file: %s", fileName)
	}

}
