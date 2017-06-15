package resource

import (
	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
	//log "github.com/sirupsen/logrus"
)

type DbResource struct {
	model        *api2go.Api2GoModel
	db           *sqlx.DB
	cruds        map[string]*DbResource
	ms           *MiddlewareSet
	contextCache map[string]interface{}
}

func NewDbResource(model *api2go.Api2GoModel, db *sqlx.DB, ms *MiddlewareSet, cruds map[string]*DbResource) *DbResource {
	cols := model.GetColumns()
	model.SetColumns(cols)
	//log.Infof("Columns [%v]: %v\n", model.GetName(), model.GetColumnNames())
	return &DbResource{
		model:        model,
		db:           db,
		ms:           ms,
		cruds:        cruds,
		contextCache: make(map[string]interface{}),
	}
}

func (dr *DbResource) PutContext(key string, val interface{}) {
	dr.contextCache[key] = val
}

func (dr *DbResource) GetContext(key string) interface{} {
	return dr.contextCache[key]
}
