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

type mailSubmissionActionPerformer struct {
	sender    mailSendActionPerformer
	operation string
}

func (d *mailSubmissionActionPerformer) Name() string {
	if d.operation == "status" {
		return "mail.submission.status"
	}
	return "mail.submit"
}
func (d *mailSubmissionActionPerformer) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	encoded, err := standardjson.Marshal(fields["submission"])
	if err != nil {
		return nil, nil, []error{err}
	}
	var input resource.MailSubmissionRequest
	decoder := standardjson.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&input); err != nil {
		return nil, nil, []error{api2go.NewHTTPError(err, "invalid submission fields", 400)}
	}
	caller, _ := fields["sessionUser"].(*auth.SessionUser)
	var result map[string]interface{}
	if d.operation == "status" {
		result, err = d.sender.cruds["mail"].MailSubmissionStatus(input, caller, tx)
	} else {
		result, err = d.sender.cruds["mail"].SubmitMailDraft(input, caller, tx, d.sender.signMail)
	}
	if err != nil {
		return nil, nil, []error{err}
	}
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("mail.submission", result)}, nil
}
func NewMailSubmissionActionPerformer(cruds map[string]*resource.DbResource, certificates *resource.CertificateManager, operation string) (actionresponse.ActionPerformerInterface, error) {
	return &mailSubmissionActionPerformer{sender: mailSendActionPerformer{cruds: cruds, certificateManager: certificates}, operation: operation}, nil
}
