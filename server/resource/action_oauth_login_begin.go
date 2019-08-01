package resource

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	//"golang.org/x/oauth2"
	"github.com/artpar/api2go"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/oauth2"
	"time"
)

type OauthLoginBeginActionPerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*DbResource
	configStore   *ConfigStore
	otpKey        string
}

func (d *OauthLoginBeginActionPerformer) Name() string {
	return "oauth.client.redirect"
}

func (d *OauthLoginBeginActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	state, err := totp.GenerateCodeCustom(d.otpKey, time.Now(), totp.ValidateOpts{
		Period:    300,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		log.Errorf("Failed to generate code: %v", err)
		return nil, nil, []error{err}
	}

	authConnectorData := inFieldMap["authenticator"].(string)

	//redirectUri := authConnectorData["redirect_uri"].(string)
	//
	//if strings.Index(redirectUri, "?") > -1 {
	//	redirectUri = redirectUri + "&authenticator=" + authConnectorData["name"].(string)
	//} else {
	//	redirectUri = redirectUri + "?authenticator=" + authConnectorData["name"].(string)
	//}

	conf, _, err := GetOauthConnectionDescription(authConnectorData, d.cruds["oauth_connect"])
	CheckErr(err, "Failed to get oauth.conf from authenticator name")

	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	var url string
	if len(conf.Scopes) > 1 {
		url = conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
	} else {
		url = conf.AuthCodeURL(state)

	}
	fmt.Printf("Visit the URL for the auth dialog: %v", url)

	responseAttrs := make(map[string]interface{})

	responseAttrs["location"] = url
	responseAttrs["window"] = "self"
	responseAttrs["delay"] = 0

	setStateResponse := NewActionResponse("client.store.set", map[string]interface{}{
		"key":   "secret",
		"value": state,
	})
	actionResponse := NewActionResponse("client.redirect", responseAttrs)

	return nil, []ActionResponse{setStateResponse, actionResponse}, nil
}

func NewOauthLoginBeginActionPerformer(initConfig *CmsConfig, cruds map[string]*DbResource, configStore *ConfigStore) (ActionPerformerInterface, error) {

	secret, err := configStore.GetConfigValueFor("totp.secret", "backend")
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
		configStore.SetConfigValueFor("totp.secret", key.Secret(), "backend")
		secret = key.Secret()
	}

	handler := OauthLoginBeginActionPerformer{
		cruds:       cruds,
		otpKey:      secret,
		configStore: configStore,
	}

	return &handler, nil

}
