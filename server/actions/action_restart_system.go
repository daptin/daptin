package actions

import (
	"errors"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/jmoiron/sqlx"
)

// Restart authorization belongs to the action definition, like other trusted
// backend outcomes. The process owner handles readiness, drain, and replacement.
type restartSystemActionPerformer struct {
	requestRestart func()
}

func (d *restartSystemActionPerformer) Name() string {
	return "__restart"
}

func (d *restartSystemActionPerformer) DoAction(_ actionresponse.Outcome, _ map[string]interface{}, _ *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	d.requestRestart()
	return nil, []actionresponse.ActionResponse{{ResponseType: "Restart"}}, nil
}

func NewRestartSystemPerformer(requestRestart func()) (actionresponse.ActionPerformerInterface, error) {
	if requestRestart == nil {
		return nil, errors.New("restart callback is required")
	}
	return &restartSystemActionPerformer{requestRestart: requestRestart}, nil
}
