package resource

import (
	"errors"
	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/gommon/log"
)

type actionTransactionPerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
}

func (d *actionTransactionPerformer) Name() string {
	return "$transaction"
}

func (d *actionTransactionPerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	action, ok := inFields["action"].(string)
	if !ok {
		return nil, nil, []error{errors.New("action is required")}
	}

	var err error
	switch action {
	case "commit":
		err = transaction.Commit()
	case "rollback":
		err = transaction.Rollback()
	case "query":
		statement, err := transaction.Preparex(inFields["query"].(string))
		if err != nil {
			return nil, nil, []error{err}
		}

		rows, err := statement.Queryx(inFields["arguments"].([]interface{})...)
		if err != nil {
			return nil, nil, []error{err}
		}
		typeName := inFields["typeName"].(string)
		result, err := RowsToMap(rows, typeName)
		if err != nil {
			return nil, nil, []error{err}
		}
		return nil, []ActionResponse{NewActionResponse(typeName, result)}, nil

	case "begin":
		var newTx *sqlx.Tx
		newTx, err = d.cruds["user_account"].Connection.Beginx()
		if err != nil {
			return nil, nil, []error{err}
		}
		*transaction = *newTx

	}

	if err != nil {
		log.Errorf("Failed to commit transaction: [%v]", err)
		return nil, nil, []error{err}
	}
	return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Column deleted", "Success"))}, nil
}

func NewActionCommitTransactionPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := actionTransactionPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
