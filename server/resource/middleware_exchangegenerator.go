package resource

import (
	"fmt"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

const (
	exchangeOnErrorContinue = "continue"
	exchangeOnErrorRetry    = "retry"
	exchangeOnErrorError    = "error"
)

type exchangeMiddleware struct {
	cmsConfig     *CmsConfig
	exchangeMap   map[string][]ExchangeContract
	cruds         *map[string]*DbResource
	executions    *ExchangeExecutionService
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
		executions:  NewExchangeExecutionService(cmsConfig, cruds),
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

			log.Printf("executing before exchange: %v -> %v", exchange.SourceType, exchange.TargetType)
			exchangeResult, succeeded, err := em.execute(exchange, reqmethod, resultRow, *req, transaction)
			if err != nil {
				return nil, err
			}
			if succeeded {
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

			log.Printf("executing after exchange: %v -> %v", exchange.SourceType, exchange.TargetType)
			exchangeResult, succeeded, err := em.execute(exchange, reqmethod, resultRow, *req, transaction)
			if err != nil {
				return nil, err
			}
			if succeeded {
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

func (em *exchangeMiddleware) execute(exchange ExchangeContract, method string, row map[string]interface{},
	request api2go.Request, transaction *sqlx.Tx) (map[string]interface{}, bool, error) {
	policy, err := exchangeErrorPolicy(exchange)
	if err != nil {
		return nil, false, err
	}
	if policy == exchangeOnErrorRetry && transaction != nil {
		if _, err := transaction.Exec("SAVEPOINT data_exchange_attempt"); err != nil {
			return nil, false, err
		}
	}

	result, executionErr := NewExchangeExecution(exchange, em.cruds).Execute([]map[string]interface{}{row}, transaction)
	if executionErr == nil {
		if policy == exchangeOnErrorRetry && transaction != nil {
			if _, err := transaction.Exec("RELEASE SAVEPOINT data_exchange_attempt"); err != nil {
				return nil, false, err
			}
		}
		return result, true, nil
	}

	log.Errorf("Failed to execute exchange: %v", executionErr)
	switch policy {
	case exchangeOnErrorContinue:
		return nil, false, nil
	case exchangeOnErrorError:
		return nil, false, executionErr
	case exchangeOnErrorRetry:
		if transaction == nil {
			return nil, false, fmt.Errorf("retry exchange [%s] requires an active transaction", exchange.Name)
		}
		if _, err := transaction.Exec("ROLLBACK TO SAVEPOINT data_exchange_attempt"); err != nil {
			return nil, false, err
		}
		if _, err := transaction.Exec("RELEASE SAVEPOINT data_exchange_attempt"); err != nil {
			return nil, false, err
		}
		if !isMutationMethod(method) {
			return nil, false, fmt.Errorf("retry exchange [%s] requires a mutation method", exchange.Name)
		}
		if err := em.enqueue(exchange, method, row, request, transaction); err != nil {
			return nil, false, fmt.Errorf("enqueue data exchange retry [%s]: %w", exchange.Name, err)
		}
		return nil, false, nil
	default:
		panic("unreachable exchange error policy")
	}
}

func (em *exchangeMiddleware) enqueue(exchange ExchangeContract, method string, row map[string]interface{},
	request api2go.Request, transaction *sqlx.Tx) error {
	if _, err := transaction.Exec("SAVEPOINT data_exchange_enqueue"); err != nil {
		return err
	}
	if err := em.executions.Enqueue(exchange, method, row, request, transaction); err != nil {
		if _, rollbackErr := transaction.Exec("ROLLBACK TO SAVEPOINT data_exchange_enqueue"); rollbackErr != nil {
			return rollbackErr
		}
		if _, releaseErr := transaction.Exec("RELEASE SAVEPOINT data_exchange_enqueue"); releaseErr != nil {
			return releaseErr
		}
		return err
	}
	_, err := transaction.Exec("RELEASE SAVEPOINT data_exchange_enqueue")
	return err
}

func exchangeErrorPolicy(exchange ExchangeContract) (string, error) {
	if exchange.Options == nil || exchange.Options["on_error"] == nil {
		return exchangeOnErrorContinue, nil
	}
	policy, ok := exchange.Options["on_error"].(string)
	if !ok {
		return "", fmt.Errorf("data exchange [%s] option on_error must be a string", exchange.Name)
	}
	switch policy {
	case exchangeOnErrorContinue, exchangeOnErrorRetry, exchangeOnErrorError:
		return policy, nil
	default:
		return "", fmt.Errorf("data exchange [%s] has invalid on_error policy [%s]", exchange.Name, policy)
	}
}

func isMutationMethod(method string) bool {
	switch strings.ToLower(method) {
	case "post", "put", "patch", "delete":
		return true
	default:
		return false
	}
}
