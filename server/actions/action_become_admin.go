package actions

import (
	"context"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// becomeAdminActionPerformer daptin action implementation
type becomeAdminActionPerformer struct {
	cruds map[string]*resource.DbResource
}

// Name of the action
func (d *becomeAdminActionPerformer) Name() string {
	return "__become_admin"
}

// becomeAdminActionPerformer Perform action and try to make the current user the admin of the system
// Checks CanBecomeAdmin and then invokes BecomeAdmin if true
func (d *becomeAdminActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	if !d.cruds["world"].CanBecomeAdmin(transaction) {
		return nil, nil, []error{errors.New("Unauthorized")}
	}
	u := inFieldMap["user"]
	if u == nil {
		return nil, nil, []error{errors.New("Unauthorized")}
	}
	user := u.(map[string]interface{})

	responseAttrs := make(map[string]interface{})

	var actionResponse actionresponse.ActionResponse
	if d.cruds["world"].BecomeAdmin(user["id"].(int64), transaction) {
		commitError := transaction.Commit()
		resource.CheckErr(commitError, "failed to rollback")
		responseAttrs["location"] = "/"
		responseAttrs["window"] = "self"
		responseAttrs["delay"] = 7000
		//go Restart()
		actionResponse = resource.NewActionResponse("client.redirect", responseAttrs)
		_ = resource.OlricCache.Destroy(context.Background())
	} else {
		rollbackError := transaction.Rollback()
		resource.CheckErr(rollbackError, "failed to rollback")
	}

	return nil, []actionresponse.ActionResponse{actionResponse, {
		ResponseType: "Restart",
		Attributes:   nil,
	}}, nil
}

// Create a new action performer for becoming administrator action
func NewBecomeAdminPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := becomeAdminActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
