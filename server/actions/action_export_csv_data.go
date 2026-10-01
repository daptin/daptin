package actions

import (
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

type exportCsvDataPerformer struct {
	cmsConfig *resource.CmsConfig
	cruds     map[string]*resource.DbResource
}

func (d *exportCsvDataPerformer) Name() string {
	return "__csv_data_export"
}

func (d *exportCsvDataPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	return (&exportDataPerformer{cmsConfig: d.cmsConfig, cruds: d.cruds}).DoAction(request, map[string]interface{}{
		"table_name": inFields["table_name"],
		"format":     "csv",
	}, transaction)
}

func NewExportCsvDataPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &exportCsvDataPerformer{cmsConfig: initConfig, cruds: cruds}, nil
}
