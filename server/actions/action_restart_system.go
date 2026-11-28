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
	requestRestart func(*sqlx.Tx) error
}

func (d *restartSystemActionPerformer) Name() string {
	return "__restart"
}

func (d *restartSystemActionPerformer) DoAction(_ actionresponse.Outcome, _ map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if err := d.requestRestart(tx); err != nil {
		return nil, nil, []error{err}
	}
	return nil, []actionresponse.ActionResponse{{ResponseType: "Restart"}}, nil
}

func NewRestartSystemPerformer(requestRestart func(*sqlx.Tx) error) (actionresponse.ActionPerformerInterface, error) {
	if requestRestart == nil {
		return nil, errors.New("restart callback is required")
	}
	return &restartSystemActionPerformer{requestRestart: requestRestart}, nil
}
