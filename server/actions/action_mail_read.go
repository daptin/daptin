package actions

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

type mailReadActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *mailReadActionPerformer) Name() string { return "mail.read" }
func (d *mailReadActionPerformer) DoAction(request actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	caller, _ := fields["sessionUser"].(*auth.SessionUser)
	ref := daptinid.InterfaceToDIR(fields["mail_id"])
	row, err := d.cruds["mail"].ReadMailForSession(ref, caller, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	if fields["format"] == "source" {
		bytes, err := d.cruds["mail"].MailColumnBytes("mail", "mail", row["mail"])
		if err != nil {
			return nil, nil, []error{err}
		}
		if int64(len(bytes)) > mailMaxRawBytes {
			return nil, nil, []error{mailUnpackHTTPError(mailUnpackError("mail_raw_too_large", nil))}
		}
		hash := sha256.Sum256(bytes)
		return nil, []actionresponse.ActionResponse{resource.NewActionResponse("mail.source", map[string]interface{}{
			"reference_id": ref.String(), "revision": hex.EncodeToString(hash[:]), "raw_base64": base64.StdEncoding.EncodeToString(bytes),
		})}, nil
	}
	// Reuse the bounded parser and storage resolver used by mail.unpack. The
	// client cannot choose raw MIME or asset locations for this read operation.
	parser := mailUnpackActionPerformer{cruds: d.cruds}
	result, _, errs := parser.DoAction(request, map[string]interface{}{"mail": row["mail"]}, tx)
	if len(errs) != 0 {
		return nil, nil, errs
	}
	model, ok := result.Result().(api2go.Api2GoModel)
	if !ok {
		return nil, nil, []error{fmt.Errorf("invalid mail parser result")}
	}
	payload := model.GetAttributes()
	payload["reference_id"] = ref.String()
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("mail.unpack", payload)}, nil
}

func NewMailReadActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &mailReadActionPerformer{cruds: cruds}, nil
}
