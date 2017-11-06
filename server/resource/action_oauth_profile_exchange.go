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
)

type OuathProfileExchangePerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*DbResource
}

func (d *OuathProfileExchangePerformer) Name() string {
	return "oauth.profile.exchange"
}

func GetTokensScope(tokUrl string, scope string, clientId string, secret string) (string, error) {
	body := bytes.NewBuffer([]byte("grant_type=client_credentials&client_id=" + clientId + "&access_token=" + secret + "&scope=" + scope))
	req, err := http.NewRequest("POST", tokUrl, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	rsBody, err := ioutil.ReadAll(resp.Body)
	type WithScope struct {
		Scope string `json:"scope"`
		Email string
	}
	bstr := string(rsBody)
	log.Infof("Exx: %v", bstr)
	var dat WithScope
	err = json.Unmarshal(rsBody, &dat)
	if err != nil {
		return "", err
	}

	return dat.Scope, err
}

func (d *OuathProfileExchangePerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	authenticator := inFieldMap["authenticator"].(string)
	token := inFieldMap["token"].(string)

	conf, _, err := GetOauthConnectionDescription(authenticator, d.cruds["oauth_connect"])

	if err != nil {
		return nil, nil, []error{err}
	}

	tokenResponse, err := GetTokensScope("https://www.googleapis.com/oauth2/v1/tokeninfo", strings.Join(conf.Scopes, ","), conf.ClientID, token)
	if err != nil {
		log.Errorf("Failed to exchange code for token: %v", err)
		return nil, nil, []error{err}
	}
	log.Infof("token reponse: %v", tokenResponse)

	return nil, []ActionResponse{}, nil
}

func NewOuathProfileExchangePerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := OuathProfileExchangePerformer{
		cruds: cruds,
	}

	return &handler, nil

}
