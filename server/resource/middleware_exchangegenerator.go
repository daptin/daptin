package resource

import (
	"github.com/artpar/api2go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type exchangeMiddleware struct {
	cmsConfig     *CmsConfig
	exchangeMap   map[string][]ExchangeContract
	cruds         *map[string]*DbResource
	actionHandler *func(*gin.Context)
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

					//client := oauthDesc.Client(ctx, token)

					go func(exchange ExchangeContract) {

						log.Printf("executing exchange in routine: %v -> %v", exchange.SourceType, exchange.TargetType)
						exchangeExecution := NewExchangeExecution(exchange, em.cruds)

						err := exchangeExecution.Execute([]map[string]interface{}{result})
						if err != nil {
							log.Errorf("Failed to execute exchange: %v", err)
							//errors = append(errors, err)
						}
					}(exchange)
				}
			}
		}

		break
	case "UPDATE":
		fallthrough
	case "PATCH":
		break
	case "DELETE":
		break
	default:
		log.Errorf("Invalid method: %v", reqmethod)
	}

	return results, nil
}
