package resource

import (
	"github.com/artpar/api2go"
	//"errors"
	log "github.com/sirupsen/logrus"
	"strings"
	//"github.com/lann/ps"
	"golang.org/x/oauth2"
	"gopkg.in/Masterminds/squirrel.v1"
	"time"
	"context"
)

type eventHandlerMiddleware struct {
}

func (pc eventHandlerMiddleware) String() string {
	return "EventGenerator"
}

func (pc *eventHandlerMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	switch strings.ToLower(req.PlainRequest.Method) {
	case "get":
		break
	case "post":
		break
	case "update":
		break
	case "delete":
		break
	case "patch":
		break
	default:
		log.Errorf("Invalid method: %v", req.PlainRequest.Method)
	}

	return results, nil

}

func (pc *eventHandlerMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request, objects []map[string]interface{}) ([]map[string]interface{}, error) {

	var err error = nil

	reqmethod := req.PlainRequest.Method
	log.Infof("Event generator handle new request to intercept: %v", reqmethod)
	switch reqmethod {
	case "GET":
		break
	case "POST":
		break
	case "UPDATE":
		break
	case "DELETE":
		break
	case "PATCH":
		break
	default:
		log.Errorf("Invalid method: %v", reqmethod)
	}

	//currentUserId := context.Get(req.PlainRequest, "user_id").(string)
	//currentUserGroupId := context.Get(req.PlainRequest, "usergroup_id").([]string)

	return objects, err

}

type ExchangeMiddleware struct {
	cmsConfig   *CmsConfig
	exchangeMap map[string][]ExchangeContract
	cruds       *map[string]*DbResource
}

func (em *ExchangeMiddleware) String() string {
	return "ExchangeMiddleware"
}

func NewExchangeMiddleware(cmsConfig *CmsConfig, cruds *map[string]*DbResource) DatabaseRequestInterceptor {

	exchangeMap := make(map[string][]ExchangeContract)

	for _, exc := range cmsConfig.ExchangeContracts {

		if exc.SourceType == "self" {

			if exc.SourceAttributes["name"] == nil {
				continue
			}

			m, ok := exchangeMap[exc.SourceAttributes["name"].(string)]
			if !ok {
				m = make([]ExchangeContract, 0)
			}

			m = append(m, exc)
			exchangeMap[exc.SourceAttributes["name"].(string)] = m
		} else if exc.TargetType == "self" {
			m, ok := exchangeMap[exc.TargetAttributes["name"].(string)]
			if !ok {
				m = make([]ExchangeContract, 0)
			}

			m = append(m, exc)
			exchangeMap[exc.TargetAttributes["name"].(string)] = m
		}

	}

	return &ExchangeMiddleware{
		cmsConfig:   cmsConfig,
		exchangeMap: exchangeMap,
		cruds:       cruds,
	}
}

func (em *ExchangeMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request, objects []map[string]interface{}) ([]map[string]interface{}, error) {
	return nil, nil
}
func (em *ExchangeMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	//errors := []error{}

	reqmethod := req.PlainRequest.Method
	log.Infof("Request to intercept in middleware exchange: %v", reqmethod)
	switch reqmethod {
	case "GET":
		break
	case "POST":

		if len(results) > 0 {

			for _, result := range results {

				typ, ok := result["__type"]

				if !ok || typ == nil {
					continue
				}
				resultType := result["__type"].(string)

				exchanges, ok := em.exchangeMap[resultType]

				if ok {
					log.Infof("Got %d exchanges for [%v]", len(exchanges), resultType)
				}

				for _, exchange := range exchanges {
					token, err := dr.GetTokenForExchangeByTokenId(exchange.OauthTokenId)
					if err != nil {
						log.Errorf("No token selected for [%v][%v]: %v", exchange.Name, exchange.OauthTokenId, err)
					}

					oauthDesc, err := dr.GetOauthDescriptionByTokenId(exchange.OauthTokenId)

					if err != nil {
						log.Errorf("No oauth description for [%v][%v]: %v", exchange.Name, exchange.OauthTokenId, err)
					}
					ctx := context.Background()

					if !token.Valid() {
						tokenSource := oauthDesc.TokenSource(ctx, token)
						token, err = tokenSource.Token()
						CheckErr(err, "Failed to get new access token")
						if err != nil {
							return results, err
						}

						err = dr.UpdateAccessToken(exchange.OauthTokenId, token.AccessToken, token.Expiry.Unix())
						CheckErr(err, "failed to update access token")
					}

					if err != nil {
						return results, err
					}

					//client := oauthDesc.Client(ctx, token)

					exchangeExecution := NewExchangeExecution(exchange, token, oauthDesc)

					inFields := make(map[string]interface{})

					err = exchangeExecution.Execute(inFields, []map[string]interface{}{result})
					if err != nil {
						log.Errorf("Failed to execute exchange: %v", err)
						//errors = append(errors, err)
					}

				}

			}

		}

		break
	case "UPDATE":
		break
	case "DELETE":
		break
	case "PATCH":
		break
	default:
		log.Errorf("Invalid method: %v", reqmethod)
	}

	return results, nil
}

