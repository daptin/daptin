package resource

import (
	"github.com/daptin/daptin/server/auth"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	//"bytes"
	"bytes"
)

type ExchangeInterface interface {
	Update(target string, data []map[string]interface{}) error
}

type ExternalExchange interface {
	ExecuteTarget(row map[string]interface{}) error
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
	SourceType       string                 `db:"source_type"`
	TargetAttributes map[string]interface{} `db:"target_attributes"`
	TargetType       string                 `db:"target_type"`
	Attributes       []ColumnMap            `db:"attributes"`
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

func (ec *ExchangeExecution) Execute(data []map[string]interface{}) (err error) {

	var handler ExternalExchange

	switch ec.ExchangeContract.TargetType {
	case "action":
		handler = NewActionExchangeHandler(ec.ExchangeContract, *ec.cruds)
		break
	case "rest":
		handler, err = NewRestExchangeHandler(ec.ExchangeContract)
		if err != nil {
			return err
		}
		break
	default:
		log.Errorf("exchange contract: target: 'self' is not yet implemented")
		return errors.New("unknown target in exchange, not yet implemented")
	}

	//targetAttrs := ec.ExchangeContract.TargetAttributes
	//
	//for k, v := range targetAttrs {
	//	inFields[k] = v
	//}

	for _, row := range data {
		err = handler.ExecuteTarget(row)
		if err != nil {
			log.Errorf("Failed to execute target for [%v]: %v", row["__type"], err)
		}
	}

	return nil
}

func NewExchangeExecution(exchange ExchangeContract, cruds *map[string]*DbResource) *ExchangeExecution {

	return &ExchangeExecution{
		ExchangeContract: exchange,
		cruds:            cruds,
	}
}
