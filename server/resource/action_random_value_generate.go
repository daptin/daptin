package resource

import (
	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
)

// becomeAdminActionPerformer daptin action implementation
type randomValueGeneratePerformerr struct {
	cruds map[string]*DbResource
}

// Name of the action
func (d *randomValueGeneratePerformerr) Name() string {
	return "random.generate"
}

// becomeAdminActionPerformer Perform action and try to make the current user the admin of the system
// Checks CanBecomeAdmin and then invokes BecomeAdmin if true
func (d *randomValueGeneratePerformerr) DoAction(request Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	responseAttrs := make(map[string]interface{})

	randomType := inFieldMap["type"].(string)
	responseAttrs["value"] = ColumnManager.ColumnMap[randomType].Fake()
	//actionResponse := NewActionResponse("random.string", responseAttrs)

	return api2go.Response{
		Res: &api2go.Api2GoModel{
			Data: responseAttrs,
		},
	}, []ActionResponse{}, nil
}

// Create a new action performer for becoming administrator action
func NewRandomValueGeneratePerformer() (ActionPerformerInterface, error) {

	handler := randomValueGeneratePerformerr{}

	return &handler, nil

}
