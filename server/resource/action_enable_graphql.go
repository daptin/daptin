package resource

import (
	"github.com/artpar/api2go"
)

/**
  Become administrator of daptin action implementation
*/
type GraphqlEnableActionPerformer struct {
	cruds map[string]*DbResource
}

// Name of the action
func (d *GraphqlEnableActionPerformer) Name() string {
	return "__enable_graphql"
}

// Perform action and try to make the current user the admin of the system
// Checks CanGraphqlEnable and then invokes GraphqlEnable if true
func (d *GraphqlEnableActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	err := d.cruds["world"].configStore.SetConfigValueFor("graphql.enable", "true", "backend")

	if err != nil {
		go restart()

		return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Restarting with graphql enabled", "Success"))}, nil
	} else {
		return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Failed to update config: "+err.Error(), "Failed"))}, nil
	}
}

// Create a new action performer for becoming administrator action
func NewGraphqlEnablePerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := GraphqlEnableActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
