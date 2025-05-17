package resource

import (
	"github.com/artpar/api2go/v2"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
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

			if exc.Attributes["name"] == nil {
				continue
			}

			m, ok := exchangeMap[exc.Attributes["name"].(string)]
			if !ok {
				m = make([]ExchangeContract, 0)
			}

			m = append(m, exc)
			exchangeMap[exc.Attributes["name"].(string)] = m
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
func (em *exchangeMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request, results []map[string]interface{}, transaction *sqlx.Tx) ([]map[string]interface{}, error) {

	reqmethod := req.PlainRequest.Method
	reqmethod = strings.ToLower(reqmethod)
	log.Tracef("[75] Request to intercept in middleware exchange: [%v]", reqmethod)

	for i, resultRow := range results {

		typ, ok := resultRow["__type"]

		if !ok || typ == nil {
			continue
		}
		resultType := resultRow["__type"].(string)

		exchanges, ok := em.exchangeMap[resultType]

		if ok {
			//log.Printf("Got %d exchanges for [%v]", len(exchanges), resultType)
		} else {
			continue
		}

		for _, exchange := range exchanges {

			hook, ok := exchange.Attributes["hook"]
			if !ok || hook == "" {
				log.Warnf("hook value not present in exchange: %v", exchange.Name)
				continue
			}
			hookEvent := hook.(string)
			if hookEvent != "before" {
				continue
			}
			methods := exchange.Attributes["methods"].([]interface{})
			if !InArray(methods, reqmethod) {
				continue
			}

			//client := oauthDesc.Client(ctx, token)

			log.Printf("executing exchange in routine: %v -> %v", exchange.SourceType, exchange.TargetType)
			exchangeExecution := NewExchangeExecution(exchange, em.cruds)

			exchangeResult, err := exchangeExecution.Execute([]map[string]interface{}{resultRow}, transaction)
			if err != nil {
				log.Errorf("Failed to execute exchange: %v", err)
				//errors = append(errors, err)
			} else {

				if exchange.Attributes != nil && len(exchange.Attributes) > 0 {
					resultValue, err := BuildActionContext(exchange.Attributes, exchangeResult)
					if err != nil {
						resultMap := resultValue.(map[string]interface{})
						for key, val := range resultMap {
							exchangeResult[key] = val
						}
					}
					results[i] = exchangeResult
				}

			}
		}
	}
	log.Tracef("[135] Finished request to intercept in middleware exchange: %v => [%v]", reqmethod, results)
	return results, nil
}

// Called after the data changes are complete, resposible for calling the external api.
func (em *exchangeMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request,
	results []map[string]interface{}, transaction *sqlx.Tx) ([]map[string]interface{}, error) {

	//errors := []error{}

	reqmethod := req.PlainRequest.Method
	reqmethod = strings.ToLower(reqmethod)
	log.Tracef("[145] Request to intercept in middleware exchange: [%v]%v", em, reqmethod)

	for _, resultRow := range results {

		typ, ok := resultRow["__type"]

		if !ok || typ == nil {
			continue
		}
		resultType := resultRow["__type"].(string)

		exchanges, ok := em.exchangeMap[resultType]

		if ok {
			//log.Printf("Got %d exchanges for [%v]", len(exchanges), resultType)
		} else {
			continue
		}

		for _, exchange := range exchanges {

			hook, ok := exchange.Attributes["hook"]
			if !ok || hook == "" || hook == nil {
				log.Warnf("hook value not present in exchange: %v", exchange.Name)
				continue
			}

			hookEvent := hook.(string)
			if hookEvent != "after" {
				continue
			}

			methods := exchange.Attributes["methods"].([]interface{})
			if !InArray(methods, reqmethod) {
				continue
			}

			//client := oauthDesc.Client(ctx, token)

			log.Printf("executing exchange in routine: %v -> %v", exchange.SourceType, exchange.TargetType)
			exchangeExecution := NewExchangeExecution(exchange, em.cruds)

			exchangeResult, err := exchangeExecution.Execute([]map[string]interface{}{resultRow}, transaction)
			if err != nil {
				log.Errorf("Failed to execute exchange: %v", err)
				//errors = append(errors, err)
			} else {

				if exchange.Attributes != nil && len(exchange.Attributes) > 0 {
					resultValue, err := BuildActionContext(exchange.Attributes, exchangeResult)
					if err != nil {
						resultMap := resultValue.(map[string]interface{})
						for key, val := range resultMap {
							exchangeResult[key] = val
						}
					}
				}

			}
		}
	}

	log.Tracef("[208] Completed request to intercept in middleware exchange: %v => %v", reqmethod, results)
	return results, nil
}
