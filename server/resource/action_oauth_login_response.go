package resource

import (
	"context"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"net/http"
	"strings"
	"time"
)

type oauthLoginResponseActionPerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*DbResource
	configStore   *ConfigStore
	otpKey        string
}

func (d *oauthLoginResponseActionPerformer) Name() string {
	return "oauth.login.response"
}

func GetOauthConnectionDescription(authenticator string, dbResource *DbResource, transaction *sqlx.Tx) (*oauth2.Config, string, error) {

	rows, _, err := dbResource.Cruds["oauth_connect"].GetRowsByWhereClauseWithTransaction("oauth_connect",
		nil, transaction, goqu.Ex{"name": authenticator})

	if err != nil {
		log.Errorf("Failed to get oauth Connection details for in response handler  [%v]", authenticator)
		return nil, "", err
	}

	if len(rows) < 1 {
		log.Errorf("Failed to get oauth Connection details for  [%v]", authenticator)
		err = errors.New(fmt.Sprintf("No such authenticator [%v]", authenticator))
		return nil, "", err
	}

	secret, err := dbResource.configStore.GetConfigValueFor("encryption.secret", "backend", transaction)
	if err != nil {
		log.Errorf("Failed to get secret: %v", err)
		return nil, "", err
	}

	conf, err := mapToOauthConfig(rows[0], secret)
	log.Printf("[%v] oauth config: %v", authenticator, conf)
	return conf, rows[0]["reference_id"].(string), err

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
	clientSecretPlainText, err := Decrypt([]byte(secret), clientSecretEncrypted)
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

func (dbResource *DbResource) StoreToken(token *oauth2.Token,
	token_type string, oauth_connect_reference_id string,
	user_reference_id string, transaction *sqlx.Tx) error {
	storeToken := make(map[string]interface{})

	storeToken["access_token"] = token.AccessToken
	storeToken["refresh_token"] = token.RefreshToken
	expiry := token.Expiry.Unix()
	if expiry < 0 {
		expiry = time.Now().Add(24 * 300 * time.Hour).Unix()
	}
	storeToken["expires_in"] = expiry
	storeToken["token_type"] = token_type
	storeToken["oauth_connect_id"] = oauth_connect_reference_id

	userId, err := dbResource.GetReferenceIdToId(USER_ACCOUNT_TABLE_NAME, user_reference_id, transaction)

	if err != nil {
		return err
	}

	sessionUser := &auth.SessionUser{
		UserId:          userId,
		UserReferenceId: user_reference_id,
		Groups:          nil,
	}

	pr := &http.Request{
		Method: "POST",
	}
	pr = pr.WithContext(context.WithValue(context.Background(), "user", sessionUser))

	req := api2go.Request{
		PlainRequest: pr,
	}

	model := api2go.NewApi2GoModelWithData("oauth_token", nil, int64(auth.DEFAULT_PERMISSION), nil, storeToken)

	_, err = dbResource.Cruds["oauth_token"].CreateWithoutFilter(model, req, transaction)
	return err
}

func (d *oauthLoginResponseActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

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
	user_reference_id := inFieldMap["user_reference_id"].(string)

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
	CheckErr(err, "Failed to store new auth token")

	responseAttrs := make(map[string]interface{})

	responseAttrs["title"] = "Successfully connected"
	responseAttrs["message"] = "You can use this Connection now"
	responseAttrs["type"] = "success"

	actionResponse := NewActionResponse("client.notify", responseAttrs)

	setStateResponse := NewActionResponse("client.store.set", map[string]interface{}{
		"key":   "token",
		"value": token.AccessToken,
	})

	redirectAttrs := make(map[string]interface{})
	redirectAttrs["delay"] = 0
	redirectAttrs["location"] = "/in/item/oauth_token"
	redirectAttrs["window"] = "self"
	redirectResponse := NewActionResponse("client.redirect", redirectAttrs)

	modelResponse := NewResponse(nil, api2go.Api2GoModel{
		Data: map[string]interface{}{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"expiry":        token.Expiry,
		},
	}, 0, nil)

	return modelResponse, []ActionResponse{setStateResponse, actionResponse, redirectResponse}, nil
}

func NewOauthLoginResponseActionPerformer(initConfig *CmsConfig, cruds map[string]*DbResource, configStore *ConfigStore, transaction *sqlx.Tx) (ActionPerformerInterface, error) {

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
