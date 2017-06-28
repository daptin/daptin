package resource

import (
  log "github.com/sirupsen/logrus"
  "net/http"
  "github.com/pkg/errors"
  "fmt"
)

type ExternalExchange interface {
  ExecuteTarget(inFields map[string]interface{}) error
}

type RestExchange struct {
  Name        string
  Method      string
  Url         string
  Headers     map[string]string
  Body        string
  QueryParams map[string]string
}

var restExchanges = []RestExchange{
  {

    Name:   "gsheet-append",
    Method: "POST",
    Url:    "https://sheets.googleapis.com/v4/spreadsheets/$spreadSheetId/values/$range:append?valueInputOption=$valueInputOption",
    Headers: map[string]string{
      "Accept": "application/json",
    },
    Body: "",
    QueryParams: map[string]string{
      "param1": "$sheetId",
    },

  },
}

type RestExternalExchange struct {
  RestExchange        RestExchange
  httpClient          *http.Client
  exchangeContract    ExchangeContract
  exchangeInformation *RestExchange
}

func (g *RestExternalExchange) ExecuteTarget(inFields map[string]interface{}) error {

  log.Infof("Execute rest external exchange")

  return nil
}

func NewRestExchangeHandler(exchangeContext ExchangeContract, inFields map[string]interface{}, httpClient *http.Client) (ExternalExchange, error) {

  found := false
  var selected *RestExchange

  for _, ra := range restExchanges {
    if ra.Name == exchangeContext.TargetType {
      found = true
      selected = &ra
    }
  }

  if !found {
    return nil, errors.New(fmt.Sprintf("Unknown target type [%v]", exchangeContext.TargetType))
  }

  return &RestExternalExchange{
    httpClient:          httpClient,
    exchangeContract:    exchangeContext,
    exchangeInformation: selected,
  }, nil
}
