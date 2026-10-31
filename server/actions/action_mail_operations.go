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

type mailOperationActionPerformer struct {
	cruds     map[string]*resource.DbResource
	family    string
	operation string
}

func (d *mailOperationActionPerformer) Name() string { return "mail." + d.family + "." + d.operation }
func (d *mailOperationActionPerformer) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	encoded, err := standardjson.Marshal(fields[d.family])
	if err != nil {
		return nil, nil, []error{err}
	}
	decoder := standardjson.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	caller, _ := fields["sessionUser"].(*auth.SessionUser)
	var result map[string]interface{}
	if d.family == "message" {
		var input resource.MailMessageOperation
		if err = decoder.Decode(&input); err != nil {
			return nil, nil, []error{api2go.NewHTTPError(err, "invalid mail operation fields", 400)}
		} else {
			result, err = d.cruds["mail"].MailMessageOperation(input, caller, d.operation, tx)
		}
	} else {
		var input resource.MailFolderOperation
		if err = decoder.Decode(&input); err != nil {
			return nil, nil, []error{api2go.NewHTTPError(err, "invalid mail operation fields", 400)}
		} else {
			result, err = d.cruds["mail_box"].MailFolderOperation(input, caller, d.operation, tx)
		}
	}
	if err != nil {
		return nil, nil, []error{err}
	}
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("mail."+d.family, result)}, nil
}
func NewMailOperationActionPerformer(cruds map[string]*resource.DbResource, family, operation string) (actionresponse.ActionPerformerInterface, error) {
	return &mailOperationActionPerformer{cruds: cruds, family: family, operation: operation}, nil
}
