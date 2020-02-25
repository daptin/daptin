package resource

import (
	"github.com/artpar/api2go"
	"github.com/pkg/errors"
)

// BecomeAdminActionPerformer daptin action implementation
type BecomeAdminActionPerformer struct {
	cruds map[string]*DbResource
}

// Name of the action
func (d *BecomeAdminActionPerformer) Name() string {
	return "__become_admin"
}

// BecomeAdminActionPerformer Perform action and try to make the current user the admin of the system
// Checks CanBecomeAdmin and then invokes BecomeAdmin if true
func (d *BecomeAdminActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	if !d.cruds["world"].CanBecomeAdmin() {
		return nil, nil, []error{errors.New("Unauthorized")}
	}
	u := inFieldMap["user"]
	if u == nil {
		return nil, nil, []error{errors.New("Unauthorized")}
	}
	user := u.(map[string]interface{})

	responseAttrs := make(map[string]interface{})

	if d.cruds["world"].BecomeAdmin(user["id"].(int64)) {
		responseAttrs["location"] = "/"
		responseAttrs["window"] = "self"
		responseAttrs["delay"] = 7000
	}

	actionResponse := NewActionResponse("client.redirect", responseAttrs)

	go restart()

	return nil, []ActionResponse{actionResponse}, nil
}

// Create a new action performer for becoming administrator action
func NewBecomeAdminPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := BecomeAdminActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
