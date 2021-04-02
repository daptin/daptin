package resource

import (
	"github.com/artpar/api2go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"strings"
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

	hasExchange := make(map[string]bool)

	for i := range cmsConfig.ExchangeContracts {
		exc := cmsConfig.ExchangeContracts[len(cmsConfig.ExchangeContracts)-i-1]

		if hasExchange[exc.Name] {
			continue
		}

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
			hasExchange[exc.Name] = true
		} else if exc.TargetType == "self" {
			m, ok := exchangeMap[exc.TargetAttributes["name"].(string)]
			if !ok {
				m = make([]ExchangeContract, 0)
			}

			m = append(m, exc)
			exchangeMap[exc.TargetAttributes["name"].(string)] = m
			hasExchange[exc.Name] = true
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
	reqmethod = strings.ToLower(reqmethod)
	//log.Infof("Request to intercept in middleware exchange: %v", reqmethod)

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
			} else {
				continue
			}

			for _, exchange := range exchanges {

				methods := exchange.SourceAttributes["methods"].([]interface{})
				if !InArray(methods, reqmethod) {
					continue
				}

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

	return results, nil
}
