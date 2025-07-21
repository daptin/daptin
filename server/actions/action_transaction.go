package actions

import (
	"errors"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type actionTransactionPerformer struct {
	cmsConfig *resource.CmsConfig
	cruds     map[string]*resource.DbResource
}

func (d *actionTransactionPerformer) Name() string {
	return "$transaction"
}

func (d *actionTransactionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

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
		query := inFields["query"].(string)
		queryArgs := inFields["arguments"].([]interface{})
		log.Tracef("$transaction.query [%s][%v]", query, queryArgs)
		statement, err := transaction.Preparex(query)
		if err != nil {
			return nil, nil, []error{err}
		}
		defer statement.Close()

		rows, err := statement.Queryx(queryArgs...)
		if err != nil {
			log.Errorf("$transaction query failed [%v]", err.Error())
			return nil, nil, []error{err}
		}
		defer rows.Close()
		typeName := inFields["typeName"].(string)
		result, err := resource.RowsToMap(rows, typeName)
		if err != nil {
			log.Errorf("$transaction rowstomap failed [%v]", err.Error())
			return nil, nil, []error{err}
		}
		return nil, []actionresponse.ActionResponse{resource.NewActionResponse(typeName, result)}, nil

	case "begin":
		var newTx *sqlx.Tx
		newTx, err = d.cruds["user_account"].Connection().Beginx()
		if err != nil {
			return nil, nil, []error{err}
		}
		*transaction = *newTx

	}

	if err != nil {
		log.Errorf("[64] Failed to commit transaction: [%v]", err)
		return nil, nil, []error{err}
	}
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("client.notify", resource.NewClientNotification("message", "Column deleted", "Success"))}, nil
}

func NewActionCommitTransactionPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := actionTransactionPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
