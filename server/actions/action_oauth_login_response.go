package actions

import (
	"context"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"strings"
	"time"
)

type oauthLoginResponseActionPerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*resource.DbResource
	configStore   *resource.ConfigStore
}

func (d *oauthLoginResponseActionPerformer) Name() string {
	return "oauth.login.response"
}

func GetOauthConnectionDescription(authenticator string, dbResource *resource.DbResource, transaction *sqlx.Tx) (*oauth2.Config, daptinid.DaptinReferenceId, map[string]interface{}, error) {

	rows, _, err := dbResource.Cruds["oauth_connect"].GetRowsByWhereClauseWithTransaction("oauth_connect",
		nil, transaction, goqu.Ex{"name": authenticator})

	if err != nil {
		log.Errorf("Failed to get oauth connection details for in response handler  [%v]", authenticator)
		return nil, daptinid.NullReferenceId, nil, err
	}

	if len(rows) < 1 {
		log.Errorf("Failed to get oauth connection details for  [%v]", authenticator)
		err = errors.New(fmt.Sprintf("No such authenticator [%v]", authenticator))
		return nil, daptinid.NullReferenceId, nil, err
	}

	secret, err := dbResource.ConfigStore.GetConfigValueFor("encryption.secret", "backend", transaction)
	if err != nil {
		log.Errorf("Failed to get secret: %v", err)
		return nil, daptinid.NullReferenceId, nil, err
	}

	conf, err := mapToOauthConfig(rows[0], secret)
	log.Printf("[%v] oauth config loaded: auth_url=%s token_url=%s redirect_url=%s scopes=%v",
		authenticator, conf.Endpoint.AuthURL, conf.Endpoint.TokenURL, conf.RedirectURL, conf.Scopes)
	return conf, daptinid.InterfaceToDIR(rows[0]["reference_id"]), rows[0], err

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
	user := inFieldMap["sessionUser"]
	sessionUser, _ := user.(*auth.SessionUser)
	//user := inFieldMap["user"].(map[string]interface{})

	authenticator := inFieldMap["authenticator"].(string)
	code := inFieldMap["code"].(string)

	conf, authReferenceId, row, err := GetOauthConnectionDescription(authenticator, d.cruds["oauth_connect"], transaction)

	if err != nil {
		return nil, nil, []error{err}
	}

	now := time.Now().UTC()
	oauthState, err := loadOAuthState(d.cruds, d.configStore, authReferenceId, state, now, oauthConnectorPKCEEnabled(row), transaction)
	if err != nil {
		log.Errorf("Failed to validate oauth state: %v", err)
		return nil, nil, []error{errors.New("No ongoing authentication")}
	}

	ctx := context.Background()
	token, err := exchangeOAuthCode(ctx, conf, code, row, oauthState)
	if err != nil {
		log.Errorf("Failed to exchange code for token in login response: %v", err)
		return nil, nil, []error{err}
	}

	if oauthState != nil {
		sessionUser = oauthStateOwnerSession(oauthState, sessionUser)
		err = markOAuthStateUsed(d.cruds, oauthState, now, sessionUser, transaction)
		if err != nil {
			return nil, nil, []error{err}
		}
	}

	err = d.cruds["oauth_token"].StoreToken(token, authenticator, authReferenceId, sessionUser, transaction)
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

	handler := oauthLoginResponseActionPerformer{
		cruds:       cruds,
		configStore: configStore,
	}

	return &handler, nil

}
