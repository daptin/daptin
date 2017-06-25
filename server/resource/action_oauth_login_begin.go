package resource

import (
  "fmt"
  "github.com/pkg/errors"
  log "github.com/sirupsen/logrus"
  "golang.org/x/oauth2"
  "gopkg.in/Masterminds/squirrel.v1"
  "github.com/pquerna/otp/totp"
  "time"
)

type OauthLoginBeginActionPerformer struct {
  responseAttrs map[string]interface{}
  cruds         map[string]*DbResource
  otpKey        string
}

func (d *OauthLoginBeginActionPerformer) Name() string {
  return "oauth.client.redirect"
}

func (d *OauthLoginBeginActionPerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) ([]ActionResponse, []error) {

  state, err := totp.GenerateCode(d.otpKey, time.Now())
  if err != nil {
    log.Errorf("Failed to generate code: %v", err)
    return nil, []error{err}
  }

  authenticator := "google"

  rows, _, err := d.cruds["oauthconnect"].GetRowsByWhereClause("oauthconnect", squirrel.Eq{"name": authenticator})

  if err != nil {
    log.Errorf("Failed to get oauth connection details for  [%v]", authenticator)
    return nil, []error{err}
  }

  if len(rows) < 1 {
    log.Errorf("Failed to get oauth connection details for  [%v]", authenticator)
    err = errors.New(fmt.Sprintf("No such authenticator [%v]", authenticator))
    return nil, []error{err}
  }

  authConnectorData := rows[0]

  conf := &oauth2.Config{
    ClientID:     authConnectorData["client_id"].(string),
    ClientSecret: authConnectorData["client_secret"].(string),
    RedirectURL:  "http://site.goms.com:8080/oauth/response",
    Scopes:       []string{"https://www.googleapis.com/auth/spreadsheets"},
    Endpoint: oauth2.Endpoint{
      AuthURL:  "https://accounts.google.com/o/oauth2/auth",
      TokenURL: "https://accounts.google.com/o/oauth2/token",
    },
  }

  // Redirect user to consent page to ask for permission
  // for the scopes specified above.
  url := conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
  fmt.Printf("Visit the URL for the auth dialog: %v", url)

  responseAttrs := make(map[string]interface{})

  responseAttrs["location"] = url
  responseAttrs["window"] = "self"
  responseAttrs["delay"] = 0

  actionResponse := NewActionResponse("client.redirect", responseAttrs)

  return []ActionResponse{actionResponse}, nil
}

func NewOauthLoginBeginActionPerformer(initConfig *CmsConfig, cruds map[string]*DbResource, configStore *ConfigStore) (ActionPerformerInterface, error) {

  secret, err := configStore.GetConfigValueFor("otp.secret", "backend")
  if err != nil {
    key, err := totp.Generate(totp.GenerateOpts{
      Issuer:      "site.goms.com",
      AccountName: "dummy@site.goms.com",
      Period:      300,
      SecretSize:  10,
    })

    if err != nil {
      log.Errorf("Failed to generate code: %v", err)
      return nil, err
    }
    configStore.SetConfigValueFor("otp.secret", key.Secret(), "backend")
    secret = key.Secret()
  }

  handler := OauthLoginBeginActionPerformer{
    cruds:  cruds,
    otpKey: secret,
  }

  return &handler, nil

}
