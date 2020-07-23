package resource

import (
	"github.com/artpar/api2go"
	"github.com/pkg/errors"
)

type GenerateOauth2TokenActionPerformer struct {
	cruds  map[string]*DbResource
	secret []byte
}

func (d *GenerateOauth2TokenActionPerformer) Name() string {
	return "oauth.token"
}

func (d *GenerateOauth2TokenActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	referenceId, ok := inFieldMap["reference_id"]
	if !ok {
		return nil, responses, []error{errors.New("Token Reference id missing")}
	}
	referenceIdString := referenceId.(string)
	token, _, err := d.cruds["oauth_token"].GetTokenByTokenReferenceId(referenceIdString)

	responseObject := api2go.NewApi2GoModelWithData("oauth_token", nil, 0, nil, map[string]interface{}{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expiry":        token.Expiry,
	})
	responses = append(responses, NewActionResponse("oauth_token", responseObject))
	return NewResponse(nil, responseObject, 200, nil), responses, []error{err}
}

func NewGenerateOauth2TokenPerformer(configStore *ConfigStore, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := GenerateOauth2TokenActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
