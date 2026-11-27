package actions

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/artpar/resty"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type networkRequestActionPerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*resource.DbResource
}

func (d *networkRequestActionPerformer) Name() string {
	return "$network.request"
}

func (d *networkRequestActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	headers, isHeader := inFieldMap["Headers"]
	headerMap := make(map[string]string)
	if isHeader {
		headerMapInterface, ok := headers.(map[string]interface{})
		if !ok {
			return nil, nil, []error{fmt.Errorf("Headers must be an object")}
		}
		for key, val := range headerMapInterface {
			text, ok := val.(string)
			if !ok {
				return nil, nil, []error{fmt.Errorf("header %s must be a string", key)}
			}
			headerMap[key] = text
		}

	}

	url, isUrlPresent := inFieldMap["Url"]
	var urlString string
	if isUrlPresent {
		var ok bool
		urlString, ok = url.(string)
		if !ok || urlString == "" {
			return nil, nil, []error{fmt.Errorf("Url must be a nonempty string")}
		}
	} else {
		return nil, nil, []error{fmt.Errorf("URL not present in action attributes")}
	}

	body, isBody := inFieldMap["Body"]
	var bodyMap interface{}
	if isBody {
		bodyMap = body.(interface{})
	}
	log.Debugf("Request body: %v", resource.ToJson(body))
	log.Debugf("Headers: %v", resource.ToJson(headerMap))

	formData, isFormData := inFieldMap["FormData"]
	formDataMap := make(map[string]string)
	if isFormData {
		log.Printf("FormData: %v", formData)
		form, ok := formData.(map[string]interface{})
		if !ok {
			return nil, nil, []error{fmt.Errorf("FormData must be an object")}
		}
		formDataMap = ToURLQuery(form)
		log.Debugf("Form values: %v", resource.ToJson(formDataMap))
	}

	queryParams, isQueryParams := inFieldMap["Query"]
	queryParamsMap := make(map[string]string)
	if isQueryParams {
		queryParamsMapInterface, ok := queryParams.(map[string]interface{})
		if !ok {
			return nil, nil, []error{fmt.Errorf("Query must be an object")}
		}
		for key, val := range queryParamsMapInterface {
			text, ok := val.(string)
			if !ok {
				return nil, nil, []error{fmt.Errorf("query %s must be a string", key)}
			}
			queryParamsMap[key] = text
		}
	}

	method, isMethodPresent := inFieldMap["Method"]
	if !isMethodPresent {
		method = "GET"
	}
	methodText, ok := method.(string)
	if !ok {
		return nil, nil, []error{fmt.Errorf("Method must be a string")}
	}
	methodString := strings.ToUpper(methodText)

	ctx := context.Background()
	if request, ok := inFieldMap["httpRequest"].(*http.Request); ok && request != nil {
		ctx = request.Context()
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := resty.New().SetTimeout(30 * time.Second).R().SetContext(bounded).SetDoNotParseResponse(true)
	resty.DetectContentType(false)

	if isBody {
		client.SetBody(bodyMap)

	}
	if isFormData {
		client.SetFormData(formDataMap)
	}
	client.SetHeaders(headerMap)
	client.SetQueryParams(queryParamsMap)

	var response *resty.Response
	var err error

	if transaction != nil {
		err = transaction.Commit()
		if err != nil {
			return nil, nil, []error{err}
		}
	}

	response, err = client.Execute(methodString, urlString)

	var responseBody []byte
	if response != nil && err == nil {
		responseBody, err = io.ReadAll(io.LimitReader(response.RawBody(), (1<<20)+1))
		_ = response.RawBody().Close()
		if err == nil && len(responseBody) > 1<<20 {
			err = fmt.Errorf("network response exceeds 1 MiB")
		}
	}

	if transaction != nil {
		newTransaction, beginErr := d.cruds["world"].BeginTransaction(ctx)
		if beginErr != nil {
			return nil, nil, []error{beginErr}
		}
		*transaction = *newTransaction
	}

	responseMap := make(map[string]interface{})
	if response == nil || err != nil {
		return nil, nil, []error{err}
	}

	responseHeaders := response.Header()
	responseContentType := responseHeaders.Get("Content-Type")
	if strings.Index(responseContentType, "application/json") > -1 {
		m := make(map[string]interface{})
		err = json.Unmarshal(responseBody, &m)
		if err != nil {
			log.Errorf("Failed to read response body: %v: %v", err, response.String())
		}
		responseMap["body"] = m
	} else {
		responseMap["body"] = string(responseBody)
		var m interface{}
		err := json.Unmarshal(responseBody, &m)
		if err == nil {
			responseMap["body"] = m
			responseMap["bodyPlainText"] = string(responseBody)
		}
		responseMap["base32EncodedBody"] = base64.StdEncoding.EncodeToString(responseBody)
	}
	log.Printf("Response body [%v][%v]: %v", methodString, urlString, responseMap["body"])
	responseMap["headers"] = responseHeaders

	return api2go.Response{
			Res: api2go.NewApi2GoModelWithData("$network.response", nil, 0, nil, responseMap),
		}, []actionresponse.ActionResponse{{
			ResponseType: request.Type,
			Attributes:   responseMap,
		}}, []error{}
}

func NewNetworkRequestPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := networkRequestActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}

// encodeQuery is a recursive function that generates URL-encoded query strings
func encodeQuery(key string, value interface{}, v map[string]string) {
	rv := reflect.ValueOf(value)

	switch rv.Kind() {
	case reflect.Map:
		for _, k := range rv.MapKeys() {
			mapKey := fmt.Sprintf("%v", k)
			encodeQuery(fmt.Sprintf("%s[%s]", key, mapKey), rv.MapIndex(k).Interface(), v)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			encodeQuery(fmt.Sprintf("%s[%d]", key, i), rv.Index(i).Interface(), v)
		}
	case reflect.Struct:
		for i := 0; i < rv.NumField(); i++ {
			field := rv.Type().Field(i)
			fieldName := field.Name
			fieldValue := rv.Field(i).Interface()
			encodeQuery(fmt.Sprintf("%s.%s", key, fieldName), fieldValue, v)
		}
	default:
		v[key] = fmt.Sprintf("%v", value)
	}
}

// ToURLQuery converts a Go object into a x-www-form-urlencoded query string
func ToURLQuery(input interface{}) map[string]string {
	v := map[string]string{}
	rv := reflect.ValueOf(input)

	if rv.Kind() == reflect.Map {
		for _, k := range rv.MapKeys() {
			mapKey := fmt.Sprintf("%v", k)
			encodeQuery(mapKey, rv.MapIndex(k).Interface(), v)
		}
	} else if rv.Kind() == reflect.Struct {
		for i := 0; i < rv.NumField(); i++ {
			field := rv.Type().Field(i)
			fieldName := field.Name
			fieldValue := rv.Field(i).Interface()
			encodeQuery(fieldName, fieldValue, v)
		}
	} else {
		encodeQuery("", input, v)
	}

	return v
}
