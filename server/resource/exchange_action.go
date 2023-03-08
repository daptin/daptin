package resource

import (
	"context"
	"errors"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type ActionExchangeHandler struct {
	cruds            map[string]*DbResource
	exchangeContract ExchangeContract
}

func (exchangeHandler *ActionExchangeHandler) ExecuteTarget(row map[string]interface{}, transaction *sqlx.Tx) (map[string]interface{}, error) {

	log.Printf("Execute action exchange on: %v - %v", row["__type"], row["reference_id"])

	targetType, ok := exchangeHandler.exchangeContract.TargetAttributes["type"]
	if !ok {
		log.Warnf("target type value not present in action exchange: %v", exchangeHandler.exchangeContract.Name)
	}
	tableName := targetType.(string)
	targetAttributes := exchangeHandler.exchangeContract.TargetAttributes["attributes"]
	if targetAttributes == nil {
		targetAttributes = make(map[string]interface{})
	}
	request := ActionRequest{
		Type:       tableName,
		Action:     exchangeHandler.exchangeContract.TargetAttributes["action"].(string),
		Attributes: targetAttributes.(map[string]interface{}),
	}
	//
	//if exchangeHandler.exchangeContract.SourceType == row["__type"] {
	//	request.Attributes[exchangeHandler.exchangeContract.SourceType+"_id"] = row["reference_id"]
	//}

	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "POST",
		},
	}

	userRow, _, err := exchangeHandler.cruds[USER_ACCOUNT_TABLE_NAME].GetSingleRowById(USER_ACCOUNT_TABLE_NAME, exchangeHandler.exchangeContract.AsUserId, nil, transaction)
	if err != nil {
		return nil, errors.New("user account not found to execute data exchange with action")
	}
	userReferenceId := userRow["reference_id"].(string)

	query, args1, err := auth.UserGroupSelectQuery.Where(goqu.Ex{"uug.user_account_id": exchangeHandler.exchangeContract.AsUserId}).ToSQL()

	stmt1, err := transaction.Preparex(query)
	if err != nil {
		log.Errorf("[59] failed to prepare statment: %v", err)
	}

	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(args1...)
	userGroups := make([]auth.GroupPermission, 0)

	if err != nil {
		log.Errorf("Failed to get user group permissions: %v", err)
	} else {
		defer rows.Close()
		//cols, _ := rows.Columns()
		//log.Printf("Columns: %v", cols)
		for rows.Next() {
			var p auth.GroupPermission
			err = rows.StructScan(&p)
			p.ObjectReferenceId = userReferenceId
			if err != nil {
				log.Errorf("failed to scan group permission struct: %v", err)
				continue
			}
			userGroups = append(userGroups, p)
		}

	}

	sessionUser := auth.SessionUser{
		UserId:          exchangeHandler.exchangeContract.AsUserId,
		UserReferenceId: userReferenceId,
		Groups:          userGroups,
	}

	req.PlainRequest = req.PlainRequest.WithContext(context.WithValue(context.Background(), "user", &sessionUser))

	request.Attributes["subject"] = row
	request.Attributes[tableName+"_id"] = row["reference_id"]
	response, err := exchangeHandler.cruds[tableName].HandleActionRequest(request, req, transaction)

	log.Printf("Response from action exchange execution: %v", response)
	CheckErr(err, "Error from action exchange execution: %v")

	res := make(map[string]interface{})
	for _, r := range response {
		res[fmt.Sprintf("%v", r.ResponseType)] = r.Attributes
	}

	return res, err
}

func NewActionExchangeHandler(exchangeContract ExchangeContract, cruds map[string]*DbResource) ExternalExchange {

	return &ActionExchangeHandler{
		exchangeContract: exchangeContract,
		cruds:            cruds,
	}
}
