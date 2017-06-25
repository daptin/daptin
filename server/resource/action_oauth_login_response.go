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
)

type OauthLoginResponseActionPerformer struct {
  responseAttrs map[string]interface{}
  cruds         map[string]*DbResource
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

  code := inFieldMap["code"].(string)

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
  storeToken["oauthconnect_id"] = authConnectorData["reference_id"]

  pr := &http.Request{
    Method: "POST",
  }

  req := api2go.Request{
    PlainRequest: pr,
  }

  gorillaContext.Set(pr, "user_id", user["reference_id"])
  gorillaContext.Set(pr, "usergroup_id", []auth.GroupPermission{})
  gorillaContext.Set(pr, "user_id_integer", user["id"])

  model := api2go.NewApi2GoModelWithData("oauthtoken", nil, auth.DEFAULT_PERMISSION, nil, storeToken)

  _, err = d.cruds["oauthtoken"].Create(model, req)
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
  redirectAttrs["location"] = "/in/oauthtoken"
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
    cruds:  cruds,
    otpKey: secret,
  }

  return &handler, nil

}
