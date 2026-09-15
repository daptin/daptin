package resource

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type ActionExchangeHandler struct {
	cruds            map[string]*DbResource
	exchangeContract ExchangeContract
}

func (exchangeHandler *ActionExchangeHandler) ExecuteTarget(row map[string]interface{}, transaction *sqlx.Tx) (map[string]interface{}, error) {

	rowType := row["__type"]
	log.Printf("Execute action exchange on: %v - %v", rowType, row["reference_id"])

	tableName, ok := exchangeHandler.exchangeContract.TargetAttributes["type"].(string)
	if !ok || tableName == "" {
		return nil, fmt.Errorf("action exchange [%s] requires target_attributes.type", exchangeHandler.exchangeContract.Name)
	}
	actionName, ok := exchangeHandler.exchangeContract.TargetAttributes["action"].(string)
	if !ok || actionName == "" {
		return nil, fmt.Errorf("action exchange [%s] requires target_attributes.action", exchangeHandler.exchangeContract.Name)
	}
	if exchangeHandler.cruds[tableName] == nil {
		return nil, fmt.Errorf("action exchange [%s] targets unknown resource [%s]", exchangeHandler.exchangeContract.Name, tableName)
	}
	targetAttributes := exchangeHandler.exchangeContract.TargetAttributes["attributes"]
	if targetAttributes == nil {
		targetAttributes = make(map[string]interface{})
	}
	configuredAttributes, ok := targetAttributes.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("action exchange [%s] target_attributes.attributes must be an object", exchangeHandler.exchangeContract.Name)
	}
	actionAttributes := make(map[string]interface{}, len(configuredAttributes)+2)
	for key, value := range configuredAttributes {
		actionAttributes[key] = value
	}
	request := actionresponse.ActionRequest{
		Type:       tableName,
		Action:     actionName,
		Attributes: actionAttributes,
	}
	//
	//if exchangeHandler.exchangeContract.SourceType == row["__type"] {
	//	request.Attributes[exchangeHandler.exchangeContract.SourceType+"_id"] = row["reference_id"]
	//}

	ur, _ := url.Parse("/" + tableName)

	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "POST",
			URL:    ur,
		},
	}

	sessionUser, err := exchangeSessionUser(exchangeHandler.cruds, exchangeHandler.exchangeContract.AsUserId, transaction)
	if err != nil {
		return nil, fmt.Errorf("resolve data exchange user: %w", err)
	}

	req.PlainRequest = req.PlainRequest.WithContext(context.WithValue(context.Background(), "user", sessionUser))

	request.Attributes["subject"] = row
	request.Attributes[tableName+"_id"] = exchangeSourceReference(row["reference_id"])
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
