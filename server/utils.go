package server

import (
	"github.com/jmoiron/sqlx"
	"github.com/artpar/goms/server/resource"
	log "github.com/sirupsen/logrus"
	"encoding/json"
	"path/filepath"
	"os"
	"github.com/artpar/api2go"
	"strings"
	"github.com/satori/go.uuid"
)

func CheckSystemSecrets(store *resource.ConfigStore) error {
	jwtSecret, err := store.GetConfigValueFor("jwt.secret", "backend")
	if err != nil {
		jwtSecret = uuid.NewV4().String()
		err = store.SetConfigValueFor("jwt.secret", jwtSecret, "backend")
		resource.CheckErr(err, "Failed to store jwt secret")
	}

	encryptionSecret, err := store.GetConfigValueFor("encryption.secret", "backend")

	if err != nil || len(encryptionSecret) < 10 {

		newSecret := strings.Replace(uuid.NewV4().String(), "-", "", -1)
		err = store.SetConfigValueFor("encryption.secret", newSecret, "backend")
	}
	return err

}

func AddResourcesToApi2Go(api *api2go.API, tables []resource.TableInfo, db *sqlx.DB, ms *resource.MiddlewareSet, configStore *resource.ConfigStore) map[string]*resource.DbResource {
	cruds = make(map[string]*resource.DbResource)
	for _, table := range tables {
		log.Infof("Table [%v] Relations: %v", table.TableName)
		for _, r := range table.Relations {
			log.Infof("Relation :: %v", r.String())
		}
		model := api2go.NewApi2GoModel(table.TableName, table.Columns, table.DefaultPermission, table.Relations)

		res := resource.NewDbResource(model, db, ms, cruds, configStore)

		cruds[table.TableName] = res
		api.AddResource(model, res)
	}
	return cruds
}

func GetTablesFromWorld(db *sqlx.DB) ([]resource.TableInfo, error) {

	ts := make([]resource.TableInfo, 0)

	res, err := db.Queryx("select table_name, permission, default_permission, schema_json, is_top_level, is_hidden, is_state_tracking_enabled" +
			" from world where deleted_at is null and table_name not like '%_has_%' and table_name not in ('world', 'world_column', 'action', 'user', 'usergroup')")
	if err != nil {
		log.Infof("Failed to select from world table: %v", err)
		return ts, err
	}

	for res.Next() {
		var table_name string
		var permission int64
		var default_permission int64
		var schema_json string
		var is_top_level bool
		var is_hidden bool
		var is_state_tracking_enabled bool

		err = res.Scan(&table_name, &permission, &default_permission, &schema_json, &is_top_level, &is_hidden, &is_state_tracking_enabled)
		if err != nil {
			log.Errorf("Failed to scan json schema from world: %v", err)
			continue
		}

		var t resource.TableInfo

		err = json.Unmarshal([]byte(schema_json), &t)
		if err != nil {
			log.Errorf("Failed to unmarshal json schema: %v", err)
			continue
		}

		t.TableName = table_name
		t.Permission = permission
		t.DefaultPermission = default_permission
		t.IsHidden = is_hidden
		t.IsTopLevel = is_top_level
		t.IsStateTrackingEnabled = is_state_tracking_enabled
		ts = append(ts, t)

	}

	log.Infof("Loaded %d tables from world table", len(ts))

	return ts, nil

}

func BuildMiddlewareSet(cmsConfig *resource.CmsConfig) resource.MiddlewareSet {

	var ms resource.MiddlewareSet

	exchangeMiddleware := resource.NewExchangeMiddleware(cmsConfig, &cruds)

	tablePermissionChecker := &resource.TableAccessPermissionChecker{}
	objectPermissionChecker := &resource.ObjectAccessPermissionChecker{}
	dataValidationMiddleware := resource.NewDataValidationMiddleware(cmsConfig, &cruds)

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

	ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		findOneHandler,
	}
	ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{
		tablePermissionChecker,
		objectPermissionChecker,
		findOneHandler,
	}
	return ms
}

func CleanUpConfigFiles() {

	files, _ := filepath.Glob("schema_*_gocms.json")
	log.Infof("Clean up config files: %v", files)

	for _, fileName := range files {
		os.Remove(fileName)

	}

}
