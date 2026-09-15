package resource

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/artpar/resty"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

const restExchangeTimeout = 30 * time.Second

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
		headerValue, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("REST exchange header [%s] must be a string", k)
		}
		headersMap[k] = headerValue
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

		queryParamsMap[k] = fmt.Sprint(v)
	}

	attrs := make(map[string]interface{})
	urlStr, err := EvaluateString(g.exchangeInformation.Url, inFieldMap)
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
	if err != nil {
		return nil, err
	}
	buildAttrs := buildAttrsInterface.(map[string]interface{})

	url, urlOK := buildAttrs["url"].(string)
	method, methodOK := buildAttrs["method"].(string)
	if !urlOK || url == "" || !methodOK || method == "" {
		return nil, fmt.Errorf("REST exchange requires string url and method")
	}

	requestFactory := resty.New().SetTimeout(restExchangeTimeout)
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
	case "patch":
		response, err = client.Patch(url)
	case "delete":
		response, err = client.Delete(url)
	default:
		return nil, fmt.Errorf("unsupported REST exchange method [%s]", method)
	}
	if response == nil {
		if err == nil {
			err = fmt.Errorf("REST exchange returned no response")
		}
		return nil, err
	}
	log.Printf("Response status from exchange execution: %d", response.StatusCode())
	log.Printf("Error from exchange execution: %v", err)

	res := make(map[string]interface{})
	res["headers"] = response.Header()
	if err != nil || response.IsError() {
		if responseBody := response.RawBody(); responseBody != nil {
			bodyBytes, readErr := io.ReadAll(responseBody)
			if readErr == nil {
				res["bodyString"] = string(bodyBytes)
				bodyAttrs := make(map[string]interface{})
				json.Unmarshal(bodyBytes, &bodyAttrs)
				res["body"] = bodyAttrs
			}
		}
		if response.IsError() {
			return res, fmt.Errorf("REST exchange returned HTTP status %d", response.StatusCode())
		}
	}

	return res, err
}

func NewRestExchangeHandler(exchangeContext ExchangeContract) (ExternalExchange, error) {

	var selected *RestExchange
	if exchangeContext.TargetType == "rest" {
		url, urlOK := exchangeContext.TargetAttributes["url"].(string)
		method, methodOK := exchangeContext.TargetAttributes["method"].(string)
		if !urlOK || url == "" || !methodOK || method == "" {
			return nil, fmt.Errorf("REST exchange requires target_attributes.url and target_attributes.method")
		}
		selected = &RestExchange{Name: "rest", Url: url, Method: method}
		if value, exists := exchangeContext.TargetAttributes["headers"]; exists {
			headers, ok := value.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("REST exchange target_attributes.headers must be an object")
			}
			selected.Headers = headers
		}
		if value, exists := exchangeContext.TargetAttributes["body"]; exists {
			body, ok := value.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("REST exchange target_attributes.body must be an object")
			}
			selected.Body = body
		}
		if value, exists := exchangeContext.TargetAttributes["query_params"]; exists {
			query, ok := value.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("REST exchange target_attributes.query_params must be an object")
			}
			selected.QueryParams = query
		}
	}

	for _, ra := range restExchanges {
		if ra.Name == exchangeContext.TargetType {
			selected = &ra
			break
		}
	}

	if selected == nil {
		return nil, fmt.Errorf("unknown REST target type [%s]", exchangeContext.TargetType)
	}

	return &RestExternalExchange{
		exchangeContract:    exchangeContext,
		exchangeInformation: selected,
	}, nil
}
