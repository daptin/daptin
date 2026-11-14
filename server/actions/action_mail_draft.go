package actions

import (
	"bytes"
	standardjson "encoding/json"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

type mailDraftActionPerformer struct {
	cruds     map[string]*resource.DbResource
	operation string
}

func (d *mailDraftActionPerformer) Name() string { return "mail.draft." + d.operation }
func (d *mailDraftActionPerformer) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	encoded, err := standardjson.Marshal(fields["draft"])
	if err != nil {
		return nil, nil, []error{err}
	}
	var input resource.MailDraftRequest
	decoder := standardjson.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&input); err != nil {
		return nil, nil, []error{api2go.NewHTTPError(err, "invalid draft fields", 400)}
	}
	caller, _ := fields["sessionUser"].(*auth.SessionUser)
	if d.operation == "status" {
		result, err := d.cruds["mail"].MailDraftStatus(input, caller, tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		return nil, []actionresponse.ActionResponse{resource.NewActionResponse("mail.draft.status", result)}, nil
	}
	if d.operation == "delete" {
		if err = d.cruds["mail"].DeleteMailDraft(input, caller, tx); err != nil {
			return nil, nil, []error{err}
		}
		return nil, []actionresponse.ActionResponse{resource.NewActionResponse("mail.draft.deleted", map[string]interface{}{"compose_key": input.ComposeKey, "deleted": true})}, nil
	}
	result, err := d.cruds["mail"].SaveMailDraft(input, caller, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("mail.draft", map[string]interface{}{
		"reference_id": result.ReferenceID, "compose_key": result.ComposeKey, "write_key": result.WriteKey, "revision": result.Revision, "uid": result.UID,
	})}, nil
}
func NewMailDraftActionPerformer(cruds map[string]*resource.DbResource, operation string) (actionresponse.ActionPerformerInterface, error) {
	return &mailDraftActionPerformer{cruds: cruds, operation: operation}, nil
}
