package resource

import (
  "fmt"
  "github.com/pkg/errors"
  log "github.com/sirupsen/logrus"
  "github.com/pquerna/otp/totp"
  "gopkg.in/Masterminds/squirrel.v1"
  "golang.org/x/oauth2"
  "context"
  gorillaContext "github.com/gorilla/context"
  "github.com/artpar/api2go"
  "net/http"
  "github.com/artpar/goms/server/auth"
  "strings"
)

type OauthLoginResponseActionPerformer struct {
  responseAttrs map[string]interface{}
  cruds         map[string]*DbResource
  configStore   *ConfigStore
  otpKey        string
}

func (d *OauthLoginResponseActionPerformer) Name() string {
  return "oauth.login.response"
}

func (d *OauthLoginResponseActionPerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) ([]ActionResponse, []error) {

  state := inFieldMap["state"].(string)
  user := inFieldMap["user"].(map[string]interface{})

  ok := totp.Validate(state, d.otpKey)
  if !ok {
    log.Errorf("Failed to validate otp key")
    return nil, []error{errors.New("No ongoing authentication")}
  }

  authenticator := inFieldMap["authenticator"].(string)

  rows, _, err := d.cruds["oauth_connect"].GetRowsByWhereClause("oauth_connect", squirrel.Eq{"name": authenticator})

  if err != nil {
    log.Errorf("Failed to get oauth connection details for in response handler  [%v]", authenticator)
    return nil, []error{err}
  }

  if len(rows) < 1 {
    log.Errorf("Failed to get oauth connection details for  [%v]", authenticator)
    err = errors.New(fmt.Sprintf("No such authenticator [%v]", authenticator))
    return nil, []error{err}
  }

  authConnectorData := rows[0]

  code := inFieldMap["code"].(string)

  redirectUri := authConnectorData["redirect_uri"].(string)

  if strings.Index(redirectUri, "?") > -1 {
    redirectUri = redirectUri + "&authenticator=" + authenticator
  } else {
    redirectUri = redirectUri + "?authenticator=" + authenticator
  }

  secret, err := d.configStore.GetConfigValueFor("encryption.secret", "backend")
  if err != nil {
    log.Errorf("Failed to get secret: %v", err)
    return nil, []error{err}
  }

  clientSecretEncrypted := authConnectorData["client_secret"].(string)
  clientSecretPlainText, err := Decrypt([]byte(secret), clientSecretEncrypted)
  if err != nil {
    log.Errorf("Failed to get decrypt text: %v", err)
    return nil, []error{err}
  }

  conf := &oauth2.Config{
    ClientID:     authConnectorData["client_id"].(string),
    ClientSecret: clientSecretPlainText,
    RedirectURL:  redirectUri,
    Scopes:       []string{"https://www.googleapis.com/auth/spreadsheets"},
    Endpoint: oauth2.Endpoint{
      AuthURL:  authConnectorData["auth_url"].(string),
      TokenURL: authConnectorData["token_url"].(string),
    },
  }

  ctx := context.Background()
  token, err := conf.Exchange(ctx, code)
  if err != nil {
    log.Errorf("Failed to exchange code for token: %v", err)
    return nil, []error{err}
  }

  storeToken := make(map[string]interface{})

  storeToken["access_token"] = token.AccessToken
  storeToken["refresh_token"] = token.RefreshToken
  storeToken["expires_in"] = token.Expiry.Unix()
  storeToken["token_type"] = "auth_token"
  storeToken["oauth_connect_id"] = authConnectorData["reference_id"]

  pr := &http.Request{
    Method: "POST",
  }

  req := api2go.Request{
    PlainRequest: pr,
  }

  gorillaContext.Set(pr, "user_id", user["reference_id"])
  gorillaContext.Set(pr, "usergroup_id", []auth.GroupPermission{})
  gorillaContext.Set(pr, "user_id_integer", user["id"])

  model := api2go.NewApi2GoModelWithData("oauth_token", nil, auth.DEFAULT_PERMISSION, nil, storeToken)

  _, err = d.cruds["oauth_token"].Create(model, req)
  if err != nil {
    log.Errorf("Failed to store oauth token: %v", err)
    return nil, []error{err}
  }

  responseAttrs := make(map[string]interface{})

  responseAttrs["title"] = "Successfully connected"
  responseAttrs["message"] = "You can use this connection now"
  responseAttrs["type"] = "success"

  actionResponse := NewActionResponse("client.notify", responseAttrs)

  redirectAttrs := make(map[string]interface{})
  redirectAttrs["delay"] = 0
  redirectAttrs["location"] = "/in/oauth_token"
  redirectAttrs["window"] = "self"
  redirectResponse := NewActionResponse("client.redirect", redirectAttrs)

  return []ActionResponse{actionResponse, redirectResponse}, nil
}

func NewOauthLoginResponseActionPerformer(initConfig *CmsConfig, cruds map[string]*DbResource, configStore *ConfigStore) (ActionPerformerInterface, error) {

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

  handler := OauthLoginResponseActionPerformer{
    cruds:       cruds,
    otpKey:      secret,
    configStore: configStore,
  }

  return &handler, nil

}
