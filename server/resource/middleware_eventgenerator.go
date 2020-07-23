package resource

import (
	"context"
	"github.com/artpar/api2go"
	log "github.com/sirupsen/logrus"
	"strings"
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

	reqmethod := req.PlainRequest.Method
	//log.Infof("Generate events for objects", reqmethod)
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

	return objects, nil

}

type exchangeMiddleware struct {
	cmsConfig   *CmsConfig
	exchangeMap map[string][]ExchangeContract
	cruds       *map[string]*DbResource
}

func (em *exchangeMiddleware) String() string {
	return "exchangeMiddleware"
}

// Creates a new exchange middleware which is responsible for calling external apis on data updates
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

	return &exchangeMiddleware{
		cmsConfig:   cmsConfig,
		exchangeMap: exchangeMap,
		cruds:       cruds,
	}
}

// Intercept before does nothing for exchange middleware and the calls are made only if data update was successful
func (em *exchangeMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request, objects []map[string]interface{}) ([]map[string]interface{}, error) {
	return nil, nil
}

// Called after the data changes are complete, resposible for calling the external api.
func (em *exchangeMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	//errors := []error{}

	reqmethod := req.PlainRequest.Method
	//log.Infof("Request to intercept in middleware exchange: %v", reqmethod)
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

					if exchange.OauthTokenId == nil {
						log.Infof("Oauth token for exchange [%v] is not set", exchange.Name)
						continue
					}

					token, err := dr.GetTokenByTokenId(*exchange.OauthTokenId)
					if err != nil {
						log.Errorf("No token selected for [%v][%v]: %v", exchange.Name, exchange.OauthTokenId, err)
					}

					oauthDesc, err := dr.GetOauthDescriptionByTokenId(*exchange.OauthTokenId)

					if err != nil {
						err = nil
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

						err = dr.UpdateAccessTokenByTokenId(*exchange.OauthTokenId, token.AccessToken, token.Expiry.Unix())
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
