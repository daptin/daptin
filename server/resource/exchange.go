package resource

import (
	"fmt"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	//"bytes"
	"bytes"
)

type ExchangeInterface interface {
	Update(target string, data []map[string]interface{}) error
}

type ExternalExchange interface {
	ExecuteTarget(row map[string]interface{}, transaction *sqlx.Tx) (map[string]interface{}, error)
}

type ColumnMap struct {
	SourceColumn     string
	SourceColumnType string
	TargetColumn     string
	TargetColumnType string
}

type ColumnMapping []ColumnMap

type ExchangeContract struct {
	Name             string
	SourceAttributes map[string]interface{} `db:"source_attributes"`
	Attributes       map[string]interface{} `db:"attributes"`
	SourceType       string                 `db:"source_type"`
	TargetAttributes map[string]interface{} `db:"target_attributes"`
	TargetType       string                 `db:"target_type"`
	User             auth.SessionUser
	Options          map[string]interface{}
	ReferenceId      string `db:"reference_id"`
	AsUserId         int64
}

var objectSuffix = []byte("{")
var arraySuffix = []byte("[")
var stringSuffix = []byte(`"`)

func (c *ColumnMapping) UnmarshalJSON(payload []byte) error {
	if bytes.HasPrefix(payload, objectSuffix) {
		return json.Unmarshal(payload, &c)
	}

	if bytes.HasPrefix(payload, arraySuffix) {
		return json.Unmarshal(payload, &c)
	}

	return errors.New("expected a JSON encoded object or array")
}

type ExchangeExecution struct {
	ExchangeContract ExchangeContract
	cruds            *map[string]*DbResource
}

func (exchangeExecution *ExchangeExecution) Execute(data []map[string]interface{}, transaction *sqlx.Tx) (result map[string]interface{}, err error) {

	var handler ExternalExchange

	switch exchangeExecution.ExchangeContract.TargetType {
	case "action":
		handler = NewActionExchangeHandler(exchangeExecution.ExchangeContract, *exchangeExecution.cruds)
	default:
		handler, err = NewRestExchangeHandler(exchangeExecution.ExchangeContract)
		if err != nil {
			return nil, fmt.Errorf("unknown data exchange target [%s]: %w", exchangeExecution.ExchangeContract.TargetType, err)
		}
	}

	//targetAttrs := exchangeExecution.ExchangeContract.TargetAttributes
	//
	//for k, v := range targetAttrs {
	//	inFields[k] = v
	//}

	for _, row := range data {
		result, err = handler.ExecuteTarget(row, transaction)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to execute target for [%v]", row["__type"])
		}
	}

	return result, err
}

func NewExchangeExecution(exchange ExchangeContract, cruds *map[string]*DbResource) *ExchangeExecution {

	return &ExchangeExecution{
		ExchangeContract: exchange,
		cruds:            cruds,
	}
}

func exchangeSessionUser(cruds map[string]*DbResource, userID int64, transaction *sqlx.Tx) (*auth.SessionUser, error) {
	userResource := cruds[USER_ACCOUNT_TABLE_NAME]
	if userResource == nil {
		return nil, fmt.Errorf("user_account resource is unavailable")
	}
	user, _, err := userResource.GetSingleRowById(USER_ACCOUNT_TABLE_NAME, userID, nil, transaction)
	if err != nil {
		return nil, fmt.Errorf("load user_account [%d]: %w", userID, err)
	}
	userReferenceID := daptinid.InterfaceToDIR(user["reference_id"])
	if userReferenceID == daptinid.NullReferenceId {
		return nil, fmt.Errorf("user_account [%d] has no reference_id", userID)
	}
	groups := userResource.GetObjectUserGroupsByWhereWithTransaction(USER_ACCOUNT_TABLE_NAME, transaction, "id", userID)
	authVersion := int64(1)
	if user[auth.AuthVersionColumn] != nil {
		authVersion, err = ResourceRowInt64(user[auth.AuthVersionColumn])
		if err != nil {
			return nil, fmt.Errorf("invalid auth_version for user_account [%d]: %w", userID, err)
		}
	}
	return &auth.SessionUser{
		UserId:          userID,
		UserReferenceId: userReferenceID,
		Groups:          groups,
		AuthVersion:     auth.AuthVersionOrDefault(authVersion),
	}, nil
}