func (resource *DbResource) GetTokenForExchangeByTokenId(id *int64) (*oauth2.Token, error) {

	var access_token, refresh_token, token_type string
	var expires_in int64
	var token oauth2.Token
	s, v, err := squirrel.Select("access_token", "refresh_token", "token_type", "expires_in").From("oauth_token").
			Where(squirrel.Eq{"deleted_at": nil}).Where(squirrel.Eq{"id": id}).ToSql()

	if err != nil {
		return nil, err
	}

	err = resource.db.QueryRowx(s, v...).Scan(&access_token, &refresh_token, &token_type, &expires_in)

	if err != nil {
		return nil, err
	}

	secret, err := resource.configStore.GetConfigValueFor("encryption.secret", "backend")
	CheckErr(err, "Failed to get encryption secret")

	dec, err := Decrypt([]byte(secret), access_token)
	CheckErr(err, "Failed to decrypt access token")

	ref, err := Decrypt([]byte(secret), refresh_token)
	CheckErr(err, "Failed to decrypt refresh token")

	token.AccessToken = dec
	token.RefreshToken = ref
	token.TokenType = token_type
	token.Expiry = time.Unix(expires_in, 0)

	return &token, err

}

func (resource *DbResource) UpdateAccessToken(id *int64, accessToken string, expiresIn int64) (error) {

	encryptionSecret, err := resource.configStore.GetConfigValueFor("encryption.secret", "backend")
	if err != nil {
		return err
	}

	accessToken, err = Encrypt([]byte(encryptionSecret), accessToken)
	if err != nil {
		return err
	}

	s, v, err := squirrel.Update("oauth_token").
			Set("access_token", accessToken).
			Set("expires_in", expiresIn).
			Where(squirrel.Eq{"id": id}).ToSql()

	if err != nil {
		return err
	}

	_, err = resource.db.Exec(s, v...)
	return err

}

func (resource *DbResource) GetOauthDescriptionByTokenId(id *int64) (*oauth2.Config, error) {

	var clientId, clientSecret, redirectUri, authUrl, tokenUrl, scope string

	s, v, err := squirrel.
	Select("oc.client_id", "oc.client_secret", "oc.redirect_uri", "oc.auth_url", "oc.token_url", "oc.scope").
			From("oauth_token ot").Join("oauth_connect oc").
			JoinClause("on oc.id = ot.oauth_connect_id").
			Where(squirrel.Eq{"ot.deleted_at": nil}).Where(squirrel.Eq{"ot.id": id}).ToSql()

	if err != nil {
		return nil, err
	}

	err = resource.db.QueryRowx(s, v...).Scan(&clientId, &clientSecret, &redirectUri, &authUrl, &tokenUrl, &scope)

	if err != nil {
		return nil, err
	}

	encryptionSecret, err := resource.configStore.GetConfigValueFor("encryption.secret", "backend")
	if err != nil {
		return nil, err
	}

	clientSecret, err = Decrypt([]byte(encryptionSecret), clientSecret)
	if err != nil {
		return nil, err
	}

	conf := &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUri,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authUrl,
			TokenURL: tokenUrl,
		},
		Scopes: strings.Split(scope, ","),
	}

	return conf, nil

}
