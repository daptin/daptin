package resource

import (
	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
)

type DbResource struct {
	model         *api2go.Api2GoModel
	db            *sqlx.DB
	tableInfo     *TableInfo
	cruds         map[string]*DbResource
	ms            *MiddlewareSet
	configStore   *ConfigStore
	contextCache  map[string]interface{}
	defaultGroups []int64
}

func NewDbResource(model *api2go.Api2GoModel, db *sqlx.DB, ms *MiddlewareSet, cruds map[string]*DbResource, configStore *ConfigStore, tableInfo *TableInfo) *DbResource {
	cols := model.GetColumns()
	model.SetColumns(cols)
	//log.Infof("Columns [%v]: %v\n", model.GetName(), model.GetColumnNames())
	return &DbResource{
		model:         model,
		db:            db,
		ms:            ms,
		configStore:   configStore,
		cruds:         cruds,
		tableInfo:     tableInfo,
		defaultGroups: GroupNamesToIds(db, tableInfo.DefaultGroups),
		contextCache:  make(map[string]interface{}),
	}
}
func GroupNamesToIds(db *sqlx.DB, groupsName []string) []int64 {

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
