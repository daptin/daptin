package actions

import (
	"fmt"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	log "github.com/sirupsen/logrus"
)

type oauthLoginBeginActionPerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*resource.DbResource
	configStore   *resource.ConfigStore
	otpKey        string
}

func (d *oauthLoginBeginActionPerformer) Name() string {
	return "oauth.client.redirect"
}

func (d *oauthLoginBeginActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	authConnectorData := inFieldMap["authenticator"].(string)
	sessionUser, _ := inFieldMap["sessionUser"].(*auth.SessionUser)

	//redirectUri := authConnectorData["redirect_uri"].(string)
	//
	//if strings.Index(redirectUri, "?") > -1 {
	//	redirectUri = redirectUri + "&authenticator=" + authConnectorData["name"].(string)
	//} else {
	//	redirectUri = redirectUri + "?authenticator=" + authConnectorData["name"].(string)
	//}

	conf, authReferenceId, row, err := GetOauthConnectionDescription(authConnectorData, d.cruds["oauth_connect"], transaction)
	if err != nil {
		resource.CheckErr(err, "Failed to get oauth.conf from authenticator name")
		return nil, nil, []error{err}
	}

	var state string
	if oauthConnectorPKCEEnabled(row) {
		state, err = resource.OAuthRandomToken()
	} else {
		state, err = totp.GenerateCodeCustom(d.otpKey, time.Now(), totp.ValidateOpts{
			Period:    300,
			Skew:      1,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
	}
	if err != nil {
		log.Errorf("Failed to generate oauth state: %v", err)
		return nil, nil, []error{err}
	}

	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	// Use access_type_offline from the oauth_connect config to request offline access.
	verifier, err := oauthVerifierForConnector(row)
	if err != nil {
		return nil, nil, []error{err}
	}
	opts, err := oauthAuthorizationOptions(row, verifier)
	if err != nil {
		return nil, nil, []error{err}
	}
	if oauthConnectorPKCEEnabled(row) {
		err = storeOAuthState(d.cruds, authReferenceId, state, verifier, time.Now().UTC(), sessionUser, transaction)
		if err != nil {
			return nil, nil, []error{err}
		}
	}
	url := conf.AuthCodeURL(state, opts...)
	fmt.Printf("Visit the URL for the auth dialog: %v", url)

	responseAttrs := make(map[string]interface{})

	responseAttrs["location"] = url
	responseAttrs["window"] = "self"
	responseAttrs["delay"] = 0

	setStateResponse := resource.NewActionResponse("client.store.set", map[string]interface{}{
		"key":   "secret",
		"value": state,
	})
	actionResponse := resource.NewActionResponse("client.redirect", responseAttrs)

	return nil, []actionresponse.ActionResponse{setStateResponse, actionResponse}, nil
}

func NewOauthLoginBeginActionPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, configStore *resource.ConfigStore, transaction *sqlx.Tx) (actionresponse.ActionPerformerInterface, error) {

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

	handler := oauthLoginBeginActionPerformer{
		cruds:       cruds,
		configStore: configStore,
		otpKey:      secret,
	}

	return &handler, nil

}
