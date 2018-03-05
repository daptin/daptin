package resource

import (
	"github.com/artpar/api2go"
	"github.com/pkg/errors"
)

/**
  Become administrator of the instance
*/
type BecomeAdminActionPerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*DbResource
}

func (d *BecomeAdminActionPerformer) Name() string {
	return "__become_admin"
}

func (d *BecomeAdminActionPerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

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
		responseAttrs["delay"] = 15000
	}

	actionResponse := NewActionResponse("client.redirect", responseAttrs)

	go restart()

	return nil, []ActionResponse{actionResponse}, nil
}

func NewBecomeAdminPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := BecomeAdminActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
