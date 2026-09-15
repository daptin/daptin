package actions

import (
	"sync"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

type dataExchangeExecutionProcessPerformer struct {
	executions *resource.ExchangeExecutionService
	running    sync.Mutex
}

func (performer *dataExchangeExecutionProcessPerformer) Name() string {
	return "data_exchange.execution.process"
}

func (performer *dataExchangeExecutionProcessPerformer) DoAction(_ actionresponse.Outcome,
	_ map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if !performer.running.TryLock() {
		return nil, nil, nil
	}
	defer performer.running.Unlock()
	if err := performer.executions.ProcessPending(transaction); err != nil {
		return nil, nil, []error{err}
	}
	return nil, nil, nil
}

func NewDataExchangeExecutionProcessPerformer(config *resource.CmsConfig,
	cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &dataExchangeExecutionProcessPerformer{
		executions: resource.NewExchangeExecutionService(config, &cruds),
	}, nil
}
