package resource

import (
	"github.com/artpar/api2go"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

type generateOauth2TokenActionPerformer struct {
	cruds  map[string]*DbResource
	secret []byte
}

func (d *generateOauth2TokenActionPerformer) Name() string {
	return "oauth.token"
}

func (d *generateOauth2TokenActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	referenceId, ok := inFieldMap["reference_id"]
	if !ok {
		return nil, responses, []error{errors.New("Token Reference id missing")}
	}
	referenceIdString := referenceId.(daptinid.DaptinReferenceId)
	token, _, err := d.cruds["oauth_token"].GetTokenByTokenReferenceId(referenceIdString, transaction)

	responseObject := api2go.NewApi2GoModelWithData("oauth_token", nil, 0, nil, map[string]interface{}{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expiry":        token.Expiry,
	})
	responses = append(responses, NewActionResponse("oauth_token", responseObject))
	return NewResponse(nil, responseObject, 200, nil), responses, []error{err}
}

func NewGenerateOauth2TokenPerformer(configStore *ConfigStore, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := generateOauth2TokenActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
