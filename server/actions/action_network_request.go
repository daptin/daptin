package actions

import (
	"encoding/base64"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/resty"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"reflect"
	"strings"
)

type networkRequestActionPerformer struct {
	responseAttrs map[string]interface{}
}

func (d *networkRequestActionPerformer) Name() string {
	return "$network.request"
}

func (d *networkRequestActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	headers, isHeader := inFieldMap["Headers"]
	headerMap := make(map[string]string)
	if isHeader {
		headerMapInterface := headers.(map[string]interface{})
		for key, val := range headerMapInterface {
			headerMap[key] = val.(string)
		}

	}

	url, isUrlPresent := inFieldMap["Url"]
	var urlString string
	if isUrlPresent {
		urlString = url.(string)
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
		formDataMap = ToURLQuery(formData.(map[string]interface{}))
		log.Debugf("Form values: %v", resource.ToJson(formDataMap))
	}

	queryParams, isQueryParams := inFieldMap["Query"]
	queryParamsMap := make(map[string]string)
	if isQueryParams {
		queryParamsMapInterface := queryParams.(map[string]interface{})
		for key, val := range queryParamsMapInterface {
			queryParamsMap[key] = val.(string)
		}
	}

	method, isMethodPresent := inFieldMap["Method"]
	if !isMethodPresent {
		method = "GET"
	}
	methodString := strings.ToUpper(method.(string))

	client := resty.New().R()
	resty.DetectContentType(false)

	if isBody {
		var bodyMapM []interface{}
		s, _ := json.Marshal(bodyMap)
		json.Unmarshal(s, &bodyMapM)
		client.SetBody(bodyMapM)

	}
	if isFormData {
		client.SetFormData(formDataMap)
	}
	client.SetHeaders(headerMap)
	client.SetQueryParams(queryParamsMap)

	var response *resty.Response
	var err error

	response, err = client.Execute(methodString, urlString)
	responseMap := make(map[string]interface{})
	if response == nil || err != nil {
		return nil, nil, []error{err}
	}

	responseHeaders := response.Header()
	responseContentType := responseHeaders.Get("Content-Type")
	responseBody := response.Body()
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

	handler := networkRequestActionPerformer{}

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
