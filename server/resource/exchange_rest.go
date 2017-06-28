package resource

import (
  log "github.com/sirupsen/logrus"
  "github.com/pkg/errors"
  "fmt"
  "gopkg.in/resty.v0"
  "golang.org/x/oauth2"
  "strings"
)

type ExternalExchange interface {
  ExecuteTarget(row map[string]interface{}, inFieldMap map[string]interface{}) error
}

type RestExchange struct {
  Name        string
  Method      string
  Url         string
  Headers     map[string]interface{}
  Body        map[string]interface{}
  QueryParams map[string]interface{}
}

var restExchanges = []RestExchange{
  {

    Name:   "gsheet-append",
    Method: "POST",
    Url:    "$sheetUrl",
    Headers: map[string]interface{}{
      "Accept": "application/json",
    },
    Body: map[string]interface{}{
      "values": []string{
        "!Object.keys(subject).sort().map(function(e){return subject[e];})",
      },
    },
    QueryParams: map[string]interface{}{
      "valueInputOption": "RAW",
      "key":              "AIzaSyAC2xame4NShrzH9ZJeEpWT5GkySooa0XM",
    },

  },
}

type RestExternalExchange struct {
  oauthToken          *oauth2.Token
  exchangeContract    ExchangeContract
  exchangeInformation *RestExchange
  oauthConfig         *oauth2.Config
}

func (g *RestExternalExchange) ExecuteTarget(row map[string]interface{}, inFieldMap map[string]interface{}) error {

  log.Infof("Execute rest external exchange")

  headersMap := make(map[string]string)

  headers := buildActionContext(g.exchangeInformation.Headers, inFieldMap).(map[string]interface{})

  for k, v := range headers {
    if v == nil {
      continue
    }
    headersMap[k] = v.(string)
  }

  queryParamsMap := make(map[string]string)
  queryParams := buildActionContext(g.exchangeInformation.QueryParams, inFieldMap).(map[string]interface{})

  for k, v := range queryParams {

    if v == nil {
      continue
    }

    queryParamsMap[k] = v.(string)
  }

  attrs := make(map[string]interface{})
  attrs["url"] = g.exchangeInformation.Url
  attrs["method"] = g.exchangeInformation.Method

  body := g.exchangeInformation.Body

  var bodyMap interface{}
  if len(body) == 0 {
    bodyMap = row
  } else {
    inFieldMap["subject"] = row
    bodyMap = buildActionContext(body, inFieldMap)
  }

  buildAttrs := buildActionContext(attrs, inFieldMap).(map[string]interface{})

  url := buildAttrs["url"].(string)
  method := buildAttrs["method"].(string)

  resty.SetDebug(true)
  client := resty.R()
  client.SetBody(bodyMap)

  client.SetHeaders(headersMap)
  client.SetQueryParams(queryParamsMap)

  client.SetAuthToken(g.oauthToken.AccessToken)

  method = strings.ToLower(method)

  var response *resty.Response
  var err error

  switch method {
  case "get":
    response, err = client.Get(url)
    break
  case "post":
    response, err = client.Post(url)
    break
  case "put":
    response, err = client.Put(url)
    break
  case "delete":
    response, err = client.Delete(url)
    break

  }
  log.Infof("Response from exchange execution: %v", response.String())
  log.Infof("Response from exchange execution: %v", err)

  return nil
}

func NewRestExchangeHandler(exchangeContext ExchangeContract, oauthToken *oauth2.Token, oauthConfig *oauth2.Config) (ExternalExchange, error) {

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
    oauthToken:          oauthToken,
    oauthConfig:         oauthConfig,
    exchangeContract:    exchangeContext,
    exchangeInformation: selected,
  }, nil
}
