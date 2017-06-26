package resource

import (
  "fmt"
  log "github.com/sirupsen/logrus"
  "golang.org/x/oauth2"
  "github.com/pquerna/otp/totp"
  "time"
  "strings"
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

func (d *OauthLoginBeginActionPerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) ([]ActionResponse, []error) {

  state, err := totp.GenerateCode(d.otpKey, time.Now())
  if err != nil {
    log.Errorf("Failed to generate code: %v", err)
    return nil, []error{err}
  }

  scope := inFieldMap["scope"].(string)
  authConnectorData := inFieldMap["subject"].(map[string]interface{})

  //rows, _, err := d.cruds["oauth_connect"].GetRowsByWhereClause("oauth_connect", squirrel.Eq{"name": authenticator})

  //if err != nil {
  //  log.Errorf("Failed to get oauth connection details for in begin login  [%v]", authenticator)
  //  return nil, []error{err}
  //}
  //
  //if len(rows) < 1 {
  //  log.Errorf("Failed to get oauth connection details for  [%v]", authenticator)
  //  err = errors.New(fmt.Sprintf("No such authenticator [%v]", authenticator))
  //  return nil, []error{err}
  //}

  //authConnectorData := rows[0]

  redirectUri := authConnectorData["redirect_uri"].(string)

  if strings.Index(redirectUri, "?") > -1 {
    redirectUri = redirectUri + "&authenticator=" + authConnectorData["name"].(string)
  } else {
    redirectUri = redirectUri + "?authenticator=" + authConnectorData["name"].(string)
  }

  clientSecretEncrypted := authConnectorData["client_secret"].(string)

  secret, err := d.configStore.GetConfigValueFor("encryption.secret", "backend")
  if err != nil {
    log.Errorf("Failed to get secret: %v", err)
    return nil, []error{err}
  }

  clientSecretPlainText, err := Decrypt([]byte(secret), clientSecretEncrypted)
  if err != nil {
    log.Errorf("Failed to get decrypt text: %v", err)
    return nil, []error{err}
  }

  conf := &oauth2.Config{
    ClientID:     authConnectorData["client_id"].(string),
    ClientSecret: clientSecretPlainText,
    RedirectURL:  redirectUri,
    Scopes:       strings.Split(scope, ","),
    Endpoint: oauth2.Endpoint{
      AuthURL:  authConnectorData["auth_url"].(string),
      TokenURL: authConnectorData["token_url"].(string),
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
    cruds:       cruds,
    otpKey:      secret,
    configStore: configStore,
  }

  return &handler, nil

}
