package actions

import (
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

type generateOauth2TokenActionPerformer struct {
	cruds  map[string]*resource.DbResource
	secret []byte
}

func (d *generateOauth2TokenActionPerformer) Name() string {
	return "oauth.token"
}

func (d *generateOauth2TokenActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	referenceId := daptinid.InterfaceToDIR(inFieldMap["reference_id"])
	if referenceId == daptinid.NullReferenceId {
		return nil, responses, []error{errors.New("Token Reference id missing")}
	}

	token, _, err := d.cruds["oauth_token"].GetTokenByTokenReferenceId(referenceId, transaction)
	if err != nil {
		return nil, responses, []error{err}
	}

	responseObject := api2go.NewApi2GoModelWithData("oauth_token", nil, 0, nil, map[string]interface{}{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expiry":        token.Expiry,
	})
	responses = append(responses, resource.NewActionResponse("oauth_token", map[string]interface{}{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expiry":        token.Expiry,
	}))
	return resource.NewResponse(nil, responseObject, 200, nil), responses, nil
}

func NewGenerateOauth2TokenPerformer(configStore *resource.ConfigStore, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := generateOauth2TokenActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
