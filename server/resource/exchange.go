package resource

import (
  "github.com/artpar/api2go"
  "golang.org/x/oauth2"
)

type ExchangeInterface interface {
  Update(target string, data []map[string]interface{}) error
}

type ExchangeContract struct {
  SourceName string
  SourceType string
  TargetName string
  TargetType string
  Attributes []api2go.ColumnInfo
}

type ExchangeExecution struct {
  ExchangeContract ExchangeContract
  token            *oauth2.Token
}

func (ec *ExchangeExecution) Update(target string, data []map[string]interface{}) error {

  var targetType string
  var targetName string

  switch target {
  case "source":
    targetType = ec.ExchangeContract.TargetType
    targetName = ec.ExchangeContract.TargetName
    break;
  case "target":
    targetType = ec.ExchangeContract.SourceType
    targetName = ec.ExchangeContract.SourceName
  }

  var handler ExternalExchange

  switch targetType {
  case "gsheet":
    handler = NewGsheetExternalExchange(ec.ExchangeContract.Attributes, ec.token)
  case "gdrive":
    handler = NewGdriveExternalExchange(ec.ExchangeContract.Attributes, ec.token)
  }

  handler.UpdateDestination(targetName, data)

  return nil
}
func (ec *ExchangeExecution) Read(target string, data []map[string]interface{}) ([]map[string]interface{}, error) {

  var targetType string
  var targetName string

  switch target {
  case "source":
    targetType = ec.ExchangeContract.TargetType
    targetName = ec.ExchangeContract.TargetName
    break;
  case "target":
    targetType = ec.ExchangeContract.SourceType
    targetName = ec.ExchangeContract.SourceName
  }

  var handler ExternalExchange

  switch targetType {
  case "gsheet":
    handler = NewGsheetExternalExchange(ec.ExchangeContract.Attributes, ec.token)
  case "gdrive":
    handler = NewGdriveExternalExchange(ec.ExchangeContract.Attributes, ec.token)
  }

  data, err := handler.ReadDestination(targetName)

  return data, err
}

func ExecuteExchange(cruds map[string]*DbResource, exchange ExchangeContract) error {

  return nil
}
