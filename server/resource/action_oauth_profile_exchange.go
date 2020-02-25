package resource

import (
	"github.com/artpar/api2go"
	log "github.com/sirupsen/logrus"
	//"context"
	"bytes"
	"golang.org/x/oauth2"
	"io/ioutil"
	"net/http"
	"strings"
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

	log.Infof("Profile url for token exchange: %v", tokUrl)
	urlParams := ""
	dat := map[string]interface{}{}

	//if len(clientSecret) > 0 {
	//	urlParams = urlParams + "&key=" + clientSecret
	//}

	//if len(token) > 0 {
	//	urlParams = urlParams
	//}

	scope = strings.TrimSpace(scope)
	if len(scope) > 0 {
		//urlParams = urlParams + "&scope=" + scope
	}

	body := bytes.NewBuffer([]byte(urlParams))

	tokUrl = tokUrl + urlParams

	req, err := http.NewRequest("GET", tokUrl, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if len(token) > 0 {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	tenSeconds, err := time.ParseDuration("10s")
	client := &http.Client{
		Timeout: tenSeconds,
	}
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

func (d *OuathProfileExchangePerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	authenticator := inFieldMap["authenticator"].(string)
	token := inFieldMap["token"].(string)

	conf, _, err := GetOauthConnectionDescription(authenticator, d.cruds["oauth_connect"])

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
		tokenResponse, err = GetTokensScope(inFieldMap["profileUrl"].(string), strings.Join(conf.Scopes, ","), conf.ClientID, conf.ClientSecret, token)
		if err != nil {
			log.Errorf("Failed to exchange code for token during profile exchange: %v", err)
			return nil, nil, []error{err}
		}
		log.Infof("token response: %v", tokenResponse)

		if token_type != nil {
			oauthToken, err = d.cruds["oauth_token"].GetTokenByTokenName(token_type.(string))

			oauthToken := oauth2.Token{}
			if tokenResponse["expires_in"] != nil {
				seconds := int(tokenResponse["expires_in"].(float64))
				oauthToken.Expiry = time.Now().Add(time.Duration(seconds) * time.Second)
				tokenResponse["expiry"] = oauthToken.Expiry.Unix()
			}

		}
	} else {
		tokenResponse = make(map[string]interface{})
		tokenResponse["access_token"] = oauthToken.AccessToken
		tokenResponse["refresh_token"] = oauthToken.RefreshToken
		tokenResponse["token_type"] = oauthToken.TokenType
		tokenResponse["expiry"] = oauthToken.Expiry.Unix()
	}

	responder := api2go.Response{
		Res: api2go.NewApi2GoModelWithData("oauth_profile", nil, 0, nil, tokenResponse),
	}

	return responder, []ActionResponse{
		{
			ResponseType: "ACTIONRESPONSE",
			Attributes: map[string]interface{}{
				"location": "/auth/signin",
				"window":   "self",
				"delay":    2000,
			},
		}}, nil
}

func NewOuathProfileExchangePerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := OuathProfileExchangePerformer{
		cruds: cruds,
	}

	return &handler, nil

}
