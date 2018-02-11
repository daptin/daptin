package resource

import (
	log "github.com/sirupsen/logrus"
	"github.com/artpar/api2go"
	//"context"
	"bytes"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"strings"
	"golang.org/x/oauth2"
	"time"
)

type OuathProfileExchangePerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*DbResource
}

func (d *OuathProfileExchangePerformer) Name() string {
	return "oauth.profile.exchange"
}

func GetTokensScope(tokUrl string, scope string, clientId string, clientSecret string, token string) (map[string]interface{}, error) {

	log.Infof("Token url for token exchange: %v", tokUrl)
	urlParams := "grant_type=client_credentials&client_id=" + clientId
	dat := map[string]interface{}{}

	//if len(clientSecret) > 0 {
	//	urlParams = urlParams + "&client_secret=" + clientSecret
	//}

	if len(token) > 0 {
		urlParams = urlParams + "&access_token=" + token
	}

	scope = strings.TrimSpace(scope)
	if len(scope) > 0 {
		urlParams = urlParams + "&scope=" + scope
	}

	body := bytes.NewBuffer([]byte(urlParams))
	req, err := http.NewRequest("POST", tokUrl, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientId, clientSecret)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return dat, err
	}

	defer resp.Body.Close()
	rsBody, err := ioutil.ReadAll(resp.Body)
	bstr := string(rsBody)
	log.Infof("oauth token exchange response: %v", bstr)
	err = json.Unmarshal(rsBody, &dat)
	if err != nil {
		return dat, err
	}

	return dat, err
}

type TokenResponse struct {
	oauth2.Token
	Scope string
}

func (d *OuathProfileExchangePerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	authenticator := inFieldMap["authenticator"].(string)
	token := inFieldMap["token"].(string)

	conf, oauthConnectRefId, err := GetOauthConnectionDescription(authenticator, d.cruds["oauth_connect"])

	if err != nil {
		return nil, nil, []error{err}
	}

	var oauthToken *oauth2.Token
	token_type := inFieldMap["token_type"]
	if token_type != nil {
		oauthToken, err = d.cruds["oauth_token"].GetTokenByTokenName(token_type.(string))
		CheckErr(err, "No existing token by name [%v]", token_type)
	}
	var tokenResponse map[string]interface{}
	if oauthToken == nil || !oauthToken.Valid() {
		tokenResponse, err = GetTokensScope(inFieldMap["tokenInfoUrl"].(string), strings.Join(conf.Scopes, ","), conf.ClientID, conf.ClientSecret, token)
		if err != nil {
			log.Errorf("Failed to exchange code for token: %v", err)
			return nil, nil, []error{err}
		}
		log.Infof("token response: %v", tokenResponse)

		if token_type != nil {
			oauthToken, err = d.cruds["oauth_token"].GetTokenByTokenName(token_type.(string))

			oauthToken := oauth2.Token{}
			oauthToken.AccessToken = tokenResponse["access_token"].(string)
			if tokenResponse["expiry"] != nil {
				oauthToken.Expiry = time.Unix(int64(tokenResponse["expiry"].(float64)), 0)
			}
			if tokenResponse["expires_in"] != nil {
				seconds := int(tokenResponse["expires_in"].(float64))
				oauthToken.Expiry = time.Now().Add(time.Duration(seconds) * time.Second)
				tokenResponse["expiry"] = oauthToken.Expiry.Unix()
			}
			if tokenResponse["refresh_token"] != nil {
				oauthToken.RefreshToken = tokenResponse["refresh_token"].(string)
			}
			oauthToken.TokenType = tokenResponse["token_type"].(string)

			err = d.cruds["oauth_token"].StoreToken(&oauthToken, token_type.(string), oauthConnectRefId)
		}
	} else {
		tokenResponse = make(map[string]interface{})
		tokenResponse["access_token"] = oauthToken.AccessToken
		tokenResponse["refresh_token"] = oauthToken.RefreshToken
		tokenResponse["token_type"] = oauthToken.TokenType
		tokenResponse["expiry"] = oauthToken.Expiry.Unix()
	}

	return nil, []ActionResponse{
		{
			ResponseType: "token",
			Attributes:   tokenResponse,
		},
	}, nil
}

func NewOuathProfileExchangePerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := OuathProfileExchangePerformer{
		cruds: cruds,
	}

	return &handler, nil

}
