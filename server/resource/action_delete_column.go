package resource

import (
	"github.com/artpar/api2go"
	"net/http"
)

type DeleteWorldColumnPerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
}

func (d *DeleteWorldColumnPerformer) Name() string {
	return "world.column.delete"
}

func (d *DeleteWorldColumnPerformer) DoAction(request ActionRequest, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {


	worldId := inFields["world_id"].(string)
	columnToDelete := inFields["column_name"].(string)

	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "GET",
		},
	}
	tableName := d.cruds["world"].FindOne(worldId, req)
	return nil, []ActionResponse{}, nil
}
