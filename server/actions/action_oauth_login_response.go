package actions

import (
	"context"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"strings"
	"time"
)

type oauthLoginResponseActionPerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*resource.DbResource
	configStore   *resource.ConfigStore
	otpKey        string
}

func (d *oauthLoginResponseActionPerformer) Name() string {
	return "oauth.login.response"
}

func GetOauthConnectionDescription(authenticator string, dbResource *resource.DbResource, transaction *sqlx.Tx) (*oauth2.Config, daptinid.DaptinReferenceId, error) {

	rows, _, err := dbResource.Cruds["oauth_connect"].GetRowsByWhereClauseWithTransaction("oauth_connect",
		nil, transaction, goqu.Ex{"name": authenticator})

	if err != nil {
		log.Errorf("Failed to get oauth connection details for in response handler  [%v]", authenticator)
		return nil, daptinid.NullReferenceId, err
	}

	if len(rows) < 1 {
		log.Errorf("Failed to get oauth connection details for  [%v]", authenticator)
		err = errors.New(fmt.Sprintf("No such authenticator [%v]", authenticator))
		return nil, daptinid.NullReferenceId, err
	}

	secret, err := dbResource.ConfigStore.GetConfigValueFor("encryption.secret", "backend", transaction)
	if err != nil {
		log.Errorf("Failed to get secret: %v", err)
		return nil, daptinid.NullReferenceId, err
	}

	conf, err := mapToOauthConfig(rows[0], secret)
	log.Printf("[%v] oauth config: %v", authenticator, conf)
	return conf, daptinid.InterfaceToDIR(rows[0]["reference_id"]), err

}

func mapToOauthConfig(authConnectorData map[string]interface{}, secret string) (*oauth2.Config, error) {

	redirectUri := authConnectorData["redirect_uri"].(string)
	authenticator := authConnectorData["name"].(string)

	if strings.Index(redirectUri, "?") > -1 {
		redirectUri = redirectUri + "&authenticator=" + authenticator
	} else {
		redirectUri = redirectUri + "?authenticator=" + authenticator
	}

	clientSecretEncrypted := authConnectorData["client_secret"].(string)
	clientSecretPlainText, err := resource.Decrypt([]byte(secret), clientSecretEncrypted)
	if err != nil {
		log.Errorf("Failed to get decrypt text: %v", err)
		return nil, err
	}

	conf := &oauth2.Config{
		ClientID:     authConnectorData["client_id"].(string),
		ClientSecret: clientSecretPlainText,
		RedirectURL:  redirectUri,
		Scopes:       strings.Split(authConnectorData["scope"].(string), ","),
		Endpoint: oauth2.Endpoint{
			AuthURL:  authConnectorData["auth_url"].(string),
			TokenURL: authConnectorData["token_url"].(string),
		},
	}

	return conf, nil
}

func (d *oauthLoginResponseActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	state := inFieldMap["state"].(string)
	//user := inFieldMap["user"].(map[string]interface{})

	ok, err := totp.ValidateCustom(state, d.otpKey, time.Now().UTC(), totp.ValidateOpts{
		Period:    300,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if !ok {
		log.Errorf("Failed to validate otp key")
		return nil, nil, []error{errors.New("No ongoing authentication")}
	}

	authenticator := inFieldMap["authenticator"].(string)
	code := inFieldMap["code"].(string)
	user_reference_id := daptinid.InterfaceToDIR(inFieldMap["user_reference_id"])

	conf, authReferenceId, err := GetOauthConnectionDescription(authenticator, d.cruds["oauth_connect"], transaction)

	if err != nil {
		return nil, nil, []error{err}
	}

	ctx := context.Background()
	token, err := conf.Exchange(ctx, code)
	if err != nil {
		log.Errorf("Failed to exchange code for token in login response: %v", err)
		return nil, nil, []error{err}
	}

	err = d.cruds["oauth_token"].StoreToken(token, authenticator, authReferenceId, user_reference_id, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}
	resource.CheckErr(err, "Failed to store new auth token")

	responseAttrs := make(map[string]interface{})

	responseAttrs["title"] = "Successfully connected"
	responseAttrs["message"] = "You can use this connection now"
	responseAttrs["type"] = "success"

	actionResponse := resource.NewActionResponse("client.notify", responseAttrs)

	setStateResponse := resource.NewActionResponse("client.store.set", map[string]interface{}{
		"key":   "token",
		"value": token.AccessToken,
	})

	redirectAttrs := make(map[string]interface{})
	redirectAttrs["delay"] = 0
	redirectAttrs["location"] = "/in/item/oauth_token"
	redirectAttrs["window"] = "self"
	redirectResponse := resource.NewActionResponse("client.redirect", redirectAttrs)

	modelResponse := resource.NewResponse(nil, api2go.NewApi2GoModelWithData(
		"", nil, 0, nil, map[string]interface{}{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"expiry":        token.Expiry,
		}), 0, nil)

	return modelResponse, []actionresponse.ActionResponse{setStateResponse, actionResponse, redirectResponse}, nil
}

func NewOauthLoginResponseActionPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, configStore *resource.ConfigStore, transaction *sqlx.Tx) (actionresponse.ActionPerformerInterface, error) {

	secret, err := configStore.GetConfigValueFor("totp.secret", "backend", transaction)
	if err != nil {
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      "site.daptin.com",
			AccountName: "dummy@site.daptin.com",
			Period:      300,
			SecretSize:  10,
		})

		if err != nil {
			log.Errorf("Failed to generate code: %v", err)
			return nil, err
		}
		configStore.SetConfigValueFor("totp.secret", key.Secret(), "backend", transaction)
		secret = key.Secret()
	}

	handler := oauthLoginResponseActionPerformer{
		cruds:       cruds,
		otpKey:      secret,
		configStore: configStore,
	}

	return &handler, nil

}
