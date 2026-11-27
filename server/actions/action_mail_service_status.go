package actions

import (
	"errors"
	"net/http"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

type mailServiceStatusActionPerformer struct {
	snapshot func() interface{}
	world    *resource.DbResource
}

func (*mailServiceStatusActionPerformer) Name() string { return "mail.service.status" }

func (p *mailServiceStatusActionPerformer) DoAction(_ actionresponse.Outcome, input map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	denied := api2go.NewHTTPError(errors.New("administrator membership required"), "administrator membership required", http.StatusForbidden)
	active, _ := input["sessionUser"].(*auth.SessionUser)
	if active == nil || p.world == nil || p.world.AdministratorGroupId == daptinid.NullReferenceId {
		return nil, nil, []error{denied}
	}
	// Trusted action outcomes temporarily augment Groups. Resolve persisted
	// membership again so wrapper execution cannot grant diagnostic access.
	persisted, err := p.world.MailSessionUser(active, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	if !(permission.PermissionInstance{}).CanExecute(persisted.UserReferenceId, persisted.Groups, p.world.AdministratorGroupId) {
		return nil, nil, []error{denied}
	}
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("mail.service.status", p.snapshot())}, nil
}

func NewMailServiceStatusActionPerformer(cruds map[string]*resource.DbResource, snapshot func() interface{}) (actionresponse.ActionPerformerInterface, error) {
	if snapshot == nil {
		return nil, errors.New("mail service status snapshot callback is required")
	}
	return &mailServiceStatusActionPerformer{snapshot: snapshot, world: cruds["world"]}, nil
}
