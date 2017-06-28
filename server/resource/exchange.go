package resource

import (
  log "github.com/sirupsen/logrus"
  "net/http"
  "github.com/pkg/errors"
  //"bytes"
  "encoding/json"
  "bytes"
)

type ExchangeInterface interface {
  Update(target string, data []map[string]interface{}) error
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
  SourceAttributes map[string]interface{} `db:"source_name",json:"source_name"`
  SourceType       string `db:"source_type",json:"source_type"`
  TargetAttributes map[string]interface{} `db:"target_name",json:"target_name"`
  TargetType       string `db:"target_type",json:"target_type"`
  Attributes       []ColumnMap `db:"attributes",json:"attributes"`
  Options          map[string]interface{}
  ReferenceId      string `db:"reference_id",json:"reference_id"`
  OauthTokenId     *int64 `db:"oauth_token_id",json:"oauth_token_id"`
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
  httpClient       *http.Client
}

func (ec *ExchangeExecution) Execute(inFields map[string]interface{}, data []map[string]interface{}) (err error) {

  var handler ExternalExchange

  switch ec.ExchangeContract.TargetType {
  case "self":
    log.Errorf("self in target, not yet implemented")
    return errors.New("self in target, not yet implemented")
    break
  default:
    handler, err = NewRestExchangeHandler(ec.ExchangeContract, inFields, ec.httpClient)
    if err != nil {
      return err
    }
    break
  }

  for _, row := range data {
    err = handler.ExecuteTarget(row)
    if err != nil {
      log.Errorf("Failed to execute target for [%v]: %v", row["__type"], err)
    }
  }

  return nil
}

func NewExchangeExecution(exchange ExchangeContract, httpClient *http.Client) (*ExchangeExecution) {

  return &ExchangeExecution{
    ExchangeContract: exchange,
    httpClient:       httpClient,
  }
}
