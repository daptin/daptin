package actions

import (
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"

	//"os/exec"
	//"fmt"
	"github.com/artpar/api2go"
)

type restartSystemActionPerformer struct {
	responseAttrs map[string]interface{}
}

func (d *restartSystemActionPerformer) Name() string {
	return "__restart"
}

func (d *restartSystemActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Initiating system update."
	restartAttrs["title"] = "Success"
	actionResponse := resource.NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	// new response
	restartAttrs = make(map[string]interface{})
	restartAttrs["location"] = "/"
	restartAttrs["window"] = "self"
	restartAttrs["delay"] = 5000
	actionResponse = resource.NewActionResponse("client.redirect", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewRestartSystemPerformer(initConfig *resource.CmsConfig) (actionresponse.ActionPerformerInterface, error) {

	handler := restartSystemActionPerformer{}

	return &handler, nil

}
