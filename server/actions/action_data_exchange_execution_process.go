package actions

import (
	"fmt"
	"sync"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

type dataExchangeExecutionProcessPerformer struct {
	executions *resource.ExchangeExecutionService
	running    sync.Mutex
}

type dataExchangeExecutionRetryPerformer struct {
	executions *resource.ExchangeExecutionService
}

func (performer *dataExchangeExecutionRetryPerformer) Name() string {
	return "data_exchange.execution.retry"
}

func (performer *dataExchangeExecutionRetryPerformer) DoAction(_ actionresponse.Outcome,
	inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	subject, ok := inFields["subject"].(map[string]interface{})
	if !ok {
		return nil, nil, []error{fmt.Errorf("data exchange retry requires an execution subject")}
	}
	referenceID := daptinid.InterfaceToDIR(subject["reference_id"])
	if err := performer.executions.Retry(referenceID, transaction); err != nil {
		return nil, nil, []error{err}
	}
	return nil, nil, nil
}

func (performer *dataExchangeExecutionProcessPerformer) Name() string {
	return "data_exchange.execution.process"
}

func NewDataExchangeExecutionRetryPerformer(config *resource.CmsConfig,
	cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &dataExchangeExecutionRetryPerformer{
		executions: resource.NewExchangeExecutionService(config, &cruds),
	}, nil
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
