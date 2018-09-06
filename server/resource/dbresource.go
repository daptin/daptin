package resource

import (
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/database"
	"github.com/jmoiron/sqlx"
)

type DbResource struct {
	model            *api2go.Api2GoModel
	db               database.DatabaseConnection
	tableInfo        *TableInfo
	Cruds            map[string]*DbResource
	ms               *MiddlewareSet
	ActionHandlerMap map[string]ActionPerformerInterface
	configStore      *ConfigStore
	contextCache     map[string]interface{}
	defaultGroups    []int64
}

func NewDbResource(model *api2go.Api2GoModel, db database.DatabaseConnection, ms *MiddlewareSet, cruds map[string]*DbResource, configStore *ConfigStore, tableInfo TableInfo) *DbResource {
	//log.Infof("Columns [%v]: %v\n", model.GetName(), model.GetColumnNames())
	return &DbResource{
		model:         model,
		db:            db,
		ms:            ms,
		configStore:   configStore,
		Cruds:         cruds,
		tableInfo:     &tableInfo,
		defaultGroups: GroupNamesToIds(db, tableInfo.DefaultGroups),
		contextCache:  make(map[string]interface{}),
	}
}
func GroupNamesToIds(db database.DatabaseConnection, groupsName []string) []int64 {

	if len(groupsName) == 0 {
		return []int64{}
	}

	var retArray []int64

	query, args, err := sqlx.In("select id from usergroup where name in (?)", groupsName)
	CheckErr(err, "Failed to convert usergroup names to ids")
	query = db.Rebind(query)

	err = db.Select(&retArray, query, args...)
	CheckErr(err, "Failed to query user-group names to ids")

	//retInt := make([]int64, 0)

	//for _, val := range retArray {
	//	iVal, _ := strconv.ParseInt(val, 10, 64)
	//	retInt = append(retInt, iVal)
	//}

	return retArray

}

func (dr *DbResource) PutContext(key string, val interface{}) {
	dr.contextCache[key] = val
}

func (dr *DbResource) GetContext(key string) interface{} {
	return dr.contextCache[key]
}

func (dr *DbResource) GetAdminReferenceId() string {
	cacheVal := dr.GetContext("administrator_reference_id")
	if cacheVal == nil {

		userRefId := dr.GetUserIdByUsergroupId(2)
		dr.PutContext("administrator_reference_id", userRefId)
		return userRefId
	} else {
		return cacheVal.(string)
	}
}

func (dr *DbResource) GetAdminEmailId() string {
	cacheVal := dr.GetContext("administrator_email_id")
	if cacheVal == nil {
		userRefId := dr.GetUserEmailIdByUsergroupId(2)
		dr.PutContext("administrator_email_id", userRefId)
		return userRefId
	} else {
		return cacheVal.(string)
	}
}
