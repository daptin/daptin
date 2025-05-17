package actions

import (
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

// becomeAdminActionPerformer daptin action implementation
type randomValueGeneratePerformerr struct {
	cruds map[string]*resource.DbResource
}

// Name of the action
func (d *randomValueGeneratePerformerr) Name() string {
	return "random.generate"
}

// becomeAdminActionPerformer Perform action and try to make the current user the admin of the system
// Checks CanBecomeAdmin and then invokes BecomeAdmin if true
func (d *randomValueGeneratePerformerr) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responseAttrs := make(map[string]interface{})

	randomType := inFieldMap["type"].(string)
	responseAttrs["value"] = resource.ColumnManager.ColumnMap[randomType].Fake()
	//actionResponse := resource.NewActionResponse("random.string", responseAttrs)

	return api2go.Response{
		Res: api2go.NewApi2GoModelWithData(randomType, nil, 0, nil, responseAttrs),
	}, []actionresponse.ActionResponse{}, nil
}

// Create a new action performer for becoming administrator action
func NewRandomValueGeneratePerformer() (actionresponse.ActionPerformerInterface, error) {

	handler := randomValueGeneratePerformerr{}

	return &handler, nil

}
