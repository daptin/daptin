package resource

import (
	"fmt"
	"github.com/artpar/resty"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"strings"
)

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
		Url:    "~sheetUrl",
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
			"key":              "~appKey",
		},
	},
}

type RestExternalExchange struct {
	exchangeContract    ExchangeContract
	exchangeInformation *RestExchange
}

func (g *RestExternalExchange) ExecuteTarget(row map[string]interface{}, transaction *sqlx.Tx) (map[string]interface{}, error) {

	log.Printf("Execute rest external exchange")

	headersMap := make(map[string]string)

	inFieldMap := make(map[string]interface{})

	for k, v := range g.exchangeContract.TargetAttributes {
		inFieldMap[k] = v
	}

	headInterface, err := BuildActionContext(g.exchangeInformation.Headers, inFieldMap)
	if err != nil {
		return nil, err
	}
	headers := headInterface.(map[string]interface{})

	for k, v := range headers {
		if v == nil {
			continue
		}
		headersMap[k] = v.(string)
	}

	queryParamsMap := make(map[string]string)
	queryInterface, err := BuildActionContext(g.exchangeInformation.QueryParams, inFieldMap)
	if err != nil {
		return nil, err
	}
	queryParams := queryInterface.(map[string]interface{})

	for k, v := range queryParams {

		if v == nil {
			continue
		}

		queryParamsMap[k] = v.(string)
	}

	attrs := make(map[string]interface{})
	urlStr, err := evaluateString(g.exchangeInformation.Url, inFieldMap)
	if err != nil {
		return nil, err
	}
	attrs["url"] = urlStr
	attrs["method"] = g.exchangeInformation.Method

	body := g.exchangeInformation.Body

	var bodyMap interface{}
	if len(body) == 0 {
		bodyMap = row
	} else {
		inFieldMap["subject"] = row
		bodyMap, err = BuildActionContext(body, inFieldMap)
		if err != nil {
			return nil, err
		}
	}

	buildAttrsInterface, err := BuildActionContext(attrs, inFieldMap)
	buildAttrs := buildAttrsInterface.(map[string]interface{})

	url := buildAttrs["url"].(string)
	method := buildAttrs["method"].(string)

	requestFactory := resty.New()
	requestFactory.Debug = true
	client := requestFactory.R()
	client.SetBody(bodyMap)

	client.SetHeaders(headersMap)
	client.SetQueryParams(queryParamsMap)

	//client.SetAuthToken(g.oauthToken.AccessToken)

	method = strings.ToLower(method)

	var response *resty.Response

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
	log.Printf("Response from exchange execution: %v", response.String())
	log.Printf("Error from exchange execution: %v", err)

	res := make(map[string]interface{})
	res["headers"] = response.Header()
	if err != nil {
		bodyBytes, err := ioutil.ReadAll(response.RawBody())
		if err == nil {
			res["bodyString"] = string(bodyBytes)
			bodyAttrs := make(map[string]interface{})
			json.Unmarshal(bodyBytes, &bodyAttrs)
			res["body"] = bodyAttrs
		}
	}

	return res, err
}

func NewRestExchangeHandler(exchangeContext ExchangeContract) (ExternalExchange, error) {

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
		exchangeContract:    exchangeContext,
		exchangeInformation: selected,
	}, nil
}
